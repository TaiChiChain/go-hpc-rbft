package rbft

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/axiomesh/axiom-bft/common"
	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/types"
)

// MaxNumEpochEndingCheckpoint is max checkpoints allowed include in EpochChangeProof
const MaxNumEpochEndingCheckpoint = 100

const (
	EpochStatePrefix = "epoch_q_chkpt."
	EpochIndexKey    = "epoch_latest_idx"
)

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
	epochService EpochService

	// It is persisted after updating to epochs
	epochProofCache map[uint64]*consensus.EpochChange

	// peer pool
	peerMgr *peerManager

	storage Storage // manage non-volatile storage of consensus log

	// logger
	logger common.Logger

	config Config

	chainConfig *ChainConfig
}

func newEpochManager(chainConfig *ChainConfig, c Config, pp *peerManager, epochService EpochService, storage Storage) *epochManager {
	em := &epochManager{
		chainConfig:          chainConfig,
		configBatchToCheck:   nil,
		configBatchToExecute: uint64(0),
		epochService:         epochService,
		epochProofCache:      make(map[uint64]*consensus.EpochChange),
		peerMgr:              pp,
		storage:              storage,
		logger:               c.Logger,
		config:               c,
	}

	return em
}

// dispatchEpochMsg dispatches epoch service messages using service type
func (rbft *rbftImpl[T, Constraint]) dispatchEpochMsg(e consensusEvent) consensusEvent {
	switch et := e.(type) {
	case *consensus.FetchCheckpoint:
		return rbft.recvFetchCheckpoint(et)
	case *consensus.EpochChangeRequest:
		rbft.logger.Debugf("Replica %d don't process epoch change request from %d in same epoch",
			rbft.chainConfig.SelfID, et)
	case *consensus.EpochChangeProof:
		rbft.logger.Debugf("Replica %d don't process epoch change proof from %d in same epoch",
			rbft.chainConfig.SelfID, et.GetAuthor())
	}
	return nil
}

func (rbft *rbftImpl[T, Constraint]) fetchCheckpoint() consensusEvent {
	if rbft.epochMgr.configBatchToCheck == nil {
		rbft.logger.Debugf("Replica %d doesn't need to check any batches", rbft.chainConfig.SelfID)
		return nil
	}

	fetch := &consensus.FetchCheckpoint{
		ReplicaId:      rbft.chainConfig.SelfID,
		SequenceNumber: rbft.epochMgr.configBatchToCheck.Height,
	}
	rbft.startFetchCheckpointTimer()

	payload, err := fetch.MarshalVTStrict()
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_PREPARE Marshal Error: %s", err)
		return nil
	}
	consensusMsg := &consensus.ConsensusMessage{
		Type:    consensus.Type_FETCH_CHECKPOINT,
		Payload: payload,
	}

	rbft.logger.Debugf("Replica %d is fetching checkpoint %d", rbft.chainConfig.SelfID, fetch.SequenceNumber)
	rbft.peerMgr.broadcast(context.TODO(), consensusMsg)
	return nil
}

func (rbft *rbftImpl[T, Constraint]) recvFetchCheckpoint(fetch *consensus.FetchCheckpoint) consensusEvent {
	signedCheckpoint, ok := rbft.storeMgr.localCheckpoints[fetch.SequenceNumber]
	// If we can find a checkpoint in corresponding height, just send it back.
	if !ok {
		// If we cannot find it, the requesting node might fell behind a lot
		// send back our latest stable-checkpoint-info to help it to recover
		signedCheckpoint, ok = rbft.storeMgr.localCheckpoints[rbft.chainConfig.H]
		if !ok {
			rbft.logger.Warningf("Replica %d cannot find digest of its low watermark %d, "+
				"current node may fall behind", rbft.chainConfig.SelfID, rbft.chainConfig.H)
			return nil
		}
	}

	payload, err := signedCheckpoint.MarshalVTStrict()
	if err != nil {
		rbft.logger.Errorf("ConsensusMessage_CHECKPOINT Marshal Error: %s", err)
		return nil
	}
	consensusMsg := &consensus.ConsensusMessage{
		Type:    consensus.Type_SIGNED_CHECKPOINT,
		Payload: payload,
	}
	rbft.peerMgr.unicast(context.TODO(), consensusMsg, fetch.ReplicaId)

	return nil
}

