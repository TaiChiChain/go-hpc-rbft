package mempool

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/google/btree"
)

// Mempool contains all currently known transactions.
type mempoolImpl[T any, Constraint consensus.TXConstraint[T]] struct {
	logger              Logger
	localID             uint64
	batchSize           uint64
	isTimed             bool
	txStore             *transactionStore[T, Constraint] // store all transactions info
	toleranceTime       time.Duration
	toleranceRemoveTime time.Duration
	poolSize            uint64
	getAccountNonce     GetAccountNonceFunc
	pendingMu           sync.Mutex
}

// AddNewRequests adds transactions into txPool.
// When current node is primary, judge if we need to generate a batch by batch size.
// When current node is backup, judge if we can eliminate some missing batches.
// local indicates if this transaction is froward internally from RPC layer or not.
// When we receive txs from other nodes(which have been added to other's tx pool) or
// locally(API layer), we need to check duplication from ledger to avoid duplication
// with committed txs on ledger.
// Also, when we receive a tx locally, we need to check if these txs are out-of-date
// time by time.
func (mpi *mempoolImpl[T, Constraint]) AddNewRequests(txs [][]byte, isPrimary, local, isReplace bool) ([]*RequestHashBatch[T, Constraint], []string) {
	return mpi.addNewRequests(txs, isPrimary, local, isReplace)
}

func (mpi *mempoolImpl[T, Constraint]) addNewRequests(txs [][]byte, isPrimary, local, isReplace bool) ([]*RequestHashBatch[T, Constraint], []string) {
	var hasConfigTx bool
	completionMissingBatchHashes := make([]string, 0)
	existTxsHash := make(map[string]struct{})
	validTxs := make(map[string][]*mempoolTransaction[T, Constraint])
	decodeTxs, err := consensus.DecodeTxs[T, Constraint](txs)
	if err != nil {
		mpi.logger.Errorf("Decode txs failed", "err", err)
		return nil, nil
	}
	for _, tx := range decodeTxs {
		txAccount := Constraint(tx).RbftGetFrom()
		txHash := Constraint(tx).RbftGetTxHash()
		txNonce := Constraint(tx).RbftGetNonce()
		// currentSeqNo means the current wanted nonce of the account
		currentSeqNo := mpi.txStore.nonceCache.getPendingNonce(txAccount)
		if txNonce < currentSeqNo {
			if !isReplace {
				mpi.logger.Warningf("Receive transaction [account: %s, nonce: %d, hash: %s], but we required %d", txAccount, txNonce, txHash, currentSeqNo)
				continue
			} else {
				if txPointer := mpi.txStore.txHashMap[txHash]; txPointer != nil {
					continue
				}
				mpi.logger.Warningf("Receive missing transaction [account: %s, nonce: %d, hash: %s] from primary, we will replace the old transaction", txAccount, txNonce, txHash)
				mpi.replaceTx(tx)
				continue
			}
		}
		if txPointer := mpi.txStore.txHashMap[txHash]; txPointer != nil {
			mpi.logger.Debugf("Transaction [account: %s, nonce: %d, hash: %s] has already existed in txHashMap", txAccount, txNonce, txHash)
			continue
		}
		_, ok := validTxs[txAccount]
		if !ok {
			validTxs[txAccount] = make([]*mempoolTransaction[T, Constraint], 0)
		}
		now := time.Now().Unix()
		txItem := &mempoolTransaction[T, Constraint]{
			rawTx:       tx,
			local:       local,
			lifeTime:    Constraint(tx).RbftGetTimeStamp(),
			arrivedTime: now,
		}
		validTxs[txAccount] = append(validTxs[txAccount], txItem)

		// check if there is config tx
		if Constraint(tx).RbftIsConfigTx() {
			hasConfigTx = true
		}

		// for replica, add validTxs to nonBatchedTxs and check if the missing transactions and batches are fetched
		if !isPrimary {
			for batchHash, missingTxsHash := range mpi.txStore.missingBatch {
				for _, missingTxHash := range missingTxsHash {
					// we have received a tx which we are fetching.
					if txHash == missingTxHash {
						existTxsHash[txHash] = struct{}{}
					}
				}
				// receive all the missing txs
				if len(existTxsHash) == len(missingTxsHash) {
					delete(mpi.txStore.missingBatch, batchHash)
					completionMissingBatchHashes = append(completionMissingBatchHashes, batchHash)
				}
			}
		}
	}

	// Process all the new transaction and merge any errors into the original slice
	dirtyAccounts := mpi.txStore.insertTxs(validTxs, local)
	// send tx to mempool store
	mpi.processDirtyAccount(dirtyAccounts)

	// if no timedBlock, generator batch by block size
	if isPrimary {
		var batches []*RequestHashBatch[T, Constraint]
		if mpi.txStore.priorityNonBatchSize >= mpi.batchSize {
			batches, err = mpi.generateRequestBatch()
			if err != nil {
				mpi.logger.Errorf("Generate batch failed, err: %s", err.Error())
				return nil, nil
			}
			return batches, nil
		}
		if hasConfigTx {
			mpi.logger.Debug("Receive config tx, we will generate a batch")
			batches, err = mpi.generateRequestBatch()
			if err != nil {
				mpi.logger.Errorf("Generate batch failed, err: %s", err.Error())
				return nil, nil
			}
			return batches, nil
		}
	}

	return nil, completionMissingBatchHashes
}

// IsConfigBatch check if such batch is config-change batch or not
func (mpi *mempoolImpl[T, Constraint]) IsConfigBatch(batchHash string) bool {
	if batch, ok := mpi.txStore.batchesCache[batchHash]; ok {
		if len(batch.TxList) == 1 {
			tx, err := consensus.DecodeTx[T, Constraint](batch.TxList[0])
			if err != nil {
				mpi.logger.Errorf("Decode tx failed", "err", err)
				return false
			}
			return Constraint(tx).RbftIsConfigTx()
		}
	}
	return false
}

