package rbft

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/common/metrics/disabled"
	mockexternal "github.com/axiomesh/axiom-bft/mock/mock_external"
	txpoolmock "github.com/axiomesh/axiom-bft/txpool/mock"
	"github.com/axiomesh/axiom-bft/types"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

// ============================================
// Basic Tools
// ============================================
func newMockConfig[T any, Constraint consensus.TXConstraint[T]](t *testing.T, ctrl *gomock.Controller) Config[T, Constraint] {
	pool := txpoolmock.NewMockTxPool[T, Constraint](ctrl)
	log := newRawLogger()
	external := mockexternal.NewMockMinimalExternal[T, Constraint](ctrl)

	conf := Config[T, Constraint]{
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
		SyncStateTimeout:        1 * time.Second,
		SyncStateRestartTimeout: 10 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger:      log,
		External:    external,
		RequestPool: pool,
		MetricsProv: &disabled.Provider{},
		DelFlag:     make(chan bool),

		EpochInit:    uint64(0),
		LatestConfig: nil,
	}
	return conf
}
func TestRBFT_newRBFT(t *testing.T) {
	ctrl := gomock.NewController(t)
	//defer ctrl.Finish()
	conf := newMockConfig[consensus.Transaction](t, ctrl)
	rbft, _ := newRBFT[consensus.Transaction](conf)

	// Normal case
	structName, nilElems, err := checkNilElems(rbft)
	if err == nil {
		assert.Contains(t, structName, "rbftImpl")
		assert.Nil(t, nilElems)
	}

	// Nil Peers
	conf.Peers = nil
	_, err = newRBFT(conf)
	assert.Equal(t, errors.New("nil peers"), err)

	// Is a New Node
	conf.Peers = peerSet
	conf.ID = 4
	rbft, _ = newRBFT(conf)
	assert.Equal(t, 4, rbft.N)
}

//============================================
// Consensus Message Filter
//============================================

func TestRBFT_consensusMessageFilter(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	unlockCluster(rbfts)

	rbfts[2].initSyncState()
	sync := nodes[2].broadcastMessageCache
	assert.Equal(t, consensus.Type_SYNC_STATE, sync.Type)
	sync.Epoch = uint64(5)
	rbfts[1].consensusMessageFilter(context.TODO(), sync.ConsensusMessage)

	tx := newTx()
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	rbfts[0].batchMgr.requestPool.AddNewRequests([][]byte{txBytes}, false, true)
	batchTimerEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreBatchTimerEvent,
	}
	rbfts[0].processEvent(batchTimerEvent)
	preprepMsg := nodes[0].broadcastMessageCache
	assert.Equal(t, consensus.Type_PRE_PREPARE, preprepMsg.Type)
	preprepMsg.Epoch = uint64(5)
	rbfts[1].consensusMessageFilter(context.TODO(), preprepMsg.ConsensusMessage)
}

//============================================
// Process Request Set
//============================================

func TestRBFT_processReqSetEvent_PrimaryGenerateBatch(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	unlockCluster(rbfts)

	// the batch size is 4
	// it means we will generate a batch directly when we receive 4 transactions
	var transactionSet [][]byte
	for i := 0; i < 4; i++ {
		tx := newTx()
		txBytes, err := tx.Marshal()
		assert.Nil(t, err)
		transactionSet = append(transactionSet, txBytes)
	}
	req := &consensus.RequestSet{
		Local:    true,
		Requests: transactionSet,
	}

	// for primary
	rbfts[0].processEvent(req)
	conMsg := nodes[0].broadcastMessageCache
	assert.Equal(t, consensus.Type_PRE_PREPARE, conMsg.Type)
	assert.True(t, rbfts[0].timerMgr.getTimer(batchTimer))
}

func TestRBFT_processReqSetEvent(t *testing.T) {

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()
	unlockCluster(rbfts)

	ctx := newCTX(defaultValidatorSet)
	ctxBytes, err := ctx.Marshal()
	assert.Nil(t, err)
	req := &consensus.RequestSet{
		Local:    true,
		Requests: [][]byte{ctxBytes},
	}

	rbfts[1].atomicOn(InConfChange)
	rbfts[1].processEvent(req)
	batch := rbfts[1].batchMgr.requestPool.GenerateRequestBatch()
	assert.NotNil(t, batch)
}

