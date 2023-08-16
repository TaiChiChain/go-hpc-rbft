// Copyright 2016-2017 Hyperchain Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rbft

import (
	"context"
	"fmt"

	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/types"
	"github.com/gogo/protobuf/proto"
)

// executor manages exec related params
type executor struct {
	lastExec uint64
}

// newExecutor initializes an instance of executor
func newExecutor() *executor {
	exec := &executor{}
	return exec
}

// setLastExec sets the value of lastExec
func (e *executor) setLastExec(l uint64) {
	e.lastExec = l
}

// msgToEvent converts ConsensusMessage to the corresponding consensus event.
func (rbft *rbftImpl[T, Constraint]) msgToEvent(msg *consensus.ConsensusMessage) (interface{}, error) {
	// reuse FetchBatchResponse to avoid too many useless FetchBatchResponse message caused by
	// broadcast fetch.
	if msg.Type == consensus.Type_FETCH_BATCH_RESPONSE {
		err := proto.Unmarshal(msg.Payload, rbft.reusableRequestBatch)
		if err != nil {
			rbft.logger.Errorf("Unmarshal error, can not unmarshal %v, error: %v", msg.Type, err)
			return nil, err
		}
		digest := rbft.reusableRequestBatch.BatchDigest
		if _, ok := rbft.storeMgr.missingReqBatches[digest]; !ok {
			// either the wrong digest, or we got it already from someone else
			rbft.logger.Debugf("Replica %d received missing request batch: %s, but we don't miss "+
				"this request, ignore it", rbft.peerPool.ID, digest)
			return nil, fmt.Errorf("useless request batch with %s", digest)
		}
	}

	if fn, ok := eventCreators[msg.Type]; ok {
		event, ok := fn().(proto.Message)
		if !ok {
			rbft.logger.Errorf("Unmarshal error, can not unmarshal msg type %v", msg.Type)
			return nil, fmt.Errorf("unknown msg type %v", msg.Type)
		}
		err := proto.Unmarshal(msg.Payload, event)
		if err != nil {
			rbft.logger.Errorf("Unmarshal error, can not unmarshal %v, error: %v", msg.Type, err)
			return nil, err
		}
		return event, nil
	}

	return nil, fmt.Errorf("unknown msg type %v", msg.Type)
}

var eventCreators map[consensus.Type]func() interface{}

// initMsgEventMap maps consensus_message to real consensus msg type which used to Unmarshal consensus_message's payload
// to actual consensus msg
func initMsgEventMap() {
	eventCreators = make(map[consensus.Type]func() interface{})

	eventCreators[consensus.Type_NULL_REQUEST] = func() interface{} { return &consensus.NullRequest{} }
	eventCreators[consensus.Type_PRE_PREPARE] = func() interface{} { return &consensus.PrePrepare{} }
	eventCreators[consensus.Type_PREPARE] = func() interface{} { return &consensus.Prepare{} }
	eventCreators[consensus.Type_COMMIT] = func() interface{} { return &consensus.Commit{} }
	eventCreators[consensus.Type_SIGNED_CHECKPOINT] = func() interface{} { return &consensus.SignedCheckpoint{} }
	eventCreators[consensus.Type_FETCH_CHECKPOINT] = func() interface{} { return &consensus.FetchCheckpoint{} }
	eventCreators[consensus.Type_VIEW_CHANGE] = func() interface{} { return &consensus.ViewChange{} }
	eventCreators[consensus.Type_QUORUM_VIEW_CHANGE] = func() interface{} { return &consensus.QuorumViewChange{} }
	eventCreators[consensus.Type_NEW_VIEW] = func() interface{} { return &consensus.NewView{} }
	eventCreators[consensus.Type_FETCH_VIEW] = func() interface{} { return &consensus.FetchView{} }
	eventCreators[consensus.Type_RECOVERY_RESPONSE] = func() interface{} { return &consensus.RecoveryResponse{} }
	eventCreators[consensus.Type_FETCH_BATCH_REQUEST] = func() interface{} { return &consensus.FetchBatchRequest{} }
	eventCreators[consensus.Type_FETCH_BATCH_RESPONSE] = func() interface{} { return &consensus.FetchBatchResponse{} }
	eventCreators[consensus.Type_FETCH_PQC_REQUEST] = func() interface{} { return &consensus.FetchPQCRequest{} }
	eventCreators[consensus.Type_FETCH_PQC_RESPONSE] = func() interface{} { return &consensus.FetchPQCResponse{} }
	eventCreators[consensus.Type_FETCH_MISSING_REQUEST] = func() interface{} { return &consensus.FetchMissingRequest{} }
	eventCreators[consensus.Type_FETCH_MISSING_RESPONSE] = func() interface{} { return &consensus.FetchMissingResponse{} }
	eventCreators[consensus.Type_SYNC_STATE] = func() interface{} { return &consensus.SyncState{} }
	eventCreators[consensus.Type_SYNC_STATE_RESPONSE] = func() interface{} { return &consensus.SyncStateResponse{} }
	eventCreators[consensus.Type_EPOCH_CHANGE_REQUEST] = func() interface{} { return &consensus.EpochChangeRequest{} }
	eventCreators[consensus.Type_EPOCH_CHANGE_PROOF] = func() interface{} { return &consensus.EpochChangeProof{} }
}

