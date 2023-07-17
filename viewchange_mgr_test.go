package rbft

import (
	"testing"

	"github.com/hyperchain/go-hpc-rbft/v2/common/consensus"
	"github.com/hyperchain/go-hpc-rbft/v2/types"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
)

func TestVC_FullProcess(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	unlockCluster(rbfts)
	// init recovery to stable view 1, primary is node2
	clusterInitRecovery(t, nodes, rbfts, -1)
	assert.Equal(t, uint64(1), rbfts[1].view)
	assert.Equal(t, uint64(1), rbfts[2].view)
	assert.Equal(t, uint64(1), rbfts[3].view)

	tx := newTx()
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	// node3 cache some txs.
	rbfts[2].batchMgr.requestPool.AddNewRequests([][]byte{txBytes}, false, true)

	// replica 1 send vc
	rbfts[0].sendViewChange()
	vcNode3 := nodes[0].broadcastMessageCache
	assert.Equal(t, consensus.Type_VIEW_CHANGE, vcNode3.Type)

	// replica 2 send vc
	rbfts[1].sendViewChange()
	vcNode4 := nodes[1].broadcastMessageCache
	assert.Equal(t, consensus.Type_VIEW_CHANGE, vcNode4.Type)

	// replica 3 received f+1 vc and then enter vc, trigger vc quorum
	rbfts[2].processEvent(vcNode3)
	quorumEvent := rbfts[2].processEvent(vcNode4)

	ev := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeQuorumEvent,
	}
	assert.Equal(t, ev, quorumEvent)

	// new primary 3 send new view.
	done := rbfts[2].processEvent(ev)
	nv := nodes[2].broadcastMessageCache
	doneEvent := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeDoneEvent,
	}
	assert.Equal(t, consensus.Type_NEW_VIEW, nv.Type)
	assert.Equal(t, doneEvent, done)

	// new backup nodes process new view.
	rbfts[1].processEvent(nv)
	rbfts[3].processEvent(nv)
	assert.Equal(t, uint64(2), rbfts[1].view)
	assert.Equal(t, uint64(2), rbfts[2].view)
	assert.Equal(t, uint64(2), rbfts[3].view)

	assert.NotEqual(t, consensus.Type_PRE_PREPARE, nodes[2].broadcastMessageCache.Type)
	rbfts[2].handleViewChangeEvent(doneEvent)
	assert.Equal(t, consensus.Type_PRE_PREPARE, nodes[2].broadcastMessageCache.Type)
}

func TestVC_recvViewChange_FromPrimary(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	unlockCluster(rbfts)
	// init recovery to stable view 1, primary is node2
	clusterInitRecovery(t, nodes, rbfts, -1)

	// primary node2 send vc.
	rbfts[1].sendViewChange()
	assert.Equal(t, uint64(2), rbfts[1].view)

	vcPayload := nodes[1].broadcastMessageCache.Payload
	vc := &consensus.ViewChange{}
	_ = proto.Unmarshal(vcPayload, vc)

	// backup node3 receive vc and trigger vc.
	rbfts[2].recvViewChange(vc, unMarshalVcBasis(vc), false)
	assert.Equal(t, consensus.Type_VIEW_CHANGE, nodes[2].broadcastMessageCache.Type)

	assert.Equal(t, uint64(2), rbfts[2].view)
}

func TestVC_recvViewChange_Quorum(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	unlockCluster(rbfts)
	// init recovery to stable view 1, primary is node2
	clusterInitRecovery(t, nodes, rbfts, -1)

	rbfts[1].sendViewChange()
	rbfts[3].sendViewChange()

	vcPayload1 := nodes[1].broadcastMessageCache.Payload
	vc1 := &consensus.ViewChange{}
	_ = proto.Unmarshal(vcPayload1, vc1)

	vcPayload3 := nodes[3].broadcastMessageCache.Payload
	vc3 := &consensus.ViewChange{}
	_ = proto.Unmarshal(vcPayload3, vc3)

	// get quorum
	rbfts[2].recvViewChange(vc1, unMarshalVcBasis(vc1), false)
	ret := rbfts[2].recvViewChange(vc3, unMarshalVcBasis(vc3), false)
	exp := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeQuorumEvent,
	}
	assert.Equal(t, exp, ret)
}

