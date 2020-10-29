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
	"fmt"

	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/gogo/protobuf/proto"
)

// executor manages exec related params
type executor struct {
	lastExec    uint64
	currentExec *uint64
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

// setCurrentExec sets the value of pointer currentExec
func (e *executor) setCurrentExec(c *uint64) {
	e.currentExec = c
}

// msgToEvent converts ConsensusMessage to the corresponding consensus event.
func (rbft *rbftImpl) msgToEvent(msg *pb.ConsensusMessage) (interface{}, error) {
	event := eventCreators[msg.Type]().(proto.Message)
	err := proto.Unmarshal(msg.Payload, event)
	if err != nil {
		rbft.logger.Errorf("Unmarshal error, can not unmarshal %v, error: %v", msg.Type, err)
		localEvent := &LocalEvent{
			Service:   CoreRbftService,
			EventType: CoreResendFetchMissingEvent,
		}
		go rbft.postMsg(localEvent)

		return nil, err
	}

	return event, nil
}

var eventCreators map[pb.Type]func() interface{}

// initMsgEventMap maps consensus_message to real consensus msg type which used to Unmarshal consensus_message's payload
// to actual consensus msg
func initMsgEventMap() {
	eventCreators = make(map[pb.Type]func() interface{})

	eventCreators[pb.Type_NULL_REQUEST] = func() interface{} { return &pb.NullRequest{} }
	eventCreators[pb.Type_PRE_PREPARE] = func() interface{} { return &pb.PrePrepare{} }
	eventCreators[pb.Type_PREPARE] = func() interface{} { return &pb.Prepare{} }
	eventCreators[pb.Type_COMMIT] = func() interface{} { return &pb.Commit{} }
	eventCreators[pb.Type_CHECKPOINT] = func() interface{} { return &pb.Checkpoint{} }
	eventCreators[pb.Type_FETCH_CHECKPOINT] = func() interface{} { return &pb.FetchCheckpoint{} }
	eventCreators[pb.Type_VIEW_CHANGE] = func() interface{} { return &pb.ViewChange{} }
	eventCreators[pb.Type_NEW_VIEW] = func() interface{} { return &pb.NewView{} }
	eventCreators[pb.Type_FETCH_REQUEST_BATCH] = func() interface{} { return &pb.FetchRequestBatch{} }
	eventCreators[pb.Type_SEND_REQUEST_BATCH] = func() interface{} { return &pb.SendRequestBatch{} }
	eventCreators[pb.Type_RECOVERY_FETCH_QPC] = func() interface{} { return &pb.RecoveryFetchPQC{} }
	eventCreators[pb.Type_RECOVERY_RETURN_QPC] = func() interface{} { return &pb.RecoveryReturnPQC{} }
	eventCreators[pb.Type_FETCH_MISSING_REQUESTS] = func() interface{} { return &pb.FetchMissingRequests{} }
	eventCreators[pb.Type_SEND_MISSING_REQUESTS] = func() interface{} { return &pb.SendMissingRequests{} }
	eventCreators[pb.Type_SYNC_STATE] = func() interface{} { return &pb.SyncState{} }
	eventCreators[pb.Type_SYNC_STATE_RESPONSE] = func() interface{} { return &pb.SyncStateResponse{} }
	eventCreators[pb.Type_NOTIFICATION] = func() interface{} { return &pb.Notification{} }
	eventCreators[pb.Type_NOTIFICATION_RESPONSE] = func() interface{} { return &pb.NotificationResponse{} }
}

// dispatchLocalEvent dispatches local Event to corresponding handles using its service type
func (rbft *rbftImpl) dispatchLocalEvent(e *LocalEvent) consensusEvent {
	switch e.Service {
	case CoreRbftService:
		return rbft.handleCoreRbftEvent(e)
	case ViewChangeService:
		return rbft.handleViewChangeEvent(e)
	case RecoveryService:
		return rbft.handleRecoveryEvent(e)
	default:
		rbft.logger.Errorf("Not Supported event: %v", e)
		return nil
	}
}

// handleCoreRbftEvent handles core RBFT service events
func (rbft *rbftImpl) handleCoreRbftEvent(e *LocalEvent) consensusEvent {
	switch e.EventType {
	case CoreBatchTimerEvent:
		if !rbft.isNormal() {
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

	case CoreNullRequestTimerEvent:
		rbft.handleNullRequestTimerEvent()
		return nil

	case CoreFirstRequestTimerEvent:
		rbft.logger.Warningf("Replica %d first request timer expired", rbft.peerPool.ID)
		if rbft.atomicInOne(InViewChange, InRecovery) {
			rbft.logger.Debugf("Replica %d has already been in view-change or recovery, return directly", rbft.peerPool.ID)
			return nil
		}
		return rbft.sendViewChange()

	case CoreCheckPoolTimerEvent:
		if !rbft.isNormal() {
			return nil
		}
		rbft.processOutOfDateReqs()
		rbft.restartCheckPoolTimer()
		return nil

	case CoreStateUpdatedEvent:
		return rbft.recvStateUpdatedEvent(e.Event.(*pb.ServiceState))

	case CoreResendMissingTxsEvent:
		ev := e.Event.(*pb.FetchMissingRequests)
		_ = rbft.recvFetchMissingTxs(ev)
		return nil

	case CoreResendFetchMissingEvent:
		rbft.executeAfterStateUpdate()
		return nil

	default:
		rbft.logger.Errorf("Invalid core RBFT event: %v", e)
		return nil
	}
}

// handleRecoveryEvent handles recovery services related events.
func (rbft *rbftImpl) handleRecoveryEvent(e *LocalEvent) consensusEvent {
	switch e.EventType {
	case RecoveryInitEvent:
		rbft.logger.Debugf("Replica %d init recovery", rbft.peerPool.ID)
		return rbft.initRecovery()
	case RecoveryDoneEvent:
		if rbft.in(isNewNode) {
			rbft.off(isNewNode)
		}
		rbft.atomicOff(InRecovery)
		rbft.metrics.statusGaugeInRecovery.Set(0)
		rbft.recoveryMgr.recoveryHandled = false
		rbft.recoveryMgr.notificationStore = make(map[ntfIdx]*pb.Notification)
		rbft.recoveryMgr.outOfElection = make(map[ntfIdx]*pb.NotificationResponse)
		rbft.recoveryMgr.differentEpoch = make(map[ntfIde]*pb.NotificationResponse)
		rbft.vcMgr.viewChangeStore = make(map[vcIdx]*pb.ViewChange)
		rbft.maybeSetNormal()
		rbft.timerMgr.stopTimer(recoveryRestartTimer)
		rbft.logger.Noticef("======== Replica %d finished recovery, epoch=%d/view=%d/height=%d.", rbft.peerPool.ID, rbft.epoch, rbft.view, rbft.exec.lastExec)
		rbft.logger.Notice(`

  +==============================================+
  |                                              |
  |            RBFT Recovery Finished            |
  |                                              |
  +==============================================+

`)
		finishMsg := fmt.Sprintf("======== Replica %d finished recovery, primary=%d, epoch=%d/n=%d/f=%d/view=%d/h=%d/lastExec=%d", rbft.peerPool.ID, rbft.primaryID(rbft.view), rbft.epoch, rbft.N, rbft.f, rbft.view, rbft.h, rbft.exec.lastExec)
		rbft.external.SendFilterEvent(pb.InformType_FilterFinishRecovery, finishMsg)
		// after recovery, new primary need to send null request as a heartbeat, and non-primary will start a
		// first request timer which must be longer than null request timer in which non-primary must receive a
		// request from primary(null request or pre-prepare...), or this node will send viewChange.
		// However, primary may have resend some batch during primaryResendBatch, so, for primary need
		// only startTimerIfOutstandingRequests.
		if rbft.isPrimary(rbft.peerPool.ID) {
			rbft.startTimerIfOutstandingRequests()
		} else {
			event := &LocalEvent{
				Service:   CoreRbftService,
				EventType: CoreFirstRequestTimerEvent,
			}

			rbft.timerMgr.startTimer(firstRequestTimer, event)
		}

		// here, we always fetch PQC after finish recovery as we only recovery to the largest checkpoint which
		// is lower or equal to the lastExec quorum of others, which, in this way, we avoid sending prepare and
		// commit or other consensus messages during add/delete node
		rbft.fetchRecoveryPQC()

		rbft.primaryResubmitTransactions()

		return nil

	case RecoveryRestartTimerEvent:
		rbft.logger.Debugf("Replica %d recovery restart timer expired, start epoch sync", rbft.peerPool.ID)
		return rbft.restartRecovery()
	case RecoverySyncStateRspTimerEvent:
		rbft.logger.Noticef("Replica %d sync state response timer expired", rbft.peerPool.ID)
		if !rbft.isNormal() {
			rbft.logger.Noticef("Replica %d is in abnormal, not restart recovery", rbft.peerPool.ID)
			return nil
		}
		rbft.off(InSyncState)
		rbft.exitSyncState()
		rbft.initRecovery()
		return nil
	case RecoverySyncStateRestartTimerEvent:
		rbft.logger.Debugf("Replica %d sync state restart timer expired", rbft.peerPool.ID)
		rbft.exitSyncState()
		rbft.restartSyncState()
		return nil
	case NotificationQuorumEvent:
		rbft.logger.Infof("Replica %d received notification quorum, processing new view", rbft.peerPool.ID)
		if rbft.isPrimary(rbft.peerPool.ID) {
			if rbft.in(SkipInProgress) {
				rbft.logger.Debugf("Replica %d is in catching up, don't send new view", rbft.peerPool.ID)
				return nil
			}
			// primary construct and send new view message
			return rbft.sendNewView(true)
		}
		return rbft.replicaCheckNewView()
	default:
		rbft.logger.Errorf("Invalid recovery service events : %v", e)
		return nil
	}
}

// handleViewChangeEvent handles view change service related events.
func (rbft *rbftImpl) handleViewChangeEvent(e *LocalEvent) consensusEvent {
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

		if rbft.atomicIn(InRecovery) {
			rbft.logger.Debugf("Replica %d restart recovery after new view timer expired in recovery", rbft.peerPool.ID)
			return rbft.initRecovery()
		}

		// Here, we directly send viewchange with a bigger target view (which is rbft.view+1) because it is the
		// new view timer who triggered this ViewChangeTimerEvent so we send a new viewchange request
		return rbft.sendViewChange()

	case ViewChangedEvent:
		// set a viewChangeSeqNo if needed
		rbft.updateViewChangeSeqNo(rbft.exec.lastExec, rbft.K)
		delete(rbft.vcMgr.newViewStore, rbft.view)
		rbft.vcMgr.vcHandled = false

		rbft.startTimerIfOutstandingRequests()
		rbft.storeMgr.missingReqBatches = make(map[string]bool)
		primaryID := rbft.primaryID(rbft.view)

		// set normal to 1 which indicates system comes into normal status after viewchange
		rbft.atomicOff(InViewChange)
		rbft.metrics.statusGaugeInViewChange.Set(0)
		rbft.maybeSetNormal()
		rbft.logger.Noticef("======== Replica %d finished viewChange, primary=%d, view=%d/height=%d", rbft.peerPool.ID, primaryID, rbft.view, rbft.exec.lastExec)
		finishMsg := fmt.Sprintf("Replica %d finished viewChange, primary=%d, view=%d/height=%d", rbft.peerPool.ID, primaryID, rbft.view, rbft.exec.lastExec)

		// clear storage from lower view
		for idx := range rbft.recoveryMgr.notificationStore {
			if idx.v < rbft.view {
				delete(rbft.recoveryMgr.notificationStore, idx)
			}
		}
		for idx := range rbft.vcMgr.viewChangeStore {
			if idx.v < rbft.view {
				delete(rbft.vcMgr.viewChangeStore, idx)
			}
		}

		// send viewchange result to web socket API
		rbft.external.SendFilterEvent(pb.InformType_FilterFinishViewChange, finishMsg)

		rbft.primaryResubmitTransactions()

	case ViewChangeResendTimerEvent:
		if !rbft.atomicIn(InViewChange) {
			rbft.logger.Warningf("Replica %d had its viewChange resend timer expired but it's not in viewChange,ignore it", rbft.peerPool.ID)
			return nil
		}
		rbft.logger.Infof("Replica %d viewChange resend timer expired before viewChange quorum was reached, try send notification", rbft.peerPool.ID)

		rbft.timerMgr.stopTimer(newViewTimer)
		rbft.atomicOff(InViewChange)
		rbft.metrics.statusGaugeInViewChange.Set(0)

		// if viewChange resend timer expired, directly enter recovery status,
		// decrease view first as we may increase view when send notification.
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
			return rbft.sendNewView(false)
		}
		return rbft.replicaCheckNewView()

	default:
		rbft.logger.Errorf("Invalid viewChange event: %v", e)
		return nil
	}
	return nil
}

