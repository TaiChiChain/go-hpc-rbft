package mempool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/axiomesh/axiom-bft/common/consensus"
)

func TestNewMempool(t *testing.T) {
	ast := assert.New(t)
	conf := NewMockMempoolConfig()
	pool := NewMempool[consensus.FltTransaction, *consensus.FltTransaction](conf)
	ast.Equal(uint64(0), pool.GetPendingNonceByAccount("account1"))
}

func TestAddNewRequests(t *testing.T) {
	ast := assert.New(t)
	conf := NewMockMempoolConfig()
	conf.ToleranceTime = 500 * time.Millisecond
	pool := newMempoolImpl[consensus.FltTransaction, *consensus.FltTransaction](conf)
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(1))
	tx3 := ConstructTxByAccountAndNonce("account2", uint64(0))
	tx4 := ConstructTxByAccountAndNonce("account2", uint64(1))
	tx5 := ConstructTxByAccountAndNonce("account2", uint64(2))
	txList := make([][]byte, 0)
	tx1Bytes, err := tx1.RbftMarshal()
	ast.Nil(err)
	tx2Bytes, err := tx2.RbftMarshal()
	ast.Nil(err)
	tx3Bytes, err := tx3.RbftMarshal()
	ast.Nil(err)
	tx4Bytes, err := tx4.RbftMarshal()
	ast.Nil(err)
	tx5Bytes, err := tx5.RbftMarshal()
	ast.Nil(err)
	txList = append(txList, tx1Bytes, tx2Bytes, tx3Bytes, tx4Bytes, tx5Bytes)
	batch, completionMissingBatchHashes := pool.AddNewRequests(txList, true, true, false)
	ast.Equal(1, len(batch))
	ast.Equal(0, len(completionMissingBatchHashes))
	ast.Equal(4, len(batch[0].TxList))
	actTx1, err := consensus.DecodeTx[consensus.FltTransaction, *consensus.FltTransaction](batch[0].TxList[0])
	ast.Nil(err)
	actTx2, err := consensus.DecodeTx[consensus.FltTransaction, *consensus.FltTransaction](batch[0].TxList[1])
	ast.Nil(err)
	actTx3, err := consensus.DecodeTx[consensus.FltTransaction, *consensus.FltTransaction](batch[0].TxList[2])
	ast.Nil(err)
	actTx4, err := consensus.DecodeTx[consensus.FltTransaction, *consensus.FltTransaction](batch[0].TxList[3])
	ast.Nil(err)
	ast.Equal(tx1.RbftGetNonce(), actTx1.RbftGetNonce())
	ast.Equal(tx2.RbftGetNonce(), actTx2.RbftGetNonce())
	ast.Equal(tx3.RbftGetNonce(), actTx3.RbftGetNonce())
	ast.Equal(tx4.RbftGetNonce(), actTx4.RbftGetNonce())
	pendingNonce := pool.GetPendingNonceByAccount(tx1.RbftGetFrom())
	ast.Equal(uint64(2), pendingNonce)
	pendingNonce = pool.GetPendingNonceByAccount(tx3.RbftGetFrom())
	ast.Equal(uint64(3), pendingNonce)

	// test replace tx
	oldPoolTx := pool.txStore.getPoolTxByTxnPointer(tx1.RbftGetFrom(), uint64(0))
	oldTimeStamp := oldPoolTx.arrivedTime
	txList = make([][]byte, 0)
	txList = append(txList, tx1Bytes)
	time.Sleep(50 * time.Millisecond)
	batch, _ = pool.AddNewRequests(txList, true, true, true)
	ast.Nil(batch)
	// replace same tx, needn't update
	poolTx := pool.txStore.getPoolTxByTxnPointer(tx1.RbftGetFrom(), uint64(0))
	ast.Equal(poolTx.getHash(), tx1.RbftGetTxHash())
	ast.Equal(5, pool.txStore.localTTLIndex.size())
	ast.True(poolTx.arrivedTime == oldTimeStamp)

	// replace different tx with same nonce
	time.Sleep(2 * time.Second)
	tx11 := ConstructTxByAccountAndNonce("account1", uint64(0))
	ast.NotEqual(tx1.RbftGetTxHash(), tx11.RbftGetTxHash())
	txList = make([][]byte, 0)
	tx11Bytes, err := tx11.RbftMarshal()
	ast.Nil(err)
	txList = append(txList, tx11Bytes)
	batch, _ = pool.AddNewRequests(txList, true, false, true)
	ast.Nil(batch)
	ast.Nil(pool.txStore.txHashMap[tx1.RbftGetTxHash()])
	poolTx = pool.txStore.getPoolTxByTxnPointer(tx1.RbftGetFrom(), uint64(0))
	ast.Equal(poolTx.getHash(), tx11.RbftGetTxHash())
	ast.Equal(5, len(pool.txStore.txHashMap))
	ast.Equal(0, pool.txStore.parkingLotIndex.size())
	ast.Equal(5, pool.txStore.priorityIndex.size())
	// tx11 is not local
	ast.Equal(4, pool.txStore.localTTLIndex.size(), "tx1 will be remove")
}

func constructAllTxs[T any, Constraint consensus.TXConstraint[T]](txs []*T) map[string]*txSortedMap[T, Constraint] {
	txMap := make(map[string]*txSortedMap[T, Constraint])
	for _, tx := range txs {
		txMap[Constraint(tx).RbftGetFrom()] = newTxSortedMap[T, Constraint]()
		memTx := &mempoolTransaction[T, Constraint]{
			lifeTime: Constraint(tx).RbftGetTimeStamp(),
			rawTx:    tx,
		}
		txMap[Constraint(tx).RbftGetFrom()].items[Constraint(tx).RbftGetNonce()] = memTx
	}
	return txMap
}

