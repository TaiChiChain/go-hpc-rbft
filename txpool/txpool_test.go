package txpool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/axiomesh/axiom-bft/common/consensus"
)

func TestNewTxPool(t *testing.T) {
	ast := assert.New(t)
	conf := NewMockTxPoolConfig()
	pool := NewTxPool[consensus.FltTransaction, *consensus.FltTransaction](conf)
	ast.Equal(uint64(0), pool.GetPendingTxCountByAccount("account1"))
}

func TestAddNewRequests(t *testing.T) {
	ast := assert.New(t)
	conf := NewMockTxPoolConfig()
	conf.ToleranceTime = 500 * time.Millisecond
	pool := newTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction](conf)
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(1))
	tx3 := ConstructTxByAccountAndNonce("account2", uint64(0))
	tx4 := ConstructTxByAccountAndNonce("account2", uint64(1))
	tx5 := ConstructTxByAccountAndNonce("account2", uint64(2))

	batch, completionMissingBatchHashes := pool.AddNewRequests([]*consensus.FltTransaction{tx1, tx2, tx3, tx4, tx5}, true, true, false, true)
	ast.Equal(1, len(batch))
	ast.Equal(0, len(completionMissingBatchHashes))
	ast.Equal(4, len(batch[0].TxList))
	actTx1 := batch[0].TxList[0]
	actTx2 := batch[0].TxList[1]
	actTx3 := batch[0].TxList[2]
	actTx4 := batch[0].TxList[3]
	ast.Equal(tx1.RbftGetNonce(), actTx1.RbftGetNonce())
	ast.Equal(tx2.RbftGetNonce(), actTx2.RbftGetNonce())
	ast.Equal(tx3.RbftGetNonce(), actTx3.RbftGetNonce())
	ast.Equal(tx4.RbftGetNonce(), actTx4.RbftGetNonce())
	pendingNonce := pool.GetPendingTxCountByAccount(tx1.RbftGetFrom())
	ast.Equal(uint64(2), pendingNonce)
	pendingNonce = pool.GetPendingTxCountByAccount(tx3.RbftGetFrom())
	ast.Equal(uint64(3), pendingNonce)

	// test replace tx
	oldPoolTx := pool.txStore.getPoolTxByTxnPointer(tx1.RbftGetFrom(), uint64(0))
	oldTimeStamp := oldPoolTx.arrivedTime

	time.Sleep(50 * time.Millisecond)
	batch, _ = pool.AddNewRequests([]*consensus.FltTransaction{tx1}, true, true, true, true)
	ast.Nil(batch)

	poolTx := pool.txStore.getPoolTxByTxnPointer(tx1.RbftGetFrom(), uint64(0))
	ast.Equal(poolTx.getHash(), tx1.RbftGetTxHash())
	ast.Equal(5, pool.txStore.localTTLIndex.size())
	ast.True(poolTx.arrivedTime != oldTimeStamp, "tx1 has been replaced")
	ast.Equal(0, pool.txStore.parkingLotIndex.size())
	ast.Equal(5, pool.txStore.priorityIndex.size())

	// replace different tx with same nonce, smaller than currentSeqNo
	time.Sleep(2 * time.Millisecond)
	dupTx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	ast.NotEqual(tx1.RbftGetTxHash(), dupTx1.RbftGetTxHash())
	batch, _ = pool.AddNewRequests([]*consensus.FltTransaction{dupTx1}, true, false, true, true)
	ast.Nil(batch)
	ast.Nil(pool.txStore.txHashMap[tx1.RbftGetTxHash()])
	poolTx = pool.txStore.getPoolTxByTxnPointer(tx1.RbftGetFrom(), uint64(0))
	ast.Equal(poolTx.getHash(), dupTx1.RbftGetTxHash())
	ast.Equal(5, len(pool.txStore.txHashMap))
	ast.Equal(0, pool.txStore.parkingLotIndex.size())
	ast.Equal(5, pool.txStore.priorityIndex.size())
	// tx11 is not local
	ast.Equal(4, pool.txStore.localTTLIndex.size(), "tx1 will be remove, tx11 is not local, "+
		"so the size will decrease 1")

	// replace different tx with same nonce, bigger than currentSeqNo
	time.Sleep(2 * time.Millisecond)
	tx15 := ConstructTxByAccountAndNonce("account1", uint64(5))
	batch, _ = pool.AddNewRequests([]*consensus.FltTransaction{tx15}, true, false, true, true)
	ast.Nil(batch)
	ast.True(pool.txStore.txHashMap[tx15.RbftGetTxHash()] != nil)
	ast.Equal(6, len(pool.txStore.txHashMap))
	ast.Equal(1, pool.txStore.parkingLotIndex.size())
	ast.Equal(5, pool.txStore.priorityIndex.size())
	ast.Equal(4, pool.txStore.localTTLIndex.size())
	ast.Equal(6, pool.txStore.removeTTLIndex.size())
	ast.Equal(3, pool.txStore.allTxs[tx15.RbftGetFrom()].index.size())

	sameNonce := uint64(5)
	dupTx15 := ConstructTxByAccountAndNonce("account1", sameNonce)
	batch, _ = pool.AddNewRequests([]*consensus.FltTransaction{dupTx15}, true, false, true, true)
	ast.Nil(batch)
	ast.True(pool.txStore.txHashMap[tx15.RbftGetTxHash()] == nil, "tx15 has been replaced")
	ast.Equal(6, len(pool.txStore.txHashMap))
	ast.Equal(1, pool.txStore.parkingLotIndex.size())
	ast.Equal(4, pool.txStore.localTTLIndex.size())
	ast.Equal(6, pool.txStore.removeTTLIndex.size())
	ast.Equal(3, pool.txStore.allTxs[tx15.RbftGetFrom()].index.size())
	ast.Equal(pool.txStore.allTxs[tx15.RbftGetFrom()].items[sameNonce].getHash(), dupTx15.RbftGetTxHash(),
		"tx15 has been replaced by dupTx15")

	// test generate batch flag
	tx1 = ConstructTxByAccountAndNonce("1account1", uint64(0))
	tx2 = ConstructTxByAccountAndNonce("1account1", uint64(1))
	tx3 = ConstructTxByAccountAndNonce("1account2", uint64(0))
	tx4 = ConstructTxByAccountAndNonce("1account2", uint64(1))
	tx5 = ConstructTxByAccountAndNonce("1account2", uint64(2))
	batch, completionMissingBatchHashes = pool.AddNewRequests([]*consensus.FltTransaction{tx1, tx2, tx3, tx4, tx5}, true, true, false, false)
	ast.Equal(0, len(batch))
	ast.Equal(0, len(completionMissingBatchHashes))

	oldPoolTxLen := len(pool.txStore.txHashMap)
	illegalTx := ConstructTxByAccountAndNonce("mockAccount1", DefaultToleranceNonceGap+1)
	batch, completionMissingBatchHashes = pool.AddNewRequests([]*consensus.FltTransaction{illegalTx}, true, true, false, true)

	_, ok := pool.txStore.allTxs[illegalTx.RbftGetFrom()]
	ast.Equal(oldPoolTxLen, len(pool.txStore.txHashMap), "illegalTx should not be added to txHashMap")
	ast.False(ok, "illegalTx should not be added to allTxs")
}

