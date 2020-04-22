package rbft

import (
	"testing"
	"time"

	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// helper functions for sort
// =============================================================================

func TestHelper_Len(t *testing.T) {
	a := sortableUint64List{
		1,
		2,
		3,
		4,
		5,
	}
	assert.Equal(t, 5, a.Len())
}

func TestHelper_Less(t *testing.T) {
	a := sortableUint64List{
		1,
		2,
		3,
		4,
		5,
	}
	assert.Equal(t, false, a.Less(2, 1))
}

func TestHelper_Swap(t *testing.T) {
	a := sortableUint64List{
		1,
		2,
		3,
		4,
		5,
	}
	a.Swap(1, 2)
	assert.Equal(t, true, a.Less(2, 1))
	a.Swap(1, 2)
}

// =============================================================================
// helper functions for RBFT
// =============================================================================

func TestHelper_RBFT(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	// primaryID
	assert.Equal(t, uint64(3), rbft.primaryID(uint64(14)))
	// Test for Warningf
	rbft.N = 5
	assert.Equal(t, uint64(0), rbft.primaryID(uint64(4)))
	rbft.N = 4

	// isPrimary
	// Default view.id=1 rbft.view=0
	assert.Equal(t, false, rbft.isPrimary(uint64(5)))
	assert.Equal(t, false, rbft.isPrimary(uint64(3)))
	assert.Equal(t, true, rbft.isPrimary(uint64(1)))

	// inW
	// Default rbft.h=0
	assert.Equal(t, true, rbft.inW(uint64(1)))

	// inV
	assert.Equal(t, true, rbft.inV(uint64(0)))

	// inWV
	assert.Equal(t, false, rbft.inWV(uint64(1), uint64(1)))

	// sendInW
	assert.Equal(t, true, rbft.sendInW(uint64(3)))

	var n int64
	var v uint64

	// getAddNV
	n, v = rbft.getAddNV()
	assert.Equal(t, int64(5), n)
	assert.Equal(t, uint64(5), v)

	rbft.setView(5)
	_, v = rbft.getAddNV()
	assert.Equal(t, uint64(6), v)

	// getDelNV
	rbft.setView(0)
	n, v = rbft.getDelNV(uint64(2))
	assert.Equal(t, int64(3), n)
	assert.Equal(t, uint64(3), v)

	rbft.setView(3)
	_, v = rbft.getDelNV(uint64(2))
	assert.Equal(t, uint64(5), v)

	rbft.setView(uint64(11))
	rbft.N = 5
	n, v = rbft.getDelNV(uint64(0))
	assert.Equal(t, uint64(12), v)
	assert.Equal(t, int64(4), n)

	// cleanOutstandingAndCert
	rbft.cleanOutstandingAndCert()

	// commonCaseQuorum
	rbft.N = 4
	assert.Equal(t, 3, rbft.commonCaseQuorum())

	// allCorrectReplicasQuorum
	assert.Equal(t, 3, rbft.allCorrectReplicasQuorum())

	// oneCorrectQuorum
	assert.Equal(t, 2, rbft.oneCorrectQuorum())
}

// =============================================================================
// pre-prepare/prepare/commit check helper
// =============================================================================

func TestHelper_Check(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	var IDTmp = msgID{
		v: 1,
		n: 20,
		d: "msg",
	}

	var prePrepareTmp = &pb.PrePrepare{
		ReplicaId:      2,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
		HashBatch:      nil,
	}

	var prepare1Tmp = pb.Prepare{
		ReplicaId:      1,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	var prepare2Tmp = pb.Prepare{
		ReplicaId:      3,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	var prepare3Tmp = pb.Prepare{
		ReplicaId:      4,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	var prepareMapTmp = map[pb.Prepare]bool{
		prepare1Tmp: true,
		prepare2Tmp: true,
		prepare3Tmp: true,
	}

	var commit1Tmp = pb.Commit{
		ReplicaId:      1,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	var commit2Tmp = pb.Commit{
		ReplicaId:      3,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	var commit3Tmp = pb.Commit{
		ReplicaId:      4,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	var commitMapTmp = map[pb.Commit]bool{
		commit1Tmp: true,
		commit2Tmp: true,
		commit3Tmp: true,
	}

	var certTmp = &msgCert{
		prePrepare:  prePrepareTmp,
		sentPrepare: false,
		prepare:     prepareMapTmp,
		sentCommit:  false,
		commit:      commitMapTmp,
		sentExecute: false,
	}
	rbft.storeMgr.certStore[IDTmp] = certTmp

	assert.Equal(t, true, rbft.prePrepared("msg", uint64(1), uint64(20)))
	assert.Equal(t, false, rbft.prePrepared("error msg", uint64(1), uint64(20)))

	assert.Equal(t, false, rbft.prepared("no prePrepared", 1, 20))
	assert.Equal(t, true, rbft.prepared("msg", 1, 20))

	assert.Equal(t, false, rbft.committed("no prepared", 1, 20))
	assert.Equal(t, true, rbft.committed("msg", 1, 20))
}

// =============================================================================
// helper functions for check the validity of consensus messages
// =============================================================================

func TestHelper_isPrePrepareLegal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	preprep := &pb.PrePrepare{
		ReplicaId:      2,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
		HashBatch:      nil,
	}

	rbft.atomicOn(InRecovery)
	assert.Equal(t, false, rbft.isPrePrepareLegal(preprep))
	rbft.atomicOff(InRecovery)

	rbft.atomicOn(InViewChange)
	assert.Equal(t, false, rbft.isPrePrepareLegal(preprep))
	rbft.atomicOff(InViewChange)

	rbft.atomicOn(InConfChange)
	assert.Equal(t, false, rbft.isPrePrepareLegal(preprep))
	rbft.atomicOff(InConfChange)

	assert.Equal(t, false, rbft.isPrePrepareLegal(preprep))

	preprep.ReplicaId = 1
	assert.Equal(t, false, rbft.isPrePrepareLegal(preprep))

	rbft.peerPool.ID = 1
	assert.Equal(t, false, rbft.isPrePrepareLegal(preprep))

	rbft.peerPool.ID = 3
	assert.Equal(t, false, rbft.isPrePrepareLegal(preprep))

	rbft.on(SkipInProgress)
	assert.Equal(t, false, rbft.isPrePrepareLegal(preprep))

	preprep.View = 0
	rbft.setView(0)
	rbft.exec.setLastExec(uint64(30))
	assert.Equal(t, false, rbft.isPrePrepareLegal(preprep))

	rbft.exec.setLastExec(uint64(10))
	assert.Equal(t, true, rbft.isPrePrepareLegal(preprep))
}

func TestHelper_isPrepareLegal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	prep := &pb.Prepare{
		ReplicaId:      2,
		View:           0,
		SequenceNumber: 2,
		BatchDigest:    "test",
	}
	assert.Equal(t, true, rbft.isPrepareLegal(prep))
	prep.View = 1
	assert.Equal(t, false, rbft.isPrepareLegal(prep))
	rbft.h = 2
	assert.Equal(t, false, rbft.isPrepareLegal(prep))
	prep.ReplicaId = 1
	assert.Equal(t, false, rbft.isPrepareLegal(prep))
}

func TestHelper_isCommitLegal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	commit := &pb.Commit{
		ReplicaId:      1,
		View:           0,
		SequenceNumber: 2,
		BatchDigest:    "test",
	}

	assert.Equal(t, true, rbft.isCommitLegal(commit))
	commit.View = 1
	assert.Equal(t, false, rbft.isCommitLegal(commit))
	rbft.h = 2
	assert.Equal(t, false, rbft.isCommitLegal(commit))
}

func TestHelper_compareCheckpointWithWeakSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	var (
		flag bool
		val  int
	)

	chkpt := &pb.Checkpoint{
		ReplicaId:      1,
		SequenceNumber: 50,
		Digest:         "msg",
	}
	chkptR1 := &pb.Checkpoint{
		ReplicaId:      3,
		SequenceNumber: 50,
		Digest:         "msgR1",
	}
	chkptR2 := &pb.Checkpoint{
		ReplicaId:      4,
		SequenceNumber: 50,
		Digest:         "msgR2",
	}

	// if !rbft.inW(chkpt.SequenceNumber)
	// if chkpt.SequenceNumber != rbft.h && !rbft.in(SkipInProgress)
	rbft.h = 70
	rbft.on(SkipInProgress)
	flag, val = rbft.compareCheckpointWithWeakSet(chkpt)
	assert.Equal(t, false, flag)
	assert.Equal(t, 0, val)

	rbft.off(SkipInProgress)
	flag, val = rbft.compareCheckpointWithWeakSet(chkpt)
	assert.Equal(t, false, flag)
	assert.Equal(t, 0, val)

	// if len(diffValues) > rbft.f+1
	rbft.h = 20
	rbft.storeMgr.checkpointStore[*chkpt] = true
	rbft.storeMgr.checkpointStore[*chkptR1] = true
	rbft.storeMgr.checkpointStore[*chkptR2] = true
	flag, val = rbft.compareCheckpointWithWeakSet(chkpt)
	assert.Equal(t, false, flag)
	assert.Equal(t, 0, val)

	// if len(correctValues) == 0
	delete(rbft.storeMgr.checkpointStore, *chkpt)
	delete(rbft.storeMgr.checkpointStore, *chkptR1)
	delete(rbft.storeMgr.checkpointStore, *chkptR2)
	rbft.storeMgr.checkpointStore[*chkpt] = true
	flag, val = rbft.compareCheckpointWithWeakSet(chkpt)
	assert.Equal(t, false, flag)
	assert.Equal(t, 0, val)

	// if len(correctValues) > 1
	rbft.f = 1
	rbft.atomicOff(Pending)
	rbft.on(Normal)
	chkpt.Digest = "msg"
	chkptR1.Digest = "msg"
	chkptR2.Digest = "msg"
	rbft.storeMgr.checkpointStore[*chkpt] = true
	rbft.storeMgr.checkpointStore[*chkptR1] = true
	rbft.storeMgr.checkpointStore[*chkptR2] = true
	chkpt.Digest = "msgR1"
	chkptR1.Digest = "msgR1"
	chkptR2.Digest = "msgR1"
	rbft.storeMgr.checkpointStore[*chkpt] = true
	rbft.storeMgr.checkpointStore[*chkptR1] = true
	rbft.storeMgr.checkpointStore[*chkptR2] = true
	delete(rbft.storeMgr.checkpointStore, *chkptR2)
	flag, val = rbft.compareCheckpointWithWeakSet(chkpt)
	assert.Equal(t, false, flag)
	assert.Equal(t, 0, val)
	assert.Equal(t, false, rbft.atomicIn(Pending))
	assert.Equal(t, true, rbft.in(Normal))
}

func TestRBFT_startTimerIfOutstandingRequests(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	rbft.off(SkipInProgress)

	requestBatchTmp := &pb.RequestBatch{
		RequestHashList: []string{"request hash list", "request hash list"},
		RequestList:     mockRequestLists,
		Timestamp:       time.Now().UnixNano(),
		SeqNo:           2,
		LocalList:       []bool{true, true},
		BatchHash:       "hash",
	}
	rbft.storeMgr.outstandingReqBatches["msg"] = requestBatchTmp

	rbft.startTimerIfOutstandingRequests()
}