// GetUncommittedTransactions returns the uncommitted transactions.
// not used
func (mpi *mempoolImpl[T, Constraint]) GetUncommittedTransactions(maxsize uint64) [][]byte {
	return [][]byte{}
}

// Start start metrics
// TODO(lrx): add metrics
func (mpi *mempoolImpl[T, Constraint]) Start() error {
	return nil
}

// Stop stop metrics
// TODO(lrx): add metrics
func (mpi *mempoolImpl[T, Constraint]) Stop() {
	return
}

// newMempoolImpl returns the mempool instance.
func newMempoolImpl[T any, Constraint consensus.TXConstraint[T]](config Config) *mempoolImpl[T, Constraint] {
	mpi := &mempoolImpl[T, Constraint]{
		localID:         config.ID,
		logger:          config.Logger,
		getAccountNonce: config.GetAccountNonce,
		isTimed:         config.IsTimed,
	}
	mpi.txStore = newTransactionStore[T, Constraint](config.GetAccountNonce)
	if config.BatchSize == 0 {
		mpi.batchSize = DefaultBatchSize
	} else {
		mpi.batchSize = config.BatchSize
	}
	if config.PoolSize == 0 {
		mpi.poolSize = DefaultPoolSize
	} else {
		mpi.poolSize = config.PoolSize
	}
	if config.ToleranceTime == 0 {
		mpi.toleranceTime = DefaultToleranceTime
	} else {
		mpi.toleranceTime = config.ToleranceTime
	}
	if config.ToleranceRemoveTime == 0 {
		mpi.toleranceRemoveTime = DefaultToleranceRemoveTime
	} else {
		mpi.toleranceRemoveTime = config.ToleranceRemoveTime
	}
	mpi.logger.Infof("MemPool pool size = %d", mpi.poolSize)
	mpi.logger.Infof("MemPool batch size = %d", mpi.batchSize)
	mpi.logger.Infof("MemPool batch mem limit = %v", config.BatchMemLimit)
	mpi.logger.Infof("MemPool batch max mem size = %d", config.BatchMaxMem)
	mpi.logger.Infof("MemPool tolerance time = %v", config.ToleranceTime)
	mpi.logger.Infof("MemPool tolerance remove time = %v", mpi.toleranceRemoveTime)
	return mpi
}

// GetPendingNonceByAccount returns the latest pending nonce of the account in mempool
func (mpi *mempoolImpl[T, Constraint]) GetPendingNonceByAccount(account string) uint64 {
	return mpi.txStore.nonceCache.getPendingNonce(account)
}

func (mpi *mempoolImpl[T, Constraint]) GetPendingTxByHash(hash string) []byte {
	key, ok := mpi.txStore.txHashMap[hash]
	if !ok {
		return nil
	}

	txMap, ok := mpi.txStore.allTxs[key.account]
	if !ok {
		return nil
	}

	item, ok := txMap.items[key.nonce]
	if !ok {
		return nil
	}

	txData, err := Constraint(item.rawTx).RbftMarshal()
	if err != nil {
		mpi.logger.Errorf("Marshal tx failed, err: %s", err.Error())
		return nil
	}
	return txData
}

// GenerateRequestBatch generates a transaction batch and post it
// to outside if there are transactions in txPool.
func (mpi *mempoolImpl[T, Constraint]) GenerateRequestBatch() []*RequestHashBatch[T, Constraint] {
	batches, err := mpi.generateRequestBatch()
	if err != nil {
		mpi.logger.Errorf("Generator batch failed, err: %s", err.Error())
		return nil
	}
	return batches
}

// Reset clears all cached txs in txPool and start with a pure empty environment, only used in abnormal status.
// except batches in saveBatches and local non-batched-txs that not included in ledger.
// todo(lrx): except saveBatches
func (mpi *mempoolImpl[T, Constraint]) Reset(saveBatches []string) {
	mpi.logger.Debug("Reset mempool...")
	oldNonceCache := mpi.txStore.nonceCache
	mpi.txStore = newTransactionStore[T, Constraint](mpi.getAccountNonce)
	mpi.txStore.nonceCache.commitNonces = oldNonceCache.commitNonces
	for account, commitNonce := range oldNonceCache.commitNonces {
		pendingNonce := oldNonceCache.getPendingNonce(account)
		mpi.logger.Debugf("Reset account %s pending nonce %d to commit nonce %d", account, pendingNonce, commitNonce)
		mpi.txStore.nonceCache.pendingNonces[account] = commitNonce
	}
}

// RestoreOneBatch moves one batch from batchStore.
func (mpi *mempoolImpl[T, Constraint]) RestoreOneBatch(hash string) error {
	batch, ok := mpi.txStore.batchesCache[hash]
	if !ok {
		return errors.New("can't find batch from batchesCache")
	}

	// remove from batchedTxs and batchStore
	for _, hash := range batch.TxHashList {
		ptr := mpi.txStore.txHashMap[hash]
		if ptr == nil {
			return fmt.Errorf("can't find tx %s from txHashMap", hash)
		}
		// check if the given tx exist in priorityIndex
		poolTx := mpi.txStore.getPoolTxByTxnPointer(ptr.account, ptr.nonce)
		key := &orderedIndexKey{poolTx.getRawTimestamp(), ptr.account, ptr.nonce}
		if tx := mpi.txStore.priorityIndex.data.Get(key); tx == nil {
			return fmt.Errorf("can't find tx %s from priorityIndex", hash)
		}
		delete(mpi.txStore.batchedTxs, *ptr)
	}
	delete(mpi.txStore.batchesCache, hash)
	mpi.increasePriorityNonBatchSize(uint64(len(batch.TxHashList)))

	mpi.logger.Debugf("Restore one batch, which hash is %s, now there are %d non-batched txs, "+
		"%d batches in txPool", hash, mpi.txStore.priorityNonBatchSize, len(mpi.txStore.batchesCache))
	return nil
}