// dispatchLocalEvent dispatches local Event to corresponding handles using its service type
func (rbft *rbftImpl[T, Constraint]) dispatchLocalEvent(e *LocalEvent) consensusEvent {
	switch e.Service {
	case CoreRbftService:
		return rbft.handleCoreRbftEvent(e)
	case ViewChangeService:
		return rbft.handleViewChangeEvent(e)
	case RecoveryService:
		return rbft.handleRecoveryEvent(e)
	case EpochMgrService:
		return rbft.handleEpochMgrEvent(e)
	default:
		rbft.logger.Errorf("Not Supported event: %v", e)
		return nil
	}
}

// dispatchMiscEvent dispatches misc Event to corresponding handles using its event type
func (rbft *rbftImpl[T, Constraint]) dispatchMiscEvent(e *MiscEvent) consensusEvent {
	switch e.EventType {
	case ReqTxEvent:
		return rbft.handleReqTxEvent(e.Event.(*ReqTxMsg))
	case ReqNonceEvent:
		return rbft.handleReqNonceEvent(e.Event.(*ReqNonceMsg))
	default:
		rbft.logger.Errorf("Not Supported event: %v", e)
		return nil
	}
}

func (rbft *rbftImpl[T, Constraint]) handleReqTxEvent(e *ReqTxMsg) consensusEvent {
	e.ch <- rbft.batchMgr.requestPool.GetPendingTxByHash(e.hash)
	return nil
}

func (rbft *rbftImpl[T, Constraint]) handleReqNonceEvent(e *ReqNonceMsg) consensusEvent {
	e.ch <- rbft.batchMgr.requestPool.GetPendingNonceByAccount(e.account)
	return nil
}

