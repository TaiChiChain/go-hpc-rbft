package rbft

import (
	"sync"

	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/gogo/protobuf/proto"
)

// epochManager manages the epoch structure for RBFT.
type epochManager struct {
	// storage of epoch check response to check epoch
	epochRspStore map[epochState]map[string]bool

	// storage for node to check if it has been out of epoch
	checkOutOfEpoch map[uint64]uint64

	// track the sequence number of the config transaction to execute
	// to notice Node transfer state related to config transactions into core
	configTransactionToExecute uint64

	// mutex to set value of configTransactionToExecute
	configTransactionToExecuteLock sync.RWMutex

	// logger
	logger Logger
}

func newEpochManager(c Config) *epochManager {
	em := &epochManager{
		epochRspStore:              make(map[epochState]map[string]bool),
		checkOutOfEpoch:            make(map[uint64]uint64),
		configTransactionToExecute: uint64(0),

		logger: c.Logger,
	}

	return em
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

func (rbft *rbftImpl) initEpochCheck(appliedToCheck uint64) consensusEvent {
	if rbft.atomicIn(Pending) {
		rbft.logger.Debugf("Replica %d is in pending status, don't start init epoch check", rbft.peerPool.ID)
		return nil
	}

	if rbft.in(InEpochCheck) {
		rbft.logger.Debugf("Replica %d try to send epochCheck, but it's already in epoch check", rbft.peerPool.ID)
		return nil
	}

	// open epoch check state and reset epoch check response store
	rbft.on(InEpochCheck)
	rbft.epochMgr.epochRspStore = make(map[epochState]map[string]bool)
	rbft.logger.Infof("======== Replica %d start epoch check, epoch=%d",
		rbft.peerPool.ID, rbft.epoch)

	// start epoch check response timer, when timeout restart epoch check
	event := &LocalEvent{
		Service:   EpochMgrService,
		EventType: EpochCheckTimerEvent,
		Event:     appliedToCheck,
	}
	rbft.timerMgr.startTimer(epochCheckRspTimer, event)

	rbft.sendEpochCheck(appliedToCheck)
	return nil
}

func (rbft *rbftImpl) sendEpochCheck(appliedToCheck uint64) consensusEvent {
	epochCheck := &pb.EpochCheck{
		ReplicaHash: rbft.peerPool.hash,
		Epoch:       rbft.epoch,
		Applied:     appliedToCheck,
	}

	payload, err := proto.Marshal(epochCheck)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_EPOCH_CHECK marshal error: %v", err)
		return nil
	}

	msg := &pb.ConsensusMessage{
		Type:    pb.Type_EPOCH_CHECK,
		From:    rbft.peerPool.ID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}
	rbft.logger.Debugf("Replica %d broadcast epoch-check: %+v", rbft.peerPool.ID, epochCheck)
	rbft.peerPool.broadcast(msg)

	epochCheckRsp := rbft.createEpochCheckRsp(epochCheck)
	return rbft.recvEpochCheckRsp(epochCheckRsp)
}

// recvEpochCheck recv the epoch check msg, and send epoch check response to that node
func (rbft *rbftImpl) recvEpochCheck(check *pb.EpochCheck) consensusEvent {
	if rbft.epoch < check.Epoch {
		rbft.logger.Debugf("Replica %d in a lower epoch %d reject check request from epoch %d",
			rbft.peerPool.ID, rbft.epoch, check.Epoch)
		return nil
	}

	epochCheckRsp := rbft.createEpochCheckRsp(check)
	payload, err := proto.Marshal(epochCheckRsp)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_EPOCH_CHECK_RESPONSE marshal error: %v", err)
		return nil
	}
	consensusMsg := &pb.ConsensusMessage{
		Type:    pb.Type_EPOCH_CHECK_RESPONSE,
		From:    rbft.peerPool.ID,
		Epoch:   rbft.epoch,
		Payload: payload,
	}

	rbft.logger.Debugf("Replica %d send epoch check response to Hash{%s}: %+v",
		rbft.peerPool.ID, check.ReplicaHash, epochCheckRsp)
	rbft.peerPool.unicastByHash(consensusMsg, check.ReplicaHash)

	return nil
}

