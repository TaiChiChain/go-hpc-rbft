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
	TxList     []*T     // list of all txs
	LocalList  []bool   // list track if tx is received locally or not
	Timestamp  int64    // generation time of this batch
}

type GetAccountNonceFunc func(address string) uint64

// Config defines the mempool config items.
type Config struct {
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

type txPointer struct {
	account string
	nonce   uint64
}

// Logger is the mempool logger interface which managers logger output.
type Logger interface {
	Debug(v ...any)
	Debugf(format string, v ...any)

	Info(v ...any)
	Infof(format string, v ...any)

	Notice(v ...any)
	Noticef(format string, v ...any)

	Warning(v ...any)
	Warningf(format string, v ...any)

	Error(v ...any)
	Errorf(format string, v ...any)

	Critical(v ...any)
	Criticalf(format string, v ...any)
}

type LogWrapper struct {
	logger *log.Logger
}

func NewLogWrapper() Logger {
	return &LogWrapper{
		logger: log.New(log.Writer(), "", log.Lshortfile|log.Ldate|log.Ltime),
	}
}

func (lw *LogWrapper) Debug(v ...any) {
	lw.logger.Println("[DEBUG]", fmt.Sprint(v...))
}

func (lw *LogWrapper) Debugf(format string, v ...any) {
	lw.logger.Printf("[DEBUG] "+format, v...)
}

func (lw *LogWrapper) Info(v ...any) {
	lw.logger.Println("[INFO]", fmt.Sprint(v...))
}

func (lw *LogWrapper) Infof(format string, v ...any) {
	lw.logger.Printf("[INFO] "+format, v...)
}

func (lw *LogWrapper) Notice(v ...any) {
	lw.logger.Println("[NOTICE]", fmt.Sprint(v...))
}

func (lw *LogWrapper) Noticef(format string, v ...any) {
	lw.logger.Printf("[NOTICE] "+format, v...)
}

func (lw *LogWrapper) Warning(v ...any) {
	lw.logger.Println("[WARNING]", fmt.Sprint(v...))
}

func (lw *LogWrapper) Warningf(format string, v ...any) {
	lw.logger.Printf("[WARNING] "+format, v...)
}

func (lw *LogWrapper) Error(v ...any) {
	lw.logger.Println("[ERROR]", fmt.Sprint(v...))
}

func (lw *LogWrapper) Errorf(format string, v ...any) {
	lw.logger.Printf("[ERROR] "+format, v...)
}

func (lw *LogWrapper) Critical(v ...any) {
	lw.logger.Println("[CRITICAL]", fmt.Sprint(v...))
}

func (lw *LogWrapper) Criticalf(format string, v ...any) {
	lw.logger.Printf("[CRITICAL] "+format, v...)
}
