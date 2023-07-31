// Copyright 2016-2017 Hyperchain Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package txpool

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/common/fancylogger"
	"github.com/axiomesh/axiom-bft/common/metrics/disabled"
	"github.com/stretchr/testify/assert"
)

type defaultChainSupport[T any, Constraint consensus.TXConstraint[T]] struct {
	ledgerTxsSet map[string][]byte // tx hash as key
}

func (dcs *defaultChainSupport[T, Constraint]) IsRequestsExist(txs [][]byte) []bool {
	results := make([]bool, len(txs))
	for i := range results {
		_, ok := dcs.ledgerTxsSet[requestHash[T, Constraint](txs[i])]
		results[i] = ok
	}
	return results
}

func (dcs *defaultChainSupport[T, Constraint]) CheckSigns(txs [][]byte) {
	return
}

// NewRawLogger returns a raw logger used in test in which logger messages
// will be printed to Stdout with string formatter.
func NewRawLogger() *fancylogger.Logger {
	rawLogger := fancylogger.NewLogger("test", fancylogger.DEBUG)
	consoleFormatter := &fancylogger.StringFormatter{
		EnableColors:    true,
		TimestampFormat: "2006-01-02T15:04:05.000",
		IsTerminal:      true,
	}
	consoleBackend := fancylogger.NewIOBackend(consoleFormatter, os.Stdout)
	rawLogger.SetBackends(consoleBackend)
	rawLogger.SetEnableCaller(true)
	return rawLogger
}