func TestFilterOutOfDateRequests(t *testing.T) {
	ast := assert.New(t)
	pool := mockMempoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	poolTx2 := ConstructTxByAccountAndNonce("account2", 1)
	time.Sleep(1 * time.Second)
	poolTx1 := ConstructTxByAccountAndNonce("account1", 0)
	time.Sleep(1 * time.Second)
	poolTx3 := ConstructTxByAccountAndNonce("account3", 2)
	pool.txStore.localTTLIndex.insertByOrderedQueueKey(poolTx1.RbftGetTimeStamp(), poolTx1.RbftGetFrom(), poolTx1.RbftGetNonce())
	pool.txStore.localTTLIndex.insertByOrderedQueueKey(poolTx2.RbftGetTimeStamp(), poolTx2.RbftGetFrom(), poolTx2.RbftGetNonce())
	pool.txStore.localTTLIndex.insertByOrderedQueueKey(poolTx3.RbftGetTimeStamp(), poolTx3.RbftGetFrom(), poolTx3.RbftGetNonce())

	txs := []*consensus.FltTransaction{&poolTx1, &poolTx2, &poolTx3}
	pool.txStore.allTxs = constructAllTxs[consensus.FltTransaction, *consensus.FltTransaction](txs)

	pool.toleranceTime = 1 * time.Second
	time.Sleep(2 * time.Second)
	// poolTx1, poolTx2, poolTx3 out of date
	reqs, err := pool.FilterOutOfDateRequests()
	ast.Nil(err)
	ast.Equal(3, len(reqs))
	actNonce, err := consensus.GetNonce[consensus.FltTransaction, *consensus.FltTransaction](reqs[0])
	ast.Nil(err)
	ast.Equal(uint64(1), actNonce, "poolTx2")
	actNonce, err = consensus.GetNonce[consensus.FltTransaction, *consensus.FltTransaction](reqs[1])
	ast.Nil(err)
	ast.Equal(uint64(0), actNonce, "poolTx1")
	actNonce, err = consensus.GetNonce[consensus.FltTransaction, *consensus.FltTransaction](reqs[2])
	ast.Nil(err)
	ast.Equal(uint64(2), actNonce, "poolTx3")
	tx1NewTimestamp := pool.txStore.allTxs[poolTx1.RbftGetFrom()].items[0].lifeTime
	tx2NewTimestamp := pool.txStore.allTxs[poolTx2.RbftGetFrom()].items[1].lifeTime
	tx3NewTimestamp := pool.txStore.allTxs[poolTx3.RbftGetFrom()].items[2].lifeTime
	ast.NotEqual(poolTx1.RbftGetTimeStamp(), tx1NewTimestamp, "has update the timestamp of poolTx1")
	ast.NotEqual(poolTx2.RbftGetTimeStamp(), tx2NewTimestamp, "has update the timestamp of poolTx2")
	ast.NotEqual(poolTx3.RbftGetTimeStamp(), tx3NewTimestamp, "has update the timestamp of poolTx3")
	ast.Equal(tx1NewTimestamp, tx2NewTimestamp)
	ast.Equal(tx2NewTimestamp, tx3NewTimestamp)

	pool.txStore.batchedTxs[txnPointer{account: poolTx2.RbftGetFrom(), nonce: uint64(1)}] = true
	time.Sleep(2 * time.Second)
	reqs, _ = pool.FilterOutOfDateRequests()
	ast.Equal(2, len(reqs))
	actNonce1, err := consensus.GetNonce[consensus.FltTransaction, *consensus.FltTransaction](reqs[0])
	ast.Nil(err)
	actNonce3, err := consensus.GetNonce[consensus.FltTransaction, *consensus.FltTransaction](reqs[1])
	ast.Nil(err)
	ast.Equal(uint64(0), actNonce1, "poolTx1")
	ast.Equal(uint64(2), actNonce3, "poolTx3")
	ast.NotEqual(poolTx1.RbftGetTimeStamp(), tx1NewTimestamp, "has update the timestamp of poolTx1")
	ast.NotEqual(poolTx3.RbftGetTimeStamp(), tx3NewTimestamp, "has update the timestamp of poolTx3")
}

func TestReset(t *testing.T) {
	ast := assert.New(t)
	pool := mockMempoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	pool.txStore.nonceCache.setCommitNonce("account1", uint64(1))
	pool.txStore.nonceCache.setCommitNonce("account2", uint64(1))
	pool.txStore.nonceCache.setPendingNonce("account1", uint64(2))
	pool.txStore.nonceCache.setPendingNonce("account2", uint64(4))
	pool.Reset(nil)
	ast.Equal(uint64(1), pool.txStore.nonceCache.getPendingNonce("account1"), "pending nonce should be reset to commit nonce")
	ast.Equal(uint64(1), pool.txStore.nonceCache.getCommitNonce("account1"))
	ast.Equal(uint64(1), pool.txStore.nonceCache.getPendingNonce("account2"), "pending nonce should be reset to commit nonce")
	ast.Equal(uint64(1), pool.txStore.nonceCache.getCommitNonce("account2"))
	ast.Equal(0, len(pool.txStore.batchedTxs))
	ast.Equal(0, len(pool.txStore.txHashMap))
	ast.Equal(0, len(pool.txStore.allTxs))
	ast.Equal(0, pool.txStore.localTTLIndex.size())
	ast.Equal(0, pool.txStore.parkingLotIndex.size())
	ast.Equal(0, pool.txStore.priorityIndex.size())
	ast.Equal(0, len(pool.txStore.batchesCache))
	ast.Equal(0, len(pool.txStore.missingBatch))
	ast.Equal(uint64(0), pool.txStore.priorityNonBatchSize)
}