func (rbft *rbftImpl) createEpochCheckRsp(check *pb.EpochCheck) *pb.EpochCheckResponse {
	// send the response for epoch check, including current epoch/check-state
	epochCheckRsp := &pb.EpochCheckResponse{
		ReplicaHash: rbft.peerPool.hash,
		Epoch:       rbft.epoch,
	}

	// find the checkpoint from the storage
	appliedIndex := check.Applied
	digest, ok := rbft.storeMgr.chkpts[appliedIndex]
	if ok {
		// current node has the checkpoint which the remote one want to check, send the applied and digest
		epochCheckRsp.CheckState = &pb.MetaState{
			Applied: appliedIndex,
			Digest:  digest,
		}
	} else {
		// current node cannot find the checkpoint which the remote one want to check,
		// so that the remote node might be in a incorrect state, send the stable checkpoint to trigger state-update
		rbft.logger.Debugf("Replica %d has not executed such config transaction with applied %d, "+
			"send response with self latest stable checkpoint", rbft.peerPool.ID, check.Applied)
		epochCheckRsp.CheckState = rbft.storeMgr.stableCheckpoint
	}

	return epochCheckRsp
}

func (rbft *rbftImpl) recvEpochCheckRsp(rsp *pb.EpochCheckResponse) consensusEvent {
	if !rbft.in(InEpochCheck) {
		rbft.logger.Debugf("Replica %d recv a epoch check response, but it's not in epoch check", rbft.peerPool.ID)
		return nil
	}

	// get the values
	var es epochState
	if rbft.atomicIn(InConfChange) {
		// when we are in config change, there is a config batch waiting for stable-checkpoint
		// so that we need to check the state for it
		rbft.logger.Debugf("Replica %d in config change only check the latest config batch state", rbft.peerPool.ID)
		es = epochState{
			applied: rsp.CheckState.Applied,
			digest:  rsp.CheckState.Digest,
		}
	} else {
		// we only need to check the epoch-number to find a correct epoch
		// for that we are not processing any config batches
		rbft.logger.Debugf("Replica %d only need to check epoch", rbft.peerPool.ID)
		es = epochState{
			epoch: rsp.Epoch,
		}
	}
	from := rsp.ReplicaHash

	// store the epoch message from other nodes
	if rbft.epochMgr.epochRspStore[es] == nil {
		rbft.epochMgr.epochRspStore[es] = make(map[string]bool)
	}
	rbft.epochMgr.epochRspStore[es][from] = true
	rbft.logger.Debugf("Replica %d received epoch check response from hash{%s}: %+v; now has %d",
		rbft.peerPool.ID, from, es, len(rbft.epochMgr.epochRspStore[es]))

	// when quorum responses received, check epoch and state
	if len(rbft.epochMgr.epochRspStore[es]) >= rbft.commonCaseQuorum() {
		// for epoch check, check the self-info(epoch, state of latest config transaction) at the same time
		// if self-info is included in quorum response set, it means it is correct
		if rbft.epochMgr.epochRspStore[es][rbft.peerPool.hash] {
			rbft.epochMgr.epochRspStore = make(map[epochState]map[string]bool)
			return &LocalEvent{
				Service:   EpochMgrService,
				EventType: EpochCheckDoneEvent,
				Event:     es.applied,
			}
		}

		// if there's something wrong with the state of latest config change,
		// close the config change and trigger the epoch-sync
		if rbft.atomicIn(InConfChange) {
			if rbft.exec.lastExec > es.applied {
				rbft.logger.Debugf("Replica %d (applied=%d) is ahead of others (applied=%d), continue to wait for the state of latest config batch",
					rbft.peerPool.ID, rbft.exec.lastExec, es.applied)
				return nil
			}

			rbft.logger.Noticef("Replica %d find incorrect state with latest config batch, stop config change and try to sync",
				rbft.peerPool.ID)
			rbft.atomicOff(InConfChange)
		}

		// node's epoch or state is incorrect, stop epoch check and try to sync epoch
		rbft.logger.Debugf("Replica %d in epoch %d has fell behind, try epoch sync", rbft.peerPool.ID, rbft.epoch)
		return rbft.restartEpochSync()
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
		rbft.peerPool.ID, rbft.epoch, msg.Epoch, len(rbft.epochMgr.checkOutOfEpoch))
	if len(rbft.epochMgr.checkOutOfEpoch) >= rbft.oneCorrectQuorum() {
		rbft.epochMgr.checkOutOfEpoch = make(map[uint64]uint64)
		rbft.tryEpochSync()
	}
}

