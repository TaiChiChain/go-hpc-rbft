package rbft

import (
	"context"
	"testing"
	"time"

	types2 "github.com/axiomesh/axiom-kit/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/axiomesh/axiom-bft/common"
	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/common/metrics/disabled"
	"github.com/axiomesh/axiom-bft/types"
)

// ============================================
// Basic Tools
// ============================================
// todo: mock pool
func newMockRbft[T any, Constraint types2.TXConstraint[T]](t *testing.T, ctrl *gomock.Controller) *rbftImpl[T, Constraint] {
	log := common.NewSimpleLogger()
	external := NewMockMinimalExternal[T, Constraint](ctrl)

	conf := Config{
		SelfAccountAddress: "node1",
		GenesisEpochInfo: &EpochInfo{
			Version:                   1,
			Epoch:                     1,
			EpochPeriod:               1000,
			CandidateSet:              []NodeInfo{},
			ValidatorSet:              peerSet,
			StartBlock:                1,
			P2PBootstrapNodeAddresses: []string{"1"},
			ConsensusParams: ConsensusParams{
				ValidatorElectionType:         ValidatorElectionTypeWRF,
				ProposerElectionType:          ProposerElectionTypeAbnormalRotation,
				CheckpointPeriod:              10,
				HighWatermarkCheckpointPeriod: 4,
				MaxValidatorNum:               10,
				BlockMaxTxNum:                 500,
				NotActiveWeight:               1,
				AbnormalNodeExcludeView:       10,
			},
		},
		LastServiceState: &types.ServiceState{
			MetaState: &types.MetaState{},
			Epoch:     1,
		},
		SetSize:                 25,
		BatchTimeout:            500 * time.Millisecond,
		RequestTimeout:          6 * time.Second,
		NullRequestTimeout:      9 * time.Second,
		VcResendTimeout:         10 * time.Second,
		CleanVCTimeout:          60 * time.Second,
		NewViewTimeout:          8 * time.Second,
		SyncStateTimeout:        1 * time.Second,
		SyncStateRestartTimeout: 10 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger: log,

		MetricsProv: &disabled.Provider{},
		DelFlag:     make(chan bool),
	}
	// todo: mock pool
	rbft, err := newRBFT[T, Constraint](conf, external, nil, true)
	if err != nil {
		panic(err)
	}
	return rbft
}

func TestRBFT_newRBFT(t *testing.T) {
	ctrl := gomock.NewController(t)
	rbft := newMockRbft[consensus.FltTransaction, *consensus.FltTransaction](t, ctrl)

	// Normal case
	structName, nilElems, err := checkNilElems(rbft)
	if err == nil {
		assert.Contains(t, structName, "rbftImpl")
		assert.Nil(t, nilElems)
	}

	// Nil Peers
	rbft.config.GenesisEpochInfo.ValidatorSet = []NodeInfo{}
	_, err = newRBFT(rbft.config, rbft.external, rbft.batchMgr.requestPool, true)
	assert.Error(t, err)

	// Is a New Node
	rbft.config.GenesisEpochInfo.ValidatorSet = peerSet
	rbft.config.SelfAccountAddress = "node4"
	rbft, _ = newRBFT(rbft.config, rbft.external, rbft.batchMgr.requestPool, true)
	assert.Equal(t, 4, rbft.chainConfig.N)
}

// ============================================
// Consensus Message Filter
// ============================================
func TestRBFT_consensusMessageFilter(t *testing.T) {
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	unlockCluster(rbfts)

	rbfts[2].initSyncState()
	sync := nodes[2].broadcastMessageCache
	assert.Equal(t, consensus.Type_SYNC_STATE, sync.Type)
	sync.Epoch = uint64(5)
	rbfts[1].consensusMessageFilter(context.TODO(), sync, sync.ConsensusMessage)

	tx := newTx()
	err := rbfts[0].batchMgr.requestPool.AddLocalTx(tx)
	assert.Nil(t, err)
	batchTimerEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreBatchTimerEvent,
	}
	rbfts[0].processEvent(batchTimerEvent)
	preprepMsg := nodes[0].broadcastMessageCache
	assert.Equal(t, consensus.Type_PRE_PREPARE, preprepMsg.Type)
	preprepMsg.Epoch = uint64(5)
	rbfts[1].consensusMessageFilter(context.TODO(), preprepMsg, preprepMsg.ConsensusMessage)
}