//============================================
// Post Tools
// Test Case: Should receive target Event from rbft.recvChan
//============================================

func TestRBFT_reportStateUpdated(t *testing.T) {

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()

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
	obj := <-rbfts[0].recvChan
	assert.Equal(t, event, obj)
}

func TestRBFT_reportStableCheckpointFinished(t *testing.T) {

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()
	// in pending
	rbfts[0].reportStableCheckpointFinished(20)
	unlockCluster(rbfts)
	// not in conf change
	rbfts[0].reportStableCheckpointFinished(20)
	rbfts[0].atomicOn(InConfChange)
	rbfts[0].reportStableCheckpointFinished(20)
	obj := <-rbfts[0].confChan
	assert.EqualValues(t, 20, obj)
}

func TestRBFT_postMsg(t *testing.T) {

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()
	unlockCluster(rbfts)

	rbfts[0].postMsg([]byte("postMsg"))
	obj := <-rbfts[0].recvChan
	assert.Equal(t, []byte("postMsg"), obj)
}

//============================================
// Get Status
//============================================

func TestRBFT_getStatus(t *testing.T) {

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()
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

	rbfts[0].atomicOn(inEpochSyncing)
	status = rbfts[0].getStatus()
	assert.Equal(t, InConfChange, int(status.Status))
	rbfts[0].atomicOff(inEpochSyncing)

	rbfts[0].on(Normal)
	status = rbfts[0].getStatus()
	assert.Equal(t, Normal, int(status.Status))
}

//============================================
// General Event Process Method
//============================================

func TestRBFT_processNullRequset(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	nullRequestEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreNullRequestTimerEvent,
	}

	rbfts[0].processEvent(nullRequestEvent)
	nullRequestMsg := nodes[0].broadcastMessageCache
	assert.Equal(t, consensus.Type_NULL_REQUEST, nullRequestMsg.Type)

	// If success process it, mode NeedSyncState will on
	rbfts[0].on(Normal)

	rbfts[0].atomicOn(InViewChange)
	rbfts[0].processEvent(nullRequestMsg)
	assert.Equal(t, false, rbfts[0].in(NeedSyncState))
	rbfts[0].atomicOff(InViewChange)

	rbfts[0].processEvent(nullRequestMsg)
	assert.Equal(t, true, rbfts[0].in(NeedSyncState))
}

func TestRBFT_handleNullRequestTimerEvent(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()

	rbfts[0].atomicOn(InViewChange)
	rbfts[0].handleNullRequestTimerEvent()
	assert.Equal(t, uint64(0), rbfts[0].view)
	assert.Nil(t, nodes[0].broadcastMessageCache)
	rbfts[0].atomicOff(InViewChange)

	rbfts[0].setView(uint64(1))
	rbfts[0].handleNullRequestTimerEvent()
	assert.Equal(t, uint64(2), rbfts[0].view)
	assert.Equal(t, consensus.Type_VIEW_CHANGE, nodes[0].broadcastMessageCache.Type)
	rbfts[0].atomicOff(InViewChange)

	rbfts[0].setView(uint64(4))
	rbfts[0].handleNullRequestTimerEvent()
	assert.Equal(t, uint64(4), rbfts[0].view)
	assert.Equal(t, consensus.Type_NULL_REQUEST, nodes[0].broadcastMessageCache.Type)
}

func TestRBFT_fetchMissingTxs(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	prePrep := &consensus.PrePrepare{
		SequenceNumber: uint64(1),
	}
	missingTxHashes := map[uint64]string{
		uint64(0): "transaction",
	}
	rbfts[0].fetchMissingTxs(context.TODO(), prePrep, missingTxHashes)

	fetch := &consensus.FetchMissingRequest{
		View:                 prePrep.View,
		SequenceNumber:       prePrep.SequenceNumber,
		BatchDigest:          prePrep.BatchDigest,
		MissingRequestHashes: missingTxHashes,
		ReplicaId:            uint64(1),
	}
	consensusMsg := rbfts[0].consensusMessagePacker(fetch)
	assert.Equal(t, consensusMsg, nodes[0].unicastMessageCache.ConsensusMessage)
}

