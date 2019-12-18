package rbft

import (
	"reflect"
	"testing"
	"time"

	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

//sendViewChange and beforeSendVC
//several recvViewChange
func TestVC_sendViewChange_and_beforeSendVC_sendNewView(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFTReplica(ctrl)

	rbft.on(InRecovery)
	rbft.sendViewChange()

	// close recovery
	assert.Equal(t, false, rbft.in(InRecovery))

	// rbft.view++
	assert.Equal(t, uint64(1), rbft.view)

	// correct vc
	rbft.peerPool.localID = uint64(2)
	vcNode2 := &pb.ViewChange{
		Basis: rbft.getVcBasis(),
	}
	rbft.peerPool.localID = uint64(3)
	vcNode3 := &pb.ViewChange{
		Basis: rbft.getVcBasis(),
	}
	rbft.peerPool.localID = uint64(4)
	vcNode4 := &pb.ViewChange{
		Basis: rbft.getVcBasis(),
	}

	// wrong vc
	vcNodeWrong1 := &pb.ViewChange{
		Basis: rbft.getVcBasis(),
	}
	P := &pb.Vc_PQ{
		SequenceNumber: 0,
		BatchDigest:    "",
		View:           0,
	}
	vcNodeWrong1.Basis.Pset = []*pb.Vc_PQ{P}

	vcNodeWrong2 := &pb.ViewChange{
		Basis: rbft.getVcBasis(),
	}
	vcNodeWrong2.Basis.View = 0

	rbft.peerPool.localID = uint64(2)
	rbft.recvViewChange(vcNode3)

	// could not recv wrong msg
	retNil1 := rbft.recvViewChange(vcNode2)
	retNil2 := rbft.recvViewChange(vcNodeWrong1)
	retNil3 := rbft.recvViewChange(vcNodeWrong2)

	// recv correct msg and reach quorum
	retEvent := rbft.recvViewChange(vcNode4)
	expEvent := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangeQuorumEvent,
	}

	assert.Nil(t, retNil1)
	assert.Nil(t, retNil2)
	assert.Nil(t, retNil3)
	assert.Equal(t, expEvent, retEvent)

	// Primary send new view
	rbft.peerPool.localID = uint64(2)
	retEvent = rbft.sendNewView(false)
	expEvent = &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangedEvent,
	}
	assert.Equal(t, expEvent, retEvent)
}

func TestVC_recvViewChange_normalPrimary_sendViewChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFTReplica(ctrl)

	rbft.peerPool.localID = uint64(1)
	vcNode1 := &pb.ViewChange{
		Basis: rbft.getVcBasis(),
	}

	rbft.peerPool.localID = uint64(2)
	rbft.on(Normal)
	payload, _ := proto.Marshal(vcNode1)
	event := &pb.ConsensusMessage{
		Type:    pb.Type_VIEW_CHANGE,
		Payload: payload,
	}
	rbft.processEvent(event)
	assert.Equal(t, uint64(1), rbft.view)
}

func TestVC_recvViewChange_enoughViewChange_inGreaterView(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFTReplica(ctrl)

	// If there are msg in rbft.vcMgr.viewChangeStore
	// Use them directly
	// Change view when reach quorum
	vcID1 := vcIdx{
		v:  3,
		id: 1,
	}
	vc1 := &pb.ViewChange{
		Basis:     nil,
		Signature: nil,
		Timestamp: time.Now().UnixNano(),
	}
	vcID2 := vcIdx{
		v:  3,
		id: 2,
	}
	vc2 := &pb.ViewChange{
		Basis:     nil,
		Signature: nil,
		Timestamp: time.Now().UnixNano(),
	}
	vcID3 := vcIdx{
		v:  3,
		id: 3,
	}
	vc3 := &pb.ViewChange{
		Basis:     nil,
		Signature: nil,
		Timestamp: time.Now().UnixNano(),
	}

	rbft.vcMgr.viewChangeStore[vcID1] = vc1
	rbft.vcMgr.viewChangeStore[vcID2] = vc2
	rbft.vcMgr.viewChangeStore[vcID3] = vc3

	vcNode1 := &pb.ViewChange{
		Basis: rbft.getVcBasis(),
	}
	rbft.recvViewChange(vcNode1)
	assert.Equal(t, uint64(3), rbft.view)
}