func TestGenerateRequestBatch(t *testing.T) {
	ast := assert.New(t)
	pool := mockMempoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	pool.batchSize = 4
	tx3 := ConstructTxByAccountAndNonce("account2", uint64(0))
	time.Sleep(10 * time.Millisecond)
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(1))
	time.Sleep(10 * time.Millisecond)
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx4 := ConstructTxByAccountAndNonce("account2", uint64(1))
	tx1Bytes, _ := tx1.RbftMarshal()
	tx2Bytes, _ := tx2.RbftMarshal()
	tx3Bytes, _ := tx3.RbftMarshal()
	tx4Bytes, _ := tx4.RbftMarshal()

	txList := make([][]byte, 0)
	txList = append(txList, tx1Bytes, tx2Bytes, tx4Bytes)
	batches, _ := pool.AddNewRequests(txList, true, true, false)
	ast.Equal(0, len(batches))
	ast.Equal(0, len(pool.txStore.batchesCache))
	ast.Equal(0, len(pool.txStore.batchedTxs))
	ast.Equal(3, pool.txStore.localTTLIndex.size())
	ast.Equal(3, len(pool.txStore.txHashMap))
	ast.Equal(2, len(pool.txStore.allTxs))
	ast.Equal(2, pool.txStore.priorityIndex.size())
	ast.Equal(1, pool.txStore.parkingLotIndex.size())
	ast.Equal(uint64(2), pool.txStore.priorityNonBatchSize, "account1's txs with nonce 0,1")
	ast.Equal(uint64(2), pool.txStore.nonceCache.getPendingNonce(tx1.RbftGetFrom()), "account1 already have nonce 0,1")
	ast.Equal(uint64(0), pool.txStore.nonceCache.getCommitNonce(tx1.RbftGetFrom()))
	ast.Equal(uint64(0), pool.txStore.nonceCache.getPendingNonce(tx3.RbftGetFrom()), "account2 lack nonce 0, so don't update pending nonce")
	ast.Equal(uint64(0), pool.txStore.nonceCache.getCommitNonce(tx3.RbftGetFrom()))

	txList = make([][]byte, 0)
	txList = append(txList, tx3Bytes)
	batches, _ = pool.AddNewRequests(txList, true, true, false)
	ast.Equal(1, len(batches))
	ast.Equal(4, len(batches[0].TxList))
	// sorted by <timestamp, account, nonce>
	ast.Equal(1, len(pool.txStore.batchesCache))
	ast.Equal(4, len(pool.txStore.batchedTxs))
	ast.Equal(4, pool.txStore.localTTLIndex.size())
	ast.Equal(4, len(pool.txStore.txHashMap))
	ast.Equal(2, len(pool.txStore.allTxs))
	ast.Equal(4, pool.txStore.priorityIndex.size())
	ast.Equal(1, pool.txStore.parkingLotIndex.size())
	ast.Equal(uint64(0), pool.txStore.priorityNonBatchSize)
	ast.Equal(uint64(2), pool.txStore.nonceCache.getPendingNonce(tx1.RbftGetFrom()))
	ast.Equal(uint64(0), pool.txStore.nonceCache.getCommitNonce(tx1.RbftGetFrom()))
	ast.Equal(uint64(2), pool.txStore.nonceCache.getPendingNonce(tx3.RbftGetFrom()))
	ast.Equal(uint64(0), pool.txStore.nonceCache.getCommitNonce(tx3.RbftGetFrom()))

	tx5 := ConstructTxByAccountAndNonce("account2", uint64(2))
	tx5Bytes, _ := tx5.RbftMarshal()
	txList = make([][]byte, 0)
	txList = append(txList, tx5Bytes)
	batch, _ := pool.AddNewRequests(txList, true, true, false)
	ast.Equal(0, len(batch))
	batches = pool.GenerateRequestBatch()
	ast.Equal(1, len(batches))
	ast.Equal(1, len(batches[0].TxList))

	actTxHash, err := consensus.GetTxHash[consensus.FltTransaction, *consensus.FltTransaction](batches[0].TxList[0])
	ast.Nil(err)
	ast.Equal(tx5.RbftGetTxHash(), actTxHash)
	ast.Equal(uint64(0), pool.txStore.priorityNonBatchSize)
}

func TestCheckRequestsExist(t *testing.T) {
	ast := assert.New(t)
	pool := mockMempoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(0))
	txHashList := make([]string, 0)
	txHashList = append(txHashList, tx1.RbftGetTxHash(), tx2.RbftGetTxHash())
	pool.CheckRequestsExist(txHashList)
	tx1Ptr := &txnPointer{account: tx1.RbftGetFrom(), nonce: tx1.RbftGetNonce()}
	ast.Equal(false, pool.txStore.batchedTxs[*tx1Ptr], "not in txHashMap")

	pool.txStore.txHashMap[tx1.RbftGetTxHash()] = tx1Ptr
	tx2Ptr := &txnPointer{account: tx2.RbftGetFrom(), nonce: tx2.RbftGetNonce()}
	pool.txStore.txHashMap[tx2.RbftGetTxHash()] = tx2Ptr
	pool.CheckRequestsExist(txHashList)
	ast.Equal(true, pool.txStore.batchedTxs[*tx1Ptr])
	ast.Equal(true, pool.txStore.batchedTxs[*tx2Ptr])
}

