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

	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/gogo/protobuf/proto"
)

/**
Node control issues
*/

// nodeManager is the state manager of in configuration.
type nodeManager struct {
	newNodeHash      string                    // track new node's local hash
	addNodeInfo      map[string]bool           // track tha add node hash
	delNodeInfo      map[string]bool           // track the delete node hash
	agreeUpdateStore map[aidx]*pb.AgreeUpdateN // track agree-update-n message
	updateStore      map[uidx]*pb.UpdateN      // track last update-n we received or sent
	updateTarget     uidx                      // track the new view after update
	updateHandled    bool                      // if we have updateN or not
}

// newNodeMgr creates a new nodeManager with the specified startup parameters.
func newNodeMgr() *nodeManager {
	nm := &nodeManager{
		addNodeInfo:      make(map[string]bool),
		delNodeInfo:      make(map[string]bool),
		agreeUpdateStore: make(map[aidx]*pb.AgreeUpdateN),
		updateStore:      make(map[uidx]*pb.UpdateN),
	}

	return nm
}

// dispatchNodeMgrMsg dispatches node manager service messages from other peers
// and uses corresponding function to handle them.
func (rbft *rbftImpl) dispatchNodeMgrMsg(e consensusEvent) consensusEvent {
	switch et := e.(type) {
	case *pb.ReadyForN:
		return rbft.recvReadyforNforAdd(et)
	case *pb.UpdateN:
		return rbft.recvUpdateN(et)
	case *pb.AgreeUpdateN:
		return rbft.recvAgreeUpdateN(et)
	}
	return nil
}

// New node itself set newNode flag and send new node message to other nodes.
func (rbft *rbftImpl) recvLocalNewNode(newHash string) consensusEvent {

	rbft.logger.Debugf("New replica %d received local newNode message", rbft.no)

	if rbft.in(isNewNode) {
		rbft.logger.Warningf("New replica %d received duplicate local newNode message", rbft.no)
		return nil
	}

	// the key about new node cannot be nil, it will results failure of updateN
	if len(newHash) == 0 {
		rbft.logger.Warningf("New replica %d received nil local newNode message", rbft.no)
		return nil
	}

	rbft.on(isNewNode)

	// the key of new node should be stored, since it will be use in
	rbft.nodeMgr.newNodeHash = newHash
	rbft.persistNewNodeHash([]byte(newHash))

	return nil
}

// recvLocalAddNode handles the local consensusEvent about DelNode, which announces the
// consentor that a VP node wants to leave the consensus. If there only exists less than
// 5 VP nodes, we don't allow delete node.
// Notice: the node that wants to leave away from consensus should also handle this message.
func (rbft *rbftImpl) recvLocalDelNode(delHash string) consensusEvent {

	if !rbft.peerPool.isInRoutingTable(delHash) {
		rbft.logger.Warningf("Replica %d received delNode message, del node hash: %s, "+
			"but this node does't in routing table", rbft.no, delHash)
		return nil
	}

	rbft.logger.Debugf("Replica %d received local delNode message, del node hash: %s", rbft.no, delHash)

	if rbft.in(InViewChange) {
		rbft.logger.Warningf("Replica %d is in viewChange, reject process delete node", rbft.no)
		return nil
	}

	if rbft.in(InRecovery) {
		rbft.logger.Warningf("Replica %d is in recovery, reject process delete node", rbft.no)
		return nil
	}

	// We only support deleting when number of all VP nodes >= 5
	if rbft.N == 4 {
		rbft.logger.Warningf("Replica %d received delNode message, but we don't support delete as there're only 4 nodes", rbft.no)
		return nil
	}

	return rbft.sendAgreeUpdateNforDel(delHash)
}

