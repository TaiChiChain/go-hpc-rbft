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
	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/gogo/protobuf/proto"
)

/**
This file contains recovery related issues
*/

// recoveryManager manages recovery related events
type recoveryManager struct {
	syncRspStore      map[string]*pb.SyncStateResponse    // store sync state response
	notificationStore map[ntfIdx]*pb.Notification         // track notification messages
	outOfElection     map[ntfIdx]*pb.NotificationResponse // track out-of-election notification messages
	recoveryHandled   bool                                // if we have process new view or not

	logger Logger
}

// newRecoveryMgr news an instance of recoveryManager
func newRecoveryMgr(c Config) *recoveryManager {
	rm := &recoveryManager{
		syncRspStore:      make(map[string]*pb.SyncStateResponse),
		notificationStore: make(map[ntfIdx]*pb.Notification),
		outOfElection:     make(map[ntfIdx]*pb.NotificationResponse),
		logger:            c.Logger,
	}
	return rm
}

// dispatchRecoveryMsg dispatches recovery service messages using service type
func (rbft *rbftImpl) dispatchRecoveryMsg(e consensusEvent) consensusEvent {
	switch et := e.(type) {
	case *pb.SyncState:
		return rbft.recvSyncState(et)
	case *pb.SyncStateResponse:
		return rbft.recvSyncStateRsp(et)
	case *pb.RecoveryFetchPQC:
		return rbft.returnRecoveryPQC(et)
	case *pb.RecoveryReturnPQC:
		return rbft.recvRecoveryReturnPQC(et)
	case *pb.Notification:
		return rbft.recvNotification(et)
	case *pb.NotificationResponse:
		return rbft.recvNotificationResponse(et)
	}
	return nil
}

// initRecovery init recovery with a larger view in which we vote for next primary.
func (rbft *rbftImpl) initRecovery() consensusEvent {

	rbft.logger.Infof("Replica %d now initRecovery", rbft.peerPool.ID)

	return rbft.sendNotification(false)
}

// restartRecovery restarts recovery with current view in which we still vote for
// current primary.
func (rbft *rbftImpl) restartRecovery() consensusEvent {

	rbft.logger.Infof("Replica %d now restartRecovery", rbft.peerPool.ID)

	return rbft.sendNotification(true)
}

// sendNotification broadcasts notification messages to all other replicas to request
// all other replicas' current status, there will be two cases which need to trigger
// sendNotification:
// 1. current replica enter viewChange alone, but cannot receive enough viewChange in
//    viewChangeResend timer, so we need to confirm if other replicas are in normal
//    status(in which we need to revert current view).
// 2. system starts with no primary, we need to send notifications to confirm if all
//    other replicas are in normal status(in which we need to revert current view) or
//    all replicas are restarting together(in which we need to elect a new primary).
// flag keepCurrentVote means if we still vote for current primary or not.
func (rbft *rbftImpl) sendNotification(keepCurrentVote bool) consensusEvent {

	// as viewChange and recovery are mutually exclusive, we need to ensure
	// we have totally exit viewChange before send notification.
	if rbft.atomicIn(InViewChange) {
		rbft.logger.Infof("Replica %d in viewChange changes to recovery status", rbft.peerPool.ID)
		rbft.atomicOff(InViewChange)
		rbft.timerMgr.stopTimer(vcResendTimer)
	}

	rbft.stopNewViewTimer()
	rbft.timerMgr.stopTimer(nullRequestTimer)
	rbft.timerMgr.stopTimer(firstRequestTimer)

	rbft.atomicOn(InRecovery)
	rbft.recoveryMgr.recoveryHandled = false
	rbft.setAbNormal()

	// increase view according to what we predicate as next primary.
	if !keepCurrentVote {
		newView := rbft.view + uint64(1)
		rbft.setView(newView)
	}
	delete(rbft.vcMgr.newViewStore, rbft.view)

	// clear out messages
	for idx := range rbft.recoveryMgr.notificationStore {
		if idx.v < rbft.view {
			delete(rbft.recoveryMgr.notificationStore, idx)
		}
	}
	rbft.recoveryMgr.outOfElection = make(map[ntfIdx]*pb.NotificationResponse)

	n := &pb.Notification{
		Basis:     rbft.getVcBasis(),
		ReplicaId: rbft.peerPool.ID,
	}

	rbft.logger.Infof("Replica %d sending notification", rbft.peerPool.ID)

	rbft.logger.Infof("Replica %d sending notification, v:%d, h:%d, |C|:%d, |P|:%d, |Q|:%d",
		rbft.peerPool.ID, n.Basis.View, n.Basis.H, len(n.Basis.Cset), len(n.Basis.Pset), len(n.Basis.Qset))

	payload, err := proto.Marshal(n)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_NOTIFICATION Marshal Error: %s", err)
		return nil
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_NOTIFICATION,
		From:    rbft.peerPool.ID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	rbft.peerPool.broadcast(consensusMsg)

	event := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoveryRestartTimerEvent,
	}
	// use recoveryRestartTimer to track resend of notification.
	rbft.timerMgr.startTimer(recoveryRestartTimer, event)

	// use epoch sync when fetch the latest epoch

	if rbft.in(isNewNode) {
		rbft.logger.Debugf("New node %d doesn't send notification to itself", rbft.peerPool.ID)
		return nil
	}

	return rbft.recvNotification(n)
}

