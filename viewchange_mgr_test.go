package rbft

import (
	"testing"

	"github.com/axiomesh/axiom-kit/txpool"
	"github.com/stretchr/testify/assert"

	"github.com/axiomesh/axiom-bft/common/consensus"
)

func TestVC_FullProcess(t *testing.T) {
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	unlockCluster(rbfts)
	// init recovery to stable view 1, primary is node2
	clusterInitRecovery(t, nodes, rbfts, -1)
	assert.Equal(t, uint64(1), rbfts[1].chainConfig.View)
	assert.Equal(t, uint64(1), rbfts[2].chainConfig.View)
	assert.Equal(t, uint64(1), rbfts[3].chainConfig.View)

	tx := newTx()
	// node3 cache some txs.
	err := rbfts[2].batchMgr.requestPool.AddLocalTx(tx)
	assert.Nil(t, err)

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
	assert.Equal(t, uint64(2), rbfts[1].chainConfig.View)
	assert.Equal(t, uint64(2), rbfts[2].chainConfig.View)
	assert.Equal(t, uint64(2), rbfts[3].chainConfig.View)

	assert.NotEqual(t, consensus.Type_PRE_PREPARE, nodes[2].broadcastMessageCache.Type)
	rbfts[2].handleViewChangeEvent(doneEvent)
	assert.Equal(t, consensus.Type_PRE_PREPARE, nodes[2].broadcastMessageCache.Type)
}

func TestVC_recvViewChange_FromPrimary(t *testing.T) {
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	unlockCluster(rbfts)
	// init recovery to stable view 1, primary is node2
	clusterInitRecovery(t, nodes, rbfts, -1)

	// primary node2 send vc.
	rbfts[1].sendViewChange()
	assert.Equal(t, uint64(2), rbfts[1].chainConfig.View)

	vcPayload := nodes[1].broadcastMessageCache.Payload
	vc := &consensus.ViewChange{}
	_ = vc.UnmarshalVT(vcPayload)

	// backup node3 receive vc and trigger vc.
	rbfts[2].recvViewChange(vc, false)
	assert.Equal(t, consensus.Type_VIEW_CHANGE, nodes[2].broadcastMessageCache.Type)

	assert.Equal(t, uint64(2), rbfts[2].chainConfig.View)
}

func TestVC_recvViewChange_Quorum(t *testing.T) {
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	unlockCluster(rbfts)
	// init recovery to stable view 1, primary is node2
	clusterInitRecovery(t, nodes, rbfts, -1)

	rbfts[1].sendViewChange()
	rbfts[3].sendViewChange()

	vcPayload1 := nodes[1].broadcastMessageCache.Payload
	vc1 := &consensus.ViewChange{}
	_ = vc1.UnmarshalVT(vcPayload1)

	vcPayload3 := nodes[3].broadcastMessageCache.Payload
	vc3 := &consensus.ViewChange{}
	_ = vc3.UnmarshalVT(vcPayload3)

	// get quorum
	rbfts[2].recvViewChange(vc1, false)
	ret := rbfts[2].recvViewChange(vc3, false)
	exp := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeQuorumEvent,
	}
	assert.Equal(t, exp, ret)
}

func TestVC_fetchRequestBatches(t *testing.T) {
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	unlockCluster(rbfts)
	// init recovery to stable view 1, primary is node2
	clusterInitRecovery(t, nodes, rbfts, -1)

	rbfts[2].storeMgr.missingReqBatches["lost-batch"] = true
	rbfts[2].fetchRequestBatches()
	assert.Equal(t, consensus.Type_FETCH_BATCH_REQUEST, nodes[2].broadcastMessageCache.Type)
}

func TestVC_recvFetchRequestBatch(t *testing.T) {
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
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

	rbfts[1].storeMgr.batchStore["lost-batch"] = &RequestBatch[consensus.FltTransaction, *consensus.FltTransaction]{}
	ret2 := rbfts[1].processEvent(fr)
	assert.Nil(t, ret2)
	assert.Equal(t, consensus.Type_FETCH_BATCH_RESPONSE, nodes[1].unicastMessageCache.Type)
}