func TestVC_recvFetchRequestBatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	fr := &pb.FetchRequestBatch{
		ReplicaId:   1,
		BatchDigest: "msg",
	}

	rbft.storeMgr.batchStore["msg"] = &pb.RequestBatch{}
	payload, _ := proto.Marshal(fr)
	event := &pb.ConsensusMessage{
		Type:    pb.Type_FETCH_REQUEST_BATCH,
		Payload: payload,
	}
	ret := rbft.processEvent(event)
	assert.Nil(t, ret)
}

func TestVC_recvSendRequestBatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	// return nil
	ret := rbft.recvSendRequestBatch(nil)
	assert.Nil(t, ret)

	batch := &pb.RequestBatch{
		RequestHashList: []string{"Hash11", "Hash12"},
		RequestList:     mockRequestLists,
		Timestamp:       time.Now().UnixNano(),
		SeqNo:           2,
		LocalList:       []bool{true, true},
		BatchHash:       calculateMD5Hash([]string{"Hash11", "Hash12"}, 1),
	}
	batchToSend := &pb.SendRequestBatch{
		ReplicaId:   1,
		Batch:       batch,
		BatchDigest: calculateMD5Hash([]string{"Hash11", "Hash12"}, 1),
	}

	ret = rbft.recvSendRequestBatch(batchToSend)
	assert.Nil(t, ret)

	rbft.storeMgr.missingReqBatches[batch.BatchHash] = true
	rbft.on(InViewChange)
	rbft.vcMgr.newViewStore[rbft.view] = &pb.NewView{}
	ret = rbft.recvSendRequestBatch(batchToSend)
	assert.Equal(t, false, rbft.storeMgr.missingReqBatches[batch.BatchHash])
	assert.Equal(t, batch, rbft.storeMgr.batchStore[batch.BatchHash])
	expEvent := &LocalEvent{
		Service:   ViewChangeService,
		EventType: ViewChangedEvent,
	}
	assert.Equal(t, expEvent, ret)

	rbft.off(InViewChange)
	rbft.storeMgr.missingReqBatches[batch.BatchHash] = true
	rbft.on(InRecovery)
	rbft.vcMgr.newViewStore[rbft.view] = &pb.NewView{}
	rbft.recoveryMgr.recoveryHandled = false
	payload, _ := proto.Marshal(batchToSend)
	event := &pb.ConsensusMessage{
		Type:    pb.Type_SEND_REQUEST_BATCH,
		Payload: payload,
	}
	ret = rbft.processEvent(event)
	assert.Equal(t, false, rbft.storeMgr.missingReqBatches[batch.BatchHash])
	assert.Equal(t, batch, rbft.storeMgr.batchStore[batch.BatchHash])
	expEvent = &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoveryDoneEvent,
	}
	assert.Equal(t, expEvent, ret)
}

func TestVC_getViewChangeBasis(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	vbTmp := &pb.VcBasis{}
	vcTmp := &pb.ViewChange{Basis: vbTmp}
	rbft.vcMgr.viewChangeStore = map[vcIdx]*pb.ViewChange{
		{uint64(1), uint64(1)}: vcTmp,
	}
	b := rbft.getViewChangeBasis()
	assert.Equal(t, []*pb.VcBasis{vbTmp}, b)
}

func TestVC_recvNewView(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFTReplica(ctrl)

	nv := &pb.NewView{View: 1}
	rbft.off(InViewChange)
	rbft.off(InRecovery)
	ret := rbft.recvNewView(nv)
	assert.Nil(t, ret)

	rbft.on(InViewChange)
	ret = rbft.recvNewView(nv)
	assert.Nil(t, ret)

	nv.View = 1
	nv.ReplicaId = uint64(2)
	payload, _ := proto.Marshal(nv)
	event := &pb.ConsensusMessage{
		Type:    pb.Type_NEW_VIEW,
		Payload: payload,
	}
	rbft.processEvent(event)
	assert.Equal(t, nv, rbft.vcMgr.newViewStore[nv.View])
}