// recvNotification process notification messages:
// 1. while current node is new node, directly return
// 2. while current node is also in recovery, judge if we have received more than f+1
//    notification with larger view in which we need to resend notification with that view.
// 3. while current node is in normal, directly return NotificationResponse.
func (rbft *rbftImpl) recvNotification(n *pb.Notification) consensusEvent {

	rbft.logger.Debugf("Replica %d received notification from replica %d, v:%d, h:%d, |C|:%d, |P|:%d, |Q|:%d",
		rbft.peerPool.ID, n.ReplicaId, n.Basis.View, n.Basis.H, len(n.Basis.Cset), len(n.Basis.Pset), len(n.Basis.Qset))

	if rbft.in(InEpochCheck) {
		rbft.logger.Debugf("Replica %d is in epoch check, reject it", rbft.peerPool.ID)
		return nil
	}

	// new node cannot process notification as new node is not in a consistent
	// view/N with other nodes.
	if rbft.in(isNewNode) {
		rbft.logger.Debugf("New node %d ignore notification", rbft.peerPool.ID)
		return nil
	}

	if n.Basis.View < rbft.view {
		if rbft.isNormal() {
			// directly return notification response as we are in normal.
			return rbft.sendNotificationResponse(n.ReplicaId)
		}
		// ignore notification with lower view as we are in abnormal.
		rbft.logger.Debugf("Replica %d ignore notification with a lower view %d than self "+
			"view %d because we are in abnormal.", rbft.peerPool.ID, n.Basis.View, rbft.view)
		return nil
	}

	rbft.recoveryMgr.notificationStore[ntfIdx{v: n.Basis.View, nodeID: n.ReplicaId}] = n
	// find if there exists more than f same vote for view larger than current view.
	replicas := make(map[uint64]bool)
	minView := uint64(0)
	quorum := 0
	for idx := range rbft.recoveryMgr.notificationStore {
		if idx.v == n.Basis.View {
			quorum++
		}

		if idx.v <= rbft.view {
			continue
		}
		replicas[idx.nodeID] = true
		if minView == 0 || idx.v < minView {
			minView = idx.v
		}
	}
	if len(replicas) >= rbft.oneCorrectQuorum() {
		rbft.logger.Infof("Replica %d received f+1 notification messages whose view is greater than "+
			"current view %d, detailed: %v, sending notification for view %d", rbft.peerPool.ID, rbft.view, replicas, minView)
		newView := minView - uint64(1)
		rbft.setView(newView)
		return rbft.initRecovery()
	}

	rbft.logger.Debugf("Replica %d now has %d notification in notificationStore for view %d",
		rbft.peerPool.ID, quorum, n.Basis.View)

	if rbft.atomicIn(InRecovery) && n.Basis.View == rbft.view && quorum >= rbft.allCorrectReplicasQuorum() {
		rbft.timerMgr.stopTimer(recoveryRestartTimer)
		rbft.softStartNewViewTimer(rbft.vcMgr.lastNewViewTimeout, "new Notification", true)
		rbft.vcMgr.lastNewViewTimeout = 2 * rbft.vcMgr.lastNewViewTimeout
		if rbft.vcMgr.lastNewViewTimeout > 5*rbft.timerMgr.getTimeoutValue(newViewTimer) {
			rbft.vcMgr.lastNewViewTimeout = 5 * rbft.timerMgr.getTimeoutValue(newViewTimer)
		}

		return &LocalEvent{
			Service:   RecoveryService,
			EventType: NotificationQuorumEvent,
		}
	}

	// if we are in normal status, we should send back our info to the recovering node.
	if rbft.isNormal() {
		if rbft.isPrimary(n.ReplicaId) && n.Basis.View > rbft.view {
			rbft.logger.Infof("Replica %d received notification from old primary %d, trigger recovery.", rbft.peerPool.ID, n.ReplicaId)
			return rbft.initRecovery()
		}
		return rbft.sendNotificationResponse(n.ReplicaId)
	}

	return nil
}