func TestRestorePool(t *testing.T) {
	ast := assert.New(t)
	pool := mockMempoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(1))
	tx3 := ConstructTxByAccountAndNonce("account2", uint64(0))
	tx4 := ConstructTxByAccountAndNonce("account2", uint64(1))
	tx5 := ConstructTxByAccountAndNonce("account2", uint64(2))
	tx1Bytes, _ := tx1.RbftMarshal()
	tx2Bytes, _ := tx2.RbftMarshal()
	tx3Bytes, _ := tx3.RbftMarshal()
	tx4Bytes, _ := tx4.RbftMarshal()
	tx5Bytes, _ := tx5.RbftMarshal()
	txList := make([][]byte, 0)
	txList = append(txList, tx1Bytes, tx2Bytes, tx3Bytes, tx4Bytes, tx5Bytes)
	ast.Equal(false, pool.isTimed)
	batch, _ := pool.AddNewRequests(txList, true, true, false)
	ast.Equal(1, len(batch))
	ast.Equal(4, len(pool.txStore.batchedTxs))
	ast.Equal(5, len(pool.txStore.txHashMap))
	ast.Equal(1, len(pool.txStore.batchesCache))
	ast.Equal(5, pool.txStore.priorityIndex.size())
	ast.Equal(0, pool.txStore.parkingLotIndex.size())
	ast.Equal(uint64(2), pool.GetPendingNonceByAccount(tx1.RbftGetFrom()))
	ast.Equal(uint64(3), pool.GetPendingNonceByAccount(tx3.RbftGetFrom()))

	pool.RestorePool()
	ast.Equal(0, len(pool.txStore.batchedTxs))
	ast.Equal(5, len(pool.txStore.txHashMap))
	ast.Equal(0, len(pool.txStore.batchesCache))
	ast.Equal(5, pool.txStore.priorityIndex.size())
	ast.Equal(0, pool.txStore.parkingLotIndex.size())
	ast.Equal(uint64(0), pool.GetPendingNonceByAccount(tx1.RbftGetFrom()), "nonce had already rollback to 0")
	ast.Equal(uint64(0), pool.GetPendingNonceByAccount(tx3.RbftGetFrom()), "nonce had already rollback to 0")
}

func TestReConstructBatchByOrder(t *testing.T) {
	ast := assert.New(t)
	pool := mockMempoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(1))
	tx3 := ConstructTxByAccountAndNonce("account2", uint64(0))
	tx4 := ConstructTxByAccountAndNonce("account2", uint64(1))
	tx5 := ConstructTxByAccountAndNonce("account2", uint64(2))

	tx1Bytes, _ := tx1.RbftMarshal()
	tx2Bytes, _ := tx2.RbftMarshal()
	tx3Bytes, _ := tx3.RbftMarshal()
	tx4Bytes, _ := tx4.RbftMarshal()
	tx5Bytes, _ := tx5.RbftMarshal()

	txList := make([][]byte, 0)
	txList = append(txList, tx1Bytes, tx2Bytes, tx3Bytes, tx4Bytes, tx5Bytes)
	batch, _ := pool.AddNewRequests(txList, true, true, false)
	_, err := pool.ReConstructBatchByOrder(batch[0])
	ast.Contains(err.Error(), "invalid batch", "already exists in bathedCache")

	txList = make([][]byte, 0)
	txList = append(txList, tx1Bytes, tx2Bytes, tx3Bytes)
	newBatch := &RequestHashBatch[consensus.FltTransaction, *consensus.FltTransaction]{
		TxList:    txList,
		LocalList: []bool{true},
		Timestamp: time.Now().UnixNano(),
	}
	_, err = pool.ReConstructBatchByOrder(newBatch)
	ast.Contains(err.Error(), "invalid batch", "TxHashList and TxList have different length")

	txHashList := make([]string, 0)
	txHashList = append(txHashList, tx1.RbftGetTxHash(), tx2.RbftGetTxHash(), tx4.RbftGetTxHash())
	newBatch.TxHashList = txHashList
	batchDigest := getBatchHash(newBatch)
	newBatch.BatchHash = batchDigest
	_, err = pool.ReConstructBatchByOrder(newBatch)
	ast.Contains(err.Error(), "invalid batch", "invalid tx hash")

	txHashList = make([]string, 0)
	txHashList = append(txHashList, tx1.RbftGetTxHash(), tx2.RbftGetTxHash(), tx3.RbftGetTxHash())
	newBatch.TxHashList = txHashList
	_, err = pool.ReConstructBatchByOrder(newBatch)
	ast.Contains(err.Error(), "invalid batch", "invalid batch hash")

	batchDigest = getBatchHash(newBatch)
	newBatch.BatchHash = batchDigest
	deDuplicateTxHashes, err := pool.ReConstructBatchByOrder(newBatch)
	ast.Nil(err)
	ast.Equal(3, len(deDuplicateTxHashes))
	ast.Equal(2, len(pool.txStore.batchesCache))
}

