package rbft

import (
	"testing"
	"time"

	"github.com/hyperchain/go-hpc-common/types/protos"
	pb "github.com/hyperchain/go-hpc-rbft/v2/rbftpb"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestBatchMgr_startBatchTimer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()
	assert.False(t, rbfts[0].timerMgr.getTimer(batchTimer))
	assert.False(t, rbfts[0].batchMgr.isBatchTimerActive())
	rbfts[0].startBatchTimer()
	assert.True(t, rbfts[0].timerMgr.getTimer(batchTimer))
	assert.True(t, rbfts[0].batchMgr.isBatchTimerActive())
}

// Test for only this function
func TestBatchMgr_maybeSendPrePrepare(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()

	// Set a Batch
	batchTmp41 := &pb.RequestBatch{
		RequestHashList: []string{"tx-hash-41"},
		RequestList:     []*protos.Transaction{newTx()},
		Timestamp:       time.Now().UnixNano(),
		LocalList:       []bool{true},
		BatchHash:       "test digest 41",
	}
	batchTmp42 := &pb.RequestBatch{
		RequestHashList: []string{"tx-hash-42"},
		RequestList:     []*protos.Transaction{newTx()},
		Timestamp:       time.Now().UnixNano(),
		LocalList:       []bool{true},
		BatchHash:       "test digest 42",
	}

	// Be out of range, need usage of catch
	// And, it is the first one to be in catch
	rbfts[0].batchMgr.setSeqNo(40)
	rbfts[0].maybeSendPrePrepare(batchTmp41, false)
	rbfts[0].maybeSendPrePrepare(batchTmp42, false)
	assert.Equal(t, batchTmp41, rbfts[0].batchMgr.cacheBatch[0])
	assert.Equal(t, batchTmp42, rbfts[0].batchMgr.cacheBatch[1])

	// Be in the range
	// to find in catch
	// Now, rbft.batchMgr.cacheBatch[0] has already store a value
	// Set rbft.h 10, 10~50
	rbfts[0].moveWatermarks(10)
	rbfts[0].maybeSendPrePrepare(nil, true)
	//assume that
	assert.Equal(t, batchTmp41, rbfts[0].storeMgr.batchStore[batchTmp41.BatchHash])
	assert.Equal(t, batchTmp42, rbfts[0].storeMgr.batchStore[batchTmp42.BatchHash])
}

func TestBatchMgr_findNextPrepareBatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()

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

	// Define an empty cert first
	certTmp := &msgCert{
		prePrepare:  nil,
		sentPrepare: false,
		prepare:     nil, //map[pb.Prepare]bool{prePareTmp: true},
		sentCommit:  false,
		commit:      nil, //map[pb.Commit]bool{commitTmp: true},
		sentExecute: false,
	}
	rbfts[0].storeMgr.certStore[msgIDTmp] = certTmp

	t.Run("When view is incorrect, exit with nil, without any change", func(t *testing.T) {
		rbfts[0].setView(1)
		assert.Nil(t, rbfts[0].findNextPrepareBatch(0, 20, "msg"))
		rbfts[0].setView(0)
	})

	t.Run("When prePrepare is nil, exit with nil, without any change", func(t *testing.T) {
		assert.Nil(t, rbfts[0].findNextPrepareBatch(0, 20, "msg"))
		certTmp.prePrepare = prePrepareTmp
	})

	t.Run("If replica is in stateUpdate, exit with nil, without any change", func(t *testing.T) {
		rbfts[0].on(SkipInProgress)
		assert.Nil(t, rbfts[0].findNextPrepareBatch(0, 20, "msg"))
		rbfts[0].off(SkipInProgress)
	})

	t.Run("Normal case, there are no batches in storeMgr", func(t *testing.T) {

		// store the HashBatch which was input by certTmp
		certTmp.prepare = map[pb.Prepare]bool{prePareTmp: true}
		certTmp.commit = map[pb.Commit]bool{commitTmp: true}

		assert.Nil(t, rbfts[0].findNextPrepareBatch(0, 20, "msg"))

		// verified key: Timestamp
		assert.Equal(t, int64(10086), rbfts[0].storeMgr.outstandingReqBatches["msg"].Timestamp)
		assert.Equal(t, int64(10086), rbfts[0].storeMgr.batchStore["msg"].Timestamp)
		assert.Equal(t, true, rbfts[0].storeMgr.certStore[msgIDTmp].sentPrepare)

		// To resend commit
		rbfts[0].storeMgr.certStore[msgIDTmp].sentPrepare = false
		assert.Nil(t, rbfts[0].findNextPrepareBatch(0, 20, "msg"))
		assert.Equal(t, true, rbfts[0].storeMgr.certStore[msgIDTmp].sentPrepare)
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
		rbfts[0].setView(0)
		rbfts[0].storeMgr.certStore[msgIDTmpNil] = certTmpNil
		rbfts[0].storeMgr.certStore[msgIDTmpNil].sentPrepare = false
		_ = rbfts[0].findNextPrepareBatch(0, 30, "")
		assert.Equal(t, true, rbfts[0].storeMgr.certStore[msgIDTmpNil].sentPrepare)
		assert.Equal(t, true, rbfts[0].storeMgr.certStore[msgIDTmpNil].sentPrepare)
	})

}