// handleCoreRbftEvent handles core RBFT service events
func (rbft *rbftImpl[T, Constraint]) handleCoreRbftEvent(e *LocalEvent) consensusEvent {
	switch e.EventType {
	case CoreBatchTimerEvent:
		if !rbft.isNormal() {
			rbft.logger.Debugf("Replica %d is in abnormal, not try to create a batch", rbft.peerPool.ID)
			rbft.stopBatchTimer()
			return nil
		}
		if !rbft.isPrimary(rbft.peerPool.ID) {
			rbft.logger.Debugf("Replica %d is not primary, not try to create a batch", rbft.peerPool.ID)
			rbft.stopBatchTimer()
			return nil
		}
		rbft.logger.Debugf("Primary %d batch timer expired, try to create a batch", rbft.peerPool.ID)

		if rbft.atomicIn(InConfChange) {
			rbft.logger.Debugf("Replica %d is processing a config transaction, cannot generate batches", rbft.peerPool.ID)
			rbft.restartBatchTimer()
			return nil
		}

		if len(rbft.batchMgr.cacheBatch) > 0 {
			rbft.restartBatchTimer()
			rbft.maybeSendPrePrepare(nil, true)
			return nil
		}
		rbft.stopBatchTimer()

		// call requestPool module to generate a tx batch
		batches := rbft.batchMgr.requestPool.GenerateRequestBatch()
		rbft.postBatches(batches)

		return nil

	case CoreNoTxBatchTimerEvent:
		if !rbft.isNormal() {
			rbft.logger.Debugf("Replica %d is in abnormal, not try to create a no-tx batch", rbft.peerPool.ID)
			rbft.stopNoTxBatchTimer()
			return nil
		}
		if !rbft.isPrimary(rbft.peerPool.ID) {
			rbft.logger.Debugf("Replica %d is not primary, not try to create a no-tx batch", rbft.peerPool.ID)
			rbft.stopNoTxBatchTimer()
			return nil
		}
		if !rbft.config.IsTimed {
			rbft.logger.Debugf("Replica %d is not support generate no-tx batch", rbft.peerPool.ID)
			rbft.stopNoTxBatchTimer()
			return nil
		}

		rbft.logger.Debugf("Primary %d no-tx batch timer expired, try to create a no-tx batch", rbft.peerPool.ID)

		if rbft.atomicIn(InConfChange) {
			rbft.logger.Debugf("Replica %d is processing a config transaction, cannot generate no-tx batches", rbft.peerPool.ID)
			rbft.restartNoTxBatchTimer()
			return nil
		}

		if len(rbft.batchMgr.cacheBatch) > 0 || rbft.batchMgr.requestPool.HasPendingRequestInPool() {
			rbft.logger.Warningf("mempool is not empty, cannot generate no-tx batches", rbft.peerPool.ID)
			rbft.stopNoTxBatchTimer()
			return nil
		}
		rbft.stopNoTxBatchTimer()

		// call requestPool module to generate a tx batch
		batches := rbft.batchMgr.requestPool.GenerateRequestBatch()
		rbft.postBatches(batches)

		return nil

	case CoreNullRequestTimerEvent:
		rbft.handleNullRequestTimerEvent()
		return nil

	case CoreCheckPoolTimerEvent:
		if !rbft.isNormal() {
			return nil
		}
		rbft.processOutOfDateReqs()
		rbft.restartCheckPoolTimer()
		return nil

	case CoreStateUpdatedEvent:
		return rbft.recvStateUpdatedEvent(e.Event.(*types.ServiceState))

	case CoreHighWatermarkEvent:
		if rbft.atomicIn(InViewChange) {
			rbft.logger.Debugf("Replica %d is in viewChange, ignore the high-watermark event")
			return nil
		}

		// get the previous low-watermark from event
		preLowWatermark, ok := e.Event.(uint64)
		if !ok {
			rbft.logger.Error("previous low-watermark parsing error")
			return nil
		}

		// if the watermark has already been updated, just finish the timeout process
		if preLowWatermark < rbft.h {
			rbft.logger.Debugf("Replica %d has already updated low-watermark to %d, just return", rbft.peerPool.ID, rbft.h)
			return nil
		}

		rbft.logger.Infof("Replica %d high-watermark timer expired, send view change: %s",
			rbft.peerPool.ID, rbft.highWatermarkTimerReason)
		return rbft.sendViewChange()

	case CoreCheckPoolRemoveTimerEvent:
		rbft.processNeedRemoveReqs()
		rbft.restartCheckPoolRemoveTimer()
		return nil

	default:
		rbft.logger.Errorf("Invalid core RBFT event: %v", e)
		return nil
	}
}

