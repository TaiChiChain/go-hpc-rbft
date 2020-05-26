package rbft

import (
	pb "github.com/ultramesh/flato-rbft/rbftpb"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestVC_replicaCheckNewView(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	assert.Nil(t, rbft.replicaCheckNewView())

	rbft.vcMgr.newViewStore[rbft.view] = &pb.NewView{}
	rbft.atomicOff(InViewChange)
	rbft.atomicOff(InRecovery)
	assert.Nil(t, rbft.replicaCheckNewView())

	rbft.atomicOn(InViewChange)
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
	rbft.atomicOff(InViewChange)
	rbft.atomicOff(InRecovery)
	assert.Nil(t, rbft.resetStateForNewView())

	rbft.atomicOn(InViewChange)
	rbft.vcMgr.vcHandled = true
	assert.Nil(t, rbft.resetStateForNewView())
	rbft.atomicOff(InViewChange)

	rbft.atomicOn(InRecovery)
	rbft.recoveryMgr.recoveryHandled = true
	assert.Nil(t, rbft.resetStateForNewView())
	rbft.atomicOff(InRecovery)

	rbft.atomicOn(InViewChange)
	rbft.vcMgr.vcHandled = false
	rbft.recoveryMgr.recoveryHandled = false
	assert.Equal(t, reflect.TypeOf(&LocalEvent{}), reflect.TypeOf(rbft.resetStateForNewView()))
	rbft.atomicOff(InViewChange)

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

	rbft.atomicOn(InRecovery)
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

	rbft.h = 10
	xSet := map[uint64]string{
		uint64(1):  "value1",
		uint64(2):  "value2",
		uint64(3):  "",
		uint64(4):  "value4",
		uint64(5):  "",
		uint64(6):  "",
		uint64(7):  "",
		uint64(8):  "",
		uint64(9):  "",
		uint64(10): "value10",
		uint64(11): "",
		uint64(12): "value12",
	}
	assert.Equal(t, true, rbft.feedMissingReqBatchIfNeeded(xSet))
	assert.Equal(t, false, rbft.storeMgr.missingReqBatches["value1"])
	assert.Equal(t, false, rbft.storeMgr.missingReqBatches["value2"])
	assert.Equal(t, false, rbft.storeMgr.missingReqBatches["value4"])
	assert.Equal(t, true, rbft.storeMgr.missingReqBatches["value12"])
}

func TestVC_assignSequenceNumbers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

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
