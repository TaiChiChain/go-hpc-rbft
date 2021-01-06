package rbft

import (
	"testing"
	"time"

	"github.com/ultramesh/flato-common/types/protos"
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
	assert.False(t, a.Less(2, 1))
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
	assert.True(t, a.Less(2, 1))
	a.Swap(1, 2)
}

// =============================================================================
// helper functions for RBFT
// =============================================================================

func TestHelper_RBFT(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()

	rbfts[0].N = 4

	// isPrimary
	// Default view.id=1 rbft.view=0
	assert.Equal(t, false, rbfts[0].isPrimary(uint64(5)))
	assert.Equal(t, false, rbfts[0].isPrimary(uint64(3)))
	assert.Equal(t, true, rbfts[0].isPrimary(uint64(1)))

	// inW
	// Default rbft.h=0
	assert.Equal(t, true, rbfts[0].inW(uint64(1)))

	// inV
	assert.Equal(t, true, rbfts[0].inV(uint64(0)))

	// inWV
	assert.Equal(t, false, rbfts[0].inWV(uint64(1), uint64(1)))

	// sendInW
	assert.Equal(t, true, rbfts[0].sendInW(uint64(3)))

	// cleanOutstandingAndCert
	rbfts[0].cleanOutstandingAndCert()

	// commonCaseQuorum
	rbfts[0].N = 4
	assert.Equal(t, 3, rbfts[0].commonCaseQuorum())

	// allCorrectReplicasQuorum
	assert.Equal(t, 3, rbfts[0].allCorrectReplicasQuorum())

	// oneCorrectQuorum
	assert.Equal(t, 2, rbfts[0].oneCorrectQuorum())
}

// =============================================================================
// pre-prepare/prepare/commit check helper
// =============================================================================

func TestHelper_Check(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()

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
	rbfts[0].storeMgr.certStore[IDTmp] = certTmp

	assert.True(t, rbfts[0].prePrepared("msg", uint64(1), uint64(20)))
	assert.False(t, rbfts[0].prePrepared("error msg", uint64(1), uint64(20)))

	assert.False(t, rbfts[0].prepared("no prePrepared", 1, 20))
	assert.True(t, rbfts[0].prepared("msg", 1, 20))

	assert.False(t, rbfts[0].committed("no prepared", 1, 20))
	assert.True(t, rbfts[0].committed("msg", 1, 20))
}

// =============================================================================
// helper functions for check the validity of consensus messages
// =============================================================================

func TestHelper_isPrePrepareLegal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()

	preprep := &pb.PrePrepare{
		ReplicaId:      1,
		View:           0,
		SequenceNumber: 20,
		BatchDigest:    "msg",
		HashBatch:      nil,
	}

	rbfts[0].atomicOn(InRecovery)
	assert.False(t, rbfts[0].isPrePrepareLegal(preprep))
	rbfts[0].atomicOff(InRecovery)

	rbfts[0].atomicOn(InViewChange)
	assert.False(t, rbfts[0].isPrePrepareLegal(preprep))
	rbfts[0].atomicOff(InViewChange)

	rbfts[0].atomicOn(InConfChange)
	assert.False(t, rbfts[0].isPrePrepareLegal(preprep))
	rbfts[0].atomicOff(InConfChange)

	assert.False(t, rbfts[0].isPrePrepareLegal(preprep))

	assert.True(t, rbfts[3].isPrePrepareLegal(preprep))

	rbfts[3].h = 20
	assert.False(t, rbfts[3].isPrePrepareLegal(preprep))

	rbfts[3].h = 100
	assert.False(t, rbfts[3].isPrePrepareLegal(preprep))

	rbfts[3].h = 10
	rbfts[3].exec.setLastExec(uint64(21))
	assert.False(t, rbfts[3].isPrePrepareLegal(preprep))
}

func TestHelper_isPrepareLegal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()

	prep := &pb.Prepare{
		ReplicaId:      2,
		View:           0,
		SequenceNumber: 2,
		BatchDigest:    "test",
	}
	assert.True(t, rbfts[0].isPrepareLegal(prep))
	prep.View = 1
	assert.False(t, rbfts[0].isPrepareLegal(prep))
	rbfts[0].h = 10
	assert.False(t, rbfts[0].isPrepareLegal(prep))
	prep.ReplicaId = 1
	assert.False(t, rbfts[0].isPrepareLegal(prep))
}

