package txpool

import (
	"sync"

	"github.com/google/btree"

	"github.com/axiomesh/axiom-bft/common/consensus"
)

type transactionStore[T any, Constraint consensus.TXConstraint[T]] struct {
	logger Logger
	// track all valid tx hashes cached in txpool
	txHashMap map[string]*txPointer

	// track all valid tx, mapping user' account to all related transactions.
	allTxs map[string]*txSortedMap[T, Constraint]

	// track the commit nonce and pending nonce of each account.
	nonceCache *nonceCache

	// keeps track of "non-ready" txs (txs that can't be included in next block)
	// only used to help remove some txs if pool is full.
	parkingLotIndex *btreeIndex[T, Constraint]

	// keeps track of "ready" txs
	priorityIndex *btreeIndex[T, Constraint]

	// cache all the batched txs which haven't executed.
	batchedTxs map[txPointer]bool

	// cache all batches created by current primary in order, removed after they are been executed.
	batchesCache map[string]*RequestHashBatch[T, Constraint]

	// trace the missing transaction
	missingBatch map[string]map[uint64]string

	// track the non-batch priority transaction.
	priorityNonBatchSize uint64

	// localTTLIndex based on the tolerance time to track all the remained txs
	// that generate by itself and rebroadcast to other vps.
	localTTLIndex *btreeIndex[T, Constraint]

	// removeTTLIndex based on the remove tolerance time to track all the remained txs
	// that arrived in txpool and remove these txs from memPoll cache in case these exist too long.
	removeTTLIndex *btreeIndex[T, Constraint]
}

func newTransactionStore[T any, Constraint consensus.TXConstraint[T]](f GetAccountNonceFunc, logger Logger) *transactionStore[T, Constraint] {
	return &transactionStore[T, Constraint]{
		logger:               logger,
		priorityNonBatchSize: 0,
		txHashMap:            make(map[string]*txPointer),
		allTxs:               make(map[string]*txSortedMap[T, Constraint]),
		batchedTxs:           make(map[txPointer]bool),
		missingBatch:         make(map[string]map[uint64]string),
		batchesCache:         make(map[string]*RequestHashBatch[T, Constraint]),
		parkingLotIndex:      newBtreeIndex[T, Constraint](Ordered),
		priorityIndex:        newBtreeIndex[T, Constraint](Ordered),
		localTTLIndex:        newBtreeIndex[T, Constraint](Rebroadcast),
		removeTTLIndex:       newBtreeIndex[T, Constraint](Remove),
		nonceCache:           newNonceCache(f),
	}
}

func (txStore *transactionStore[T, Constraint]) insertPoolTx(txHash string, pointer *txPointer) {
	if _, ok := txStore.txHashMap[txHash]; !ok {
		txStore.txHashMap[txHash] = pointer
		poolTxNum.Inc()
		return
	}
	txStore.logger.Warningf("tx %s already exists in txpool", txHash)
	txStore.txHashMap[txHash] = pointer
}

func (txStore *transactionStore[T, Constraint]) deletePoolTx(txHash string) {
	if _, ok := txStore.txHashMap[txHash]; ok {
		delete(txStore.txHashMap, txHash)
		poolTxNum.Dec()
	}
}

func (txStore *transactionStore[T, Constraint]) insertTxs(txItems map[string][]*internalTransaction[T, Constraint], isLocal bool) map[string]bool {
	dirtyAccounts := make(map[string]bool)
	for account, list := range txItems {
		for _, txItem := range list {
			txHash := txItem.getHash()
			pointer := &txPointer{
				account: account,
				nonce:   txItem.getNonce(),
			}
			txStore.insertPoolTx(txHash, pointer)
			txList, ok := txStore.allTxs[account]
			if !ok {
				// if this is new account to send tx, create a new txSortedMap
				txStore.allTxs[account] = newTxSortedMap[T, Constraint]()
			}
			txList = txStore.allTxs[account]
			txList.items[txItem.getNonce()] = txItem
			txList.index.insertBySortedNonceKey(txItem.getNonce())
			if isLocal {
				txStore.localTTLIndex.insertByOrderedQueueKey(txItem)
			}
			// record the tx arrived timestamp
			txStore.removeTTLIndex.insertByOrderedQueueKey(txItem)
		}
		dirtyAccounts[account] = true
	}
	return dirtyAccounts
}

// getPoolTxByTxnPointer gets transaction by account address + nonce
func (txStore *transactionStore[T, Constraint]) getPoolTxByTxnPointer(account string, nonce uint64) *internalTransaction[T, Constraint] {
	if list, ok := txStore.allTxs[account]; ok {
		return list.items[nonce]
	}
	return nil
}