func constructAllTxs[T any, Constraint consensus.TXConstraint[T]](txs []*T) map[string]*txSortedMap[T, Constraint] {
	txMap := make(map[string]*txSortedMap[T, Constraint])
	for _, tx := range txs {
		txMap[Constraint(tx).RbftGetFrom()] = newTxSortedMap[T, Constraint]()
		memTx := &internalTransaction[T, Constraint]{
			lifeTime: Constraint(tx).RbftGetTimeStamp(),
			rawTx:    tx,
		}
		txMap[Constraint(tx).RbftGetFrom()].items[Constraint(tx).RbftGetNonce()] = memTx
	}
	return txMap
}

func TestFilterOutOfDateRequests(t *testing.T) {
	ast := assert.New(t)
	pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx2 := ConstructTxByAccountAndNonce("account2", 1)
	time.Sleep(1 * time.Millisecond)
	tx1 := ConstructTxByAccountAndNonce("account1", 0)
	time.Sleep(1 * time.Millisecond)
	tx3 := ConstructTxByAccountAndNonce("account3", 2)

	poolTx1 := &internalTransaction[consensus.FltTransaction, *consensus.FltTransaction]{
		rawTx:       tx1,
		local:       true,
		lifeTime:    tx1.RbftGetTimeStamp(),
		arrivedTime: time.Now().UnixNano(),
	}

	poolTx2 := &internalTransaction[consensus.FltTransaction, *consensus.FltTransaction]{
		rawTx:       tx2,
		local:       true,
		lifeTime:    tx2.RbftGetTimeStamp(),
		arrivedTime: time.Now().UnixNano(),
	}

	poolTx3 := &internalTransaction[consensus.FltTransaction, *consensus.FltTransaction]{
		rawTx:       tx3,
		local:       true,
		lifeTime:    tx3.RbftGetTimeStamp(),
		arrivedTime: time.Now().UnixNano(),
	}
	pool.txStore.localTTLIndex.insertByOrderedQueueKey(poolTx1)
	pool.txStore.localTTLIndex.insertByOrderedQueueKey(poolTx2)
	pool.txStore.localTTLIndex.insertByOrderedQueueKey(poolTx3)

	txs := []*consensus.FltTransaction{tx1, tx2, tx3}
	pool.txStore.allTxs = constructAllTxs[consensus.FltTransaction, *consensus.FltTransaction](txs)

	pool.toleranceTime = 1 * time.Second
	time.Sleep(2 * time.Second)
	// poolTx1, poolTx2, poolTx3 out of date
	reqs, err := pool.FilterOutOfDateRequests()
	ast.Nil(err)
	ast.Equal(3, len(reqs))
	actNonce := reqs[0].RbftGetNonce()
	ast.Equal(uint64(1), actNonce, "poolTx2")
	actNonce = reqs[1].RbftGetNonce()
	ast.Equal(uint64(0), actNonce, "poolTx1")
	actNonce = reqs[2].RbftGetNonce()
	ast.Equal(uint64(2), actNonce, "poolTx3")
	tx1NewTimestamp := pool.txStore.allTxs[tx1.RbftGetFrom()].items[0].lifeTime
	tx2NewTimestamp := pool.txStore.allTxs[tx2.RbftGetFrom()].items[1].lifeTime
	tx3NewTimestamp := pool.txStore.allTxs[tx3.RbftGetFrom()].items[2].lifeTime
	ast.NotEqual(tx1.RbftGetTimeStamp(), tx1NewTimestamp, "has update the timestamp of poolTx1")
	ast.NotEqual(tx2.RbftGetTimeStamp(), tx2NewTimestamp, "has update the timestamp of poolTx2")
	ast.NotEqual(tx3.RbftGetTimeStamp(), tx3NewTimestamp, "has update the timestamp of poolTx3")
	ast.Equal(tx1NewTimestamp, tx2NewTimestamp)
	ast.Equal(tx2NewTimestamp, tx3NewTimestamp)

	pool.txStore.batchedTxs[txPointer{account: tx2.RbftGetFrom(), nonce: uint64(1)}] = true
	time.Sleep(2 * time.Second)
	reqs, _ = pool.FilterOutOfDateRequests()
	ast.Equal(2, len(reqs))
	actNonce1 := reqs[0].RbftGetNonce()
	actNonce3 := reqs[1].RbftGetNonce()
	ast.Equal(uint64(0), actNonce1, "poolTx1")
	ast.Equal(uint64(2), actNonce3, "poolTx3")
	ast.NotEqual(tx1.RbftGetTimeStamp(), tx1NewTimestamp, "has update the timestamp of poolTx1")
	ast.NotEqual(tx3.RbftGetTimeStamp(), tx3NewTimestamp, "has update the timestamp of poolTx3")
}