// sendReadyForN broadcasts the ReadyForN message to others.
// Only new node will call this after finished recovery, ReadyForN message means that
// it has already caught up with others and wants to truly participate in the consensus.
func (rbft *rbftImpl) sendReadyForN() {

	if !rbft.in(isNewNode) {
		rbft.logger.Errorf("Replica %d isn't a new replica, but try to send readyForN", rbft.no)
		return
	}

	// If new node loses the local key, there may be something wrong
	if rbft.nodeMgr.newNodeHash == "" {
		rbft.logger.Errorf("New replica %d doesn't have local key for readyForN", rbft.no)
		return
	}

	if rbft.in(InViewChange) {
		rbft.logger.Errorf("New replica %d finds itself in viewChange, not sending readyForN", rbft.no)
		return
	}
	if rbft.in(InRecovery) {
		rbft.logger.Errorf("New replica %d finds itself in recovery, not sending readyForN", rbft.no)
		return
	}

	rbft.on(InUpdatingN)
	rbft.timerMgr.stopTimer(nullRequestTimer)
	rbft.timerMgr.stopTimer(firstRequestTimer)
	rbft.startUpdateTimer()
	rbft.setAbNormal()

	ready := &pb.ReadyForN{
		Hash:    rbft.nodeMgr.newNodeHash,
		ExpectN: uint64(rbft.N),
	}

	rbft.logger.Infof("Replica %d sending readyForN, node hash: %s, expectN: %d",
		rbft.no, rbft.nodeMgr.newNodeHash, ready.ExpectN)

	payload, err := proto.Marshal(ready)
	if err != nil {
		rbft.logger.Errorf("Marshal ReadyForN Error!")
		return
	}
	msg := &pb.ConsensusMessage{
		Type:    pb.Type_READY_FOR_N,
		Payload: payload,
	}

	// Broadcast to all VP nodes except itself
	rbft.peerPool.broadcast(msg)

	return
}

// recvReadyforNforAdd handles the ReadyForN message sent by new node.
func (rbft *rbftImpl) recvReadyforNforAdd(ready *pb.ReadyForN) consensusEvent {

	rbft.logger.Debugf("Replica %d received readyForN from new node %s, expectN: %d",
		rbft.no, ready.Hash, ready.ExpectN)

	if rbft.in(InViewChange) {
		rbft.logger.Warningf("Replica %d is in viewChange, reject the readyForN message", rbft.no)
		return nil
	}

	if rbft.in(InRecovery) {
		rbft.logger.Warningf("Replica %d is in recovery, reject the readyForN message", rbft.no)
		return nil
	}

	if rbft.peerPool.isInRoutingTable(ready.Hash) {
		rbft.logger.Debugf("Replica %d has %s in its routing table, ignore it...", rbft.no, ready.Hash)
		return nil
	}

	return rbft.sendAgreeUpdateNForAdd(ready.Hash, ready.ExpectN)
}

// sendAgreeUpdateNForAdd broadcasts the AgreeUpdateN message to others.
// This will be only called after receiving the ReadyForN message sent by new node or
// received f+1 others' AgreeUpdateNForAdd.
func (rbft *rbftImpl) sendAgreeUpdateNForAdd(newNodeHash string, expectN uint64) consensusEvent {

	rbft.logger.Debugf("Replica %d try to send agree updateN for add", rbft.no)

	if rbft.in(InUpdatingN) {
		rbft.logger.Debugf("Replica %d already in updatingN, don't send agreeUpdateN again")
		return nil
	}

	if rbft.in(isNewNode) {
		rbft.logger.Warningf("New replica %d does not need to send agreeUpdateN", rbft.no)
		return nil
	}

	// we only support add only one node at the same time whose expectN is current N + 1.
	if expectN != uint64(rbft.N)+1 {
		rbft.logger.Noticef("Replica %d reject agree updateN for add node with expect N %d, because current N is %d",
			rbft.no, expectN, rbft.N)
		return nil
	}

	if _, ok := rbft.nodeMgr.addNodeInfo[newNodeHash]; ok {
		rbft.logger.Debugf("Replica %d has sent agree updateN for add node %s, ignore it...", rbft.no, newNodeHash)
		return nil
	}
	rbft.nodeMgr.addNodeInfo[newNodeHash] = true

	// record new node hash in peerPool temporarily until finish updateN.
	rbft.peerPool.tempNewNodeHash = newNodeHash

	rbft.nodeMgr.updateHandled = false
	rbft.nodeMgr.updateStore = make(map[uidx]*pb.UpdateN)
	// clear old messages
	for idx := range rbft.nodeMgr.agreeUpdateStore {
		if idx.v < rbft.view {
			delete(rbft.nodeMgr.agreeUpdateStore, idx)
		}
	}

	// Calculate the new N and view
	n, view := rbft.getAddNV()

	basis := rbft.getVcBasis()
	// In add/delete node, view in vcBasis shouldn't be the current view because after
	// UpdateN, view will always be changed.
	basis.View = view
	// Broadcast the AgreeUpdateN message
	agree := &pb.AgreeUpdateN{
		Basis:   basis,
		Flag:    true,
		Hash:    newNodeHash,
		N:       n,
		ExpectN: expectN,
	}

	// Replica may receive ReadyForN after it has already finished updatingN
	// (it happens in bad network environment)
	if int(agree.N) == rbft.N && agree.Basis.View == rbft.view {
		rbft.logger.Debugf("Replica %d already finished updateN for N=%d/view=%d", rbft.no, rbft.N, rbft.view)
		return nil
	}

	delete(rbft.nodeMgr.updateStore, rbft.nodeMgr.updateTarget)
	rbft.stopNewViewTimer()
	rbft.timerMgr.stopTimer(nullRequestTimer)
	rbft.timerMgr.stopTimer(firstRequestTimer)
	rbft.on(InUpdatingN)
	rbft.startUpdateTimer()
	rbft.setAbNormal()

	// Generate the AgreeUpdateN message and broadcast it to others
	rbft.agreeUpdateHelper(agree)
	rbft.logger.Debugf("Replica %d sending agreeUpdateN, v:%d, h:%d, expectN: %d, |C|:%d, |P|:%d, |Q|:%d",
		rbft.no, agree.Basis.View, agree.Basis.H, agree.ExpectN, len(agree.Basis.Cset), len(agree.Basis.Pset), len(agree.Basis.Qset))

	payload, err := proto.Marshal(agree)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_AGREE_UPDATE_N Marshal Error: %s", err)
		return nil
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_AGREE_UPDATE_N,
		Payload: payload,
	}
	rbft.peerPool.broadcast(consensusMsg)
	return rbft.recvAgreeUpdateN(agree)
}