type txSortedMap[T any, Constraint consensus.TXConstraint[T]] struct {
	items map[uint64]*internalTransaction[T, Constraint] // map nonce to transaction
	index *btreeIndex[T, Constraint]                     // index for items' nonce
}

func newTxSortedMap[T any, Constraint consensus.TXConstraint[T]]() *txSortedMap[T, Constraint] {
	return &txSortedMap[T, Constraint]{
		items: make(map[uint64]*internalTransaction[T, Constraint]),
		index: newBtreeIndex[T, Constraint](SortNonce),
	}
}

func (m *txSortedMap[T, Constraint]) filterReady(demandNonce uint64) ([]*internalTransaction[T, Constraint], []*internalTransaction[T, Constraint], uint64) {
	var readyTxs, nonReadyTxs []*internalTransaction[T, Constraint]
	if m.index.data.Len() == 0 {
		return nil, nil, demandNonce
	}
	demandKey := makeSortedNonceKey(demandNonce)
	m.index.data.AscendGreaterOrEqual(demandKey, func(i btree.Item) bool {
		nonce := i.(*sortedNonceKey).nonce
		if nonce == demandNonce {
			readyTxs = append(readyTxs, m.items[demandNonce])
			demandNonce++
		} else {
			nonReadyTxs = append(nonReadyTxs, m.items[nonce])
		}
		return true
	})

	return readyTxs, nonReadyTxs, demandNonce
}

// forward removes all allTxs from the map with a nonce lower than the
// provided commitNonce.
func (m *txSortedMap[T, Constraint]) forward(commitNonce uint64) map[string][]*internalTransaction[T, Constraint] {
	removedTxs := make(map[string][]*internalTransaction[T, Constraint])
	commitNonceKey := makeSortedNonceKey(commitNonce)
	m.index.data.AscendLessThan(commitNonceKey, func(i btree.Item) bool {
		// delete tx from map.
		nonce := i.(*sortedNonceKey).nonce
		txItem := m.items[nonce]
		account := txItem.getAccount()
		if _, ok := removedTxs[account]; !ok {
			removedTxs[account] = make([]*internalTransaction[T, Constraint], 0)
		}
		removedTxs[account] = append(removedTxs[account], txItem)
		delete(m.items, nonce)
		return true
	})
	return removedTxs
}

// TODO (YH): persist and restore commitNonce and pendingNonce from db.
type nonceCache struct {
	// commitNonces records each account's latest committed nonce in ledger.
	commitNonces map[string]uint64

	// pendingNonces records each account's latest nonce which has been included in
	// priority queue. Invariant: pendingNonces[account] >= commitNonces[account]
	pendingNonces map[string]uint64

	pendingMu       sync.RWMutex
	commitMu        sync.Mutex
	getAccountNonce GetAccountNonceFunc
}

func newNonceCache(f GetAccountNonceFunc) *nonceCache {
	return &nonceCache{
		commitNonces:    make(map[string]uint64),
		pendingNonces:   make(map[string]uint64),
		getAccountNonce: f,
	}
}

func (nc *nonceCache) getCommitNonce(account string) uint64 {
	nc.commitMu.Lock()
	defer nc.commitMu.Unlock()

	nonce, ok := nc.commitNonces[account]
	if !ok {
		cn := nc.getAccountNonce(account)
		nc.commitNonces[account] = cn
		return cn
	}
	return nonce
}

func (nc *nonceCache) setCommitNonce(account string, nonce uint64) {
	nc.commitNonces[account] = nonce
}

func (nc *nonceCache) getPendingNonce(account string) uint64 {
	nc.pendingMu.RLock()
	defer nc.pendingMu.RUnlock()
	nonce, ok := nc.pendingNonces[account]
	if !ok {
		// if nonce is unknown, check if there is committed nonce persisted in db
		// cause there are no pending txs in txpool now, pending nonce is equal to committed nonce
		return nc.getCommitNonce(account)
	}
	return nonce
}

func (nc *nonceCache) setPendingNonce(account string, nonce uint64) {
	nc.pendingMu.Lock()
	nc.pendingNonces[account] = nonce
	nc.pendingMu.Unlock()
}

func (tx *internalTransaction[T, Constraint]) getRawTimestamp() int64 {
	return Constraint(tx.rawTx).RbftGetTimeStamp()
}

func (tx *internalTransaction[T, Constraint]) getAccount() string {
	return Constraint(tx.rawTx).RbftGetFrom()
}

func (tx *internalTransaction[T, Constraint]) getNonce() uint64 {
	return Constraint(tx.rawTx).RbftGetNonce()
}

func (tx *internalTransaction[T, Constraint]) getHash() string {
	return Constraint(tx.rawTx).RbftGetTxHash()
}