func TestReset(t *testing.T) {
	ast := assert.New(t)
	pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()

	tx1 := ConstructTxByAccountAndNonce("account1", 0)
	tx2 := ConstructTxByAccountAndNonce("account2", 0)
	tx3 := ConstructTxByAccountAndNonce("account3", 0)
	batch1 := &RequestHashBatch[consensus.FltTransaction, *consensus.FltTransaction]{
		BatchHash:  "batch1",
		TxHashList: []string{tx1.RbftGetTxHash()},
		TxList:     []*consensus.FltTransaction{tx1},
		LocalList:  []bool{true},
		Timestamp:  time.Now().Unix(),
	}

	batch2 := &RequestHashBatch[consensus.FltTransaction, *consensus.FltTransaction]{
		BatchHash:  "batch2",
		TxHashList: []string{tx2.RbftGetTxHash()},
		TxList:     []*consensus.FltTransaction{tx2},
		LocalList:  []bool{true},
		Timestamp:  time.Now().Unix(),
	}

	batch3 := &RequestHashBatch[consensus.FltTransaction, *consensus.FltTransaction]{
		BatchHash:  "batch3",
		TxHashList: []string{tx3.RbftGetTxHash()},
		TxList:     []*consensus.FltTransaction{tx3},
		LocalList:  []bool{true},
		Timestamp:  time.Now().Unix(),
	}

	pool.txStore.batchesCache["batch1"] = batch1
	pool.txStore.batchesCache["batch2"] = batch2
	pool.txStore.batchesCache["batch3"] = batch3

	pool.Reset([]string{"batch2"})
	ast.Equal(1, len(pool.txStore.batchedTxs))
	ast.Equal(1, len(pool.txStore.txHashMap))
	ast.Equal(1, len(pool.txStore.allTxs))
	ast.Equal(1, pool.txStore.localTTLIndex.size())
	ast.Equal(0, pool.txStore.parkingLotIndex.size())
	ast.Equal(1, pool.txStore.priorityIndex.size())
	ast.Equal(1, len(pool.txStore.batchesCache))
	ast.Equal(0, len(pool.txStore.missingBatch))
	ast.Equal(uint64(0), pool.txStore.priorityNonBatchSize)
}