// sendAgreeUpdateNforDel broadcasts the AgreeUpdateN message to other notify
// that it agree update the View & N as deleting a node.
func (rbft *rbftImpl) sendAgreeUpdateNforDel(delHash string) consensusEvent {
	delIndex := rbft.peerPool.findRouterIndexByHash(delHash)
	rbft.logger.Debugf("Replica %d try to send agree updateN for delete node %d", rbft.no, delIndex)

	if rbft.in(InUpdatingN) {
		rbft.logger.Debugf("Replica %d already in updatingN, don't send agreeUpdateN again")
		return nil
	}

	if _, ok := rbft.nodeMgr.delNodeInfo[delHash]; ok {
		rbft.logger.Debugf("Replica %d has sent agree updateN for delete node %s, ignore it...", rbft.no, delHash)
		return nil
	}
	rbft.nodeMgr.delNodeInfo[delHash] = true

	delete(rbft.nodeMgr.updateStore, rbft.nodeMgr.updateTarget)
	rbft.stopNewViewTimer()
	rbft.on(InUpdatingN)
	rbft.timerMgr.stopTimer(nullRequestTimer)
	rbft.timerMgr.stopTimer(firstRequestTimer)
	rbft.startUpdateTimer()
	rbft.setAbNormal()

	// Calculate the new N and view
	n, view := rbft.getDelNV(delIndex)
	basis := rbft.getVcBasis()
	// In add/delete node, view in vcBasis shouldn't be the current view because after
	// UpdateN, view will always be changed.
	basis.View = view
	agree := &pb.AgreeUpdateN{
		Basis: basis,
		Flag:  false,
		Hash:  delHash,
		N:     n,
	}

	rbft.agreeUpdateHelper(agree)
	rbft.logger.Debugf("Replica %d sending agreeUpdateN, v:%d, h:%d, |C|:%d, |P|:%d, |Q|:%d",
		rbft.no, agree.Basis.View, agree.Basis.H, len(agree.Basis.Cset), len(agree.Basis.Pset), len(agree.Basis.Qset))

	payload, err := proto.Marshal(agree)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_AGREE_UPDATE_N Marshal Error: %s", err)
		return nil
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_AGREE_UPDATE_N,
		Payload: payload,
	}
	rbft.peerPool.broadcast(consensusMsg)
	return rbft.recvAgreeUpdateN(agree)
}

