package mempool

import (
	"fmt"
	"log"
	"time"

	"github.com/axiomesh/axiom-bft/common/consensus"
)

const (
	btreeDegree = 10
)

// nolint
const (
	DefaultPoolSize            = 50000
	DefaultBatchSize           = 500
	DefaultToleranceTime       = 5 * time.Minute
	DefaultToleranceRemoveTime = 15 * time.Minute
)

// RequestHashBatch contains transactions that batched by primary.
type RequestHashBatch[T any, Constraint consensus.TXConstraint[T]] struct {
	BatchHash  string   // hash of this batch calculated by MD5
	TxHashList []string // list of all txs' hashes
	TxList     [][]byte // list of all txs
	LocalList  []bool   // list track if tx is received locally or not
	Timestamp  int64    // generation time of this batch
}

type GetAccountNonceFunc func(address string) uint64

// Config defines the mempool config items.
type Config struct {
	ID                  uint64
	BatchSize           uint64
	PoolSize            uint64
	BatchMemLimit       bool
	BatchMaxMem         uint64
	IsTimed             bool
	ToleranceTime       time.Duration
	ToleranceRemoveTime time.Duration
	Logger              Logger
	GetAccountNonce     GetAccountNonceFunc
}

type mempoolTransaction[T any, Constraint consensus.TXConstraint[T]] struct {
	rawTx       *T
	local       bool
	lifeTime    int64 // track the local txs' broadcast time
	arrivedTime int64 // track the local txs' arrived memPool time
}

type txnPointer struct {
	account string
	nonce   uint64
}

// Logger is the mempool logger interface which managers logger output.
type Logger interface {
	Debug(v ...interface{})
	Debugf(format string, v ...interface{})

	Info(v ...interface{})
	Infof(format string, v ...interface{})

	Notice(v ...interface{})
	Noticef(format string, v ...interface{})

	Warning(v ...interface{})
	Warningf(format string, v ...interface{})

	Error(v ...interface{})
	Errorf(format string, v ...interface{})

	Critical(v ...interface{})
	Criticalf(format string, v ...interface{})
}

type LogWrapper struct {
	logger *log.Logger
}

func NewLogWrapper() Logger {
	return &LogWrapper{
		logger: log.New(log.Writer(), "", log.Lshortfile|log.Ldate|log.Ltime),
	}
}

func (lw *LogWrapper) Debug(v ...interface{}) {
	lw.logger.Println("[DEBUG]", fmt.Sprint(v...))
}

func (lw *LogWrapper) Debugf(format string, v ...interface{}) {
	lw.logger.Printf("[DEBUG] "+format, v...)
}

func (lw *LogWrapper) Info(v ...interface{}) {
	lw.logger.Println("[INFO]", fmt.Sprint(v...))
}

func (lw *LogWrapper) Infof(format string, v ...interface{}) {
	lw.logger.Printf("[INFO] "+format, v...)
}

func (lw *LogWrapper) Notice(v ...interface{}) {
	lw.logger.Println("[NOTICE]", fmt.Sprint(v...))
}

func (lw *LogWrapper) Noticef(format string, v ...interface{}) {
	lw.logger.Printf("[NOTICE] "+format, v...)
}

func (lw *LogWrapper) Warning(v ...interface{}) {
	lw.logger.Println("[WARNING]", fmt.Sprint(v...))
}

func (lw *LogWrapper) Warningf(format string, v ...interface{}) {
	lw.logger.Printf("[WARNING] "+format, v...)
}

func (lw *LogWrapper) Error(v ...interface{}) {
	lw.logger.Println("[ERROR]", fmt.Sprint(v...))
}

func (lw *LogWrapper) Errorf(format string, v ...interface{}) {
	lw.logger.Printf("[ERROR] "+format, v...)
}

func (lw *LogWrapper) Critical(v ...interface{}) {
	lw.logger.Println("[CRITICAL]", fmt.Sprint(v...))
}

func (lw *LogWrapper) Criticalf(format string, v ...interface{}) {
	lw.logger.Printf("[CRITICAL] "+format, v...)
}