func TestReceiveMissingRequests(t *testing.T) {
	ast := assert.New(t)
	pool := mockMempoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(1))
	tx3 := ConstructTxByAccountAndNonce("account2", uint64(0))
	tx4 := ConstructTxByAccountAndNonce("account2", uint64(1))
	tx1Bytes, _ := tx1.RbftMarshal()
	tx2Bytes, _ := tx2.RbftMarshal()
	tx3Bytes, _ := tx3.RbftMarshal()
	tx4Bytes, _ := tx4.RbftMarshal()

	txList := make([][]byte, 0)
	txHashList := make([]string, 0)
	txList = append(txList, tx1Bytes, tx2Bytes, tx3Bytes, tx4Bytes)
	txHashList = append(txHashList, tx1.RbftGetTxHash(), tx2.RbftGetTxHash(),
		tx3.RbftGetTxHash(), tx4.RbftGetTxHash())
	newBatch := &RequestHashBatch[consensus.FltTransaction, *consensus.FltTransaction]{
		TxList:     txList,
		TxHashList: txHashList,
		LocalList:  []bool{true},
		Timestamp:  time.Now().Unix(),
	}
	batchDigest := getBatchHash(newBatch)
	txs := make(map[uint64][]byte, 0)
	txs[uint64(1)] = tx1Bytes
	txs[uint64(2)] = tx2Bytes
	txs[uint64(3)] = tx3Bytes
	txs[uint64(4)] = tx4Bytes
	_ = pool.ReceiveMissingRequests(batchDigest, txs)
	ast.Equal(0, len(pool.txStore.txHashMap), "missingBatch is nil")

	missingHashList := make(map[uint64]string)
	missingHashList[uint64(1)] = tx1.RbftGetTxHash()
	pool.txStore.missingBatch[batchDigest] = missingHashList
	err := pool.ReceiveMissingRequests(batchDigest, txs)
	ast.NotNil(err, "expect len is not equal to missingBatch len")

	missingHashList[uint64(2)] = tx2.RbftGetTxHash()
	missingHashList[uint64(3)] = tx3.RbftGetTxHash()
	missingHashList[uint64(4)] = "hash4"
	pool.txStore.missingBatch[batchDigest] = missingHashList
	err = pool.ReceiveMissingRequests(batchDigest, txs)
	ast.NotNil(err, "find a hash mismatch tx")
	ast.Equal(1, len(pool.txStore.missingBatch))

	missingHashList[uint64(4)] = tx4.RbftGetTxHash()
	pool.txStore.missingBatch[batchDigest] = missingHashList
	_ = pool.ReceiveMissingRequests(batchDigest, txs)
	ast.Equal(0, len(pool.txStore.missingBatch))

	// before receive missing requests from primary, receive from addNewRequests
	tx5 := ConstructTxByAccountAndNonce("account1", uint64(2))
	tx6 := ConstructTxByAccountAndNonce("account1", uint64(3))
	tx7 := ConstructTxByAccountAndNonce("account2", uint64(2))
	tx8 := ConstructTxByAccountAndNonce("account2", uint64(3))
	tx5Bytes, _ := tx5.RbftMarshal()
	tx6Bytes, _ := tx6.RbftMarshal()
	tx7Bytes, _ := tx7.RbftMarshal()
	tx8Bytes, _ := tx8.RbftMarshal()

	txList = make([][]byte, 0)
	txHashList = make([]string, 0)
	txList = append(txList, tx5Bytes, tx6Bytes, tx7Bytes, tx8Bytes)
	txHashList = append(txHashList, tx5.RbftGetTxHash(), tx6.RbftGetTxHash(),
		tx7.RbftGetTxHash(), tx8.RbftGetTxHash())
	newBatch = &RequestHashBatch[consensus.FltTransaction, *consensus.FltTransaction]{
		TxList:     txList,
		TxHashList: txHashList,
		LocalList:  []bool{true},
		Timestamp:  time.Now().Unix(),
	}
	batchDigest = getBatchHash(newBatch)

	missingHashList = make(map[uint64]string)
	missingHashList[uint64(5)] = tx5.RbftGetTxHash()
	missingHashList[uint64(6)] = tx6.RbftGetTxHash()
	missingHashList[uint64(7)] = tx7.RbftGetTxHash()
	missingHashList[uint64(8)] = tx8.RbftGetTxHash()

	pool.txStore.missingBatch[batchDigest] = missingHashList

	batch, completionMissingBatchHashes := pool.AddNewRequests(txList, false, true, false)
	ast.Equal(0, len(batch))
	ast.Equal(1, len(completionMissingBatchHashes))
	ast.Equal(batchDigest, completionMissingBatchHashes[0])
	ast.Equal(0, len(pool.txStore.missingBatch))
}

func TestSendMissingRequests(t *testing.T) {
	ast := assert.New(t)
	pool := mockMempoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(1))
	tx3 := ConstructTxByAccountAndNonce("account2", uint64(0))
	tx1Bytes, _ := tx1.RbftMarshal()
	tx2Bytes, _ := tx2.RbftMarshal()
	tx3Bytes, _ := tx3.RbftMarshal()

	txHashList := make([]string, 0)
	txList := make([][]byte, 0)
	txList = append(txList, tx1Bytes, tx2Bytes)
	txHashList = append(txHashList, tx1.RbftGetTxHash(), tx2.RbftGetTxHash())
	newBatch := &RequestHashBatch[consensus.FltTransaction, *consensus.FltTransaction]{
		TxList:     txList,
		TxHashList: txHashList,
		LocalList:  []bool{true},
		Timestamp:  time.Now().Unix(),
	}
	batchDigest := getBatchHash[consensus.FltTransaction, *consensus.FltTransaction](newBatch)
	newBatch.BatchHash = batchDigest

	txList = append(txList, tx3Bytes)
	pool.AddNewRequests(txList, false, true, false)
	missingHashList := make(map[uint64]string)
	missingHashList[0] = "hash1"
	_, err := pool.SendMissingRequests(batchDigest, missingHashList)
	ast.NotNil(err, "doesn't exist in txHashMap")

	missingHashList[0] = tx1.RbftGetTxHash()
	missingHashList[1] = tx2.RbftGetTxHash()
	_, err = pool.SendMissingRequests(batchDigest, missingHashList)
	ast.NotNil(err, "doesn't exist in batchedCache")

	pool.txStore.batchesCache[batchDigest] = newBatch
	missingHashList[2] = tx3.RbftGetTxHash()
	_, err = pool.SendMissingRequests(batchDigest, missingHashList)
	ast.NotNil(err, "find invalid transaction")

	delete(missingHashList, uint64(2))
	txs, err1 := pool.SendMissingRequests(batchDigest, missingHashList)
	ast.Nil(err1)
	ast.Equal(2, len(txs))
	actTxHash1, err := consensus.GetTxHash[consensus.FltTransaction, *consensus.FltTransaction](txs[0])
	ast.Nil(err)
	actTxHash2, err := consensus.GetTxHash[consensus.FltTransaction, *consensus.FltTransaction](txs[1])
	ast.Nil(err)
	ast.Equal(tx1.RbftGetTxHash(), actTxHash1)
	ast.Equal(tx2.RbftGetTxHash(), actTxHash2)
}