// sendNotificationResponse helps send notification response to the given sender.
func (rbft *rbftImpl) sendNotificationResponse(destID uint64) consensusEvent {

	nr := &pb.NotificationResponse{
		Basis:    rbft.getVcBasis(),
		N:        uint64(rbft.N),
		Epoch:    rbft.epoch,
		NodeInfo: rbft.getNodeInfo(),
	}

	rbft.logger.Debugf("Replica %d send NotificationResponse to replica %d, v:%d, h:%d, |C|:%d, |P|:%d, |Q|:%d",
		rbft.peerPool.ID, destID, nr.Basis.View, nr.Basis.H, len(nr.Basis.Cset), len(nr.Basis.Pset), len(nr.Basis.Qset))

	rspMsg, err := proto.Marshal(nr)
	if err != nil {
		rbft.logger.Errorf("NotificationResponse marshal error")
		return nil
	}

	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_NOTIFICATION_RESPONSE,
		From:    rbft.peerPool.ID,
		Epoch:   rbft.epoch,
		Payload: rspMsg,
	}
	rbft.peerPool.unicast(consensusMsg, destID)
	return nil
}

// recvNotificationResponse only receives response from normal nodes, so we need only
// collect quorum same responses to process new view.
func (rbft *rbftImpl) recvNotificationResponse(nr *pb.NotificationResponse) consensusEvent {

	rbft.logger.Debugf("Replica %d received notificationResponse from replica %d, v:%d, h:%d, |C|:%d, |P|:%d, |Q|:%d",
		rbft.peerPool.ID, nr.NodeInfo.ReplicaId, nr.Basis.View, nr.Basis.H, len(nr.Basis.Cset), len(nr.Basis.Pset), len(nr.Basis.Qset))

	if !rbft.atomicIn(InRecovery) {
		rbft.logger.Debugf("Replica %d is not in recovery, ignore notificationResponse", rbft.peerPool.ID)
		return nil
	}

	// current is pending, sender is not pending, record to outOfElection and check
	// quorum same notifications in outOfElection.
	rbft.recoveryMgr.outOfElection[ntfIdx{v: nr.Basis.View, nodeID: nr.NodeInfo.ReplicaId}] = nr
	quorum := 0
	for idx := range rbft.recoveryMgr.outOfElection {
		if idx.v == nr.Basis.View {
			quorum++
		}
	}
	rbft.logger.Debugf("Replica %d now has %d notification response in outOfElection for "+
		"view %d, current view %d, need %d", rbft.peerPool.ID, quorum, nr.Basis.View, rbft.view, rbft.commonCaseQuorum())
	if quorum >= rbft.commonCaseQuorum() {
		states := make(wholeStates)
		for _, nrs := range rbft.recoveryMgr.outOfElection {
			states[nrs.NodeInfo] = nodeState{
				n:     nrs.N,
				view:  nrs.Basis.View,
				epoch: nrs.Epoch,
			}
		}
		return rbft.compareWholeStates(states)
	}

	return nil
}

