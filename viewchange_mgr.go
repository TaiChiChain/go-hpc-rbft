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
	"reflect"
	"sort"
	"time"

	pb "github.com/ultramesh/flato-rbft/rbftpb"
	txpool "github.com/ultramesh/flato-txpool"

	"github.com/gogo/protobuf/proto"
)

// vcManager manages the whole process of view change
type vcManager struct {
	viewChangePeriod   uint64        // period between automatic view changes. Default value is 0 means close automatic view changes
	viewChangeSeqNo    uint64        // next seqNo to perform view change
	lastNewViewTimeout time.Duration // last timeout we used during this view change
	newViewTimerReason string        // what triggered the timer
	vcHandled          bool          // if we have finished process new view or not

	qlist           map[qidx]*pb.Vc_PQ       // store Pre-Prepares for view change
	plist           map[uint64]*pb.Vc_PQ     // store Prepares for view change
	newViewStore    map[uint64]*pb.NewView   // track last new-view we received or sent
	viewChangeStore map[vcIdx]*pb.ViewChange // track view-change messages
	logger          Logger
}

// dispatchViewChangeMsg dispatches view change consensus messages from
// other peers And push them into corresponding function
func (rbft *rbftImpl) dispatchViewChangeMsg(e consensusEvent) consensusEvent {
	switch et := e.(type) {
	case *pb.ViewChange:
		return rbft.recvViewChange(et)
	case *pb.NewView:
		return rbft.recvNewView(et)
	case *pb.FetchRequestBatch:
		return rbft.recvFetchRequestBatch(et)
	case *pb.SendRequestBatch:
		return rbft.recvSendRequestBatch(et)
	}
	return nil
}

// newVcManager init a instance of view change manager and initialize each parameter
// according to the configuration file.
func newVcManager(c Config) *vcManager {
	vcm := &vcManager{
		qlist:           make(map[qidx]*pb.Vc_PQ),
		plist:           make(map[uint64]*pb.Vc_PQ),
		newViewStore:    make(map[uint64]*pb.NewView),
		viewChangeStore: make(map[vcIdx]*pb.ViewChange),
		logger:          c.Logger,
	}

	vcm.viewChangePeriod = c.VCPeriod
	// automatic view changes is off by default(should be read from config)
	if vcm.viewChangePeriod > 0 {
		vcm.logger.Infof("RBFT viewChange period = %v", vcm.viewChangePeriod)
	} else {
		vcm.logger.Infof("RBFT automatic viewChange disabled")
	}

	vcm.lastNewViewTimeout = c.NewViewTimeout

	return vcm
}

// setView sets the view with the viewLock.
func (rbft *rbftImpl) setView(view uint64) {
	rbft.viewLock.Lock()
	rbft.view = view
	rbft.viewLock.Unlock()
}

// sendViewChange sends view change message to other peers using broadcast.
// Then it sends view change message to itself and jump to recvViewChange.
func (rbft *rbftImpl) sendViewChange() consensusEvent {

	//Do some check and do some preparation
	//such as stop nullRequest timer, clean vcMgr.viewChangeStore and so on.
	err := rbft.beforeSendVC()
	if err != nil {
		return nil
	}

	//create viewChange message
	vc := &pb.ViewChange{
		Basis: rbft.getVcBasis(),
	}

	rbft.logger.Infof("Replica %d sending viewChange, v:%d, h:%d, |C|:%d, |P|:%d, |Q|:%d",
		rbft.peerPool.ID, vc.Basis.View, vc.Basis.H, len(vc.Basis.Cset), len(vc.Basis.Pset), len(vc.Basis.Qset))

	payload, err := proto.Marshal(vc)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_VIEW_CHANGE Marshal Error: %s", err)
		return nil
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_VIEW_CHANGE,
		From:    rbft.peerPool.ID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	//Broadcast viewChange message to other peers
	rbft.peerPool.broadcast(consensusMsg)

	event := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeResendTimerEvent,
	}
	//Start vcResendTimer. If peers can't viewChange successfully within the given time. timer well resend viewChange message
	rbft.timerMgr.startTimer(vcResendTimer, event)
	return rbft.recvViewChange(vc)
}