func TestGenerateRequestBatch(t *testing.T) {
	ast := assert.New(t)
	pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	pool.batchSize = 4
	tx3 := ConstructTxByAccountAndNonce("account2", uint64(0))
	time.Sleep(10 * time.Millisecond)
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(1))
	time.Sleep(10 * time.Millisecond)
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx4 := ConstructTxByAccountAndNonce("account2", uint64(1))

	batches, _ := pool.AddNewRequests([]*consensus.FltTransaction{tx1, tx2, tx4}, true, true, false, true)
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

	batches, _ = pool.AddNewRequests([]*consensus.FltTransaction{tx3}, true, true, false, true)
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
	batch, _ := pool.AddNewRequests([]*consensus.FltTransaction{tx5}, true, true, false, true)
	ast.Equal(0, len(batch))
	batches = pool.GenerateRequestBatch()
	ast.Equal(1, len(batches))
	ast.Equal(1, len(batches[0].TxList))

	actTxHash := batches[0].TxList[0].RbftGetTxHash()
	ast.Equal(tx5.RbftGetTxHash(), actTxHash)
	ast.Equal(uint64(0), pool.txStore.priorityNonBatchSize)
}

func TestCheckRequestsExist(t *testing.T) {
	ast := assert.New(t)
	pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(0))
	txHashList := make([]string, 0)
	txHashList = append(txHashList, tx1.RbftGetTxHash(), tx2.RbftGetTxHash())
	pool.CheckRequestsExist(txHashList)
	tx1Ptr := &txPointer{account: tx1.RbftGetFrom(), nonce: tx1.RbftGetNonce()}
	ast.Equal(false, pool.txStore.batchedTxs[*tx1Ptr], "not in txHashMap")

	pool.txStore.txHashMap[tx1.RbftGetTxHash()] = tx1Ptr
	tx2Ptr := &txPointer{account: tx2.RbftGetFrom(), nonce: tx2.RbftGetNonce()}
	pool.txStore.txHashMap[tx2.RbftGetTxHash()] = tx2Ptr
	pool.CheckRequestsExist(txHashList)
	ast.Equal(true, pool.txStore.batchedTxs[*tx1Ptr])
	ast.Equal(true, pool.txStore.batchedTxs[*tx2Ptr])
}

func TestRestorePool(t *testing.T) {
	ast := assert.New(t)
	pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(1))
	tx3 := ConstructTxByAccountAndNonce("account2", uint64(0))
	tx4 := ConstructTxByAccountAndNonce("account2", uint64(1))
	tx5 := ConstructTxByAccountAndNonce("account2", uint64(2))
	ast.Equal(false, pool.isTimed)
	batch, _ := pool.AddNewRequests([]*consensus.FltTransaction{tx1, tx2, tx3, tx4, tx5}, true, true, false, true)
	ast.Equal(1, len(batch))
	ast.Equal(4, len(pool.txStore.batchedTxs))
	ast.Equal(5, len(pool.txStore.txHashMap))
	ast.Equal(1, len(pool.txStore.batchesCache))
	ast.Equal(5, pool.txStore.priorityIndex.size())
	ast.Equal(0, pool.txStore.parkingLotIndex.size())
	ast.Equal(uint64(2), pool.GetPendingTxCountByAccount(tx1.RbftGetFrom()))
	ast.Equal(uint64(3), pool.GetPendingTxCountByAccount(tx3.RbftGetFrom()))

	pool.RestorePool()
	ast.Equal(0, len(pool.txStore.batchedTxs))
	ast.Equal(5, len(pool.txStore.txHashMap))
	ast.Equal(0, len(pool.txStore.batchesCache))
	ast.Equal(5, pool.txStore.priorityIndex.size())
	ast.Equal(0, pool.txStore.parkingLotIndex.size())
	ast.Equal(uint64(0), pool.GetPendingTxCountByAccount(tx1.RbftGetFrom()), "nonce had already rollback to 0")
	ast.Equal(uint64(0), pool.GetPendingTxCountByAccount(tx3.RbftGetFrom()), "nonce had already rollback to 0")
}

