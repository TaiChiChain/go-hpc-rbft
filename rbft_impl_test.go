package rbft

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hyperchain/go-hpc-common/metrics/disabled"
	commonTypes "github.com/hyperchain/go-hpc-common/types"
	"github.com/hyperchain/go-hpc-common/types/protos"
	mockexternal "github.com/hyperchain/go-hpc-rbft/mock/mock_external"
	pb "github.com/hyperchain/go-hpc-rbft/rbftpb"
	"github.com/hyperchain/go-hpc-rbft/types"
	txpoolmock "github.com/hyperchain/go-hpc-txpool/mock"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

//============================================
// Basic Tools
//============================================

func TestRBFT_newRBFT(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pool := txpoolmock.NewMockTxPool(ctrl)
	log := FrameworkNewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)

	conf := Config{
		ID:                      1,
		Hash:                    calHash("node1"),
		Peers:                   peerSet,
		K:                       10,
		LogMultiplier:           4,
		SetSize:                 25,
		SetTimeout:              100 * time.Millisecond,
		BatchTimeout:            500 * time.Millisecond,
		RequestTimeout:          6 * time.Second,
		NullRequestTimeout:      9 * time.Second,
		VcResendTimeout:         10 * time.Second,
		CleanVCTimeout:          60 * time.Second,
		NewViewTimeout:          8 * time.Second,
		FirstRequestTimeout:     30 * time.Second,
		SyncStateTimeout:        1 * time.Second,
		SyncStateRestartTimeout: 10 * time.Second,
		RecoveryTimeout:         10 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger:      log,
		External:    external,
		RequestPool: pool,
		MetricsProv: &disabled.Provider{},
		DelFlag:     make(chan bool),

		EpochInit:    uint64(0),
		LatestConfig: nil,
	}
	cpChan := make(chan *types.ServiceState)
	confC := make(chan uint64)
	rbft, _ := newRBFT(cpChan, confC, conf)

	// Normal case
	structName, nilElems, err := checkNilElems(rbft)
	if err == nil {
		assert.Equal(t, "rbftImpl", structName)
		assert.Nil(t, nilElems)
	}

	// Nil Peers
	conf.Peers = nil
	_, err = newRBFT(cpChan, confC, conf)
	assert.Equal(t, errors.New("nil peers"), err)

	// Is a New Node
	conf.Peers = peerSet
	conf.ID = 4
	rbft, _ = newRBFT(cpChan, confC, conf)
	assert.Equal(t, 4, rbft.N)
}

//============================================
// Consensus Message Filter
//============================================

func TestRBFT_consensusMessageFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	rbfts[2].initSyncState()
	sync := nodes[2].broadcastMessageCache
	assert.Equal(t, pb.Type_SYNC_STATE, sync.Type)
	sync.Epoch = uint64(5)
	rbfts[1].consensusMessageFilter(context.Background(), sync)
	assert.Equal(t, 0, len(rbfts[1].epochMgr.checkOutOfEpoch))

	tx := newTx()
	rbfts[0].batchMgr.requestPool.AddNewRequests([]*protos.Transaction{tx}, false, true)
	batchTimerEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreBatchTimerEvent,
	}
	rbfts[0].processEvent(context.Background(), batchTimerEvent)
	preprepMsg := nodes[0].broadcastMessageCache
	assert.Equal(t, pb.Type_PRE_PREPARE, preprepMsg.Type)
	preprepMsg.Epoch = uint64(5)
	rbfts[1].consensusMessageFilter(context.Background(), preprepMsg)
	assert.Equal(t, 1, len(rbfts[1].epochMgr.checkOutOfEpoch))
}

//============================================
// Process Request Set
//============================================

func TestRBFT_processReqSetEvent_PrimaryGenerateBatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	// the batch size is 4
	// it means we will generate a batch directly when we receive 4 transactions
	var transactionSet []*protos.Transaction
	for i := 0; i < 4; i++ {
		tx := newTx()
		transactionSet = append(transactionSet, tx)
	}
	req := &pb.RequestSet{
		Local:    true,
		Requests: transactionSet,
	}

	// for primary
	rbfts[0].processEvent(context.Background(), req)
	conMsg := nodes[0].broadcastMessageCache
	assert.Equal(t, pb.Type_PRE_PREPARE, conMsg.Type)
	assert.True(t, rbfts[0].timerMgr.getTimer(batchTimer))
}