func TestVC_fetchRequestBatches(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	unlockCluster(rbfts)
	// init recovery to stable view 1, primary is node2
	clusterInitRecovery(t, nodes, rbfts, -1)

	rbfts[2].storeMgr.missingReqBatches["lost-batch"] = true
	rbfts[2].fetchRequestBatches()
	assert.Equal(t, consensus.Type_FETCH_BATCH_REQUEST, nodes[2].broadcastMessageCache.Type)
}

func TestVC_recvFetchRequestBatch(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	unlockCluster(rbfts)
	// init recovery to stable view 1, primary is node2
	clusterInitRecovery(t, nodes, rbfts, -1)

	rbfts[2].storeMgr.missingReqBatches["lost-batch"] = true
	rbfts[2].fetchRequestBatches()
	fr := nodes[2].broadcastMessageCache
	assert.Equal(t, consensus.Type_FETCH_BATCH_REQUEST, fr.Type)

	ret1 := rbfts[1].processEvent(fr)
	assert.Nil(t, ret1)
	assert.Nil(t, nodes[1].unicastMessageCache)

	rbfts[1].storeMgr.batchStore["lost-batch"] = &consensus.RequestBatch{}
	ret2 := rbfts[1].processEvent(fr)
	assert.Nil(t, ret2)
	assert.Equal(t, consensus.Type_FETCH_BATCH_RESPONSE, nodes[1].unicastMessageCache.Type)
}

func TestVC_recvFetchBatchResponse_AfterViewChanged(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	unlockCluster(rbfts)
	// init recovery to stable view 1, primary is node2
	clusterInitRecovery(t, nodes, rbfts, -1)

	// f+1 view-change from view 1 to view 2
	rbfts[0].sendViewChange()
	rbfts[1].sendViewChange()
	vcPayload0 := nodes[0].broadcastMessageCache.Payload
	vc0 := &consensus.ViewChange{}
	_ = proto.Unmarshal(vcPayload0, vc0)
	vcPayload1 := nodes[1].broadcastMessageCache.Payload
	vc1 := &consensus.ViewChange{}
	_ = proto.Unmarshal(vcPayload1, vc1)

	// new primary node3 receive f+1 vc request and trigger vc quorum.
	rbfts[2].recvViewChange(vc0, unMarshalVcBasis(vc0), false)
	ret := rbfts[2].recvViewChange(vc1, unMarshalVcBasis(vc1), false)
	exp := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeQuorumEvent,
	}
	assert.Equal(t, exp, ret)

	// node3 send new view
	rbfts[2].sendNewView()
	nvMessage := nodes[2].broadcastMessageCache
	nv := &consensus.NewView{}
	_ = proto.Unmarshal(nvMessage.Payload, nv)

	// mock node1 store the new-view from primary
	rbfts[0].vcMgr.newViewStore[nv.View] = nv

	// start the test for recv nil fetch request batch response
	ret1 := rbfts[0].recvFetchBatchResponse(nil)
	assert.Nil(t, ret1)

	// construct a fetch request batch response
	tx := newTx()
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	batch := &consensus.RequestBatch{
		RequestHashList: []string{"tx-hash"},
		RequestList:     [][]byte{txBytes},
		SeqNo:           uint64(1),
		LocalList:       []bool{true},
		BatchHash:       "lost-batch",
	}
	sr := &consensus.FetchBatchResponse{
		ReplicaId:   uint64(2),
		Batch:       batch,
		BatchDigest: batch.BatchHash,
	}

	// the request batch is not the one we want
	rbfts[0].storeMgr.missingReqBatches["lost-batch-another"] = true
	rbfts[0].recvFetchBatchResponse(sr)
	assert.Equal(t, true, rbfts[0].storeMgr.missingReqBatches["lost-batch-another"])
	delete(rbfts[0].storeMgr.missingReqBatches, "lost-batch-another")

	// recv the request batch we want
	rbfts[0].storeMgr.missingReqBatches["lost-batch"] = true
	rbfts[0].atomicOn(InViewChange)
	ret3 := rbfts[0].recvFetchBatchResponse(sr)
	exp3 := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeDoneEvent,
	}
	assert.Equal(t, false, rbfts[0].storeMgr.missingReqBatches["lost-batch"])
	assert.Equal(t, exp3, ret3)
}