func TestReConstructBatchByOrder(t *testing.T) {
	ast := assert.New(t)
	pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(1))
	tx3 := ConstructTxByAccountAndNonce("account2", uint64(0))
	tx4 := ConstructTxByAccountAndNonce("account2", uint64(1))
	tx5 := ConstructTxByAccountAndNonce("account2", uint64(2))

	batch, _ := pool.AddNewRequests([]*consensus.FltTransaction{tx1, tx2, tx3, tx4, tx5}, true, true, false, true)
	_, err := pool.ReConstructBatchByOrder(batch[0])
	ast.Contains(err.Error(), "invalid batch", "already exists in bathedCache")

	newBatch := &RequestHashBatch[consensus.FltTransaction, *consensus.FltTransaction]{
		TxList:    []*consensus.FltTransaction{tx1, tx2, tx3},
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
	pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(1))
	tx3 := ConstructTxByAccountAndNonce("account2", uint64(0))
	tx4 := ConstructTxByAccountAndNonce("account2", uint64(1))

	txHashList := make([]string, 0)
	txHashList = append(txHashList, tx1.RbftGetTxHash(), tx2.RbftGetTxHash(),
		tx3.RbftGetTxHash(), tx4.RbftGetTxHash())
	newBatch := &RequestHashBatch[consensus.FltTransaction, *consensus.FltTransaction]{
		TxList:     []*consensus.FltTransaction{tx1, tx2, tx3, tx4},
		TxHashList: txHashList,
		LocalList:  []bool{true},
		Timestamp:  time.Now().UnixNano(),
	}
	batchDigest := getBatchHash(newBatch)
	txs := make(map[uint64]*consensus.FltTransaction, 0)
	txs[uint64(1)] = tx1
	txs[uint64(2)] = tx2
	txs[uint64(3)] = tx3
	txs[uint64(4)] = tx4
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

	txHashList = make([]string, 0)
	txHashList = append(txHashList, tx5.RbftGetTxHash(), tx6.RbftGetTxHash(),
		tx7.RbftGetTxHash(), tx8.RbftGetTxHash())
	newBatch = &RequestHashBatch[consensus.FltTransaction, *consensus.FltTransaction]{
		TxList:     []*consensus.FltTransaction{tx5, tx6, tx7, tx8},
		TxHashList: txHashList,
		LocalList:  []bool{true},
		Timestamp:  time.Now().UnixNano(),
	}
	batchDigest2 := getBatchHash(newBatch)

	missingHashList = make(map[uint64]string)
	missingHashList[uint64(5)] = tx5.RbftGetTxHash()
	missingHashList[uint64(6)] = tx6.RbftGetTxHash()
	missingHashList[uint64(7)] = tx7.RbftGetTxHash()
	missingHashList[uint64(8)] = tx8.RbftGetTxHash()
	pool.txStore.missingBatch[batchDigest2] = missingHashList

	batchDigest3 := "mock batchDigest3"
	tx9 := ConstructTxByAccountAndNonce("account9", uint64(0))
	pool.txStore.missingBatch[batchDigest3] = map[uint64]string{
		0: tx9.RbftGetTxHash(),
	}

	// exist 2 missingBatch, but only completion the first batch
	batch, completionMissingBatchHashes := pool.AddNewRequests([]*consensus.FltTransaction{tx5, tx6, tx7, tx8}, false, true, false, true)
	ast.Equal(0, len(batch))
	ast.Equal(1, len(completionMissingBatchHashes))
	ast.Equal(batchDigest2, completionMissingBatchHashes[0])
	ast.Equal(1, len(pool.txStore.missingBatch))
	_, batchDigest3Exist := pool.txStore.missingBatch[batchDigest3]
	ast.True(batchDigest3Exist)
}