// RemoveBatches removes several batches by given digests of
// transaction batches from the pool(batchedTxs).
func (mpi *mempoolImpl[T, Constraint]) RemoveBatches(hashList []string) {
	// update current cached commit nonce for account
	updateAccounts := make(map[string]uint64)
	for _, batchHash := range hashList {
		batch, ok := mpi.txStore.batchesCache[batchHash]
		if !ok {
			mpi.logger.Debugf("Cannot find batch %s in mempool batchedCache which may have been "+
				"discard when ReConstructBatchByOrder", batchHash)
			continue
		}
		delete(mpi.txStore.batchesCache, batchHash)
		dirtyAccounts := make(map[string]bool)
		for _, txHash := range batch.TxHashList {
			txPointer := mpi.txStore.txHashMap[txHash]
			txPointer, ok := mpi.txStore.txHashMap[txHash]
			if !ok {
				mpi.logger.Warningf("Remove transaction %s failed, Can't find it from txHashMap", txHash)
				continue
			}
			preCommitNonce := mpi.txStore.nonceCache.getCommitNonce(txPointer.account)
			// next wanted nonce
			newCommitNonce := txPointer.nonce + 1
			if preCommitNonce < newCommitNonce {
				mpi.txStore.nonceCache.setCommitNonce(txPointer.account, newCommitNonce)
				// Note!!! updating pendingNonce to commitNonce for the restart node
				pendingNonce := mpi.txStore.nonceCache.getPendingNonce(txPointer.account)
				if pendingNonce < newCommitNonce {
					updateAccounts[txPointer.account] = newCommitNonce
					mpi.txStore.nonceCache.setPendingNonce(txPointer.account, newCommitNonce)
				}
			}
			delete(mpi.txStore.txHashMap, txHash)
			delete(mpi.txStore.batchedTxs, *txPointer)
			dirtyAccounts[txPointer.account] = true
		}
		// clean related txs info in cache
		for account := range dirtyAccounts {
			commitNonce := mpi.txStore.nonceCache.getCommitNonce(account)
			if list, ok := mpi.txStore.allTxs[account]; ok {
				// remove all previous seq number txs for this account.
				removedTxs := list.forward(commitNonce + 1)
				// remove index smaller than commitNonce delete index.
				var wg sync.WaitGroup
				wg.Add(5)
				go func(ready map[string][]*mempoolTransaction[T, Constraint]) {
					defer wg.Done()
					list.index.removeBySortedNonceKeys(removedTxs)
				}(removedTxs)
				go func(ready map[string][]*mempoolTransaction[T, Constraint]) {
					defer wg.Done()
					mpi.txStore.priorityIndex.removeByOrderedQueueKeys(removedTxs)
				}(removedTxs)
				go func(ready map[string][]*mempoolTransaction[T, Constraint]) {
					defer wg.Done()
					mpi.txStore.parkingLotIndex.removeByOrderedQueueKeys(removedTxs)
				}(removedTxs)
				go func(ready map[string][]*mempoolTransaction[T, Constraint]) {
					defer wg.Done()
					mpi.txStore.localTTLIndex.removeByTTLIndexByKeys(removedTxs, Rebroadcast)
				}(removedTxs)
				go func(ready map[string][]*mempoolTransaction[T, Constraint]) {
					defer wg.Done()
					mpi.txStore.removeTTLIndex.removeByTTLIndexByKeys(removedTxs, Remove)
				}(removedTxs)
				wg.Wait()
			}
		}
	}
	readyNum := uint64(mpi.txStore.priorityIndex.size())
	// set priorityNonBatchSize to min(nonBatchedTxs, readyNum),
	if mpi.txStore.priorityNonBatchSize > readyNum {
		mpi.logger.Warningf("Set priorityNonBatchSize from %d to the length of priorityIndex %d", mpi.txStore.priorityNonBatchSize, readyNum)
		mpi.setPriorityNonBatchSize(readyNum)
	}
	for account, pendingNonce := range updateAccounts {
		mpi.logger.Debugf("Account %s update its pendingNonce to %d by commitNonce", account, pendingNonce)
	}
	mpi.logger.Debugf("Removes batches in mempool, and now there are %d non-batched txs, %d batches, "+
		"priority len: %d, parkingLot len: %d, batchedTx len: %d, txHashMap len: %d", mpi.txStore.priorityNonBatchSize,
		len(mpi.txStore.batchesCache), mpi.txStore.priorityIndex.size(), mpi.txStore.parkingLotIndex.size(),
		len(mpi.txStore.batchedTxs), len(mpi.txStore.txHashMap))
}