func TestRBFT_processReqSetEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	ctx := newCTX(defaultValidatorSet)
	req := &pb.RequestSet{
		Local:    true,
		Requests: []*protos.Transaction{ctx},
	}

	rbfts[1].atomicOn(InConfChange)
	rbfts[1].processEvent(context.Background(), req)
	batch := rbfts[1].batchMgr.requestPool.GenerateRequestBatch()
	assert.NotNil(t, batch)
}

//============================================
// Post Tools
// Test Case: Should receive target Event from rbft.recvChan
//============================================

func TestRBFT_reportStateUpdated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()

	unlockCluster(rbfts)

	state2 := &types.ServiceState{}
	state2.MetaState = &types.MetaState{
		Height: 20,
		Digest: "block-number-20",
	}

	event := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreStateUpdatedEvent,
		Event:     state2,
	}

	rbfts[0].reportStateUpdated(state2)
	w := <-rbfts[0].recvChan
	assert.Equal(t, event, w.event)
}

func TestRBFT_postMsg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	rbfts[0].postMsg(context.Background(), []byte("postMsg"))
	w := <-rbfts[0].recvChan
	assert.Equal(t, []byte("postMsg"), w.event)
}

//============================================
// Get Status
//============================================

func TestRBFT_getStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	var status NodeStatus

	rbfts[0].atomicOn(InViewChange)
	status = rbfts[0].getStatus()
	assert.Equal(t, InViewChange, int(status.Status))
	rbfts[0].atomicOff(InViewChange)

	rbfts[0].atomicOn(InRecovery)
	status = rbfts[0].getStatus()
	assert.Equal(t, InRecovery, int(status.Status))
	rbfts[0].atomicOff(InRecovery)

	rbfts[0].atomicOn(StateTransferring)
	status = rbfts[0].getStatus()
	assert.Equal(t, StateTransferring, int(status.Status))
	rbfts[0].atomicOff(StateTransferring)

	rbfts[0].atomicOn(PoolFull)
	status = rbfts[0].getStatus()
	assert.Equal(t, PoolFull, int(status.Status))
	rbfts[0].atomicOff(PoolFull)

	rbfts[0].atomicOn(Pending)
	status = rbfts[0].getStatus()
	assert.Equal(t, Pending, int(status.Status))
	rbfts[0].atomicOff(Pending)

	rbfts[0].atomicOn(InConfChange)
	status = rbfts[0].getStatus()
	assert.Equal(t, InConfChange, int(status.Status))
	rbfts[0].atomicOff(InConfChange)

	rbfts[0].on(Normal)
	status = rbfts[0].getStatus()
	assert.Equal(t, Normal, int(status.Status))
}

//============================================
// General Event Process Method
//============================================

func TestRBFT_processNullRequset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	nullRequestEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreNullRequestTimerEvent,
	}

	rbfts[0].processEvent(context.Background(), nullRequestEvent)
	nullRequestMsg := nodes[0].broadcastMessageCache
	assert.Equal(t, pb.Type_NULL_REQUEST, nullRequestMsg.Type)

	// If success process it, mode NeedSyncState will on
	rbfts[0].on(Normal)

	rbfts[0].atomicOn(InRecovery)
	rbfts[0].processEvent(context.Background(), nullRequestMsg)
	assert.Equal(t, false, rbfts[0].in(NeedSyncState))
	rbfts[0].atomicOff(InRecovery)

	rbfts[0].atomicOn(InViewChange)
	rbfts[0].processEvent(context.Background(), nullRequestMsg)
	assert.Equal(t, false, rbfts[0].in(NeedSyncState))
	rbfts[0].atomicOff(InViewChange)

	rbfts[0].processEvent(context.Background(), nullRequestMsg)
	assert.Equal(t, true, rbfts[0].in(NeedSyncState))

	// not primary
	rbfts[1].on(NeedSyncState)
	rbfts[0].processEvent(context.Background(), nullRequestMsg)
	event := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreFirstRequestTimerEvent,
	}
	rbfts[1].timerMgr.startTimer(firstRequestTimer, event)
	assert.Equal(t, true, rbfts[1].timerMgr.getTimer(firstRequestTimer))
}