func (rbft *rbftImpl) restartEpochSync() consensusEvent {
	if rbft.atomicIn(InEpochSync) {
		rbft.logger.Infof("Replica %d in epoch %d retry epoch sync", rbft.peerPool.ID, rbft.epoch)
		rbft.atomicOff(InEpochSync)
	}

	return rbft.tryEpochSync()
}

// tryEpochSync will start a process to sync into latest epoch
// we will start InEpochSync when we have found a node was out of epoch
func (rbft *rbftImpl) tryEpochSync() consensusEvent {
	if rbft.atomicIn(InConfChange) {
		rbft.logger.Debugf("Replica %d is processing a config transaction, don't start epoch sync", rbft.peerPool.ID)
		return nil
	}

	if rbft.atomicIn(InEpochSync) {
		rbft.logger.Debugf("Replica %d has already been in epoch sync, ignore it", rbft.peerPool.ID)
		return nil
	}

	rbft.logger.Noticef("======== Replica %d start epoch sync, N=%d/epoch=%d/height=%d/view=%d",
		rbft.peerPool.ID, rbft.N, rbft.epoch, rbft.exec.lastExec, rbft.view)
	rbft.atomicOn(InEpochSync)
	rbft.setAbNormal()

	if rbft.atomicIn(InRecovery) {
		rbft.logger.Infof("Replica %d close recovery status", rbft.peerPool.ID)
		rbft.atomicOff(InRecovery)
		rbft.timerMgr.stopTimer(recoveryRestartTimer)
	}

	if rbft.atomicIn(InViewChange) {
		rbft.logger.Infof("Replica %d close view-change status", rbft.peerPool.ID)
		rbft.atomicOff(InViewChange)
		rbft.timerMgr.stopTimer(newViewTimer)
	}

	if rbft.in(InEpochCheck) {
		rbft.logger.Infof("Replica %d close epoch-check status", rbft.peerPool.ID)
		rbft.off(InEpochCheck)
		rbft.epochMgr.epochRspStore = make(map[epochState]map[string]bool)
		rbft.timerMgr.stopTimer(epochCheckRspTimer)
	}

	return rbft.restartSyncState()
}

func (rbft *rbftImpl) turnIntoEpoch(router *pb.Router, epoch uint64) {
	// validator set has been changed, start a new epoch and check new epoch
	rbft.peerPool.updateRouter(router)

	// set the latest epoch
	rbft.setEpoch(epoch)

	// initial the view, start from view=0
	rbft.setView(uint64(0))
	rbft.persistView(rbft.view)

	// update N/f
	rbft.N = len(rbft.peerPool.router.Peers)
	rbft.f = (rbft.N - 1) / 3
	rbft.persistN(rbft.N)

	rbft.logger.Debugf("======== Replica %d turn into a new epoch, N=%d/epoch=%d",
		rbft.peerPool.ID, rbft.N, rbft.epoch)
	rbft.logger.Notice(`

  +==============================================+
  |                                              |
  |             RBFT Start New Epoch             |
  |                                              |
  +==============================================+

`)

	// reset reload router cache
	rbft.node.setReloadRouter(nil)
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

func (rbft *rbftImpl) resetConfigTransactionToExecute() {
	rbft.epochMgr.configTransactionToExecuteLock.Lock()
	defer rbft.epochMgr.configTransactionToExecuteLock.Unlock()
	rbft.epochMgr.configTransactionToExecute = uint64(0)
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
	rbft.logger.Debugf("Replica %d clean all the certs", rbft.peerPool.ID)
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
			rbft.logger.Debugf("Replica %d clear batch %s with seqNo %d <= initial checkpoint %d", rbft.peerPool.ID, digest, batch.SeqNo, rbft.h)
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
			rbft.logger.Debugf("Replica %d finds temporarily useless batch %s which is not contained in xSet", rbft.peerPool.ID, digest)
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
		rbft.logger.Warningf("Replica %d missing base checkpoint %d (%s), our most recent execution %d", rbft.peerPool.ID, initialCp.SequenceNumber, initialCp.Digest, lastExec)

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