func TestVC_processNewView_AfterViewChanged_PrimaryNormal(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	unlockCluster(rbfts)
	// init recovery to stable view 1, primary is node2
	clusterInitRecovery(t, nodes, rbfts, -1)

	// mock primary node2 cached some txs.
	nodes[1].broadcastMessageCache = nil
	tx := newTx()
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	rbfts[1].batchMgr.requestPool.AddNewRequests([][]byte{txBytes}, false, true)
	batch := rbfts[1].batchMgr.requestPool.GenerateRequestBatch()

	// a message list
	msgList := []*consensus.Vc_PQ{
		{SequenceNumber: 1, BatchDigest: ""},
		{SequenceNumber: 2, BatchDigest: ""},
		{SequenceNumber: 3, BatchDigest: batch[0].BatchHash},
	}

	rbfts[1].putBackRequestBatches(msgList)

	batch3 := &consensus.RequestBatch{
		RequestHashList: batch[0].TxHashList,
		RequestList:     batch[0].TxList,
		Timestamp:       batch[0].Timestamp,
		LocalList:       batch[0].LocalList,
		BatchHash:       batch[0].BatchHash,
		SeqNo:           uint64(3),
	}
	rbfts[1].storeMgr.batchStore[batch[0].BatchHash] = batch3
	rbfts[1].processNewView(msgList)
	assert.Equal(t, uint64(3), rbfts[1].batchMgr.seqNo)
	assert.Nil(t, nodes[1].broadcastMessageCache)
}

func TestVC_processNewView_AfterViewChanged_ReplicaNormal(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	unlockCluster(rbfts)
	// init recovery to stable view 1, primary is node2
	clusterInitRecovery(t, nodes, rbfts, -1)

	// mock backup node1 cached some txs.
	tx := newTx()
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	rbfts[0].batchMgr.requestPool.AddNewRequests([][]byte{txBytes}, false, true)
	batch := rbfts[0].batchMgr.requestPool.GenerateRequestBatch()

	// a message list
	msgList := []*consensus.Vc_PQ{
		{SequenceNumber: 1, BatchDigest: ""},
		{SequenceNumber: 2, BatchDigest: ""},
		{SequenceNumber: 3, BatchDigest: batch[0].BatchHash},
	}

	rbfts[0].putBackRequestBatches(msgList)

	batch3 := &consensus.RequestBatch{
		RequestHashList: batch[0].TxHashList,
		RequestList:     batch[0].TxList,
		Timestamp:       batch[0].Timestamp,
		LocalList:       batch[0].LocalList,
		BatchHash:       batch[0].BatchHash,
		SeqNo:           uint64(3),
	}
	rbfts[0].storeMgr.batchStore[batch[0].BatchHash] = batch3
	rbfts[0].setView(uint64(1))
	rbfts[0].exec.setLastExec(uint64(3))
	rbfts[0].processNewView(msgList)
	assert.Equal(t, uint64(3), rbfts[0].batchMgr.seqNo)
	assert.Equal(t, consensus.Type_COMMIT, nodes[0].broadcastMessageCache.Type)
}

