package mempool

import (
	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/google/btree"
)

const (
	Rebroadcast TTLIndexKeyType = "Rebroadcast" // indicates that the transaction latest arrived in memPool and rebroadcast to other vps.
	Remove      TTLIndexKeyType = "Remove"
)

type TTLIndexKeyType string

// the key of priorityIndex and parkingLotIndex.
type orderedIndexKey struct {
	time    int64
	account string
	nonce   uint64
}

// Less should guarantee item can be cast into orderedIndexKey.
func (oik *orderedIndexKey) Less(than btree.Item) bool {
	other := than.(*orderedIndexKey)
	if oik.time != other.time {
		return oik.time < other.time
	}
	if oik.account != other.account {
		return oik.account < other.account
	}
	return oik.nonce < other.nonce
}

type sortedNonceKey struct {
	nonce uint64
}

// Less should guarantee item can be cast into sortedNonceKey.
func (snk *sortedNonceKey) Less(item btree.Item) bool {
	dst, _ := item.(*sortedNonceKey)
	return snk.nonce < dst.nonce
}

func makeOrderedIndexKey(timestamp int64, account string, nonce uint64) *orderedIndexKey {
	return &orderedIndexKey{
		account: account,
		nonce:   nonce,
		time:    timestamp,
	}
}

func makeSortedNonceKey(nonce uint64) *sortedNonceKey {
	return &sortedNonceKey{
		nonce: nonce,
	}
}

type btreeIndex[T any, Constraint consensus.TXConstraint[T]] struct {
	data *btree.BTree
}

func newBtreeIndex[T any, Constraint consensus.TXConstraint[T]]() *btreeIndex[T, Constraint] {
	return &btreeIndex[T, Constraint]{
		data: btree.New(btreeDegree),
	}
}

func (idx *btreeIndex[T, Constraint]) insertBySortedNonceKey(nonce uint64) {
	idx.data.ReplaceOrInsert(makeSortedNonceKey(nonce))
}

func (idx *btreeIndex[T, Constraint]) removeBySortedNonceKeys(txs map[string][]*mempoolTransaction[T, Constraint]) {
	for _, list := range txs {
		for _, poolTx := range list {
			idx.data.Delete(makeSortedNonceKey(poolTx.getNonce()))
		}
	}
}

func (idx *btreeIndex[T, Constraint]) insertByOrderedQueueKey(timestamp int64, account string, nonce uint64) {
	idx.data.ReplaceOrInsert(makeOrderedIndexKey(timestamp, account, nonce))
}

func (idx *btreeIndex[T, Constraint]) removeByOrderedQueueKey(timestamp int64, account string, nonce uint64) {
	idx.data.Delete(makeOrderedIndexKey(timestamp, account, nonce))
}

func (idx *btreeIndex[T, Constraint]) removeByOrderedQueueKeys(poolTxs map[string][]*mempoolTransaction[T, Constraint]) {
	for _, list := range poolTxs {
		for _, poolTx := range list {
			idx.removeByOrderedQueueKey(poolTx.arrivedTime, poolTx.getAccount(), poolTx.getNonce())
		}
	}
}

// size returns the size of the index
func (idx *btreeIndex[T, Constraint]) size() int {
	return idx.data.Len()
}

func (idx *btreeIndex[T, Constraint]) insertByTTLIndexKey(poolTx *mempoolTransaction[T, Constraint], typ TTLIndexKeyType) {
	switch typ {
	case Remove:
		idx.data.ReplaceOrInsert(makeOrderedIndexKey(poolTx.arrivedTime, poolTx.getAccount(), poolTx.getNonce()))
	case Rebroadcast:
		idx.data.ReplaceOrInsert(makeOrderedIndexKey(poolTx.lifeTime, poolTx.getAccount(), poolTx.getNonce()))
	}
}

func (idx *btreeIndex[T, Constraint]) updateTTLIndex(oldTimestamp int64, account string, nonce uint64, newTimestamp int64) {
	oldOrderedKey := &orderedIndexKey{oldTimestamp, account, nonce}
	newOrderedKey := &orderedIndexKey{newTimestamp, account, nonce}
	idx.data.Delete(oldOrderedKey)
	idx.data.ReplaceOrInsert(newOrderedKey)
}

func (idx *btreeIndex[T, Constraint]) removeByTTLIndexByKeys(poolTxs map[string][]*mempoolTransaction[T, Constraint], typ TTLIndexKeyType) {
	for _, list := range poolTxs {
		for _, poolTx := range list {
			idx.removeByTTLIndexByKey(poolTx, typ)
		}
	}
}

func (idx *btreeIndex[T, Constraint]) removeByTTLIndexByKey(poolTx *mempoolTransaction[T, Constraint], typ TTLIndexKeyType) {
	switch typ {
	case Remove:
		idx.data.Delete(makeOrderedIndexKey(poolTx.arrivedTime, poolTx.getAccount(), poolTx.getNonce()))
	case Rebroadcast:
		idx.data.Delete(makeOrderedIndexKey(poolTx.lifeTime, poolTx.getAccount(), poolTx.getNonce()))
	}
}