func TestSendMissingRequests(t *testing.T) {
	ast := assert.New(t)
	pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(1))
	tx3 := ConstructTxByAccountAndNonce("account2", uint64(0))

	txHashList := make([]string, 0)
	txHashList = append(txHashList, tx1.RbftGetTxHash(), tx2.RbftGetTxHash())
	newBatch := &RequestHashBatch[consensus.FltTransaction, *consensus.FltTransaction]{
		TxList:     []*consensus.FltTransaction{tx1, tx2},
		TxHashList: txHashList,
		LocalList:  []bool{true},
		Timestamp:  time.Now().UnixNano(),
	}
	batchDigest := getBatchHash[consensus.FltTransaction, *consensus.FltTransaction](newBatch)
	newBatch.BatchHash = batchDigest

	pool.AddNewRequests([]*consensus.FltTransaction{tx1, tx2, tx3}, false, true, false, true)
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
	actTxHash1 := txs[0].RbftGetTxHash()
	actTxHash2 := txs[1].RbftGetTxHash()
	ast.Equal(tx1.RbftGetTxHash(), actTxHash1)
	ast.Equal(tx2.RbftGetTxHash(), actTxHash2)
}

func TestGetRequestsByHashList(t *testing.T) {
	ast := assert.New(t)
	pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(1))
	tx3 := ConstructTxByAccountAndNonce("account2", uint64(0))
	tx4 := ConstructTxByAccountAndNonce("account2", uint64(1))
	tx5 := ConstructTxByAccountAndNonce("account2", uint64(3))

	batches, _ := pool.AddNewRequests([]*consensus.FltTransaction{tx1, tx2, tx3, tx4, tx5}, true, true, false, true)
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

	pool.txStore.batchedTxs = make(map[txPointer]bool)
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
	pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	pool.poolSize = 4
	tx1 := ConstructTxByAccountAndNonce("account2", uint64(1))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx3 := ConstructTxByAccountAndNonce("account1", uint64(1))
	tx4 := ConstructTxByAccountAndNonce("account2", uint64(0))
	tx5 := ConstructTxByAccountAndNonce("account1", uint64(3))

	batches, _ := pool.AddNewRequests([]*consensus.FltTransaction{tx1, tx2, tx4, tx3, tx5}, true, true, false, true)
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
	pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	pool.toleranceRemoveTime = 1 * time.Second
	tx1 := ConstructTxByAccountAndNonce("Alice", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("Bob", uint64(0))

	tx3 := ConstructTxByAccountAndNonce("Alice", uint64(2))
	tx4 := ConstructTxByAccountAndNonce("Bob", uint64(2))

	tx5 := ConstructTxByAccountAndNonce("Alice", uint64(3))
	tx6 := ConstructTxByAccountAndNonce("Bob", uint64(3))

	pool.AddNewRequests([]*consensus.FltTransaction{tx1, tx2, tx3, tx4, tx5, tx6}, true, true, false, true)
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

	readyTxA := ConstructTxByAccountAndNonce("Alice", uint64(1))
	readyTxB := ConstructTxByAccountAndNonce("Bob", uint64(1))

	pool.AddNewRequests([]*consensus.FltTransaction{readyTxA, readyTxB}, false, true, false, false)

	ast.Equal(4, len(list1.items))
	ast.Equal(4, len(list2.items))
	ast.Equal(4, pool.txStore.parkingLotIndex.size(), "tx3-tx6, remove priorityIndex must in checkpoint")
	ast.Equal(8, pool.txStore.priorityIndex.size(), "tx1-tx6, readyTxA, readyTxB")
	ast.Equal(8, pool.txStore.removeTTLIndex.size())
	ast.Equal(8, len(pool.txStore.txHashMap))

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
	ast.Equal(6, int(reqLen), "remove tx3-tx6, readyTxA, readyTxB")
	ast.Equal(2, pool.txStore.removeTTLIndex.size())
	ast.Equal(0, pool.txStore.parkingLotIndex.size())
	ast.Equal(2, pool.txStore.priorityIndex.size(), "tx1,tx2 belong to batchedTxs")
	ast.Equal(2, len(pool.txStore.txHashMap))
}