// dispatchConsensusMsg dispatches consensus messages to corresponding handlers using its service type
func (rbft *rbftImpl) dispatchConsensusMsg(e consensusEvent) consensusEvent {
	service := rbft.dispatchMsgToService(e)
	switch service {
	case CoreRbftService:
		return rbft.dispatchCoreRbftMsg(e)
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
func (rbft *rbftImpl) dispatchMsgToService(e consensusEvent) int {
	switch e.(type) {
	// core RBFT service
	case *pb.NullRequest:
		return CoreRbftService
	case *pb.PrePrepare:
		return CoreRbftService
	case *pb.Prepare:
		return CoreRbftService
	case *pb.Commit:
		return CoreRbftService
	case *pb.Checkpoint:
		return CoreRbftService
	case *pb.FetchMissingRequests:
		return CoreRbftService
	case *pb.SendMissingRequests:
		return CoreRbftService

		// view change service
	case *pb.ViewChange:
		return ViewChangeService
	case *pb.NewView:
		return ViewChangeService
	case *pb.FetchRequestBatch:
		return ViewChangeService
	case *pb.SendRequestBatch:
		return ViewChangeService

		// recovery service
	case *pb.RecoveryFetchPQC:
		return RecoveryService
	case *pb.RecoveryReturnPQC:
		return RecoveryService
	case *pb.SyncState:
		return RecoveryService
	case *pb.SyncStateResponse:
		return RecoveryService
	case *pb.Notification:
		return RecoveryService
	case *pb.NotificationResponse:
		return RecoveryService

	case *pb.FetchCheckpoint:
		return EpochMgrService
	default:
		return NotSupportService

	}
}
