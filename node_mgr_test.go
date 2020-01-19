package rbft

import (
	"testing"
	"time"

	mockexternal "github.com/ultramesh/flato-rbft/mock/mock_external"
	pb "github.com/ultramesh/flato-rbft/rbftpb"
	txpoolmock "github.com/ultramesh/flato-txpool/mock"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

// Process Test AddNormal
// Test for primary Add New Node
// Eventually primary state: rbft.N from 4 to 5
// With serial state changes in process
func TestNodeMgr_recvAgreeUpdateN_AddNormal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	pool := txpoolmock.NewMockMinimalTxPool(ctrl)
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)

	conf := Config{
		ID:                      1,
		IsNew:                   false,
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
		UpdateTimeout:           4 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger:      log,
		External:    external,
		RequestPool: pool,
	}

	node, _ := newNode(conf)
	rbft := node.rbft

	// success msg for add node
	C := &pb.Vc_C{
		SequenceNumber: 0,
		Digest:         "XXX GENESIS",
	}
	CSet := []*pb.Vc_C{C}
	var PSet []*pb.Vc_PQ
	var QSet []*pb.Vc_PQ

	basisTmpAdd1 := &pb.VcBasis{
		ReplicaId: 1,
		View:      5,
		H:         0,
		Cset:      CSet,
		Pset:      PSet,
		Qset:      QSet,
	}
	basisTmpAdd2 := &pb.VcBasis{
		ReplicaId: 2,
		View:      5,
		H:         0,
		Cset:      CSet,
		Pset:      PSet,
		Qset:      QSet,
	}
	basisTmpAdd3 := &pb.VcBasis{
		ReplicaId: 3,
		View:      5,
		H:         0,
		Cset:      CSet,
		Pset:      PSet,
		Qset:      QSet,
	}

	agreeAdd2 := &pb.AgreeUpdateN{
		Basis:   basisTmpAdd2,
		Flag:    true, //add
		Id:      5,
		Info:    "",
		N:       5,
		ExpectN: 5,
	}
	agreeAdd3 := &pb.AgreeUpdateN{
		Basis:   basisTmpAdd3,
		Flag:    true, //add
		Id:      5,
		Info:    "",
		N:       5,
		ExpectN: 5,
	}

	// A success recvAgreeUpdate process, return a LocalEvent to Add
	// quorum: node2, node3, node1
	rbft.recvAgreeUpdateN(agreeAdd2)
	objAdd := rbft.recvAgreeUpdateN(agreeAdd3)
	retAdd := &LocalEvent{
		Service:   NodeMgrService,
		EventType: NodeMgrAgreeUpdateQuorumEvent,
	}
	assert.Equal(t, retAdd, objAdd)

	// Primary success send UpdateN
	// rbft.nodeMgr.updateStore[rbft.nodeMgr.updateTarget] = update
	// Then, primaryCheckUpdateN
	// return NodeMgrUpdatedEvent
	_ = rbft.sendUpdateN()
	BSetTmp := []*pb.VcBasis{
		basisTmpAdd2,
		basisTmpAdd3,
		basisTmpAdd1,
	}
	updateTmp := &pb.UpdateN{
		Flag:      rbft.nodeMgr.updateTarget.flag,
		N:         rbft.nodeMgr.updateTarget.n,
		View:      rbft.nodeMgr.updateTarget.v,
		Id:        rbft.nodeMgr.updateTarget.id,
		Xset:      map[uint64]string{},
		ReplicaId: rbft.peerPool.localID,
		Bset:      BSetTmp,
	}
	assert.Equal(t, updateTmp.Bset[2].Cset, rbft.nodeMgr.updateStore[rbft.nodeMgr.updateTarget].Bset[2].Cset)
	assert.Equal(t, 5, rbft.N)
}