func TestGetRequestsByHashList(t *testing.T) {
	ast := assert.New(t)
	pool := mockMempoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(1))
	tx3 := ConstructTxByAccountAndNonce("account2", uint64(0))
	tx4 := ConstructTxByAccountAndNonce("account2", uint64(1))
	tx5 := ConstructTxByAccountAndNonce("account2", uint64(3))

	tx1Bytes, _ := tx1.RbftMarshal()
	tx2Bytes, _ := tx2.RbftMarshal()
	tx3Bytes, _ := tx3.RbftMarshal()
	tx4Bytes, _ := tx4.RbftMarshal()
	tx5Bytes, _ := tx5.RbftMarshal()

	txList := make([][]byte, 0)
	txList = append(txList, tx1Bytes, tx2Bytes, tx3Bytes, tx4Bytes, tx5Bytes)
	batches, _ := pool.AddNewRequests(txList, true, true, false)
	ast.Equal(1, len(batches))
	ast.Equal(4, len(pool.txStore.batchedTxs))
	ast.Equal(1, len(pool.txStore.batchesCache))
	ast.Equal(5, len(pool.txStore.txHashMap))
	ast.Equal(uint64(0), pool.txStore.priorityNonBatchSize)
	ast.Equal(4, pool.txStore.priorityIndex.size())
	ast.Equal(1, pool.txStore.parkingLotIndex.size(), "tx5")
	ast.Equal(5, pool.txStore.localTTLIndex.size())
	ast.Equal(uint64(2), pool.txStore.nonceCache.getPendingNonce(tx1.RbftGetFrom()))
	ast.Equal(uint64(2), pool.txStore.nonceCache.getPendingNonce(tx3.RbftGetFrom()))

	txHashList := make([]string, 0)
	txHashList = append(txHashList, tx1.RbftGetTxHash(), tx2.RbftGetTxHash(),
		tx3.RbftGetTxHash(), tx4.RbftGetTxHash())
	txs, localList, missingTxsHash, err := pool.GetRequestsByHashList(batches[0].BatchHash, batches[0].Timestamp, txHashList, nil)
	ast.Nil(err)
	ast.Equal(4, len(txs))
	ast.Equal(4, len(localList))
	ast.Equal(0, len(missingTxsHash))

	delete(pool.txStore.batchesCache, batches[0].BatchHash)
	missingBatch := make(map[uint64]string)
	missingBatch[1] = "hash1"
	pool.txStore.missingBatch[batches[0].BatchHash] = missingBatch
	txs, localList, missingTxsHash, err = pool.GetRequestsByHashList(batches[0].BatchHash, batches[0].Timestamp, txHashList, nil)
	ast.Nil(err)
	ast.Equal(0, len(txs))
	ast.Equal(0, len(localList))
	ast.Equal(1, len(missingTxsHash))

	delete(pool.txStore.missingBatch, batches[0].BatchHash)
	txs, localList, missingTxsHash, err = pool.GetRequestsByHashList(batches[0].BatchHash, batches[0].Timestamp, txHashList, nil)
	ast.NotNil(err, "duplicate transaction")
	ast.Equal(0, len(txs))
	ast.Equal(0, len(localList))
	ast.Equal(0, len(missingTxsHash))

	pool.txStore.batchedTxs = make(map[txnPointer]bool)
	poolTx1 := pool.txStore.allTxs[tx1.RbftGetFrom()].items[1]
	delete(pool.txStore.allTxs[tx1.RbftGetFrom()].items, uint64(1))
	txs, localList, missingTxsHash, err = pool.GetRequestsByHashList(batches[0].BatchHash, batches[0].Timestamp, txHashList, nil)
	ast.Nil(err, "missing transaction")
	ast.Equal(0, len(txs))
	ast.Equal(0, len(localList))
	ast.Equal(1, len(missingTxsHash), "missing tx1")
	ast.Equal(1, len(pool.txStore.missingBatch))

	delete(pool.txStore.missingBatch, batches[0].BatchHash)
	pool.txStore.allTxs[tx1.RbftGetFrom()].items[1] = poolTx1
	txs, localList, missingTxsHash, err = pool.GetRequestsByHashList(batches[0].BatchHash, batches[0].Timestamp, txHashList, nil)
	ast.Nil(err)
	ast.Equal(4, len(txs))
	ast.Equal(4, len(localList))
	ast.Equal(0, len(missingTxsHash))
	ast.Equal(4, len(pool.txStore.batchedTxs))
	ast.Equal(1, len(pool.txStore.batchesCache))
	ast.Equal(5, len(pool.txStore.txHashMap))
	ast.Equal(uint64(0), pool.txStore.priorityNonBatchSize)
	ast.Equal(4, pool.txStore.priorityIndex.size())
	ast.Equal(1, pool.txStore.parkingLotIndex.size(), "tx5")
	ast.Equal(5, pool.txStore.localTTLIndex.size())
	ast.Equal(uint64(2), pool.txStore.nonceCache.getPendingNonce(tx1.RbftGetFrom()))
	ast.Equal(uint64(2), pool.txStore.nonceCache.getPendingNonce(tx3.RbftGetFrom()))
}