// recvAgreeUpdateN handles the AgreeUpdateN message sent by others,
// checks the correctness and judges if it can move on to QuorumEvent.
func (rbft *rbftImpl) recvAgreeUpdateN(agree *pb.AgreeUpdateN) consensusEvent {

	sender := rbft.peerPool.noMap[agree.Basis.ReplicaId]
	rbft.logger.Debugf("Replica %d received agreeUpdateN from replica %d, v:%d, n:%d, flag:%v, h:%d, |C|:%d, |P|:%d, |Q|:%d",
		rbft.no, sender, agree.Basis.View, agree.N, agree.Flag, agree.Basis.H, len(agree.Basis.Cset), len(agree.Basis.Pset), len(agree.Basis.Qset))

	// Reject response to updating N as replica is in viewChange or recovery
	if rbft.in(InViewChange) {
		rbft.logger.Infof("Replica %d try to receive AgreeUpdateN, but it's in viewChange", rbft.no)
		return nil
	}
	if rbft.in(InRecovery) {
		rbft.logger.Infof("Replica %d try to receive AgreeUpdateN, but it's in recovery", rbft.no)
		return nil
	}

	rbft.checkAgreeUpdateN(agree)

	key := aidx{
		v:    agree.Basis.View,
		n:    agree.N,
		flag: agree.Flag,
		id:   agree.Basis.ReplicaId,
	}

	// Cast the vote of AgreeUpdateN into an existing or new tally
	if _, ok := rbft.nodeMgr.agreeUpdateStore[key]; ok {
		rbft.logger.Warningf("Replica %d already has a agreeUpdateN message"+
			" for view=%d/n=%d from replica %d", rbft.no, agree.Basis.View, agree.N, sender)
		return nil
	}
	rbft.nodeMgr.agreeUpdateStore[key] = agree

	// Count of the amount of AgreeUpdateN message for the same key
	replicas := make(map[uint64]bool)
	for idx := range rbft.nodeMgr.agreeUpdateStore {
		if !(idx.v == agree.Basis.View && idx.n == agree.N && idx.flag == agree.Flag) {
			continue
		}
		replicas[idx.id] = true
	}
	quorum := len(replicas)

	// We only enter this if there are enough agree-update-n messages but locally not inUpdateN
	if agree.Flag && quorum >= rbft.oneCorrectQuorum() && !rbft.in(InUpdatingN) {
		rbft.logger.Debugf("Replica %d received f+1 agreeUpdateN messages, triggering sendAgreeUpdateNForAdd",
			rbft.no)
		rbft.timerMgr.stopTimer(firstRequestTimer)
		return rbft.sendAgreeUpdateNForAdd(agree.Hash, agree.ExpectN)
	}

	// We only enter this if there are enough agree-update-n messages but locally not inUpdateN
	if !agree.Flag && quorum >= rbft.oneCorrectQuorum() && !rbft.in(InUpdatingN) {
		rbft.logger.Debugf("Replica %d received f+1agreeUpdateN messages, triggering sendAgreeUpdateNForDel",
			rbft.no)
		rbft.timerMgr.stopTimer(firstRequestTimer)
		return rbft.sendAgreeUpdateNforDel(agree.Hash)
	}

	rbft.logger.Debugf("Replica %d now has %d agreeUpdate requests for view=%d/n=%d, need %d", rbft.no, quorum, agree.Basis.View, agree.N, rbft.allCorrectReplicasQuorum())

	// Quorum of AgreeUpdateN reach the N, replica can jump to NODE_MGR_AGREE_UPDATEN_QUORUM_consensusEvent,
	// which mean all nodes agree in updating N
	if quorum >= rbft.allCorrectReplicasQuorum() {
		rbft.nodeMgr.updateTarget = uidx{v: agree.Basis.View, n: agree.N, flag: agree.Flag, hash: agree.Hash}
		return &LocalEvent{
			Service:   NodeMgrService,
			EventType: NodeMgrAgreeUpdateQuorumEvent,
		}
	}

	return nil
}

// sendUpdateN broadcasts the UpdateN message to other, it will only be called
// by primary like the NewView.
func (rbft *rbftImpl) sendUpdateN() consensusEvent {

	// Reject repeatedly broadcasting of UpdateN message
	if _, ok := rbft.nodeMgr.updateStore[rbft.nodeMgr.updateTarget]; ok {
		rbft.logger.Debugf("Primary %d already has updateN in store for n=%d/view=%d, skipping", rbft.no, rbft.nodeMgr.updateTarget.n, rbft.nodeMgr.updateTarget.v)
		return nil
	}

	basis := rbft.getAgreeUpdateBasis()

	// Check if primary can find the initial checkpoint for updating
	cp, ok, replicas := rbft.selectInitialCheckpoint(basis)
	if !ok {
		rbft.logger.Infof("Primary %d could not find consistent checkpoint: %+v", rbft.no, rbft.vcMgr.viewChangeStore)
		return nil
	}

	// Assign the seqNo according to the set of AgreeUpdateN and the initial checkpoint
	msgList := rbft.assignSequenceNumbers(basis, cp.SequenceNumber)
	if msgList == nil {
		rbft.logger.Infof("Primary %d could not assign sequence numbers for updateN", rbft.no)
		return nil
	}

	update := &pb.UpdateN{
		Flag:      rbft.nodeMgr.updateTarget.flag,
		N:         rbft.nodeMgr.updateTarget.n,
		View:      rbft.nodeMgr.updateTarget.v,
		Hash:      rbft.nodeMgr.updateTarget.hash,
		Xset:      msgList,
		ReplicaId: rbft.peerPool.localID,
		Bset:      basis,
	}

	// Check if primary need state update
	need, err := rbft.checkIfNeedStateUpdate(cp, replicas)
	if err != nil {
		return nil
	}
	if need {
		rbft.logger.Debugf("Primary %d needs to catch up in updatingN", rbft.no)
		return nil
	}

	rbft.logger.Infof("Replica %d is primary, sending updateN, v:%d, X:%+v",
		rbft.no, update.View, update.Xset)
	payload, err := proto.Marshal(update)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_UPDATE_N Marshal Error: %s", err)
		return nil
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_UPDATE_N,
		Payload: payload,
	}
	rbft.peerPool.broadcast(consensusMsg)
	rbft.nodeMgr.updateStore[rbft.nodeMgr.updateTarget] = update
	return rbft.primaryCheckUpdateN(cp, replicas, update)
}

