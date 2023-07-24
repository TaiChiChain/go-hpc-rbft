package rbft

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/hyperchain/go-hpc-rbft/common/consensus"
	"github.com/hyperchain/go-hpc-rbft/external"
	"github.com/hyperchain/go-hpc-rbft/types"

	"github.com/gogo/protobuf/proto"
)

// MaxNumEpochEndingCheckpoint is max checkpoints allowed include in EpochChangeProof
const MaxNumEpochEndingCheckpoint = 100

// epochManager manages the epoch structure for RBFT.
type epochManager struct {
	// current epoch
	epoch uint64

	// storage for the state of config batch which is waiting for stable-verification
	// it might be assigned at the initiation of epoch-manager
	// it will be assigned after config-batch's execution to check a stable-state
	configBatchToCheck *types.MetaState

	// track the sequence number of the config block to execute
	// to notice Node transfer state related to config transactions into core
	configBatchToExecute uint64

	// only used by backup node to track the seqNo of config block in ordering, reject pre-prepare
	// with seqNo higher than this config block seqNo.
	configBatchInOrder uint64

	// mutex to set value of configBatchToExecute
	configBatchToExecuteLock sync.RWMutex

	// epoch related service
	epochService external.EpochService

	// peer pool
	peerPool *peerPool

	// logger
	logger Logger
}

func newEpochManager[T any, Constraint consensus.TXConstraint[T]](c Config[T, Constraint], pp *peerPool) *epochManager {
	em := &epochManager{
		epoch:                c.EpochInit,
		configBatchToCheck:   nil,
		configBatchToExecute: uint64(0),
		epochService:         c.External,
		peerPool:             pp,
		logger:               c.Logger,
	}

	if c.LatestConfig == nil {
		return em
	}

	// if the height of block is equal to the latest config batch,
	// we are not sure if such a config batch has been checked for stable yet or not,
	// so that, update the config-batch-to-check to trigger a stable point check
	if c.LatestConfig.Height == c.Applied {
		em.logger.Noticef("Latest config batch height %d equal to last exec, which may be non-stable",
			c.LatestConfig.Height)
		em.configBatchToCheck = c.LatestConfig
	}

	return em
}

// dispatchEpochMsg dispatches epoch service messages using service type
func (rbft *rbftImpl[T, Constraint]) dispatchEpochMsg(e consensusEvent) consensusEvent {
	switch et := e.(type) {
	case *consensus.FetchCheckpoint:
		return rbft.recvFetchCheckpoint(et)
	case *consensus.EpochChangeRequest:
		rbft.logger.Debugf("Replica %d don't process epoch change request from %s in same epoch",
			rbft.peerPool.ID, et.GetAuthor())
	case *consensus.EpochChangeProof:
		rbft.logger.Debugf("Replica %d don't process epoch change proof from %s in same epoch",
			rbft.peerPool.ID, et.GetAuthor())
	}
	return nil
}

func (rbft *rbftImpl[T, Constraint]) fetchCheckpoint() consensusEvent {
	if rbft.epochMgr.configBatchToCheck == nil {
		rbft.logger.Debugf("Replica %d doesn't need to check any batches", rbft.peerPool.ID)
		return nil
	}

	fetch := &consensus.FetchCheckpoint{
		ReplicaId:      rbft.peerPool.ID,
		SequenceNumber: rbft.epochMgr.configBatchToCheck.Height,
	}
	rbft.startFetchCheckpointTimer()

	payload, err := proto.Marshal(fetch)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_PREPARE Marshal Error: %s", err)
		return nil
	}
	consensusMsg := &consensus.ConsensusMessage{
		Type:    consensus.Type_FETCH_CHECKPOINT,
		Payload: payload,
	}

	rbft.logger.Debugf("Replica %d is fetching checkpoint %d", rbft.peerPool.ID, fetch.SequenceNumber)
	rbft.peerPool.broadcast(context.TODO(), consensusMsg)
	return nil
}

