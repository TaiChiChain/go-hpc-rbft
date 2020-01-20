package rbft

import (
	"testing"

	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

// newRecoveryMgr has been tested with newRBFT
// Test for iniRecovery and sendNotification(false)
func TestRecovery_initRecovery_and_sendNotification(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFTReplica(ctrl)

	rbft.off(InRecovery)
	rbft.recoveryMgr.notificationStore[ntfIdx{v: uint64(0), nodeID: uint64(1)}] = &pb.Notification{}
	rbft.initRecovery()

	// Open InRecovery mode
	assert.Equal(t, true, rbft.in(InRecovery))

	// sendNotification false, to increase view, so that view++
	assert.Equal(t, uint64(1), rbft.view)

	// clear messages in low view
	assert.Nil(t, rbft.recoveryMgr.notificationStore[ntfIdx{v: uint64(0), nodeID: uint64(1)}])

	// call recv notification
	assert.Equal(t, uint64(2), rbft.recoveryMgr.notificationStore[ntfIdx{v: uint64(1), nodeID: uint64(2)}].ReplicaId)
}

func TestRecovery_restartRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	assert.Equal(t, uint64(0), rbft.view)
	assert.Nil(t, rbft.restartRecovery())
	assert.Equal(t, uint64(0), rbft.view)
}

func TestRecovery_recvNotification(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFTReplica(ctrl)
	rbft.on(InRecovery)

	// Set a Notification Msg
	vbNode2 := &pb.VcBasis{
		ReplicaId: uint64(2),
		View:      1,
		H:         rbft.h,
	}
	vbNode2.Cset, vbNode2.Pset, vbNode2.Qset = rbft.gatherPQC()
	vbNode3 := &pb.VcBasis{
		ReplicaId: uint64(3),
		View:      1,
		H:         rbft.h,
	}
	vbNode3.Cset, vbNode3.Pset, vbNode3.Qset = rbft.gatherPQC()
	vbNode4 := &pb.VcBasis{
		ReplicaId: uint64(4),
		View:      1,
		H:         rbft.h,
	}
	vbNode4.Cset, vbNode4.Pset, vbNode4.Qset = rbft.gatherPQC()

	notificationNode2 := &pb.Notification{
		Basis:     vbNode2,
		ReplicaId: uint64(2),
	}
	notificationNode3 := &pb.Notification{
		Basis:     vbNode3,
		ReplicaId: uint64(3),
	}
	notificationNode4 := &pb.Notification{
		Basis:     vbNode4,
		ReplicaId: uint64(4),
	}

	// recv quorum notifications
	// return LocalEvent to RecoveryService NotificationQuorumEvent
	payload, _ := proto.Marshal(notificationNode2)
	event := &pb.ConsensusMessage{
		Type:    pb.Type_NOTIFICATION,
		Payload: payload,
	}
	rbft.processEvent(event)
	rbft.recvNotification(notificationNode3)

	// off recovery set in normal, will not reach Recovery Service Event
	rbft.off(InRecovery)
	rbft.on(Normal)
	retNil := rbft.recvNotification(notificationNode4)
	assert.Nil(t, retNil)

	// NewNode will not recv
	rbft.on(InRecovery)
	rbft.off(Normal)
	rbft.on(isNewNode)
	retNil = rbft.recvNotification(notificationNode4)
	assert.Nil(t, retNil)
	rbft.off(isNewNode)

	// in recovery, get in reEvent
	retEvent := rbft.recvNotification(notificationNode4)
	expEvent := &LocalEvent{
		Service:   RecoveryService,
		EventType: NotificationQuorumEvent,
	}
	assert.Equal(t, expEvent, retEvent)
}