func TestVC_recvFetchBatchResponse_AfterViewChanged(t *testing.T) {
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	unlockCluster(rbfts)
	// init recovery to stable view 1, primary is node2
	clusterInitRecovery(t, nodes, rbfts, -1)

	// f+1 view-change from view 1 to view 2
	rbfts[0].sendViewChange()
	rbfts[1].sendViewChange()
	vcPayload0 := nodes[0].broadcastMessageCache.Payload
	vc0 := &consensus.ViewChange{}
	_ = vc0.UnmarshalVT(vcPayload0)
	vcPayload1 := nodes[1].broadcastMessageCache.Payload
	vc1 := &consensus.ViewChange{}
	_ = vc1.UnmarshalVT(vcPayload1)

	// new primary node3 receive f+1 vc request and trigger vc quorum.
	rbfts[2].recvViewChange(vc0, false)
	ret := rbfts[2].recvViewChange(vc1, false)
	exp := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeQuorumEvent,
	}
	assert.Equal(t, exp, ret)

	// node3 send new view
	rbfts[2].sendNewView()
	nvMessage := nodes[2].broadcastMessageCache
	nv := &consensus.NewView{}
	_ = nv.UnmarshalVT(nvMessage.Payload)

	// mock node1 store the new-view from primary
	rbfts[0].vcMgr.newViewStore[nv.View] = nv

	// start the test for recv nil fetch request batch response
	ret1 := rbfts[0].recvFetchBatchResponse(nil)
	assert.Nil(t, ret1)

	// construct a fetch request batch response
	tx := newTx()
	batch := &RequestBatch[consensus.FltTransaction, *consensus.FltTransaction]{
		RequestHashList: []string{"tx-hash"},
		RequestList:     []*consensus.FltTransaction{tx},
		SeqNo:           uint64(1),
		LocalList:       []bool{true},
		BatchHash:       "lost-batch",
	}
	pbBatch, err := batch.ToPB()
	assert.Nil(t, err)
	sr := &consensus.FetchBatchResponse{
		ReplicaId:   uint64(2),
		Batch:       pbBatch,
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
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	unlockCluster(rbfts)
	// init recovery to stable view 1, primary is node2
	clusterInitRecovery(t, nodes, rbfts, -1)

	// mock primary node2 cached some txs.
	nodes[1].broadcastMessageCache = nil
	tx := newTx()
	err := rbfts[1].batchMgr.requestPool.AddLocalTx(tx)
	assert.Nil(t, err)
	batch, err := rbfts[1].batchMgr.requestPool.GenerateRequestBatch(txpool.GenBatchTimeoutEvent)
	assert.Nil(t, err)

	// a message list
	msgList := []*consensus.VcPq{
		{SequenceNumber: 1, BatchDigest: ""},
		{SequenceNumber: 2, BatchDigest: ""},
		{SequenceNumber: 3, BatchDigest: batch.BatchHash},
	}

	rbfts[1].putBackRequestBatches(msgList)

	batch3 := &RequestBatch[consensus.FltTransaction, *consensus.FltTransaction]{
		RequestHashList: batch.TxHashList,
		RequestList:     batch.TxList,
		Timestamp:       batch.Timestamp,
		LocalList:       batch.LocalList,
		BatchHash:       batch.BatchHash,
		SeqNo:           uint64(3),
	}
	rbfts[1].storeMgr.batchStore[batch.BatchHash] = batch3
	rbfts[1].processNewView(msgList)
	assert.Equal(t, uint64(3), rbfts[1].batchMgr.seqNo)
	assert.Nil(t, nodes[1].broadcastMessageCache)
}

func TestVC_processNewView_AfterViewChanged_ReplicaNormal(t *testing.T) {
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	unlockCluster(rbfts)
	// init recovery to stable view 1, primary is node2
	clusterInitRecovery(t, nodes, rbfts, -1)

	// mock backup node1 cached some txs.
	tx := newTx()
	err := rbfts[0].batchMgr.requestPool.AddLocalTx(tx)
	assert.Nil(t, err)
	batch, err := rbfts[0].batchMgr.requestPool.GenerateRequestBatch(txpool.GenBatchTimeoutEvent)
	assert.Nil(t, err)

	// a message list
	msgList := []*consensus.VcPq{
		{SequenceNumber: 1, BatchDigest: ""},
		{SequenceNumber: 2, BatchDigest: ""},
		{SequenceNumber: 3, BatchDigest: batch.BatchHash},
	}

	rbfts[0].putBackRequestBatches(msgList)

	batch3 := &RequestBatch[consensus.FltTransaction, *consensus.FltTransaction]{
		RequestHashList: batch.TxHashList,
		RequestList:     batch.TxList,
		Timestamp:       batch.Timestamp,
		LocalList:       batch.LocalList,
		BatchHash:       batch.BatchHash,
		SeqNo:           uint64(3),
	}
	rbfts[0].storeMgr.batchStore[batch.BatchHash] = batch3
	rbfts[0].setView(uint64(1))
	rbfts[0].exec.setLastExec(uint64(3))
	rbfts[0].processNewView(msgList)
	assert.Equal(t, uint64(3), rbfts[0].batchMgr.seqNo)
	assert.Equal(t, consensus.Type_COMMIT, nodes[0].broadcastMessageCache.Type)
}

