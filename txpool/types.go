package txpool

import (
	"time"

	"github.com/axiomesh/axiom-bft/common"
	"github.com/axiomesh/axiom-bft/common/consensus"
)

const (
	btreeDegree = 10
)

// nolint
const (
	DefaultPoolSize            = 50000
	DefaultBatchSize           = 500
	DefaultToleranceNonceGap   = 1000
	DefaultToleranceTime       = 5 * time.Minute
	DefaultToleranceRemoveTime = 15 * time.Minute
)

// RequestHashBatch contains transactions that batched by primary.
type RequestHashBatch[T any, Constraint consensus.TXConstraint[T]] struct {
	BatchHash  string   // hash of this batch calculated by MD5
	TxHashList []string // list of all txs' hashes
	TxList     []*T     // list of all txs
	LocalList  []bool   // list track if tx is received locally or not
	Timestamp  int64    // generation time of this batch
}

type GetAccountNonceFunc func(address string) uint64

// Config defines the txpool config items.
type Config struct {
	BatchSize           uint64
	PoolSize            uint64
	BatchMemLimit       bool
	BatchMaxMem         uint64
	IsTimed             bool
	ToleranceNonceGap   uint64
	ToleranceTime       time.Duration
	ToleranceRemoveTime time.Duration
	Logger              common.Logger
	GetAccountNonce     GetAccountNonceFunc
}

type internalTransaction[T any, Constraint consensus.TXConstraint[T]] struct {
	rawTx       *T
	local       bool
	lifeTime    int64 // track the local txs' broadcast time
	arrivedTime int64 // track the local txs' arrived txpool time
}

type txPointer struct {
	account string
	nonce   uint64
}