func (rbft *rbftImpl[T, Constraint]) recvFetchCheckpoint(fetch *consensus.FetchCheckpoint) consensusEvent {
	signedCheckpoint, ok := rbft.storeMgr.localCheckpoints[fetch.SequenceNumber]
	// If we can find a checkpoint in corresponding height, just send it back.
	if !ok {
		// If we cannot find it, the requesting node might fell behind a lot
		// send back our latest stable-checkpoint-info to help it to recover
		signedCheckpoint, ok = rbft.storeMgr.localCheckpoints[rbft.h]
		if !ok {
			rbft.logger.Warningf("Replica %d cannot find digest of its low watermark %d, "+
				"current node may fall behind", rbft.peerPool.ID, rbft.h)
			return nil
		}
	}

	payload, err := proto.Marshal(signedCheckpoint)
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_CHECKPOINT Marshal Error: %s", err)
		return nil
	}
	consensusMsg := &consensus.ConsensusMessage{
		Type:    consensus.Type_SIGNED_CHECKPOINT,
		Payload: payload,
	}
	rbft.peerPool.unicast(context.TODO(), consensusMsg, fetch.ReplicaId)

	return nil
}

func (rbft *rbftImpl[T, Constraint]) turnIntoEpoch() {
	rbft.logger.Trace(consensus.TagNameEpochChange, consensus.TagStageFinish, consensus.TagContentEpochChange{
		Epoch: rbft.epoch,
	})

	// validator set has been changed, start a new epoch and check new epoch
	epoch := rbft.external.Reconfiguration()

	// re-init vc manager and recovery manager as all caches related to view should
	// be reset in new epoch.
	// NOTE!!! all cert caches in storeManager will be clear in move watermark after
	// turnIntoEpoch.
	rbft.stopNewViewTimer()
	rbft.stopFetchViewTimer()
	rbft.vcMgr = newVcManager(rbft.config)
	rbft.recoveryMgr = newRecoveryMgr(rbft.config)

	// set the latest epoch
	rbft.setEpoch(epoch)

	// initial view 0 in new epoch.
	rbft.persistNewView(initialNewView)
	rbft.logger.Infof("Replica %d persist view=%d after epoch change", rbft.peerPool.ID, rbft.view)

	// update N/f
	rbft.N = len(rbft.peerPool.router)
	rbft.f = (rbft.N - 1) / 3
	rbft.persistN(rbft.N)

	rbft.metrics.clusterSizeGauge.Set(float64(rbft.N))
	rbft.metrics.quorumSizeGauge.Set(float64(rbft.commonCaseQuorum()))

	rbft.logger.Debugf("======== Replica %d turn into a new epoch, epoch=%d/N=%d/view=%d/height=%d.",
		rbft.peerPool.ID, rbft.epoch, rbft.N, rbft.view, rbft.exec.lastExec)
	rbft.logger.Notice(`

  +==============================================+
  |                                              |
  |             RBFT Start New Epoch             |
  |                                              |
  +==============================================+

`)
	validators := rbft.external.GetLastCheckpoint().ValidatorSet()
	vset := make([]string, 0, len(validators))
	for _, info := range validators {
		vset = append(vset, info.Hostname)
	}
	algoVersion := rbft.external.GetLastCheckpoint().Version()
	rbft.logger.Trace(consensus.TagNameEpochChange, consensus.TagStageStart, consensus.TagContentEpochChange{
		Epoch:        epoch,
		ValidatorSet: vset,
		AlgoVersion:  algoVersion,
	})
}

// setEpoch sets the epoch with the epochLock.
func (rbft *rbftImpl[T, Constraint]) setEpoch(epoch uint64) {
	rbft.epochLock.Lock()
	rbft.epoch = epoch
	rbft.epochMgr.epoch = epoch
	rbft.peerPool.epoch = epoch
	rbft.epochLock.Unlock()
	rbft.metrics.epochGauge.Set(float64(epoch))
}

func (rbft *rbftImpl[T, Constraint]) resetConfigBatchToExecute() {
	rbft.epochMgr.configBatchToExecuteLock.Lock()
	defer rbft.epochMgr.configBatchToExecuteLock.Unlock()
	rbft.epochMgr.configBatchToExecute = uint64(0)
}

func (rbft *rbftImpl[T, Constraint]) setConfigBatchToExecute(seqNo uint64) {
	rbft.epochMgr.configBatchToExecuteLock.Lock()
	defer rbft.epochMgr.configBatchToExecuteLock.Unlock()
	rbft.epochMgr.configBatchToExecute = seqNo
}