func (rbft *rbftImpl[T, Constraint]) turnIntoEpoch() {
	rbft.logger.Trace(consensus.TagNameEpochChange, consensus.TagStageFinish, consensus.TagContentEpochChange{
		Epoch: rbft.chainConfig.EpochInfo.Epoch,
	})

	// validator set has been changed, start a new epoch and check new epoch
	newEpoch, err := rbft.external.GetCurrentEpochInfo()
	if err != nil {
		rbft.logger.Errorf("Replica %d failed to get current epoch from ledger: %v", rbft.chainConfig.SelfID, err)
		rbft.stopNamespace()
		return
	}

	// re-init vc manager and recovery manager as all caches related to view should
	// be reset in new epoch.
	// NOTE!!! all cert caches in storeManager will be clear in move watermark after
	// turnIntoEpoch.
	rbft.stopNewViewTimer()
	rbft.stopFetchViewTimer()
	rbft.vcMgr = newVcManager(rbft.config)
	rbft.recoveryMgr = newRecoveryMgr(rbft.config)

	// set the latest epoch
	rbft.updateEpochInfo(newEpoch)

	// initial view 0 in new epoch.
	rbft.persistNewView(initialNewView, true)
	rbft.logger.Infof("Replica %d persist view=%d after epoch change", rbft.chainConfig.SelfID, rbft.chainConfig.View)

	// clean cached old epoch proof
	for epoch := range rbft.epochMgr.epochProofCache {
		if epoch < newEpoch.Epoch {
			delete(rbft.epochMgr.epochProofCache, epoch)
		}
	}

	rbft.metrics.clusterSizeGauge.Set(float64(rbft.chainConfig.N))
	rbft.metrics.quorumSizeGauge.Set(float64(rbft.commonCaseQuorum()))

	rbft.logger.Debugf("======== Replica %d turn into a new epoch, epoch=%d/N=%d/view=%d/height=%d, new primary=%d",
		rbft.chainConfig.SelfID, rbft.chainConfig.EpochInfo.Epoch, rbft.chainConfig.N, rbft.chainConfig.View, rbft.exec.lastExec, rbft.chainConfig.PrimaryID)
	rbft.logger.Notice(`

  +==============================================+
  |                                              |
  |             RBFT Start New Epoch             |
  |                                              |
  +==============================================+

`)
}

// setEpoch sets the epoch with the epochLock.
func (rbft *rbftImpl[T, Constraint]) updateEpochInfo(epochInfo *EpochInfo) {
	rbft.epochLock.Lock()
	oldRole := rbft.chainConfig.SelfRole
	rbft.chainConfig.EpochInfo = epochInfo
	rbft.epochMgr.epoch = epochInfo.Epoch
	if err := rbft.chainConfig.updateDerivedData(); err != nil {
		rbft.logger.Criticalf("Replica %d failed to check epoch info for epoch %d from ledger: %v", rbft.chainConfig.SelfID, epochInfo.Epoch, err)
		return
	}
	rbft.chainConfig.ResetRecentBlockNum(rbft.config.LastServiceState.MetaState.Height)
	newRole := rbft.chainConfig.SelfRole
	if oldRole != newRole {
		rbft.logger.Infof("Replica %d change role from %s to %s", rbft.chainConfig.SelfID, oldRole.String(), newRole.String())
	}
	rbft.epochLock.Unlock()
	rbft.metrics.epochGauge.Set(float64(epochInfo.Epoch))
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
		em.logger.Debugf("Replica %d received message type %s from %d with larger epoch, "+
			"current epoch %d, remote epoch %d", em.chainConfig.SelfID, msg.Type, msg.From, currentEpoch, remoteEpoch)
		// first process epoch sync response with higher epoch.
		if msg.Type == consensus.Type_EPOCH_CHANGE_PROOF {
			proof := &consensus.EpochChangeProof{}
			if uErr := proof.UnmarshalVT(msg.Payload); uErr != nil {
				em.logger.Warningf("Unmarshal EpochChangeProof failed: %s", uErr)
				return uErr
			}
			return em.processEpochChangeProof(proof)
		}
		return em.retrieveEpochChange(currentEpoch, remoteEpoch, msg.From)
	}

	if remoteEpoch < currentEpoch {
		em.logger.Debugf("Replica %d received message type %s from %d with lower epoch, "+
			"current epoch %d, remote epoch %d", em.chainConfig.SelfID, msg.Type, msg.From, currentEpoch, remoteEpoch)
		// first process epoch sync request with lower epoch.
		if msg.Type == consensus.Type_EPOCH_CHANGE_REQUEST {
			request := &consensus.EpochChangeRequest{}
			if uErr := request.UnmarshalVT(msg.Payload); uErr != nil {
				em.logger.Warningf("Unmarshal EpochChangeRequest failed: %s", uErr)
				return uErr
			}
			return em.processEpochChangeRequest(request)
		}
		em.logger.Warningf("reject process message from %d with lower epoch %d, current epoch %d",
			msg.From, remoteEpoch, currentEpoch)
	}
	return nil
}