// recvViewChange processes ViewChange message from itself or other peers
// If the number of ViewChange message for equal view reach on
// allCorrectReplicasQuorum, return ViewChangeQuorumEvent.
// Else peers may resend vc or wait more vc message arrived.
func (rbft *rbftImpl) recvViewChange(vc *pb.ViewChange) consensusEvent {
	rbft.logger.Infof("Replica %d received viewChange from replica %d, v:%d, h:%d, |C|:%d, |P|:%d, |Q|:%d",
		rbft.peerPool.ID, vc.Basis.ReplicaId, vc.Basis.View, vc.Basis.H, len(vc.Basis.Cset), len(vc.Basis.Pset), len(vc.Basis.Qset))

	// TODO(DH): verify vc signature

	if rbft.in(initialCheck) {
		rbft.logger.Debugf("Replica %d is in initialCheck, cannot process viewChange messages", rbft.peerPool.ID)
		return nil
	}

	if vc.Basis.View < rbft.view {
		rbft.logger.Warningf("Replica %d found viewChange message for old view from replica %d: self view=%d, vc view=%d",
			rbft.peerPool.ID, vc.Basis.ReplicaId, rbft.view, vc.Basis.View)
		return nil
	}
	//check whether there is pqset which its view is less then vc's view and SequenceNumber more then low watermark
	//check whether there is cset which its SequenceNumber more then low watermark
	//if so ,return nil
	if !rbft.correctViewChange(vc) {
		rbft.logger.Warningf("Replica %d found viewChange message incorrect", rbft.peerPool.ID)
		return nil
	}

	// Check if this viewchange has stored in viewChangeStore, if so,return nil
	if old, ok := rbft.vcMgr.viewChangeStore[vcIdx{v: vc.Basis.View, id: vc.Basis.ReplicaId}]; ok {
		// Check after resend limit, since we may always sending the same vc
		// if no one response to our vc request (while the whole system keep the stage)
		if reflect.DeepEqual(old.Basis, vc.Basis) {

			rbft.logger.Warningf("Replica %d already has a same viewChange message"+
				" for view %d from replica %d, ignore it", rbft.peerPool.ID, vc.Basis.View, vc.Basis.ReplicaId)
			return nil
		}

		rbft.logger.Debugf("Replica %d already has a updated viewChange message"+
			" for view %d from replica %d, replace it", rbft.peerPool.ID, vc.Basis.View, vc.Basis.ReplicaId)
	}

	vc.Timestamp = time.Now().UnixNano()

	//store vc to viewChangeStore
	rbft.vcMgr.viewChangeStore[vcIdx{v: vc.Basis.View, id: vc.Basis.ReplicaId}] = vc

	// RBFT TOCS 4.5.1 Liveness: "if a replica receives a set of
	// f+1 valid VIEW-CHANGE messages from other replicas for
	// views greater than its current view, it sends a VIEW-CHANGE
	// message for the smallest view in the set, even if its timer
	// has not expired"
	replicas := make(map[uint64]bool)
	minView := uint64(0)
	for idx := range rbft.vcMgr.viewChangeStore {
		if vc.Timestamp+int64(rbft.timerMgr.getTimeoutValue(cleanViewChangeTimer)) < time.Now().UnixNano() {
			rbft.logger.Debugf("Replica %d drop an out-of-time viewChange message from replica %d",
				rbft.peerPool.ID, vc.Basis.ReplicaId)
			delete(rbft.vcMgr.viewChangeStore, idx)
			continue
		}

		if idx.v <= rbft.view {
			continue
		}

		replicas[idx.id] = true
		if minView == 0 || idx.v < minView {
			minView = idx.v
		}
	}

	// We only enter this if there are enough view change messages greater than our current view
	if len(replicas) >= rbft.oneCorrectQuorum() {
		rbft.logger.Infof("Replica %d received f+1 viewChange messages whose view is greater than "+
			"current view %d, detailed: %v, triggering viewChange to view %d", rbft.peerPool.ID, rbft.view, replicas, minView)
		// subtract one, because sendViewChange() increments
		newView := minView - uint64(1)
		rbft.setView(newView)
		return rbft.sendViewChange()
	}
	//calculate how many peers has view = rbft.view
	quorum := 0
	for idx := range rbft.vcMgr.viewChangeStore {
		if idx.v == rbft.view {
			quorum++
		}
	}
	rbft.logger.Debugf("Replica %d now has %d viewChange requests for view %d",
		rbft.peerPool.ID, quorum, rbft.view)

	// if in viewChange/recovery and vc.view = rbft.view and quorum > allCorrectReplicasQuorum
	// rbft find new view success and jump into ViewChangeQuorumEvent
	if rbft.atomicInOne(InViewChange, InRecovery) && vc.Basis.View == rbft.view && quorum >= rbft.allCorrectReplicasQuorum() {
		// as viewChange and recovery are mutually exclusive, we need to ensure
		// we have totally exit recovery before we jump into ViewChangeQuorumEvent
		if rbft.atomicIn(InRecovery) {
			rbft.logger.Infof("Replica %d in recovery changes to viewChange status", rbft.peerPool.ID)
			rbft.atomicOff(InRecovery)
			rbft.atomicOn(InViewChange)
			rbft.timerMgr.stopTimer(recoveryRestartTimer)
		}

		// close vcResendTimer
		rbft.timerMgr.stopTimer(vcResendTimer)

		// start newViewTimer and increase lastNewViewTimeout.
		// if this view change failed, next viewChange will have more time to do it
		// !!!NOTICE: only reset newViewTimer for the first time we reach the allCorrectReplicasQuorum
		rbft.softStartNewViewTimer(rbft.vcMgr.lastNewViewTimeout, "new viewChange", true)
		rbft.vcMgr.lastNewViewTimeout = 2 * rbft.vcMgr.lastNewViewTimeout
		if rbft.vcMgr.lastNewViewTimeout > 5*rbft.timerMgr.getTimeoutValue(newViewTimer) {
			rbft.vcMgr.lastNewViewTimeout = 5 * rbft.timerMgr.getTimeoutValue(newViewTimer)
		}

		// packaging ViewChangeQuorumEvent message
		return &LocalEvent{
			Service:   ViewChangeService,
			EventType: ViewChangeQuorumEvent,
		}
	}
	//if message from primary, peers send view change to other peers directly
	if rbft.isNormal() && rbft.isPrimary(vc.Basis.ReplicaId) {
		rbft.logger.Infof("Replica %d received viewChange from old primary %d for view %d, "+
			"trigger viewChange.", rbft.peerPool.ID, vc.Basis.ReplicaId, vc.Basis.View)
		rbft.sendViewChange()
	}

	return nil
}