// handleRecoveryEvent handles recovery services related events.
func (rbft *rbftImpl[T, Constraint]) handleRecoveryEvent(e *LocalEvent) consensusEvent {
	switch e.EventType {
	case RecoveryInitEvent:
		preView, ok := e.Event.(uint64)
		if !ok {
			rbft.logger.Errorf("Replica %d parsing error, rbft.view", rbft.peerPool.ID)
			rbft.stopNamespace()
			return nil
		}

		if preView < rbft.view {
			rbft.logger.Debugf("Replica %d has initiated recovery, ignore the event", rbft.peerPool.ID)
			return nil
		}

		rbft.logger.Debugf("Replica %d init recovery", rbft.peerPool.ID)
		return rbft.initRecovery()

	case RecoverySyncStateRspTimerEvent:
		rbft.logger.Noticef("Replica %d sync state response timer expired", rbft.peerPool.ID)
		rbft.exitSyncState()
		if !rbft.isNormal() {
			rbft.logger.Noticef("Replica %d is in abnormal, not try view change", rbft.peerPool.ID)
			return nil
		}
		return rbft.initRecovery()
	case RecoverySyncStateRestartTimerEvent:
		rbft.logger.Debugf("Replica %d sync state restart timer expired", rbft.peerPool.ID)
		rbft.exitSyncState()
		if !rbft.isNormal() {
			rbft.logger.Debugf("Replica %d is in abnormal, not restart sync state", rbft.peerPool.ID)
			return nil
		}
		return rbft.restartSyncState()
	default:
		rbft.logger.Errorf("Invalid recovery service events : %v", e)
		return nil
	}
}