// recvNotificationResponse and resetStateForRecovery Normal Case
func TestRecovery_recvNotificationResponse_and_resetStateForRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	routerInfo := rbft.peerPool.serializeRouterInfo()
	nrNode2 := &pb.NotificationResponse{
		Basis:      rbft.getVcBasis(),
		N:          uint64(rbft.N),
		RouterInfo: routerInfo,
		ReplicaId:  uint64(2),
	}
	nrNode2.Basis.ReplicaId = uint64(2)
	nrNode2.Basis.View = uint64(1)
	nrNode3 := &pb.NotificationResponse{
		Basis:      rbft.getVcBasis(),
		N:          uint64(rbft.N),
		RouterInfo: routerInfo,
		ReplicaId:  uint64(3),
	}
	nrNode3.Basis.ReplicaId = uint64(3)
	nrNode3.Basis.View = uint64(1)
	nrNode4 := &pb.NotificationResponse{
		Basis:      rbft.getVcBasis(),
		N:          uint64(rbft.N),
		RouterInfo: routerInfo,
		ReplicaId:  uint64(4),
	}
	nrNode4.Basis.ReplicaId = uint64(4)
	nrNode4.Basis.View = uint64(1)

	payload, _ := proto.Marshal(nrNode2)
	event := &pb.ConsensusMessage{
		Type:    pb.Type_NOTIFICATION_RESPONSE,
		Payload: payload,
	}
	rbft.on(InRecovery)
	rbft.processEvent(event)

	rbft.recvNotificationResponse(nrNode3)
	// if not in recovery, will not recv
	rbft.off(InRecovery)
	retNil := rbft.recvNotificationResponse(nrNode4)
	assert.Nil(t, retNil)

	// as replica node in recovery, call resetStateForRecovery
	// finish the process and send recovery done event
	rbft.on(InRecovery)
	rbft.storeMgr.certStore[msgID{v: 0, n: 20, d: "msg"}] = &msgCert{sentPrepare: true}
	retEvent := rbft.recvNotificationResponse(nrNode4)
	expEvent := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoveryDoneEvent,
	}
	// cert in storeMgr with lower view is cleared
	assert.Nil(t, rbft.storeMgr.certStore[msgID{v: 0, n: 20, d: "msg"}])
	assert.Equal(t, expEvent, retEvent)
}

func TestRecovery_resetStateForRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	// behind by checkpoint, move watermark and state transfer to the target
	C := &pb.Vc_C{
		SequenceNumber: 20,
		Digest:         "cp",
	}
	routerInfo := rbft.peerPool.serializeRouterInfo()
	nrNode2 := &pb.NotificationResponse{
		Basis:      rbft.getVcBasis(),
		N:          uint64(rbft.N),
		RouterInfo: routerInfo,
		ReplicaId:  uint64(2),
	}
	nrNode2.Basis.ReplicaId = uint64(2)
	nrNode2.Basis.View = uint64(1)
	nrNode2.Basis.Cset = []*pb.Vc_C{C}

	nrNode3 := &pb.NotificationResponse{
		Basis:      rbft.getVcBasis(),
		N:          uint64(rbft.N),
		RouterInfo: routerInfo,
		ReplicaId:  uint64(3),
	}
	nrNode3.Basis.ReplicaId = uint64(3)
	nrNode3.Basis.View = uint64(1)
	nrNode3.Basis.Cset = []*pb.Vc_C{C}

	nrNode4 := &pb.NotificationResponse{
		Basis:      rbft.getVcBasis(),
		N:          uint64(rbft.N),
		RouterInfo: routerInfo,
		ReplicaId:  uint64(4),
	}
	nrNode4.Basis.ReplicaId = uint64(4)
	nrNode4.Basis.View = uint64(1)
	nrNode4.Basis.Cset = []*pb.Vc_C{C}

	rbft.on(InRecovery)
	rbft.recvNotificationResponse(nrNode2)
	rbft.recvNotificationResponse(nrNode3)
	rbft.recvNotificationResponse(nrNode4)

	// clear outstanding batch
	rbft.storeMgr.outstandingReqBatches["delete"] = &pb.RequestBatch{BatchHash: "delete"}
	rbft.resetStateForRecovery()
	assert.Nil(t, rbft.storeMgr.outstandingReqBatches["delete"])
}

func TestRecovery_fetchRecoveryPQC(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFTReplica(ctrl)

	rbft.h = 10
	ret := rbft.fetchRecoveryPQC()
	// return nil means running normally
	assert.Nil(t, ret)
}