// GetRequestsByHashList returns the transaction list corresponding to the given hash list.
// When replicas receive hashList from primary, they need to generate a totally same
// batch to primary generated one. deDuplicateTxHashes specifies some txs which should
// be excluded from duplicate rules.
//  1. If this batch has been batched, just return its transactions without error.
//  2. If we have checked this batch and found we were missing some transactions, just
//     return the same missingTxsHash as before without error.
//  3. If one transaction in hashList has been batched before in another batch,
//     return ErrDuplicateTx
//  4. If we miss some transactions, we need to fetch these transactions from primary,
//     and return missingTxsHash without error
//  5. If this node get all transactions from pool, generate a batch and return its
//     transactions without error
func (mpi *mempoolImpl[T, Constraint]) GetRequestsByHashList(batchHash string, timestamp int64, hashList []string,
	deDuplicateTxHashes []string) (txs [][]byte, localList []bool, missingTxsHash map[uint64]string, err error) {

	if batch, ok := mpi.txStore.batchesCache[batchHash]; ok {
		// If replica already has this batch, directly return tx list
		mpi.logger.Debugf("Batch %s is already in batchesCache", batchHash)

		// If it's a config-change batch, replica turn into config-change mode
		if batch.IsConfBatch() {
			mpi.logger.Infof("Replica found a config-change batch, batchHash: %s", batchHash)
		}
		txs = batch.TxList
		localList = batch.LocalList
		missingTxsHash = nil
		return
	}

	// If we have checked this batch and found we miss some transactions,
	// just return the same missingTxsHash as before
	if missingBatch, ok := mpi.txStore.missingBatch[batchHash]; ok {
		mpi.logger.Debugf("GetRequestsByHashList failed, find batch %s in missingBatch store", batchHash)
		txs = nil
		localList = nil
		missingTxsHash = missingBatch
		return
	}

	deDuplicateMap := make(map[string]bool)
	for _, duplicateHash := range deDuplicateTxHashes {
		deDuplicateMap[duplicateHash] = true
	}

	missingTxsHash = make(map[uint64]string)
	var hasMissing bool
	for index, txHash := range hashList {
		var txPointer *txnPointer
		if txPointer, _ = mpi.txStore.txHashMap[txHash]; txPointer == nil {
			mpi.logger.Debugf("Can't find tx by hash: %s from memPool", txHash)
			missingTxsHash[uint64(index)] = txHash
			hasMissing = true
			continue
		}
		if deDuplicateMap[txHash] {
			// ignore deDuplicate txs for duplicate rule
			mpi.logger.Noticef("Ignore de-duplicate tx %s when create same batch", txHash)
		} else {
			if _, ok := mpi.txStore.batchedTxs[*txPointer]; ok {
				// If this transaction has been batched, return ErrDuplicateTx
				mpi.logger.Warningf("Duplicate transaction in getTxsByHashList with "+
					"hash: %s, batch batchHash: %s", txHash, batchHash)
				err = errors.New("duplicate transaction")
				missingTxsHash = nil
				return
			}
		}
		poolTx := mpi.txStore.getPoolTxByTxnPointer(txPointer.account, txPointer.nonce)
		if poolTx == nil {
			mpi.logger.Warningf("Transaction %s exist in txHashMap but not in allTxs", txHash)
			missingTxsHash[uint64(index)] = txHash
			hasMissing = true
			continue
		}

		if !hasMissing {
			// todo(lrx): modify []byte to *T
			txData, err := Constraint(poolTx.rawTx).RbftMarshal()
			if err != nil {
				mpi.logger.Errorf("Transaction %s marshal failed: %s", txHash, err)
				return nil, nil, nil, err
			}
			txs = append(txs, txData)
			localList = append(localList, poolTx.local)
		}
	}

	if len(missingTxsHash) != 0 {
		txs = nil
		localList = nil
		mpi.txStore.missingBatch[batchHash] = missingTxsHash
		return
	}
	for _, txHash := range hashList {
		txPointer, _ := mpi.txStore.txHashMap[txHash]
		mpi.txStore.batchedTxs[*txPointer] = true
	}
	// store the batch to cache
	batch := &RequestHashBatch[T, Constraint]{
		BatchHash:  batchHash,
		TxList:     txs,
		TxHashList: hashList,
		LocalList:  localList,
		Timestamp:  timestamp,
	}
	mpi.txStore.batchesCache[batchHash] = batch
	if mpi.txStore.priorityNonBatchSize <= uint64(len(hashList)) {
		mpi.setPriorityNonBatchSize(0)
	} else {
		mpi.decreasePriorityNonBatchSize(uint64(len(hashList)))
	}
	missingTxsHash = nil
	mpi.logger.Debugf("Replica generate a batch, which digest is %s, and now there are %d "+
		"non-batched txs and %d batches in memPool", batchHash, mpi.txStore.priorityNonBatchSize, len(mpi.txStore.batchesCache))
	return
}

// IsPoolFull checks if txPool is full which means if number of all cached txs
// has exceeded the limited txSize.
func (mpi *mempoolImpl[T, Constraint]) IsPoolFull() bool {
	return uint64(len(mpi.txStore.txHashMap)) >= mpi.poolSize
}

func (mpi *mempoolImpl[T, Constraint]) HasPendingRequestInPool() bool {
	return mpi.txStore.priorityNonBatchSize > 0
}

func (mpi *mempoolImpl[T, Constraint]) SendMissingRequests(batchHash string, missingHashList map[uint64]string) (
	txs map[uint64][]byte, err error) {

	for _, txHash := range missingHashList {
		if txPointer, _ := mpi.txStore.txHashMap[txHash]; txPointer == nil {
			return nil, fmt.Errorf("transaction %s doesn't exist in txHashMap", txHash)
		}
	}
	var targetBatch *RequestHashBatch[T, Constraint]
	var ok bool
	if targetBatch, ok = mpi.txStore.batchesCache[batchHash]; !ok {
		return nil, fmt.Errorf("batch %s doesn't exist in batchedCache", batchHash)
	}
	targetBatchLen := uint64(len(targetBatch.TxList))
	txs = make(map[uint64][]byte)
	for index, txHash := range missingHashList {
		if index >= targetBatchLen || targetBatch.TxHashList[index] != txHash {
			return nil, fmt.Errorf("find invalid transaction, index: %d, targetHash: %s", index, txHash)
		}
		txs[index] = targetBatch.TxList[index]
	}
	return
}