func TestVC_replicaCheckNewView(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	assert.Nil(t, rbft.replicaCheckNewView())

	rbft.vcMgr.newViewStore[rbft.view] = &pb.NewView{}
	rbft.off(InViewChange)
	rbft.off(InRecovery)
	assert.Nil(t, rbft.replicaCheckNewView())

	rbft.on(InViewChange)
	rbft.replicaCheckNewView()
	assert.Equal(t, uint64(1), rbft.view)
	rbft.view--

	C := &pb.Vc_C{
		SequenceNumber: 4,
		Digest:         "",
	}
	CSet := []*pb.Vc_C{C}
	P1 := &pb.Vc_PQ{
		SequenceNumber: 5,
		BatchDigest:    "msg5",
		View:           1,
	}
	PSet := []*pb.Vc_PQ{P1}
	Q1 := &pb.Vc_PQ{
		SequenceNumber: 5,
		BatchDigest:    "msg5",
		View:           1,
	}
	QSet := []*pb.Vc_PQ{Q1}
	Basis := &pb.VcBasis{
		ReplicaId: 1,
		View:      1,
		H:         4,
		Cset:      CSet,
		Pset:      PSet,
		Qset:      QSet,
	}

	set := []*pb.VcBasis{Basis, Basis, Basis}
	rbft.view = 0

	rbft.vcMgr.newViewStore[rbft.view] = &pb.NewView{
		ReplicaId: 1,
		View:      0,
		Xset:      nil,
		Bset:      set,
	}
	rbft.replicaCheckNewView()
	assert.Equal(t, uint64(1), rbft.view)
	rbft.view--

	P1.BatchDigest = ""
	Q1.BatchDigest = ""
	assert.Equal(t, nil, rbft.replicaCheckNewView())
}

func TestVC_resetStateForNewView(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	assert.Nil(t, rbft.resetStateForNewView())

	rbft.vcMgr.newViewStore[rbft.view] = &pb.NewView{}
	rbft.off(InViewChange)
	rbft.off(InRecovery)
	assert.Nil(t, rbft.resetStateForNewView())

	rbft.on(InViewChange)
	rbft.vcMgr.vcHandled = true
	assert.Nil(t, rbft.resetStateForNewView())
	rbft.off(InViewChange)

	rbft.on(InRecovery)
	rbft.recoveryMgr.recoveryHandled = true
	assert.Nil(t, rbft.resetStateForNewView())
	rbft.off(InRecovery)

	rbft.on(InViewChange)
	rbft.vcMgr.vcHandled = false
	rbft.recoveryMgr.recoveryHandled = false
	assert.Equal(t, reflect.TypeOf(&LocalEvent{}), reflect.TypeOf(rbft.resetStateForNewView()))
	rbft.off(InViewChange)

	prePrepareTmp := &pb.PrePrepare{
		ReplicaId:      1,
		View:           0,
		SequenceNumber: 15,
		BatchDigest:    "",
		HashBatch:      &pb.HashBatch{Timestamp: 10086},
	}
	prePareTmp1 := &pb.Prepare{
		ReplicaId:      1,
		View:           prePrepareTmp.View,
		SequenceNumber: prePrepareTmp.SequenceNumber,
		BatchDigest:    prePrepareTmp.BatchDigest,
	}
	prePareTmp2 := &pb.Prepare{
		ReplicaId:      2,
		View:           prePrepareTmp.View,
		SequenceNumber: prePrepareTmp.SequenceNumber,
		BatchDigest:    prePrepareTmp.BatchDigest,
	}
	prePareTmp3 := &pb.Prepare{
		ReplicaId:      3,
		View:           prePrepareTmp.View,
		SequenceNumber: prePrepareTmp.SequenceNumber,
		BatchDigest:    prePrepareTmp.BatchDigest,
	}
	commitTmp1 := &pb.Commit{
		ReplicaId:      1,
		View:           prePrepareTmp.View,
		SequenceNumber: prePrepareTmp.SequenceNumber,
		BatchDigest:    prePrepareTmp.BatchDigest,
	}
	commitTmp2 := &pb.Commit{
		ReplicaId:      2,
		View:           prePrepareTmp.View,
		SequenceNumber: prePrepareTmp.SequenceNumber,
		BatchDigest:    prePrepareTmp.BatchDigest,
	}
	commitTmp3 := &pb.Commit{
		ReplicaId:      3,
		View:           prePrepareTmp.View,
		SequenceNumber: prePrepareTmp.SequenceNumber,
		BatchDigest:    prePrepareTmp.BatchDigest,
	}
	msgIDTmp := msgID{
		v: prePrepareTmp.View,
		n: prePrepareTmp.SequenceNumber,
		d: prePrepareTmp.BatchDigest,
	}
	certTmp := &msgCert{
		prePrepare:  prePrepareTmp,
		sentPrepare: false,
		prepare:     map[pb.Prepare]bool{*prePareTmp1: true, *prePareTmp2: true, *prePareTmp3: true},
		sentCommit:  false,
		commit:      map[pb.Commit]bool{*commitTmp1: true, *commitTmp2: true, *commitTmp3: true},
		sentExecute: false,
	}

	rbft.on(InRecovery)
	rbft.vcMgr.vcHandled = false
	rbft.recoveryMgr.recoveryHandled = false
	rbft.view = 1
	rbft.vcMgr.newViewStore[rbft.view] = &pb.NewView{Xset: map[uint64]string{uint64(1): "msg"}}
	rbft.storeMgr.certStore[msgIDTmp] = certTmp
	rbft.storeMgr.batchStore["msg"] = &pb.RequestBatch{SeqNo: uint64(2)}
	assert.Equal(t, reflect.TypeOf(&LocalEvent{}), reflect.TypeOf(rbft.resetStateForNewView()))
}

