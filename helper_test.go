package rbft

import (
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/hyperchain/go-hpc-rbft/common/consensus"
	"github.com/hyperchain/go-hpc-rbft/types"

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

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()

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

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()

	var IDTmp = msgID{
		v: 1,
		n: 20,
		d: "msg",
	}

	var prePrepareTmp = &consensus.PrePrepare{
		ReplicaId:      2,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
		HashBatch:      nil,
	}

	var prepare1Tmp = consensus.Prepare{
		ReplicaId:      1,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	var prepare2Tmp = consensus.Prepare{
		ReplicaId:      3,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	var prepare3Tmp = consensus.Prepare{
		ReplicaId:      4,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	var prepareMapTmp = map[consensus.Prepare]bool{
		prepare1Tmp: true,
		prepare2Tmp: true,
		prepare3Tmp: true,
	}

	var commit1Tmp = consensus.Commit{
		ReplicaId:      1,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	var commit2Tmp = consensus.Commit{
		ReplicaId:      3,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	var commit3Tmp = consensus.Commit{
		ReplicaId:      4,
		View:           1,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	var commitMapTmp = map[consensus.Commit]bool{
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

	assert.True(t, rbfts[0].prePrepared(uint64(1), uint64(20), "msg"))
	assert.False(t, rbfts[0].prePrepared(uint64(1), uint64(20), "error msg"))

	assert.False(t, rbfts[0].prepared(1, 20, "no prePrepared"))
	assert.True(t, rbfts[0].prepared(1, 20, "msg"))

	assert.False(t, rbfts[0].committed(1, 20, "no prepared"))
	assert.True(t, rbfts[0].committed(1, 20, "msg"))
}

// =============================================================================
// helper functions for check the validity of consensus messages
// =============================================================================

func TestHelper_isPrePrepareLegal(t *testing.T) {

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()

	preprep := &consensus.PrePrepare{
		ReplicaId:      1,
		View:           0,
		SequenceNumber: 20,
		BatchDigest:    "msg",
		HashBatch:      nil,
	}

	rbfts[0].atomicOn(InViewChange)
	assert.False(t, rbfts[0].isPrePrepareLegal(preprep))
	rbfts[0].atomicOff(InViewChange)

	rbfts[0].atomicOn(InConfChange)
	assert.False(t, rbfts[0].isPrePrepareLegal(preprep))
	rbfts[0].atomicOff(InConfChange)

	assert.False(t, rbfts[0].isPrePrepareLegal(preprep))

	assert.True(t, rbfts[3].isPrePrepareLegal(preprep))

	preprep.ReplicaId = 2
	assert.False(t, rbfts[3].isPrePrepareLegal(preprep))
	preprep.ReplicaId = 1

	rbfts[3].h = 20
	assert.False(t, rbfts[3].isPrePrepareLegal(preprep))

	rbfts[3].h = 100
	assert.False(t, rbfts[3].isPrePrepareLegal(preprep))

	rbfts[3].h = 10
	rbfts[3].exec.setLastExec(uint64(21))
	assert.True(t, rbfts[3].isPrePrepareLegal(preprep))
	mc := &msgCert{
		prePrepare: preprep,
	}
	rbfts[3].storeMgr.certStore[msgID{preprep.View, preprep.SequenceNumber, preprep.BatchDigest}] = mc
	assert.False(t, rbfts[3].isPrePrepareLegal(preprep))
}

func TestHelper_isPrepareLegal(t *testing.T) {

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()

	prep := &consensus.Prepare{
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

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()

	commit := &consensus.Commit{
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

func TestRBFT_startTimerIfOutstandingRequests(t *testing.T) {

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()

	rbfts[0].off(SkipInProgress)

	tx := newTx()
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	requestBatchTmp := &consensus.RequestBatch{
		RequestHashList: []string{"request hash list", "request hash list"},
		RequestList:     [][]byte{txBytes},
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

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()

	close(rbfts[0].delFlag)
	rbfts[0].stopNamespace()
}

func TestHelper_compareCheckpointWithWeakSet(t *testing.T) {

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()

	mockCheckpoint2 := &consensus.SignedCheckpoint{
		Author:     "node2",
		Checkpoint: &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 10, Digest: "block-hash-10"}},
		Signature:  nil,
	}
	mockCheckpoint3 := &consensus.SignedCheckpoint{
		Author:     "node3",
		Checkpoint: &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 10, Digest: "block-hash-10"}},
		Signature:  nil,
	}

	// out of watermark
	mockCheckpoint4 := &consensus.SignedCheckpoint{
		Author:     "node4",
		Checkpoint: &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 0, Digest: "block-hash-0"}},
		Signature:  nil,
	}

	legal, matchingCheckpoints := rbfts[0].compareCheckpointWithWeakSet(mockCheckpoint4)
	assert.False(t, legal)
	assert.Nil(t, matchingCheckpoints)

	// in watermark, but don't have enough checkpoint
	mockCheckpoint4 = &consensus.SignedCheckpoint{
		Author:     "node4",
		Checkpoint: &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 10, Digest: "block-hash-10"}},
		Signature:  nil,
	}

	legal, matchingCheckpoints = rbfts[0].compareCheckpointWithWeakSet(mockCheckpoint4)
	assert.True(t, legal)
	assert.Nil(t, matchingCheckpoints)

	// in watermark, but don't have self checkpoint
	rbfts[0].storeMgr.checkpointStore[chkptID{author: "node2", sequence: 10}] = mockCheckpoint2
	rbfts[0].storeMgr.checkpointStore[chkptID{author: "node3", sequence: 10}] = mockCheckpoint3
	mockCheckpoint4 = &consensus.SignedCheckpoint{
		Author:     "node4",
		Checkpoint: &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 10, Digest: "block-hash-10"}},
		Signature:  nil,
	}

	legal, matchingCheckpoints = rbfts[0].compareCheckpointWithWeakSet(mockCheckpoint4)
	assert.True(t, legal)
	assert.NotNil(t, matchingCheckpoints)

	// in watermark, have self valid checkpoint
	selfCheckpoint := &consensus.SignedCheckpoint{
		Author:     "node1",
		Checkpoint: &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 10, Digest: "block-hash-10"}},
		Signature:  nil,
	}
	rbfts[0].storeMgr.localCheckpoints = map[uint64]*consensus.SignedCheckpoint{10: selfCheckpoint}

	legal, matchingCheckpoints = rbfts[0].compareCheckpointWithWeakSet(mockCheckpoint4)
	assert.True(t, legal)
	assert.NotNil(t, matchingCheckpoints)

	// in watermark, have self invalid checkpoint(incorrect block hash)
	selfCheckpoint = &consensus.SignedCheckpoint{
		Author:     "node1",
		Checkpoint: &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 10, Digest: "block-hash-20"}},
		Signature:  nil,
	}
	rbfts[0].storeMgr.localCheckpoints = map[uint64]*consensus.SignedCheckpoint{10: selfCheckpoint}

	legal, matchingCheckpoints = rbfts[0].compareCheckpointWithWeakSet(mockCheckpoint4)
	assert.False(t, legal)
	assert.Nil(t, matchingCheckpoints)

	// in watermark, have more than f+1 different checkpoint hash
	rbfts[0].off(SkipInProgress)
	rbfts[0].off(StateTransferring)
	rbfts[0].setNormal()
	mockCheckpoint2.Checkpoint = &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 10, Digest: "block-hash-0"}}
	mockCheckpoint4 = &consensus.SignedCheckpoint{
		Author:     "node4",
		Checkpoint: &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 10, Digest: "block-hash-20"}},
		Signature:  nil,
	}
	rbfts[0].storeMgr.localCheckpoints = map[uint64]*consensus.SignedCheckpoint{10: selfCheckpoint}

	go func() {
		<-rbfts[0].delFlag
	}()
	legal, matchingCheckpoints = rbfts[0].compareCheckpointWithWeakSet(mockCheckpoint4)
	assert.False(t, legal)
	assert.Nil(t, matchingCheckpoints)

	// in watermark, but fork has happened(2 vs 2)
	rbfts[0].off(Inconsistent)
	rbfts[0].setNormal()
	mockCheckpoint2.Checkpoint = &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 10, Digest: "block-hash-10"}}
	mockCheckpoint4 = &consensus.SignedCheckpoint{
		Author:     "node4",
		Checkpoint: &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 10, Digest: "block-hash-20"}},
		Signature:  nil,
	}
	rbfts[0].storeMgr.checkpointStore[chkptID{author: "node1", sequence: 10}] = selfCheckpoint

	go func() {
		<-rbfts[0].delFlag
	}()
	legal, matchingCheckpoints = rbfts[0].compareCheckpointWithWeakSet(mockCheckpoint4)
	assert.False(t, legal)
	assert.Nil(t, matchingCheckpoints)
}