// handleViewChangeEvent handles view change service related events.
func (rbft *rbftImpl[T, Constraint]) handleViewChangeEvent(e *LocalEvent) consensusEvent {
	switch e.EventType {
	case ViewChangeTimerEvent:
		demand, ok := e.Event.(nextDemandNewView)
		if ok {
			if rbft.view > uint64(demand) {
				rbft.logger.Debugf("Replica %d received viewChangeTimerEvent, but we"+
					"have sent the next viewChange maybe just before a moment.", rbft.peerPool.ID)
				return nil
			}
		} else if rbft.atomicIn(InViewChange) {
			rbft.logger.Debugf("Replica %d received viewChangeTimerEvent, but we"+
				"are already in view-change and it has not reached quorum.", rbft.peerPool.ID)
			return nil
		}

		rbft.logger.Infof("Replica %d viewChange timer expired, sending viewChange: %s", rbft.peerPool.ID, rbft.vcMgr.newViewTimerReason)

		// Here, we directly send viewchange with a bigger target view (which is rbft.view+1) because it is the
		// new view timer who triggered this ViewChangeTimerEvent so we send a new viewchange request
		return rbft.sendViewChange()

	case ViewChangeDoneEvent:
		// set a viewChangeSeqNo if needed
		rbft.updateViewChangeSeqNo(rbft.exec.lastExec, rbft.K)
		delete(rbft.vcMgr.newViewStore, rbft.view)
		rbft.storeMgr.missingBatchesInFetching = make(map[string]msgID)

		rbft.stopNewViewTimer()
		rbft.stopFetchViewTimer()
		rbft.startTimerIfOutstandingRequests()
		primaryID := rbft.primaryID(rbft.view)

		// set normal to 1 which indicates system comes into normal status after viewchange
		rbft.atomicOff(InViewChange)
		rbft.metrics.statusGaugeInViewChange.Set(0)
		var finishMsg string
		if rbft.atomicIn(InRecovery) {
			rbft.atomicOff(InRecovery)
			rbft.metrics.statusGaugeInRecovery.Set(0)
			rbft.logger.Noticef("======== Replica %d finished recovery, primary=%d, "+
				"epoch=%d/n=%d/f=%d/view=%d/h=%d/lastExec=%d", rbft.peerPool.ID, primaryID,
				rbft.epoch, rbft.N, rbft.f, rbft.view, rbft.h, rbft.exec.lastExec)

			rbft.logger.Notice(`

  +==============================================+
  |                                              |
  |            RBFT Recovery Finished            |
  |                                              |
  +==============================================+

`)
			finishMsg = fmt.Sprintf("======== Replica %d finished recovery, primary=%d, "+
				"epoch=%d/n=%d/f=%d/view=%d/h=%d/lastExec=%d", rbft.peerPool.ID, primaryID,
				rbft.epoch, rbft.N, rbft.f, rbft.view, rbft.h, rbft.exec.lastExec)
			rbft.external.SendFilterEvent(types.InformTypeFilterFinishRecovery, finishMsg)

			// start noTx batch timer if there is no pending tx in pool
			if rbft.config.IsTimed && !rbft.batchMgr.requestPool.HasPendingRequestInPool() {
				if !rbft.batchMgr.isNoTxBatchTimerActive() {
					rbft.startNoTxBatchTimer()
				}
			}
		} else {
			rbft.logger.Noticef("======== Replica %d finished viewChange, primary=%d, "+
				"epoch=%d/n=%d/f=%d/view=%d/h=%d/lastExec=%d", rbft.peerPool.ID, primaryID,
				rbft.epoch, rbft.N, rbft.f, rbft.view, rbft.h, rbft.exec.lastExec)

			finishMsg = fmt.Sprintf("======== Replica %d finished viewChange, primary=%d, "+
				"epoch=%d/n=%d/f=%d/view=%d/h=%d/lastExec=%d", rbft.peerPool.ID, primaryID,
				rbft.epoch, rbft.N, rbft.f, rbft.view, rbft.h, rbft.exec.lastExec)
			rbft.external.SendFilterEvent(types.InformTypeFilterFinishViewChange, finishMsg)
		}
		rbft.maybeSetNormal()
		rbft.logger.Trace(consensus.TagNameViewChange, consensus.TagStageFinish, consensus.TagContentViewChange{
			Node: rbft.peerPool.ID,
			View: rbft.view,
		})

		// clear storage from lower view
		for idx := range rbft.vcMgr.viewChangeStore {
			if idx.v <= rbft.view {
				delete(rbft.vcMgr.viewChangeStore, idx)
			}
		}

		for view := range rbft.vcMgr.newViewStore {
			if view < rbft.view {
				delete(rbft.vcMgr.newViewStore, view)
			}
		}

		for idx := range rbft.vcMgr.newViewCache {
			if idx.targetView <= rbft.view {
				delete(rbft.vcMgr.newViewCache, idx)
			}
		}

		// check if epoch has been changed, if changed, trigger another round of view change
		// after epoch change to find correct view-number
		epochChanged := rbft.syncEpoch()
		if epochChanged {
			rbft.logger.Debugf("Replica %d sending view change again because of epoch change", rbft.peerPool.ID)
			// trigger another round of view change after epoch change to find correct view-number
			return rbft.initRecovery()
		}

		if rbft.isPrimary(rbft.peerPool.ID) {
			rbft.primaryResubmitTransactions()
		} else {
			// here, we always fetch PQC after finish recovery as we only recovery to the largest checkpoint which
			// is lower or equal to the lastExec quorum of others.
			rbft.fetchRecoveryPQC()
		}

	case ViewChangeResendTimerEvent:
		if !rbft.atomicIn(InViewChange) {
			rbft.logger.Warningf("Replica %d had its viewChange resend timer expired but it's "+
				"not in viewChange, ignore it", rbft.peerPool.ID)
			return nil
		}
		rbft.logger.Infof("Replica %d viewChange resend timer expired before viewChange quorum "+
			"was reached, try recovery", rbft.peerPool.ID)

		// if viewChange resend timer expired, directly enter recovery status,
		// decrease view first as we may increase view when send view change.
		newView := rbft.view - uint64(1)
		rbft.setView(newView)

		return rbft.initRecovery()

	case ViewChangeQuorumEvent:
		rbft.logger.Infof("Replica %d received viewChange quorum, processing new view", rbft.peerPool.ID)
		if rbft.isPrimary(rbft.peerPool.ID) {
			// if we are catching up, don't send new view as a primary and after a while, other nodes will
			// send a new viewchange whose seqNo=previous viewchange's seqNo + 1 because of new view timeout
			// and eventually others will finish viewchange with a new view in which primary is not in
			// skipInProgress
			if rbft.in(SkipInProgress) {
				rbft.logger.Debugf("Replica %d is in catching up, don't send new view", rbft.peerPool.ID)
				return nil
			}
			// primary construct and send new view message
			return rbft.sendNewView()
		}
		return rbft.replicaCheckNewView()

	case FetchViewEvent:
		rbft.logger.Debugf("Replica %d fetch view timer expired", rbft.peerPool.ID)
		rbft.tryFetchView()

	default:
		rbft.logger.Errorf("Invalid viewChange event: %v", e)
		return nil
	}
	return nil
}