// sendNewView select suitable pqc from viewChangeStore as a new view message and
// broadcast to replica peers when peer is primary and it receives
// allCorrectReplicasQuorum for new view.
// Then jump into primaryProcessNewView.
func (rbft *rbftImpl) sendNewView(notification bool) consensusEvent {

	//if this new view has stored, return nil.
	if _, ok := rbft.vcMgr.newViewStore[rbft.view]; ok {
		rbft.logger.Warningf("Replica %d already has newView in store for view %d, ignore it", rbft.peerPool.ID, rbft.view)
		return nil
	}
	var basis []*pb.VcBasis
	if notification {
		basis = rbft.getNotificationBasis()
	} else {
		basis = rbft.getViewChangeBasis()
	}

	//get suitable checkpoint for later recovery, replicas contains the peer no who has this checkpoint.
	//if can't find suitable checkpoint, ok return false.
	cp, ok := rbft.selectInitialCheckpoint(basis)
	if !ok {
		rbft.logger.Infof("Replica %d could not find consistent checkpoint: %+v", rbft.peerPool.ID, rbft.vcMgr.viewChangeStore)
		return nil
	}
	rbft.logger.Debugf("initial checkpoint: %+v", cp)
	//select suitable pqcCerts for later recovery.Their sequence is greater then cp
	//if msgList is nil, must some bug happened
	msgList := rbft.assignSequenceNumbers(basis, cp.SequenceNumber)
	if msgList == nil {
		rbft.logger.Infof("Replica %d could not assign sequence numbers for newView", rbft.peerPool.ID)
		return nil
	}
	rbft.logger.Debugf("x-set: %+v", msgList)
	//create new view message
	nv := &pb.NewView{
		View:      rbft.view,
		Xset:      msgList,
		ReplicaId: rbft.peerPool.ID,
		Bset:      basis,
	}

	// Check if primary need state update
	need, err := rbft.checkIfNeedStateUpdate(cp)
	if err != nil {
		return nil
	}
	if need {
		rbft.logger.Debugf("Primary %d needs to catch up in viewChange", rbft.peerPool.ID)
		return nil
	}

	rbft.logger.Infof("Replica %d is new primary, sending newView, v:%d, X:%+v",
		rbft.peerPool.ID, nv.View, nv.Xset)
	payload, err := proto.Marshal(nv)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_NEW_VIEW Marshal Error: %s", err)
		return nil
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_NEW_VIEW,
		From:    rbft.peerPool.ID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	//broadcast new view
	rbft.peerPool.broadcast(consensusMsg)
	//set new view to newViewStore
	rbft.vcMgr.newViewStore[rbft.view] = nv

	return rbft.primaryCheckNewView(msgList)
}

// recvNewView receives new view message and check if this node could
// process this message or not.
func (rbft *rbftImpl) recvNewView(nv *pb.NewView) consensusEvent {

	rbft.logger.Infof("Replica %d received newView %d from replica %d", rbft.peerPool.ID, nv.View, nv.ReplicaId)

	if !rbft.atomicInOne(InViewChange, InRecovery) {
		rbft.logger.Debugf("Replica %d reject newView as we are not in viewChange or recovery", rbft.peerPool.ID)
		return nil
	}

	if !(nv.View >= rbft.view && rbft.primaryID(nv.View) == nv.ReplicaId && rbft.vcMgr.newViewStore[nv.View] == nil) {
		rbft.logger.Warningf("Replica %d reject invalid newView from %d, v:%d", rbft.peerPool.ID, nv.ReplicaId, nv.View)
		return nil
	}

	// TODO(DH): verify vc/notification signature

	rbft.vcMgr.newViewStore[nv.View] = nv

	return rbft.replicaCheckNewView()
}

// primaryCheckNewView do some prepare for change to New view
// such as check if primary need state update and fetch missed batches
func (rbft *rbftImpl) primaryCheckNewView(xSet xset) consensusEvent {

	rbft.logger.Infof("New primary %d try to check new view", rbft.peerPool.ID)

	//check if we have all request batch in xSet
	newReqBatchMissing := rbft.feedMissingReqBatchIfNeeded(xSet)
	if len(rbft.storeMgr.missingReqBatches) == 0 {
		return rbft.resetStateForNewView()
	} else if newReqBatchMissing {
		// if received all batches, jump into resetStateForNewView
		rbft.fetchRequestBatches(xSet)
	}

	return nil
}

