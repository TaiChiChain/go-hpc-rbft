package txpool

import (
	"github.com/samber/lo"
	"sort"
)

func (p *txPoolImpl[T, Constraint]) GetTotalPendingTxCount() uint64 {
	return uint64(len(p.txStore.txHashMap))
}

// GetPendingTxCountByAccount returns the latest pending nonce of the account in txpool
func (p *txPoolImpl[T, Constraint]) GetPendingTxCountByAccount(account string) uint64 {
	return p.txStore.nonceCache.getPendingNonce(account)
}

func (p *txPoolImpl[T, Constraint]) GetPendingTxByHash(hash string) *T {
	key, ok := p.txStore.txHashMap[hash]
	if !ok {
		return nil
	}

	txMap, ok := p.txStore.allTxs[key.account]
	if !ok {
		return nil
	}

	item, ok := txMap.items[key.nonce]
	if !ok {
		return nil
	}

	return item.rawTx
}

func (p *txPoolImpl[T, Constraint]) GetAccountMeta(account string, full bool) *AccountMeta[T, Constraint] {
	if p.txStore.allTxs[account] == nil {
		return &AccountMeta[T, Constraint]{
			CommitNonce:  p.txStore.nonceCache.getCommitNonce(account),
			PendingNonce: p.txStore.nonceCache.getPendingNonce(account),
			TxCount:      0,
			Txs:          []*TxInfo[T, Constraint]{},
			SimpleTxs:    []*TxSimpleInfo{},
		}
	}

	fullTxs := p.txStore.allTxs[account].items
	res := &AccountMeta[T, Constraint]{
		CommitNonce:  p.txStore.nonceCache.getCommitNonce(account),
		PendingNonce: p.txStore.nonceCache.getPendingNonce(account),
		TxCount:      uint64(len(fullTxs)),
	}

	if full {
		res.Txs = make([]*TxInfo[T, Constraint], 0, len(fullTxs))
		for _, tx := range fullTxs {
			res.Txs = append(res.Txs, &TxInfo[T, Constraint]{
				Tx:          tx.rawTx,
				Local:       tx.local,
				LifeTime:    tx.lifeTime,
				ArrivedTime: tx.arrivedTime,
			})
		}
		sort.Slice(res.Txs, func(i, j int) bool {
			return Constraint(res.Txs[i].Tx).RbftGetNonce() < Constraint(res.Txs[j].Tx).RbftGetNonce()
		})
	} else {
		res.SimpleTxs = make([]*TxSimpleInfo, 0, len(fullTxs))
		for _, tx := range fullTxs {
			c := Constraint(tx.rawTx)
			res.SimpleTxs = append(res.SimpleTxs, &TxSimpleInfo{
				Hash:        c.RbftGetTxHash(),
				Nonce:       c.RbftGetNonce(),
				Size:        c.RbftGetSize(),
				Local:       tx.local,
				LifeTime:    tx.lifeTime,
				ArrivedTime: tx.arrivedTime,
			})
		}
		sort.Slice(res.SimpleTxs, func(i, j int) bool {
			return res.SimpleTxs[i].Nonce < res.SimpleTxs[j].Nonce
		})
	}

	return res
}

func (p *txPoolImpl[T, Constraint]) GetMeta(full bool) *Meta[T, Constraint] {
	res := &Meta[T, Constraint]{
		TxCountLimit:    p.poolSize,
		TxCount:         uint64(len(p.txStore.txHashMap)),
		ReadyTxCount:    p.txStore.priorityNonBatchSize,
		Batches:         make(map[string]*BatchSimpleInfo, len(p.txStore.batchesCache)),
		MissingBatchTxs: make(map[string]map[uint64]string, len(p.txStore.missingBatch)),
		Accounts:        make(map[string]*AccountMeta[T, Constraint], len(p.txStore.allTxs)),
	}
	for _, batch := range p.txStore.batchesCache {
		txs := make([]*TxSimpleInfo, 0, len(batch.TxHashList))
		for _, txHash := range batch.TxHashList {
			txIdx := p.txStore.txHashMap[txHash]
			tx := p.txStore.allTxs[txIdx.account].items[txIdx.nonce]
			c := Constraint(tx.rawTx)
			txs = append(txs, &TxSimpleInfo{
				Hash:        c.RbftGetTxHash(),
				Nonce:       c.RbftGetNonce(),
				Size:        c.RbftGetSize(),
				Local:       tx.local,
				LifeTime:    tx.lifeTime,
				ArrivedTime: tx.arrivedTime,
			})
		}
		res.Batches[batch.BatchHash] = &BatchSimpleInfo{
			TxCount:   uint64(len(batch.TxHashList)),
			Txs:       txs,
			Timestamp: batch.Timestamp,
		}
	}
	for h, b := range p.txStore.missingBatch {
		res.MissingBatchTxs[h] = lo.MapEntries(b, func(key uint64, value string) (uint64, string) {
			return key, value
		})
	}
	for addr := range p.txStore.allTxs {
		res.Accounts[addr] = p.GetAccountMeta(addr, full)
	}

	return res
}