func TestRBFT_start_cache_message(t *testing.T) {

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()
	assert.Equal(t, true, rbfts[0].atomicIn(Pending))
	// unknown author.
	errorMsg1 := &consensus.ConsensusMessage{
		Epoch: 1,
		From:  10,
		Type:  consensus.Type_VIEW_CHANGE,
	}
	rbfts[0].step(context.TODO(), errorMsg1)

	// incorrect epoch
	errorMsg2 := &consensus.ConsensusMessage{
		Epoch: 10,
		From:  2,
		Type:  consensus.Type_VIEW_CHANGE,
	}
	rbfts[0].step(context.TODO(), errorMsg2)

	err := rbfts[0].start()
	assert.Nil(t, err)
	assert.Equal(t, false, rbfts[0].atomicIn(Pending))

	rbfts[0].atomicOn(inEpochSyncing)
	correctMsg := &consensus.ConsensusMessage{
		From:  2,
		Epoch: 1,
		Type:  consensus.Type_NULL_REQUEST,
	}
	rbfts[0].step(context.TODO(), correctMsg)

	rs := &consensus.RequestSet{}
	rbfts[0].postRequests(rs)
}

func TestRBFT_postConfState_NormalCase(t *testing.T) {

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()
	vSetNode5 := append(defaultValidatorSet, &consensus.NodeInfo{Hostname: "node5", PubKey: []byte("pub-5")})
	r := vSetToRouters(vSetNode5)
	cc := &types.ConfState{
		QuorumRouter: &r,
	}
	rbfts[0].postConfState(cc)
	assert.Equal(t, 5, len(rbfts[0].peerPool.router))
}