func TestRBFT_handleNullRequestTimerEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()

	rbfts[0].atomicOn(InRecovery)
	rbfts[0].handleNullRequestTimerEvent()
	assert.Equal(t, uint64(0), rbfts[0].view)
	assert.Nil(t, nodes[0].broadcastMessageCache)
	rbfts[0].atomicOff(InRecovery)

	rbfts[0].atomicOn(InViewChange)
	rbfts[0].handleNullRequestTimerEvent()
	assert.Equal(t, uint64(0), rbfts[0].view)
	assert.Nil(t, nodes[0].broadcastMessageCache)
	rbfts[0].atomicOff(InViewChange)

	rbfts[0].setView(uint64(1))
	rbfts[0].handleNullRequestTimerEvent()
	assert.Equal(t, uint64(2), rbfts[0].view)
	assert.Equal(t, pb.Type_VIEW_CHANGE, nodes[0].broadcastMessageCache.Type)
	rbfts[0].atomicOff(InViewChange)

	rbfts[0].setView(uint64(4))
	rbfts[0].handleNullRequestTimerEvent()
	assert.Equal(t, uint64(4), rbfts[0].view)
	assert.Equal(t, pb.Type_NULL_REQUEST, nodes[0].broadcastMessageCache.Type)
}

func TestRBFT_fetchMissingTxs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	prePrep := &pb.PrePrepare{
		SequenceNumber: uint64(1),
	}
	missingTxHashes := map[uint64]string{
		uint64(0): "transaction",
	}
	rbfts[0].fetchMissingTxs(context.Background(), prePrep, missingTxHashes)

	fetch := &pb.FetchMissingRequests{
		View:                 prePrep.View,
		SequenceNumber:       prePrep.SequenceNumber,
		BatchDigest:          prePrep.BatchDigest,
		MissingRequestHashes: missingTxHashes,
		ReplicaId:            uint64(1),
	}
	consensusMsg := rbfts[0].consensusMessagePacker(fetch)
	assert.Equal(t, consensusMsg, nodes[0].unicastMessageCache)
}

func TestRBFT_start(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()
	err := rbfts[0].start()
	assert.Nil(t, err)

	assert.Equal(t, false, rbfts[0].in(Pending))
}

func TestRBFT_postConfState_NormalCase(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()
	vSetNode5 := append(defaultValidatorSet, &protos.NodeInfo{Hostname: "node5", PubKey: []byte("pub-5")})
	r := vSetToRouters(vSetNode5)
	cc := &types.ConfState{
		QuorumRouter: &r,
	}
	rbfts[0].postConfState(cc)
	assert.Equal(t, 5, len(rbfts[0].peerPool.routerMap.HostMap))
}

func TestRBFT_postConfState_NotExitInRouter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()
	newVSet := []*protos.NodeInfo{
		{Hostname: "node2", PubKey: []byte("pub-2")},
		{Hostname: "node3", PubKey: []byte("pub-3")},
		{Hostname: "node4", PubKey: []byte("pub-4")},
		{Hostname: "node5", PubKey: []byte("pub-5")},
	}
	r := vSetToRouters(newVSet)
	cc := &types.ConfState{
		QuorumRouter: &r,
	}
	go rbfts[0].postConfState(cc)
	assert.Equal(t, true, rbfts[0].atomicIn(Pending))
}

func TestRBFT_processOutOfDateReqs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()
	tx := newTx()

	rbfts[1].batchMgr.requestPool.AddNewRequests([]*protos.Transaction{tx}, false, true)
	rbfts[1].setFull()
	rbfts[1].processOutOfDateReqs()
	assert.Equal(t, true, rbfts[1].isPoolFull())

	rbfts[1].setNormal()
	rbfts[1].processOutOfDateReqs()
	assert.Equal(t, false, rbfts[1].isPoolFull())

	// split according to set size when broadcast.
	batch := make([]*protos.Transaction, 100)
	for i := 0; i < 100; i++ {
		batch[i] = newTx()
	}
	rbfts[1].batchMgr.requestPool.AddNewRequests(batch, false, true)
	// sleep to trigger txpool tolerance time.
	time.Sleep(1 * time.Second)
	rbfts[1].processOutOfDateReqs()
	assert.Equal(t, false, rbfts[1].isPoolFull())

	// split according to set mem size when broadcast.
	rbfts[1].flowControl = true
	rbfts[1].flowControlMaxMem = 3 * batch[0].Size()
	rbfts[1].batchMgr.requestPool.AddNewRequests(batch, false, true)
	// sleep to trigger txpool tolerance time.
	time.Sleep(1 * time.Second)
	rbfts[1].processOutOfDateReqs()
	assert.Equal(t, false, rbfts[1].isPoolFull())
}