// replicaCheckNewView checks this newView message and see if it's legal.
func (rbft *rbftImpl) replicaCheckNewView() consensusEvent {

	rbft.logger.Infof("Replica %d try to check new view", rbft.peerPool.ID)

	nv, ok := rbft.vcMgr.newViewStore[rbft.view]
	if !ok {
		rbft.logger.Debugf("Replica %d ignore processNewView as it could not find view %d in its newViewStore", rbft.peerPool.ID, rbft.view)
		return nil
	}

	if !rbft.atomicInOne(InViewChange, InRecovery) {
		rbft.logger.Debugf("Replica %d reject newView as we are not in viewChange or recovery", rbft.peerPool.ID)
		return nil
	}

	cp, ok := rbft.selectInitialCheckpoint(nv.Bset)
	// todo wgr we won't step into such branch
	if !ok {
		rbft.logger.Infof("Replica %d could not determine initial checkpoint", rbft.peerPool.ID)
		return rbft.sendViewChange()
	}
	rbft.logger.Debugf("initial checkpoint: %+v", cp)

	// Check if the xset sent by new primary is built correctly by the aset
	msgList := rbft.assignSequenceNumbers(nv.Bset, cp.SequenceNumber)
	// todo wgr we won't step into such branch
	if msgList == nil {
		rbft.logger.Infof("Replica %d could not assign sequence numbers: %+v",
			rbft.peerPool.ID, rbft.vcMgr.viewChangeStore)
		return rbft.sendViewChange()
	}
	rbft.logger.Debugf("x-set: %+v", msgList)
	// todo wgr we won't step into such branch
	if !(len(msgList) == 0 && len(nv.Xset) == 0) && !reflect.DeepEqual(msgList, nv.Xset) {
		rbft.logger.Warningf("Replica %d failed to verify newView xset: computed %+v, received %+v",
			rbft.peerPool.ID, msgList, nv.Xset)
		return rbft.sendViewChange()
	}

	// Check if replica need state update
	need, err := rbft.checkIfNeedStateUpdate(cp)
	if err != nil {
		return nil
	}
	if need {
		// TODO(DH): is backup need to ensure state update before finishing viewChange?
		rbft.logger.Debugf("Replica %d needs to catch up in viewChange/recovery", rbft.peerPool.ID)
		return nil
	}

	// replica checks if we have all request batch in xSet
	newReqBatchMissing := rbft.feedMissingReqBatchIfNeeded(msgList)
	if len(rbft.storeMgr.missingReqBatches) == 0 {
		return rbft.resetStateForNewView()
	} else if newReqBatchMissing {
		// if received all batches, jump into resetStateForNewView
		rbft.fetchRequestBatches(msgList)
	}

	return nil
}

// resetStateForNewView reset all states for new view
func (rbft *rbftImpl) resetStateForNewView() consensusEvent {

	nv, ok := rbft.vcMgr.newViewStore[rbft.view]
	if !ok || nv == nil {
		rbft.logger.Warningf("Replica %d ignore processReqInNewView as it could not find view %d in its newViewStore", rbft.peerPool.ID, rbft.view)
		return nil
	}

	if !rbft.atomicInOne(InViewChange, InRecovery) {
		rbft.logger.Debugf("Replica %d is not in viewChange or recovery, not process new view", rbft.peerPool.ID)
		return nil
	}

	// if vcHandled active, return nil, else set vcHandled active
	if rbft.atomicIn(InViewChange) && rbft.vcMgr.vcHandled {
		rbft.logger.Debugf("Replica %d enter resetStateForNewView again, ignore it", rbft.peerPool.ID)
		return nil
	}
	rbft.vcMgr.vcHandled = true

	// if recoveryHandled active, return nil, else set recoveryHandled active
	if rbft.atomicIn(InRecovery) && rbft.recoveryMgr.recoveryHandled {
		rbft.logger.Debugf("Replica %d enter resetStateForNewView again, ignore it", rbft.peerPool.ID)
		return nil
	}
	rbft.recoveryMgr.recoveryHandled = true

	rbft.logger.Debugf("Replica %d accept newView to view %d", rbft.peerPool.ID, rbft.view)

	// empty the outstandingReqBatch, it is useless since new primary will resend pre-prepare
	rbft.cleanOutstandingAndCert()
	rbft.stopNewViewTimer()

	// set seqNo to lastExec for new primary to sort following batches from correct seqNo.
	rbft.batchMgr.setSeqNo(rbft.exec.lastExec)

	// clear requestPool cache to a correct state.
	rbft.putBackRequestBatches(nv.Xset)

	// clear consensus cache to a correct state.
	rbft.processNewView(nv.Xset)

	rbft.persistView(rbft.view)
	rbft.logger.Infof("Replica %d persist view=%d after new view", rbft.peerPool.ID, rbft.view)

	if rbft.atomicIn(InViewChange) {
		return &LocalEvent{
			Service:   ViewChangeService,
			EventType: ViewChangedEvent,
		}
	}
	if rbft.atomicIn(InRecovery) {
		return &LocalEvent{
			Service:   RecoveryService,
			EventType: RecoveryDoneEvent,
		}
	}
	return nil
}

// used in view-change to fetch missing assigned, non-checkpointed requests
func (rbft *rbftImpl) fetchRequestBatches(xSet xset) {

	for digest := range rbft.storeMgr.missingReqBatches {
		rbft.logger.Debugf("Replica %d try to fetch missing request batch with digest: %s", rbft.peerPool.ID, digest)
		frb := &pb.FetchRequestBatch{
			BatchDigest: digest,
			ReplicaId:   rbft.peerPool.ID,
		}
		payload, err := proto.Marshal(frb)
		if err != nil {
			rbft.logger.Errorf("ConsensusMessage_FRTCH_REQUEST_BATCH Marshal Error: %s", err)
			return
		}
		consensusMsg := &pb.ConsensusMessage{
			Type:    pb.Type_FETCH_REQUEST_BATCH,
			From:    rbft.peerPool.ID,
			Epoch:   rbft.epoch,
			Payload: payload,
		}
		rbft.peerPool.broadcast(consensusMsg)
	}

	return
}