// recvUpdateN handles the UpdateN message sent by primary.
func (rbft *rbftImpl) recvUpdateN(update *pb.UpdateN) consensusEvent {

	sender := rbft.peerPool.noMap[update.ReplicaId]
	rbft.logger.Debugf("Replica %d received updateN from replica %d", rbft.no, sender)

	// Reject response to updating N as replica is in viewChange or recovery
	if rbft.in(InViewChange) {
		rbft.logger.Infof("Replica %d try to receive UpdateN, but it's in viewChange", rbft.no)
		return nil
	}
	if rbft.in(InRecovery) {
		rbft.logger.Infof("Replica %d try to receive UpdateN, but it's in recovery", rbft.no)
		return nil
	}

	if !rbft.in(InUpdatingN) {
		rbft.logger.Debugf("Replica %d reject recvUpdateN as we are not in updatingN", rbft.no)
		return nil
	}

	// UpdateN can only be sent by primary
	if !(update.View >= 0 && rbft.isPrimary(update.ReplicaId)) {
		rbft.logger.Warningf("Replica %d rejecting invalid updateN from %d, v:%d", rbft.no, sender, update.View)
		return nil
	}

	key := uidx{
		flag: update.Flag,
		hash: update.Hash,
		n:    update.N,
		v:    update.View,
	}
	rbft.nodeMgr.updateStore[key] = update

	// Count of the amount of AgreeUpdateN message for the same key
	quorum := 0
	for idx, agree := range rbft.nodeMgr.agreeUpdateStore {
		if idx.v == update.View && idx.n == update.N && idx.flag == update.Flag && agree.Hash == update.Hash {
			quorum++
		}
	}
	// Reject to process UpdateN if replica has not reach allCorrectReplicasQuorum
	if quorum < rbft.allCorrectReplicasQuorum() {
		rbft.logger.Debugf("Replica %d has not meet agreeUpdateNQuorum", rbft.no)
		return nil
	}

	return rbft.replicaCheckUpdateN()
}

// primaryProcessUpdateN processes the UpdateN message after it has already reached
// updateN-quorum.
func (rbft *rbftImpl) primaryCheckUpdateN(initialCp pb.Vc_C, replicas []replicaInfo, update *pb.UpdateN) consensusEvent {

	// Check if primary need fetch missing requests
	newReqBatchMissing := rbft.feedMissingReqBatchIfNeeded(update.Xset)
	if len(rbft.storeMgr.missingReqBatches) == 0 {
		return rbft.resetStateForUpdate(update)
	} else if newReqBatchMissing {
		rbft.fetchRequestBatches(update.Xset)
	}

	return nil
}