func newDefaultConfig[T any, Constraint consensus.TXConstraint[T]](lSet map[string][]byte) (Config, chainSupport) {
	conf := Config{
		BatchMemLimit: false,
		MetricsProv:   &disabled.Provider{},
		Logger:        NewRawLogger(),
	}
	dcs := &defaultChainSupport[T, Constraint]{
		ledgerTxsSet: make(map[string][]byte),
	}
	if lSet != nil {
		dcs.ledgerTxsSet = lSet
	}
	return conf, dcs
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func newRandomTx(n int) *consensus.Transaction {
	txValue := make([]byte, n)
	for i := range txValue {
		txValue[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return &consensus.Transaction{
		Value: txValue,
	}
}

func newCTX(n int) *consensus.Transaction {
	txValue := make([]byte, n)
	for i := range txValue {
		txValue[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return &consensus.Transaction{
		Value:  txValue,
		TxType: consensus.Transaction_CTX,
	}
}

func Test_NewTxPool(t *testing.T) {
	conf, rl := newDefaultConfig[consensus.Transaction](nil)
	pool := newTxPoolImpl[consensus.Transaction](DefaultNamespace, rl, conf)
	_ = pool.Start()
	assert.Equal(t, DefaultPoolSize, pool.poolSize)
	assert.Equal(t, DefaultBatchSize, pool.batchSize)
	assert.Equal(t, DefaultBatchMaxMem, pool.batchMaxMem)
	assert.Equal(t, DefaultToleranceTime, pool.toleranceTime)
}

func Test_Start_Stop(t *testing.T) {
	conf, rl := newDefaultConfig[consensus.Transaction](nil)
	pool := newTxPoolImpl[consensus.Transaction](DefaultNamespace, rl, conf)
	err := pool.Start()
	assert.Nil(t, err)
	pool.Stop()
}

func Test_AddNewRequest(t *testing.T) {
	testAddNewRequest[consensus.Transaction](t)
}

func testAddNewRequest[T any, Constraint consensus.TXConstraint[T]](t *testing.T) {
	conf, rl := newDefaultConfig[T, Constraint](nil)
	conf.BatchSize = 2
	pool := newTxPoolImpl[T, Constraint](DefaultNamespace, rl, conf)
	_ = pool.Start()

	// insert a tx through primary
	tx1 := newRandomTx(10)
	txBytes1, err := tx1.Marshal()
	assert.Nil(t, err)
	hashBatch1, _ := pool.AddNewRequests([][]byte{txBytes1}, true, false)
	assert.Nil(t, hashBatch1)

	// insert a duplicate tx through primary
	pool.AddNewRequests([][]byte{txBytes1}, true, false)
	assert.Equal(t, 1, pool.nonBatchedTxs.Len())

	// insert a tx through replica
	tx2 := newRandomTx(10)
	txBytes2, err := tx2.Marshal()
	assert.Nil(t, err)
	hashBatch2, _ := pool.AddNewRequests([][]byte{txBytes2}, false, false)
	assert.Nil(t, hashBatch2)

	// insert a duplicate tx through replica
	pool.AddNewRequests([][]byte{txBytes2}, false, false)
	assert.Equal(t, 2, pool.nonBatchedTxs.Len())

	// insert a tx through primary that reach size limit
	tx3 := newRandomTx(10)
	txBytes3, err := tx3.Marshal()
	assert.Nil(t, err)
	hashBatch3, _ := pool.AddNewRequests([][]byte{txBytes3}, true, false)
	assert.Equal(t, 1, len(hashBatch3))
	assert.Equal(t, 1, pool.nonBatchedTxs.Len())

	// insert a duplicate tx(batched) through primary
	pool.AddNewRequests([][]byte{txBytes1}, true, false)
	assert.Equal(t, 1, pool.nonBatchedTxs.Len())

	// insert a duplicate tx(batched) through replica
	pool.AddNewRequests([][]byte{txBytes2}, false, false)
	assert.Equal(t, 1, pool.nonBatchedTxs.Len())

	txs := pool.GetUncommittedTransactions(1)
	assert.Equal(t, 1, len(txs))

	pool.Reset(nil)
	assert.Equal(t, 0, pool.nonBatchedTxs.Len())

	// insert a duplicate tx(batched) through replica
	tx := newRandomTx(5)
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	pool.AddNewRequests([][]byte{txBytes}, false, false)
	ctx := newCTX(10)
	ctxBytes, err := ctx.Marshal()
	assert.Nil(t, err)
	pool.AddNewRequests([][]byte{ctxBytes}, false, false)
	assert.Equal(t, 2, pool.nonBatchedTxs.Len())
	tx = newRandomTx(15)
	txBytes, err = tx.Marshal()
	assert.Nil(t, err)
	batch, _ := pool.AddNewRequests([][]byte{txBytes}, true, false)
	assert.Equal(t, true, batch[0].IsConfBatch())
	assert.Equal(t, 2, pool.nonBatchedTxs.Len())
}

func Test_GenerateRequestBatch(t *testing.T) {
	conf, rl := newDefaultConfig[consensus.Transaction](nil)
	conf.BatchSize = 2
	pool := newTxPoolImpl[consensus.Transaction](DefaultNamespace, rl, conf)
	_ = pool.Start()

	// generate batch when nonbatch < batch size limit
	tx := newRandomTx(10)
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	pool.AddNewRequests([][]byte{txBytes}, false, false)
	pool.GenerateRequestBatch()
	assert.Equal(t, 1, len(pool.batchStore))
	assert.Equal(t, 0, pool.nonBatchedTxs.Len())

	// generate batch when nonbatch > batch size limit
	for i := 0; i < 3; i++ {
		tx = newRandomTx(10)
		txBytes, err = tx.Marshal()
		assert.Nil(t, err)
		pool.AddNewRequests([][]byte{txBytes}, false, false)
	}
	pool.GenerateRequestBatch()
	assert.Equal(t, 2, len(pool.batchStore))
	assert.Equal(t, 1, pool.nonBatchedTxs.Len())
}

func Test_SplitBatch(t *testing.T) {
	conf, rl := newDefaultConfig[consensus.Transaction](nil)
	conf.BatchMemLimit = false
	conf.BatchMaxMem = 50
	pool := newTxPoolImpl[consensus.Transaction](DefaultNamespace, rl, conf)
	_ = pool.Start()

	// add 10 txs
	for i := 0; i < 10; i++ {
		tx := newRandomTx(10)
		txBytes, err := tx.Marshal()
		assert.Nil(t, err)
		pool.AddNewRequests([][]byte{txBytes}, false, false)
	}

	nonBatchedTxLen := pool.nonBatchedTxs.Len()
	batches := pool.GenerateRequestBatch()
	assert.Equal(t, 1, len(batches))
	assert.Equal(t, 0, pool.nonBatchedTxs.Len())
	assert.Equal(t, nonBatchedTxLen, len(batches[0].TxList)+pool.nonBatchedTxs.Len())

	pool.batchMemLimit = true
	pool.RestorePool()
	batches = pool.GenerateRequestBatch()
	assert.Equal(t, 1, len(batches))
	assert.NotEqual(t, 0, pool.nonBatchedTxs.Len())
	assert.Equal(t, nonBatchedTxLen, len(batches[0].TxList)+pool.nonBatchedTxs.Len())

	pool.batchMaxMem = 200
	pool.batchMemLimit = true
	pool.RestorePool()
	batches = pool.GenerateRequestBatch()
	assert.Equal(t, 1, len(batches))
	assert.Equal(t, 0, pool.nonBatchedTxs.Len())
	assert.Equal(t, nonBatchedTxLen, len(batches[0].TxList)+pool.nonBatchedTxs.Len())
}

func Test_IsPoolFull(t *testing.T) {
	conf, rl := newDefaultConfig[consensus.Transaction](nil)
	conf.K = 1
	conf.PoolSize = 2
	conf.BatchSize = 1
	pool := newTxPoolImpl[consensus.Transaction](DefaultNamespace, rl, conf)
	_ = pool.Start()

	tx := newRandomTx(10)
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	pool.AddNewRequests([][]byte{txBytes}, false, false)
	assert.False(t, pool.IsPoolFull())

	tx = newRandomTx(10)
	txBytes, err = tx.Marshal()
	assert.Nil(t, err)
	pool.AddNewRequests([][]byte{txBytes}, false, false)
	assert.True(t, pool.IsPoolFull())
}

func Test_MakePoolSizeLegal(t *testing.T) {
	conf, rl := newDefaultConfig[consensus.Transaction](nil)
	conf.K = 1
	conf.PoolSize = 1
	conf.BatchSize = 1
	pool := newTxPoolImpl[consensus.Transaction](DefaultNamespace, rl, conf)
	_ = pool.Start()

	tx := newRandomTx(10)
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	pool.AddNewRequests([][]byte{txBytes}, false, false)
	assert.False(t, pool.IsPoolFull())

	tx = newRandomTx(10)
	txBytes, err = tx.Marshal()
	assert.Nil(t, err)
	pool.AddNewRequests([][]byte{txBytes}, false, false)
	assert.False(t, pool.IsPoolFull())

	tx = newRandomTx(10)
	txBytes, err = tx.Marshal()
	assert.Nil(t, err)
	pool.AddNewRequests([][]byte{txBytes}, false, false)
	assert.False(t, pool.IsPoolFull())

	tx = newRandomTx(10)
	txBytes, err = tx.Marshal()
	assert.Nil(t, err)
	pool.AddNewRequests([][]byte{txBytes}, false, false)
	assert.True(t, pool.IsPoolFull())
}

func Test_HasPendingRequestInPool(t *testing.T) {
	conf, rl := newDefaultConfig[consensus.Transaction](nil)
	pool := newTxPoolImpl[consensus.Transaction](DefaultNamespace, rl, conf)
	_ = pool.Start()

	assert.False(t, pool.HasPendingRequestInPool())

	tx := newRandomTx(10)
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	pool.AddNewRequests([][]byte{txBytes}, false, false)
	assert.True(t, pool.HasPendingRequestInPool())
}

func Test_RemoveBatches(t *testing.T) {
	conf, rl := newDefaultConfig[consensus.Transaction](nil)
	pool := newTxPoolImpl[consensus.Transaction](DefaultNamespace, rl, conf)
	_ = pool.Start()

	var hashList []string

	// generate two batches in batchStore
	tx := newRandomTx(10)
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	pool.AddNewRequests([][]byte{txBytes}, false, false)
	batches := pool.GenerateRequestBatch()
	hash1 := batches[0].BatchHash
	hashList = append(hashList, hash1)

	tx = newRandomTx(10)
	txBytes, err = tx.Marshal()
	assert.Nil(t, err)
	pool.AddNewRequests([][]byte{txBytes}, false, false)
	batches = pool.GenerateRequestBatch()
	hash2 := batches[0].BatchHash
	hashList = append(hashList, hash2)

	assert.Equal(t, 2, len(pool.batchStore))

	// remove the batched batches
	hashList = append(hashList, "test")
	pool.RemoveBatches(hashList)
	assert.Equal(t, 0, len(pool.batchStore))
}

func Test_RestoreOneBatch_RestorePool(t *testing.T) {
	conf, rl := newDefaultConfig[consensus.Transaction](nil)
	pool := newTxPoolImpl[consensus.Transaction](DefaultNamespace, rl, conf)
	_ = pool.Start()

	// generate three batches in batchStore
	tx := newRandomTx(10)
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	pool.AddNewRequests([][]byte{txBytes}, false, false)
	batches := pool.GenerateRequestBatch()
	hash1 := batches[0].BatchHash

	tx = newRandomTx(10)
	txBytes, err = tx.Marshal()
	assert.Nil(t, err)
	pool.AddNewRequests([][]byte{txBytes}, false, false)
	pool.GenerateRequestBatch()

	tx = newRandomTx(10)
	txBytes, err = tx.Marshal()
	assert.Nil(t, err)
	pool.AddNewRequests([][]byte{txBytes}, false, false)
	pool.GenerateRequestBatch()

	assert.Equal(t, 3, len(pool.batchStore))
	assert.Equal(t, 0, pool.nonBatchedTxs.Len())

	// restore the first batch
	err = pool.RestoreOneBatch(hash1)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(pool.batchStore))
	assert.Equal(t, 1, pool.nonBatchedTxs.Len())

	// restore all batches
	pool.RestorePool()
	assert.Equal(t, 0, len(pool.batchStore))
	assert.Equal(t, 3, pool.nonBatchedTxs.Len())
}

func Test_GetRequestsByHashList_WithoutMissing(t *testing.T) {
	testGetRequestsByHashListWithoutMissing[consensus.Transaction](t)
}

func testGetRequestsByHashListWithoutMissing[T any, Constraint consensus.TXConstraint[T]](t *testing.T) {
	conf, rl := newDefaultConfig[T, Constraint](nil)
	primaryPool := newTxPoolImpl[T, Constraint](DefaultNamespace, rl, conf)
	_ = primaryPool.Start()
	replicaConf, replicaRl := newDefaultConfig[T, Constraint](nil)
	replicaPool := newTxPoolImpl[T, Constraint](DefaultNamespace, replicaRl, replicaConf)
	_ = replicaPool.Start()

	// test no deDuplicate
	tx1, tx2, tx3, tx4 := newRandomTx(10), newRandomTx(10), newRandomTx(10), newRandomTx(10)
	txBytes1, err := tx1.Marshal()
	assert.Nil(t, err)
	txBytes2, err := tx2.Marshal()
	assert.Nil(t, err)
	txBytes3, err := tx3.Marshal()
	assert.Nil(t, err)
	txBytes4, err := tx4.Marshal()
	assert.Nil(t, err)
	primaryPool.AddNewRequests([][]byte{txBytes1}, true, false)
	primaryPool.AddNewRequests([][]byte{txBytes2}, true, false)
	batches := primaryPool.GenerateRequestBatch()
	batch1 := batches[0]

	replicaPool.AddNewRequests([][]byte{txBytes1}, false, false)
	replicaPool.AddNewRequests([][]byte{txBytes2}, false, false)
	txs, localList, _, _ := replicaPool.GetRequestsByHashList(batch1.BatchHash, batch1.Timestamp, batch1.TxHashList, nil)
	assert.Equal(t, [][]byte{txBytes1, txBytes2}, txs)
	assert.Equal(t, []bool{false, false}, localList)

	// get the same batch again, should return the same batch.
	txs1, localList2, missing, err := replicaPool.GetRequestsByHashList(batch1.BatchHash, batch1.Timestamp, batch1.TxHashList, nil)
	assert.Equal(t, [][]byte{txBytes1, txBytes2}, txs1)
	assert.Equal(t, []bool{false, false}, localList2)
	assert.Nil(t, missing)
	assert.Nil(t, err)

	// test with deDuplicate tx1
	primaryPool.RestorePool()
	primaryPool.AddNewRequests([][]byte{txBytes3}, true, false)
	primaryPool.AddNewRequests([][]byte{txBytes4}, true, false)
	batches = primaryPool.GenerateRequestBatch()
	batch2 := batches[0]

	replicaPool.AddNewRequests([][]byte{txBytes3}, false, false)
	replicaPool.AddNewRequests([][]byte{txBytes4}, false, false)
	_, _, _, err = replicaPool.GetRequestsByHashList(batch2.BatchHash, batch2.Timestamp, batch2.TxHashList, []string{requestHash[T, Constraint](txBytes1)})
	assert.Equal(t, ErrDuplicateTx, err)

	replicaPool.RestorePool()
	replicaPool.batchStore[batch1.BatchHash] = batch1
	txs, localList, _, _ = replicaPool.GetRequestsByHashList(batch2.BatchHash, batch2.Timestamp, batch2.TxHashList, []string{requestHash[T, Constraint](txBytes1), requestHash[T, Constraint](txBytes2)})
	assert.Equal(t, txs, [][]byte{txBytes1, txBytes2, txBytes3, txBytes4})
	assert.Equal(t, localList, []bool{false, false, false, false})
}

func Test_GetRequestsByHashList_WithMissing(t *testing.T) {
	testGetRequestsByHashListWithMissing[consensus.Transaction](t)
}
func testGetRequestsByHashListWithMissing[T any, Constraint consensus.TXConstraint[T]](t *testing.T) {
	conf, rl := newDefaultConfig[T, Constraint](nil)
	primaryPool := newTxPoolImpl[T, Constraint](DefaultNamespace, rl, conf)
	_ = primaryPool.Start()
	replicaConf, replicaRl := newDefaultConfig[T, Constraint](nil)
	replicaPool := newTxPoolImpl[T, Constraint](DefaultNamespace, replicaRl, replicaConf)
	_ = replicaPool.Start()

	// [1] primary generate batch with [tx1, tx2, tx3, tx4]
	tx1, tx2, tx3, tx4 := newRandomTx(10), newRandomTx(10), newRandomTx(10), newRandomTx(10)
	txBytes1, err := tx1.Marshal()
	assert.Nil(t, err)
	txBytes2, err := tx2.Marshal()
	assert.Nil(t, err)
	txBytes3, err := tx3.Marshal()
	assert.Nil(t, err)
	txBytes4, err := tx4.Marshal()
	assert.Nil(t, err)
	primaryPool.AddNewRequests([][]byte{txBytes1}, true, false)
	primaryPool.AddNewRequests([][]byte{txBytes2}, true, false)
	primaryPool.AddNewRequests([][]byte{txBytes3}, true, false)
	primaryPool.AddNewRequests([][]byte{txBytes4}, true, false)
	batches := primaryPool.GenerateRequestBatch()
	batch := batches[0]

	// [2] replica only has [tx1, tx2] in txpool
	replicaPool.AddNewRequests([][]byte{txBytes1}, true, false)
	replicaPool.AddNewRequests([][]byte{txBytes2}, true, false)
	_, _, missing, _ := replicaPool.GetRequestsByHashList(batch.BatchHash, batch.Timestamp, batch.TxHashList, []string{requestHash[T, Constraint](txBytes1), requestHash[T, Constraint](txBytes2)})
	expectMissing := make(map[uint64]string)
	expectMissing[uint64(2)] = requestHash[T, Constraint](txBytes3)
	expectMissing[uint64(3)] = requestHash[T, Constraint](txBytes4)
	assert.Equal(t, expectMissing, missing)

	// get the same batch again, should return the same missing record.
	txs1, localList2, missing, err := replicaPool.GetRequestsByHashList(batch.BatchHash, batch.Timestamp, batch.TxHashList, []string{requestHash[T, Constraint](txBytes1), requestHash[T, Constraint](txBytes2)})
	assert.Nil(t, txs1)
	assert.Nil(t, localList2)
	assert.Equal(t, expectMissing, missing)
	assert.Nil(t, err)

	// [3] primary send missing
	// 1. input wrong batchhash
	_, err = primaryPool.SendMissingRequests("test", missing)
	assert.Equal(t, ErrNoBatch, err)
	// 2. input wrong missing
	wMissing := make(map[uint64]string)
	wMissing[0] = "test"
	_, err = primaryPool.SendMissingRequests(batch.BatchHash, wMissing)
	assert.Equal(t, ErrMismatch, err)
	// 3. return the right missing
	expectTxs := make(map[uint64][]byte)
	expectTxs[uint64(2)] = txBytes3
	expectTxs[uint64(3)] = txBytes4
	txs, _ := primaryPool.SendMissingRequests(batch.BatchHash, missing)
	assert.Equal(t, expectTxs, txs)

	// [4] replica receive missing
	mockTxs := make(map[uint64][]byte)
	mockTxs[uint64(2)] = txBytes3
	// receive tx3.
	err = replicaPool.ReceiveMissingRequests(batch.BatchHash, mockTxs)
	assert.Nil(t, err)
	_, _, missing, _ = replicaPool.GetRequestsByHashList(batch.BatchHash, batch.Timestamp, batch.TxHashList, nil)
	expectMissing = make(map[uint64]string)
	expectMissing[uint64(3)] = requestHash[T, Constraint](txBytes4)
	assert.Equal(t, expectMissing, missing)
	// receive tx3 again.
	err = replicaPool.ReceiveMissingRequests(batch.BatchHash, mockTxs)
	assert.Nil(t, err)
	mockTxs[uint64(3)] = txBytes4
	// receive tx4.
	_ = replicaPool.ReceiveMissingRequests(batch.BatchHash, mockTxs)
	_, _, missing, _ = replicaPool.GetRequestsByHashList(batch.BatchHash, batch.Timestamp, batch.TxHashList, nil)
	assert.Nil(t, missing)
}

func Test_FilterOutOfDateRequests(t *testing.T) {
	testFilterOutOfDateRequests[consensus.Transaction](t)
}

func testFilterOutOfDateRequests[T any, Constraint consensus.TXConstraint[T]](t *testing.T) {
	conf, rl := newDefaultConfig[T, Constraint](nil)
	pool := newTxPoolImpl[T, Constraint](DefaultNamespace, rl, conf)
	_ = pool.Start()

	tx1, tx2, tx3, tx4 := newRandomTx(10), newRandomTx(10), newRandomTx(10), newRandomTx(10)
	txBytes1, err := tx1.Marshal()
	assert.Nil(t, err)
	txBytes2, err := tx2.Marshal()
	assert.Nil(t, err)
	txBytes3, err := tx3.Marshal()
	assert.Nil(t, err)
	txBytes4, err := tx4.Marshal()
	assert.Nil(t, err)
	// add two txs
	pool.toleranceTime = 1 * time.Second
	pool.AddNewRequests([][]byte{txBytes1}, false, false)
	pool.AddNewRequests([][]byte{txBytes2}, false, true)

	// sleep for 2s
	time.Sleep(2 * time.Second)
	pool.AddNewRequests([][]byte{txBytes3}, false, false)
	pool.AddNewRequests([][]byte{txBytes4}, false, true)

	txs, _ := pool.FilterOutOfDateRequests()
	assert.Equal(t, txBytes2, txs[0])
	assert.False(t, pool.nonBatchedTxs.Has(requestHash[T, Constraint](txBytes1)))
	assert.True(t, pool.nonBatchedTxs.Has(requestHash[T, Constraint](txBytes2)))
	assert.True(t, pool.nonBatchedTxs.Has(requestHash[T, Constraint](txBytes3)))
	assert.True(t, pool.nonBatchedTxs.Has(requestHash[T, Constraint](txBytes4)))
}

func Test_ReConstructBatchByOrder(t *testing.T) {
	testReConstructBatchByOrder[consensus.Transaction](t)
}

func testReConstructBatchByOrder[T any, Constraint consensus.TXConstraint[T]](t *testing.T) {
	conf, rl := newDefaultConfig[T, Constraint](nil)
	pool := newTxPoolImpl[T, Constraint](DefaultNamespace, rl, conf)
	_ = pool.Start()

	// generate three batches in batchStore
	tx := newRandomTx(10)
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	pool.AddNewRequests([][]byte{txBytes}, false, false)
	batches := pool.GenerateRequestBatch()
	batch1 := batches[0]

	tx = newRandomTx(10)
	txBytes, err = tx.Marshal()
	assert.Nil(t, err)
	pool.AddNewRequests([][]byte{txBytes}, false, false)
	batches = pool.GenerateRequestBatch()
	batch2 := batches[0]

	// restore the second batch
	_ = pool.RestoreOneBatch(batch2.BatchHash)

	rtx := newRandomTx(10)
	rtxBytes, err := rtx.Marshal()
	assert.Nil(t, err)
	wBatch := &RequestHashBatch[T, Constraint]{
		BatchHash:  "BatchHash",
		TxHashList: []string{"TxHashList"},
		TxList:     [][]byte{rtxBytes},
		LocalList:  []bool{false},
		TimeList:   []int64{time.Now().UnixNano()},
		Timestamp:  time.Now().UnixNano(),
	}
	// 1. input existes batch
	_, err = pool.ReConstructBatchByOrder(batch1)
	assert.Equal(t, ErrInvalidBatch, err)

	// 2. input mismatch batch
	wBatch.TxHashList = []string{"1", "2"}
	_, err = pool.ReConstructBatchByOrder(wBatch)
	assert.Equal(t, ErrInvalidBatch, err)

	wBatch.TxHashList = []string{"TxHashList"}
	_, err = pool.ReConstructBatchByOrder(wBatch)
	assert.Equal(t, ErrInvalidBatch, err)

	// 3. right batch with no duplicate
	ret, err := pool.ReConstructBatchByOrder(batch2)
	assert.Nil(t, ret)
	assert.Nil(t, err)
}

func Test_Reset(t *testing.T) {
	testReset[consensus.Transaction](t)
}

func testReset[T any, Constraint consensus.TXConstraint[T]](t *testing.T) {
	ledgerTxsSet := make(map[string][]byte)
	conf, cs := newDefaultConfig[T, Constraint](ledgerTxsSet)
	conf.BatchSize = 1
	tx1 := newRandomTx(10)
	tx2 := newRandomTx(10)

	tx3 := newRandomTx(10)
	tx4 := newRandomTx(10)
	tx5 := newRandomTx(10)

	txBytes1, err := tx1.Marshal()
	assert.Nil(t, err)
	txBytes2, err := tx2.Marshal()
	assert.Nil(t, err)
	txBytes3, err := tx3.Marshal()
	assert.Nil(t, err)
	txBytes4, err := tx4.Marshal()
	assert.Nil(t, err)
	txBytes5, err := tx5.Marshal()
	assert.Nil(t, err)

	pool := newTxPoolImpl[T, Constraint](DefaultNamespace, cs, conf)
	_ = pool.Start()

	hashBatch1, _ := pool.AddNewRequests([][]byte{txBytes1}, true, true) // tx1 in saveBatches
	pool.AddNewRequests([][]byte{txBytes2}, true, true)                  // tx2 not in saveBatches

	pool.AddNewRequests([][]byte{txBytes3}, false, false) // tx3 not local
	pool.AddNewRequests([][]byte{txBytes4}, false, true)  // tx4 included in ledger
	pool.AddNewRequests([][]byte{txBytes5}, false, true)  // tx5 not included in ledger

	ledgerTxsSet[requestHash[T, Constraint](txBytes4)] = txBytes4 // tx4 is included in ledger

	pool.Reset([]string{hashBatch1[0].BatchHash})

	assert.Equal(t, 1, len(pool.batchStore))
	assert.NotNil(t, pool.batchStore[hashBatch1[0].BatchHash])

	assert.Equal(t, 1, pool.nonBatchedTxs.Len())
	req, ok := pool.nonBatchedTxs.Front().Value.(*request)
	assert.True(t, ok)
	assert.Equal(t, txBytes5, req.tx)
}