// Process Test DelNormal, Not Finished
func TestNodeMgr_recvAgreeUpdateN_DelNormal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	// success msg for del node
	peerDel := &pb.Peer{
		Id:      4,
		Context: nil,
	}
	tmp, _ := proto.Marshal(peerDel)
	infoDel := byte2Hex(tmp)

	C := &pb.Vc_C{
		SequenceNumber: 20,
		Digest:         "msg",
	}
	CSet := []*pb.Vc_C{C}

	PQ := &pb.Vc_PQ{
		SequenceNumber: 20,
		BatchDigest:    "msg",
		View:           3,
	}
	PSet := []*pb.Vc_PQ{PQ}
	QSet := []*pb.Vc_PQ{PQ}

	basisTmpDel2 := &pb.VcBasis{
		ReplicaId: 2,
		View:      3,
		H:         10,
		Cset:      CSet,
		Pset:      PSet,
		Qset:      QSet,
	}
	basisTmpDel3 := &pb.VcBasis{
		ReplicaId: 3,
		View:      3,
		H:         10,
		Cset:      CSet,
		Pset:      PSet,
		Qset:      QSet,
	}

	agreeDel2 := &pb.AgreeUpdateN{
		Basis:   basisTmpDel2,
		Flag:    false, //delele
		Id:      4,
		Info:    infoDel,
		N:       3,
		ExpectN: 3,
	}
	agreeDel3 := &pb.AgreeUpdateN{
		Basis:   basisTmpDel3,
		Flag:    false, //delele
		Id:      4,
		Info:    infoDel,
		N:       3,
		ExpectN: 3,
	}
	// A success recvAgreeUpdate process, return a LocalEvent to Del
	// Send LocalEvent
	objDel1 := rbft.recvAgreeUpdateN(agreeDel2)
	//rbft.recvAgreeUpdateN(agreeDel2)
	objDel2 := rbft.recvAgreeUpdateN(agreeDel3)
	retDel := &LocalEvent{
		Service:   NodeMgrService,
		EventType: NodeMgrAgreeUpdateQuorumEvent,
	}
	assert.Nil(t, objDel1)
	assert.Equal(t, retDel, objDel2)
	// Not Finished
}

func TestNodeMgr_recvLocalDelNode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	pool := txpoolmock.NewMockMinimalTxPool(ctrl)
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)

	conf := Config{
		ID:                      2,
		IsNew:                   false,
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
		UpdateTimeout:           4 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger:      log,
		External:    external,
		RequestPool: pool,
	}

	node, _ := newNode(conf)
	rbft := node.rbft

	// Some nil return
	// An invalid peer
	assert.Nil(t, rbft.recvLocalDelNode(uint64(10)))
	// In view change
	rbft.on(InViewChange)
	assert.Nil(t, rbft.recvLocalDelNode(uint64(4)))
	// In recovery
	rbft.off(InViewChange)
	rbft.on(InRecovery)
	assert.Nil(t, rbft.recvLocalDelNode(uint64(4)))
	// no more than 4 peers
	rbft.off(InRecovery)
	assert.Nil(t, rbft.recvLocalDelNode(uint64(4)))

	peerSetDel := []*pb.Peer{
		{
			Id:      1,
			Context: []byte("Peer1"),
		},
		{
			Id:      2,
			Context: []byte("Peer2"),
		},
		{
			Id:      3,
			Context: []byte("Peer3"),
		},
		{
			Id:      4,
			Context: []byte("Peer4"),
		},
		{
			Id:      5,
			Context: []byte("Peer5"),
		},
	}
	conf.Peers = peerSetDel
	node, _ = newNode(conf)
	rbft = node.rbft
	// success sendAgreeUpdateNForDel
	rbft.recvLocalDelNode(uint64(4))
	tmp, _ := proto.Marshal(peerSetDel[3])
	info := byte2Hex(tmp)
	assert.Equal(t, info, rbft.nodeMgr.delNodeInfo[uint64(4)])
}

func TestNodeMgr_sendReadyForN(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	rbft.off(InUpdatingN)
	rbft.off(isNewNode)
	rbft.sendReadyForN()
	assert.Equal(t, false, rbft.in(InUpdatingN))

	rbft.on(isNewNode)
	rbft.on(InViewChange)
	rbft.sendReadyForN()
	assert.Equal(t, false, rbft.in(InUpdatingN))

	rbft.off(InViewChange)
	rbft.on(InRecovery)
	rbft.sendReadyForN()
	assert.Equal(t, false, rbft.in(InUpdatingN))

	rbft.off(InRecovery)
	rbft.sendReadyForN()
	assert.Equal(t, true, rbft.in(InUpdatingN))
}

