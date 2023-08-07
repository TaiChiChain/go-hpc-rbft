package mempool

import (
	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/google/btree"
)

// the key of priorityIndex and parkingLotIndex.
type orderedIndexKey struct {
	recvTime   int64
	arriveTime int64
	account    string
	nonce      uint64
}

// Less should guarantee item can be cast into orderedIndexKey.
func (oik *orderedIndexKey) Less(than btree.Item) bool {
	other := than.(*orderedIndexKey)
	if oik.recvTime != other.recvTime {
		return oik.recvTime < other.recvTime
	}
	if oik.arriveTime != other.arriveTime {
		return oik.arriveTime < other.arriveTime
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

func makeOrderedIndexKey(recvTimestamp, arrivedTimestamp int64, account string, nonce uint64) *orderedIndexKey {
	return &orderedIndexKey{
		account:    account,
		nonce:      nonce,
		recvTime:   recvTimestamp,
		arriveTime: arrivedTimestamp,
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

func (idx *btreeIndex[T, Constraint]) insertByOrderedQueueKey(recvTimestamp, arrivedTimestamp int64, account string, nonce uint64) {
	idx.data.ReplaceOrInsert(makeOrderedIndexKey(recvTimestamp, arrivedTimestamp, account, nonce))
}

func (idx *btreeIndex[T, Constraint]) removeByOrderedQueueKey(recvTimestamp, arrivedTimestamp int64, account string, nonce uint64) {
	idx.data.Delete(makeOrderedIndexKey(recvTimestamp, arrivedTimestamp, account, nonce))
}

func (idx *btreeIndex[T, Constraint]) removeByOrderedQueueKeys(poolTxs map[string][]*mempoolTransaction[T, Constraint]) {
	for _, list := range poolTxs {
		for _, poolTx := range list {
			idx.removeByOrderedQueueKey(poolTx.getRawTimestamp(), poolTx.arrivedTime, poolTx.getAccount(), poolTx.getNonce())
		}
	}
}

// size returns the size of the index
func (idx *btreeIndex[T, Constraint]) size() int {
	return idx.data.Len()
}

func (idx *btreeIndex[T, Constraint]) insertByTTLIndexKey(poolTx *mempoolTransaction[T, Constraint]) {
	idx.data.ReplaceOrInsert(makeOrderedIndexKey(poolTx.lifeTime, poolTx.arrivedTime, poolTx.getAccount(), poolTx.getNonce()))
}

func (idx *btreeIndex[T, Constraint]) updateTTLIndex(oldTimestamp int64, arrivedTimestamp int64, account string, nonce uint64, newTimestamp int64) {
	oldOrderedKey := &orderedIndexKey{oldTimestamp, arrivedTimestamp, account, nonce}
	newOrderedKey := &orderedIndexKey{newTimestamp, arrivedTimestamp, account, nonce}
	idx.data.Delete(oldOrderedKey)
	idx.data.ReplaceOrInsert(newOrderedKey)
}

func (idx *btreeIndex[T, Constraint]) removeByTTLIndexByKeys(poolTxs map[string][]*mempoolTransaction[T, Constraint]) {
	for _, list := range poolTxs {
		for _, poolTx := range list {
			idx.removeByTTLIndexByKey(poolTx)
		}
	}
}

func (idx *btreeIndex[T, Constraint]) removeByTTLIndexByKey(poolTx *mempoolTransaction[T, Constraint]) {
	idx.data.Delete(makeOrderedIndexKey(poolTx.lifeTime, poolTx.arrivedTime, poolTx.getAccount(), poolTx.getNonce()))
}