// ============================================
// Process Request Set
// ============================================
func TestRBFT_processReqSetEvent_PrimaryGenerateBatch(t *testing.T) {
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	unlockCluster(rbfts)

	// the batch size is 4
	// it means we will generate a batch directly when we receive 4 transactions
	var transactionSet []*consensus.FltTransaction
	for i := 0; i < 500; i++ {
		tx := newTx()
		transactionSet = append(transactionSet, tx)
	}

	// for primary
	rbfts[0].batchMgr.requestPool.AddRemoteTxs(transactionSet)
	ev := <-rbfts[0].recvChan
	event := ev.(*MiscEvent)

	rbfts[0].processEvent(event)
	assert.Equal(t, NotifyGenBatchEvent, event.EventType)
	conMsg := nodes[0].broadcastMessageCache
	assert.Equal(t, consensus.Type_PRE_PREPARE, conMsg.Type)
	assert.True(t, rbfts[0].timerMgr.getTimer(batchTimer))
}

//func TestRBFT_processReqSetEvent(t *testing.T) {
//	_, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
//	unlockCluster(rbfts)
//
//	ctx := newTx()
//	req := &RequestSet[consensus.FltTransaction, *consensus.FltTransaction]{
//		Requests: []*consensus.FltTransaction{ctx},
//	}
//
//	rbfts[1].atomicOn(InConfChange)
//	rbfts[1].processEvent(req)
//	batch, err := rbfts[1].batchMgr.requestPool.GenerateRequestBatch()
//	assert.NotNil(t, batch)
//}

// ============================================
// Post Tools
// Test Case: Should receive target Event from rbft.recvChan
// ============================================
func TestRBFT_reportStateUpdated(t *testing.T) {
	_, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()

	unlockCluster(rbfts)

	state2 := &types.ServiceState{}
	state2.MetaState = &types.MetaState{
		Height: 20,
		Digest: "block-number-20",
	}

	event := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreStateUpdatedEvent,
		Event: &types.ServiceSyncState{
			ServiceState: *state2,
			EpochChanged: false,
		},
	}

	rbfts[0].reportStateUpdated(&types.ServiceSyncState{
		ServiceState: *state2,
		EpochChanged: false,
	})
	obj := <-rbfts[0].recvChan
	assert.Equal(t, event, obj)
}

func TestRBFT_postMsg(t *testing.T) {
	_, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	unlockCluster(rbfts)

	rbfts[0].postMsg([]byte("postMsg"))
	obj := <-rbfts[0].recvChan
	assert.Equal(t, []byte("postMsg"), obj)
}

