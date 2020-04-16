package rbft

import (
	"sync"

	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/gogo/protobuf/proto"
)

type epochManager struct {
	// storage of epoch check response to check epoch
	epochRspStore map[uint64]map[string]bool

	// storage for node to check if it has been out of epoch
	checkOutOfEpoch map[uint64]uint64

	// track the state when current epoch start
	epochStartState *pb.EpochStartState

	// track the sequence number of the config tx to execute
	configTransactionToExecute uint64

	// mutex to set value of configTransactionToExecute
	configTransactionToExecuteLock sync.RWMutex

	// logger
	logger Logger
}

//TODO(wgr): initial of epoch start state
func newEpochManager(c Config) *epochManager {
	return &epochManager{
		epochRspStore:              make(map[uint64]map[string]bool),
		checkOutOfEpoch:            make(map[uint64]uint64),
		epochStartState:            &pb.EpochStartState{},
		configTransactionToExecute: uint64(0),

		logger: c.Logger,
	}
}

// dispatchRecoveryMsg dispatches recovery service messages using service type
func (rbft *rbftImpl) dispatchEpochMsg(e consensusEvent) consensusEvent {
	switch et := e.(type) {
	case *pb.EpochCheck:
		return rbft.recvEpochCheck(et)
	case *pb.EpochCheckResponse:
		return rbft.recvEpochCheckRsp(et)
	}
	return nil
}

func (rbft *rbftImpl) initEpochCheck() consensusEvent {
	if rbft.in(Pending) {
		rbft.logger.Debugf("Replica %d is in pending status, don't start init epoch check", rbft.peerPool.localID)
		return nil
	}

	if rbft.in(InEpochCheck) {
		rbft.logger.Debugf("Replica %d try to send epochCheck, but it's already in epoch check", rbft.peerPool.localID)
		return nil
	}

	// open epoch check state and reset epoch check response store
	rbft.on(InEpochCheck)
	rbft.epochMgr.epochRspStore = make(map[uint64]map[string]bool)
	rbft.logger.Infof("======== Replica %d start epoch check: epoch=%d", rbft.peerPool.localID, rbft.epoch)

	// start epoch check response timer, when timeout restart epoch check
	event := &LocalEvent{
		Service:   EpochMgrService,
		EventType: EpochCheckTimerEvent,
	}
	rbft.timerMgr.startTimer(epochCheckRspTimer, event)

	rbft.sendEpochCheck()
	return nil
}

func (rbft *rbftImpl) sendEpochCheck() consensusEvent {
	epochCheck := &pb.EpochCheck{
		ReplicaId:   rbft.peerPool.localID,
		ReplicaHash: rbft.peerPool.localHash,
	}

	payload, err := proto.Marshal(epochCheck)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_EPOCH_CHECK marshal error: %v", err)
		return nil
	}

	msg := &pb.ConsensusMessage{
		Type:    pb.Type_EPOCH_CHECK,
		From:    rbft.peerPool.localID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	rbft.peerPool.broadcast(msg)

	epochCheckRsp := &pb.EpochCheckResponse{
		ReplicaHash: rbft.peerPool.localHash,
		Epoch:       rbft.epoch,
	}
	return rbft.recvEpochCheckRsp(epochCheckRsp)
}

// recvEpochCheck recv the epoch check msg, and send epoch check response to that node
func (rbft *rbftImpl) recvEpochCheck(check *pb.EpochCheck) consensusEvent {
	epochCheckRsp := &pb.EpochCheckResponse{
		ReplicaHash: rbft.peerPool.localHash,
		Epoch:       rbft.epoch,
	}

	payload, err := proto.Marshal(epochCheckRsp)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_EPOCH_CHECK_RESPONSE marshal error: %v", err)
		return nil
	}

	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_EPOCH_CHECK_RESPONSE,
		From:    rbft.peerPool.localID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}

	rbft.logger.Debugf("Replica %d send epoch check response to Hash{%s}: epoch=%d",
		rbft.peerPool.localID, check.ReplicaHash, rbft.epoch)
	rbft.peerPool.unicastByHash(consensusMsg, check.ReplicaHash)

	return nil
}