func TestVC_processNewView_AfterViewChanged_LargerConfig(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()

	tx := newTx()
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	rbfts[0].batchMgr.requestPool.AddNewRequests([][]byte{txBytes}, true, true)
	batch := rbfts[0].batchMgr.requestPool.GenerateRequestBatch()[0]

	ctx := newCTX(defaultValidatorSet)
	ctxBytes, err := ctx.Marshal()
	assert.Nil(t, err)

	batch1, _ := rbfts[0].batchMgr.requestPool.AddNewRequests([][]byte{ctxBytes}, true, true)
	configBatch := batch1[0]

	// a message list
	msgList := []*consensus.Vc_PQ{
		{SequenceNumber: 0, BatchDigest: "XXX GENESIS"},
		{SequenceNumber: 1, BatchDigest: ""},
		{SequenceNumber: 2, BatchDigest: ""},
		{SequenceNumber: 3, BatchDigest: batch.BatchHash},
		{SequenceNumber: 4, BatchDigest: configBatch.BatchHash},
	}

	rbfts[0].putBackRequestBatches(msgList)

	requestBatch := &consensus.RequestBatch{
		RequestHashList: batch.TxHashList,
		RequestList:     batch.TxList,
		Timestamp:       batch.Timestamp,
		LocalList:       batch.LocalList,
		BatchHash:       batch.BatchHash,
		SeqNo:           uint64(3),
	}
	rbfts[0].storeMgr.batchStore[batch.BatchHash] = requestBatch

	configRequestBatch := &consensus.RequestBatch{
		RequestHashList: configBatch.TxHashList,
		RequestList:     configBatch.TxList,
		Timestamp:       configBatch.Timestamp,
		LocalList:       configBatch.LocalList,
		BatchHash:       configBatch.BatchHash,
		SeqNo:           uint64(4),
	}
	rbfts[0].storeMgr.batchStore[configBatch.BatchHash] = configRequestBatch

	rbfts[0].setView(uint64(1))
	rbfts[0].exec.setLastExec(uint64(5))
	rbfts[0].processNewView(msgList)
	assert.Equal(t, uint64(5), rbfts[0].batchMgr.seqNo)
	assert.Equal(t, consensus.Type_COMMIT, nodes[0].broadcastMessageCache.Type)
}

func TestVC_processNewView_AfterViewChanged_LowerConfig(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()

	tx := newTx()
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	rbfts[0].batchMgr.requestPool.AddNewRequests([][]byte{txBytes}, true, true)
	batch := rbfts[0].batchMgr.requestPool.GenerateRequestBatch()[0]

	ctx := newCTX(defaultValidatorSet)
	ctxBytes, err := ctx.Marshal()
	assert.Nil(t, err)
	batch1, _ := rbfts[0].batchMgr.requestPool.AddNewRequests([][]byte{ctxBytes}, true, true)
	configBatch := batch1[0]

	// a message list
	msgList := []*consensus.Vc_PQ{
		{SequenceNumber: 0, BatchDigest: "XXX GENESIS"},
		{SequenceNumber: 1, BatchDigest: ""},
		{SequenceNumber: 2, BatchDigest: ""},
		{SequenceNumber: 3, BatchDigest: batch.BatchHash},
		{SequenceNumber: 4, BatchDigest: configBatch.BatchHash},
	}

	rbfts[0].putBackRequestBatches(msgList)

	requestBatch := &consensus.RequestBatch{
		RequestHashList: batch.TxHashList,
		RequestList:     batch.TxList,
		Timestamp:       batch.Timestamp,
		LocalList:       batch.LocalList,
		BatchHash:       batch.BatchHash,
		SeqNo:           uint64(3),
	}
	rbfts[0].storeMgr.batchStore[batch.BatchHash] = requestBatch

	configRequestBatch := &consensus.RequestBatch{
		RequestHashList: configBatch.TxHashList,
		RequestList:     configBatch.TxList,
		Timestamp:       configBatch.Timestamp,
		LocalList:       configBatch.LocalList,
		BatchHash:       configBatch.BatchHash,
		SeqNo:           uint64(4),
	}
	rbfts[0].storeMgr.batchStore[configBatch.BatchHash] = configRequestBatch

	rbfts[0].setView(uint64(2))
	rbfts[0].exec.setLastExec(uint64(3))
	rbfts[0].processNewView(msgList)
	assert.Equal(t, uint64(4), rbfts[0].batchMgr.seqNo)
	assert.Equal(t, consensus.Type_PREPARE, nodes[0].broadcastMessageCache.Type)
}