func (mpi *mempoolImpl[T, Constraint]) ReceiveMissingRequests(batchHash string, txs map[uint64][]byte) error {
	mpi.logger.Debugf("Replica received %d missingTxs, batch hash: %s", len(txs), batchHash)
	if _, ok := mpi.txStore.missingBatch[batchHash]; !ok {
		mpi.logger.Debugf("Can't find batch %s from missingBatch", batchHash)
		return nil
	}
	expectLen := len(mpi.txStore.missingBatch[batchHash])
	if len(txs) != expectLen {
		return fmt.Errorf("receive unmatched fetching txn response, expect "+
			"length: %d, received length: %d", expectLen, len(txs))
	}
	validTxn := make([][]byte, 0)
	targetBatch := mpi.txStore.missingBatch[batchHash]
	for index, txData := range txs {
		txHash, err := consensus.GetTxHash[T, Constraint](txData)
		if err != nil {
			return fmt.Errorf("get hash of tx %d failed: %s", index, err)
		}
		if txHash != targetBatch[index] {
			return errors.New("find a hash mismatch tx")
		}
		validTxn = append(validTxn, txData)
	}
	mpi.AddNewRequests(validTxn, false, false, true)
	delete(mpi.txStore.missingBatch, batchHash)
	return nil
}

// ReConstructBatchByOrder reconstruct batch from empty txPool by order, must be called after RestorePool.
func (mpi *mempoolImpl[T, Constraint]) ReConstructBatchByOrder(oldBatch *RequestHashBatch[T, Constraint]) (
	deDuplicateTxHashes []string, err error) {

	// check if there exists duplicate batch hash.
	if _, ok := mpi.txStore.batchesCache[oldBatch.BatchHash]; ok {
		mpi.logger.Warningf("When re-construct batch, batch %s already exists", oldBatch.BatchHash)
		err = errors.New("invalid batch: batch already exists")
		return
	}

	// TxHashList has to match TxList by length and content
	if len(oldBatch.TxHashList) != len(oldBatch.TxList) {
		mpi.logger.Warningf("Batch is invalid because TxHashList and TxList have different lengths.")
		err = errors.New("invalid batch: TxHashList and TxList have different lengths")
		return
	}

	for i, tx := range oldBatch.TxList {
		txHash, err1 := consensus.GetTxHash[T, Constraint](tx)
		if err1 != nil {
			mpi.logger.Warningf("Batch is invalid because the hash of the %dth transaction cannot be calculated.", i)
			err = errors.New("invalid batch: hash of transaction cannot be calculated")
			return
		}
		if txHash != oldBatch.TxHashList[i] {
			mpi.logger.Warningf("Batch is invalid because the hash %s in txHashList does not match "+
				"the calculated hash %s of the corresponding transaction.", oldBatch.TxHashList[i], txHash)
			err = errors.New("invalid batch: hash of transaction does not match")
			return
		}
	}

	localList := make([]bool, len(oldBatch.TxHashList))
	for range oldBatch.TxHashList {
		localList = append(localList, false)
	}

	batch := &RequestHashBatch[T, Constraint]{
		TxHashList: oldBatch.TxHashList,
		TxList:     oldBatch.TxList,
		LocalList:  localList,
		Timestamp:  oldBatch.Timestamp,
	}
	// The given batch hash should match with the calculated batch hash.
	batch.BatchHash = getBatchHash[T, Constraint](batch)
	if batch.BatchHash != oldBatch.BatchHash {
		mpi.logger.Warningf("The given batch hash %s does not match with the "+
			"calculated batch hash %s.", oldBatch.BatchHash, batch.BatchHash)
		err = errors.New("invalid batch: batch hash does not match")
		return
	}

	// There may be some duplicate transactions which are batched in different batches during vc, for those txs,
	// we only accept them in the first batch containing them and de-duplicate them in following batches.
	for _, rawTx := range oldBatch.TxList {
		tx, err1 := consensus.DecodeTx[T, Constraint](rawTx)
		if err1 != nil {
			mpi.logger.Warningf("Batch is invalid because the transaction cannot be decoded.")
			err = errors.New("invalid batch: transaction cannot be decoded")
			return
		}
		ptr := &txnPointer{
			account: Constraint(tx).RbftGetFrom(),
			nonce:   Constraint(tx).RbftGetNonce(),
		}
		txHash := Constraint(tx).RbftGetTxHash()
		if _, ok := mpi.txStore.batchedTxs[*ptr]; ok {
			mpi.logger.Noticef("De-duplicate tx %s when re-construct batch by order", txHash)
			deDuplicateTxHashes = append(deDuplicateTxHashes, txHash)
		} else {
			mpi.txStore.batchedTxs[*ptr] = true
		}
	}
	mpi.logger.Debugf("ReConstructBatchByOrder batch %s into batchedCache", oldBatch.BatchHash)
	mpi.txStore.batchesCache[batch.BatchHash] = batch
	return
}