func (rbft *rbftImpl) recvEpochCheckRsp(rsp *pb.EpochCheckResponse) consensusEvent {
	if !rbft.in(InEpochCheck) {
		rbft.logger.Debugf("Replica %d recv a epoch check response, but it's not in epoch check", rbft.peerPool.localID)
		return nil
	}

	// get the values
	epoch := rsp.Epoch
	from := rsp.ReplicaHash

	if epoch < rbft.epoch {
		rbft.logger.Debugf("Replica %d received a response with epoch=%d from Hash=%s which is smaller than self, ignore it",
			rbft.peerPool.localID, epoch, from)
		return nil
	}

	// store the epoch message from other nodes
	if rbft.epochMgr.epochRspStore[epoch] == nil {
		rbft.epochMgr.epochRspStore[epoch] = make(map[string]bool)
	}
	rbft.epochMgr.epochRspStore[epoch][from] = true
	rbft.logger.Debugf("Replica %d received epoch check response from Hash{%s}: epoch=%d, now has %d",
		rbft.peerPool.localID, from, epoch, len(rbft.epochMgr.epochRspStore[epoch]))

	// there are quorum responses
	// 1. if the quorum epoch is equal to self, check done and start new epoch
	// 2. if the quorum epoch is different from self, start epoch sync
	if len(rbft.epochMgr.epochRspStore[epoch]) >= rbft.commonCaseQuorum() {
		if rbft.epoch == epoch {
			rbft.epochMgr.epochRspStore = make(map[uint64]map[string]bool)
			return &LocalEvent{
				Service:   EpochMgrService,
				EventType: EpochCheckDoneEvent,
			}
		}
		return rbft.tryEpochSync()
	}

	return nil
}

// checkIfOurOfEpoch track all the messages from nodes in lager epoch than self
func (rbft *rbftImpl) checkIfOutOfEpoch(msg *pb.ConsensusMessage) {
	// new node needn't check if it is out of epoch, for that it jumped into sync at the moment started
	if rbft.in(isNewNode) {
		return
	}

	rbft.epochMgr.checkOutOfEpoch[msg.From] = msg.Epoch
	rbft.logger.Debugf("Replica %d in epoch %d received message from epoch %d, now %d messages with larger epoch",
		rbft.peerPool.localID, rbft.epoch, msg.Epoch, len(rbft.epochMgr.checkOutOfEpoch))
	if len(rbft.epochMgr.checkOutOfEpoch) >= rbft.oneCorrectQuorum() {
		rbft.epochMgr.checkOutOfEpoch = make(map[uint64]uint64)
		rbft.tryEpochSync()
	}
}

// tryEpochSync will start a process to sync into latest epoch
// we will start InEpochSync when we have found a node was out of epoch
func (rbft *rbftImpl) tryEpochSync() consensusEvent {
	if rbft.in(InConfChange) {
		rbft.logger.Debugf("Replica %d is processing a config transaction, don't start epoch sync", rbft.peerPool.localID)
		return nil
	}

	if !rbft.in(InEpochSync) {
		rbft.logger.Noticef("======== Replica %d start epoch sync, N=%d/epoch=%d/height=%d/view=%d",
			rbft.peerPool.localID, rbft.N, rbft.epoch, rbft.exec.lastExec, rbft.view)
		rbft.on(InEpochSync)
	}

	if rbft.in(oneRoundOfEpochSync) {
		rbft.logger.Debugf("Replica %d has already started a round of epoch sync, waiting for such round finished",
			rbft.peerPool.localID)
		return nil
	}

	rbft.logger.Infof("Replica %d start one round of epoch sync, current epoch is %d", rbft.peerPool.localID, rbft.epoch)
	rbft.on(oneRoundOfEpochSync)
	rbft.setAbNormal()

	if rbft.in(InRecovery) {
		rbft.logger.Infof("Replica %d close recovery status", rbft.peerPool.localID)
		rbft.off(InRecovery)
		rbft.timerMgr.stopTimer(recoveryRestartTimer)
	}

	if rbft.in(InViewChange) {
		rbft.logger.Infof("Replica %d close view-change status", rbft.peerPool.localID)
		rbft.off(InViewChange)
		rbft.timerMgr.stopTimer(newViewTimer)
	}

	if rbft.in(InEpochCheck) {
		rbft.logger.Infof("Replica %d close epoch-check status", rbft.peerPool.localID)
		rbft.off(InEpochCheck)
		rbft.epochMgr.epochRspStore = make(map[uint64]map[string]bool)
		rbft.timerMgr.stopTimer(epochCheckRspTimer)
	}

	return rbft.restartSyncState()
}

func (rbft *rbftImpl) turnIntoEpoch(epoch uint64) {
	// 1. set the latest epoch
	rbft.setEpoch(epoch)
	rbft.persistEpoch(epoch)

	// 2. initial the view, start from view=0
	rbft.setView(uint64(0))
	rbft.persistView(rbft.view)

	// 3. update N/f
	rbft.N = len(rbft.peerPool.router.Peers)
	rbft.f = (rbft.N - 1) / 3

	rbft.logger.Debugf("Replica %d try to turn into a new epoch, N=%d/epoch=%d", rbft.peerPool.localID, rbft.N, rbft.epoch)
}