func TestVC_processNewView_AfterViewChanged_EqualConfig(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()

	tx := newTx()
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	rbfts[0].batchMgr.requestPool.AddNewRequests([][]byte{txBytes}, true, true)
	batch := rbfts[0].batchMgr.requestPool.GenerateRequestBatch()[0]

	ctx := newCTX(defaultValidatorSet)
	ctxBytes, err := ctx.Marshal()
	assert.Nil(t, err)
	batch1, _ := rbfts[0].batchMgr.requestPool.AddNewRequests([][]byte{ctxBytes}, true, true)
	configBatch := batch1[0]

	// a message list
	msgList := []*consensus.Vc_PQ{
		{SequenceNumber: 0, BatchDigest: "XXX GENESIS"},
		{SequenceNumber: 1, BatchDigest: ""},
		{SequenceNumber: 2, BatchDigest: ""},
		{SequenceNumber: 3, BatchDigest: batch.BatchHash},
		{SequenceNumber: 4, BatchDigest: configBatch.BatchHash},
	}

	rbfts[0].putBackRequestBatches(msgList)

	requestBatch := &consensus.RequestBatch{
		RequestHashList: batch.TxHashList,
		RequestList:     batch.TxList,
		Timestamp:       batch.Timestamp,
		LocalList:       batch.LocalList,
		BatchHash:       batch.BatchHash,
		SeqNo:           uint64(3),
	}
	rbfts[0].storeMgr.batchStore[batch.BatchHash] = requestBatch

	configRequestBatch := &consensus.RequestBatch{
		RequestHashList: configBatch.TxHashList,
		RequestList:     configBatch.TxList,
		Timestamp:       configBatch.Timestamp,
		LocalList:       configBatch.LocalList,
		BatchHash:       configBatch.BatchHash,
		SeqNo:           uint64(4),
	}
	rbfts[0].storeMgr.batchStore[configBatch.BatchHash] = configRequestBatch

	rbfts[0].setView(uint64(3))
	rbfts[0].exec.setLastExec(uint64(4))
	rbfts[0].node.ReportExecuted(&types.ServiceState{
		MetaState: &types.MetaState{
			Height: 4,
			Digest: "digest-4",
		},
	})
	rbfts[0].processNewView(msgList)
	assert.Equal(t, uint64(4), rbfts[0].batchMgr.seqNo)
	assert.Equal(t, consensus.Type_COMMIT, nodes[0].broadcastMessageCache.Type)
}

func TestVC_fetchMissingReqBatchIfNeeded(t *testing.T) {

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()

	tx := newTx()
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	rbfts[0].batchMgr.requestPool.AddNewRequests([][]byte{txBytes}, false, true)
	batch := rbfts[0].batchMgr.requestPool.GenerateRequestBatch()

	// a message list
	msgList := []*consensus.Vc_PQ{
		{SequenceNumber: 1, BatchDigest: ""},
		{SequenceNumber: 2, BatchDigest: ""},
		{SequenceNumber: 3, BatchDigest: batch[0].BatchHash},
	}

	rbfts[0].putBackRequestBatches(msgList)

	batch3 := &consensus.RequestBatch{
		RequestHashList: batch[0].TxHashList,
		RequestList:     batch[0].TxList,
		Timestamp:       batch[0].Timestamp,
		LocalList:       batch[0].LocalList,
		BatchHash:       batch[0].BatchHash,
		SeqNo:           uint64(3),
	}
	rbfts[0].storeMgr.batchStore[batch[0].BatchHash] = batch3

	flag1 := rbfts[0].checkIfNeedFetchMissingReqBatch(msgList)
	assert.Equal(t, false, flag1)

	delete(rbfts[0].storeMgr.batchStore, batch3.BatchHash)
	flag2 := rbfts[0].checkIfNeedFetchMissingReqBatch(msgList)
	assert.Equal(t, true, flag2)
}

