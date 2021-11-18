package rbft

import (
	"testing"

	"github.com/ultramesh/flato-common/types/protos"
	pb "github.com/ultramesh/flato-rbft/rbftpb"
	"github.com/ultramesh/flato-rbft/types"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestExec_handleCoreRbftEvent_batchTimerEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()

	// start state
	ev := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreBatchTimerEvent,
	}
	assert.False(t, rbfts[0].timerMgr.getTimer(batchTimer))

	// abnormal state
	rbfts[0].handleCoreRbftEvent(ev)
	assert.False(t, rbfts[0].timerMgr.getTimer(batchTimer))

	// normal
	rbfts[0].setNormal()
	rbfts[0].atomicOn(InConfChange)
	rbfts[0].handleCoreRbftEvent(ev)
	assert.True(t, rbfts[0].timerMgr.getTimer(batchTimer))
	rbfts[0].atomicOff(InConfChange)

	tx := newTx()
	rbfts[0].batchMgr.requestPool.AddNewRequests([]*protos.Transaction{tx}, false, true)
	reqBatch := rbfts[0].batchMgr.requestPool.GenerateRequestBatch()
	batch := &pb.RequestBatch{
		RequestHashList: reqBatch[0].TxHashList,
		RequestList:     reqBatch[0].TxList,
		Timestamp:       reqBatch[0].Timestamp,
		LocalList:       reqBatch[0].LocalList,
		BatchHash:       reqBatch[0].BatchHash,
	}
	rbfts[0].batchMgr.cacheBatch = append(rbfts[0].batchMgr.cacheBatch, batch)
	rbfts[0].setNormal()
	rbfts[0].handleCoreRbftEvent(ev)
	assert.Equal(t, pb.Type_PRE_PREPARE, nodes[0].broadcastMessageCache.Type)
}

func TestExec_handleCoreRbftEvent_NullRequestTimerEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()

	// replica will send view change, primary will send null request
	ev := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreNullRequestTimerEvent,
	}
	rbfts[0].handleCoreRbftEvent(ev)
	rbfts[1].handleCoreRbftEvent(ev)
	rbfts[2].handleCoreRbftEvent(ev)
	rbfts[3].handleCoreRbftEvent(ev)
	assert.Equal(t, pb.Type_NULL_REQUEST, nodes[0].broadcastMessageCache.Type)
	assert.Equal(t, pb.Type_VIEW_CHANGE, nodes[1].broadcastMessageCache.Type)
	assert.Equal(t, pb.Type_VIEW_CHANGE, nodes[2].broadcastMessageCache.Type)
	assert.Equal(t, pb.Type_VIEW_CHANGE, nodes[3].broadcastMessageCache.Type)
}

func TestExec_handleCoreRbftEvent_FirstRequestTimerEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()

	// every node will send view change
	ev := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreFirstRequestTimerEvent,
	}
	rbfts[0].handleCoreRbftEvent(ev)
	rbfts[1].handleCoreRbftEvent(ev)
	rbfts[2].handleCoreRbftEvent(ev)
	rbfts[3].handleCoreRbftEvent(ev)
	assert.Equal(t, pb.Type_VIEW_CHANGE, nodes[0].broadcastMessageCache.Type)
	assert.Equal(t, pb.Type_VIEW_CHANGE, nodes[1].broadcastMessageCache.Type)
	assert.Equal(t, pb.Type_VIEW_CHANGE, nodes[2].broadcastMessageCache.Type)
	assert.Equal(t, pb.Type_VIEW_CHANGE, nodes[3].broadcastMessageCache.Type)
}

func TestExec_handleCoreRbftEvent_CheckPoolTimerEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()

	// node will restart check pool timer
	ev := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreCheckPoolTimerEvent,
	}
	assert.False(t, rbfts[0].timerMgr.getTimer(checkPoolTimer))

	// for abnormal node
	rbfts[0].handleCoreRbftEvent(ev)
	assert.False(t, rbfts[0].timerMgr.getTimer(checkPoolTimer))

	// for normal node
	rbfts[0].setNormal()
	rbfts[0].handleCoreRbftEvent(ev)
	assert.True(t, rbfts[0].timerMgr.getTimer(checkPoolTimer))
}

func TestExec_handleCoreRbftEvent_StateUpdatedEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()

	metaS := &types.MetaState{
		Height: uint64(10),
		Digest: "block-number-10",
	}
	epochInfo := &types.EpochInfo{
		Epoch: uint64(0),
		VSet:  defaultValidatorSet,
	}
	ss := &types.ServiceState{
		MetaState: metaS,
		EpochInfo: epochInfo,
	}
	checkpoint := &protos.Checkpoint{
		Epoch:   uint64(0),
		Height:  uint64(10),
		Digest:  "block-number-10",
		NextSet: nil,
	}
	ev := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreStateUpdatedEvent,
		Event:     ss,
	}
	assert.Equal(t, uint64(0), rbfts[1].exec.lastExec)
	rbfts[1].storeMgr.highStateTarget = &stateUpdateTarget{
		metaState: metaS,
		checkpointSet: []*pb.SignedCheckpoint{
			{NodeInfo: &pb.NodeInfo{ReplicaId: 1, ReplicaHash: "test-hash-1"}, Checkpoint: checkpoint, Signature: []byte("sig-1")},
			{NodeInfo: &pb.NodeInfo{ReplicaId: 2, ReplicaHash: "test-hash-2"}, Checkpoint: checkpoint, Signature: []byte("sig-2")},
			{NodeInfo: &pb.NodeInfo{ReplicaId: 3, ReplicaHash: "test-hash-3"}, Checkpoint: checkpoint, Signature: []byte("sig-3")},
		},
	}
	rbfts[1].handleCoreRbftEvent(ev)
	assert.Equal(t, uint64(10), rbfts[1].exec.lastExec)
}