// processUpdateN handles the UpdateN message sent from primary, it can only be called
// once replica has reached update-quorum.
func (rbft *rbftImpl) replicaCheckUpdateN() consensusEvent {

	// Get the UpdateN from the local cache
	update, ok := rbft.nodeMgr.updateStore[rbft.nodeMgr.updateTarget]
	if !ok {
		rbft.logger.Debugf("Replica %d ignore process UpdateN as it could not find n=%d/view=%d in its updateStore", rbft.no, rbft.nodeMgr.updateTarget.n, rbft.nodeMgr.updateTarget.v)
		return nil
	}

	if rbft.in(InViewChange) {
		sender := rbft.peerPool.noMap[update.ReplicaId]
		rbft.logger.Infof("Replica %d ignore updateN from replica %d, v:%d: we are in viewChange to view=%d",
			rbft.no, sender, update.View, rbft.view)
		return nil
	}

	if rbft.in(InRecovery) {
		sender := rbft.peerPool.noMap[update.ReplicaId]
		rbft.logger.Infof("Replica %d ignore updateN from replica %d, v:%d: we are in recovery to view=%d",
			rbft.no, sender, update.View, rbft.view)
		return nil
	}

	if !rbft.in(InUpdatingN) {
		sender := rbft.peerPool.noMap[update.ReplicaId]
		rbft.logger.Infof("Replica %d ignore updateN from replica %d, v:%d: we are not in updatingN",
			rbft.no, sender, update.View)
		return nil
	}

	// Find the initial checkpoint
	cp, ok, replicas := rbft.selectInitialCheckpoint(update.Bset)
	if !ok {
		rbft.logger.Infof("Replica %d could not determine initial checkpoint: %+v",
			rbft.no, rbft.vcMgr.viewChangeStore)
		return rbft.sendViewChange()
	}

	// Check if the xset sent by new primary is built correctly by the basis
	msgList := rbft.assignSequenceNumbers(update.Bset, cp.SequenceNumber)
	if msgList == nil {
		rbft.logger.Infof("Replica %d could not assign sequence numbers: %+v",
			rbft.no, rbft.vcMgr.viewChangeStore)
		return rbft.sendViewChange()
	}
	if !(len(msgList) == 0 && len(update.Xset) == 0) && !reflect.DeepEqual(msgList, update.Xset) {
		rbft.logger.Warningf("Replica %d failed to verify updateN xset: computed %+v, received %+v",
			rbft.no, msgList, update.Xset)
		return rbft.sendViewChange()
	}

	// Check if need state update
	need, err := rbft.checkIfNeedStateUpdate(cp, replicas)
	if err != nil {
		return nil
	}
	if need {
		rbft.logger.Debugf("Replica %d needs to catch up in updatingN", rbft.no)
		return nil
	}

	// replica checks if we have all request batch in xSet
	newReqBatchMissing := rbft.feedMissingReqBatchIfNeeded(msgList)
	if len(rbft.storeMgr.missingReqBatches) == 0 {
		return rbft.resetStateForUpdate(update)
	} else if newReqBatchMissing {
		// if received all batches, jump into resetStateForNewView
		rbft.fetchRequestBatches(msgList)
	}
	return nil
}

// resetStateForUpdate resets all the variables that need to be updated after updating n
func (rbft *rbftImpl) resetStateForUpdate(update *pb.UpdateN) consensusEvent {

	update, ok := rbft.nodeMgr.updateStore[rbft.nodeMgr.updateTarget]
	if !ok || update == nil {
		rbft.logger.Debugf("Primary %d ignore processReqInUpdate as it could not find target %v in its updateStore", rbft.no, rbft.nodeMgr.updateTarget)
		return nil
	}

	if !rbft.in(InUpdatingN) {
		rbft.logger.Debugf("Replica %d is not in updateN, not process updateN", rbft.no)
		return nil
	}

	if rbft.nodeMgr.updateHandled {
		rbft.logger.Debugf("Replica %d enter resetStateForUpdate again, ignore it", rbft.no)
		return nil
	}
	rbft.nodeMgr.updateHandled = true

	rbft.logger.Debugf("Replica %d accept updateN to target %+v", rbft.no, rbft.nodeMgr.updateTarget)

	rbft.stopNewViewTimer()
	rbft.timerMgr.stopTimer(nullRequestTimer)

	// Update the view, N and f
	rbft.view = update.View
	rbft.N = int(update.N)
	rbft.f = (rbft.N - 1) / 3

	if update.Flag {
		// when adding node, new node itself need only append itself to routing table, other origin nodes need append
		// new node to routing table
		if rbft.in(isNewNode) {
			rbft.logger.Debugf("New node %d itself update routingTable for %s in adding node", rbft.no, update.Hash)
			rbft.peerPool.updateAddNode(update.Hash)
		} else {
			if _, ok := rbft.nodeMgr.addNodeInfo[update.Hash]; !ok {
				rbft.logger.Debugf("Replica %d try to reset state when add node %s but cannot find add node info", rbft.no, update.Hash)
				return nil
			}

			// in adding node, wait until N and view have been confirmed, then update routing
			// table.
			rbft.logger.Debugf("Replica %d update routingTable for %s in adding node", rbft.no, update.Hash)
			rbft.peerPool.updateAddNode(update.Hash)
		}
	} else {
		if ok := rbft.nodeMgr.delNodeInfo[update.Hash]; !ok {
			rbft.logger.Debugf("Replica %d try to reset state when delete node %s but cannot find delete node info", rbft.no, update.Hash)
			return nil
		}

		// in deleting node, wait until N and view have been confirmed, then update routing
		// table.
		rbft.logger.Debugf("Replica %d update routingTable for %s in deleting node", rbft.no, update.Hash)
		rbft.peerPool.updateDelNode(update.Hash)
	}

	// Clean the AgreeUpdateN messages in this turn
	for idx := range rbft.nodeMgr.agreeUpdateStore {
		if idx.v == update.View && idx.n == update.N && idx.flag == update.Flag {
			delete(rbft.nodeMgr.agreeUpdateStore, idx)
		}
	}

	// Clean the AddNode/DelNode messages in this turn
	rbft.nodeMgr.addNodeInfo = make(map[string]bool)
	rbft.nodeMgr.delNodeInfo = make(map[string]bool)

	// empty the outstandingReqBatch, it is useless since new primary will resend pre-prepare
	rbft.cleanOutstandingAndCert()

	// set seqNo to lastExec for new primary to sort following batches from correct seqNo.
	rbft.batchMgr.setSeqNo(rbft.exec.lastExec)

	// clear pool cache to a correct state.
	rbft.putBackRequestBatches(update.Xset)

	// clear consensus cache to a correct state.
	rbft.processNewView(update.Xset)

	// persist N here rather than finished updatingN because now we have confirmed N already.
	// NOTE!!! we don't persist view here as we cannot confirm it's a stable view.
	rbft.persistN(rbft.N)
	rbft.persistView(rbft.view)
	rbft.logger.Infof("Replica %d persist view=%d/N=%d after updateN", rbft.no, rbft.view, rbft.N)

	return &LocalEvent{
		Service:   NodeMgrService,
		EventType: NodeMgrUpdatedEvent,
	}
}

