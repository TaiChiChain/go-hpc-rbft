package txpool

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/btree"
	"github.com/pkg/errors"
	"github.com/samber/lo"

	"github.com/axiomesh/axiom-bft/common"
	"github.com/axiomesh/axiom-bft/common/consensus"
)

// txPoolImpl contains all currently known transactions.
type txPoolImpl[T any, Constraint consensus.TXConstraint[T]] struct {
	logger              common.Logger
	selfID              uint64
	batchSize           uint64
	isTimed             bool
	txStore             *transactionStore[T, Constraint] // store all transaction info
	toleranceNonceGap   uint64
	toleranceTime       time.Duration
	toleranceRemoveTime time.Duration
	poolSize            uint64
	getAccountNonce     GetAccountNonceFunc
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
func (p *txPoolImpl[T, Constraint]) AddNewRequests(txs []*T, isPrimary, local, isReplace bool, needGenerateBatch bool) ([]*RequestHashBatch[T, Constraint], []string) {
	return p.addNewRequests(txs, isPrimary, local, isReplace, needGenerateBatch, true)
}

func (p *txPoolImpl[T, Constraint]) addNewRequests(txs []*T, isPrimary, local, isReplace bool, needGenerateBatch bool, needCompletionMissingBatch bool) ([]*RequestHashBatch[T, Constraint], []string) {
	completionMissingBatchHashes := make([]string, 0)
	validTxs := make(map[string][]*internalTransaction[T, Constraint])

	// key: missing batch hash
	// val: txHash -> bool
	matchMissingTxBatch := make(map[string]map[string]struct{})

	currentSeqNoList := make(map[string]uint64)
	duplicateTxHash := make(map[string]bool)
	duplicatePointer := make(map[txPointer]string)
	for _, tx := range txs {
		txAccount := Constraint(tx).RbftGetFrom()
		txHash := Constraint(tx).RbftGetTxHash()
		txNonce := Constraint(tx).RbftGetNonce()

		// currentSeqNo means the current wanted nonce of the account
		currentSeqNo, ok := currentSeqNoList[txAccount]
		if !ok {
			currentSeqNo = p.txStore.nonceCache.getPendingNonce(txAccount)
			currentSeqNoList[txAccount] = currentSeqNo
		}

		var ready bool
		if txNonce < currentSeqNo {
			ready = true
			if !isReplace {
				// just log tx warning message from api
				if local {
					p.logger.Warningf("Receive transaction [account: %s, nonce: %d, hash: %s], but we required %d", txAccount, txNonce, txHash, currentSeqNo)
				}
				continue
			} else {
				p.logger.Warningf("Receive transaction [account: %s, nonce: %d, hash: %s], but we required %d,"+
					" will replace old tx", txAccount, txNonce, txHash, currentSeqNo)
				p.replaceTx(tx, local, ready)
				continue
			}
		}

		// reject nonce too high tx from api
		if txNonce > currentSeqNo+p.toleranceNonceGap && local {
			p.logger.Warningf("Receive transaction [account: %s, nonce: %d, hash: %s], but we required %d,"+
				" and the nonce gap is %d, reject it", txAccount, txNonce, txHash, currentSeqNo, p.toleranceNonceGap)
			continue
		}

		// ignore duplicate tx
		if pointer := p.txStore.txHashMap[txHash]; pointer != nil || duplicateTxHash[txHash] {
			p.logger.Warningf("Transaction [account: %s, nonce: %d, hash: %s] has already existed in txHashMap, "+
				"ignore it", txAccount, txNonce, txHash)
			continue
		}

		// check duplicate account and nonce(but different txHash), and replace old tx
		if oldTxhash, ok := duplicatePointer[txPointer{account: txAccount, nonce: txNonce}]; ok {
			p.logger.Warningf("Receive duplicate nonce transaction [account: %s, nonce: %d, hash: %s],"+
				" will replace old tx[hash: %s]", txAccount, txNonce, txHash, oldTxhash)
			txItems := validTxs[txAccount]
			// remove old tx from txItems
			newItems := lo.Filter(txItems, func(txItem *internalTransaction[T, Constraint], index int) bool {
				return txItem.getNonce() != txNonce
			})
			validTxs[txAccount] = newItems
		}

		if p.txStore.allTxs[txAccount] != nil {
			if oldTx, ok := p.txStore.allTxs[txAccount].items[txNonce]; ok {
				p.logger.Warningf("Receive duplicate nonce transaction [account: %s, nonce: %d, hash: %s],"+
					" will replace old tx[hash: %s]", txAccount, txNonce, txHash, oldTx.getHash())
				p.replaceTx(tx, local, ready)
			}
		}

		// update currentSeqNoList
		if txNonce == currentSeqNo {
			currentSeqNoList[txAccount]++
		}

		_, ok = validTxs[txAccount]
		if !ok {
			validTxs[txAccount] = make([]*internalTransaction[T, Constraint], 0)
		}
		now := time.Now().UnixNano()
		txItem := &internalTransaction[T, Constraint]{
			rawTx:       tx,
			local:       local,
			lifeTime:    Constraint(tx).RbftGetTimeStamp(),
			arrivedTime: now,
		}
		validTxs[txAccount] = append(validTxs[txAccount], txItem)
		duplicateTxHash[txHash] = true
		duplicatePointer[txPointer{account: txAccount, nonce: txNonce}] = txHash

		// for replica, add validTxs to nonBatchedTxs and check if the missing transactions and batches are fetched
		if !isPrimary && needCompletionMissingBatch {
			for batchHash, missingTxsHash := range p.txStore.missingBatch {
				if _, ok := matchMissingTxBatch[batchHash]; !ok {
					matchMissingTxBatch[batchHash] = make(map[string]struct{}, 0)
				}
				for _, missingTxHash := range missingTxsHash {
					// we have received a tx which we are fetching.
					if txHash == missingTxHash {
						matchMissingTxBatch[batchHash][txHash] = struct{}{}
					}
				}

				// we receive all missing txs
				if len(missingTxsHash) == len(matchMissingTxBatch[batchHash]) {
					delete(p.txStore.missingBatch, batchHash)
					completionMissingBatchHashes = append(completionMissingBatchHashes, batchHash)
				}
			}
		}
	}

	// Process all the new transaction and merge any errors into the original slice
	dirtyAccounts := p.txStore.insertTxs(validTxs, local)
	// send tx to txpool store
	p.processDirtyAccount(dirtyAccounts)

	// if no timedBlock, generator batch by block size
	if isPrimary && needGenerateBatch {
		if p.txStore.priorityNonBatchSize >= p.batchSize {
			batches, err := p.generateRequestBatch()
			if err != nil {
				p.logger.Errorf("Generate batch failed, err: %s", err.Error())
				return nil, nil
			}
			return batches, nil
		}
	}

	return nil, completionMissingBatchHashes
}

// GetUncommittedTransactions returns the uncommitted transactions.
// not used
func (p *txPoolImpl[T, Constraint]) GetUncommittedTransactions(maxsize uint64) []*T {
	return []*T{}
}

// Start start metrics
// TODO(lrx): add metrics
func (p *txPoolImpl[T, Constraint]) Start() error {
	return nil
}

// Stop stop metrics
// TODO(lrx): add metrics
func (p *txPoolImpl[T, Constraint]) Stop() {}

// newTxPoolImpl returns the txpool instance.
func newTxPoolImpl[T any, Constraint consensus.TXConstraint[T]](config Config) *txPoolImpl[T, Constraint] {
	mpi := &txPoolImpl[T, Constraint]{
		logger:          config.Logger,
		getAccountNonce: config.GetAccountNonce,
		isTimed:         config.IsTimed,
	}
	mpi.txStore = newTransactionStore[T, Constraint](config.GetAccountNonce, config.Logger)
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
	if config.ToleranceNonceGap == 0 {
		mpi.toleranceNonceGap = DefaultToleranceNonceGap
	} else {
		mpi.toleranceNonceGap = config.ToleranceNonceGap
	}
	mpi.logger.Infof("TxPool pool size = %d", mpi.poolSize)
	mpi.logger.Infof("TxPool batch size = %d", mpi.batchSize)
	mpi.logger.Infof("TxPool batch mem limit = %v", config.BatchMemLimit)
	mpi.logger.Infof("TxPool batch max mem size = %d", config.BatchMaxMem)
	mpi.logger.Infof("TxPool tolerance time = %v", config.ToleranceTime)
	mpi.logger.Infof("TxPool tolerance remove time = %v", mpi.toleranceRemoveTime)
	mpi.logger.Infof("TxPool tolerance nonce gap = %d", mpi.toleranceNonceGap)
	return mpi
}

func (p *txPoolImpl[T, Constraint]) Init(selfID uint64) error {
	p.selfID = selfID
	return nil
}

// GenerateRequestBatch generates a transaction batch and post it
// to outside if there are transactions in txPool.
func (p *txPoolImpl[T, Constraint]) GenerateRequestBatch() []*RequestHashBatch[T, Constraint] {
	batches, err := p.generateRequestBatch()
	if err != nil {
		p.logger.Errorf("Generator batch failed, err: %s", err.Error())
		return nil
	}
	return batches
}

// Reset clears all cached txs in txPool and start with a pure empty environment, only used in abnormal status.
// except batches in saveBatches and local non-batched-txs that not included in ledger.
// todo(lrx): except saveBatches
func (p *txPoolImpl[T, Constraint]) Reset(saveBatches []string) {
	p.logger.Info("Reset txpool...")
	oldBatchCache := p.txStore.batchesCache
	// 1. reset txStore in txPool
	p.txStore = newTransactionStore[T, Constraint](p.getAccountNonce, p.logger)

	// 2. clean txPool metrics
	resetTxPoolMetrics()

	lo.ForEach(saveBatches, func(exceptBatch string, index int) {
		if oldBatch, ok := oldBatchCache[exceptBatch]; ok {
			p.logger.Debugf("Reset txpool, batch %s is in saveBatches, ignore it", oldBatch.BatchHash)
			err := p.saveOldBatchedTxs(oldBatch)
			if err != nil {
				p.logger.Errorf("Reset txpool, save old batched txs failed, err: %s", err.Error())
			}
		}
	})
}

// saveOldBatchedTxs re-put batch txs into txPool, only used in state updated status.
func (p *txPoolImpl[T, Constraint]) saveOldBatchedTxs(batch *RequestHashBatch[T, Constraint]) error {
	// check if the batch is valid
	if len(batch.TxList) != len(batch.TxHashList) {
		return fmt.Errorf("batch[batchHash:%s] txList length %d is not equal to txHashList length %d",
			batch.BatchHash, len(batch.TxList), len(batch.TxHashList))
	}

	insertTxs := make(map[string][]*internalTransaction[T, Constraint])
	for i, tx := range batch.TxList {
		// 2. insert batched txs into batchedTxs and batchesCache
		pointer := &txPointer{
			account: Constraint(tx).RbftGetFrom(),
			nonce:   Constraint(tx).RbftGetNonce(),
		}
		p.txStore.batchedTxs[*pointer] = true
		p.txStore.batchesCache[batch.BatchHash] = batch

		// 3. insert batched txs into txHashMap、allTxs、removeTTLIndex、localTTLIndex
		txAccount := Constraint(tx).RbftGetFrom()
		if len(batch.LocalList) != len(batch.TxList) {
			return fmt.Errorf("batch localList length %d is not equal to txList length %d", len(batch.LocalList), len(batch.TxList))
		}

		_, ok := insertTxs[txAccount]
		if !ok {
			insertTxs[txAccount] = make([]*internalTransaction[T, Constraint], 0)
		}
		now := time.Now().UnixNano()
		txItem := &internalTransaction[T, Constraint]{
			rawTx:       tx,
			local:       batch.LocalList[i],
			lifeTime:    Constraint(tx).RbftGetTimeStamp(),
			arrivedTime: now,
		}
		insertTxs[txAccount] = append(insertTxs[txAccount], txItem)

		// 4. insert batched txs into priorityIndex
		p.txStore.priorityIndex.insertByOrderedQueueKey(txItem)
	}
	p.txStore.insertTxs(insertTxs, true)
	return nil
}

// RestoreOneBatch moves one batch from batchStore.
func (p *txPoolImpl[T, Constraint]) RestoreOneBatch(hash string) error {
	batch, ok := p.txStore.batchesCache[hash]
	if !ok {
		return errors.New("can't find batch from batchesCache")
	}

	// remove from batchedTxs and batchStore
	for _, hash := range batch.TxHashList {
		ptr := p.txStore.txHashMap[hash]
		if ptr == nil {
			return fmt.Errorf("can't find tx %s from txHashMap", hash)
		}
		// check if the given tx exist in priorityIndex
		poolTx := p.txStore.getPoolTxByTxnPointer(ptr.account, ptr.nonce)
		key := &orderedIndexKey{time: poolTx.getRawTimestamp(), account: ptr.account, nonce: ptr.nonce}
		if tx := p.txStore.priorityIndex.data.Get(key); tx == nil {
			return fmt.Errorf("can't find tx %s from priorityIndex", hash)
		}
		delete(p.txStore.batchedTxs, *ptr)
	}
	delete(p.txStore.batchesCache, hash)
	p.increasePriorityNonBatchSize(uint64(len(batch.TxHashList)))

	p.logger.Debugf("Restore one batch, which hash is %s, now there are %d non-batched txs, "+
		"%d batches in txPool", hash, p.txStore.priorityNonBatchSize, len(p.txStore.batchesCache))
	return nil
}

// RemoveBatches removes several batches by given digests of
// transaction batches from the pool(batchedTxs).
func (p *txPoolImpl[T, Constraint]) RemoveBatches(hashList []string) {
	// update current cached commit nonce for account
	p.logger.Debugf("RemoveBatches: batch len:%d", len(hashList))
	updateAccounts := make(map[string]uint64)
	for _, batchHash := range hashList {
		batch, ok := p.txStore.batchesCache[batchHash]
		if !ok {
			p.logger.Debugf("Cannot find batch %s in txpool batchedCache which may have been "+
				"discard when ReConstructBatchByOrder", batchHash)
			continue
		}
		delete(p.txStore.batchesCache, batchHash)
		dirtyAccounts := make(map[string]bool)
		for _, txHash := range batch.TxHashList {
			txPointer, ok := p.txStore.txHashMap[txHash]
			if !ok {
				p.logger.Warningf("Remove transaction %s failed, Can't find it from txHashMap", txHash)
				continue
			}
			preCommitNonce := p.txStore.nonceCache.getCommitNonce(txPointer.account)
			// next wanted nonce
			newCommitNonce := txPointer.nonce + 1
			if preCommitNonce < newCommitNonce {
				p.txStore.nonceCache.setCommitNonce(txPointer.account, newCommitNonce)
				// Note!!! updating pendingNonce to commitNonce for the restart node
				pendingNonce := p.txStore.nonceCache.getPendingNonce(txPointer.account)
				if pendingNonce < newCommitNonce {
					updateAccounts[txPointer.account] = newCommitNonce
					p.txStore.nonceCache.setPendingNonce(txPointer.account, newCommitNonce)
				}
			}
			p.txStore.deletePoolTx(txHash)
			delete(p.txStore.batchedTxs, *txPointer)
			dirtyAccounts[txPointer.account] = true
		}
		// clean related txs info in cache
		for account := range dirtyAccounts {
			if err := p.cleanTxsBeforeCommitNonce(account, p.txStore.nonceCache.getCommitNonce(account)); err != nil {
				p.logger.Errorf("cleanTxsBeforeCommitNonce error: %v", err)
			}
		}
	}
	readyNum := uint64(p.txStore.priorityIndex.size())
	// set priorityNonBatchSize to min(nonBatchedTxs, readyNum),
	if p.txStore.priorityNonBatchSize > readyNum {
		p.logger.Debugf("Set priorityNonBatchSize from %d to the length of priorityIndex %d", p.txStore.priorityNonBatchSize, readyNum)
		p.setPriorityNonBatchSize(readyNum)
	}
	for account, pendingNonce := range updateAccounts {
		p.logger.Debugf("Account %s update its pendingNonce to %d by commitNonce", account, pendingNonce)
	}
	p.logger.Infof("Removes batches in txpool, and now there are %d non-batched txs, %d batches, "+
		"priority len: %d, parkingLot len: %d, batchedTx len: %d, txHashMap len: %d", p.txStore.priorityNonBatchSize,
		len(p.txStore.batchesCache), p.txStore.priorityIndex.size(), p.txStore.parkingLotIndex.size(),
		len(p.txStore.batchedTxs), len(p.txStore.txHashMap))
}

func (p *txPoolImpl[T, Constraint]) cleanTxsBeforeCommitNonce(account string, commitNonce uint64) error {
	var outErr error
	// clean related txs info in cache
	if list, ok := p.txStore.allTxs[account]; ok {
		// remove all previous seq number txs for this account.
		removedTxs := list.forward(commitNonce)
		// remove index smaller than commitNonce delete index.
		if err := p.cleanTxsByAccount(account, list, removedTxs[account]); err != nil {
			outErr = err
		}
	}
	return outErr
}

func (p *txPoolImpl[T, Constraint]) cleanTxsByAccount(account string, list *txSortedMap[T, Constraint], removedTxs []*internalTransaction[T, Constraint]) error {
	var failed atomic.Bool
	var wg sync.WaitGroup
	wg.Add(5)
	go func(txs []*internalTransaction[T, Constraint]) {
		defer wg.Done()
		if err := list.index.removeBySortedNonceKeys(account, txs); err != nil {
			p.logger.Errorf("removeBySortedNonceKeys error: %v", err)
			failed.Store(true)
		} else {
			lo.ForEach(txs, func(tx *internalTransaction[T, Constraint], _ int) {
				delete(list.items, tx.getNonce())
			})
		}
	}(removedTxs)
	go func(txs []*internalTransaction[T, Constraint]) {
		defer wg.Done()
		if err := p.txStore.priorityIndex.removeByOrderedQueueKeys(account, txs); err != nil {
			p.logger.Errorf("remove priorityIndex error: %v", err)
			failed.Store(true)
		}
	}(removedTxs)
	go func(txs []*internalTransaction[T, Constraint]) {
		defer wg.Done()
		if err := p.txStore.parkingLotIndex.removeByOrderedQueueKeys(account, txs); err != nil {
			p.logger.Errorf("remove parkingLotIndex error: %v", err)
			failed.Store(true)
		}
	}(removedTxs)
	go func(txs []*internalTransaction[T, Constraint]) {
		defer wg.Done()
		if err := p.txStore.localTTLIndex.removeByOrderedQueueKeys(account, txs); err != nil {
			p.logger.Errorf("remove localTTLIndex error: %v", err)
			failed.Store(true)
		}
	}(removedTxs)
	go func(txs []*internalTransaction[T, Constraint]) {
		defer wg.Done()
		if err := p.txStore.removeTTLIndex.removeByOrderedQueueKeys(account, txs); err != nil {
			p.logger.Errorf("remove removeTTLIndex error: %v", err)
			failed.Store(true)
		}
	}(removedTxs)
	wg.Wait()

	if failed.Load() {
		return errors.New("failed to remove txs")
	}
	return nil
}

func (p *txPoolImpl[T, Constraint]) updateNonceCache(pointer *txPointer, updateAccounts map[string]uint64) {
	preCommitNonce := p.txStore.nonceCache.getCommitNonce(pointer.account)
	// next wanted nonce
	newCommitNonce := pointer.nonce + 1
	if preCommitNonce < newCommitNonce {
		p.txStore.nonceCache.setCommitNonce(pointer.account, newCommitNonce)
		// Note!!! updating pendingNonce to commitNonce for the restart node
		pendingNonce := p.txStore.nonceCache.getPendingNonce(pointer.account)
		if pendingNonce < newCommitNonce {
			updateAccounts[pointer.account] = newCommitNonce
			p.txStore.nonceCache.setPendingNonce(pointer.account, newCommitNonce)
		}
	}
}

func (p *txPoolImpl[T, Constraint]) RemoveStateUpdatingTxs(txHashList []string) {
	p.logger.Infof("start RemoveStateUpdatingTxs, len:%d", len(txHashList))
	removeCount := 0
	dirtyAccounts := make(map[string]bool)
	updateAccounts := make(map[string]uint64)
	removeTxs := make(map[string][]*internalTransaction[T, Constraint])
	lo.ForEach(txHashList, func(txHash string, _ int) {
		if pointer, ok := p.txStore.txHashMap[txHash]; ok {
			poolTx := p.txStore.getPoolTxByTxnPointer(pointer.account, pointer.nonce)
			if poolTx == nil {
				p.logger.Errorf("pool tx %s not found in txpool, but exists in txHashMap", txHash)
				return
			}
			// update nonce when remove tx
			p.updateNonceCache(pointer, updateAccounts)
			// remove from txHashMap
			p.txStore.deletePoolTx(txHash)
			if removeTxs[pointer.account] == nil {
				removeTxs[pointer.account] = make([]*internalTransaction[T, Constraint], 0)
			}
			// record dirty accounts and removeTxs
			removeTxs[pointer.account] = append(removeTxs[pointer.account], poolTx)
			dirtyAccounts[pointer.account] = true
		}
	})

	for account := range dirtyAccounts {
		if list, ok := p.txStore.allTxs[account]; ok {
			if err := p.cleanTxsByAccount(account, list, removeTxs[account]); err != nil {
				p.logger.Errorf("cleanTxsByAccount error: %v", err)
			} else {
				removeCount += len(removeTxs[account])
			}
		}
	}

	readyNum := uint64(p.txStore.priorityIndex.size())
	// set priorityNonBatchSize to min(nonBatchedTxs, readyNum),
	if p.txStore.priorityNonBatchSize > readyNum {
		p.logger.Infof("Set priorityNonBatchSize from %d to the length of priorityIndex %d", p.txStore.priorityNonBatchSize, readyNum)
		p.setPriorityNonBatchSize(readyNum)
	}

	for account, pendingNonce := range updateAccounts {
		p.logger.Debugf("Account %s update its pendingNonce to %d by commitNonce", account, pendingNonce)
	}

	p.logger.Infof("finish RemoveStateUpdatingTxs, len:%d, removeCount:%d", len(txHashList), removeCount)
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
func (p *txPoolImpl[T, Constraint]) GetRequestsByHashList(batchHash string, timestamp int64, hashList []string,
	deDuplicateTxHashes []string) (txs []*T, localList []bool, missingTxsHash map[uint64]string, err error) {
	if batch, ok := p.txStore.batchesCache[batchHash]; ok {
		// If replica already has this batch, directly return tx list
		p.logger.Debugf("Batch %s is already in batchesCache", batchHash)

		txs = batch.TxList
		localList = batch.LocalList
		missingTxsHash = nil
		return
	}

	// If we have checked this batch and found we miss some transactions,
	// just return the same missingTxsHash as before
	if missingBatch, ok := p.txStore.missingBatch[batchHash]; ok {
		p.logger.Debugf("GetRequestsByHashList failed, find batch %s in missingBatch store", batchHash)
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
		txPointer := p.txStore.txHashMap[txHash]
		if txPointer == nil {
			p.logger.Debugf("Can't find tx by hash: %s from txpool", txHash)
			missingTxsHash[uint64(index)] = txHash
			hasMissing = true
			continue
		}
		if deDuplicateMap[txHash] {
			// ignore deDuplicate txs for duplicate rule
			p.logger.Noticef("Ignore de-duplicate tx %s when create same batch", txHash)
		} else {
			if _, ok := p.txStore.batchedTxs[*txPointer]; ok {
				// If this transaction has been batched, return ErrDuplicateTx
				p.logger.Warningf("Duplicate transaction in getTxsByHashList with "+
					"hash: %s, batch batchHash: %s", txHash, batchHash)
				err = errors.New("duplicate transaction")
				missingTxsHash = nil
				return
			}
		}
		poolTx := p.txStore.getPoolTxByTxnPointer(txPointer.account, txPointer.nonce)
		if poolTx == nil {
			p.logger.Warningf("Transaction %s exist in txHashMap but not in allTxs", txHash)
			missingTxsHash[uint64(index)] = txHash
			hasMissing = true
			continue
		}

		if !hasMissing {
			txs = append(txs, poolTx.rawTx)
			localList = append(localList, poolTx.local)
		}
	}

	if len(missingTxsHash) != 0 {
		txs = nil
		localList = nil
		p.txStore.missingBatch[batchHash] = missingTxsHash
		return
	}
	for _, txHash := range hashList {
		txPointer := p.txStore.txHashMap[txHash]
		p.txStore.batchedTxs[*txPointer] = true
	}
	// store the batch to cache
	batch := &RequestHashBatch[T, Constraint]{
		BatchHash:  batchHash,
		TxList:     txs,
		TxHashList: hashList,
		LocalList:  localList,
		Timestamp:  timestamp,
	}
	p.txStore.batchesCache[batchHash] = batch
	if p.txStore.priorityNonBatchSize <= uint64(len(hashList)) {
		p.setPriorityNonBatchSize(0)
	} else {
		p.decreasePriorityNonBatchSize(uint64(len(hashList)))
	}
	missingTxsHash = nil
	p.logger.Debugf("Replica generate a batch, which digest is %s, and now there are %d "+
		"non-batched txs and %d batches in txpool", batchHash, p.txStore.priorityNonBatchSize, len(p.txStore.batchesCache))
	return
}

// IsPoolFull checks if txPool is full which means if number of all cached txs
// has exceeded the limited txSize.
func (p *txPoolImpl[T, Constraint]) IsPoolFull() bool {
	return uint64(len(p.txStore.txHashMap)) >= p.poolSize
}

func (p *txPoolImpl[T, Constraint]) HasPendingRequestInPool() bool {
	return p.txStore.priorityNonBatchSize > 0
}

func (p *txPoolImpl[T, Constraint]) PendingRequestsNumberIsReady() bool {
	return p.txStore.priorityNonBatchSize >= p.batchSize
}

func (p *txPoolImpl[T, Constraint]) SendMissingRequests(batchHash string, missingHashList map[uint64]string) (
	txs map[uint64]*T, err error) {
	for _, txHash := range missingHashList {
		if txPointer := p.txStore.txHashMap[txHash]; txPointer == nil {
			return nil, fmt.Errorf("transaction %s doesn't exist in txHashMap", txHash)
		}
	}
	var targetBatch *RequestHashBatch[T, Constraint]
	var ok bool
	if targetBatch, ok = p.txStore.batchesCache[batchHash]; !ok {
		return nil, fmt.Errorf("batch %s doesn't exist in batchedCache", batchHash)
	}
	targetBatchLen := uint64(len(targetBatch.TxList))
	txs = make(map[uint64]*T)
	for index, txHash := range missingHashList {
		if index >= targetBatchLen || targetBatch.TxHashList[index] != txHash {
			return nil, fmt.Errorf("find invalid transaction, index: %d, targetHash: %s", index, txHash)
		}
		txs[index] = targetBatch.TxList[index]
	}
	return
}

func (p *txPoolImpl[T, Constraint]) ReceiveMissingRequests(batchHash string, txs map[uint64]*T) error {
	p.logger.Infof("Replica received %d missingTxs, batch hash: %s", len(txs), batchHash)
	if _, ok := p.txStore.missingBatch[batchHash]; !ok {
		p.logger.Warningf("Can't find batch %s from missingBatch", batchHash)
		return nil
	}
	expectLen := len(p.txStore.missingBatch[batchHash])
	if len(txs) != expectLen {
		return fmt.Errorf("receive unmatched fetching txn response, expect "+
			"length: %d, received length: %d", expectLen, len(txs))
	}
	validTxn := make([]*T, 0)
	targetBatch := p.txStore.missingBatch[batchHash]
	for index, tx := range txs {
		txHash := Constraint(tx).RbftGetTxHash()
		if txHash != targetBatch[index] {
			return errors.New("find a hash mismatch tx")
		}
		validTxn = append(validTxn, tx)
	}
	p.addNewRequests(validTxn, false, false, true, false, false)
	delete(p.txStore.missingBatch, batchHash)
	return nil
}

// ReConstructBatchByOrder reconstruct batch from empty txPool by order, must be called after RestorePool.
func (p *txPoolImpl[T, Constraint]) ReConstructBatchByOrder(oldBatch *RequestHashBatch[T, Constraint]) (
	deDuplicateTxHashes []string, err error) {
	// check if there exists duplicate batch hash.
	if _, ok := p.txStore.batchesCache[oldBatch.BatchHash]; ok {
		p.logger.Warningf("When re-construct batch, batch %s already exists", oldBatch.BatchHash)
		err = errors.New("invalid batch: batch already exists")
		return
	}

	// TxHashList has to match TxList by length and content
	if len(oldBatch.TxHashList) != len(oldBatch.TxList) {
		p.logger.Warningf("Batch is invalid because TxHashList and TxList have different lengths.")
		err = errors.New("invalid batch: TxHashList and TxList have different lengths")
		return
	}

	for i, tx := range oldBatch.TxList {
		txHash := Constraint(tx).RbftGetTxHash()
		if txHash != oldBatch.TxHashList[i] {
			p.logger.Warningf("Batch is invalid because the hash %s in txHashList does not match "+
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
		p.logger.Warningf("The given batch hash %s does not match with the "+
			"calculated batch hash %s.", oldBatch.BatchHash, batch.BatchHash)
		err = errors.New("invalid batch: batch hash does not match")
		return
	}

	// There may be some duplicate transactions which are batched in different batches during vc, for those txs,
	// we only accept them in the first batch containing them and de-duplicate them in following batches.
	for _, tx := range oldBatch.TxList {
		ptr := &txPointer{
			account: Constraint(tx).RbftGetFrom(),
			nonce:   Constraint(tx).RbftGetNonce(),
		}
		txHash := Constraint(tx).RbftGetTxHash()
		if _, ok := p.txStore.batchedTxs[*ptr]; ok {
			p.logger.Noticef("De-duplicate tx %s when re-construct batch by order", txHash)
			deDuplicateTxHashes = append(deDuplicateTxHashes, txHash)
		} else {
			p.txStore.batchedTxs[*ptr] = true
		}
	}
	p.logger.Debugf("ReConstructBatchByOrder batch %s into batchedCache", oldBatch.BatchHash)
	p.txStore.batchesCache[batch.BatchHash] = batch
	return
}

// RestorePool move all batched txs back to non-batched tx which should
// only be used after abnormal recovery.
func (p *txPoolImpl[T, Constraint]) RestorePool() {
	p.logger.Debugf("Before restore pool, there are %d non-batched txs, %d batches, "+
		"priority len: %d, parkingLot len: %d, batchedTx len: %d, txHashMap len: %d", p.txStore.priorityNonBatchSize,
		len(p.txStore.batchesCache), p.txStore.priorityIndex.size(), p.txStore.parkingLotIndex.size(),
		len(p.txStore.batchedTxs), len(p.txStore.txHashMap))

	// if the batches are fetched from primary, those txs will be put back to non-batched cache.
	// rollback txs currentSeq
	var (
		account string
		nonce   uint64
	)
	// record the smallest nonce of each account
	rollbackTxsNonce := make(map[string]uint64)
	for _, batch := range p.txStore.batchesCache {
		for _, tx := range batch.TxList {
			account = Constraint(tx).RbftGetFrom()
			nonce = Constraint(tx).RbftGetNonce()

			if _, ok := rollbackTxsNonce[account]; !ok {
				rollbackTxsNonce[account] = nonce
			} else if nonce < rollbackTxsNonce[account] {
				// if the nonce is smaller than the currentSeq, update it
				rollbackTxsNonce[account] = nonce
			}
		}
		// increase priorityNonBatchSize
		p.increasePriorityNonBatchSize(uint64(len(batch.TxList)))
	}
	// rollback nonce for each account
	for acc, non := range rollbackTxsNonce {
		p.txStore.nonceCache.setPendingNonce(acc, non)
	}

	// resubmit these txs to txpool.
	for batchDigest, batch := range p.txStore.batchesCache {
		p.logger.Debugf("Put batch %s back to pool with %d txs", batchDigest, len(batch.TxList))
		p.AddNewRequests(batch.TxList, false, false, true, false)
		delete(p.txStore.batchesCache, batchDigest)
	}

	// clear missingTxs after abnormal.
	p.txStore.missingBatch = make(map[string]map[uint64]string)
	p.txStore.batchedTxs = make(map[txPointer]bool)
	p.logger.Debugf("After restore pool, there are %d non-batched txs, %d batches, "+
		"priority len: %d, parkingLot len: %d, batchedTx len: %d, txHashMap len: %d", p.txStore.priorityNonBatchSize,
		len(p.txStore.batchesCache), p.txStore.priorityIndex.size(), p.txStore.parkingLotIndex.size(),
		len(p.txStore.batchedTxs), len(p.txStore.txHashMap))
}

// FilterOutOfDateRequests get the remained local txs in TTLIndex and broadcast to other vp peers by tolerance time.
func (p *txPoolImpl[T, Constraint]) FilterOutOfDateRequests() ([]*T, error) {
	now := time.Now().UnixNano()
	var forward []*internalTransaction[T, Constraint]
	p.txStore.localTTLIndex.data.Ascend(func(a btree.Item) bool {
		orderedKey := a.(*orderedIndexKey)
		poolTx := p.txStore.getPoolTxByTxnPointer(orderedKey.account, orderedKey.nonce)
		if poolTx == nil {
			p.logger.Error("Get nil poolTx from txStore")
			return true
		}
		if now-poolTx.lifeTime > p.toleranceTime.Nanoseconds() {
			// for those batched txs, we don't need to forward temporarily.
			if _, ok := p.txStore.batchedTxs[txPointer{account: orderedKey.account, nonce: orderedKey.nonce}]; !ok {
				forward = append(forward, poolTx)
			}
		} else {
			return false
		}
		return true
	})
	result := make([]*T, len(forward))
	// update pool tx's timestamp to now for next forwarding check.
	for i, poolTx := range forward {
		// update localTTLIndex
		p.txStore.localTTLIndex.updateIndex(poolTx, now)
		result[i] = poolTx.rawTx
		// update txpool tx in allTxs
		poolTx.lifeTime = now
	}
	return result, nil
}

func (p *txPoolImpl[T, Constraint]) fillRemoveTxs(orderedKey *orderedIndexKey, poolTx *internalTransaction[T, Constraint],
	removedTxs map[string][]*internalTransaction[T, Constraint]) {
	if _, ok := removedTxs[orderedKey.account]; !ok {
		removedTxs[orderedKey.account] = make([]*internalTransaction[T, Constraint], 0)
	}
	// record need removedTxs and the count
	removedTxs[orderedKey.account] = append(removedTxs[orderedKey.account], poolTx)
}

// RemoveTimeoutRequests remove the remained local txs in timeoutIndex and removeTxs in txpool by tolerance time.
func (p *txPoolImpl[T, Constraint]) RemoveTimeoutRequests() (uint64, error) {
	now := time.Now().UnixNano()
	removedTxs := make(map[string][]*internalTransaction[T, Constraint])
	var count uint64
	var index int
	p.txStore.removeTTLIndex.data.Ascend(func(a btree.Item) bool {
		index++
		removeKey := a.(*orderedIndexKey)
		poolTx := p.txStore.getPoolTxByTxnPointer(removeKey.account, removeKey.nonce)
		if poolTx == nil {
			p.logger.Errorf("Get nil poolTx from txStore:[account:%s, nonce:%d]", removeKey.account, removeKey.nonce)
			return true
		}
		if now-poolTx.arrivedTime > p.toleranceRemoveTime.Nanoseconds() {
			// for those batched txs, we don't need to removedTxs temporarily.
			if _, ok := p.txStore.batchedTxs[txPointer{account: removeKey.account, nonce: removeKey.nonce}]; ok {
				p.logger.Debugf("find tx[account: %s, nonce:%d] from batchedTxs, ignore remove request",
					removeKey.account, removeKey.nonce)
				return true
			}

			orderedKey := &orderedIndexKey{time: poolTx.getRawTimestamp(), account: poolTx.getAccount(), nonce: poolTx.getNonce()}
			if tx := p.txStore.priorityIndex.data.Get(orderedKey); tx != nil {
				p.fillRemoveTxs(orderedKey, poolTx, removedTxs)
				count++
				// remove txHashMap
				txHash := poolTx.getHash()
				p.txStore.deletePoolTx(txHash)
				return true
			}

			if tx := p.txStore.parkingLotIndex.data.Get(orderedKey); tx != nil {
				p.fillRemoveTxs(orderedKey, poolTx, removedTxs)
				count++
				// remove txHashMap
				txHash := poolTx.getHash()
				p.txStore.deletePoolTx(txHash)
				return true
			}
		}
		return true
	})
	for account, txs := range removedTxs {
		if list, ok := p.txStore.allTxs[account]; ok {
			// remove index from removedTxs
			_ = p.cleanTxsByAccount(account, list, txs)
		}
	}

	// when remove priorityIndex, we need to decrease priorityNonBatchSize
	// NOTICE!!! we remove priorityIndex when it's timeout, it may cause the priorityNonBatchSize is not correct.
	// for example, if we have 10 txs in priorityIndex whitch nonce is 1-10, and we remove nonce 1-5, then the priorityNonBatchSize is 5.
	// but the nonce 6-10 is not ready because of we remove nonce 1-5, so the priorityNonBatchSize is actual 0.
	readyNum := uint64(p.txStore.priorityIndex.size())
	// set priorityNonBatchSize to min(nonBatchedTxs, readyNum),
	if p.txStore.priorityNonBatchSize > readyNum {
		p.logger.Debugf("Set priorityNonBatchSize from %d to the length of priorityIndex %d", p.txStore.priorityNonBatchSize, readyNum)
		p.setPriorityNonBatchSize(readyNum)
	}

	return count, nil
}

// CheckRequestsExist check if the txs are in pool and mark them as batched if they are.
func (p *txPoolImpl[T, Constraint]) CheckRequestsExist(hashList []string) {
	for _, txHash := range hashList {
		if txnPointer, ok := p.txStore.txHashMap[txHash]; ok {
			p.txStore.batchedTxs[*txnPointer] = true
			continue
		}
		p.logger.Debugf("Can't find tx %s from txHashMap, but it exist in rbft batchStore, maybe has reset the pool", txHash)
	}
}

// =============================================================================
// internal methods
// =============================================================================
func (p *txPoolImpl[T, Constraint]) processDirtyAccount(dirtyAccounts map[string]bool) {
	for account := range dirtyAccounts {
		if list, ok := p.txStore.allTxs[account]; ok {
			// search for related sequential txs in allTxs
			// and add these txs into priorityIndex and parkingLotIndex.
			pendingNonce := p.txStore.nonceCache.getPendingNonce(account)
			readyTxs, nonReadyTxs, nextDemandNonce := list.filterReady(pendingNonce)
			p.txStore.nonceCache.setPendingNonce(account, nextDemandNonce)

			// insert ready txs into priorityIndex
			for _, poolTx := range readyTxs {
				p.txStore.priorityIndex.insertByOrderedQueueKey(poolTx)
				// p.logger.Debugf("insert ready tx[account: %s, nonce: %d] into priorityIndex", poolTx.getAccount(), poolTx.getNonce())
			}
			p.increasePriorityNonBatchSize(uint64(len(readyTxs)))

			// insert non-ready txs into parkingLotIndex
			for _, poolTx := range nonReadyTxs {
				p.txStore.parkingLotIndex.insertByOrderedQueueKey(poolTx)
			}
		}
	}
}

// generateRequestBatch fetches next block of transactions for consensus,
// batchedTx are all txs sent to consensus but were not committed yet, txpool should filter out such txs.
func (p *txPoolImpl[T, Constraint]) generateRequestBatch() ([]*RequestHashBatch[T, Constraint], error) {
	if !p.isTimed && !p.HasPendingRequestInPool() {
		p.logger.Debug("txpool is empty")
		return nil, nil
	}
	result := make([]txPointer, 0, p.batchSize)
	// txs has lower nonce will be observed first in priority index iterator.
	p.logger.Debugf("Length of non-batched transactions: %d", p.txStore.priorityNonBatchSize)
	var batchSize uint64
	if p.txStore.priorityNonBatchSize > p.batchSize {
		batchSize = p.batchSize
	} else {
		batchSize = p.txStore.priorityNonBatchSize
	}
	skippedTxs := make(map[txPointer]bool)
	p.txStore.priorityIndex.data.Ascend(func(a btree.Item) bool {
		tx := a.(*orderedIndexKey)
		// if tx has existed in bathedTxs
		// TODO (YH): refactor batchedTxs to seen (all the transactions that have been executed) to track all txs batched in ledger.
		if _, ok := p.txStore.batchedTxs[txPointer{account: tx.account, nonce: tx.nonce}]; ok {
			return true
		}
		txSeq := tx.nonce
		// p.logger.Debugf("txpool txNonce:%s-%d", tx.account, tx.nonce)
		commitNonce := p.txStore.nonceCache.getCommitNonce(tx.account)
		// p.logger.Debugf("ledger txNonce:%s-%d", tx.account, commitNonce)
		var seenPrevious bool
		if txSeq >= 1 {
			_, seenPrevious = p.txStore.batchedTxs[txPointer{account: tx.account, nonce: txSeq - 1}]
		}
		// include transaction if it's "next" for given account or
		// we've already sent its ancestor to Consensus
		ptr := txPointer{account: tx.account, nonce: tx.nonce}
		// commitNonce is the nonce of last committed tx for given account,
		// todo(lrx): not sure if txSeq == commitNonce is correct, maybe txSeq == commitNonce+1 is correct
		if seenPrevious || (txSeq == commitNonce) {
			p.txStore.batchedTxs[ptr] = true
			result = append(result, ptr)
			if uint64(len(result)) == batchSize {
				return false
			}

			// check if we can now include some txs that were skipped before for given account
			skippedTxn := txPointer{account: tx.account, nonce: tx.nonce + 1}
			for {
				if _, ok := skippedTxs[skippedTxn]; !ok {
					break
				}
				p.txStore.batchedTxs[skippedTxn] = true
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

	if !p.isTimed && len(result) == 0 && p.HasPendingRequestInPool() {
		p.logger.Warningf("===== Note!!! Primary generate a batch with 0 txs, "+
			"but PriorityNonBatchSize is %d, we need reset PriorityNonBatchSize", p.txStore.priorityNonBatchSize)
		p.setPriorityNonBatchSize(0)
		return nil, nil
	}

	batches := make([]*RequestHashBatch[T, Constraint], 0)
	// convert transaction pointers to real values
	hashList := make([]string, len(result))
	localList := make([]bool, len(result))
	txList := make([]*T, len(result))
	for i, v := range result {
		poolTx := p.txStore.getPoolTxByTxnPointer(v.account, v.nonce)
		if poolTx == nil {
			return nil, errors.New("get nil poolTx from txStore")
		}
		hashList[i] = poolTx.getHash()

		txList[i] = poolTx.rawTx
		localList[i] = poolTx.local
	}

	txBatch := &RequestHashBatch[T, Constraint]{
		TxHashList: hashList,
		TxList:     txList,
		LocalList:  localList,
		Timestamp:  time.Now().UnixNano(),
	}
	batchHash := getBatchHash[T, Constraint](txBatch)
	txBatch.BatchHash = batchHash
	p.txStore.batchesCache[batchHash] = txBatch
	if p.txStore.priorityNonBatchSize <= uint64(len(hashList)) {
		p.setPriorityNonBatchSize(0)
	} else {
		p.decreasePriorityNonBatchSize(uint64(len(hashList)))
	}
	batches = append(batches, txBatch)
	p.logger.Debugf("Primary generate a batch with %d txs, which hash is %s, and now there are %d "+
		"pending txs and %d batches in txPool", len(hashList), batchHash, p.txStore.priorityNonBatchSize, len(p.txStore.batchesCache))
	return batches, nil
}

func (p *txPoolImpl[T, Constraint]) replaceTx(tx *T, local, ready bool) {
	account := Constraint(tx).RbftGetFrom()
	txHash := Constraint(tx).RbftGetTxHash()
	txNonce := Constraint(tx).RbftGetNonce()
	oldPoolTx := p.txStore.getPoolTxByTxnPointer(account, txNonce)
	pointer := &txPointer{
		account: account,
		nonce:   txNonce,
	}

	// remove old tx from txHashMap、priorityIndex、parkingLotIndex、localTTLIndex and removeTTLIndex
	if oldPoolTx != nil {
		p.txStore.deletePoolTx(oldPoolTx.getHash())
		p.txStore.priorityIndex.removeByOrderedQueueKey(oldPoolTx)
		p.txStore.parkingLotIndex.removeByOrderedQueueKey(oldPoolTx)
		p.txStore.removeTTLIndex.removeByOrderedQueueKey(oldPoolTx)
		p.txStore.localTTLIndex.removeByOrderedQueueKey(oldPoolTx)
	}

	if !ready {
		// if not ready, just delete old tx, outbound will handle insert new tx
		return
	}
	// insert new tx
	p.txStore.insertPoolTx(txHash, pointer)

	// update txPointer in allTxs
	list, ok := p.txStore.allTxs[account]
	if !ok {
		list = newTxSortedMap[T, Constraint]()
		p.txStore.allTxs[account] = list
	}
	list.index.insertBySortedNonceKey(txNonce)
	now := time.Now().UnixNano()
	newPoolTx := &internalTransaction[T, Constraint]{
		local:       false,
		rawTx:       tx,
		lifeTime:    Constraint(tx).RbftGetTimeStamp(),
		arrivedTime: now,
	}
	list.items[txNonce] = newPoolTx

	// insert new tx received from remote vp
	p.txStore.priorityIndex.insertByOrderedQueueKey(newPoolTx)
	if local {
		p.txStore.localTTLIndex.insertByOrderedQueueKey(newPoolTx)
	}
	p.txStore.removeTTLIndex.insertByOrderedQueueKey(newPoolTx)
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

func (p *txPoolImpl[T, Constraint]) increasePriorityNonBatchSize(addSize uint64) {
	p.txStore.priorityNonBatchSize = p.txStore.priorityNonBatchSize + addSize
	readyTxNum.Set(float64(p.txStore.priorityNonBatchSize))
}

func (p *txPoolImpl[T, Constraint]) decreasePriorityNonBatchSize(subSize uint64) {
	p.txStore.priorityNonBatchSize = p.txStore.priorityNonBatchSize - subSize
	readyTxNum.Set(float64(p.txStore.priorityNonBatchSize))
}

func (p *txPoolImpl[T, Constraint]) setPriorityNonBatchSize(txnSize uint64) {
	p.txStore.priorityNonBatchSize = txnSize
	readyTxNum.Set(float64(p.txStore.priorityNonBatchSize))
}