func TestExec_handleCoreRbftEvent_ResendMissingTxsEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()

	// create batch
	tx := newTx()
	rbfts[0].batchMgr.requestPool.AddNewRequests([]*protos.Transaction{tx}, false, true)
	reqBatch := rbfts[0].batchMgr.requestPool.GenerateRequestBatch()
	batch := &pb.RequestBatch{
		RequestHashList: reqBatch[0].TxHashList,
		RequestList:     reqBatch[0].TxList,
		Timestamp:       reqBatch[0].Timestamp,
		LocalList:       reqBatch[0].LocalList,
		BatchHash:       reqBatch[0].BatchHash,
	}
	rbfts[0].storeMgr.batchStore[batch.BatchHash] = batch

	// create pre-prepare
	hashBatch := &pb.HashBatch{
		RequestHashList: batch.RequestHashList,
		Timestamp:       batch.Timestamp,
	}
	prePrep := &pb.PrePrepare{
		ReplicaId:      rbfts[0].peerPool.ID,
		View:           0,
		SequenceNumber: 1,
		BatchDigest:    batch.BatchHash,
		HashBatch:      hashBatch,
	}

	// replica 2 reconstruct batch, find missing tx and create fetch missing request
	_, _, missingTxHashes, _ := rbfts[1].batchMgr.requestPool.GetRequestsByHashList(prePrep.BatchDigest, prePrep.HashBatch.Timestamp, prePrep.HashBatch.RequestHashList, prePrep.HashBatch.DeDuplicateRequestHashList)
	fetch := &pb.FetchMissingRequests{
		View:                 prePrep.View,
		SequenceNumber:       prePrep.SequenceNumber,
		BatchDigest:          prePrep.BatchDigest,
		MissingRequestHashes: missingTxHashes,
		ReplicaId:            rbfts[1].peerPool.ID,
	}

	// time out in primary 1
	ev := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreResendMissingTxsEvent,
		Event:     fetch,
	}
	rbfts[0].handleCoreRbftEvent(ev)
	assert.Equal(t, pb.Type_SEND_MISSING_REQUESTS, nodes[0].unicastMessageCache.Type)
}

func TestExec_handleRecoveryEvent_RestartTimerEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()

	ev := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoveryRestartTimerEvent,
	}
	assert.False(t, rbfts[1].atomicIn(InRecovery))
	rbfts[1].handleRecoveryEvent(ev)
	assert.True(t, rbfts[1].atomicIn(InRecovery))
	assert.Equal(t, uint64(0), rbfts[1].view)
}

func TestExec_handleRecoveryEvent_SyncStateRspTimerEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()

	ev := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoverySyncStateRspTimerEvent,
	}
	assert.False(t, rbfts[1].atomicIn(InRecovery))

	// for abnormal
	rbfts[1].handleRecoveryEvent(ev)
	assert.False(t, rbfts[1].atomicIn(InRecovery))

	// for normal
	rbfts[1].setNormal()
	rbfts[1].handleRecoveryEvent(ev)
	assert.True(t, rbfts[1].atomicIn(InRecovery))
	assert.Equal(t, uint64(1), rbfts[1].view)
}

func TestExec_handleRecoveryEvent_SyncStateRestartTimerEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()

	ev := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoverySyncStateRestartTimerEvent,
	}
	assert.False(t, rbfts[1].timerMgr.getTimer(syncStateRestartTimer))

	rbfts[1].handleRecoveryEvent(ev)
	assert.True(t, rbfts[1].timerMgr.getTimer(syncStateRestartTimer))
}

func TestExec_handleViewChangeEvent_ViewChangeTimerEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()

	ev := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeTimerEvent,
	}

	preView := rbfts[1].view
	rbfts[1].restartRecovery()
	assert.Equal(t, pb.Type_NOTIFICATION, nodes[1].broadcastMessageCache.Type)

	rbfts[1].handleViewChangeEvent(ev)
	assert.Equal(t, pb.Type_NOTIFICATION, nodes[1].broadcastMessageCache.Type)
	assert.Equal(t, preView+1, rbfts[1].view)

	rbfts[2].handleViewChangeEvent(ev)
	assert.Equal(t, pb.Type_VIEW_CHANGE, nodes[2].broadcastMessageCache.Type)
}

func TestExec_handleViewChangeEvent_ViewChangeResendTimerEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()

	ev := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeResendTimerEvent,
	}

	rbfts[1].atomicOn(InViewChange)
	rbfts[1].handleViewChangeEvent(ev)
	assert.Equal(t, pb.Type_NOTIFICATION, nodes[1].broadcastMessageCache.Type)
}
