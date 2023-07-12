package rbft

import (
	"testing"

	consensus "github.com/hyperchain/go-hpc-rbft/v2/common/consensus"
	"github.com/hyperchain/go-hpc-rbft/v2/types"

	"github.com/stretchr/testify/assert"
)

func TestExec_handleCoreRbftEvent_batchTimerEvent(t *testing.T) {
	//ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()

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
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	rbfts[0].batchMgr.requestPool.AddNewRequests([][]byte{txBytes}, false, true)
	reqBatch := rbfts[0].batchMgr.requestPool.GenerateRequestBatch()
	batch := &consensus.RequestBatch{
		RequestHashList: reqBatch[0].TxHashList,
		RequestList:     reqBatch[0].TxList,
		Timestamp:       reqBatch[0].Timestamp,
		LocalList:       reqBatch[0].LocalList,
		BatchHash:       reqBatch[0].BatchHash,
	}
	rbfts[0].batchMgr.cacheBatch = append(rbfts[0].batchMgr.cacheBatch, batch)
	rbfts[0].setNormal()
	rbfts[0].handleCoreRbftEvent(ev)
	assert.Equal(t, consensus.Type_PRE_PREPARE, nodes[0].broadcastMessageCache.Type)
}

func TestExec_handleCoreRbftEvent_NullRequestTimerEvent(t *testing.T) {
	//ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()

	// replica will send view change, primary will send null request
	ev := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreNullRequestTimerEvent,
	}
	rbfts[0].handleCoreRbftEvent(ev)
	rbfts[1].handleCoreRbftEvent(ev)
	rbfts[2].handleCoreRbftEvent(ev)
	rbfts[3].handleCoreRbftEvent(ev)
	assert.Equal(t, consensus.Type_NULL_REQUEST, nodes[0].broadcastMessageCache.Type)
	assert.Equal(t, consensus.Type_VIEW_CHANGE, nodes[1].broadcastMessageCache.Type)
	assert.Equal(t, consensus.Type_VIEW_CHANGE, nodes[2].broadcastMessageCache.Type)
	assert.Equal(t, consensus.Type_VIEW_CHANGE, nodes[3].broadcastMessageCache.Type)
}

func TestExec_handleCoreRbftEvent_CheckPoolTimerEvent(t *testing.T) {
	//ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()

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
	//ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()

	metaS := &types.MetaState{
		Height: uint64(10),
		Digest: "block-number-10",
	}

	ss := &types.ServiceState{
		MetaState: metaS,
		Epoch:     uint64(0),
	}
	checkpoint := &consensus.Checkpoint{
		Epoch: uint64(0),
		ExecuteState: &consensus.Checkpoint_ExecuteState{
			Height: uint64(10),
			Digest: "block-number-10",
		},
	}
	ev := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreStateUpdatedEvent,
		Event:     ss,
	}
	assert.Equal(t, uint64(0), rbfts[1].exec.lastExec)
	rbfts[1].storeMgr.highStateTarget = &stateUpdateTarget{
		metaState: metaS,
		checkpointSet: []*consensus.SignedCheckpoint{
			{Author: "node1", Checkpoint: checkpoint, Signature: []byte("sig-1")},
			{Author: "node2", Checkpoint: checkpoint, Signature: []byte("sig-2")},
			{Author: "node3", Checkpoint: checkpoint, Signature: []byte("sig-3")},
		},
	}
	rbfts[1].handleCoreRbftEvent(ev)
	assert.Equal(t, uint64(10), rbfts[1].exec.lastExec)
}

func TestExec_handleRecoveryEvent_SyncStateRspTimerEvent(t *testing.T) {
	//ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()

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
	//ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()

	ev := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoverySyncStateRestartTimerEvent,
	}
	assert.False(t, rbfts[1].timerMgr.getTimer(syncStateRestartTimer))

	rbfts[1].handleRecoveryEvent(ev)
	assert.False(t, rbfts[1].timerMgr.getTimer(syncStateRestartTimer))
}

func TestExec_handleViewChangeEvent_ViewChangeTimerEvent(t *testing.T) {
	//ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()

	ev := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeTimerEvent,
	}

	preView := rbfts[1].view
	rbfts[1].sendViewChange()
	assert.Equal(t, consensus.Type_VIEW_CHANGE, nodes[1].broadcastMessageCache.Type)

	rbfts[1].handleViewChangeEvent(ev)
	assert.Equal(t, consensus.Type_VIEW_CHANGE, nodes[1].broadcastMessageCache.Type)
	assert.Equal(t, preView+1, rbfts[1].view)

	rbfts[2].handleViewChangeEvent(ev)
	assert.Equal(t, consensus.Type_VIEW_CHANGE, nodes[2].broadcastMessageCache.Type)
}

func TestExec_handleViewChangeEvent_ViewChangeResendTimerEvent(t *testing.T) {
	//ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()

	ev := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeResendTimerEvent,
	}

	rbfts[1].atomicOn(InViewChange)
	rbfts[1].handleViewChangeEvent(ev)
	assert.Equal(t, consensus.Type_VIEW_CHANGE, nodes[1].broadcastMessageCache.Type)
}

func TestExec_handleEpochMgrEvent_FetchCheckpointEvent(t *testing.T) {
	//ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()

	ev := &LocalEvent{
		Service:   EpochMgrService,
		EventType: FetchCheckpointEvent,
	}

	rbfts[1].handleEpochMgrEvent(ev)

	rbfts[1].epochMgr.configBatchToCheck = &types.MetaState{
		Height: 10,
		Digest: "block-hash-10",
	}
	rbfts[1].handleEpochMgrEvent(ev)
	assert.Equal(t, consensus.Type_FETCH_CHECKPOINT, nodes[1].broadcastMessageCache.Type)
}