func (rbft *rbftImpl[T, Constraint]) readConfigBatchToExecute() uint64 {
	rbft.epochMgr.configBatchToExecuteLock.RLock()
	defer rbft.epochMgr.configBatchToExecuteLock.RUnlock()
	return rbft.epochMgr.configBatchToExecute
}

// checkEpoch compares local epoch and remote epoch:
// 1. remoteEpoch > currentEpoch, only accept EpochChangeProof, else retrieveEpochChange
// 2. remoteEpoch < currentEpoch, only accept EpochChangeRequest, else ignore
func (em *epochManager) checkEpoch(msg *consensus.ConsensusMessage) consensusEvent {
	currentEpoch := em.epoch
	remoteEpoch := msg.Epoch
	if remoteEpoch > currentEpoch {
		em.logger.Debugf("Replica %d received message type %s from %s with larger epoch, "+
			"current epoch %d, remote epoch %d", em.peerPool.ID, msg.Type, msg.Author, currentEpoch, remoteEpoch)
		// first process epoch sync response with higher epoch.
		if msg.Type == consensus.Type_EPOCH_CHANGE_PROOF {
			proof := &consensus.EpochChangeProof{}
			if uErr := proto.Unmarshal(msg.Payload, proof); uErr != nil {
				em.logger.Warningf("Unmarshal EpochChangeProof failed: %s", uErr)
				return uErr
			}
			return em.processEpochChangeProof(proof)
		}
		return em.retrieveEpochChange(currentEpoch, remoteEpoch, msg.Author)
	}

	if remoteEpoch < currentEpoch {
		em.logger.Debugf("Replica %d received message type %s from %s with lower epoch, "+
			"current epoch %d, remote epoch %d", em.peerPool.ID, msg.Type, msg.Author, currentEpoch, remoteEpoch)
		// first process epoch sync request with lower epoch.
		if msg.Type == consensus.Type_EPOCH_CHANGE_REQUEST {
			request := &consensus.EpochChangeRequest{}
			if uErr := proto.Unmarshal(msg.Payload, request); uErr != nil {
				em.logger.Warningf("Unmarshal EpochChangeRequest failed: %s", uErr)
				return uErr
			}
			return em.processEpochChangeRequest(request)
		}
		em.logger.Warningf("reject process message from %s with lower epoch %d, current epoch %d",
			msg.Author, remoteEpoch, currentEpoch)
	}
	return nil
}

func (em *epochManager) retrieveEpochChange(start, target uint64, recipient string) error {
	em.logger.Debugf("Replica %d request epoch changes %d to %d from %s", em.peerPool.ID, start, target, recipient)
	req := &consensus.EpochChangeRequest{
		Author:      em.peerPool.hostname,
		StartEpoch:  start,
		TargetEpoch: target,
	}
	payload, mErr := proto.Marshal(req)
	if mErr != nil {
		em.logger.Warningf("Marshal EpochChangeRequest failed: %s", mErr)
		return mErr
	}
	cum := &consensus.ConsensusMessage{
		Type:    consensus.Type_EPOCH_CHANGE_REQUEST,
		Payload: payload,
	}
	em.peerPool.unicastByHostname(context.TODO(), cum, recipient)
	return nil
}

func (em *epochManager) processEpochChangeRequest(request *consensus.EpochChangeRequest) error {
	em.logger.Debugf("Replica %d received epoch change request %s", em.peerPool.ID, request)

	if err := em.verifyEpochChangeRequest(request); err != nil {
		em.logger.Warningf("Verify epoch change request failed: %s", err)
		return err
	}

	if proof := em.pagingGetEpochChangeProof(request.StartEpoch, request.TargetEpoch, MaxNumEpochEndingCheckpoint); proof != nil {
		em.logger.Noticef("Replica %d send epoch change proof towards %s, info %s", em.peerPool.ID, request.GetAuthor(), proof)
		payload, mErr := proto.Marshal(proof)
		if mErr != nil {
			em.logger.Warningf("Marshal EpochChangeProof failed: %s", mErr)
			return mErr
		}
		cum := &consensus.ConsensusMessage{
			Type:    consensus.Type_EPOCH_CHANGE_PROOF,
			Payload: payload,
		}
		em.peerPool.unicastByHostname(context.TODO(), cum, request.Author)
		return nil
	}
	return nil
}