// handleViewChangeEvent handles epoch service related events.
func (rbft *rbftImpl[T, Constraint]) handleEpochMgrEvent(e *LocalEvent) consensusEvent {
	switch e.EventType {
	case FetchCheckpointEvent:
		rbft.logger.Debugf("Replica %d fetch checkpoint timer expired", rbft.peerPool.ID)
		rbft.stopFetchCheckpointTimer()
		rbft.fetchCheckpoint()

	case EpochSyncEvent:
		proof := e.Event.(*consensus.EpochChangeProof)
		quorumCheckpoint := proof.Last()
		rbft.logger.Noticef("Replica %d try epoch sync to height %d, epoch %d", rbft.peerPool.ID,
			quorumCheckpoint.Height(), quorumCheckpoint.NextEpoch())

		// block consensus progress until sync to epoch change height.
		rbft.atomicOn(inEpochSyncing)
		target := &types.MetaState{
			Height: quorumCheckpoint.Height(),
			Digest: quorumCheckpoint.Digest(),
		}
		var checkpointSet []*consensus.SignedCheckpoint
		for _, sig := range quorumCheckpoint.Signatures {
			checkpointSet = append(checkpointSet, &consensus.SignedCheckpoint{
				Checkpoint: quorumCheckpoint.Checkpoint,
				Signature:  sig,
			})
		}
		rbft.updateHighStateTarget(target, checkpointSet, proof.GetCheckpoints()...) // for new epoch
		rbft.tryStateTransfer()

	default:
		rbft.logger.Errorf("Invalid viewChange quorumCheckpoint: %v", e)
		return nil
	}
	return nil
}

// dispatchConsensusMsg dispatches consensus messages to corresponding handlers using its service type
func (rbft *rbftImpl[T, Constraint]) dispatchConsensusMsg(ctx context.Context, e consensusEvent) consensusEvent {
	service := rbft.dispatchMsgToService(e)
	switch service {
	case CoreRbftService:
		return rbft.dispatchCoreRbftMsg(ctx, e)
	case ViewChangeService:
		return rbft.dispatchViewChangeMsg(e)
	case RecoveryService:
		return rbft.dispatchRecoveryMsg(e)
	case EpochMgrService:
		return rbft.dispatchEpochMsg(e)
	default:
		rbft.logger.Errorf("Not Supported event: %v", e)
	}
	return nil
}

// dispatchMsgToService returns the service type of the given event. There exist 4 service types:
// 1. CoreRbftService: including tx related events, pre-prepare, prepare, commit, checkpoint, missing txs related events
// 2. ViewChangeService: including viewChange related events
// 3. RecoveryService: including recovery related events
// 4. EpochMgrService: including epochManager related events
func (rbft *rbftImpl[T, Constraint]) dispatchMsgToService(e consensusEvent) int {
	switch e.(type) {
	// core RBFT service
	case *consensus.NullRequest:
		return CoreRbftService
	case *consensus.PrePrepare:
		return CoreRbftService
	case *consensus.Prepare:
		return CoreRbftService
	case *consensus.Commit:
		return CoreRbftService
	case *consensus.FetchMissingRequest:
		return CoreRbftService
	case *consensus.FetchMissingResponse:
		return CoreRbftService
	case *consensus.SignedCheckpoint:
		return CoreRbftService

		// view change service
	case *consensus.ViewChange:
		return ViewChangeService
	case *consensus.NewView:
		return ViewChangeService
	case *consensus.FetchBatchRequest:
		return ViewChangeService
	case *consensus.FetchBatchResponse:
		return ViewChangeService
	case *consensus.FetchView:
		return ViewChangeService
	case *consensus.RecoveryResponse:
		return ViewChangeService
	case *consensus.QuorumViewChange:
		return ViewChangeService

		// recovery service
	case *consensus.FetchPQCRequest:
		return RecoveryService
	case *consensus.FetchPQCResponse:
		return RecoveryService
	case *consensus.SyncState:
		return RecoveryService
	case *consensus.SyncStateResponse:
		return RecoveryService
	case *consensus.FetchCheckpoint:
		return EpochMgrService
	case *consensus.EpochChangeRequest:
		return EpochMgrService
	case *consensus.EpochChangeProof:
		return EpochMgrService
	default:
		return NotSupportService

	}
}