func TestRemoveBatches(t *testing.T) {
	ast := assert.New(t)
	pool := mockMempoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	pool.poolSize = 4
	tx1 := ConstructTxByAccountAndNonce("account2", uint64(1))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx3 := ConstructTxByAccountAndNonce("account1", uint64(1))
	tx4 := ConstructTxByAccountAndNonce("account2", uint64(0))
	tx5 := ConstructTxByAccountAndNonce("account1", uint64(3))

	tx1Bytes, _ := tx1.RbftMarshal()
	tx2Bytes, _ := tx2.RbftMarshal()
	tx3Bytes, _ := tx3.RbftMarshal()
	tx4Bytes, _ := tx4.RbftMarshal()
	tx5Bytes, _ := tx5.RbftMarshal()

	txList := make([][]byte, 0)
	txList = append(txList, tx1Bytes, tx2Bytes, tx4Bytes, tx3Bytes, tx5Bytes)
	batches, _ := pool.AddNewRequests(txList, true, true, false)
	ast.Equal(1, len(batches))

	pool.RemoveBatches([]string{batches[0].BatchHash})
	ast.Equal(0, len(pool.txStore.batchedTxs))
	ast.Equal(0, len(pool.txStore.batchesCache))
	ast.Equal(1, len(pool.txStore.txHashMap))
	ast.Equal(uint64(0), pool.txStore.priorityNonBatchSize)
	ast.Equal(0, pool.txStore.priorityIndex.size())
	ast.Equal(1, pool.txStore.parkingLotIndex.size(), "tx5")
	ast.Equal(1, pool.txStore.localTTLIndex.size(), "tx5")
	ast.Equal(uint64(2), pool.txStore.nonceCache.getCommitNonce(tx1.RbftGetFrom()), "commit nonce had already update to pending nonce")
	ast.Equal(uint64(2), pool.txStore.nonceCache.getPendingNonce(tx1.RbftGetFrom()))
	ast.Equal(uint64(2), pool.txStore.nonceCache.getCommitNonce(tx2.RbftGetFrom()), "commit nonce had already update to pending nonce")
	ast.Equal(uint64(2), pool.txStore.nonceCache.getPendingNonce(tx2.RbftGetFrom()))
}

func TestRemoveTimeoutRequests(t *testing.T) {
	ast := assert.New(t)
	pool := mockMempoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	pool.toleranceRemoveTime = 1 * time.Second
	tx1 := ConstructTxByAccountAndNonce("Alice", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("Bob", uint64(0))

	tx3 := ConstructTxByAccountAndNonce("Alice", uint64(2))
	tx4 := ConstructTxByAccountAndNonce("Bob", uint64(2))

	tx5 := ConstructTxByAccountAndNonce("Alice", uint64(3))
	tx6 := ConstructTxByAccountAndNonce("Bob", uint64(3))

	tx1Bytes, _ := tx1.RbftMarshal()
	tx2Bytes, _ := tx2.RbftMarshal()
	tx3Bytes, _ := tx3.RbftMarshal()
	tx4Bytes, _ := tx4.RbftMarshal()
	tx5Bytes, _ := tx5.RbftMarshal()
	tx6Bytes, _ := tx6.RbftMarshal()

	txList := make([][]byte, 0)
	txList = append(txList, tx1Bytes, tx2Bytes, tx3Bytes, tx4Bytes, tx5Bytes, tx6Bytes)
	pool.AddNewRequests(txList, true, true, false)
	batches := pool.GenerateRequestBatch()
	ast.Equal(1, len(batches))
	for _, batch := range batches {
		ast.Equal(2, len(batch.TxHashList))
	}
	account1 := tx1.RbftGetFrom()
	account2 := tx1.RbftGetFrom()
	list1, ok := pool.txStore.allTxs[account1]
	ast.Equal(true, ok)
	list2, ok := pool.txStore.allTxs[account2]
	ast.Equal(true, ok)
	ast.Equal(3, len(list1.items))
	ast.Equal(3, len(list2.items))
	ast.Equal(6, pool.txStore.removeTTLIndex.size())
	ast.Equal(4, pool.txStore.parkingLotIndex.size())
	ast.Equal(2, pool.txStore.priorityIndex.size())
	ast.Equal(6, len(pool.txStore.txHashMap))

	time.Sleep(2 * time.Second)
	// after remove timeout request
	reqLen, err := pool.RemoveTimeoutRequests()
	ast.Nil(err)
	list1, ok = pool.txStore.allTxs[account1]
	ast.Equal(true, ok)
	list2, ok = pool.txStore.allTxs[account2]
	ast.Equal(true, ok)
	ast.Equal(1, len(list1.items))
	ast.Equal(1, len(list2.items))
	ast.Equal(4, int(reqLen))
	ast.Equal(2, pool.txStore.removeTTLIndex.size())
	ast.Equal(0, pool.txStore.parkingLotIndex.size())
	ast.Equal(2, pool.txStore.priorityIndex.size())
	ast.Equal(2, len(pool.txStore.txHashMap))
}

func TestRestoreOneBatch(t *testing.T) {
	ast := assert.New(t)
	pool := mockMempoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(1))
	tx3 := ConstructTxByAccountAndNonce("account2", uint64(0))
	tx4 := ConstructTxByAccountAndNonce("account2", uint64(1))
	tx1Bytes, _ := tx1.RbftMarshal()
	tx2Bytes, _ := tx2.RbftMarshal()
	tx3Bytes, _ := tx3.RbftMarshal()
	tx4Bytes, _ := tx4.RbftMarshal()

	txList := make([][]byte, 0)
	txList = append(txList, tx1Bytes, tx2Bytes, tx4Bytes)
	batches, _ := pool.AddNewRequests(txList, true, true, false)
	ast.Equal(0, len(batches))

	txList = make([][]byte, 0)
	txList = append(txList, tx3Bytes)
	batches, _ = pool.AddNewRequests(txList, true, true, false)
	ast.Equal(1, len(batches))
	ast.Equal(4, len(pool.txStore.batchedTxs))
	ast.Equal(1, len(pool.txStore.batchesCache))
	ast.Equal(4, len(pool.txStore.txHashMap))
	ast.Equal(uint64(0), pool.txStore.priorityNonBatchSize)
	ast.Equal(4, pool.txStore.priorityIndex.size())
	ast.Equal(1, pool.txStore.parkingLotIndex.size(), "tx4 in parkingLotIndex")
	ast.Equal(4, pool.txStore.localTTLIndex.size())

	err := pool.RestoreOneBatch("testBatch")
	ast.NotNil(err, "can't find batch from batchesCache")

	batches[0].TxHashList[0] = "hash1"
	pool.txStore.batchesCache[batches[0].BatchHash] = batches[0]
	err = pool.RestoreOneBatch(batches[0].BatchHash)
	ast.NotNil(err, "can't find tx from txHashMap")

	batches[0].TxHashList[0] = tx1.RbftGetTxHash()
	pool.txStore.batchesCache[batches[0].BatchHash] = batches[0]

	account := tx1.RbftGetFrom()
	list, ok := pool.txStore.allTxs[account]
	ast.Equal(true, ok)
	poolTx := list.items[tx1.RbftGetNonce()]
	ast.NotNil(poolTx)
	pool.txStore.priorityIndex.removeByOrderedQueueKey(poolTx.getRawTimestamp(),
		poolTx.getAccount(), poolTx.getNonce())
	err = pool.RestoreOneBatch(batches[0].BatchHash)
	ast.NotNil(err, "can't find tx from priorityIndex")

	pool.txStore.priorityIndex.insertByOrderedQueueKey(poolTx.getRawTimestamp(),
		poolTx.getAccount(), poolTx.getNonce())
	err = pool.RestoreOneBatch(batches[0].BatchHash)
	ast.Nil(err)
	ast.Equal(0, len(pool.txStore.batchedTxs))
	ast.Equal(0, len(pool.txStore.batchesCache))
	ast.Equal(4, len(pool.txStore.txHashMap))
	ast.Equal(uint64(4), pool.txStore.priorityNonBatchSize)
	ast.Equal(4, pool.txStore.priorityIndex.size())
	ast.Equal(1, pool.txStore.parkingLotIndex.size(), "tx4 in parkingLotIndex")
	ast.Equal(4, pool.txStore.localTTLIndex.size())
}

