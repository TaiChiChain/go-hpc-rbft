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
	"time"

	"github.com/axiomesh/axiom-kit/txpool"

	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/types"
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
func (rbft *rbftImpl[T, Constraint]) msgToEvent(msg *consensus.ConsensusMessage) (any, error) {
	// reuse FetchBatchResponse to avoid too many useless FetchBatchResponse message caused by
	// broadcast fetch.
	if msg.Type == consensus.Type_FETCH_BATCH_RESPONSE {
		err := rbft.reusableRequestBatch.UnmarshalVT(msg.Payload)
		if err != nil {
			rbft.logger.Errorf("Unmarshal error, can not unmarshal %v, error: %v", msg.Type, err)
			return nil, err
		}
		digest := rbft.reusableRequestBatch.BatchDigest
		if _, ok := rbft.storeMgr.missingReqBatches[digest]; !ok {
			// either the wrong digest, or we got it already from someone else
			rbft.logger.Debugf("Replica %d received missing request batch: %s, but we don't miss "+
				"this request, ignore it", rbft.chainConfig.SelfID, digest)
			return nil, fmt.Errorf("useless request batch with %s", digest)
		}
	}

	if fn, ok := eventCreators[msg.Type]; ok {
		event := fn()
		err := event.UnmarshalVT(msg.Payload)
		if err != nil {
			rbft.logger.Errorf("Unmarshal error, can not unmarshal %v, error: %v", msg.Type, err)
			return nil, err
		}
		return event, nil
	}

	return nil, fmt.Errorf("unknown msg type %v", msg.Type)
}

var eventCreators map[consensus.Type]func() consensus.Message

// initMsgEventMap maps consensus_message to real consensus msg type which used to Unmarshal consensus_message's payload
// to actual consensus msg
func initMsgEventMap() {
	eventCreators = make(map[consensus.Type]func() consensus.Message)

	eventCreators[consensus.Type_NULL_REQUEST] = func() consensus.Message { return &consensus.NullRequest{} }
	eventCreators[consensus.Type_PRE_PREPARE] = func() consensus.Message { return &consensus.PrePrepare{} }
	eventCreators[consensus.Type_PREPARE] = func() consensus.Message { return &consensus.Prepare{} }
	eventCreators[consensus.Type_COMMIT] = func() consensus.Message { return &consensus.Commit{} }
	eventCreators[consensus.Type_SIGNED_CHECKPOINT] = func() consensus.Message { return &consensus.SignedCheckpoint{} }
	eventCreators[consensus.Type_FETCH_CHECKPOINT] = func() consensus.Message { return &consensus.FetchCheckpoint{} }
	eventCreators[consensus.Type_VIEW_CHANGE] = func() consensus.Message { return &consensus.ViewChange{} }
	eventCreators[consensus.Type_QUORUM_VIEW_CHANGE] = func() consensus.Message { return &consensus.QuorumViewChange{} }
	eventCreators[consensus.Type_NEW_VIEW] = func() consensus.Message { return &consensus.NewView{} }
	eventCreators[consensus.Type_FETCH_VIEW] = func() consensus.Message { return &consensus.FetchView{} }
	eventCreators[consensus.Type_RECOVERY_RESPONSE] = func() consensus.Message { return &consensus.RecoveryResponse{} }
	eventCreators[consensus.Type_FETCH_BATCH_REQUEST] = func() consensus.Message { return &consensus.FetchBatchRequest{} }
	eventCreators[consensus.Type_FETCH_BATCH_RESPONSE] = func() consensus.Message { return &consensus.FetchBatchResponse{} }
	eventCreators[consensus.Type_FETCH_PQC_REQUEST] = func() consensus.Message { return &consensus.FetchPQCRequest{} }
	eventCreators[consensus.Type_FETCH_PQC_RESPONSE] = func() consensus.Message { return &consensus.FetchPQCResponse{} }
	eventCreators[consensus.Type_FETCH_MISSING_REQUEST] = func() consensus.Message { return &consensus.FetchMissingRequest{} }
	eventCreators[consensus.Type_FETCH_MISSING_RESPONSE] = func() consensus.Message { return &consensus.FetchMissingResponse{} }
	eventCreators[consensus.Type_SYNC_STATE] = func() consensus.Message { return &consensus.SyncState{} }
	eventCreators[consensus.Type_SYNC_STATE_RESPONSE] = func() consensus.Message { return &consensus.SyncStateResponse{} }
	eventCreators[consensus.Type_EPOCH_CHANGE_REQUEST] = func() consensus.Message { return &consensus.EpochChangeRequest{} }
	eventCreators[consensus.Type_EPOCH_CHANGE_PROOF] = func() consensus.Message { return &consensus.EpochChangeProof{} }
	eventCreators[consensus.Type_REBROADCAST_REQUEST_SET] = func() consensus.Message { return &consensus.ReBroadcastRequestSet{} }
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
		return rbft.handleReqTxEvent(e.Event.(*ReqTxMsg[T, Constraint]))
	case ReqNonceEvent:
		return rbft.handleReqNonceEvent(e.Event.(*ReqNonceMsg))
	case ReqPendingTxCountEvent:
		return rbft.handleReqPendingTxCountEvent(e.Event.(*ReqPendingTxCountMsg))
	case ReqGetWatermarkEvent:
		return rbft.handleReqGetWatermarkEvent(e.Event.(*ReqGetWatermarkMsg))
	case ReqGetPoolMetaEvent:
		return rbft.handleReqGetPoolMetaEvent(e.Event.(*ReqGetPoolMetaMsg[T, Constraint]))
	case ReqGetAccountMetaEvent:
		return rbft.handleReqGetAccountMetaEvent(e.Event.(*ReqGetAccountPoolMetaMsg[T, Constraint]))
	case ReqRemoveTxsEvent:
		return rbft.handleReqRemoveTxsEvent(e.Event.(*ReqRemoveTxsMsg[T, Constraint]))
	case NotifyGenBatchEvent:
		return rbft.handleNotifyGenBatchEvent()
	case NotifyFindNextBatchEvent:
		return rbft.handleNotifyFindNextBatchEvent(e.Event.(*NotifyFindNextBatchMsg).hashes)
	default:
		rbft.logger.Errorf("Not Supported event: %v", e)
		return nil
	}
}