func (em *epochManager) retrieveEpochChange(start, target uint64, recipient uint64) error {
	em.logger.Debugf("Replica %d request epoch changes %d to %d from %d", em.chainConfig.SelfID, start, target, recipient)
	req := &consensus.EpochChangeRequest{
		Author:          em.chainConfig.SelfID,
		StartEpoch:      start,
		TargetEpoch:     target,
		AuthorP2PNodeId: em.chainConfig.SelfP2PNodeID,
	}
	payload, mErr := req.MarshalVTStrict()
	if mErr != nil {
		em.logger.Warningf("Marshal EpochChangeRequest failed: %s", mErr)
		return mErr
	}
	cum := &consensus.ConsensusMessage{
		Type:    consensus.Type_EPOCH_CHANGE_REQUEST,
		Payload: payload,
	}
	em.peerMgr.unicast(context.TODO(), cum, recipient)
	return nil
}

func (em *epochManager) processEpochChangeRequest(request *consensus.EpochChangeRequest) error {
	em.logger.Debugf("Replica %d received epoch change request %s", em.chainConfig.SelfID, request)

	if err := em.verifyEpochChangeRequest(request); err != nil {
		em.logger.Warningf("Verify epoch change request failed: %s", err)
		return err
	}

	proof, err := em.pagingGetEpochChangeProof(request.StartEpoch, request.TargetEpoch, MaxNumEpochEndingCheckpoint)
	if err != nil {
		return err
	}
	if proof != nil {
		em.logger.Noticef("Replica %d send epoch change proof towards %d, info %s", em.chainConfig.SelfID, request.GetAuthor(), proof.Pretty())
		proof.GenesisBlockDigest = em.config.GenesisBlockDigest
		payload, mErr := proof.MarshalVTStrict()
		if mErr != nil {
			em.logger.Warningf("Marshal EpochChangeProof failed: %s", mErr)
			return mErr
		}
		cum := &consensus.ConsensusMessage{
			Type:    consensus.Type_EPOCH_CHANGE_PROOF,
			Payload: payload,
		}
		em.peerMgr.unicastByP2PID(context.TODO(), cum, request.AuthorP2PNodeId)
	}

	return nil
}

func (em *epochManager) processEpochChangeProof(proof *consensus.EpochChangeProof) consensusEvent {
	em.logger.Debugf("Replica %d received epoch change proof from %d", em.chainConfig.SelfID, proof.Author)

	if changeTo := proof.NextEpoch(); changeTo <= em.epoch {
		// ignore proof old epoch which we have already started
		em.logger.Debugf("reject lower epoch change to %d", changeTo)
		return nil
	}

	if proof.GenesisBlockDigest != em.config.GenesisBlockDigest {
		em.logger.Criticalf("Replica %d reject epoch change proof, because self genesis config is not consistent with most nodes, expected genesis block hash: %s, self genesis block hash: %s",
			em.chainConfig.SelfID, proof.GenesisBlockDigest, em.config.GenesisBlockDigest)
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
		return errors.New("nil epoch-change-request")
	}
	if request.StartEpoch >= request.TargetEpoch {
		return fmt.Errorf("reject epoch change request for illegal change from %d to %d", request.StartEpoch, request.TargetEpoch)
	}
	if em.epoch < request.TargetEpoch {
		return fmt.Errorf("reject epoch change request for higher target %d from %d", request.TargetEpoch, request.Author)
	}
	return nil
}