func TestVC_fetchMissingReqBatchIfNeeded(t *testing.T) {
	_, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()

	tx := newTx()
	err := rbfts[0].batchMgr.requestPool.AddLocalTx(tx)
	assert.Nil(t, err)
	batch, err := rbfts[0].batchMgr.requestPool.GenerateRequestBatch(txpool.GenBatchTimeoutEvent)
	assert.Nil(t, err)

	// a message list
	msgList := []*consensus.VcPq{
		{SequenceNumber: 1, BatchDigest: ""},
		{SequenceNumber: 2, BatchDigest: ""},
		{SequenceNumber: 3, BatchDigest: batch.BatchHash},
	}

	rbfts[0].putBackRequestBatches(msgList)

	batch3 := &RequestBatch[consensus.FltTransaction, *consensus.FltTransaction]{
		RequestHashList: batch.TxHashList,
		RequestList:     batch.TxList,
		Timestamp:       batch.Timestamp,
		LocalList:       batch.LocalList,
		BatchHash:       batch.BatchHash,
		SeqNo:           uint64(3),
	}
	rbfts[0].storeMgr.batchStore[batch.BatchHash] = batch3

	flag1 := rbfts[0].checkIfNeedFetchMissingReqBatch(msgList)
	assert.Equal(t, false, flag1)

	delete(rbfts[0].storeMgr.batchStore, batch3.BatchHash)
	flag2 := rbfts[0].checkIfNeedFetchMissingReqBatch(msgList)
	assert.Equal(t, true, flag2)
}

func TestVC_correctViewChange(t *testing.T) {
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()

	rbfts[2].sendViewChange()
	vcPayload := nodes[2].broadcastMessageCache.Payload
	vc := &consensus.ViewChange{}
	_ = vc.UnmarshalVT(vcPayload)

	flag1 := rbfts[0].correctViewChange(unMarshalVcBasis(vc))
	assert.Equal(t, true, flag1)

	p := &consensus.VcPq{
		SequenceNumber: uint64(19),
		BatchDigest:    "batch-number-19",
		View:           uint64(2),
	}
	pSet := []*consensus.VcPq{p}

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
	_, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()

	// Quorum = 3
	// oneQuorum = 2
	C := &consensus.SignedCheckpoint{
		Checkpoint: &consensus.Checkpoint{
			ExecuteState: &consensus.Checkpoint_ExecuteState{
				Height: uint64(5),
				Digest: "msg",
			},
		},
	}
	CSet := []*consensus.SignedCheckpoint{C}
	P1 := &consensus.VcPq{
		SequenceNumber: 5,
		BatchDigest:    "msg",
		View:           1,
	}
	// with another sequenceNumber
	P2 := &consensus.VcPq{
		SequenceNumber: 10,
		BatchDigest:    "msgHigh",
		View:           0,
	}
	PSet := []*consensus.VcPq{P1, P2}
	Q1 := &consensus.VcPq{
		SequenceNumber: 5,
		BatchDigest:    "msg",
		View:           1,
	}
	Q2 := &consensus.VcPq{
		SequenceNumber: 10,
		BatchDigest:    "msgHigh",
		View:           0,
	}
	QSet := []*consensus.VcPq{Q1, Q2}
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

	expectList := []*consensus.VcPq{
		{SequenceNumber: 5, BatchDigest: "msg"},
		{SequenceNumber: 6, BatchDigest: ""},
		{SequenceNumber: 7, BatchDigest: ""},
		{SequenceNumber: 8, BatchDigest: ""},
		{SequenceNumber: 9, BatchDigest: ""},
		{SequenceNumber: 10, BatchDigest: "msgHigh"},
	}

	assert.EqualValues(t, expectList, list)
}