func (rbft *rbftImpl[T, Constraint]) handleNotifyFindNextBatchEvent(completionMissingBatchHashes []string) consensusEvent {

	// if current node is in abnormal, add normal txs into txPool without generate batches.
	if !rbft.isNormal() || rbft.in(SkipInProgress) || rbft.in(InRecovery) || rbft.in(inEpochSyncing) || rbft.in(waitCheckpointBatchExecute) {
		var completionMissingBatchIdxs []msgID
		for _, batchHash := range completionMissingBatchHashes {
			idx, ok := rbft.storeMgr.missingBatchesInFetching[batchHash]
			if !ok {
				rbft.logger.Warningf("Replica %d completion batch with hash %s but not found missing record",
					rbft.chainConfig.SelfID, batchHash)
			} else {
				rbft.logger.Infof("Replica %d completion batch with hash %s",
					rbft.chainConfig.SelfID, batchHash)
				completionMissingBatchIdxs = append(completionMissingBatchIdxs, idx)
			}
			delete(rbft.storeMgr.missingBatchesInFetching, batchHash)
		}
		if rbft.in(waitCheckpointBatchExecute) {
			// findNextPrepareBatch after call checkpoint
			rbft.storeMgr.beforeCheckpointEventCache = append(rbft.storeMgr.beforeCheckpointEventCache, &LocalEvent{
				Service:   CoreRbftService,
				EventType: CoreFindNextPrepareBatchesEvent,
				Event:     completionMissingBatchIdxs,
			})
		}
	}

	if rbft.isPrimary(rbft.chainConfig.SelfID) {
		return nil
	}
	for _, batchHash := range completionMissingBatchHashes {
		idx, ok := rbft.storeMgr.missingBatchesInFetching[batchHash]
		if !ok {
			rbft.logger.Warningf("Replica %d completion batch with hash %s but not found missing record",
				rbft.chainConfig.SelfID, batchHash)
		} else {
			rbft.logger.Infof("Replica %d completion batch with hash %s, try to prepare this batch",
				rbft.chainConfig.SelfID, batchHash)

			var ctx context.Context
			if cert, ok := rbft.storeMgr.certStore[idx]; ok {
				ctx = cert.prePrepareCtx
			} else {
				ctx = context.TODO()
			}

			_ = rbft.findNextPrepareBatch(ctx, idx.v, idx.n, idx.d)
		}
		delete(rbft.storeMgr.missingBatchesInFetching, batchHash)
	}
	return nil
}

