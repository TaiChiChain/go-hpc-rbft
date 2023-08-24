package mempool

import (
	"encoding/hex"
	"math/rand"
	"time"

	"github.com/axiomesh/axiom-bft/common/consensus"
)

// nolint
const (
	DefaultTestBatchSize = uint64(4)
)

func mockMempoolImpl[T any, Constraint consensus.TXConstraint[T]]() *mempoolImpl[T, Constraint] {
	config := NewMockMempoolConfig()
	mempool := newMempoolImpl[T, Constraint](config)
	return mempool
}

// NewMockMempoolConfig returns the default test config
func NewMockMempoolConfig() Config {
	poolConfig := Config{
		BatchSize:     DefaultTestBatchSize,
		PoolSize:      DefaultPoolSize,
		Logger:        NewRawLogger(),
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
		Timestamp: time.Now().Unix(),
	}
}

// ConstructTxByAccountAndNonce constructs a tx by given account and nonce.
func ConstructTxByAccountAndNonce(account string, nonce uint64) consensus.FltTransaction {
	from := make([]byte, 0)
	strLen := len(account)
	for i := 0; i < 20; i++ {
		from = append(from, account[i%strLen])
	}
	fromStr := hex.EncodeToString(from)
	tx := newMockFltTx(fromStr, int64(nonce))
	return tx
}

func NewRawLogger() Logger {
	return NewLogWrapper()
}