func TestVC_feedMissingReqBatchIfNeeded(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	rbft.h = 2
	xSet := map[uint64]string{
		uint64(1): "value1",
		uint64(2): "value2",
		uint64(3): "value3",
		uint64(4): "",
	}
	assert.Equal(t, true, rbft.feedMissingReqBatchIfNeeded(xSet))
}

func TestVC_assignSequenceNumbers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	/**********************
	type VcBasis struct {
		ReplicaId uint64   `protobuf:"varint,1,opt,name=replica_id,json=replicaId,proto3" json:"replica_id,omitempty"`
		View      uint64   `protobuf:"varint,2,opt,name=view,proto3" json:"view,omitempty"`
		H         uint64   `protobuf:"varint,3,opt,name=h,proto3" json:"h,omitempty"`
		Cset      []*Vc_C  `protobuf:"bytes,4,rep,name=cset,proto3" json:"cset,omitempty"`
		Pset      []*Vc_PQ `protobuf:"bytes,5,rep,name=pset,proto3" json:"pset,omitempty"`
		Qset      []*Vc_PQ `protobuf:"bytes,6,rep,name=qset,proto3" json:"qset,omitempty"`
	}
	type Vc_PQ struct {
		SequenceNumber uint64 `protobuf:"varint,1,opt,name=sequence_number,json=sequenceNumber,proto3" json:"sequence_number,omitempty"`
		BatchDigest    string `protobuf:"bytes,2,opt,name=batch_digest,json=batchDigest,proto3" json:"batch_digest,omitempty"`
		View           uint64 `protobuf:"varint,3,opt,name=view,proto3" json:"view,omitempty"`
	}
	type Vc_C struct {
		SequenceNumber uint64 `protobuf:"varint,1,opt,name=sequence_number,json=sequenceNumber,proto3" json:"sequence_number,omitempty"`
		Digest         string `protobuf:"bytes,2,opt,name=digest,proto3" json:"digest,omitempty"`
	}
	********************/
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

	list := rbft.assignSequenceNumbers(set, 4)
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

func TestVC_processNewView(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFTReplica(ctrl)

	xset := map[uint64]string{
		uint64(1): "msg1",
		uint64(2): "msg2",
		uint64(3): "msg3",
		uint64(6): "msg6",
		uint64(5): "msg5",
	}

	rbft.storeMgr.batchStore["msg5"] = &pb.RequestBatch{
		RequestHashList: nil,
		RequestList:     nil,
		Timestamp:       10086,
		SeqNo:           0,
		LocalList:       nil,
		BatchHash:       "msg5",
	}
	rbft.exec.setLastExec(uint64(8))
	assert.Equal(t, (*msgCert)(nil), rbft.storeMgr.certStore[msgID{rbft.view, 2, "msg2"}])

	rbft.processNewView(xset)

	prePrepareTmp := &pb.PrePrepare{
		ReplicaId:      1,
		View:           0,
		SequenceNumber: 5,
		BatchDigest:    "msg5",
		HashBatch:      &pb.HashBatch{Timestamp: 10086},
	}
	prePrepTmp := pb.Prepare{
		ReplicaId:      2,
		View:           0,
		SequenceNumber: 5,
		BatchDigest:    "msg5",
	}
	commitTmp := pb.Commit{
		ReplicaId:      2,
		View:           0,
		SequenceNumber: 5,
		BatchDigest:    "msg5",
	}
	certTmp := &msgCert{
		prePrepare:  prePrepareTmp,
		sentPrepare: true,
		prepare:     map[pb.Prepare]bool{prePrepTmp: true},
		sentCommit:  true,
		commit:      map[pb.Commit]bool{commitTmp: true},
		sentExecute: false,
	}

	assert.Equal(t, certTmp, rbft.storeMgr.certStore[msgID{rbft.view, 5, "msg5"}])
}
