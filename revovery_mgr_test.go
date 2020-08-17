package rbft

import (
	"testing"

	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

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
	rbft.atomicOn(InRecovery)

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

	nodeInfo2 := &pb.NodeInfo{
		ReplicaId:   uint64(2),
		ReplicaHash: "node2",
	}
	notificationNode2 := &pb.Notification{
		Basis:    vbNode2,
		NodeInfo: nodeInfo2,
	}
	nodeInfo3 := &pb.NodeInfo{
		ReplicaId:   uint64(3),
		ReplicaHash: "node3",
	}
	notificationNode3 := &pb.Notification{
		Basis:    vbNode3,
		NodeInfo: nodeInfo3,
	}
	nodeInfo4 := &pb.NodeInfo{
		ReplicaId:   uint64(4),
		ReplicaHash: "node4",
	}
	notificationNode4 := &pb.Notification{
		Basis:    vbNode4,
		NodeInfo: nodeInfo4,
	}

	// recv quorum notifications
	// return LocalEvent to RecoveryService NotificationQuorumEvent
	payload, _ := proto.Marshal(notificationNode2)
	event := &pb.ConsensusMessage{
		Type:    pb.Type_NOTIFICATION,
		Epoch:   uint64(0),
		Payload: payload,
	}
	rbft.processEvent(event)
	rbft.recvNotification(notificationNode3)

	// NewNode will not recv
	rbft.atomicOn(InRecovery)
	rbft.off(Normal)
	rbft.on(isNewNode)
	retNil := rbft.recvNotification(notificationNode4)
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

func TestRecovery_resetStateForRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	// behind by checkpoint, move watermark and state transfer to the target
	C := &pb.Vc_C{
		SequenceNumber: 20,
		Digest:         "cp",
	}

	nrNode2 := &pb.NotificationResponse{
		Basis: rbft.getVcBasis(),
		N:     uint64(rbft.N),
		NodeInfo: &pb.NodeInfo{
			ReplicaId:   uint64(2),
			ReplicaHash: "node2",
		},
	}
	nrNode2.Basis.ReplicaId = uint64(2)
	nrNode2.Basis.View = uint64(1)
	nrNode2.Basis.Cset = []*pb.Vc_C{C}

	nrNode3 := &pb.NotificationResponse{
		Basis: rbft.getVcBasis(),
		N:     uint64(rbft.N),
		NodeInfo: &pb.NodeInfo{
			ReplicaId:   uint64(3),
			ReplicaHash: "node3",
		},
	}
	nrNode3.Basis.ReplicaId = uint64(3)
	nrNode3.Basis.View = uint64(1)
	nrNode3.Basis.Cset = []*pb.Vc_C{C}

	nrNode4 := &pb.NotificationResponse{
		Basis: rbft.getVcBasis(),
		N:     uint64(rbft.N),
		NodeInfo: &pb.NodeInfo{
			ReplicaId:   uint64(4),
			ReplicaHash: "node4",
		},
	}
	nrNode4.Basis.ReplicaId = uint64(4)
	nrNode4.Basis.View = uint64(1)
	nrNode4.Basis.Cset = []*pb.Vc_C{C}

	rbft.atomicOn(InRecovery)
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
		ReplicaId:      rbft.peerPool.ID,
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
		ReplicaId:      rbft.peerPool.ID,
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
		Epoch:   uint64(0),
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