func (rbft *rbftImpl) resetStateForNewEpoch() {
	// delete txs in old epoch
	var delHashList []string
	for hash := range rbft.storeMgr.batchStore {
		delHashList = append(delHashList, hash)
	}
	rbft.batchMgr.requestPool.RemoveBatches(delHashList)

	// reset certs
	rbft.resetCertsAndBatches()

	// reset recovery store
	rbft.resetRecoveryStore()

	// reset view change store
	rbft.resetViewChangeStore()

	// reset check out of epoch
	rbft.epochMgr.checkOutOfEpoch = make(map[uint64]uint64)
}

// setEpoch sets the epoch with the epochLock.
func (rbft *rbftImpl) setEpoch(epoch uint64) {
	rbft.epochLock.Lock()
	defer rbft.epochLock.Unlock()
	rbft.epoch = epoch
}

func (rbft *rbftImpl) setConfigTransactionToExecute(seqNo uint64) {
	rbft.epochMgr.configTransactionToExecuteLock.Lock()
	defer rbft.epochMgr.configTransactionToExecuteLock.Unlock()
	rbft.epochMgr.configTransactionToExecute = seqNo
}

func (rbft *rbftImpl) readConfigTransactionToExecute() uint64 {
	rbft.epochMgr.configTransactionToExecuteLock.RLock()
	defer rbft.epochMgr.configTransactionToExecuteLock.RUnlock()
	return rbft.epochMgr.configTransactionToExecute
}

func (rbft *rbftImpl) resetCertsAndBatches() {
	rbft.logger.Debugf("Replica %d clean all the certs", rbft.peerPool.localID)
	for idx := range rbft.storeMgr.certStore {
		rbft.persistDelQPCSet(idx.v, idx.n, idx.d)
	}
	rbft.persistDelAllBatches()

	rbft.storeMgr.certStore = make(map[msgID]*msgCert)
	rbft.storeMgr.committedCert = make(map[msgID]string)
	rbft.storeMgr.outstandingReqBatches = make(map[string]*pb.RequestBatch)
	rbft.storeMgr.batchStore = make(map[string]*pb.RequestBatch)
	rbft.storeMgr.missingReqBatches = make(map[string]bool)

	rbft.batchMgr.cacheBatch = []*pb.RequestBatch{}
}

func (rbft *rbftImpl) resetRecoveryStore() {
	rbft.recoveryMgr.syncRspStore = make(map[string]*pb.SyncStateResponse)
	rbft.recoveryMgr.outOfElection = make(map[ntfIdx]*pb.NotificationResponse)
	rbft.recoveryMgr.notificationStore = make(map[ntfIdx]*pb.Notification)
}

func (rbft *rbftImpl) resetViewChangeStore() {
	rbft.vcMgr.qlist = make(map[qidx]*pb.Vc_PQ)
	rbft.vcMgr.plist = make(map[uint64]*pb.Vc_PQ)
	rbft.vcMgr.newViewStore = make(map[uint64]*pb.NewView)
	rbft.vcMgr.viewChangeStore = make(map[vcIdx]*pb.ViewChange)
	rbft.persistDelQPList()
}

// putBackRequestBatches reset all txs into 'non-batched' state in requestPool to prepare re-arrange by order.
func (rbft *rbftImpl) putBackRequestBatches(xset xset) {

	// remove all the batches that smaller than initial checkpoint.
	// those batches are the dependency of duplicator,
	// but we can remove since we already have checkpoint after viewChange.
	var deleteList []string
	for digest, batch := range rbft.storeMgr.batchStore {
		if batch.SeqNo <= rbft.h {
			rbft.logger.Debugf("Replica %d clear batch %s with seqNo %d <= initial checkpoint %d", rbft.peerPool.localID, digest, batch.SeqNo, rbft.h)
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
			rbft.logger.Debugf("Replica %d finds temporarily useless batch %s which is not contained in xSet", rbft.peerPool.localID, digest)
		}
	}
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
		rbft.logger.Warningf("Replica %d missing base checkpoint %d (%s), our most recent execution %d", rbft.peerPool.localID, initialCp.SequenceNumber, initialCp.Digest, lastExec)

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

func (rbft *rbftImpl) updateEpochStartState() {
	state := rbft.node.getCurrentState()
	rbft.epochMgr.epochStartState = &pb.EpochStartState{
		Applied: state.Applied,
		Digest:  state.Digest,
	}
}