// resetStateForRecovery only used by unusual nodes while quorum others nodes are in
// normal status.
func (rbft *rbftImpl) resetStateForRecovery() consensusEvent {
	rbft.logger.Debugf("Replica %d reset state in recovery for view=%d", rbft.peerPool.ID, rbft.view)

	basis := rbft.getOutOfElectionBasis()
	cp, ok, replicas := rbft.selectInitialCheckpoint(basis)
	if !ok {
		rbft.logger.Infof("Replica %d could not find consistent checkpoint.", rbft.peerPool.ID)
		return nil
	}
	// check if need state update
	need, err := rbft.checkIfNeedStateUpdate(cp, replicas)
	if err != nil {
		return nil
	}
	if need {
		// if we are behind by checkpoint, move watermark and state transfer to the target
		rbft.logger.Debugf("Replica %d in recovery find itself fall behind, "+
			"move watermark to %d and state transfer.", rbft.peerPool.ID, cp.SequenceNumber)

		// clear useless outstanding batch to avoid viewChange caused by outstanding batches after recovery.
		rbft.cleanOutstandingAndCert()
		return nil
	}

	// if recoveryHandled active, return nil, else set recoveryHandled active to avoid enter
	// RecoveryDoneEvent again.
	if rbft.recoveryMgr.recoveryHandled {
		rbft.logger.Debugf("Replica %d enter resetStateForRecovery again, ignore it", rbft.peerPool.ID)
		return nil
	}
	rbft.recoveryMgr.recoveryHandled = true

	rbft.cleanOutstandingAndCert()

	rbft.stopNewViewTimer()

	// clear all cert with different view.
	for idx := range rbft.storeMgr.certStore {
		if idx.v != rbft.view {
			rbft.logger.Debugf("Replica %d clear cert with view=%d/seqNo=%d/digest=%s when reset state for recovery",
				rbft.peerPool.ID, idx.v, idx.n, idx.d)
			delete(rbft.storeMgr.certStore, idx)
			rbft.persistDelQPCSet(idx.v, idx.n, idx.d)
		}
	}

	return &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoveryDoneEvent,
	}
}

// fetchRecoveryPQC always fetches PQC info after recovery done to fetch PQC info after target checkpoint
func (rbft *rbftImpl) fetchRecoveryPQC() consensusEvent {

	rbft.logger.Debugf("Replica %d fetchRecoveryPQC", rbft.peerPool.ID)

	fetch := &pb.RecoveryFetchPQC{
		H:         rbft.h,
		ReplicaId: rbft.peerPool.ID,
	}
	payload, err := proto.Marshal(fetch)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_RECOVERY_FETCH_QPC marshal error")
		return nil
	}
	conMsg := &pb.ConsensusMessage{
		Type:    pb.Type_RECOVERY_FETCH_QPC,
		From:    rbft.peerPool.ID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}

	rbft.peerPool.broadcast(conMsg)

	return nil
}