//##########################################################################
//           node management auxiliary functions
//##########################################################################

// putBackRequestBatches reset all txs into 'non-batched' state in requestPool to prepare re-arrange by order.
func (rbft *rbftImpl) putBackRequestBatches(xset xset) {

	// remove all the batches that smaller than initial checkpoint.
	// those batches are the dependency of duplicator,
	// but we can remove since we already have checkpoint after viewChange.
	var deleteList []string
	for digest, batch := range rbft.storeMgr.batchStore {
		if batch.SeqNo <= rbft.h {
			rbft.logger.Debugf("Replica %d clear batch %s with seqNo %d <= initial checkpoint %d", rbft.no, digest, batch.SeqNo, rbft.h)
			delete(rbft.storeMgr.batchStore, digest)
			rbft.persistDelBatch(digest)
			deleteList = append(deleteList, digest)
		}
	}
	rbft.batchMgr.requestPool.RemoveBatches(deleteList)

	// directly restore all batchedTxs back into non-batched txs and re-arrange them by order when processNewView.
	rbft.batchMgr.requestPool.RestorePool()

	// clear cacheBatch as they are useless and all related batch have been restored in requestPool.
	rbft.batchMgr.cacheBatch = nil

	hashListMap := make(map[string]bool)
	for _, hash := range xset {
		hashListMap[hash] = true
	}

	// don't remove those batches which are not contained in xSet from batchStore as they may be useful
	// in next viewChange round.
	for digest := range rbft.storeMgr.batchStore {
		if hashListMap[digest] == false {
			rbft.logger.Debugf("Replica %d finds temporarily useless batch %s which is not contained in xSet", rbft.no, digest)
		}
	}
}

// agreeUpdateHelper helps generate the AgreeUpdateN message.
func (rbft *rbftImpl) agreeUpdateHelper(agree *pb.AgreeUpdateN) {
	for idx := range rbft.nodeMgr.agreeUpdateStore {
		if !(idx.v == agree.Basis.View && idx.n == agree.N && idx.flag == agree.Flag) {
			delete(rbft.nodeMgr.agreeUpdateStore, idx)
		}
	}
}