func TestRecovery_returnRecoveryPQC(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	fetchTmp := &pb.RecoveryFetchPQC{
		ReplicaId: uint64(1),
		H:         20,
	}
	// Too large H, could not fetch PQC
	fetchTmp.H = 100
	ret := rbft.returnRecoveryPQC(fetchTmp)
	assert.Nil(t, ret)

	// After fetching the PQC, return nil
	fetchTmp.H = 20

	rbft.h = 10
	rbft.setView(0)
	// If return is nil, it runs normally
	hashBatch := &pb.HashBatch{
		RequestHashList:            []string{"Hash11", "Hash12"},
		DeDuplicateRequestHashList: []string{"Hash11", "Hash12"},
		Timestamp:                  1,
	}
	preprep := &pb.PrePrepare{
		ReplicaId:      1,
		View:           0,
		SequenceNumber: 40,
		BatchDigest:    calculateMD5Hash([]string{"Hash11", "Hash12"}, 1),
		HashBatch:      hashBatch,
	}
	prepNode2 := &pb.Prepare{
		View:           preprep.View,
		SequenceNumber: preprep.SequenceNumber,
		BatchDigest:    preprep.BatchDigest,
		ReplicaId:      rbft.peerPool.localID,
	}
	prepNode4 := &pb.Prepare{
		View:           preprep.View,
		SequenceNumber: preprep.SequenceNumber,
		BatchDigest:    preprep.BatchDigest,
		ReplicaId:      uint64(4),
	}
	prepNode3 := &pb.Prepare{
		View:           preprep.View,
		SequenceNumber: preprep.SequenceNumber,
		BatchDigest:    preprep.BatchDigest,
		ReplicaId:      uint64(3),
	}
	commitNode2 := &pb.Commit{
		ReplicaId:      2,
		View:           preprep.View,
		SequenceNumber: preprep.SequenceNumber,
		BatchDigest:    preprep.BatchDigest,
	}
	commitNode1 := &pb.Commit{
		ReplicaId:      1,
		View:           preprep.View,
		SequenceNumber: preprep.SequenceNumber,
		BatchDigest:    preprep.BatchDigest,
	}
	commitNode3 := &pb.Commit{
		ReplicaId:      3,
		View:           preprep.View,
		SequenceNumber: preprep.SequenceNumber,
		BatchDigest:    preprep.BatchDigest,
	}
	msgIDTmp := msgID{
		v: 0,
		n: 40,
		d: calculateMD5Hash([]string{"Hash11", "Hash12"}, 1),
	}
	rbft.storeMgr.certStore[msgIDTmp] = &msgCert{
		prePrepare:  preprep,
		sentPrepare: true,
		prepare:     map[pb.Prepare]bool{*prepNode2: true, *prepNode3: true, *prepNode4: true},
		sentCommit:  true,
		commit:      map[pb.Commit]bool{*commitNode3: true, *commitNode2: true, *commitNode1: true},
		sentExecute: true,
	}
	payload, _ := proto.Marshal(fetchTmp)
	event := &pb.ConsensusMessage{
		Type:    pb.Type_RECOVERY_FETCH_QPC,
		Payload: payload,
	}
	ret = rbft.processEvent(event)
	assert.Nil(t, ret)
}

func TestRecovery_recvRecoveryReturnPQC(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFTReplica(ctrl)

	hashBatch := &pb.HashBatch{
		RequestHashList:            []string{"Hash11", "Hash12"},
		DeDuplicateRequestHashList: []string{"Hash11", "Hash12"},
		Timestamp:                  1,
	}
	preprep := &pb.PrePrepare{
		ReplicaId:      1,
		View:           0,
		SequenceNumber: 10,
		BatchDigest:    calculateMD5Hash([]string{"Hash11", "Hash12"}, 1),
		HashBatch:      hashBatch,
	}
	prepNode2 := &pb.Prepare{
		View:           preprep.View,
		SequenceNumber: preprep.SequenceNumber,
		BatchDigest:    preprep.BatchDigest,
		ReplicaId:      rbft.peerPool.localID,
	}
	prepNode4 := &pb.Prepare{
		View:           preprep.View,
		SequenceNumber: preprep.SequenceNumber,
		BatchDigest:    preprep.BatchDigest,
		ReplicaId:      uint64(4),
	}
	prepNode3 := &pb.Prepare{
		View:           preprep.View,
		SequenceNumber: preprep.SequenceNumber,
		BatchDigest:    preprep.BatchDigest,
		ReplicaId:      uint64(3),
	}
	commitNode2 := &pb.Commit{
		ReplicaId:      2,
		View:           preprep.View,
		SequenceNumber: preprep.SequenceNumber,
		BatchDigest:    preprep.BatchDigest,
	}
	commitNode4 := &pb.Commit{
		ReplicaId:      4,
		View:           preprep.View,
		SequenceNumber: preprep.SequenceNumber,
		BatchDigest:    preprep.BatchDigest,
	}
	commitNode3 := &pb.Commit{
		ReplicaId:      3,
		View:           preprep.View,
		SequenceNumber: preprep.SequenceNumber,
		BatchDigest:    preprep.BatchDigest,
	}

	PQCInfo := &pb.RecoveryReturnPQC{
		ReplicaId: 1,
		PrepreSet: []*pb.PrePrepare{preprep},
		PreSet:    []*pb.Prepare{prepNode4, prepNode2, prepNode3},
		CmtSet:    []*pb.Commit{commitNode4, commitNode2, commitNode3},
	}

	// Recovery consensus process
	payload, _ := proto.Marshal(PQCInfo)
	event := &pb.ConsensusMessage{
		Type:    pb.Type_RECOVERY_RETURN_QPC,
		Payload: payload,
	}
	rbft.processEvent(event)
	certTmp := rbft.storeMgr.getCert(preprep.View, preprep.SequenceNumber, preprep.BatchDigest)
	assert.Equal(t, preprep, certTmp.prePrepare)
	assert.Equal(t, true, certTmp.prepare[*prepNode4])
	assert.Equal(t, true, certTmp.prepare[*prepNode2])
	assert.Equal(t, true, certTmp.prepare[*prepNode3])
	assert.Equal(t, true, certTmp.sentPrepare)
	assert.Equal(t, true, certTmp.commit[*commitNode4])
	assert.Equal(t, true, certTmp.commit[*commitNode2])
	assert.Equal(t, true, certTmp.commit[*commitNode3])
	assert.Equal(t, true, certTmp.sentCommit)
}

