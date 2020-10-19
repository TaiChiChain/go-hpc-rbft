package rbft

import (
	"testing"

	"github.com/ultramesh/flato-common/types/protos"
	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestVC_FullProcess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	tx := newTx()
	rbfts[1].batchMgr.requestPool.AddNewRequest(tx, false, true)

	rbfts[2].sendViewChange()
	vcNode3 := nodes[2].broadcastMessageCache
	assert.Equal(t, pb.Type_VIEW_CHANGE, vcNode3.Type)

	rbfts[3].sendViewChange()
	vcNode4 := nodes[3].broadcastMessageCache
	assert.Equal(t, pb.Type_VIEW_CHANGE, vcNode4.Type)

	rbfts[1].processEvent(vcNode3)
	quorumEvent := rbfts[1].processEvent(vcNode4)

	ev := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeQuorumEvent,
	}
	assert.Equal(t, ev, quorumEvent)

	done := rbfts[1].processEvent(ev)
	nv := nodes[1].broadcastMessageCache
	doneEvent := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangedEvent,
	}
	assert.Equal(t, pb.Type_NEW_VIEW, nv.Type)
	assert.Equal(t, doneEvent, done)

	rbfts[2].processEvent(nv)
	rbfts[3].processEvent(nv)
	assert.Equal(t, uint64(1), rbfts[1].view)
	assert.Equal(t, uint64(1), rbfts[2].view)
	assert.Equal(t, uint64(1), rbfts[3].view)

	assert.NotEqual(t, pb.Type_PRE_PREPARE, nodes[1].broadcastMessageCache.Type)
	rbfts[1].handleViewChangeEvent(doneEvent)
	assert.Equal(t, pb.Type_PRE_PREPARE, nodes[1].broadcastMessageCache.Type)
}

func TestVC_sendViewChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	rbfts[0].sendViewChange()
	assert.Equal(t, uint64(1), rbfts[0].view)
	assert.Equal(t, pb.Type_VIEW_CHANGE, nodes[0].broadcastMessageCache.Type)
}

func TestVC_recvViewChange_FromPrimary(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	rbfts[0].sendViewChange()
	assert.Equal(t, uint64(1), rbfts[0].view)

	vcPayload := nodes[0].broadcastMessageCache.Payload
	vc := &pb.ViewChange{}
	_ = proto.Unmarshal(vcPayload, vc)

	rbfts[1].atomicOff(Pending)
	rbfts[1].setNormal()
	rbfts[1].recvViewChange(vc)

	assert.Equal(t, pb.Type_VIEW_CHANGE, nodes[1].broadcastMessageCache.Type)

	// resend it
	ret := rbfts[1].recvViewChange(vc)
	nodes[1].broadcastMessageCache = nil
	assert.Nil(t, ret)
	assert.Nil(t, nodes[1].broadcastMessageCache)

	vc2 := &pb.ViewChange{
		Basis: &pb.VcBasis{
			ReplicaId: uint64(1),
			View:      uint64(1),
			H:         uint64(0),
		},
	}
	// resend a vc from primary with different values
	vc2.Basis.Cset = []*pb.Vc_C{
		{
			SequenceNumber: uint64(0),
			Digest:         "XXX GENESIS",
		},
	}
	vc2.Basis.Qset = []*pb.Vc_PQ{
		{
			SequenceNumber: uint64(1),
			BatchDigest:    "batch-number-1",
			View:           uint64(0),
		},
	}
	nodes[1].broadcastMessageCache = nil

	assert.Equal(t, vc, rbfts[1].vcMgr.viewChangeStore[vcIdx{v: uint64(1), id: uint64(1)}])
	ret = rbfts[1].recvViewChange(vc2)
	assert.Nil(t, ret)
	assert.Equal(t, vc2, rbfts[1].vcMgr.viewChangeStore[vcIdx{v: uint64(1), id: uint64(1)}])
}

