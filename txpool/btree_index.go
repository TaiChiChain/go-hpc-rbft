package txpool

import (
	"github.com/google/btree"

	"github.com/axiomesh/axiom-bft/common/consensus"
)

const (
	Ordered = iota
	SortNonce
	Remove
	Rebroadcast
)

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
	typ  int
}

func newBtreeIndex[T any, Constraint consensus.TXConstraint[T]](typ int) *btreeIndex[T, Constraint] {
	return &btreeIndex[T, Constraint]{
		data: btree.New(btreeDegree),
		typ:  typ,
	}
}

func (idx *btreeIndex[T, Constraint]) getTimestamp(poolTx *internalTransaction[T, Constraint]) int64 {
	switch idx.typ {
	case Ordered:
		return poolTx.getRawTimestamp()
	case Rebroadcast:
		return poolTx.lifeTime
	case Remove:
		return poolTx.arrivedTime
	}
	return 0
}

func (idx *btreeIndex[T, Constraint]) insertBySortedNonceKey(nonce uint64) {
	idx.data.ReplaceOrInsert(makeSortedNonceKey(nonce))
}

func (idx *btreeIndex[T, Constraint]) removeBySortedNonceKeys(txs map[string][]*internalTransaction[T, Constraint]) {
	for _, list := range txs {
		for _, poolTx := range list {
			idx.data.Delete(makeSortedNonceKey(poolTx.getNonce()))
		}
	}
}

func (idx *btreeIndex[T, Constraint]) insertByOrderedQueueKey(poolTx *internalTransaction[T, Constraint]) {
	idx.data.ReplaceOrInsert(makeOrderedIndexKey(idx.getTimestamp(poolTx), poolTx.getAccount(), poolTx.getNonce()))
}

func (idx *btreeIndex[T, Constraint]) removeByOrderedQueueKey(poolTx *internalTransaction[T, Constraint]) {
	idx.data.Delete(makeOrderedIndexKey(idx.getTimestamp(poolTx), poolTx.getAccount(), poolTx.getNonce()))
}

func (idx *btreeIndex[T, Constraint]) removeByOrderedQueueKeys(poolTxs map[string][]*internalTransaction[T, Constraint]) {
	for _, list := range poolTxs {
		for _, poolTx := range list {
			idx.removeByOrderedQueueKey(poolTx)
		}
	}
}

// size returns the size of the index
func (idx *btreeIndex[T, Constraint]) size() int {
	return idx.data.Len()
}

func (idx *btreeIndex[T, Constraint]) updateIndex(oldPoolTx *internalTransaction[T, Constraint], newTimestamp int64) {
	oldOrderedKey := &orderedIndexKey{time: idx.getTimestamp(oldPoolTx), account: oldPoolTx.getAccount(), nonce: oldPoolTx.getNonce()}
	newOrderedKey := &orderedIndexKey{time: newTimestamp, account: oldPoolTx.getAccount(), nonce: oldPoolTx.getNonce()}
	idx.data.Delete(oldOrderedKey)
	idx.data.ReplaceOrInsert(newOrderedKey)
}