func (em *epochManager) processEpochChangeProof(proof *consensus.EpochChangeProof) consensusEvent {
	em.logger.Debugf("Replica %d received epoch change proof from %s", em.peerPool.ID, proof.Author)

	if changeTo := proof.NextEpoch(); changeTo <= em.epoch {
		// ignore proof old epoch which we have already started
		em.logger.Debugf("reject lower epoch change to %d", changeTo)
		return nil
	}

	// 1.Verify epoch-change-proof
	err := em.verifyEpochChangeProof(proof)
	if err != nil {
		em.logger.Errorf("failed to verify epoch change proof: %s", err)
		return err
	}

	// 2.Sync to epoch change state
	localEvent := &LocalEvent{
		Service:   EpochMgrService,
		EventType: EpochSyncEvent,
		Event:     proof,
	}
	return localEvent
}

// verifyEpochChangeRequest verify the legality of epoch change request.
func (em *epochManager) verifyEpochChangeRequest(request *consensus.EpochChangeRequest) error {
	if request == nil {
		return errors.New("NIL epoch-change-request")
	}
	if request.StartEpoch >= request.TargetEpoch {
		return fmt.Errorf("reject epoch change request for illegal change from %d to %d", request.StartEpoch, request.TargetEpoch)
	}
	if em.epoch < request.TargetEpoch {
		return fmt.Errorf("reject epoch change request for higher target %d from %s", request.TargetEpoch, request.Author)
	}
	return nil
}

// pagingGetEpochChangeProof returns epoch change proof with given page limit.
func (em *epochManager) pagingGetEpochChangeProof(startEpoch, endEpoch, pageLimit uint64) *consensus.EpochChangeProof {
	pagingEpoch := endEpoch
	more := uint64(0)

	if pagingEpoch-startEpoch > pageLimit {
		more = pagingEpoch
		pagingEpoch = startEpoch + pageLimit
	}

	checkpoints := make([]*consensus.QuorumCheckpoint, 0)
	for epoch := startEpoch; epoch < pagingEpoch; epoch++ {
		cp, err := em.epochService.GetCheckpointOfEpoch(epoch)
		if err != nil {
			em.logger.Warningf("Cannot find epoch change for epoch %d", epoch)
			return nil
		}
		checkpoints = append(checkpoints, cp)
	}

	return &consensus.EpochChangeProof{
		Author:      em.peerPool.hostname,
		More:        more,
		Checkpoints: checkpoints,
	}
}

func (em *epochManager) verifyEpochChangeProof(proof *consensus.EpochChangeProof) error {
	// Skip any stale checkpoints in the proof prefix. Note that with
	// the assertion above, we are guaranteed there is at least one
	// non-stale checkpoints in the proof.
	//
	// It's useful to skip these stale checkpoints to better allow for
	// concurrent node requests.
	//
	// For example, suppose the following:
	//
	// 1. My current trusted state is at epoch 5.
	// 2. I make two concurrent requests to two validators A and B, who
	//    live at epochs 9 and 11 respectively.
	//
	// If A's response returns first, I will ratchet my trusted state
	// to epoch 9. When B's response returns, I will still be able to
	// ratchet forward to 11 even though B's EpochChangeProof
	// includes a bunch of stale checkpoints (for epochs 5, 6, 7, 8).
	//
	// Of course, if B's response returns first, we will reject A's
	// response as it's completely stale.
	var (
		skip       int
		startEpoch uint64
	)
	for _, cp := range proof.GetCheckpoints() {
		if cp.Epoch() >= em.epoch {
			startEpoch = cp.Epoch()
			break
		}
		skip++
	}
	if startEpoch != em.epoch {
		return fmt.Errorf("invalid epoch change proof with start epoch %d, "+
			"current epoch %d", startEpoch, em.epoch)
	}
	proof.Checkpoints = proof.Checkpoints[skip:]

	if proof.IsEmpty() {
		return errors.New("empty epoch change proof")
	}
	return em.epochService.VerifyEpochChangeProof(proof, em.epochService.GetLastCheckpoint().ValidatorSet())
}