func TestRecovery_trySyncState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	rbft.off(NeedSyncState)

	// abnormal replica could not be in SyncState
	rbft.off(Normal)
	rbft.trySyncState()
	assert.Equal(t, false, rbft.in(NeedSyncState))

	// normal node
	rbft.on(Normal)
	rbft.trySyncState()
	assert.Equal(t, true, rbft.in(NeedSyncState))

}

func TestRecovery_initSyncState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	// in InSyncState, return nil
	rbft.on(InSyncState)
	ret := rbft.initSyncState()
	assert.Nil(t, ret)

	// call recv sync state rsp
	state := rbft.node.getCurrentState()
	routerInfo := rbft.peerPool.serializeRouterInfo()
	syncStateRsp := &pb.SyncStateResponse{
		ReplicaId:    rbft.peerPool.localID,
		View:         rbft.view,
		N:            uint64(rbft.N),
		RouterInfo:   routerInfo,
		CurrentState: state,
	}
	rbft.off(InSyncState)
	rbft.initSyncState()
	assert.Equal(t, syncStateRsp, rbft.recoveryMgr.syncRspStore[syncStateRsp.ReplicaId])
}

func TestRecovery_recvSyncState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	sync := &pb.SyncState{ReplicaId: uint64(1)}

	// abnormal return nil
	rbft.off(Normal)
	ret := rbft.recvSyncState(sync)
	assert.Nil(t, ret)

	// normal mode
	// run without errors, return nil
	rbft.on(Normal)
	payload, _ := proto.Marshal(sync)
	event := &pb.ConsensusMessage{
		Type:    pb.Type_SYNC_STATE,
		Payload: payload,
	}
	ret = rbft.processEvent(event)
	assert.Nil(t, ret)
}

func TestRecovery_recvSyncStateRsp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	rspNode1 := &pb.SyncStateResponse{
		ReplicaId:    1,
		View:         0,
		N:            4,
		RouterInfo:   rbft.peerPool.serializeRouterInfo(),
		CurrentState: &pb.ServiceState{Applied: 20, Digest: "wrong"},
	}
	rspNode2 := &pb.SyncStateResponse{
		ReplicaId:    2,
		View:         0,
		N:            4,
		RouterInfo:   rbft.peerPool.serializeRouterInfo(),
		CurrentState: &pb.ServiceState{Applied: 20, Digest: "msg"},
	}
	rspNode3 := &pb.SyncStateResponse{
		ReplicaId:    3,
		View:         0,
		N:            4,
		RouterInfo:   rbft.peerPool.serializeRouterInfo(),
		CurrentState: &pb.ServiceState{Applied: 20, Digest: "msg"},
	}
	rspNode4 := &pb.SyncStateResponse{
		ReplicaId:    4,
		View:         0,
		N:            4,
		RouterInfo:   rbft.peerPool.serializeRouterInfo(),
		CurrentState: &pb.ServiceState{Applied: 20, Digest: "msg"},
	}
	// will return before store the msg
	rbft.off(InSyncState)
	rbft.recvSyncStateRsp(rspNode1)
	assert.Nil(t, rbft.recoveryMgr.syncRspStore[rspNode1.ReplicaId])

	// already stored
	// change it to a new one
	rbft.on(InSyncState)
	rbft.off(StateTransferring)
	rbft.recvSyncStateRsp(rspNode1)
	assert.Equal(t, rspNode1, rbft.recoveryMgr.syncRspStore[rspNode1.ReplicaId])

	// Finally, open StateTransferring to change node1 digest same as others
	rbft.recvSyncStateRsp(rspNode2)
	rbft.recvSyncStateRsp(rspNode3)
	rbft.recvSyncStateRsp(rspNode4)
	assert.Equal(t, true, rbft.in(StateTransferring))

	// If selfHeight != quorumHeight
	// Primary send view change
	rbft.on(InSyncState)
	rspNode1.CurrentState.Applied = 10
	rbft.recvSyncStateRsp(rspNode1)
	assert.Equal(t, uint64(1), rbft.view)
}

func TestRecovery_restartSyncState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	rbft.initSyncState()
	assert.Equal(t, 1, len(rbft.recoveryMgr.syncRspStore))

	rbft.restartSyncState()
	assert.Equal(t, 0, len(rbft.recoveryMgr.syncRspStore))
}