func TestRestoreOneBatch(t *testing.T) {
	ast := assert.New(t)
	pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(1))
	tx3 := ConstructTxByAccountAndNonce("account2", uint64(0))
	tx4 := ConstructTxByAccountAndNonce("account2", uint64(1))

	batches, _ := pool.AddNewRequests([]*consensus.FltTransaction{tx1, tx2, tx4}, true, true, false, true)
	ast.Equal(0, len(batches))

	batches, _ = pool.AddNewRequests([]*consensus.FltTransaction{tx3}, true, true, false, true)
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
	pool.txStore.priorityIndex.removeByOrderedQueueKey(poolTx)
	err = pool.RestoreOneBatch(batches[0].BatchHash)
	ast.NotNil(err, "can't find tx from priorityIndex")

	pool.txStore.priorityIndex.insertByOrderedQueueKey(poolTx)
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
	pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	pool.poolSize = 3
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("account1", uint64(1))
	tx3 := ConstructTxByAccountAndNonce("account2", uint64(0))

	pool.AddNewRequests([]*consensus.FltTransaction{tx1, tx2, tx3}, false, true, false, true)
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
	pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))

	pool.AddNewRequests([]*consensus.FltTransaction{tx1}, false, true, false, true)

	tx := pool.GetPendingTxByHash(tx1.RbftGetTxHash())
	ast.Equal(tx1.RbftGetTxHash(), tx.RbftGetTxHash())
}

func TestGetPendingTxCount(t *testing.T) {
	ast := assert.New(t)
	pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx1 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx2 := ConstructTxByAccountAndNonce("account2", uint64(0))
	ast.Equal(uint64(0), pool.GetTotalPendingTxCount())
	pool.AddNewRequests([]*consensus.FltTransaction{tx1, tx2}, false, true, false, false)
	ast.Equal(uint64(2), pool.GetTotalPendingTxCount())
}

func TestGetMeta(t *testing.T) {
	ast := assert.New(t)

	t.Run("GetMeta with empty txpool", func(t *testing.T) {
		pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
		m := pool.GetMeta(false)
		ast.NotNil(m)
		ast.Equal(uint64(0), m.TxCount)
		ast.Equal(uint64(0), m.ReadyTxCount)
		ast.Equal(0, len(m.Batches))
		ast.Equal(0, len(m.MissingBatchTxs))
		ast.Equal(0, len(m.Accounts))
	})

	t.Run("GetAccountMeta with empty txpool", func(t *testing.T) {
		pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
		am := pool.GetAccountMeta("", false)
		ast.NotNil(am)
		ast.Equal(uint64(0), am.CommitNonce)
		ast.Equal(uint64(0), am.PendingNonce)
		ast.Equal(uint64(0), am.TxCount)
		ast.Equal(0, len(am.Txs))
		ast.Equal(0, len(am.SimpleTxs))
	})

	t.Run("GetMeta with not empty txpool", func(t *testing.T) {
		pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
		tx := ConstructTxByAccountAndNonce("account1", uint64(0))
		pool.batchSize = 1
		pool.AddNewRequests([]*consensus.FltTransaction{tx}, true, true, false, true)
		m := pool.GetMeta(false)
		ast.NotNil(m)
		ast.Equal(uint64(1), m.TxCount)
		ast.Equal(uint64(0), m.ReadyTxCount)
		ast.Equal(1, len(m.Batches))
		ast.Equal(0, len(m.MissingBatchTxs))
		ast.Equal(1, len(m.Accounts))
	})

	t.Run("GetMeta with not empty txpool", func(t *testing.T) {
		pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
		tx := ConstructTxByAccountAndNonce("account1", uint64(0))
		pool.batchSize = 2
		pool.AddNewRequests([]*consensus.FltTransaction{tx}, true, true, false, true)

		am := pool.GetAccountMeta(tx.RbftGetFrom(), false)
		ast.NotNil(am)
		ast.Equal(uint64(0), am.CommitNonce)
		ast.Equal(uint64(1), am.PendingNonce)
		ast.Equal(uint64(1), am.TxCount)
		ast.Equal(0, len(am.Txs))
		ast.Equal(1, len(am.SimpleTxs))

		am = pool.GetAccountMeta(tx.RbftGetFrom(), true)
		ast.NotNil(am)
		ast.Equal(uint64(0), am.CommitNonce)
		ast.Equal(uint64(1), am.PendingNonce)
		ast.Equal(uint64(1), am.TxCount)
		ast.Equal(1, len(am.Txs))
		ast.Equal(0, len(am.SimpleTxs))
	})
}