// returnRecoveryPQC returns all PQC info we have sent before to the sender
func (rbft *rbftImpl) returnRecoveryPQC(fetch *pb.RecoveryFetchPQC) consensusEvent {

	rbft.logger.Debugf("Replica %d returnRecoveryPQC to replica %d", rbft.peerPool.ID, fetch.ReplicaId)

	h := fetch.H
	if h >= rbft.h+rbft.L {
		rbft.logger.Warningf("Replica %d receives recoveryFetchPQC request, but its rbft.h ≥ highwatermark", rbft.peerPool.ID)
		return nil
	}

	var prePres []*pb.PrePrepare
	var pres []*pb.Prepare
	var cmts []*pb.Commit

	// replica just send all PQC info itself had sent before
	for idx, cert := range rbft.storeMgr.certStore {
		// send all PQC that n > h in current view, since it maybe wait others to execute
		if idx.n > h && idx.v == rbft.view {
			// only response with messages we have sent.
			if cert.prePrepare == nil {
				rbft.logger.Warningf("Replica %d in returnRecoveryPQC finds nil prePrepare for view=%d/seqNo=%d",
					rbft.peerPool.ID, idx.v, idx.n)
			} else if cert.prePrepare.ReplicaId == rbft.peerPool.ID {
				prePres = append(prePres, cert.prePrepare)
			}
			for pre := range cert.prepare {
				if pre.ReplicaId == rbft.peerPool.ID {
					prepare := pre
					pres = append(pres, &prepare)
				}
			}
			for cmt := range cert.commit {
				if cmt.ReplicaId == rbft.peerPool.ID {
					commit := cmt
					cmts = append(cmts, &commit)
				}
			}
		}
	}

	rcReturn := &pb.RecoveryReturnPQC{
		ReplicaId: rbft.peerPool.ID,
	}

	if prePres != nil {
		rcReturn.PrepreSet = prePres
	}
	if pres != nil {
		rcReturn.PreSet = pres
	}
	if cmts != nil {
		rcReturn.CmtSet = cmts
	}

	payload, err := proto.Marshal(rcReturn)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_RECOVERY_RETURN_QPC marshal error: %v", err)
		return nil
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_RECOVERY_RETURN_QPC,
		From:    rbft.peerPool.ID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	rbft.peerPool.unicast(consensusMsg, fetch.ReplicaId)

	rbft.logger.Debugf("Replica %d send recoveryReturnPQC to %d, detailed: %+v", rbft.peerPool.ID, fetch.ReplicaId, rcReturn)

	return nil
}

// recvRecoveryReturnPQC re-processes all the PQC received from others
func (rbft *rbftImpl) recvRecoveryReturnPQC(PQCInfo *pb.RecoveryReturnPQC) consensusEvent {
	rbft.logger.Debugf("Replica %d received recoveryReturnPQC from replica %d, return_pqc %v",
		rbft.peerPool.ID, PQCInfo.ReplicaId, PQCInfo)

	// post all the PQC
	if !rbft.isPrimary(rbft.peerPool.ID) {
		for _, preprep := range PQCInfo.GetPrepreSet() {
			_ = rbft.recvPrePrepare(preprep)
		}
	}
	for _, prep := range PQCInfo.GetPreSet() {
		_ = rbft.recvPrepare(prep)
	}
	for _, cmt := range PQCInfo.GetCmtSet() {
		_ = rbft.recvCommit(cmt)
	}

	return nil
}

// getNotificationBasis gets all the notification basis the replica received.
func (rbft *rbftImpl) getNotificationBasis() (basis []*pb.VcBasis) {
	for _, n := range rbft.recoveryMgr.notificationStore {
		basis = append(basis, n.Basis)
	}
	return
}

// getOutOfElectionBasis gets all the normal notification basis the replica stored in outOfElection.
func (rbft *rbftImpl) getOutOfElectionBasis() (basis []*pb.VcBasis) {
	for _, o := range rbft.recoveryMgr.outOfElection {
		basis = append(basis, o.Basis)
	}
	return
}

// when we are in abnormal or there are some requests in process, we don't need to sync state,
// we only need to sync state when primary is sending null request which means system is in
// normal status and there are no requests in process.
func (rbft *rbftImpl) trySyncState() {

	if !rbft.in(NeedSyncState) {
		if !rbft.isNormal() {
			rbft.logger.Debugf("Replica %d not try to sync state as we are in abnormal now", rbft.peerPool.ID)
			return
		}
		rbft.logger.Infof("Replica %d need to start sync state progress after %v", rbft.peerPool.ID, rbft.timerMgr.getTimeoutValue(syncStateRestartTimer))
		rbft.on(NeedSyncState)

		event := &LocalEvent{
			Service:   RecoveryService,
			EventType: RecoverySyncStateRestartTimerEvent,
		}

		// start sync state restart timer to cycle sync state while there are no new requests.
		rbft.timerMgr.startTimer(syncStateRestartTimer, event)
	}
}