func TestVC_recvViewChange_FromSmallerView(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	rbfts[3].sendViewChange()

	vcPayload := nodes[3].broadcastMessageCache.Payload
	vc := &pb.ViewChange{}
	_ = proto.Unmarshal(vcPayload, vc)

	rbfts[1].atomicOff(Pending)
	rbfts[1].setNormal()
	rbfts[1].setView(uint64(5))
	ret := rbfts[1].recvViewChange(vc)

	assert.Nil(t, nodes[1].broadcastMessageCache)
	assert.Nil(t, ret)
}

func TestVC_recvViewChange_Quorum(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	rbfts[1].sendViewChange()
	rbfts[3].sendViewChange()

	vcPayload1 := nodes[1].broadcastMessageCache.Payload
	vc1 := &pb.ViewChange{}
	_ = proto.Unmarshal(vcPayload1, vc1)

	vcPayload3 := nodes[3].broadcastMessageCache.Payload
	vc3 := &pb.ViewChange{}
	_ = proto.Unmarshal(vcPayload3, vc3)

	// get quorum
	rbfts[2].recvViewChange(vc1)
	ret := rbfts[2].recvViewChange(vc3)
	exp := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeQuorumEvent,
	}
	assert.Equal(t, exp, ret)

	// from recovery to view-change
	rbfts[0].sendViewChange()
	vcPayload0 := nodes[0].broadcastMessageCache.Payload
	vc0 := &pb.ViewChange{}
	_ = proto.Unmarshal(vcPayload0, vc0)
	rbfts[2].restartRecovery()
	assert.Equal(t, true, rbfts[2].atomicIn(InRecovery))
	ret2 := rbfts[2].recvViewChange(vc0)
	assert.Equal(t, exp, ret2)
	assert.Equal(t, false, rbfts[2].atomicIn(InRecovery))
}

func TestVC_fetchRequestBatches(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()

	rbfts[2].fetchRequestBatches()
	assert.Nil(t, nodes[2].broadcastMessageCache)

	rbfts[2].storeMgr.missingReqBatches["lost-batch"] = true
	rbfts[2].fetchRequestBatches()
	assert.Equal(t, pb.Type_FETCH_REQUEST_BATCH, nodes[2].broadcastMessageCache.Type)
}

func TestVC_recvFetchRequestBatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()

	rbfts[2].storeMgr.missingReqBatches["lost-batch"] = true
	rbfts[2].fetchRequestBatches()
	fr := nodes[2].broadcastMessageCache
	assert.Equal(t, pb.Type_FETCH_REQUEST_BATCH, fr.Type)

	ret1 := rbfts[1].processEvent(fr)
	assert.Nil(t, ret1)
	assert.Nil(t, nodes[1].unicastMessageCache)

	rbfts[1].storeMgr.batchStore["lost-batch"] = &pb.RequestBatch{}
	ret2 := rbfts[1].processEvent(fr)
	assert.Nil(t, ret2)
	assert.Equal(t, pb.Type_SEND_REQUEST_BATCH, nodes[1].unicastMessageCache.Type)
}