func TestRBFT_postConfState_NotExitInRouter(t *testing.T) {

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()
	newVSet := []*consensus.NodeInfo{
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

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()
	tx := newTx()
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	rbfts[1].batchMgr.requestPool.AddNewRequests([][]byte{txBytes}, false, true)
	rbfts[1].setFull()
	rbfts[1].processOutOfDateReqs()
	assert.Equal(t, true, rbfts[1].isPoolFull())

	rbfts[1].setNormal()
	rbfts[1].processOutOfDateReqs()
	assert.Equal(t, false, rbfts[1].isPoolFull())

	// split according to set size when broadcast.
	batch := make([][]byte, 100)
	for i := 0; i < 100; i++ {
		transaction := newTx()
		data, err := transaction.Marshal()
		assert.Nil(t, err)
		batch[i] = data
	}
	rbfts[1].batchMgr.requestPool.AddNewRequests(batch, false, true)
	// sleep to trigger txpool tolerance time.
	time.Sleep(1 * time.Second)
	rbfts[1].processOutOfDateReqs()
	assert.Equal(t, false, rbfts[1].isPoolFull())

	// split according to set mem size when broadcast.
	rbfts[1].flowControl = true
	//rbfts[1].flowControlMaxMem = 3 * batch[0].Size()
	rbfts[1].batchMgr.requestPool.AddNewRequests(batch, false, true)
	// sleep to trigger txpool tolerance time.
	time.Sleep(1 * time.Second)
	rbfts[1].processOutOfDateReqs()
	assert.Equal(t, false, rbfts[1].isPoolFull())
}

func TestRBFT_sendNullRequest(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()

	rbfts[0].atomicOn(InConfChange)
	rbfts[0].sendNullRequest()
	rbfts[0].atomicOff(InConfChange)

	rbfts[0].sendNullRequest()

	nullRequest := &consensus.NullRequest{
		ReplicaId: rbfts[0].peerPool.ID,
	}
	consensusMsg := rbfts[0].consensusMessagePacker(nullRequest)
	assert.Equal(t, consensusMsg, nodes[0].broadcastMessageCache.ConsensusMessage)

	nullRequest.ReplicaId = 2
	rbfts[1].atomicOn(InConfChange)
	rbfts[1].recvNullRequest(nullRequest)
	rbfts[1].atomicOff(InConfChange)

	rbfts[1].recvNullRequest(nullRequest)
}

func TestRBFT_recvPrePrepare_WrongDigest(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	hashBatch := &consensus.HashBatch{
		RequestHashList: []string{"tx-hash"},
	}
	preprep1 := &consensus.PrePrepare{
		View:           rbfts[0].view,
		SequenceNumber: uint64(1),
		BatchDigest:    "wrong hash",
		HashBatch:      hashBatch,
		ReplicaId:      rbfts[0].peerPool.ID,
	}
	err := rbfts[1].recvPrePrepare(context.TODO(), preprep1)
	assert.Nil(t, err)
	assert.Equal(t, consensus.Type_VIEW_CHANGE, nodes[1].broadcastMessageCache.Type)
}

func TestRBFT_recvPrePrepare_EmptyDigest(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	hashBatch := &consensus.HashBatch{
		RequestHashList: []string{"tx-hash"},
	}
	preprep1 := &consensus.PrePrepare{
		View:           rbfts[0].view,
		SequenceNumber: uint64(1),
		BatchDigest:    "",
		HashBatch:      hashBatch,
		ReplicaId:      rbfts[0].peerPool.ID,
	}
	err := rbfts[1].recvPrePrepare(context.TODO(), preprep1)
	assert.Nil(t, err)
	assert.Equal(t, consensus.Type_VIEW_CHANGE, nodes[1].broadcastMessageCache.Type)
}

func TestRBFT_recvPrePrepare_WrongSeqNo(t *testing.T) {

	tx := newTx()
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	txHash := requestHash[consensus.Transaction](txBytes)

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	hashBatch := &consensus.HashBatch{
		RequestHashList: []string{txHash},
	}
	preprep := &consensus.PrePrepare{
		View:           rbfts[0].view,
		SequenceNumber: uint64(1),
		HashBatch:      hashBatch,
		ReplicaId:      rbfts[0].peerPool.ID,
	}
	batchHash := calculateMD5Hash(preprep.HashBatch.RequestHashList, preprep.HashBatch.Timestamp)
	preprep.BatchDigest = batchHash

	err = rbfts[1].recvPrePrepare(context.TODO(), preprep)
	assert.Nil(t, err)
	assert.Equal(t, consensus.Type_FETCH_MISSING_REQUEST, nodes[1].unicastMessageCache.Type) // fetching missing tx for preprepare message

	rbfts[1].recvFetchMissingResponse(context.TODO(), &consensus.FetchMissingResponse{
		ReplicaId:      rbfts[0].peerPool.ID,
		View:           rbfts[0].view,
		SequenceNumber: uint64(1),
		BatchDigest:    batchHash,
		MissingRequestHashes: map[uint64]string{
			0: txHash,
		},
		MissingRequests: map[uint64][]byte{
			0: txBytes,
		},
	})

	assert.Equal(t, consensus.Type_PREPARE, nodes[1].broadcastMessageCache.Type)

	hashBatchWrong := &consensus.HashBatch{
		RequestHashList: []string{"tx-hash-wrong"},
	}
	preprepDup := &consensus.PrePrepare{
		View:           rbfts[0].view,
		SequenceNumber: uint64(1),
		HashBatch:      hashBatchWrong,
		ReplicaId:      rbfts[0].peerPool.ID,
	}
	preprepDup.BatchDigest = calculateMD5Hash(preprepDup.HashBatch.RequestHashList, preprepDup.HashBatch.Timestamp)
	err = rbfts[1].recvPrePrepare(context.TODO(), preprepDup)

	assert.Nil(t, err)
	assert.Equal(t, consensus.Type_VIEW_CHANGE, nodes[1].broadcastMessageCache.Type)
}

func TestRBFT_recvFetchMissingResponse(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()

	tx := newTx()
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	txHash := requestHash[consensus.Transaction](txBytes)

	hashBatch := &consensus.HashBatch{
		RequestHashList: []string{txHash},
	}
	preprep := &consensus.PrePrepare{
		View:           rbfts[0].view,
		SequenceNumber: uint64(1),
		HashBatch:      hashBatch,
		ReplicaId:      rbfts[0].peerPool.ID,
	}
	preprep.BatchDigest = calculateMD5Hash(preprep.HashBatch.RequestHashList, preprep.HashBatch.Timestamp)

	fetch := &consensus.FetchMissingRequest{
		View:                 preprep.View,
		SequenceNumber:       preprep.SequenceNumber,
		BatchDigest:          preprep.BatchDigest,
		MissingRequestHashes: map[uint64]string{0: txHash},
		ReplicaId:            rbfts[1].peerPool.ID,
	}

	err = rbfts[0].recvFetchMissingRequest(context.TODO(), fetch)
	assert.Nil(t, err)
	assert.Nil(t, nodes[0].unicastMessageCache)

	rbfts[0].storeMgr.batchStore[fetch.BatchDigest] = &consensus.RequestBatch{
		RequestHashList: []string{txHash},
		RequestList:     [][]byte{txBytes},
		SeqNo:           uint64(1),
		LocalList:       []bool{true},
	}
	err = rbfts[0].recvFetchMissingRequest(context.TODO(), fetch)
	assert.Nil(t, err)
	assert.Equal(t, consensus.Type_FETCH_MISSING_RESPONSE, nodes[0].unicastMessageCache.Type)

	re := &consensus.FetchMissingResponse{
		View:                 fetch.View,
		SequenceNumber:       fetch.SequenceNumber,
		BatchDigest:          fetch.BatchDigest,
		MissingRequestHashes: fetch.MissingRequestHashes,
		MissingRequests:      map[uint64][]byte{0: txBytes},
		ReplicaId:            rbfts[0].peerPool.ID,
	}

	var ret consensusEvent
	rbfts[1].exec.lastExec = uint64(10)
	ret = rbfts[1].recvFetchMissingResponse(context.TODO(), re)
	assert.Nil(t, ret)

	rbfts[1].storeMgr.missingBatchesInFetching[preprep.BatchDigest] = msgID{
		v: preprep.View,
		n: preprep.SequenceNumber,
		d: preprep.BatchDigest,
	}
	rbfts[1].exec.lastExec = uint64(10)
	ret = rbfts[1].recvFetchMissingResponse(context.TODO(), re)
	assert.Nil(t, ret)

	rbfts[1].exec.lastExec = uint64(0)
	ret = rbfts[1].recvFetchMissingResponse(context.TODO(), re)
	assert.Nil(t, ret)

	re.MissingRequestHashes = nil
	ret = rbfts[1].recvFetchMissingResponse(context.TODO(), re)
	assert.Nil(t, ret)
	re.MissingRequestHashes = fetch.MissingRequestHashes

	re.View = 2
	ret = rbfts[1].recvFetchMissingResponse(context.TODO(), re)
	assert.Nil(t, ret)
	re.View = fetch.View

	re.ReplicaId = 3
	ret = rbfts[1].recvFetchMissingResponse(context.TODO(), re)
	assert.Nil(t, ret)
	re.ReplicaId = 1

	re.MissingRequests = map[uint64][]byte{0: txBytes}
	rbfts[1].exec.lastExec = uint64(0)
	ret = rbfts[1].recvFetchMissingResponse(context.TODO(), re)
	assert.Nil(t, ret)

	_ = rbfts[1].recvPrePrepare(context.TODO(), preprep)
	ret = rbfts[1].recvFetchMissingResponse(context.TODO(), re)
	assert.Equal(t, consensus.Type_PREPARE, nodes[1].broadcastMessageCache.Type)
	assert.Nil(t, ret)

	cert := rbfts[1].storeMgr.getCert(re.View, re.SequenceNumber, re.BatchDigest)
	cert.sentCommit = true
	ret = rbfts[1].recvFetchMissingResponse(context.TODO(), re)
	assert.Nil(t, ret)

	rbfts[1].setView(uint64(2))
	ret = rbfts[1].recvFetchMissingResponse(context.TODO(), re)
	assert.Nil(t, ret)
}