func TestHelper_isCommitLegal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()

	commit := &pb.Commit{
		ReplicaId:      1,
		View:           0,
		SequenceNumber: 2,
		BatchDigest:    "test",
	}

	assert.True(t, rbfts[0].isCommitLegal(commit))
	commit.View = 1
	assert.False(t, rbfts[0].isCommitLegal(commit))
	rbfts[0].h = 10
	assert.False(t, rbfts[0].isCommitLegal(commit))
}

func TestHelper_compareCheckpointWithWeakSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()

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
	rbfts[0].h = 70
	rbfts[0].on(SkipInProgress)
	flag, val = rbfts[0].compareCheckpointWithWeakSet(chkpt)
	assert.Equal(t, false, flag)
	assert.Equal(t, 0, val)

	rbfts[0].off(SkipInProgress)
	flag, val = rbfts[0].compareCheckpointWithWeakSet(chkpt)
	assert.Equal(t, false, flag)
	assert.Equal(t, 0, val)

	// if len(diffValues) > rbft.f+1
	rbfts[0].h = 20
	rbfts[0].storeMgr.checkpointStore[*chkpt] = true
	rbfts[0].storeMgr.checkpointStore[*chkptR1] = true
	rbfts[0].storeMgr.checkpointStore[*chkptR2] = true
	flag, val = rbfts[0].compareCheckpointWithWeakSet(chkpt)
	assert.Equal(t, false, flag)
	assert.Equal(t, 0, val)

	// if len(correctValues) == 0
	delete(rbfts[0].storeMgr.checkpointStore, *chkpt)
	delete(rbfts[0].storeMgr.checkpointStore, *chkptR1)
	delete(rbfts[0].storeMgr.checkpointStore, *chkptR2)
	rbfts[0].storeMgr.checkpointStore[*chkpt] = true
	flag, val = rbfts[0].compareCheckpointWithWeakSet(chkpt)
	assert.Equal(t, false, flag)
	assert.Equal(t, 0, val)

	// if len(correctValues) > 1
	rbfts[0].atomicOff(Pending)
	rbfts[0].setNormal()
	chkpt.Digest = "msg"
	chkptR1.Digest = "msg"
	chkptR2.Digest = "msg"
	rbfts[0].storeMgr.checkpointStore[*chkpt] = true
	rbfts[0].storeMgr.checkpointStore[*chkptR1] = true
	rbfts[0].storeMgr.checkpointStore[*chkptR2] = true
	chkpt.Digest = "msgR1"
	chkptR1.Digest = "msgR1"
	chkptR2.Digest = "msgR1"
	rbfts[0].storeMgr.checkpointStore[*chkpt] = true
	rbfts[0].storeMgr.checkpointStore[*chkptR1] = true
	rbfts[0].storeMgr.checkpointStore[*chkptR2] = true
	delete(rbfts[0].storeMgr.checkpointStore, *chkptR2)
	flag, val = rbfts[0].compareCheckpointWithWeakSet(chkpt)
	assert.Equal(t, false, flag)
	assert.Equal(t, 0, val)
	assert.Equal(t, false, rbfts[0].atomicIn(Pending))
	assert.Equal(t, true, rbfts[0].in(Normal))
}

func TestRBFT_startTimerIfOutstandingRequests(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()

	rbfts[0].off(SkipInProgress)

	requestBatchTmp := &pb.RequestBatch{
		RequestHashList: []string{"request hash list", "request hash list"},
		RequestList:     []*protos.Transaction{newTx()},
		Timestamp:       time.Now().UnixNano(),
		SeqNo:           2,
		LocalList:       []bool{true, true},
		BatchHash:       "hash",
	}
	rbfts[0].storeMgr.outstandingReqBatches["msg"] = requestBatchTmp

	assert.False(t, rbfts[0].timerMgr.getTimer(newViewTimer))
	rbfts[0].startTimerIfOutstandingRequests()
	assert.True(t, rbfts[0].timerMgr.getTimer(newViewTimer))
}

func TestHelper_stopNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()

	close(rbfts[0].delFlag)
	rbfts[0].stopNamespace()
}
