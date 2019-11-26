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
		rbft.postMsg(localEvent)

		return nil, err
	}

	return event, nil
}

var eventCreators map[pb.Type]func() interface{}

// initMsgEventMap maps consensus_message to real consensus msg type which used to Unmarshal consensus_message's payload
// to actual consensus msg
func (rbft *rbftImpl) initMsgEventMap() {
	eventCreators = make(map[pb.Type]func() interface{})

	eventCreators[pb.Type_NULL_REQUEST] = func() interface{} { return &pb.NullRequest{} }
	eventCreators[pb.Type_PRE_PREPARE] = func() interface{} { return &pb.PrePrepare{} }
	eventCreators[pb.Type_PREPARE] = func() interface{} { return &pb.Prepare{} }
	eventCreators[pb.Type_COMMIT] = func() interface{} { return &pb.Commit{} }
	eventCreators[pb.Type_CHECKPOINT] = func() interface{} { return &pb.Checkpoint{} }
	eventCreators[pb.Type_VIEW_CHANGE] = func() interface{} { return &pb.ViewChange{} }
	eventCreators[pb.Type_NEW_VIEW] = func() interface{} { return &pb.NewView{} }
	eventCreators[pb.Type_FETCH_REQUEST_BATCH] = func() interface{} { return &pb.FetchRequestBatch{} }
	eventCreators[pb.Type_SEND_REQUEST_BATCH] = func() interface{} { return &pb.SendRequestBatch{} }
	eventCreators[pb.Type_RECOVERY_FETCH_QPC] = func() interface{} { return &pb.RecoveryFetchPQC{} }
	eventCreators[pb.Type_RECOVERY_RETURN_QPC] = func() interface{} { return &pb.RecoveryReturnPQC{} }
	eventCreators[pb.Type_READY_FOR_N] = func() interface{} { return &pb.ReadyForN{} }
	eventCreators[pb.Type_AGREE_UPDATE_N] = func() interface{} { return &pb.AgreeUpdateN{} }
	eventCreators[pb.Type_UPDATE_N] = func() interface{} { return &pb.UpdateN{} }
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
	case NodeMgrService:
		return rbft.handleNodeMgrEvent(e)
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
		rbft.logger.Debugf("Primary %d batch timer expired, try to create a batch", rbft.no)
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
		rbft.logger.Warningf("Replica %d first request timer expired", rbft.no)
		return rbft.sendViewChange()

	case CoreCheckPoolTimerEvent:
		if !rbft.isNormal() {
			return nil
		}
		rbft.processOutOfDateReqs()
		rbft.restartCheckPoolTimer()
		return nil

	case CoreStateUpdatedEvent:
		return rbft.recvStateUpdatedEvent(e.Event.(uint64))

	case CoreResendMissingTxsEvent:
		ev := e.Event.(*pb.FetchMissingRequests)
		_ = rbft.recvFetchMissingTxs(ev)
		return nil

	case CoreResendFetchMissingEvent:
		rbft.executeAfterStateUpdate()
		return nil

	case CoreRetrieveStatusEvent:
		statusChan := e.Event.(chan NodeStatus)
		currentStatus := rbft.getStatus()
		statusChan <- currentStatus
		return nil
	case CoreUpdateConfStateEvent:
		ev := e.Event.(*pb.ConfState)
		found := false
		for i, p := range ev.QuorumRouter.Peers {
			if p.Id == rbft.peerPool.localID {
				rbft.no = uint64(i + 1)
				found = true
			}
		}
		if !found {
			rbft.logger.Criticalf("Replica %d cannot find self id in quorum routers: %+v", rbft.no, ev.QuorumRouter.Peers)
			rbft.on(Pending)
			return nil
		}
		rbft.peerPool.initPeers(ev.QuorumRouter.Peers)
		return nil

	default:
		rbft.logger.Errorf("Invalid core RBFT event: %v", e)
		return nil
	}
}