// recvFetchRequestBatch returns the requested batch
func (rbft *rbftImpl) recvFetchRequestBatch(fr *pb.FetchRequestBatch) error {
	rbft.logger.Debugf("Replica %d received fetch request batch from replica %d with digest: %s",
		rbft.peerPool.ID, fr.ReplicaId, fr.BatchDigest)

	//Check if we have requested batch
	digest := fr.BatchDigest
	if _, ok := rbft.storeMgr.batchStore[digest]; !ok {
		return nil // we don't have it either
	}

	rbft.logger.Debugf("Replica %d return request batch with digest: %s", rbft.peerPool.ID, fr.BatchDigest)
	reqBatch := rbft.storeMgr.batchStore[digest]
	batch := &pb.SendRequestBatch{
		Batch:       reqBatch,
		BatchDigest: digest,
		ReplicaId:   rbft.peerPool.ID,
	}
	payload, err := proto.Marshal(batch)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_RETURN_REQUEST_BATCH Marshal Error: %s", err)
		return nil
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_SEND_REQUEST_BATCH,
		From:    rbft.peerPool.ID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	rbft.peerPool.unicast(consensusMsg, fr.ReplicaId)

	return nil
}

// recvSendRequestBatch receives the RequestBatch from other peers
// If receive all request batch, processing jump to processReqInNewView
// or processReqInUpdate
func (rbft *rbftImpl) recvSendRequestBatch(batch *pb.SendRequestBatch) consensusEvent {

	if batch == nil {
		rbft.logger.Errorf("Replica %d received return request batch with a nil batch", rbft.peerPool.ID)
		return nil
	}

	rbft.logger.Debugf("Replica %d received missing request batch from replica %d with digest: %s",
		rbft.peerPool.ID, batch.ReplicaId, batch.BatchDigest)

	digest := batch.BatchDigest
	if _, ok := rbft.storeMgr.missingReqBatches[digest]; !ok {
		rbft.logger.Debugf("Replica %d received missing request: %s, but we don't miss this request, ignore it", rbft.peerPool.ID, digest)
		return nil // either the wrong digest, or we got it already from someone else
	}
	// store into batchStore only，and store into requestPool by order when processNewView.
	rbft.storeMgr.batchStore[digest] = batch.Batch
	rbft.persistBatch(digest)

	//delete missingReqBatches in this batch
	delete(rbft.storeMgr.missingReqBatches, digest)

	//if receive all request batch
	//if InUpdatingN jump to processReqInUpdate
	if len(rbft.storeMgr.missingReqBatches) == 0 {
		if rbft.atomicInOne(InViewChange, InRecovery) {
			_, ok := rbft.vcMgr.newViewStore[rbft.view]
			if !ok {
				rbft.logger.Warningf("Replica %d ignore resetStateForNewView as it could not find view %d in its newViewStore", rbft.peerPool.ID, rbft.view)
				return nil
			}
			return rbft.resetStateForNewView()
		}
	}
	return nil

}

//##########################################################################
//           view change auxiliary functions
//##########################################################################

// stopNewViewTimer stops newViewTimer
func (rbft *rbftImpl) stopNewViewTimer() {

	rbft.logger.Debugf("Replica %d stop a running newView timer", rbft.peerPool.ID)
	rbft.timerMgr.stopTimer(newViewTimer)
}

// softstartNewViewTimer starts a new view timer no matter how many existed new view timer
func (rbft *rbftImpl) softStartNewViewTimer(timeout time.Duration, reason string, isNewView bool) {

	rbft.logger.Debugf("Replica %d soft start newView timer for %s: %s", rbft.peerPool.ID, timeout, reason)

	event := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeTimerEvent,
	}
	// set nextDemandView to current view because we will wait for lastNewViewTimeout
	// to confirm if the primary(in view nextDemandView) can actually finish this round
	// of viewChange by sending newView in time, but we may concurrently
	// receive f+1 others' viewChange to nextDemandView+1 and lastNewViewTimeoutEvent, so
	// we need to ensure if we really need to send viewChange when receive lastNewViewTimeoutEvent.
	if isNewView {
		event.Event = nextDemandNewView(rbft.view)
	}

	hasStarted, _ := rbft.timerMgr.softStartTimerWithNewTT(newViewTimer, timeout, event)
	if hasStarted {
		rbft.logger.Debugf("Replica %d has started new view timer before", rbft.peerPool.ID)
	} else {
		rbft.vcMgr.newViewTimerReason = reason
	}
}

