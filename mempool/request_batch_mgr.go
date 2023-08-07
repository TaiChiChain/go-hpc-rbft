package mempool

import (
	"github.com/axiomesh/axiom-bft/common/consensus"
)

// CheckRequestHashBatch provide methods for batch type
type CheckRequestHashBatch interface {
	// IsConfBatch checks if it's a config-change batch:
	// 1. there's only one tx in it
	// 2. the tx in it is config-change tx
	// true  - it is a config-change batch
	// false - it is not a config-change batch
	IsConfBatch() bool
}

// =============================================================================
// methods to check RequestHashBatch
// =============================================================================

// IsConfBatch checks if it's a config-change batch
func (batch *RequestHashBatch[T, Constraint]) IsConfBatch() bool {
	if len(batch.TxList) == 1 {
		if consensus.IsConfigTx[T, Constraint](batch.TxList[0]) {
			return true
		}
	}
	return false
}