// ============================================
// Get Status
// ============================================
func TestRBFT_getStatus(t *testing.T) {
	_, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
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

// ============================================
// General Event Process Method
// ============================================
func TestRBFT_processNullRequset(t *testing.T) {
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
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
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()

	rbfts[0].atomicOn(InViewChange)
	rbfts[0].handleNullRequestTimerEvent()
	assert.Equal(t, uint64(0), rbfts[0].chainConfig.View)
	assert.Nil(t, nodes[0].broadcastMessageCache)
	rbfts[0].atomicOff(InViewChange)

	rbfts[0].setView(uint64(1))
	rbfts[0].handleNullRequestTimerEvent()
	assert.Equal(t, uint64(2), rbfts[0].chainConfig.View)
	assert.Equal(t, consensus.Type_VIEW_CHANGE, nodes[0].broadcastMessageCache.Type)
	rbfts[0].atomicOff(InViewChange)

	rbfts[0].setView(uint64(4))
	rbfts[0].handleNullRequestTimerEvent()
	assert.Equal(t, uint64(4), rbfts[0].chainConfig.View)
	assert.Equal(t, consensus.Type_NULL_REQUEST, nodes[0].broadcastMessageCache.Type)
}

func TestRBFT_fetchMissingTxs(t *testing.T) {
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	prePrep := &consensus.PrePrepare{
		ReplicaId:      uint64(1),
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
	_, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
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

	rs := &RequestSet[consensus.FltTransaction, *consensus.FltTransaction]{}
	rbfts[0].postRequests(rs)
}

func TestRBFT_processOutOfDateReqs(t *testing.T) {
	_, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	tx := newTx()
	err := rbfts[1].batchMgr.requestPool.AddLocalTx(tx)
	assert.Nil(t, err)
	rbfts[1].setFull()
	rbfts[1].processOutOfDateReqs(true)
	assert.Equal(t, true, rbfts[1].isPoolFull())

	// sleep to trigger txpool tolerance time.
	time.Sleep(1 * time.Second)
	rbfts[1].setNormal()
	rbfts[1].processOutOfDateReqs(true)
	assert.Equal(t, false, rbfts[1].isPoolFull())
	rvc := <-rbfts[1].external.(*testExternal[consensus.FltTransaction, *consensus.FltTransaction]).ListenMsg()
	assert.Equal(t, consensus.Type_REBROADCAST_REQUEST_SET, rvc.msg.Type)
	set := &RequestSet[consensus.FltTransaction, *consensus.FltTransaction]{}
	err = set.Unmarshal(rvc.msg.Payload)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(set.Requests))

	// split according to set size when broadcast.
	//batch := make([]*consensus.FltTransaction, 100)
	for i := 0; i < 24; i++ {
		tx := newTx()
		err := rbfts[1].batchMgr.requestPool.AddLocalTx(tx)
		assert.Nil(t, err)
	}
	// sleep to trigger txpool tolerance time.
	time.Sleep(1 * time.Second)
	rbfts[1].processOutOfDateReqs(true)
	assert.Equal(t, false, rbfts[1].isPoolFull())

	rvc = <-rbfts[1].external.(*testExternal[consensus.FltTransaction, *consensus.FltTransaction]).ListenMsg()
	assert.Equal(t, consensus.Type_REBROADCAST_REQUEST_SET, rvc.msg.Type)
	set = &RequestSet[consensus.FltTransaction, *consensus.FltTransaction]{}
	err = set.Unmarshal(rvc.msg.Payload)
	assert.Nil(t, err)
	assert.Equal(t, 25, len(set.Requests))
}

func TestRBFT_sendNullRequest(t *testing.T) {
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()

	rbfts[0].atomicOn(InConfChange)
	rbfts[0].sendNullRequest()
	rbfts[0].atomicOff(InConfChange)

	rbfts[0].sendNullRequest()

	nullRequest := &consensus.NullRequest{
		ReplicaId: rbfts[0].chainConfig.SelfID,
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
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	hashBatch := &consensus.HashBatch{
		RequestHashList: []string{"tx-hash"},
	}
	preprep1 := &consensus.PrePrepare{
		View:           rbfts[0].chainConfig.View,
		SequenceNumber: uint64(1),
		BatchDigest:    "wrong hash",
		HashBatch:      hashBatch,
		ReplicaId:      rbfts[0].chainConfig.SelfID,
	}
	err := rbfts[1].recvPrePrepare(context.TODO(), preprep1)
	assert.Nil(t, err)
	assert.Equal(t, consensus.Type_VIEW_CHANGE, nodes[1].broadcastMessageCache.Type)
}

func TestRBFT_recvPrePrepare_EmptyDigest(t *testing.T) {
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	hashBatch := &consensus.HashBatch{
		RequestHashList: []string{"tx-hash"},
	}
	preprep1 := &consensus.PrePrepare{
		View:           rbfts[0].chainConfig.View,
		SequenceNumber: uint64(1),
		BatchDigest:    "",
		HashBatch:      hashBatch,
		ReplicaId:      rbfts[0].chainConfig.SelfID,
	}
	err := rbfts[1].recvPrePrepare(context.TODO(), preprep1)
	assert.Nil(t, err)
	assert.Equal(t, consensus.Type_VIEW_CHANGE, nodes[1].broadcastMessageCache.Type)
}

func TestRBFT_recvPrePrepare_WrongSeqNo(t *testing.T) {
	tx := newTx()
	txBytes, err := tx.RbftMarshal()
	assert.Nil(t, err)

	txHash := tx.RbftGetTxHash()
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	hashBatch := &consensus.HashBatch{
		RequestHashList: []string{txHash},
	}
	preprep := &consensus.PrePrepare{
		View:           rbfts[0].chainConfig.View,
		SequenceNumber: uint64(1),
		HashBatch:      hashBatch,
		ReplicaId:      rbfts[0].chainConfig.SelfID,
	}
	batchHash := calculateMD5Hash(preprep.HashBatch.RequestHashList, preprep.HashBatch.Timestamp)
	preprep.BatchDigest = batchHash

	err = rbfts[1].recvPrePrepare(context.TODO(), preprep)
	assert.Nil(t, err)
	assert.Equal(t, consensus.Type_FETCH_MISSING_REQUEST, nodes[1].unicastMessageCache.Type) // fetching missing tx for preprepare message

	rbfts[1].recvFetchMissingResponse(context.TODO(), &consensus.FetchMissingResponse{
		ReplicaId:      rbfts[0].chainConfig.SelfID,
		View:           rbfts[0].chainConfig.View,
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
		View:           rbfts[0].chainConfig.View,
		SequenceNumber: uint64(1),
		HashBatch:      hashBatchWrong,
		ReplicaId:      rbfts[0].chainConfig.SelfID,
	}
	preprepDup.BatchDigest = calculateMD5Hash(preprepDup.HashBatch.RequestHashList, preprepDup.HashBatch.Timestamp)
	err = rbfts[1].recvPrePrepare(context.TODO(), preprepDup)

	assert.Nil(t, err)
	assert.Equal(t, consensus.Type_VIEW_CHANGE, nodes[1].broadcastMessageCache.Type)
}

func TestRBFT_recvFetchMissingResponse(t *testing.T) {
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()

	tx := newTx()
	txBytes, err := tx.RbftMarshal()
	assert.Nil(t, err)
	txHash := tx.RbftGetTxHash()

	hashBatch := &consensus.HashBatch{
		RequestHashList: []string{txHash},
	}
	preprep := &consensus.PrePrepare{
		View:           rbfts[0].chainConfig.View,
		SequenceNumber: uint64(1),
		HashBatch:      hashBatch,
		ReplicaId:      rbfts[0].chainConfig.SelfID,
	}
	preprep.BatchDigest = calculateMD5Hash(preprep.HashBatch.RequestHashList, preprep.HashBatch.Timestamp)

	fetch := &consensus.FetchMissingRequest{
		View:                 preprep.View,
		SequenceNumber:       preprep.SequenceNumber,
		BatchDigest:          preprep.BatchDigest,
		MissingRequestHashes: map[uint64]string{0: txHash},
		ReplicaId:            rbfts[1].chainConfig.SelfID,
	}

	err = rbfts[0].recvFetchMissingRequest(context.TODO(), fetch)
	assert.Nil(t, err)
	assert.Nil(t, nodes[0].unicastMessageCache)

	rbfts[0].storeMgr.batchStore[fetch.BatchDigest] = &RequestBatch[consensus.FltTransaction, *consensus.FltTransaction]{
		RequestHashList: []string{txHash},
		RequestList:     []*consensus.FltTransaction{tx},
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
		ReplicaId:            rbfts[0].chainConfig.SelfID,
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