func TestVC_recvSendRequestBatch_AfterViewChanged(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()

	// a view-change from view=0 to view=1
	rbfts[2].sendViewChange()
	rbfts[3].sendViewChange()
	vcPayload2 := nodes[2].broadcastMessageCache.Payload
	vc2 := &pb.ViewChange{}
	_ = proto.Unmarshal(vcPayload2, vc2)
	vcPayload3 := nodes[3].broadcastMessageCache.Payload
	vc3 := &pb.ViewChange{}
	_ = proto.Unmarshal(vcPayload3, vc3)
	rbfts[1].recvViewChange(vc2)
	ret := rbfts[1].recvViewChange(vc3)
	exp := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeQuorumEvent,
	}
	assert.Equal(t, exp, ret)

	// get the new-view
	rbfts[1].sendNewView(false)
	nvMessage := nodes[1].broadcastMessageCache
	nv := &pb.NewView{}
	_ = proto.Unmarshal(nvMessage.Payload, nv)

	// store the new-view from primary
	rbfts[2].vcMgr.newViewStore[nv.View] = nv

	// start the test for recv nil sending request batch
	ret1 := rbfts[2].recvSendRequestBatch(nil)
	assert.Nil(t, ret1)

	// construct a sending request batch message
	tx := newTx()
	batch := &pb.RequestBatch{
		RequestHashList: []string{"tx-hash"},
		RequestList:     []*protos.Transaction{tx},
		SeqNo:           uint64(1),
		LocalList:       []bool{true},
		BatchHash:       "lost-batch",
	}
	sr := &pb.SendRequestBatch{
		ReplicaId:   uint64(2),
		Batch:       batch,
		BatchDigest: batch.BatchHash,
	}

	// the request batch is not the one we want
	rbfts[2].storeMgr.missingReqBatches["lost-batch-another"] = true
	rbfts[2].recvSendRequestBatch(sr)
	assert.Equal(t, true, rbfts[2].storeMgr.missingReqBatches["lost-batch-another"])
	delete(rbfts[2].storeMgr.missingReqBatches, "lost-batch-another")

	// recv the request batch we want
	rbfts[2].storeMgr.missingReqBatches["lost-batch"] = true
	rbfts[2].atomicOn(InViewChange)
	ret3 := rbfts[2].recvSendRequestBatch(sr)
	exp3 := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangedEvent,
	}
	assert.Equal(t, false, rbfts[1].storeMgr.missingReqBatches["lost-batch"])
	assert.Equal(t, exp3, ret3)
}

func TestVC_processNewView_AfterViewChanged_PrimaryNormal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()

	tx := newTx()
	rbfts[0].batchMgr.requestPool.AddNewRequest(tx, false, true)
	batch := rbfts[0].batchMgr.requestPool.GenerateRequestBatch()

	// a message list
	msgList := xset{
		uint64(1): "",
		uint64(2): "",
		uint64(3): batch[0].BatchHash,
	}

	rbfts[0].putBackRequestBatches(msgList)

	batch3 := &pb.RequestBatch{
		RequestHashList: batch[0].TxHashList,
		RequestList:     batch[0].TxList,
		Timestamp:       batch[0].Timestamp,
		LocalList:       batch[0].LocalList,
		BatchHash:       batch[0].BatchHash,
		SeqNo:           uint64(3),
	}
	rbfts[0].storeMgr.batchStore[batch[0].BatchHash] = batch3
	rbfts[0].processNewView(msgList)
	assert.Equal(t, uint64(3), rbfts[0].batchMgr.seqNo)
	assert.Nil(t, nodes[0].broadcastMessageCache)
}

func TestVC_processNewView_AfterViewChanged_ReplicaNormal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()

	tx := newTx()
	rbfts[0].batchMgr.requestPool.AddNewRequest(tx, false, true)
	batch := rbfts[0].batchMgr.requestPool.GenerateRequestBatch()

	// a message list
	msgList := xset{
		uint64(1): "",
		uint64(2): "",
		uint64(3): batch[0].BatchHash,
	}

	rbfts[0].putBackRequestBatches(msgList)

	batch3 := &pb.RequestBatch{
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
	assert.Equal(t, pb.Type_COMMIT, nodes[0].broadcastMessageCache.Type)
}