// beforeSendVC operates before send view change
// 1. Check if state is in recovery, if so, reset it.
// 2. Stop NewViewTimer and nullRequestTimer
// 3. increase the view and delete new view of old view in newViewStore
// 4. update pqlist
// 5. delete old viewChange message
func (rbft *rbftImpl) beforeSendVC() error {

	// as viewChange and recovery are mutually exclusive, wen need to ensure
	// we have totally exit recovery before send viewChange.
	if rbft.atomicIn(InRecovery) {
		rbft.logger.Infof("Replica %d in recovery changes to viewChange status", rbft.peerPool.ID)
		rbft.atomicOff(InRecovery)
		rbft.timerMgr.stopTimer(recoveryRestartTimer)
	}

	rbft.stopNewViewTimer()
	rbft.timerMgr.stopTimer(nullRequestTimer)
	rbft.timerMgr.stopTimer(firstRequestTimer)

	rbft.atomicOn(InViewChange)
	rbft.setAbNormal()
	rbft.vcMgr.vcHandled = false

	newView := rbft.view + uint64(1)
	rbft.setView(newView)
	delete(rbft.vcMgr.newViewStore, rbft.view)

	// clear old messages
	for idx := range rbft.vcMgr.viewChangeStore {
		if idx.v < rbft.view {
			delete(rbft.vcMgr.viewChangeStore, idx)
		}
	}
	return nil
}

// correctViewChange checks if view change messages correct
// 1. pqlist' view should be less then vc.View and SequenceNumber should greater then vc.H.
// 2. checkpoint's SequenceNumber should greater then vc.H
func (rbft *rbftImpl) correctViewChange(vc *pb.ViewChange) bool {

	for _, p := range append(vc.Basis.Pset, vc.Basis.Qset...) {
		if !(p.View < vc.Basis.View && p.SequenceNumber > vc.Basis.H) {
			rbft.logger.Debugf("Replica %d find invalid p entry in viewChange: vc(v:%d h:%d) p(v:%d n:%d)",
				rbft.peerPool.ID, vc.Basis.View, vc.Basis.H, p.View, p.SequenceNumber)
			return false
		}
	}

	for _, c := range vc.Basis.Cset {
		if !(c.SequenceNumber >= vc.Basis.H) {
			rbft.logger.Debugf("Replica %d find invalid c entry in viewChange: vc(v:%d h:%d) c(n:%d)",
				rbft.peerPool.ID, vc.Basis.View, vc.Basis.H, c.SequenceNumber)
			return false
		}
	}

	return true
}

// getViewChangeBasis returns all viewChange basis from viewChangeStore
func (rbft *rbftImpl) getViewChangeBasis() (basis []*pb.VcBasis) {
	for _, vc := range rbft.vcMgr.viewChangeStore {
		basis = append(basis, vc.Basis)
	}
	return
}

// selectInitialCheckpoint selects checkpoint from received ViewChange message
// If find suitable checkpoint, it return a certain checkpoint and the replicas
// no list which replicas has this checkpoint.
// The checkpoint is the max checkpoint which exists in at least oneCorrectQuorum
// peers and greater then low waterMark in at least commonCaseQuorum.
func (rbft *rbftImpl) selectInitialCheckpoint(set []*pb.VcBasis) (checkpoint pb.Vc_C, find bool) {

	// For the checkpoint as key, find the corresponding basis messages
	checkpoints := make(map[pb.Vc_C][]*pb.VcBasis)
	for _, basis := range set {
		// Verify that we strip duplicate checkpoints from this Cset
		set := make(map[pb.Vc_C]bool)
		for _, c := range basis.Cset {
			if ok := set[*c]; ok {
				continue
			}
			checkpoints[*c] = append(checkpoints[*c], basis)
			set[*c] = true
			rbft.logger.Debugf("Replica %d appending checkpoint from replica %d with seqNo=%d, h=%d, and checkpoint digest %s",
				rbft.peerPool.ID, basis.ReplicaId, c.SequenceNumber, basis.H, c.Digest)
		}
	}

	// Indicate that replica cannot find any checkpoint
	if len(checkpoints) == 0 {
		rbft.logger.Debugf("Replica %d has no checkpoints to select from: %d %s",
			rbft.peerPool.ID, len(rbft.vcMgr.viewChangeStore), checkpoints)
		return
	}

	for idx, vcList := range checkpoints {
		// Need weak certificate for the checkpoint
		if len(vcList) < rbft.oneCorrectQuorum() { // type casting necessary to match types
			rbft.logger.Debugf("Replica %d has no weak certificate for n:%d, vcList was %d long",
				rbft.peerPool.ID, idx.SequenceNumber, len(vcList))
			continue
		}

		quorum := 0
		// Note, this is the whole vset (S) in the paper, not just this checkpoint set (S') (vcList)
		// We need 2f+1 low watermarks from S below this seqNo from all replicas
		// We need f+1 matching checkpoints at this seqNo (S')
		for _, vc := range set {
			if vc.H <= idx.SequenceNumber {
				quorum++
			}
		}

		if quorum < rbft.commonCaseQuorum() {
			rbft.logger.Debugf("Replica %d has no quorum for n:%d", rbft.peerPool.ID, idx.SequenceNumber)
			continue
		}

		// Find the highest checkpoint
		if checkpoint.SequenceNumber <= idx.SequenceNumber {
			checkpoint = idx
			find = true
		}
	}

	return
}

