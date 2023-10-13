package txpool

import (
	"encoding/hex"
	"math/rand"
	"time"

	"github.com/axiomesh/axiom-bft/common"
	"github.com/axiomesh/axiom-bft/common/consensus"
)

// nolint
const (
	DefaultTestBatchSize = uint64(4)
)

func mockTxPoolImpl[T any, Constraint consensus.TXConstraint[T]]() *txPoolImpl[T, Constraint] {
	return newTxPoolImpl[T, Constraint](NewMockTxPoolConfig())
}

// NewMockTxPoolConfig returns the default test config
func NewMockTxPoolConfig() Config {
	poolConfig := Config{
		BatchSize:     DefaultTestBatchSize,
		PoolSize:      DefaultPoolSize,
		Logger:        common.NewSimpleLogger(),
		ToleranceTime: DefaultToleranceTime,
		GetAccountNonce: func(address string) uint64 {
			return 0
		},
		IsTimed: false,
	}
	return poolConfig
}

func newMockFltTx(from string, nonce int64) consensus.FltTransaction {
	return consensus.FltTransaction{
		From:      []byte(from),
		Value:     []byte(string(rune(rand.Int()))),
		Nonce:     nonce,
		Timestamp: time.Now().UnixNano(),
	}
}

// ConstructTxByAccountAndNonce constructs a tx by given account and nonce.
func ConstructTxByAccountAndNonce(account string, nonce uint64) *consensus.FltTransaction {
	from := make([]byte, 0)
	strLen := len(account)
	for i := 0; i < 20; i++ {
		from = append(from, account[i%strLen])
	}
	fromStr := hex.EncodeToString(from)
	tx := newMockFltTx(fromStr, int64(nonce))
	return &tx
}