func (rbft *rbftImpl[T, Constraint]) handleNotifyGenBatchEvent() consensusEvent {
	// if current node is in abnormal, ignore generate batch signal
	if !rbft.isNormal() || rbft.in(SkipInProgress) || rbft.in(InRecovery) || rbft.in(inEpochSyncing) || rbft.in(waitCheckpointBatchExecute) {
		rbft.logger.Warningf("Replica %d is in abnormal, ignore generate batch signal", rbft.chainConfig.SelfID)
		return nil
	}

	if !rbft.isPrimary(rbft.chainConfig.SelfID) {
		rbft.logger.Warningf("Replica %d is not primary, ignore post batch signal", rbft.chainConfig.SelfID)
		return nil
	}
	rbft.stopBatchTimer()
	batch, err := rbft.batchMgr.requestPool.GenerateRequestBatch(txpool.GenBatchSizeEvent)
	if err != nil {
		rbft.logger.Debugf("Replica %d generate batch error: %s", rbft.chainConfig.SelfID, err)
	} else {
		batches := []*txpool.RequestHashBatch[T, Constraint]{batch}
		now := time.Now().UnixNano()
		if rbft.batchMgr.lastBatchTime != 0 {
			interval := time.Duration(now - rbft.batchMgr.lastBatchTime).Seconds()
			rbft.metrics.batchInterval.With("type", "maxSize").Observe(interval)
		}
		rbft.batchMgr.lastBatchTime = now
		rbft.postBatches(batches)
	}

	rbft.restartBatchTimer()
	rbft.restartNoTxBatchTimer()
	return nil
}

func (rbft *rbftImpl[T, Constraint]) handleReqTxEvent(e *ReqTxMsg[T, Constraint]) consensusEvent {
	e.ch <- rbft.batchMgr.requestPool.GetPendingTxByHash(e.hash)
	return nil
}

func (rbft *rbftImpl[T, Constraint]) handleReqNonceEvent(e *ReqNonceMsg) consensusEvent {
	e.ch <- rbft.batchMgr.requestPool.GetPendingTxCountByAccount(e.account)
	return nil
}

func (rbft *rbftImpl[T, Constraint]) handleReqPendingTxCountEvent(e *ReqPendingTxCountMsg) consensusEvent {
	e.ch <- rbft.batchMgr.requestPool.GetTotalPendingTxCount()
	return nil
}

func (rbft *rbftImpl[T, Constraint]) handleReqGetWatermarkEvent(e *ReqGetWatermarkMsg) consensusEvent {
	e.ch <- rbft.chainConfig.H
	return nil
}

func (rbft *rbftImpl[T, Constraint]) handleReqGetPoolMetaEvent(e *ReqGetPoolMetaMsg[T, Constraint]) consensusEvent {
	e.ch <- rbft.batchMgr.requestPool.GetMeta(e.full)
	return nil
}

func (rbft *rbftImpl[T, Constraint]) handleReqGetAccountMetaEvent(e *ReqGetAccountPoolMetaMsg[T, Constraint]) consensusEvent {
	e.ch <- rbft.batchMgr.requestPool.GetAccountMeta(e.account, e.full)
	return nil
}