func TestVC_correctViewChange(t *testing.T) {

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()

	rbfts[2].sendViewChange()
	vcPayload := nodes[2].broadcastMessageCache.Payload
	vc := &consensus.ViewChange{}
	_ = proto.Unmarshal(vcPayload, vc)

	flag1 := rbfts[0].correctViewChange(unMarshalVcBasis(vc))
	assert.Equal(t, true, flag1)

	p := &consensus.Vc_PQ{
		SequenceNumber: uint64(19),
		BatchDigest:    "batch-number-19",
		View:           uint64(2),
	}
	pSet := []*consensus.Vc_PQ{p}

	vcBasisError1 := &consensus.VcBasis{
		View: uint64(1),
		H:    uint64(20),
		Pset: pSet,
	}
	flag2 := rbfts[0].correctViewChange(vcBasisError1)
	assert.Equal(t, false, flag2)

	c := &consensus.SignedCheckpoint{
		Checkpoint: &consensus.Checkpoint{
			ExecuteState: &consensus.Checkpoint_ExecuteState{
				Height: uint64(10),
				Digest: "block-number-10",
			},
		},
	}
	cSet := []*consensus.SignedCheckpoint{c}
	vcBasisError2 := &consensus.VcBasis{
		View: uint64(1),
		H:    uint64(60),
		Cset: cSet,
	}
	flag3 := rbfts[0].correctViewChange(vcBasisError2)
	assert.Equal(t, false, flag3)
}

func TestVC_assignSequenceNumbers(t *testing.T) {

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()

	//Quorum = 3
	//oneQuorum = 2
	C := &consensus.SignedCheckpoint{
		Checkpoint: &consensus.Checkpoint{
			ExecuteState: &consensus.Checkpoint_ExecuteState{
				Height: uint64(5),
				Digest: "msg",
			},
		},
	}
	CSet := []*consensus.SignedCheckpoint{C}
	P1 := &consensus.Vc_PQ{
		SequenceNumber: 5,
		BatchDigest:    "msg",
		View:           1,
	}
	// with another sequenceNumber
	P2 := &consensus.Vc_PQ{
		SequenceNumber: 10,
		BatchDigest:    "msgHigh",
		View:           0,
	}
	PSet := []*consensus.Vc_PQ{P1, P2}
	Q1 := &consensus.Vc_PQ{
		SequenceNumber: 5,
		BatchDigest:    "msg",
		View:           1,
	}
	Q2 := &consensus.Vc_PQ{
		SequenceNumber: 10,
		BatchDigest:    "msgHigh",
		View:           0,
	}
	QSet := []*consensus.Vc_PQ{Q1, Q2}
	Basis := &consensus.VcBasis{
		ReplicaId: 1,
		View:      1,
		H:         4,
		Cset:      CSet,
		Pset:      PSet,
		Qset:      QSet,
	}
	BasisHighH := &consensus.VcBasis{
		ReplicaId: 1,
		View:      1,
		H:         6,
		Cset:      CSet,
		Pset:      PSet,
		Qset:      QSet,
	}
	set := []*consensus.VcBasis{BasisHighH, Basis, Basis, Basis}
	list := rbfts[1].assignSequenceNumbers(set, 4)

	expectList := []*consensus.Vc_PQ{
		{SequenceNumber: 5, BatchDigest: "msg"},
		{SequenceNumber: 6, BatchDigest: ""},
		{SequenceNumber: 7, BatchDigest: ""},
		{SequenceNumber: 8, BatchDigest: ""},
		{SequenceNumber: 9, BatchDigest: ""},
		{SequenceNumber: 10, BatchDigest: "msgHigh"},
	}

	assert.EqualValues(t, expectList, list)
}