// RestorePool move all batched txs back to non-batched tx which should
// only be used after abnormal recovery.
func (mpi *mempoolImpl[T, Constraint]) RestorePool() {
	mpi.logger.Debugf("Before restore pool, there are %d non-batched txs, %d batches, "+
		"priority len: %d, parkingLot len: %d, batchedTx len: %d, txHashMap len: %d", mpi.txStore.priorityNonBatchSize,
		len(mpi.txStore.batchesCache), mpi.txStore.priorityIndex.size(), mpi.txStore.parkingLotIndex.size(),
		len(mpi.txStore.batchedTxs), len(mpi.txStore.txHashMap))

	// if the batches are fetched from primary, those txs will be put back to non-batched cache.
	// rollback txs currentSeq
	var (
		account string
		nonce   uint64
		err     error
	)
	// record the smallest nonce of each account
	rollbackTxsNonce := make(map[string]uint64)
	for _, batch := range mpi.txStore.batchesCache {
		for _, tx := range batch.TxList {
			account, err = consensus.GetAccount[T, Constraint](tx)
			if err != nil {
				mpi.logger.Warningf("Get tx account error when restore pool")
				continue
			}
			nonce, err = consensus.GetNonce[T, Constraint](tx)
			if err != nil {
				mpi.logger.Warningf("Get tx nonce error when restore pool")
				continue
			}
			if _, ok := rollbackTxsNonce[account]; !ok {
				rollbackTxsNonce[account] = nonce
			} else if nonce < rollbackTxsNonce[account] {
				// if the nonce is smaller than the currentSeq, update it
				rollbackTxsNonce[account] = nonce
			}
		}
		// increase priorityNonBatchSize
		mpi.increasePriorityNonBatchSize(uint64(len(batch.TxList)))
	}
	// rollback nonce for each account
	for acc, non := range rollbackTxsNonce {
		mpi.txStore.nonceCache.setPendingNonce(acc, non)
	}

	//resubmit these txs to mempool.
	for batchDigest, batch := range mpi.txStore.batchesCache {
		mpi.logger.Debugf("Put batch %s back to pool with %d txs", batchDigest, len(batch.TxList))
		mpi.AddNewRequests(batch.TxList, false, false, true)
		delete(mpi.txStore.batchesCache, batchDigest)
	}

	// clear missingTxs after abnormal.
	mpi.txStore.missingBatch = make(map[string]map[uint64]string)
	mpi.txStore.batchedTxs = make(map[txnPointer]bool)
	mpi.logger.Debugf("After restore pool, there are %d non-batched txs, %d batches, "+
		"priority len: %d, parkingLot len: %d, batchedTx len: %d, txHashMap len: %d", mpi.txStore.priorityNonBatchSize,
		len(mpi.txStore.batchesCache), mpi.txStore.priorityIndex.size(), mpi.txStore.parkingLotIndex.size(),
		len(mpi.txStore.batchedTxs), len(mpi.txStore.txHashMap))
}

// FilterOutOfDateRequests get the remained local txs in TTLIndex and broadcast to other vp peers by tolerance time.
func (mpi *mempoolImpl[T, Constraint]) FilterOutOfDateRequests() ([][]byte, error) {
	now := time.Now().Unix()
	var forward []*mempoolTransaction[T, Constraint]
	mpi.txStore.localTTLIndex.data.Ascend(func(a btree.Item) bool {
		orderedKey := a.(*orderedIndexKey)
		poolTx := mpi.txStore.getPoolTxByTxnPointer(orderedKey.account, orderedKey.nonce)
		if poolTx == nil {
			mpi.logger.Error("Get nil poolTx from txStore")
			return true
		}
		if float64(now-poolTx.lifeTime) > mpi.toleranceTime.Seconds() {
			// for those batched txs, we don't need to forward temporarily.
			if _, ok := mpi.txStore.batchedTxs[txnPointer{orderedKey.account, orderedKey.nonce}]; !ok {
				forward = append(forward, poolTx)
			}
		} else {
			return false
		}
		return true
	})
	result := make([][]byte, len(forward))
	// update pool tx's timestamp to now for next forwarding check.
	for i, poolTx := range forward {
		// update localTTLIndex
		mpi.txStore.localTTLIndex.updateTTLIndex(poolTx.lifeTime, poolTx.getAccount(), poolTx.getNonce(), now)

		// todo(lrx): modify []byte to *T
		txData, err := Constraint(poolTx.rawTx).RbftMarshal()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tx: %v", err)
		}
		result[i] = txData
		// update mempool tx in allTxs
		poolTx.lifeTime = now
	}
	return result, nil
}

// RemoveTimeoutRequests remove the remained local txs in timeoutIndex and removeTxs in memPool by tolerance time.
func (mpi *mempoolImpl[T, Constraint]) RemoveTimeoutRequests() (uint64, error) {
	now := time.Now().Unix()
	removedTxs := make(map[string][]*mempoolTransaction[T, Constraint])
	var count uint64
	var index int
	mpi.txStore.removeTTLIndex.data.Ascend(func(a btree.Item) bool {
		index++
		orderedKey := a.(*orderedIndexKey)
		poolTx := mpi.txStore.getPoolTxByTxnPointer(orderedKey.account, orderedKey.nonce)
		if poolTx == nil {
			mpi.logger.Errorf("Get nil poolTx from txStore:[account:%s, nonce:%d]", orderedKey.account, orderedKey.nonce)
			return true
		}
		if float64(now-poolTx.arrivedTime) > mpi.toleranceRemoveTime.Seconds() {
			mpi.logger.Debugf("handle orderedKey[account:%s, nonce:%d]", poolTx.getAccount(), poolTx.getNonce())
			// for those batched txs, we don't need to removedTxs temporarily.
			if _, ok := mpi.txStore.batchedTxs[txnPointer{orderedKey.account, orderedKey.nonce}]; ok {
				mpi.logger.Debugf("find tx[account: %s, nonce:%d] from batchedTxs, ignore remove request",
					orderedKey.account, orderedKey.nonce)
				return true
			}

			if tx := mpi.txStore.priorityIndex.data.Get(orderedKey); tx != nil {
				mpi.logger.Debugf("find tx[account: %s, nonce:%d] from priorityIndex, ignore remove request",
					orderedKey.account, orderedKey.nonce)
				return true
			}

			if tx := mpi.txStore.parkingLotIndex.data.Get(orderedKey); tx != nil {
				if _, ok := removedTxs[orderedKey.account]; !ok {
					removedTxs[orderedKey.account] = make([]*mempoolTransaction[T, Constraint], 0)
				}
				// record need removedTxs and the count
				removedTxs[orderedKey.account] = append(removedTxs[orderedKey.account], poolTx)
				count++
				// remove txHashMap
				txHash := poolTx.getHash()
				delete(mpi.txStore.txHashMap, txHash)
				return true
			}
		}
		return true
	})
	for account, txs := range removedTxs {
		if list, ok := mpi.txStore.allTxs[account]; ok {
			// remove index from removedTxs
			var wg sync.WaitGroup
			wg.Add(5)
			go func(ready map[string][]*mempoolTransaction[T, Constraint]) {
				defer wg.Done()
				list.index.removeBySortedNonceKeys(ready)
				for _, tx := range txs {
					delete(list.items, tx.getNonce())
				}
			}(removedTxs)
			go func(ready map[string][]*mempoolTransaction[T, Constraint]) {
				defer wg.Done()
				mpi.txStore.priorityIndex.removeByOrderedQueueKeys(ready)
			}(removedTxs)
			go func(ready map[string][]*mempoolTransaction[T, Constraint]) {
				defer wg.Done()
				mpi.txStore.parkingLotIndex.removeByOrderedQueueKeys(ready)
			}(removedTxs)
			go func(ready map[string][]*mempoolTransaction[T, Constraint]) {
				defer wg.Done()
				mpi.txStore.localTTLIndex.removeByTTLIndexByKeys(ready, Rebroadcast)
			}(removedTxs)
			go func(ready map[string][]*mempoolTransaction[T, Constraint]) {
				defer wg.Done()
				mpi.txStore.removeTTLIndex.removeByTTLIndexByKeys(ready, Remove)
			}(removedTxs)
			wg.Wait()
		}
	}
	return count, nil
}