func TestRBFT_sendNullRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	rbfts[0].sendNullRequest(context.Background())

	nullRequest := &pb.NullRequest{
		ReplicaId: rbfts[0].peerPool.ID,
	}
	consensusMsg := rbfts[0].consensusMessagePacker(nullRequest)
	assert.Equal(t, consensusMsg, nodes[0].broadcastMessageCache)
}

func TestRBFT_recvPrePrepare_WrongDigest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	hashBatch := &pb.HashBatch{
		RequestHashList: []string{"tx-hash"},
	}
	preprep1 := &pb.PrePrepare{
		View:           rbfts[0].view,
		SequenceNumber: uint64(1),
		BatchDigest:    "wrong hash",
		HashBatch:      hashBatch,
		ReplicaId:      rbfts[0].peerPool.ID,
	}
	err := rbfts[1].recvPrePrepare(context.Background(), preprep1)
	assert.Nil(t, err)
	assert.Equal(t, pb.Type_VIEW_CHANGE, nodes[1].broadcastMessageCache.Type)
}

func TestRBFT_recvPrePrepare_EmptyDigest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	hashBatch := &pb.HashBatch{
		RequestHashList: []string{"tx-hash"},
	}
	preprep1 := &pb.PrePrepare{
		View:           rbfts[0].view,
		SequenceNumber: uint64(1),
		BatchDigest:    "",
		HashBatch:      hashBatch,
		ReplicaId:      rbfts[0].peerPool.ID,
	}
	err := rbfts[1].recvPrePrepare(context.Background(), preprep1)
	assert.Nil(t, err)
	assert.Equal(t, pb.Type_VIEW_CHANGE, nodes[1].broadcastMessageCache.Type)
}

func TestRBFT_recvPrePrepare_WrongSeqNo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tx := newTx()
	txHash := commonTypes.GetHash(tx).String()

	nodes, rbfts := newBasicClusterInstance()
	hashBatch := &pb.HashBatch{
		RequestHashList: []string{txHash},
	}
	preprep := &pb.PrePrepare{
		View:           rbfts[0].view,
		SequenceNumber: uint64(1),
		HashBatch:      hashBatch,
		ReplicaId:      rbfts[0].peerPool.ID,
	}
	batchHash := calculateMD5Hash(preprep.HashBatch.RequestHashList, preprep.HashBatch.Timestamp)
	preprep.BatchDigest = batchHash

	err := rbfts[1].recvPrePrepare(context.Background(), preprep)
	assert.Nil(t, err)
	assert.Equal(t, pb.Type_FETCH_MISSING_REQUESTS, nodes[1].unicastMessageCache.Type) // fetching missing tx for preprepare message

	rbfts[1].recvSendMissingTxs(context.Background(), &pb.SendMissingRequests{
		ReplicaId:      rbfts[0].peerPool.ID,
		View:           rbfts[0].view,
		SequenceNumber: uint64(1),
		BatchDigest:    batchHash,
		MissingRequestHashes: map[uint64]string{
			0: txHash,
		},
		MissingRequests: map[uint64]*protos.Transaction{
			0: tx,
		},
	})

	assert.Equal(t, pb.Type_PREPARE, nodes[1].broadcastMessageCache.Type)

	hashBatchWrong := &pb.HashBatch{
		RequestHashList: []string{"tx-hash-wrong"},
	}
	preprepDup := &pb.PrePrepare{
		View:           rbfts[0].view,
		SequenceNumber: uint64(1),
		HashBatch:      hashBatchWrong,
		ReplicaId:      rbfts[0].peerPool.ID,
	}
	preprepDup.BatchDigest = calculateMD5Hash(preprepDup.HashBatch.RequestHashList, preprepDup.HashBatch.Timestamp)
	err = rbfts[1].recvPrePrepare(context.Background(), preprepDup)

	assert.Nil(t, err)
	assert.Equal(t, pb.Type_VIEW_CHANGE, nodes[1].broadcastMessageCache.Type)
}