func TestIsPoolFull(t *testing.T) {
	ast := assert.New(t)
	pool := mockMempoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	pool.poolSize = 3
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(1))
	tx3 := ConstructTxByAccountAndNonce("account2", uint64(0))

	tx1Bytes, _ := tx1.RbftMarshal()
	tx2Bytes, _ := tx2.RbftMarshal()
	tx3Bytes, _ := tx3.RbftMarshal()

	txList := make([][]byte, 0)
	txList = append(txList, tx1Bytes, tx2Bytes, tx3Bytes)
	pool.AddNewRequests(txList, false, true, false)
	isFull := pool.IsPoolFull()
	pending := pool.HasPendingRequestInPool()
	ast.Equal(true, isFull)
	ast.Equal(true, pending)
	ast.Equal(3, len(pool.txStore.txHashMap))
	ast.Equal(0, len(pool.txStore.batchedTxs))
	ast.Equal(3, len(pool.txStore.txHashMap))
	ast.Equal(0, len(pool.txStore.batchesCache))
	ast.Equal(uint64(3), pool.txStore.priorityNonBatchSize)
	ast.Equal(3, pool.txStore.priorityIndex.size())
	ast.Equal(0, pool.txStore.parkingLotIndex.size())
	ast.Equal(3, pool.txStore.localTTLIndex.size())

	batches := pool.GenerateRequestBatch()
	isFull = pool.IsPoolFull()
	pending = pool.HasPendingRequestInPool()
	ast.Equal(true, isFull)
	ast.Equal(false, pending)
	ast.Equal(1, len(batches))
	ast.Equal(3, len(pool.txStore.batchedTxs))
	ast.Equal(3, len(pool.txStore.txHashMap))
	ast.Equal(1, len(pool.txStore.batchesCache))
	ast.Equal(uint64(0), pool.txStore.priorityNonBatchSize)
	ast.Equal(3, pool.txStore.priorityIndex.size())
	ast.Equal(0, pool.txStore.parkingLotIndex.size())
	ast.Equal(3, pool.txStore.localTTLIndex.size())

	batchHashList := make([]string, 0)
	batchHashList = append(batchHashList, batches[0].BatchHash)
	pool.RemoveBatches(batchHashList)
	isFull = pool.IsPoolFull()
	pending = pool.HasPendingRequestInPool()
	ast.Equal(false, isFull)
	ast.Equal(false, pending)
	ast.Equal(0, len(pool.txStore.batchedTxs))
	ast.Equal(0, len(pool.txStore.txHashMap))
	ast.Equal(0, len(pool.txStore.batchesCache))
	ast.Equal(uint64(0), pool.txStore.priorityNonBatchSize)
	ast.Equal(0, pool.txStore.priorityIndex.size())
	ast.Equal(0, pool.txStore.parkingLotIndex.size())
	ast.Equal(0, pool.txStore.localTTLIndex.size())
}

func TestGetPendingTxByHash(t *testing.T) {
	ast := assert.New(t)
	pool := mockMempoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx1Bytes, _ := tx1.RbftMarshal()

	txList := make([][]byte, 0)
	txList = append(txList, tx1Bytes)
	pool.AddNewRequests(txList, false, true, false)

	tx := pool.GetPendingTxByHash(tx1.RbftGetTxHash())
	actHash, err := consensus.GetTxHash[consensus.FltTransaction, *consensus.FltTransaction](tx)
	ast.Nil(err)
	ast.Equal(tx1.RbftGetTxHash(), actHash)
}