// CheckRequestsExist check if the txs are in pool and mark them as batched if they are.
func (mpi *mempoolImpl[T, Constraint]) CheckRequestsExist(hashList []string) {
	for _, txHash := range hashList {
		if txnPointer, ok := mpi.txStore.txHashMap[txHash]; ok {
			mpi.txStore.batchedTxs[*txnPointer] = true
			continue
		}
		mpi.logger.Debugf("Can't find tx %s from txHashMap, but it exist in rbft batchStore, maybe has reset the pool", txHash)
	}
}

// =============================================================================
// internal methods
// =============================================================================
func (mpi *mempoolImpl[T, Constraint]) processDirtyAccount(dirtyAccounts map[string]bool) {
	for account := range dirtyAccounts {
		if list, ok := mpi.txStore.allTxs[account]; ok {
			// search for related sequential txs in allTxs
			// and add these txs into priorityIndex and parkingLotIndex.
			pendingNonce := mpi.txStore.nonceCache.getPendingNonce(account)
			readyTxs, nonReadyTxs, nextDemandNonce := list.filterReady(pendingNonce)
			mpi.txStore.nonceCache.setPendingNonce(account, nextDemandNonce)

			// insert ready txs into priorityIndex
			for _, poolTx := range readyTxs {
				mpi.txStore.priorityIndex.insertByOrderedQueueKey(poolTx.getRawTimestamp(), poolTx.getAccount(), poolTx.getNonce())
				//mpi.logger.Debugf("insert ready tx[account: %s, nonce: %d] into priorityIndex", poolTx.getAccount(), poolTx.getNonce())
			}
			mpi.increasePriorityNonBatchSize(uint64(len(readyTxs)))

			// insert non-ready txs into parkingLotIndex
			for _, poolTx := range nonReadyTxs {
				mpi.txStore.parkingLotIndex.insertByOrderedQueueKey(poolTx.getRawTimestamp(), poolTx.getAccount(), poolTx.getNonce())
			}
		}
	}
}