func TestRBFT_recvFetchMissingTxs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()

	tx := newTx()
	txHash := commonTypes.GetHash(tx).String()

	hashBatch := &pb.HashBatch{
		RequestHashList: []string{txHash},
	}
	preprep := &pb.PrePrepare{
		View:           rbfts[0].view,
		SequenceNumber: uint64(1),
		HashBatch:      hashBatch,
		ReplicaId:      rbfts[0].peerPool.ID,
	}
	preprep.BatchDigest = calculateMD5Hash(preprep.HashBatch.RequestHashList, preprep.HashBatch.Timestamp)

	fetch := &pb.FetchMissingRequests{
		View:                 preprep.View,
		SequenceNumber:       preprep.SequenceNumber,
		BatchDigest:          preprep.BatchDigest,
		MissingRequestHashes: map[uint64]string{0: txHash},
		ReplicaId:            rbfts[1].peerPool.ID,
	}

	err := rbfts[0].recvFetchMissingTxs(context.Background(), fetch)
	assert.Nil(t, err)
	assert.Nil(t, nodes[0].unicastMessageCache)

	rbfts[0].storeMgr.batchStore[fetch.BatchDigest] = &pb.RequestBatch{
		RequestHashList: []string{txHash},
		RequestList:     []*protos.Transaction{tx},
		SeqNo:           uint64(1),
		LocalList:       []bool{true},
	}
	err = rbfts[0].recvFetchMissingTxs(context.Background(), fetch)
	assert.Nil(t, err)
	assert.Equal(t, pb.Type_SEND_MISSING_REQUESTS, nodes[0].unicastMessageCache.Type)

	re := &pb.SendMissingRequests{
		View:                 fetch.View,
		SequenceNumber:       fetch.SequenceNumber,
		BatchDigest:          fetch.BatchDigest,
		MissingRequestHashes: fetch.MissingRequestHashes,
		MissingRequests:      map[uint64]*protos.Transaction{0: tx},
		ReplicaId:            rbfts[0].peerPool.ID,
	}

	var ret consensusEvent
	rbfts[1].exec.lastExec = uint64(10)
	ret = rbfts[1].recvSendMissingTxs(context.Background(), re)
	assert.Nil(t, ret)

	rbfts[1].storeMgr.missingBatchesInFetching[preprep.BatchDigest] = msgID{
		v: preprep.View,
		n: preprep.SequenceNumber,
		d: preprep.BatchDigest,
	}
	rbfts[1].exec.lastExec = uint64(10)
	ret = rbfts[1].recvSendMissingTxs(context.Background(), re)
	assert.Nil(t, ret)

	rbfts[1].exec.lastExec = uint64(0)
	ret = rbfts[1].recvSendMissingTxs(context.Background(), re)
	assert.Nil(t, ret)

	re.MissingRequests = map[uint64]*protos.Transaction{uint64(0): tx}
	rbfts[1].exec.lastExec = uint64(0)
	ret = rbfts[1].recvSendMissingTxs(context.Background(), re)
	assert.Nil(t, ret)

	_ = rbfts[1].recvPrePrepare(context.Background(), preprep)
	ret = rbfts[1].recvSendMissingTxs(context.Background(), re)
	assert.Equal(t, pb.Type_PREPARE, nodes[1].broadcastMessageCache.Type)
	assert.Nil(t, ret)

	cert := rbfts[1].storeMgr.getCert(re.View, re.SequenceNumber, re.BatchDigest)
	cert.sentCommit = true
	ret = rbfts[1].recvSendMissingTxs(context.Background(), re)
	assert.Nil(t, ret)

	rbfts[1].setView(uint64(2))
	ret = rbfts[1].recvSendMissingTxs(context.Background(), re)
	assert.Nil(t, ret)
}