// checkAgreeUpdateN checks the AgreeUpdateN message if it's valid or not.
func (rbft *rbftImpl) checkAgreeUpdateN(agree *pb.AgreeUpdateN) bool {
	if agree.Flag {
		// Check the N and view after updating
		n, view := rbft.getAddNV()
		if n != agree.N || view != agree.Basis.View {
			rbft.logger.Debugf("Replica %d received incorrect agreeUpdateN: "+
				"expected n=%d/view=%d, get n=%d/view=%d", rbft.no, n, view, agree.N, agree.Basis.View)
			return false
		}

		// Check if there's any invalid p or q entry
		for _, p := range append(agree.Basis.Pset, agree.Basis.Qset...) {
			if !(p.View <= agree.Basis.View && p.SequenceNumber > agree.Basis.H && p.SequenceNumber <= agree.Basis.H+rbft.L) {
				rbft.logger.Debugf("Replica %d received invalid p entry in agreeUpdateN: "+
					"agree(v:%d h:%d) p(v:%d n:%d)", rbft.no, agree.Basis.View, agree.Basis.H, p.View, p.SequenceNumber)
				return false
			}
		}

	} else {
		// Check the N and view after updating
		delIndex := rbft.peerPool.findRouterIndexByHash(agree.Hash)
		n, view := rbft.getDelNV(delIndex)
		if n != agree.N || view != agree.Basis.View {
			rbft.logger.Debugf("Replica %d received incorrect agreeUpdateN: "+
				"expected n=%d/view=%d, get n=%d/view=%d", rbft.no, n, view, agree.N, agree.Basis.View)
			return false
		}

		// Check if there's any invalid p or q entry
		for _, p := range append(agree.Basis.Pset, agree.Basis.Qset...) {
			if !(p.View <= agree.Basis.View+1 && p.SequenceNumber > agree.Basis.H && p.SequenceNumber <= agree.Basis.H+rbft.L) {
				rbft.logger.Debugf("Replica %d received invalid p entry in agreeUpdateN: "+
					"agree(v:%d h:%d) p(v:%d n:%d)", rbft.no, agree.Basis.View, agree.Basis.H, p.View, p.SequenceNumber)
				return false
			}
		}
	}

	// Check if there's invalid checkpoint
	for _, c := range agree.Basis.Cset {
		if !(c.SequenceNumber >= agree.Basis.H && c.SequenceNumber <= agree.Basis.H+rbft.L) {
			rbft.logger.Warningf("Replica %d received invalid c entry in agreeUpdateN: "+
				"agree(v:%d h:%d) c(n:%d)", rbft.no, agree.Basis.View, agree.Basis.H, c.SequenceNumber)
			return false
		}
	}

	return true
}

// checkIfNeedStateUpdate checks if a replica needs to do state update
func (rbft *rbftImpl) checkIfNeedStateUpdate(initialCp pb.Vc_C, replicas []replicaInfo) (bool, error) {

	lastExec := rbft.exec.lastExec
	if rbft.exec.currentExec != nil {
		lastExec = *rbft.exec.currentExec
	}

	if rbft.h < initialCp.SequenceNumber {
		rbft.moveWatermarks(initialCp.SequenceNumber)
	}

	// If replica's lastExec < initial checkpoint, replica is out of date
	if lastExec < initialCp.SequenceNumber {
		rbft.logger.Warningf("Replica %d missing base checkpoint %d (%s), our most recent execution %d", rbft.no, initialCp.SequenceNumber, initialCp.Digest, lastExec)

		target := &stateUpdateTarget{
			targetMessage: targetMessage{
				height: initialCp.SequenceNumber,
				digest: initialCp.Digest,
			},
			replicas: replicas,
		}

		rbft.updateHighStateTarget(target)
		rbft.tryStateTransfer(target)
		return true, nil
	}

	return false, nil
}

// getAgreeUpdateBasis gets all the AgreeUpdateN basis the replica received.
func (rbft *rbftImpl) getAgreeUpdateBasis() (aset []*pb.VcBasis) {
	for _, agree := range rbft.nodeMgr.agreeUpdateStore {
		aset = append(aset, agree.Basis)
	}
	return
}

// startUpdateTimer starts the update timer
func (rbft *rbftImpl) startUpdateTimer() {
	localEvent := &LocalEvent{
		Service:   NodeMgrService,
		EventType: NodeMgrUpdateTimerEvent,
	}

	rbft.timerMgr.startTimer(updateTimer, localEvent)
	rbft.logger.Debugf("Replica %d started the update timer", rbft.no)
}

// stopUpdateTimer stops the update timer
func (rbft *rbftImpl) stopUpdateTimer() {
	rbft.timerMgr.stopTimer(updateTimer)
	rbft.logger.Debugf("Replica %d stopped the update timer", rbft.no)
}

func (rbft *rbftImpl) newNodeFinishUpdateNInRecovery() {

	//rbft.peerPool.setLocalID()
	rbft.persistDelNewNodeHash()

}