// assignSequenceNumbers selects a request to pre-prepare in the new view
// for each sequence number n between h and h + L, which is according to
// Castro's TOCS PBFT, Fig. 4.
func (rbft *rbftImpl) assignSequenceNumbers(set []*pb.VcBasis, h uint64) map[uint64]string {

	msgList := make(map[uint64]string)

	maxN := h + 1

	// "for all n such that h < n <= h + L"
nLoop:
	for n := h + 1; n <= h+rbft.L; n++ {
		// "∃m ∈ S..."
		for _, m := range set {
			// "...with <n,d,v> ∈ m.P"
			for _, em := range m.Pset {
				if n != em.SequenceNumber {
					continue
				}
				quorum := 0
				// "A1. ∃2f+1 messages m' ∈ S"
			mpLoop:
				for _, mp := range set {
					if mp.H >= n {
						continue
					}
					// "∀<n,d',v'> ∈ m'.P"
					for _, emp := range mp.Pset {
						if n != emp.SequenceNumber {
							continue
						}
						if !(emp.View < em.View || (emp.View == em.View && emp.BatchDigest == em.BatchDigest)) {
							continue mpLoop
						}
					}
					quorum++
				}

				if quorum < rbft.commonCaseQuorum() {
					continue
				}

				quorum = 0
				// "A2. ∃f+1 messages m' ∈ S"
				for _, mp := range set {
					// "∃<n,d',v'> ∈ m'.Q"
					for _, emp := range mp.Qset {
						if n != emp.SequenceNumber {
							continue
						}
						if emp.View >= em.View && emp.BatchDigest == em.BatchDigest {
							quorum++
							break
						}
					}
				}

				if quorum < rbft.oneCorrectQuorum() {
					continue
				}

				// "then select the request with digest d for number n"
				msgList[n] = em.BatchDigest
				maxN = n

				continue nLoop
			}
		}

		quorum := 0
		// "else if ∃2f+1 messages m ∈ S"
	nullLoop:
		for _, m := range set {
			// "m.h < n"
			if m.H >= n {
				continue
			}
			// "m.P has no entry for n"
			for _, em := range m.Pset {
				if em.SequenceNumber == n {
					continue nullLoop
				}
			}
			quorum++
		}

		if quorum >= rbft.commonCaseQuorum() {
			// "then select the null request for number n"
			msgList[n] = ""

			continue nLoop
		}

		rbft.logger.Warningf("Replica %d could not assign value to contents of seqNo %d, found only %d missing P entries", rbft.peerPool.ID, n, quorum)
		return nil
	}

	// prune top null requests
	// TODO(DH): is it safe to prune all null request beyond maxN?
	// if new primary update its seqNo to larger maxN?
	for n, msg := range msgList {
		if n >= maxN && msg == "" {
			delete(msgList, n)
		}
	}

	keys := make([]uint64, len(msgList))
	i := 0
	for n := range msgList {
		keys[i] = n
		i++
	}
	sort.Sort(sortableUint64List(keys))
	x := h + 1
	list := make(map[uint64]string)
	for _, n := range keys {
		list[x] = msgList[n]
		x++
	}

	return list
}

// updateViewChangeSeqNo updates viewChangeSeqNo by viewChangePeriod
func (rbft *rbftImpl) updateViewChangeSeqNo(seqNo, K uint64) {

	if rbft.vcMgr.viewChangePeriod <= 0 {
		return
	}
	// Ensure the view change always occurs at a checkpoint boundary
	rbft.vcMgr.viewChangeSeqNo = seqNo - seqNo%K + rbft.vcMgr.viewChangePeriod*K
}

// feedMissingReqBatchIfNeeded feeds needed reqBatch when this node
// doesn't have all reqBatch in xset.
func (rbft *rbftImpl) feedMissingReqBatchIfNeeded(xset xset) (newReqBatchMissing bool) {

	rbft.storeMgr.missingReqBatches = make(map[string]bool)
	newReqBatchMissing = false
	for n, d := range xset {
		// RBFT: why should we use "h ≥ min{n | ∃d : (<n,d> ∈ X)}"?
		// "h ≥ min{n | ∃d : (<n,d> ∈ X)} ∧ ∀<n,d> ∈ X : (n ≤ h ∨ ∃m ∈ in : (D(m) = d))"
		if n <= rbft.h {
			continue
		} else {
			if d == "" {
				// don't need to fetch null request.
				continue
			}

			if _, ok := rbft.storeMgr.batchStore[d]; !ok {
				rbft.logger.Debugf("Replica %d missing assigned, non-checkpointed request batch %s", rbft.peerPool.ID, d)
				if _, ok := rbft.storeMgr.missingReqBatches[d]; !ok {
					rbft.logger.Infof("Replica %v needs to fetch batch %s", rbft.peerPool.ID, d)
					newReqBatchMissing = true
					rbft.storeMgr.missingReqBatches[d] = true
				}
			}
		}
	}
	return newReqBatchMissing
}