func TestRemoveStateUpdatingTxs(t *testing.T) {
	ast := assert.New(t)
	pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx1_0 := ConstructTxByAccountAndNonce("account1", uint64(0))
	tx1_1 := ConstructTxByAccountAndNonce("account1", uint64(1))
	tx2_0 := ConstructTxByAccountAndNonce("account2", uint64(0))
	tx2_1 := ConstructTxByAccountAndNonce("account2", uint64(1))
	txs := []*consensus.FltTransaction{tx1_1, tx1_0, tx2_1, tx2_0}
	pool.addNewRequests(txs, false, false, false, true, false)
	ast.Equal(4, len(pool.txStore.txHashMap))
	ast.Equal(2, pool.txStore.allTxs[tx1_0.RbftGetFrom()].index.size())
	ast.Equal(2, pool.txStore.allTxs[tx2_0.RbftGetFrom()].index.size())
	ast.Equal(4, pool.txStore.priorityIndex.size())

	removeTxs := []string{tx1_0.RbftGetTxHash(), tx2_0.RbftGetTxHash()}
	pool.RemoveStateUpdatingTxs(removeTxs)
	ast.Equal(2, len(pool.txStore.txHashMap))
	ast.Equal(1, pool.txStore.allTxs[tx1_0.RbftGetFrom()].index.size(), "successful remove tx1_0")
	ast.Equal(1, pool.txStore.allTxs[tx2_0.RbftGetFrom()].index.size(), "successful remove tx2_0")
	ast.Equal(2, pool.txStore.priorityIndex.size())
}

func TestDuplicateTxs(t *testing.T) {
	ast := assert.New(t)
	pool := mockTxPoolImpl[consensus.FltTransaction, *consensus.FltTransaction]()
	tx := ConstructTxByAccountAndNonce("account1", uint64(0))
	time.Sleep(1 * time.Millisecond)
	txDup := ConstructTxByAccountAndNonce("account1", uint64(0))

	txs := []*consensus.FltTransaction{tx, txDup}
	pool.addNewRequests(txs, false, false, false, true, false)
	ast.Equal(1, len(pool.txStore.txHashMap))
	ast.Equal(tx.RbftGetTxHash(), pool.txStore.allTxs[tx.RbftGetFrom()].items[0].rawTx.RbftGetTxHash())

	tx2 := ConstructTxByAccountAndNonce("account1", uint64(2))
	time.Sleep(1 * time.Millisecond)
	tx2Dup := ConstructTxByAccountAndNonce("account1", uint64(2))
	txs = []*consensus.FltTransaction{tx2, tx2Dup}
	pool.addNewRequests(txs, false, false, false, true, false)
	ast.Equal(2, len(pool.txStore.txHashMap))
	p, ok := pool.txStore.txHashMap[tx2.RbftGetTxHash()]
	ast.False(ok)
	p, ok = pool.txStore.txHashMap[tx2Dup.RbftGetTxHash()]
	ast.True(ok)
	ast.Equal(uint64(2), p.nonce)
	ast.Equal(tx2Dup.RbftGetTxHash(), pool.txStore.allTxs[tx2.RbftGetFrom()].items[2].rawTx.RbftGetTxHash())

	tx3 := ConstructTxByAccountAndNonce("account1", uint64(3))
	time.Sleep(1 * time.Millisecond)
	tx3Dup := ConstructTxByAccountAndNonce("account1", uint64(3))
	txs = []*consensus.FltTransaction{tx3}
	pool.addNewRequests(txs, false, false, false, true, false)
	ast.Equal(3, len(pool.txStore.txHashMap))
	p, ok = pool.txStore.txHashMap[tx3.RbftGetTxHash()]
	ast.True(ok)
	ast.Equal(uint64(3), p.nonce)
	ast.Equal(tx3.RbftGetTxHash(), pool.txStore.allTxs[tx3.RbftGetFrom()].items[3].rawTx.RbftGetTxHash())

	txs = []*consensus.FltTransaction{tx3Dup}
	pool.addNewRequests(txs, false, false, false, true, false)
	ast.Equal(3, len(pool.txStore.txHashMap))
	p, ok = pool.txStore.txHashMap[tx3.RbftGetTxHash()]
	ast.False(ok)
	ast.Nil(p)
	p, ok = pool.txStore.txHashMap[tx3Dup.RbftGetTxHash()]
	ast.True(ok)
	ast.Equal(uint64(3), p.nonce)
	ast.Equal(tx3Dup.RbftGetTxHash(), pool.txStore.allTxs[tx3.RbftGetFrom()].items[3].rawTx.RbftGetTxHash())
}
