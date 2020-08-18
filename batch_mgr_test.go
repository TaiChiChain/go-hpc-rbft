package rbft

import (
	"testing"
	"time"

	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestBatchMgr_newBatchManager(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, conf := newTestRBFT(ctrl)

	bm := newBatchManager(conf)

	structName, nilElems, err := checkNilElems(bm)
	if err == nil {
		assert.Equal(t, "batchManager", structName)
		assert.Nil(t, nilElems)
	}

	assert.Equal(t, conf.RequestPool, bm.requestPool)

	bm.seqNo = 1
	assert.Equal(t, uint64(1), bm.getSeqNo())

	bm.setSeqNo(uint64(10))
	assert.Equal(t, uint64(10), bm.getSeqNo())

	bm.batchTimerActive = true
	assert.Equal(t, true, bm.isBatchTimerActive())

	rbft.batchMgr.batchTimerActive = false
	rbft.startBatchTimer()
	assert.Equal(t, true, rbft.batchMgr.batchTimerActive)

	rbft.stopBatchTimer()
	assert.Equal(t, false, rbft.batchMgr.batchTimerActive)

	rbft.restartBatchTimer()
	assert.Equal(t, true, rbft.batchMgr.batchTimerActive)
}

// Test for only this function
func TestBatchMgr_maybeSendPrePrepare(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	// Set a Batch
	batchTmp := &pb.RequestBatch{
		RequestHashList: nil,
		RequestList:     nil,
		Timestamp:       time.Now().UnixNano(),
		SeqNo:           40,
		LocalList:       nil,
		BatchHash:       "test digest",
	}

	// Be out of range, need usage of catch
	// And, it is the first one to be in catch
	rbft.batchMgr.setSeqNo(uint64(40))
	rbft.maybeSendPrePrepare(batchTmp, false)
	assert.Equal(t, batchTmp, rbft.batchMgr.cacheBatch[0])

	// Be in the range
	// to find in catch
	// Now, rbft.batchMgr.cacheBatch[0] has already store a value
	// Set rbft.h 10, 10~50
	rbft.h = 10
	rbft.maybeSendPrePrepare(nil, true)
	//assume that
	assert.Equal(t, batchTmp, rbft.storeMgr.batchStore[batchTmp.BatchHash])
}

func TestBatchMgr_findNextCommitBatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	// Struct of certTmp which stored in rbft.storeMgr.certStore
	prePrepareTmp := &pb.PrePrepare{
		ReplicaId:      2,
		View:           0,
		SequenceNumber: 20,
		BatchDigest:    "msg",
		HashBatch:      &pb.HashBatch{Timestamp: 10086},
	}
	prePareTmp := pb.Prepare{
		ReplicaId:      3,
		View:           0,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	commitTmp := pb.Commit{
		ReplicaId:      4,
		View:           0,
		SequenceNumber: 20,
		BatchDigest:    "msg",
	}
	msgIDTmp := msgID{
		v: 0,
		n: 20,
		d: "msg",
	}

	// Define a empty cert first
	certTmp := &msgCert{
		prePrepare:  nil,
		sentPrepare: false,
		prepare:     nil, //map[pb.Prepare]bool{prePareTmp: true},
		sentCommit:  false,
		commit:      nil, //map[pb.Commit]bool{commitTmp: true},
		sentExecute: false,
	}
	rbft.storeMgr.certStore[msgIDTmp] = certTmp

	// When view is incorrect, exit with nil, without any change
	rbft.setView(1)
	assert.Nil(t, rbft.findNextCommitBatch("msg", 0, 20))
	rbft.setView(0)

	// When prePrepare is nil, exit with nil, without any change
	assert.Nil(t, rbft.findNextCommitBatch("msg", 0, 20))
	certTmp.prePrepare = prePrepareTmp

	// If replica is in stateUpdate, exit with nil, without any change
	rbft.on(SkipInProgress)
	assert.Nil(t, rbft.findNextCommitBatch("msg", 0, 20))

	// To The End
	// store the HashBatch which was input by certTmp
	// Normal case, there are no batches in storeMgr
	// verified key: Timestamp
	certTmp.prepare = map[pb.Prepare]bool{prePareTmp: true}
	certTmp.commit = map[pb.Commit]bool{commitTmp: true}
	rbft.off(SkipInProgress)
	assert.Nil(t, rbft.findNextCommitBatch("msg", 0, 20))
	assert.Equal(t, int64(10086), rbft.storeMgr.outstandingReqBatches["msg"].Timestamp)
	assert.Equal(t, int64(10086), rbft.storeMgr.batchStore["msg"].Timestamp)
	assert.Equal(t, true, rbft.storeMgr.certStore[msgIDTmp].sentCommit)

	// To resend commit
	rbft.storeMgr.certStore[msgIDTmp].sentCommit = false
	assert.Nil(t, rbft.findNextCommitBatch("msg", 0, 20))
	assert.Equal(t, true, rbft.storeMgr.certStore[msgIDTmp].sentCommit)

	// Digest == ""
	prePrepareTmpNil := &pb.PrePrepare{
		ReplicaId:      2,
		View:           0,
		SequenceNumber: 30,
		BatchDigest:    "",
		HashBatch:      &pb.HashBatch{Timestamp: 10086},
	}
	prePareTmpNil := pb.Prepare{
		ReplicaId:      3,
		View:           0,
		SequenceNumber: 30,
		BatchDigest:    "",
	}
	commitTmpNil := pb.Commit{
		ReplicaId:      4,
		View:           0,
		SequenceNumber: 30,
		BatchDigest:    "",
	}
	msgIDTmpNil := msgID{
		v: 0,
		n: 30,
		d: "",
	}
	certTmpNil := &msgCert{
		prePrepare:  prePrepareTmpNil,
		sentPrepare: false,
		prepare:     map[pb.Prepare]bool{prePareTmpNil: true},
		sentCommit:  false,
		commit:      map[pb.Commit]bool{commitTmpNil: true},
		sentExecute: false,
	}
	rbft.setView(0)
	rbft.storeMgr.certStore[msgIDTmpNil] = certTmpNil
	rbft.storeMgr.certStore[msgIDTmpNil].sentCommit = false
	_ = rbft.findNextCommitBatch("", 0, 30)
	assert.Equal(t, true, rbft.storeMgr.certStore[msgIDTmpNil].sentCommit)
}