func TestNodeMgr_recvReadyForNForAdd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	ready := &pb.ReadyForN{
		ReplicaId:   1,
		NewNodeInfo: "",
		ExpectN:     2,
	}

	// Test for some nil return
	rbft.on(InViewChange)
	assert.Nil(t, rbft.recvReadyForNForAdd(ready))
	rbft.off(InViewChange)

	rbft.on(InRecovery)
	assert.Nil(t, rbft.recvReadyForNForAdd(ready))
	rbft.off(InRecovery)

	ready.ReplicaId = uint64(1)
	assert.Nil(t, rbft.recvReadyForNForAdd(ready))

	// Sent by new node
	peerNew := &pb.Peer{
		Id:      5,
		Context: nil,
	}
	tmp, _ := proto.Marshal(peerNew)
	infoNew := byte2Hex(tmp)
	readyTmp := &pb.ReadyForN{
		ReplicaId:   5,
		NewNodeInfo: infoNew,
		ExpectN:     5,
	}

	rbft.recvReadyForNForAdd(readyTmp)
	assert.Equal(t, infoNew, rbft.nodeMgr.addNodeInfo[readyTmp.ReplicaId])
}

func TestNodeMgr_sendAgreeUpdateNForAdd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	peer := &pb.Peer{Id: 1}
	info, _ := proto.Marshal(peer)
	newNodeInfo := byte2Hex(info)

	// for nil response
	// InPudatingN
	rbft.on(InUpdatingN)
	assert.Nil(t, rbft.sendAgreeUpdateNForAdd(newNodeInfo, uint64(1)))
	// NewNode
	rbft.off(InUpdatingN)
	rbft.on(isNewNode)
	assert.Nil(t, rbft.sendAgreeUpdateNForAdd(newNodeInfo, uint64(1)))
	// expectN is not rbft.N+1
	rbft.off(isNewNode)
	assert.Nil(t, rbft.sendAgreeUpdateNForAdd(newNodeInfo, uint64(1)))

	// A new node, ID is 5
	peerNew := &pb.Peer{Id: 5}
	infoNew, _ := proto.Marshal(peerNew)
	newNodeInfoNew := byte2Hex(infoNew)

	// There is some nodeHashInfo to add, return nil, assuming is node5
	rbft.nodeMgr.addNodeInfo[uint64(5)] = newNodeInfoNew
	assert.Nil(t, rbft.sendAgreeUpdateNForAdd(newNodeInfoNew, uint64(5)))

	// addNodeInfo is empty, to add some new node info
	// delete old message: rbft.nodeMgr.agreeUpdateStore[idx.v<view],
	// which is in old view
	rbft.on(Normal)
	rbft.view = 1
	delete(rbft.nodeMgr.addNodeInfo, uint64(5))
	IDTmp := aidx{
		v:    0,
		n:    10,
		flag: true, // add
		id:   4,    // add node4
	}
	rbft.nodeMgr.agreeUpdateStore[IDTmp] = &pb.AgreeUpdateN{}
	rbft.sendAgreeUpdateNForAdd(newNodeInfoNew, uint64(5))
	assert.Equal(t, false, rbft.in(Normal))
	assert.Nil(t, rbft.nodeMgr.agreeUpdateStore[IDTmp])
}