// handleRecoveryEvent handles recovery services related events.
func (rbft *rbftImpl) handleRecoveryEvent(e *LocalEvent) consensusEvent {
	switch e.EventType {
	case RecoveryDoneEvent:
		rbft.off(InRecovery)
		rbft.recoveryMgr.recoveryHandled = false
		rbft.recoveryMgr.notificationStore = make(map[ntfIdx]*pb.Notification)
		rbft.recoveryMgr.outOfElection = make(map[ntfIdx]*pb.NotificationResponse)
		rbft.maybeSetNormal()
		rbft.timerMgr.stopTimer(recoveryRestartTimer)
		rbft.logger.Noticef("======== Replica %d finished recovery, view=%d/height=%d.", rbft.no, rbft.view, rbft.exec.lastExec)
		rbft.logger.Notice(`

  +==============================================+
  |                                              |
  |            RBFT Recovery Finished            |
  |                                              |
  +==============================================+

`)

		// after recovery, new primary need to send null request as a heartbeat, and non-primary will start a
		// first request timer which must be longer than null request timer in which non-primary must receive a
		// request from primary(null request or pre-prepare...), or this node will send viewChange.
		// However, primary may have resend some batch during primaryResendBatch, so, for primary need
		// only startTimerIfOutstandingRequests.
		if rbft.isPrimary(rbft.peerPool.localID) {
			rbft.startTimerIfOutstandingRequests()
		} else {
			event := &LocalEvent{
				Service:   CoreRbftService,
				EventType: CoreFirstRequestTimerEvent,
			}

			rbft.timerMgr.startTimer(firstRequestTimer, event)
		}

		if rbft.in(isNewNode) {
			rbft.sendReadyForN()
			return nil
		}

		// here, we always fetch PQC after finish recovery as we only recovery to the largest checkpoint which
		// is lower or equal to the lastExec quorum of others, which, in this way, we avoid sending prepare and
		// commit or other consensus messages during add/delete node
		rbft.fetchRecoveryPQC()

		rbft.primaryResubmitTransactions()

		return nil

	case RecoveryRestartTimerEvent:
		rbft.logger.Debugf("Replica %d recovery restart timer expired", rbft.no)
		return rbft.restartRecovery()
	case RecoverySyncStateRspTimerEvent:
		rbft.logger.Noticef("Replica %d sync state response timer expired", rbft.no)
		if !rbft.isNormal() {
			rbft.logger.Noticef("Replica %d is in abnormal, not restart recovery", rbft.no)
			return nil
		}
		rbft.off(InSyncState)
		rbft.exitSyncState()
		rbft.restartRecovery()
		return nil
	case RecoverySyncStateRestartTimerEvent:
		rbft.logger.Debugf("Replica %d sync state restart timer expired", rbft.no)
		rbft.restartSyncState()
		return nil
	case NotificationQuorumEvent:
		rbft.logger.Infof("Replica %d received notification quorum, processing new view", rbft.no)
		if rbft.isPrimary(rbft.peerPool.localID) {
			if rbft.in(SkipInProgress) {
				rbft.logger.Debugf("Replica %d is in catching up, don't send new view", rbft.no)
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
		if ok && rbft.view > uint64(demand) {
			rbft.logger.Debugf("Replica %d received viewChangeTimerEvent, but we"+
				"have sent the next viewChange maybe just before a moment.", rbft.no)
			return nil
		}

		rbft.logger.Infof("Replica %d viewChange timer expired, sending viewChange: %s", rbft.no, rbft.vcMgr.newViewTimerReason)

		if rbft.in(InRecovery) {
			rbft.logger.Debugf("Replica %d restart recovery after new view timer expired in recovery", rbft.no)
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
		primary := rbft.primaryIndex(rbft.view)

		// set normal to 1 which indicates system comes into normal status after viewchange
		rbft.off(InViewChange)
		rbft.maybeSetNormal()
		rbft.logger.Noticef("======== Replica %d finished viewChange, primary=%d, view=%d/height=%d", rbft.no, primary, rbft.view, rbft.exec.lastExec)
		finishMsg := fmt.Sprintf("Replica %d finished viewChange, primary=%d, view=%d/height=%d", rbft.no, primary, rbft.view, rbft.exec.lastExec)

		// send viewchange result to web socket API
		rbft.external.SendFilterEvent(pb.InformType_FilterFinishViewChange, finishMsg)
		if rbft.in(isNewNode) {
			rbft.sendReadyForN()
			return nil
		}

		rbft.primaryResubmitTransactions()

	case ViewChangeResendTimerEvent:
		if !rbft.in(InViewChange) {
			rbft.logger.Warningf("Replica %d had its viewChange resend timer expired but it's not in viewChange,ignore it", rbft.no)
			return nil
		}
		rbft.logger.Infof("Replica %d viewChange resend timer expired before viewChange quorum was reached, try send notification", rbft.no)

		rbft.timerMgr.stopTimer(newViewTimer)
		rbft.off(InViewChange)

		// if viewChange resend timer expired, directly enter recovery status,
		// decrease view first as we may increase view when send notification.
		rbft.view--

		return rbft.initRecovery()

	case ViewChangeQuorumEvent:
		rbft.logger.Infof("Replica %d received viewChange quorum, processing new view", rbft.no)
		if rbft.isPrimary(rbft.peerPool.localID) {
			// if we are catching up, don't send new view as a primary and after a while, other nodes will
			// send a new viewchange whose seqNo=previous viewchange's seqNo + 1 because of new view timeout
			// and eventually others will finish viewchange with a new view in which primary is not in
			// skipInProgress
			if rbft.in(SkipInProgress) {
				rbft.logger.Debugf("Replica %d is in catching up, don't send new view", rbft.no)
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

// handleNodeMgrEvent handles node management service related events.
func (rbft *rbftImpl) handleNodeMgrEvent(e *LocalEvent) consensusEvent {
	switch e.EventType {
	case NodeMgrDelNodeEvent:
		return rbft.recvLocalDelNode(e.Event.(uint64))
	case NodeMgrAgreeUpdateQuorumEvent:
		rbft.logger.Debugf("Replica %d received agreeUpdateN quorum, processing updateN", rbft.no)
		if rbft.isPrimary(rbft.peerPool.localID) {
			return rbft.sendUpdateN()
		}
		return rbft.replicaCheckUpdateN()
	case NodeMgrUpdatedEvent:
		rbft.startTimerIfOutstandingRequests()
		rbft.storeMgr.missingReqBatches = make(map[string]bool)
		rbft.nodeMgr.updateHandled = false
		if rbft.in(isNewNode) {
			rbft.off(isNewNode)
			// Fetch PQC in case that new node updated slowly
			rbft.fetchRecoveryPQC()
		}

		rbft.off(InUpdatingN)
		rbft.stopUpdateTimer()
		rbft.maybeSetNormal()

		if rbft.no == 0 {
			rbft.logger.Noticef("======== New Replica finished updateN, primary=%d, n=%d/f=%d/view=%d/h=%d", rbft.primaryIndex(rbft.view), rbft.N, rbft.f, rbft.view, rbft.h)
		} else {
			rbft.logger.Noticef("======== Replica %d finished updateN, primary=%d, n=%d/f=%d/view=%d/h=%d", rbft.no, rbft.primaryIndex(rbft.view), rbft.N, rbft.f, rbft.view, rbft.h)
		}

		var finishMsg interface{}
		if rbft.no == 0 {
			finishMsg = fmt.Sprintf("======== New Replica finished updateN, primary=%d, n=%d/f=%d/view=%d/h=%d", rbft.primaryIndex(rbft.view), rbft.N, rbft.f, rbft.view, rbft.h)
		} else {
			finishMsg = fmt.Sprintf("======== Replica %d finished updateN, primary=%d, n=%d/f=%d/view=%d/h=%d", rbft.no, rbft.primaryIndex(rbft.view), rbft.N, rbft.f, rbft.view, rbft.h)
		}
		rbft.external.SendFilterEvent(pb.InformType_FilterFinishUpdateN, finishMsg)
		rbft.primaryResubmitTransactions()
		delete(rbft.nodeMgr.updateStore, rbft.nodeMgr.updateTarget)
	case NodeMgrUpdateTimerEvent:
		rbft.storeMgr.missingReqBatches = make(map[string]bool)
		rbft.off(InUpdatingN)
		rbft.nodeMgr.updateHandled = false

		// clean the AddNode/DelNode messages in this turn
		rbft.nodeMgr.addNodeInfo = make(map[uint64]string)
		rbft.nodeMgr.delNodeInfo = make(map[uint64]string)
		rbft.nodeMgr.agreeUpdateStore = make(map[aidx]*pb.AgreeUpdateN)
		rbft.nodeMgr.updateStore = make(map[uidx]*pb.UpdateN)

		rbft.logger.Debugf("Replica %d update timer expired, restart recovery", rbft.no)
		return rbft.restartRecovery()

	default:
		rbft.logger.Errorf("Invalid node manager event: %v", e)
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
	case NodeMgrService:
		return rbft.dispatchNodeMgrMsg(e)
	case RecoveryService:
		return rbft.dispatchRecoveryMsg(e)
	default:
		rbft.logger.Errorf("Not Supported event: %v", e)
	}
	return nil
}

// dispatchMsgToService returns the service type of the given event. There exist 4 service types:
// 1. CoreRbftService: including tx related events, pre-prepare, prepare, commit, checkpoint, missing txs related events
// 2. ViewChangeService: including viewChange related events
// 3. RecoveryService: including recovery related events
// 4. NodeMgrService: including nodeManager related events
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

		// node_mgr service
	case *pb.ReadyForN:
		return NodeMgrService
	case *pb.UpdateN:
		return NodeMgrService
	case *pb.AgreeUpdateN:
		return NodeMgrService
	default:
		return NotSupportService

	}
}