func (rbft *rbftImpl[T, Constraint]) handleReqRemoveTxsEvent(e *ReqRemoveTxsMsg[T, Constraint]) consensusEvent {
	rbft.batchMgr.requestPool.RemoveStateUpdatingTxs(e.removeTxHashList)
	return nil
}

// handleCoreRbftEvent handles core RBFT service events
func (rbft *rbftImpl[T, Constraint]) handleCoreRbftEvent(e *LocalEvent) consensusEvent {
	switch e.EventType {
	case CoreBatchTimerEvent:
		if !rbft.chainConfig.isValidator() {
			rbft.logger.Debugf("Replica %d is not validator, not process CoreBatchTimerEvent",
				rbft.chainConfig.SelfID)
			return nil
		}

		if !rbft.isNormal() {
			rbft.logger.Debugf("Replica %d is in abnormal, not try to create a batch", rbft.chainConfig.SelfID)
			rbft.stopBatchTimer()
			return nil
		}
		if !rbft.isPrimary(rbft.chainConfig.SelfID) {
			rbft.logger.Debugf("Replica %d is not primary, not try to create a batch", rbft.chainConfig.SelfID)
			rbft.stopBatchTimer()
			return nil
		}
		rbft.logger.Debugf("Primary %d batch timer expired, try to create a batch", rbft.chainConfig.SelfID)

		if rbft.atomicIn(InConfChange) {
			rbft.logger.Debugf("Replica %d is processing a config transaction, cannot generate batches", rbft.chainConfig.SelfID)
			rbft.restartBatchTimer()
			return nil
		}

		if len(rbft.batchMgr.cacheBatch) > 0 {
			rbft.restartBatchTimer()
			rbft.maybeSendPrePrepare(nil, true)
			return nil
		}
		rbft.stopBatchTimer()

		if rbft.batchMgr.requestPool.HasPendingRequestInPool() {
			rbft.stopNoTxBatchTimer()
			// call requestPool module to generate a tx batch
			if rbft.inPrimaryTerm() {
				now := time.Now().UnixNano()
				interval := time.Duration(now - rbft.batchMgr.lastBatchTime).Seconds()

				// this case is quite unusual, with the triggering condition being when the stop timer does not take effect in a timely manner.
				// For Example: if the timeout is set to 500ms, and the stop timer is executed at 499ms
				// the timeout event maybe still triggers.
				// In such cases, the time interval for the timeout batch will be exceedingly short
				// so we need to filter out these cases
				if interval < rbft.config.BatchTimeout.Seconds() {
					rbft.logger.Warningf("Replica %d batch timer expired, but the interval is less than batch timeout, "+
						"so we don't generate a batch, interval: %f", rbft.chainConfig.SelfID, interval)
					return nil
				}
				batch, err := rbft.batchMgr.requestPool.GenerateRequestBatch(txpool.GenBatchTimeoutEvent)
				if err != nil {
					rbft.logger.Warningf("Replica %d failed to generate batch, err: %v", rbft.chainConfig.SelfID, err)
				} else {
					if rbft.batchMgr.lastBatchTime != 0 {
						rbft.metrics.batchInterval.With("type", "timeout").Observe(interval)
						if rbft.batchMgr.minTimeoutBatchTime == 0 || interval < rbft.batchMgr.minTimeoutBatchTime {
							rbft.logger.Debugf("update min timeoutBatch Time[height:%d, interval:%f, lastBatchTime:%v]",
								rbft.batchMgr.getSeqNo()+1, interval, time.Unix(0, rbft.batchMgr.lastBatchTime))
							rbft.metrics.minBatchIntervalDuration.With("type", "timeout").Set(interval)
							rbft.batchMgr.minTimeoutBatchTime = interval
						}
					}
					rbft.batchMgr.lastBatchTime = now
					rbft.postBatches([]*txpool.RequestHashBatch[T, Constraint]{batch})
				}
			}
		}

		// restart batch timer when generate a batch
		rbft.restartBatchTimer()
		// has no pending request, it means no tx match the condition of generate batch, restart no tx batch timer
		if rbft.config.GenesisEpochInfo.ConsensusParams.EnableTimedGenEmptyBlock && !rbft.batchMgr.noTxBatchTimerActive {
			rbft.startNoTxBatchTimer()
		}
		return nil

	case CoreNoTxBatchTimerEvent:
		if !rbft.chainConfig.isValidator() {
			rbft.logger.Debugf("Replica %d is not validator, not process CoreNoTxBatchTimerEvent",
				rbft.chainConfig.SelfID)
			return nil
		}

		if !rbft.isNormal() {
			rbft.logger.Debugf("Replica %d is in abnormal, not try to create a no-tx batch", rbft.chainConfig.SelfID)
			rbft.stopNoTxBatchTimer()
			return nil
		}
		if !rbft.isPrimary(rbft.chainConfig.SelfID) {
			rbft.logger.Debugf("Replica %d is not primary, not try to create a no-tx batch", rbft.chainConfig.SelfID)
			rbft.stopNoTxBatchTimer()
			return nil
		}
		if !rbft.chainConfig.EpochInfo.ConsensusParams.EnableTimedGenEmptyBlock {
			rbft.logger.Debugf("Replica %d is not support generate no-tx batch", rbft.chainConfig.SelfID)
			rbft.stopNoTxBatchTimer()
			return nil
		}

		if rbft.atomicIn(InConfChange) {
			rbft.logger.Debugf("Replica %d is processing a config transaction, cannot generate no-tx batches", rbft.chainConfig.SelfID)
			rbft.restartNoTxBatchTimer()
			return nil
		}

		if len(rbft.batchMgr.cacheBatch) > 0 || rbft.batchMgr.requestPool.HasPendingRequestInPool() {
			rbft.logger.Warningf("txpool is not empty, cannot generate no-tx batches", rbft.chainConfig.SelfID)
			rbft.stopNoTxBatchTimer()
			return nil
		}
		rbft.logger.Debugf("Primary %d no-tx batch timer expired, try to create a no-tx batch", rbft.chainConfig.SelfID)
		rbft.stopNoTxBatchTimer()

		// call requestPool module to generate a tx batch
		if rbft.inPrimaryTerm() {
			batch, err := rbft.batchMgr.requestPool.GenerateRequestBatch(txpool.GenBatchNoTxTimeoutEvent)
			if err != nil {
				rbft.logger.Warningf("Replica %d failed to generate no-tx batch, err: %v", rbft.chainConfig.SelfID, err)
			} else {
				batches := []*txpool.RequestHashBatch[T, Constraint]{batch}
				now := time.Now().UnixNano()
				if rbft.batchMgr.lastBatchTime != 0 {
					interval := time.Duration(now - rbft.batchMgr.lastBatchTime).Seconds()
					rbft.metrics.batchInterval.With("type", "timeout_no_tx").Observe(interval)
					if rbft.batchMgr.minTimeoutNoBatchTime == 0 || interval < rbft.batchMgr.minTimeoutNoBatchTime {
						rbft.metrics.minBatchIntervalDuration.With("type", "timeout_no_tx").Set(interval)
						rbft.batchMgr.minTimeoutNoBatchTime = interval
					}
				}
				rbft.batchMgr.lastBatchTime = now
				rbft.postBatches(batches)
			}
			rbft.restartNoTxBatchTimer()
		}

		return nil

	case CoreNullRequestTimerEvent:
		rbft.handleNullRequestTimerEvent()
		return nil

	case CoreCheckPoolTimerEvent:
		rbft.stopCheckPoolTimer()
		rbft.processOutOfDateReqs(true)
		rbft.restartCheckPoolTimer()
		return nil

	case CoreRebroadcastTxsEvent:
		rbft.stopCheckPoolTimer()
		rbft.processOutOfDateReqs(false)
		rbft.restartCheckPoolTimer()
		return nil

	case CoreStateUpdatedEvent:
		return rbft.recvStateUpdatedEvent(e.Event.(*types.ServiceSyncState))

	case CoreCheckpointBlockExecutedEvent:
		rbft.recvCheckpointBlockExecutedEvent(e.Event.(*types.ServiceState))
		return nil

	case CoreFindNextPrepareBatchesEvent:
		ctx := context.Background()
		completionMissingBatchIdxs := e.Event.([]msgID)
		for _, idx := range completionMissingBatchIdxs {
			rbft.logger.Infof("Replica %d completion batch with hash %s when wait checkpoint, try to prepare this batch",
				rbft.chainConfig.SelfID, idx.d)
			_ = rbft.findNextPrepareBatch(ctx, idx.v, idx.n, idx.d)
		}
		return nil

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
		if preLowWatermark < rbft.chainConfig.H {
			rbft.logger.Debugf("Replica %d has already updated low-watermark to %d, just return", rbft.chainConfig.SelfID, rbft.chainConfig.H)
			return nil
		}

		rbft.logger.Infof("Replica %d high-watermark timer expired, send view change: %s",
			rbft.chainConfig.SelfID, rbft.highWatermarkTimerReason)
		return rbft.sendViewChange()

	//case CoreCheckPoolRemoveTimerEvent:
	//	rbft.processNeedRemoveReqs()
	//	rbft.restartCheckPoolRemoveTimer()
	//	return nil

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
			rbft.logger.Errorf("Replica %d parsing error, rbft.chainConfig.View", rbft.chainConfig.SelfID)
			rbft.stopNamespace()
			return nil
		}

		if preView < rbft.chainConfig.View {
			rbft.logger.Debugf("Replica %d has initiated recovery, ignore the event", rbft.chainConfig.SelfID)
			return nil
		}

		rbft.logger.Debugf("Replica %d init recovery", rbft.chainConfig.SelfID)
		return rbft.initRecovery()

	case RecoverySyncStateRspTimerEvent:
		rbft.logger.Noticef("Replica %d sync state response timer expired", rbft.chainConfig.SelfID)
		rbft.exitSyncState()
		if !rbft.isNormal() {
			rbft.logger.Noticef("Replica %d is in abnormal, not try view change", rbft.chainConfig.SelfID)
			return nil
		}
		return rbft.initRecovery()
	case RecoverySyncStateRestartTimerEvent:
		rbft.logger.Debugf("Replica %d sync state restart timer expired", rbft.chainConfig.SelfID)
		rbft.exitSyncState()
		if !rbft.isNormal() {
			rbft.logger.Debugf("Replica %d is in abnormal, not restart sync state", rbft.chainConfig.SelfID)
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
			if rbft.chainConfig.View > uint64(demand) {
				rbft.logger.Debugf("Replica %d received viewChangeTimerEvent, but we"+
					"have sent the next viewChange maybe just before a moment.", rbft.chainConfig.SelfID)
				return nil
			}
		} else if rbft.atomicIn(InViewChange) {
			rbft.logger.Debugf("Replica %d received viewChangeTimerEvent, but we"+
				"are already in view-change and it has not reached quorum.", rbft.chainConfig.SelfID)
			return nil
		}

		rbft.logger.Infof("Replica %d viewChange timer expired, sending viewChange: %s", rbft.chainConfig.SelfID, rbft.vcMgr.newViewTimerReason)

		// Here, we directly send viewchange with a bigger target view (which is rbft.chainConfig.View+1) because it is the
		// new view timer who triggered this ViewChangeTimerEvent so we send a new viewchange request
		return rbft.sendViewChange()

	case ViewChangeDoneEvent:
		// set a viewChangeSeqNo if needed
		rbft.updateViewChangeSeqNo(rbft.exec.lastExec, rbft.chainConfig.EpochInfo.ConsensusParams.CheckpointPeriod)
		delete(rbft.vcMgr.newViewStore, rbft.chainConfig.View)
		rbft.storeMgr.missingBatchesInFetching = make(map[string]msgID)

		rbft.stopNewViewTimer()
		rbft.stopFetchViewTimer()
		rbft.startTimerIfOutstandingRequests()

		// set normal to 1 which indicates system comes into normal status after viewchange
		rbft.atomicOff(InViewChange)
		rbft.metrics.statusGaugeInViewChange.Set(0)
		var finishMsg string
		if rbft.atomicIn(InRecovery) {
			rbft.atomicOff(InRecovery)
			rbft.metrics.statusGaugeInRecovery.Set(0)
			rbft.logger.Noticef("======== Replica %d finished recovery, primary=%d, "+
				"epoch=%d/n=%d/f=%d/view=%d/h=%d/lastExec=%d/lastCkpDigest=%s", rbft.chainConfig.SelfID, rbft.chainConfig.PrimaryID,
				rbft.chainConfig.EpochInfo.Epoch, rbft.chainConfig.N, rbft.chainConfig.F, rbft.chainConfig.View, rbft.chainConfig.H, rbft.exec.lastExec, rbft.chainConfig.LastCheckpointExecBlockHash)

			rbft.logger.Notice(`

  +==============================================+
  |                                              |
  |            RBFT Recovery Finished            |
  |                                              |
  +==============================================+

`)
			finishMsg = fmt.Sprintf("======== Replica %d finished recovery, primary=%d, "+
				"epoch=%d/n=%d/f=%d/view=%d/h=%d/lastExec=%d", rbft.chainConfig.SelfID, rbft.chainConfig.PrimaryID,
				rbft.chainConfig.EpochInfo.Epoch, rbft.chainConfig.N, rbft.chainConfig.F, rbft.chainConfig.View, rbft.chainConfig.H, rbft.exec.lastExec)
			rbft.external.SendFilterEvent(types.InformTypeFilterFinishRecovery, finishMsg)

			// this means that after the block is executed, the node is terminated before the checkpoint logic
			if rbft.chainConfig.H < rbft.config.LastServiceState.MetaState.Height {
				//	try checkpoint
				go rbft.reportCheckpoint(rbft.config.LastServiceState)
			}
		} else {
			rbft.logger.Noticef("======== Replica %d finished viewChange, primary=%d, "+
				"epoch=%d/n=%d/f=%d/view=%d/h=%d/lastExec=%d", rbft.chainConfig.SelfID, rbft.chainConfig.PrimaryID,
				rbft.chainConfig.EpochInfo.Epoch, rbft.chainConfig.N, rbft.chainConfig.F, rbft.chainConfig.View, rbft.chainConfig.H, rbft.exec.lastExec)

			finishMsg = fmt.Sprintf("======== Replica %d finished viewChange, primary=%d, "+
				"epoch=%d/n=%d/f=%d/view=%d/h=%d/lastExec=%d", rbft.chainConfig.SelfID, rbft.chainConfig.PrimaryID,
				rbft.chainConfig.EpochInfo.Epoch, rbft.chainConfig.N, rbft.chainConfig.F, rbft.chainConfig.View, rbft.chainConfig.H, rbft.exec.lastExec)
			rbft.external.SendFilterEvent(types.InformTypeFilterFinishViewChange, finishMsg)
		}

		// if node doesn't start check pool timer, start it
		if !rbft.isActiveCheckPoolTimer() {
			rbft.startCheckPoolTimer()
		}
		rbft.maybeSetNormal()
		rbft.logger.Trace(consensus.TagNameViewChange, consensus.TagStageFinish, consensus.TagContentViewChange{
			Node: rbft.chainConfig.SelfID,
			View: rbft.chainConfig.View,
		})

		// clear storage from lower view
		for idx := range rbft.vcMgr.viewChangeStore {
			if idx.v <= rbft.chainConfig.View {
				delete(rbft.vcMgr.viewChangeStore, idx)
			}
		}

		for view := range rbft.vcMgr.newViewStore {
			if view < rbft.chainConfig.View {
				delete(rbft.vcMgr.newViewStore, view)
			}
		}

		for idx := range rbft.vcMgr.newViewCache {
			if idx.targetView <= rbft.chainConfig.View {
				delete(rbft.vcMgr.newViewCache, idx)
			}
		}

		// check if epoch has been changed, if changed, trigger another round of view change
		// after epoch change to find correct view-number
		epochChanged := rbft.syncEpoch()
		if epochChanged {
			rbft.logger.Debugf("Replica %d sending view change again because of epoch change", rbft.chainConfig.SelfID)
			// trigger another round of view change after epoch change to find correct view-number
			return rbft.initRecovery()
		}

		if rbft.isPrimary(rbft.chainConfig.SelfID) {
			rbft.primaryResubmitTransactions()
			// restart all batch timer when primary node finished recovery
			rbft.restartBatchTimer()
			rbft.restartNoTxBatchTimer()
		} else {
			// here, we always fetch PQC after finish recovery as we only recovery to the largest checkpoint which
			// is lower or equal to the lastExec quorum of others.
			rbft.fetchRecoveryPQC()
		}

	case ViewChangeResendTimerEvent:
		if !rbft.atomicIn(InViewChange) {
			rbft.logger.Warningf("Replica %d had its viewChange resend timer expired but it's "+
				"not in viewChange, ignore it", rbft.chainConfig.SelfID)
			return nil
		}
		rbft.logger.Infof("Replica %d viewChange resend timer expired before viewChange quorum "+
			"was reached, try recovery", rbft.chainConfig.SelfID)

		// if viewChange resend timer expired, directly enter recovery status,
		// decrease view first as we may increase view when send view change.
		newView := rbft.chainConfig.View - uint64(1)
		rbft.setView(newView)

		return rbft.initRecovery()

	case ViewChangeQuorumEvent:
		if rbft.isPrimary(rbft.chainConfig.SelfID) {
			rbft.logger.Infof("Replica %d received viewChange quorum, primary send new view", rbft.chainConfig.SelfID)
			// if we are catching up, don't send new view as a primary and after a while, other nodes will
			// send a new viewchange whose seqNo=previous viewchange's seqNo + 1 because of new view timeout
			// and eventually others will finish viewchange with a new view in which primary is not in
			// skipInProgress
			if rbft.in(SkipInProgress) {
				rbft.logger.Infof("Replica %d is in catching up, don't send new view", rbft.chainConfig.SelfID)
				return nil
			}
			// primary construct and send new view message
			return rbft.sendNewView()
		}
		rbft.logger.Infof("Replica %d received viewChange quorum, replica check newView", rbft.chainConfig.SelfID)
		return rbft.replicaCheckNewView()

	case FetchViewEvent:
		rbft.logger.Debugf("Replica %d fetch view timer expired", rbft.chainConfig.SelfID)
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
		rbft.logger.Debugf("Replica %d fetch checkpoint timer expired", rbft.chainConfig.SelfID)
		rbft.stopFetchCheckpointTimer()
		rbft.fetchCheckpoint()

	case EpochSyncEvent:
		proof := e.Event.(*consensus.EpochChangeProof)
		quorumCheckpoint := proof.Last().Checkpoint
		for _, ec := range proof.GetEpochChanges() {
			// TODO: support restore
			rbft.epochMgr.epochProofCache[ec.Checkpoint.Epoch()] = ec
		}
		rbft.logger.Noticef("Replica %d try epoch sync to height %d, epoch %d", rbft.chainConfig.SelfID,
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

		rbft.updateHighStateTarget(target, checkpointSet, proof.GetEpochChanges()...) // for new epoch
		rbft.tryStateTransfer()

	default:
		rbft.logger.Errorf("Invalid viewChange quorumCheckpoint: %v", e)
		return nil
	}
	return nil
}

// dispatchConsensusMsg dispatches consensus messages to corresponding handlers using its service type
func (rbft *rbftImpl[T, Constraint]) dispatchConsensusMsg(ctx context.Context, originEvent consensusEvent, e consensusEvent) consensusEvent {
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
	case *consensus.ReBroadcastRequestSet:
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