func TestVC_feedMissingReqBatchIfNeeded(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()

	tx := newTx()
	rbfts[0].batchMgr.requestPool.AddNewRequest(tx, false, true)
	batch := rbfts[0].batchMgr.requestPool.GenerateRequestBatch()

	// a message list
	msgList := xset{
		uint64(1): "",
		uint64(2): "",
		uint64(3): batch[0].BatchHash,
	}

	rbfts[0].putBackRequestBatches(msgList)

	batch3 := &pb.RequestBatch{
		RequestHashList: batch[0].TxHashList,
		RequestList:     batch[0].TxList,
		Timestamp:       batch[0].Timestamp,
		LocalList:       batch[0].LocalList,
		BatchHash:       batch[0].BatchHash,
		SeqNo:           uint64(3),
	}
	rbfts[0].storeMgr.batchStore[batch[0].BatchHash] = batch3

	flag1 := rbfts[0].feedMissingReqBatchIfNeeded(msgList)
	assert.Equal(t, false, flag1)

	delete(rbfts[0].storeMgr.batchStore, batch3.BatchHash)
	flag2 := rbfts[0].feedMissingReqBatchIfNeeded(msgList)
	assert.Equal(t, true, flag2)
}

func TestVC_correctViewChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()

	rbfts[2].sendViewChange()
	vcPayload := nodes[2].broadcastMessageCache.Payload
	vc := &pb.ViewChange{}
	_ = proto.Unmarshal(vcPayload, vc)

	flag1 := rbfts[0].correctViewChange(vc)
	assert.Equal(t, true, flag1)

	p := &pb.Vc_PQ{
		SequenceNumber: uint64(19),
		BatchDigest:    "batch-number-19",
		View:           uint64(2),
	}
	pSet := []*pb.Vc_PQ{p}

	vcError1 := &pb.ViewChange{
		Basis: &pb.VcBasis{
			View: uint64(1),
			H:    uint64(20),
			Pset: pSet,
		},
	}
	flag2 := rbfts[0].correctViewChange(vcError1)
	assert.Equal(t, false, flag2)

	c := &pb.Vc_C{
		SequenceNumber: uint64(10),
		Digest:         "block-number-10",
	}
	cSet := []*pb.Vc_C{c}
	vcError2 := &pb.ViewChange{
		Basis: &pb.VcBasis{
			View: uint64(1),
			H:    uint64(60),
			Cset: cSet,
		},
	}
	flag3 := rbfts[0].correctViewChange(vcError2)
	assert.Equal(t, false, flag3)
}

func TestVC_assignSequenceNumbers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()

	//Quorum = 3
	//oneQuorum = 2
	C := &pb.Vc_C{
		SequenceNumber: 5,
		Digest:         "msg",
	}
	CSet := []*pb.Vc_C{C}
	P1 := &pb.Vc_PQ{
		SequenceNumber: 5,
		BatchDigest:    "msg",
		View:           1,
	}
	// with another sequenceNumber
	P2 := &pb.Vc_PQ{
		SequenceNumber: 10,
		BatchDigest:    "msgHigh",
		View:           0,
	}
	PSet := []*pb.Vc_PQ{P1, P2}
	Q1 := &pb.Vc_PQ{
		SequenceNumber: 5,
		BatchDigest:    "msg",
		View:           1,
	}
	Q2 := &pb.Vc_PQ{
		SequenceNumber: 10,
		BatchDigest:    "msgHigh",
		View:           0,
	}
	QSet := []*pb.Vc_PQ{Q1, Q2}
	Basis := &pb.VcBasis{
		ReplicaId: 1,
		View:      1,
		H:         4,
		Cset:      CSet,
		Pset:      PSet,
		Qset:      QSet,
	}
	BasisHighH := &pb.VcBasis{
		ReplicaId: 1,
		View:      1,
		H:         6,
		Cset:      CSet,
		Pset:      PSet,
		Qset:      QSet,
	}
	set := []*pb.VcBasis{BasisHighH, Basis, Basis, Basis}

	list := rbfts[1].assignSequenceNumbers(set, 4)
	ret := map[uint64]string{
		uint64(5):  "msg",
		uint64(6):  "",
		uint64(7):  "",
		uint64(8):  "",
		uint64(9):  "",
		uint64(10): "msgHigh",
	}

	assert.Equal(t, ret, list)
}
