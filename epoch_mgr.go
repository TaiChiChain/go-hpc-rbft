package rbft

import (
	"context"
	"sync"

	"github.com/gogo/protobuf/proto"

	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/types"
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
	epochService EpochService

	// peer pool
	peerMgr *peerManager

	// logger
	logger Logger
}

func newEpochManager(c Config, pp *peerManager, epochService EpochService) *epochManager {
	em := &epochManager{
		configBatchToCheck:   nil,
		configBatchToExecute: uint64(0),
		epochService:         epochService,
		peerMgr:              pp,
		logger:               c.Logger,
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
			rbft.peerMgr.selfID, et.GetAuthor())
	case *consensus.EpochChangeProof:
		rbft.logger.Debugf("Replica %d don't process epoch change proof from %s in same epoch",
			rbft.peerMgr.selfID, et.GetAuthor())
	}
	return nil
}

func (rbft *rbftImpl[T, Constraint]) fetchCheckpoint() consensusEvent {
	if rbft.epochMgr.configBatchToCheck == nil {
		rbft.logger.Debugf("Replica %d doesn't need to check any batches", rbft.peerMgr.selfID)
		return nil
	}

	fetch := &consensus.FetchCheckpoint{
		ReplicaId:      rbft.peerMgr.selfID,
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

	rbft.logger.Debugf("Replica %d is fetching checkpoint %d", rbft.peerMgr.selfID, fetch.SequenceNumber)
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
				"current node may fall behind", rbft.peerMgr.selfID, rbft.chainConfig.H)
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
		rbft.logger.Errorf("Replica %d failed to get current epoch from ledger: %v", rbft.peerMgr.selfID, err)
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
	rbft.setEpochInfo(newEpoch)

	// initial view 0 in new epoch.
	rbft.persistNewView(initialNewView)
	rbft.logger.Infof("Replica %d persist view=%d after epoch change", rbft.peerMgr.selfID, rbft.chainConfig.View)

	// update N/f

	rbft.metrics.clusterSizeGauge.Set(float64(rbft.chainConfig.N))
	rbft.metrics.quorumSizeGauge.Set(float64(rbft.commonCaseQuorum()))

	rbft.logger.Debugf("======== Replica %d turn into a new epoch, epoch=%d/N=%d/view=%d/height=%d.",
		rbft.peerMgr.selfID, rbft.chainConfig.EpochInfo.Epoch, rbft.chainConfig.N, rbft.chainConfig.View, rbft.exec.lastExec)
	rbft.logger.Notice(`

  +==============================================+
  |                                              |
  |             RBFT Start New Epoch             |
  |                                              |
  +==============================================+

`)
}

// setEpoch sets the epoch with the epochLock.
func (rbft *rbftImpl[T, Constraint]) setEpochInfo(epochInfo *EpochInfo) {
	rbft.epochLock.Lock()
	rbft.chainConfig.EpochInfo = epochInfo
	rbft.epochMgr.epoch = epochInfo.Epoch
	rbft.chainConfig.updateDerivedData()
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

// TODO: refactor epoch sync, use state sync instead of epoch change proof
// checkEpoch compares local epoch and remote epoch:
// 1. remoteEpoch > currentEpoch, only accept EpochChangeProof, else retrieveEpochChange
// 2. remoteEpoch < currentEpoch, only accept EpochChangeRequest, else ignore
func (em *epochManager) checkEpoch(msg *consensus.ConsensusMessage) consensusEvent {
	return nil
}