func TestHelper_CheckIfNeedStateUpdate(t *testing.T) {

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()
	unlockCluster(rbfts)
	rbfts[0].storeMgr.localCheckpoints = make(map[uint64]*consensus.SignedCheckpoint)

	selfCheckpoint10 := &consensus.SignedCheckpoint{
		Author:     "node1",
		Checkpoint: &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 10, Digest: "block-hash-10"}},
		Signature:  nil,
	}
	rbfts[0].storeMgr.localCheckpoints[10] = selfCheckpoint10
	// mock move to stable checkpoint 10.
	rbfts[0].moveWatermarks(10, false)
	// mock execute to 15.
	rbfts[0].exec.lastExec = 15

	mockCheckpointSet10 := make([]*consensus.SignedCheckpoint, 0)
	// mock checkpoint 10 from node2.
	mockCheckpoint10OfNode2 := &consensus.SignedCheckpoint{
		Author:     "node2",
		Checkpoint: &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 10, Digest: "block-hash-10"}},
		Signature:  nil,
	}
	// mock checkpoint 10 from node3.
	mockCheckpoint10OfNode3 := &consensus.SignedCheckpoint{
		Author:     "node3",
		Checkpoint: &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 10, Digest: "block-hash-10"}},
		Signature:  nil,
	}
	// mock checkpoint 10 from node4.
	mockCheckpoint10OfNode4 := &consensus.SignedCheckpoint{
		Author:     "node4",
		Checkpoint: &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 10, Digest: "block-hash-10"}},
		Signature:  nil,
	}
	mockCheckpointSet10 = append(mockCheckpointSet10, mockCheckpoint10OfNode2)
	mockCheckpointSet10 = append(mockCheckpointSet10, mockCheckpoint10OfNode3)
	mockCheckpointSet10 = append(mockCheckpointSet10, mockCheckpoint10OfNode4)
	mockCheckpointState10 := &types.CheckpointState{
		Meta: types.MetaState{
			Height: 10,
			Digest: "block-hash-10",
		},
		IsConfig: false,
	}
	need := rbfts[0].checkIfNeedStateUpdate(mockCheckpointState10, mockCheckpointSet10)
	assert.False(t, need, "no need to sync chain as local height has exceeded checkpoint set")

	// mock execute to 20.
	rbfts[0].exec.lastExec = 20
	selfCheckpoint20 := &consensus.SignedCheckpoint{
		Author: "node1",
		// mock incorrect hash.
		Checkpoint: &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 20, Digest: "block-hash-20XXX"}},
		Signature:  nil,
	}
	rbfts[0].storeMgr.localCheckpoints[20] = selfCheckpoint20
	mockCheckpointSet20 := make([]*consensus.SignedCheckpoint, 0)
	// mock checkpoint 20 from node2.
	mockCheckpoint20OfNode2 := &consensus.SignedCheckpoint{
		Author:     "node2",
		Checkpoint: &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 20, Digest: "block-hash-20"}},
		Signature:  nil,
	}
	// mock checkpoint 20 from node3.
	mockCheckpoint20OfNode3 := &consensus.SignedCheckpoint{
		Author:     "node3",
		Checkpoint: &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 20, Digest: "block-hash-20"}},
		Signature:  nil,
	}
	// mock checkpoint 20 from node4.
	mockCheckpoint20OfNode4 := &consensus.SignedCheckpoint{
		Author:     "node4",
		Checkpoint: &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 20, Digest: "block-hash-20"}},
		Signature:  nil,
	}
	mockCheckpointSet20 = append(mockCheckpointSet20, mockCheckpoint20OfNode2)
	mockCheckpointSet20 = append(mockCheckpointSet20, mockCheckpoint20OfNode3)
	mockCheckpointSet20 = append(mockCheckpointSet20, mockCheckpoint20OfNode4)
	mockCheckpointState20 := &types.CheckpointState{
		Meta: types.MetaState{
			Height: 20,
			Digest: "block-hash-20",
		},
		IsConfig: true,
	}
	need = rbfts[0].checkIfNeedStateUpdate(mockCheckpointState20, mockCheckpointSet20)
	assert.True(t, need, "need to sync chain as local checkpoint not equal to checkpoint set")

	selfCheckpoint20 = &consensus.SignedCheckpoint{
		Author: "node1",
		// mock correct hash.
		Checkpoint: &consensus.Checkpoint{Epoch: 0, ExecuteState: &consensus.Checkpoint_ExecuteState{Height: 20, Digest: "block-hash-20"}},
		Signature:  nil,
	}
	rbfts[0].storeMgr.localCheckpoints[20] = selfCheckpoint20
	need = rbfts[0].checkIfNeedStateUpdate(mockCheckpointState20, mockCheckpointSet20)
	assert.False(t, need, "no need to sync chain as local config checkpoint equal to checkpoint set")

	mockCheckpointState20 = &types.CheckpointState{
		Meta: types.MetaState{
			Height: 20,
			Digest: "block-hash-20",
		},
		IsConfig: false,
	}
	need = rbfts[0].checkIfNeedStateUpdate(mockCheckpointState20, mockCheckpointSet20)
	assert.False(t, need, "no need to sync chain as local checkpoint equal to checkpoint set")
	assert.Equal(t, uint64(20), rbfts[0].h)
}

func unMarshalVcBasis(vc *consensus.ViewChange) *consensus.VcBasis {
	vcBasis := &consensus.VcBasis{}
	err := proto.Unmarshal(vc.Basis, vcBasis)
	if err != nil {
		return nil
	}
	return vcBasis
}

func marshalVcBasis(vcBasis *consensus.VcBasis) []byte {
	vcb, err := proto.Marshal(vcBasis)
	if err != nil {
		return nil
	}
	return vcb
}