// initSyncState prepares to sync state:
// 1. if we are in syncState, which means last syncState progress hasn't finish, so reject a new syncState request
// 2. if we are in abnormal, reject syncState as the priority of syncState is lower than recovery/viewChange/updateN
// 3. construct a syncState request then broadcast to other replicas
// 4. construct a syncStateRsp to myself
func (rbft *rbftImpl) initSyncState() consensusEvent {

	if rbft.in(InSyncState) {
		rbft.logger.Warningf("Replica %d try to send syncState, but it's already in sync state", rbft.peerPool.ID)
		return nil
	}

	rbft.on(InSyncState)

	rbft.logger.Debugf("Replica %d now init sync state", rbft.peerPool.ID)

	event := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoverySyncStateRspTimerEvent,
	}

	// start sync state response timer to wait for quorum response, if we cannot receive
	// enough response during this timeout, don't restart sync state as we will restart
	// sync state after syncStateRestartTimer expired.
	rbft.timerMgr.startTimer(syncStateRspTimer, event)

	rbft.recoveryMgr.syncRspStore = make(map[string]*pb.SyncStateResponse)

	// broadcast sync state message to others when it's not out of epoch
	syncStateMsg := &pb.SyncState{
		NodeInfo: rbft.getNodeInfo(),
		Epoch:    rbft.epoch,
	}
	payload, err := proto.Marshal(syncStateMsg)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_SYNC_STATE marshal error: %v", err)
		return nil
	}
	msg := &pb.ConsensusMessage{
		From:    rbft.peerPool.ID,
		Epoch:   rbft.epoch,
		Type:    pb.Type_SYNC_STATE,
		Payload: payload,
	}
	rbft.peerPool.broadcast(msg)

	if !rbft.atomicIn(InEpochSync) {
		// post the sync state response message event to myself
		state := rbft.node.getCurrentState()
		mRouter := minimizeRouter(rbft.peerPool.router)
		syncStateRsp := &pb.SyncStateResponse{
			NodeInfo:   rbft.getNodeInfo(),
			Epoch:      rbft.epoch,
			View:       rbft.view,
			MetaState:  state.MetaState,
			RouterInfo: serializeRouterInfo(mRouter),
		}
		rbft.recvSyncStateRsp(syncStateRsp)
	}
	return nil
}

func (rbft *rbftImpl) recvSyncState(sync *pb.SyncState) consensusEvent {
	rbft.logger.Debugf("Replica %d received sync state from %s", rbft.peerPool.ID, sync.NodeInfo.ReplicaHash)

	if rbft.in(isNewNode) {
		rbft.logger.Debugf("Replica %d is in a new node, don't send sync state response", rbft.peerPool.ID)
		return nil
	}

	if !rbft.isNormal() {
		rbft.logger.Debugf("Replica %d is in abnormal, don't send sync state response", rbft.peerPool.ID)
		return nil
	}

	if sync.Epoch < rbft.epoch {
		// if node send request in a lower epoch, such sync state response will help it to sync epoch
		return rbft.sendSyncStateRsp(sync.NodeInfo.ReplicaHash, true)
	} else if sync.Epoch == rbft.epoch {
		// normal case sync state
		return rbft.sendSyncStateRsp(sync.NodeInfo.ReplicaHash, false)
	} else {
		// received a message from larger epoch, current node might be out of epoch
		rbft.logger.Warningf("Replica %d might be out of epoch", rbft.peerPool.ID)
		return nil
	}
}