// generateRequestBatch fetches next block of transactions for consensus,
// batchedTx are all txs sent to consensus but were not committed yet, mempool should filter out such txs.
func (mpi *mempoolImpl[T, Constraint]) generateRequestBatch() ([]*RequestHashBatch[T, Constraint], error) {

	if !mpi.isTimed && !mpi.HasPendingRequestInPool() {
		mpi.logger.Debug("Mempool is empty")
		return nil, nil
	}
	result := make([]txnPointer, 0, mpi.batchSize)
	// txs has lower nonce will be observed first in priority index iterator.
	mpi.logger.Debugf("Length of non-batched transactions: %d", mpi.txStore.priorityNonBatchSize)
	var batchSize uint64
	if mpi.txStore.priorityNonBatchSize > mpi.batchSize {
		batchSize = mpi.batchSize
	} else {
		batchSize = mpi.txStore.priorityNonBatchSize
	}
	skippedTxs := make(map[txnPointer]bool)
	mpi.txStore.priorityIndex.data.Ascend(func(a btree.Item) bool {
		tx := a.(*orderedIndexKey)
		// if tx has existed in bathedTxs
		// TODO (YH): refactor batchedTxs to seen (all the transactions that have been executed) to track all txs batched in ledger.
		if _, ok := mpi.txStore.batchedTxs[txnPointer{tx.account, tx.nonce}]; ok {
			return true
		}
		txSeq := tx.nonce
		//mpi.logger.Debugf("memPool txNonce:%s-%d", tx.account, tx.nonce)
		commitNonce := mpi.txStore.nonceCache.getCommitNonce(tx.account)
		//mpi.logger.Debugf("ledger txNonce:%s-%d", tx.account, commitNonce)
		var seenPrevious bool
		if txSeq >= 1 {
			_, seenPrevious = mpi.txStore.batchedTxs[txnPointer{account: tx.account, nonce: txSeq - 1}]
		}
		// include transaction if it's "next" for given account or
		// we've already sent its ancestor to Consensus
		ptr := txnPointer{account: tx.account, nonce: tx.nonce}
		// commitNonce is the nonce of last committed tx for given account,
		// todo(lrx): not sure if txSeq == commitNonce is correct, maybe txSeq == commitNonce+1 is correct
		if seenPrevious || (txSeq == commitNonce) {
			mpi.txStore.batchedTxs[ptr] = true
			result = append(result, ptr)
			if uint64(len(result)) == batchSize {
				return false
			}

			// check if we can now include some txs that were skipped before for given account
			skippedTxn := txnPointer{account: tx.account, nonce: tx.nonce + 1}
			for {
				if _, ok := skippedTxs[skippedTxn]; !ok {
					break
				}
				mpi.txStore.batchedTxs[skippedTxn] = true
				result = append(result, skippedTxn)
				if uint64(len(result)) == batchSize {
					return false
				}
				skippedTxn.nonce++
			}
		} else {
			skippedTxs[ptr] = true
		}
		return true
	})

	if !mpi.isTimed && len(result) == 0 && mpi.HasPendingRequestInPool() {
		for account := range mpi.txStore.nonceCache.pendingNonces {
			nonce := mpi.txStore.nonceCache.getPendingNonce(account)
			mpi.logger.Errorf("PriorityNonBatchSize: %d,===== account:%s,  ==== pendingNonce: %d",
				mpi.txStore.priorityNonBatchSize, account, nonce)
		}
		mpi.setPriorityNonBatchSize(0)
		return nil, errors.New("===== Note!!! Primary generate a batch with 0 txs")
	}

	batches := make([]*RequestHashBatch[T, Constraint], 0)
	// convert transaction pointers to real values
	hashList := make([]string, len(result))
	localList := make([]bool, len(result))
	txList := make([][]byte, len(result))
	for i, v := range result {
		poolTx := mpi.txStore.getPoolTxByTxnPointer(v.account, v.nonce)
		if poolTx == nil {
			return nil, errors.New("get nil poolTx from txStore")
		}
		hashList[i] = poolTx.getHash()

		// todo(lrx): modify []byte to *T
		txData, err := Constraint(poolTx.rawTx).RbftMarshal()
		if err != nil {
			return nil, fmt.Errorf("marshal tx error: %v", err)
		}
		txList[i] = txData
		localList[i] = poolTx.local
	}

	txBatch := &RequestHashBatch[T, Constraint]{
		TxHashList: hashList,
		TxList:     txList,
		LocalList:  localList,
		Timestamp:  time.Now().Unix(),
	}
	batchHash := getBatchHash[T, Constraint](txBatch)
	txBatch.BatchHash = batchHash
	mpi.txStore.batchesCache[batchHash] = txBatch
	if mpi.txStore.priorityNonBatchSize <= uint64(len(hashList)) {
		mpi.setPriorityNonBatchSize(0)
	} else {
		mpi.decreasePriorityNonBatchSize(uint64(len(hashList)))
	}
	batches = append(batches, txBatch)
	mpi.logger.Debugf("Primary generate a batch with %d txs, which hash is %s, and now there are %d "+
		"pending txs and %d batches in txPool", len(hashList), batchHash, mpi.txStore.priorityNonBatchSize, len(mpi.txStore.batchesCache))
	return batches, nil
}

func (mpi *mempoolImpl[T, Constraint]) replaceTx(tx *T) {
	account := Constraint(tx).RbftGetFrom()
	txHash := Constraint(tx).RbftGetTxHash()
	txNonce := Constraint(tx).RbftGetNonce()
	oldPoolTx := mpi.txStore.getPoolTxByTxnPointer(account, txNonce)
	txPointer := &txnPointer{
		account: account,
		nonce:   txNonce,
	}
	mpi.txStore.txHashMap[txHash] = txPointer
	list, ok := mpi.txStore.allTxs[account]
	if !ok {
		list = newTxSortedMap[T, Constraint]()
		mpi.txStore.allTxs[account] = list
	}
	list.index.insertBySortedNonceKey(txNonce)
	now := time.Now().Unix()
	newPoolTx := &mempoolTransaction[T, Constraint]{
		local:       false,
		rawTx:       tx,
		lifeTime:    Constraint(tx).RbftGetTimeStamp(),
		arrivedTime: now,
	}
	list.items[txNonce] = newPoolTx

	// remove old tx from txHashMap、priorityIndex、parkingLotIndex、localTTLIndex and removeTTLIndex
	if oldPoolTx != nil {
		delete(mpi.txStore.txHashMap, oldPoolTx.getHash())
		mpi.txStore.priorityIndex.removeByOrderedQueueKey(oldPoolTx.getRawTimestamp(), oldPoolTx.getAccount(), oldPoolTx.getNonce())
		mpi.txStore.parkingLotIndex.removeByOrderedQueueKey(oldPoolTx.getRawTimestamp(), oldPoolTx.getAccount(), oldPoolTx.getNonce())
		mpi.txStore.localTTLIndex.removeByTTLIndexByKey(oldPoolTx, Rebroadcast)
		mpi.txStore.removeTTLIndex.removeByTTLIndexByKey(oldPoolTx, Remove)
	}
	// insert new tx received from remote vp
	mpi.txStore.priorityIndex.insertByOrderedQueueKey(newPoolTx.getRawTimestamp(), newPoolTx.getAccount(), newPoolTx.getNonce())
}

// getBatchHash calculate hash of a RequestHashBatch
func getBatchHash[T any, Constraint consensus.TXConstraint[T]](batch *RequestHashBatch[T, Constraint]) string {
	h := md5.New()
	for _, hash := range batch.TxHashList {
		_, _ = h.Write([]byte(hash))
	}
	if batch.Timestamp > 0 {
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, uint64(batch.Timestamp))
		_, _ = h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (mpi *mempoolImpl[T, Constraint]) increasePriorityNonBatchSize(addSize uint64) {
	mpi.txStore.priorityNonBatchSize = mpi.txStore.priorityNonBatchSize + addSize
}

func (mpi *mempoolImpl[T, Constraint]) decreasePriorityNonBatchSize(subSize uint64) {
	mpi.txStore.priorityNonBatchSize = mpi.txStore.priorityNonBatchSize - subSize
}

func (mpi *mempoolImpl[T, Constraint]) setPriorityNonBatchSize(txnSize uint64) {
	mpi.txStore.priorityNonBatchSize = txnSize
}