// pagingGetEpochChangeProof returns epoch change proof with given page limit.
func (em *epochManager) pagingGetEpochChangeProof(startEpoch, endEpoch, pageLimit uint64) (*consensus.EpochChangeProof, error) {
	pagingEpoch := endEpoch
	more := uint64(0)

	if pagingEpoch-startEpoch > pageLimit {
		more = pagingEpoch
		pagingEpoch = startEpoch + pageLimit
	}

	epochChanges := make([]*consensus.EpochChange, 0)
	for epoch := startEpoch; epoch < pagingEpoch; epoch++ {
		cp, err := em.getEpochQuorumCheckpoint(epoch)
		if err != nil {
			em.logger.Warningf("Cannot find epoch change for epoch %d", epoch)
			return nil, err
		}
		info, err := em.epochService.GetEpochInfo(epoch)
		if err != nil {
			em.logger.Warningf("Cannot find history epoch info for epoch %d", epoch)
			return nil, err
		}

		validators := make([]string, len(info.ValidatorSet))
		for i, nodeInfo := range info.ValidatorSet {
			validators[i] = nodeInfo.P2PNodeID
		}
		epochChanges = append(epochChanges, &consensus.EpochChange{Checkpoint: cp, Validators: validators})
	}

	return &consensus.EpochChangeProof{
		EpochChanges: epochChanges,
		More:         more,
		Author:       em.chainConfig.SelfID,
	}, nil
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
	for _, epc := range proof.EpochChanges {
		if epc.GetCheckpoint().Epoch() >= em.epoch {
			startEpoch = epc.GetCheckpoint().Epoch()
			break
		}
		skip++
	}
	if startEpoch != em.epoch {
		return fmt.Errorf("invalid epoch change proof with start epoch %d, "+
			"current epoch %d", startEpoch, em.epoch)
	}
	// skip smaller epoch
	proof.EpochChanges = proof.EpochChanges[skip:]

	if proof.IsEmpty() {
		return errors.New("empty epoch change proof")
	}
	// verify ValidatorSet when stateUpdate
	// return em.epochService.VerifyEpochChangeProof(proof, em.epochService.GetLastCheckpoint().ValidatorSet())
	return nil
}

// persistEpochQuorumCheckpoint persists QuorumCheckpoint or epoch to database
func (em *epochManager) persistEpochQuorumCheckpoint(c *consensus.QuorumCheckpoint) {
	key := fmt.Sprintf("%s%d", EpochStatePrefix, c.Checkpoint.Epoch)
	raw, err := c.MarshalVTStrict()
	if err != nil {
		em.logger.Errorf("Persist epoch %d quorum chkpt failed with marshal err: %s ", c.Checkpoint.Epoch, err)
		return
	}

	if err = em.epochService.StoreEpochState(key, raw); err != nil {
		em.logger.Errorf("Persist epoch %d quorum chkpt failed with err: %s ", c.Checkpoint.Epoch, err)
	}

	// update latest epoch index
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, c.Checkpoint.Epoch)
	indexKey := EpochIndexKey
	if err = em.epochService.StoreEpochState(indexKey, data); err != nil {
		em.logger.Errorf("Persist epoch index %d failed with err: %s ", c.Checkpoint.Epoch, err)
	}
}

// persistDelCheckpoint get QuorumCheckpoint with the given epoch
func (em *epochManager) getEpochQuorumCheckpoint(epoch uint64) (*consensus.QuorumCheckpoint, error) {
	key := fmt.Sprintf("%s%d", EpochStatePrefix, epoch)

	raw, err := em.epochService.ReadEpochState(key)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to read epoch %d quorum chkpt", epoch)
	}
	c := &consensus.QuorumCheckpoint{}
	if err := c.UnmarshalVT(raw); err != nil {
		return nil, errors.WithMessagef(err, "failed to unmarshal epoch %d quorum chkpt", epoch)
	}

	return c, nil
}