func (rbft *rbftImpl) sendSyncStateRsp(to string, needSyncEpoch bool) consensusEvent {
	var syncStateRsp *pb.SyncStateResponse
	mRouter := minimizeRouter(rbft.peerPool.router)
	syncStateRsp = &pb.SyncStateResponse{
		NodeInfo:   rbft.getNodeInfo(),
		Epoch:      rbft.epoch,
		View:       rbft.view,
		RouterInfo: serializeRouterInfo(mRouter),
	}

	if needSyncEpoch {
		// for request node in lower epoch, send latest stable checkpoint
		syncStateRsp.MetaState = rbft.storeMgr.stableCheckpoint
	} else {
		// for normal case, send current state
		state := rbft.node.getCurrentState()
		syncStateRsp.MetaState = state.MetaState
	}

	payload, err := proto.Marshal(syncStateRsp)
	if err != nil {
		rbft.logger.Errorf("Marshal SyncStateResponse Error!")
		return nil
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_SYNC_STATE_RESPONSE,
		From:    rbft.peerPool.ID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	rbft.peerPool.unicastByHash(consensusMsg, to)
	rbft.logger.Debugf("Replica %d send sync state response to host %s: epoch=%d, view=%d, meta_state=%+v",
		rbft.peerPool.ID, to, syncStateRsp.Epoch, syncStateRsp.View, syncStateRsp.MetaState)
	return nil
}

func (rbft *rbftImpl) recvSyncStateRsp(rsp *pb.SyncStateResponse) consensusEvent {
	rbft.logger.Debugf("Replica %d now received sync state response from host %s: epoch=%d, meta_state=%+v",
		rbft.peerPool.ID, rsp.NodeInfo.ReplicaHash, rsp.Epoch, rsp.MetaState)

	if !rbft.in(InSyncState) {
		rbft.logger.Debugf("Replica %d is not in sync state, ignore it...", rbft.peerPool.ID)
		return nil
	}

	if rsp.Epoch > rsp.MetaState.Applied {
		rbft.logger.Warningf("Replica %d in epoch %d reject a illegal response", rbft.peerPool.ID, rbft.epoch)
		return nil
	}
	if oldRsp, ok := rbft.recoveryMgr.syncRspStore[rsp.NodeInfo.ReplicaHash]; ok {
		if oldRsp.MetaState.Applied > rsp.MetaState.Applied {
			rbft.logger.Debugf("Duplicate sync state response, new applied=%d is lower than old applied=%d, reject it",
				rsp.MetaState.Applied, oldRsp.MetaState.Applied)
			return nil
		}
	}
	rbft.recoveryMgr.syncRspStore[rsp.NodeInfo.ReplicaHash] = rsp
	if len(rbft.recoveryMgr.syncRspStore) >= rbft.commonCaseQuorum() {
		states := make(wholeStates)
		for _, rsp := range rbft.recoveryMgr.syncRspStore {
			states[rsp.NodeInfo] = nodeState{
				epoch:        rsp.Epoch,
				view:         rsp.View,
				appliedIndex: rsp.MetaState.Applied,
				digest:       rsp.MetaState.Digest,
				routerInfo:   byte2Hex(rsp.RouterInfo),
			}
		}
		return rbft.compareWholeStates(states)
	}
	return nil
}

// restartSyncState restart syncState immediately, only can be invoked after sync state
// restart timer expired.
func (rbft *rbftImpl) restartSyncState() consensusEvent {

	rbft.logger.Debugf("Replica %d now restart sync state", rbft.peerPool.ID)

	rbft.recoveryMgr.syncRspStore = make(map[string]*pb.SyncStateResponse)
	rbft.initSyncState()

	event := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoverySyncStateRestartTimerEvent,
	}

	// start sync state restart timer to cycle sync state while there are no new requests.
	rbft.timerMgr.startTimer(syncStateRestartTimer, event)

	return nil
}

// exitSyncState exit syncState immediately.
func (rbft *rbftImpl) exitSyncState() {

	rbft.logger.Debugf("Replica %d now exit sync state", rbft.peerPool.ID)
	rbft.off(InSyncState)
	rbft.off(NeedSyncState)
	rbft.timerMgr.stopTimer(syncStateRspTimer)
	rbft.timerMgr.stopTimer(syncStateRestartTimer)
}