func TestNodeMgr_sendAgreeUpdateNForDel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	peer := &pb.Peer{Id: 1}
	info, _ := proto.Marshal(peer)
	delNodeInfo := byte2Hex(info)

	// As for nil return
	rbft.on(InUpdatingN)
	assert.Nil(t, rbft.sendAgreeUpdateNForDel(delNodeInfo))
	rbft.off(InUpdatingN)

	// There is some node delInfo to process, return nil
	rbft.nodeMgr.delNodeInfo[uint64(1)] = delNodeInfo
	assert.Nil(t, rbft.sendAgreeUpdateNForDel(delNodeInfo))

	delete(rbft.nodeMgr.delNodeInfo, uint64(1))
	// Will delete the old message
	rbft.nodeMgr.updateTarget = uidx{
		v:    0,
		n:    10,
		flag: false,
		id:   4,
	}
	rbft.nodeMgr.updateStore[rbft.nodeMgr.updateTarget] = &pb.UpdateN{}
	// As for agreeUpdateHelper
	// delete messages that is not in the same view, seqNo, flag
	UIDXTmp := aidx{
		v:    1,
		n:    50,
		flag: true,
		id:   4,
	}
	rbft.nodeMgr.agreeUpdateStore[UIDXTmp] = &pb.AgreeUpdateN{}
	// Run and test
	rbft.sendAgreeUpdateNForDel(delNodeInfo)
	assert.Nil(t, rbft.nodeMgr.updateStore[rbft.nodeMgr.updateTarget])
	assert.Nil(t, rbft.nodeMgr.agreeUpdateStore[UIDXTmp])
}

func TestNodeMgr_recvAgreeUpdateN(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	// A Agree checkAgreeUpdateN return true
	basisTmp := &pb.VcBasis{
		ReplicaId: 1,
		View:      3,
		H:         0,
		Cset:      nil,
		Pset:      nil,
		Qset:      nil,
	}
	agree := &pb.AgreeUpdateN{
		Basis:   basisTmp,
		Flag:    false, //delete
		Id:      1,
		Info:    "",
		N:       3,
		ExpectN: 1,
	}

	// A key for certain agree
	key := aidx{
		v:    agree.Basis.View,
		n:    agree.N,
		flag: agree.Flag,
		id:   agree.Basis.ReplicaId,
	}
	// A key for another agree
	keyAnother := aidx{
		v:    999,
		n:    999,
		flag: true,
		id:   999,
	}

	// As for nil return
	rbft.on(InViewChange)
	assert.Nil(t, rbft.recvAgreeUpdateN(agree))
	rbft.off(InViewChange)
	rbft.on(InRecovery)
	assert.Nil(t, rbft.recvAgreeUpdateN(agree))
	rbft.off(InRecovery)
	agree.N = 1
	assert.Nil(t, rbft.recvAgreeUpdateN(agree))

	// Set Valid Agree.N
	agree.N = 3

	// If there has been an agree of key, return nil
	rbft.nodeMgr.agreeUpdateStore[key] = agree
	rbft.nodeMgr.agreeUpdateStore[keyAnother] = agree
	assert.Nil(t, rbft.recvAgreeUpdateN(agree))

	// Delete agreeUpdateStore[key]
	delete(rbft.nodeMgr.agreeUpdateStore, key)
	// Only one message, return nil, failed
	assert.Nil(t, rbft.recvAgreeUpdateN(agree))
	delete(rbft.nodeMgr.agreeUpdateStore, key)
}

func TestNodeMgr_sendUpdateN(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	rbft.nodeMgr.updateTarget = uidx{
		v:    1,
		n:    20,
		flag: true,
		id:   2,
	}
	rbft.nodeMgr.updateStore[rbft.nodeMgr.updateTarget] = &pb.UpdateN{
		Flag:      true,
		ReplicaId: 0,
		Id:        10086,
		N:         0,
		View:      1,
		Xset:      nil,
		Bset:      nil,
	}

	agree := &pb.AgreeUpdateN{
		Basis:   rbft.getVcBasis(),
		Flag:    false,
		Id:      1,
		Info:    "",
		N:       1,
		ExpectN: 1,
	}
	aID1 := aidx{
		v:    1,
		n:    2,
		flag: false,
		id:   2,
	}
	aID2 := aidx{
		v:    1,
		n:    2,
		flag: false,
		id:   3,
	}
	aID3 := aidx{
		v:    1,
		n:    2,
		flag: false,
		id:   4,
	}

	// Get NodeMgrAgreeUpdateQuorumEvent from recvAgreeUpdateN
	// &LocalEvent{
	//			Service:   NodeMgrService,
	//			EventType: NodeMgrAgreeUpdateQuorumEvent,
	// 			Event:	   nil,
	// }
	// A trigger for sendUpdateN, action for primary
	// As for a replica, check the UpdateN - replicaCheckUpdateN
	//
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

	// Only test for sending process
	// To The End - Change the rbft.nodeMgr.updateStore[rbft.nodeMgr.updateTarget]
	rbft.nodeMgr.updateTarget.id = 10010
	agree.Basis = Basis
	rbft.nodeMgr.agreeUpdateStore[aID1] = agree
	rbft.nodeMgr.agreeUpdateStore[aID2] = agree
	rbft.nodeMgr.agreeUpdateStore[aID3] = agree
	rbft.exec.lastExec = uint64(999)
	assert.Nil(t, rbft.sendUpdateN())
	assert.Equal(t, uint64(10010), rbft.nodeMgr.updateStore[rbft.nodeMgr.updateTarget].Id)
}