// processNewView re-construct certStore using prePrepare and prepare with digest in xSet.
func (rbft *rbftImpl) processNewView(msgList xset) {

	if len(msgList) == 0 {
		rbft.logger.Debugf("Replica %d directly finish process new view as msgList is empty.", rbft.peerPool.ID)
		return
	}

	isPrimary := rbft.isPrimary(rbft.peerPool.ID)

	// sort msgList by seqNo
	orderedKeys := make([]uint64, len(msgList))
	i := 0
	for n := range msgList {
		orderedKeys[i] = n
		i++
	}
	sort.Sort(sortableUint64List(orderedKeys))

	maxN := rbft.exec.lastExec

	rbft.atomicOff(InConfChange)

	for _, n := range orderedKeys {
		d := msgList[n]

		if n <= rbft.h {
			rbft.logger.Debugf("Replica %d not process seqNo %d in view %d", rbft.peerPool.ID, n, rbft.view)
			continue
		}

		// check if we are lack of the txBatch with given digest
		// this should not happen as we must have fetched missing batch before we enter processNewView
		batch, ok := rbft.storeMgr.batchStore[d]
		if !ok && d != "" {
			rbft.logger.Warningf("Replica %d is missing tx batch for seqNo=%d with digest '%s' for assigned seqNo", rbft.peerPool.ID, n, d)
			continue
		}

		cert := rbft.storeMgr.getCert(rbft.view, n, d)

		prePrep := &pb.PrePrepare{
			View:           rbft.view,
			SequenceNumber: n,
			BatchDigest:    d,
			ReplicaId:      rbft.primaryID(rbft.view),
		}
		if d == "" {
			rbft.logger.Infof("Replica %d need to process seqNo %d as a null request", rbft.peerPool.ID, n)
			// construct prePrepare with an empty batch
			prePrep.HashBatch = &pb.HashBatch{
				RequestHashList: []string{},
			}
		} else {
			// rebuild prePrepare with batch recorded in batchStore
			prePrep.HashBatch = &pb.HashBatch{
				RequestHashList: batch.RequestHashList,
				Timestamp:       batch.Timestamp,
			}

			// re-construct batches by order in xSet to de-duplicate txs during different batches in msgList which
			// may be cause by 'different primary puts the same txs into different batches with different seqNo'
			oldBatch := &txpool.RequestHashBatch{
				BatchHash:  batch.BatchHash,
				TxHashList: batch.RequestHashList,
				TxList:     batch.RequestList,
				Timestamp:  batch.Timestamp,
			}
			deDuplicateTxHashes, err := rbft.batchMgr.requestPool.ReConstructBatchByOrder(oldBatch)
			if err != nil {
				rbft.logger.Warningf("Replica %d failed to re-construct batch %s, err: %s, send viewChange", rbft.peerPool.ID, d, err)
				rbft.sendViewChange()
				return
			}
			if len(deDuplicateTxHashes) != 0 {
				rbft.logger.Noticef("Replica %d finds %d duplicate txs when re-construct batch %d with digest %s, "+
					"detailed: %+v", rbft.peerPool.ID, len(deDuplicateTxHashes), n, d, deDuplicateTxHashes)
				prePrep.HashBatch.DeDuplicateRequestHashList = deDuplicateTxHashes
			}
		}
		cert.prePrepare = prePrep
		rbft.persistQSet(prePrep)

		if n > maxN {
			maxN = n
		}
	}
	// update seqNo as new primary needs to start prePrepare with a correct number.
	// NOTE: directly set seqNo to maxN in xSet.
	rbft.batchMgr.setSeqNo(maxN)

	for _, n := range orderedKeys {
		d := msgList[n]
		// only backup needs to rebuild self's Prepare and broadcast this Prepare
		if !isPrimary {
			rbft.logger.Debugf("Replica %d sending prepare for view=%d/seqNo=%d/digest=%s after new view",
				rbft.peerPool.ID, rbft.view, n, d)
			prep := &pb.Prepare{
				ReplicaId:      rbft.peerPool.ID,
				View:           rbft.view,
				SequenceNumber: n,
				BatchDigest:    d,
			}
			if n > rbft.h {
				cert := rbft.storeMgr.getCert(rbft.view, n, d)
				cert.sentPrepare = true
				_ = rbft.recvPrepare(prep)
			}
			payload, err := proto.Marshal(prep)
			if err != nil {
				rbft.logger.Errorf("ConsensusMessage_PREPARE Marshal Error: %s", err)
				return
			}

			consensusMsg := &pb.ConsensusMessage{
				Type:    pb.Type_PREPARE,
				From:    rbft.peerPool.ID,
				Epoch:   rbft.epoch,
				Payload: payload,
			}
			rbft.peerPool.broadcast(consensusMsg)
		}

		// directly construct commit message for committed batches even though we have not went through
		// prePrepare and Prepare phase in new view because we may lose commit message in the following
		// normal case(because of elimination rule of PQC), after which, if new node needs to fetchPQC
		// to recover state after stable checkpoint, it will not get enough commit messages to recover
		// to latest height.
		// NOTE: this is always correct to construct certs of committed batches.
		if n > rbft.h && n <= rbft.exec.lastExec {
			rbft.logger.Debugf("Replica %d sending commit for view=%d/seqNo=%d/digest=%s after new view",
				rbft.peerPool.ID, rbft.view, n, d)
			cmt := &pb.Commit{
				ReplicaId:      rbft.peerPool.ID,
				View:           rbft.view,
				SequenceNumber: n,
				BatchDigest:    d,
			}

			cert := rbft.storeMgr.getCert(rbft.view, n, d)
			cert.sentCommit = true
			_ = rbft.recvCommit(cmt)

			payload, err := proto.Marshal(cmt)
			if err != nil {
				rbft.logger.Errorf("ConsensusMessage_COMMIT Marshal Error: %s", err)
				return
			}

			consensusMsg := &pb.ConsensusMessage{
				Type:    pb.Type_COMMIT,
				From:    rbft.peerPool.ID,
				Epoch:   rbft.epoch,
				Payload: payload,
			}
			rbft.peerPool.broadcast(consensusMsg)
		}
	}
}