func TestNodeMgr_dispatchNodeMgrMsg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	var e consensusEvent
	e = &pb.ReadyForN{}
	assert.Nil(t, rbft.dispatchNodeMgrMsg(e))

	e = &pb.UpdateN{}
	assert.Nil(t, rbft.dispatchNodeMgrMsg(e))

	e = &pb.AgreeUpdateN{
		Basis:   &pb.VcBasis{},
		Flag:    false,
		Id:      0,
		Info:    "",
		N:       0,
		ExpectN: 0,
	}
	assert.Nil(t, rbft.dispatchNodeMgrMsg(e))

	e = nil
	assert.Nil(t, rbft.dispatchNodeMgrMsg(e))
}

func TestNodeMgr_recvUpdateN(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	update := &pb.UpdateN{
		Flag:      true,
		ReplicaId: 1,
		Id:        5,
		N:         20,
		View:      0,
		Xset:      nil,
		Bset:      nil,
	}

	// some nil return
	rbft.on(InViewChange)
	assert.Nil(t, rbft.recvUpdateN(update))
	rbft.off(InViewChange)
	rbft.on(InRecovery)
	assert.Nil(t, rbft.recvUpdateN(update))
	rbft.off(InRecovery)
	rbft.off(InUpdatingN)
	assert.Nil(t, rbft.recvUpdateN(update))
	// is not sent by primary
	update.ReplicaId = 3
	rbft.on(InUpdatingN)
	assert.Nil(t, rbft.recvUpdateN(update))

	// Test: receive the UpdateN msg or not
	// Note: only test for receiving process, it will not pass QuorumTest
	update.ReplicaId = 1
	key := uidx{
		v:    0,
		n:    20,
		flag: true,
		id:   5,
	}
	rbft.recvUpdateN(update)
	assert.Equal(t, rbft.nodeMgr.updateStore[key], update)
}

func TestNodeMgr_replicaCheckUpdateN(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	rbft.nodeMgr.updateTarget = uidx{
		v:    1,
		n:    2,
		flag: false,
		id:   1,
	}

	rbft.nodeMgr.updateStore[rbft.nodeMgr.updateTarget] = &pb.UpdateN{}

	rbft.on(InViewChange)
	assert.Nil(t, rbft.replicaCheckUpdateN())

	rbft.off(InViewChange)
	rbft.on(InRecovery)
	assert.Nil(t, rbft.replicaCheckUpdateN())

	rbft.off(InRecovery)
	rbft.off(InUpdatingN)
	assert.Nil(t, rbft.replicaCheckUpdateN())

	assert.Equal(t, uint64(0), rbft.view)
	rbft.on(InUpdatingN)
	assert.Nil(t, rbft.replicaCheckUpdateN())
	assert.Equal(t, uint64(1), rbft.view)

	rbft.view = 0
	rbft.off(InViewChange)
	rbft.nodeMgr.updateStore[rbft.nodeMgr.updateTarget].Bset = []*pb.VcBasis{rbft.getVcBasis()}
	assert.Nil(t, rbft.replicaCheckUpdateN())
}
