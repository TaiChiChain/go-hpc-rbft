package rbft

import (
	"errors"
	"os"
	"reflect"
	"time"

	"github.com/ultramesh/flato-event/inner/protos"
	mockexternal "github.com/ultramesh/flato-rbft/mock/mock_external"
	pb "github.com/ultramesh/flato-rbft/rbftpb"
	"github.com/ultramesh/flato-txpool"
	txpoolmock "github.com/ultramesh/flato-txpool/mock"

	"github.com/golang/mock/gomock"
	"github.com/ultramesh/fancylogger"
)

//============================================
// Global value
//============================================

var peerSet = []*pb.Peer{
	{
		Id:   uint64(1),
		Hash: "node1",
	},
	{
		Id:   uint64(2),
		Hash: "node2",
	},
	{
		Id:   uint64(3),
		Hash: "node3",
	},
	{
		Id:   uint64(4),
		Hash: "node4",
	},
}

var (
	mockTx1 = &protos.Transaction{
		From:      []byte("Alice"),
		To:        []byte("Bob"),
		Signature: []byte("sig1"),
	}

	mockTx2 = &protos.Transaction{
		From:      []byte("Bob"),
		To:        []byte("Alice"),
		Signature: []byte("sig2"),
	}

	mockTx3 = &protos.Transaction{
		From:      []byte("Bob"),
		To:        []byte("Alice"),
		Signature: []byte("sig3"),
	}

	mockRequestList  = []*protos.Transaction{mockTx1}
	mockRequestLists = []*protos.Transaction{mockTx1, mockTx2}
)

func newTestRBFT(ctrl *gomock.Controller) (*rbftImpl, Config) {
	reqBatchTmp1 := &txpool.RequestHashBatch{
		BatchHash:  calculateMD5Hash([]string{"Hash11", "Hash12"}, 1),
		TxHashList: []string{"Hash11", "Hash12"},
		TxList:     []*protos.Transaction{mockTx1, mockTx2},
		LocalList:  []bool{true, true},
		Timestamp:  1,
	}
	reqBatchTmp2 := &txpool.RequestHashBatch{
		BatchHash:  calculateMD5Hash([]string{"Hash21", "Hash23"}, 2),
		TxHashList: []string{"Hash21", "Hash23"},
		TxList:     []*protos.Transaction{mockTx1, mockTx3},
		LocalList:  []bool{true, true},
		Timestamp:  2,
	}
	reqBatchTmp3 := &txpool.RequestHashBatch{
		BatchHash:  calculateMD5Hash([]string{"Hash32", "Hash33"}, 3),
		TxHashList: []string{"Hash32", "Hash33"},
		TxList:     []*protos.Transaction{mockTx2, mockTx3},
		LocalList:  []bool{true, true},
		Timestamp:  3,
	}
	batch := []*txpool.RequestHashBatch{reqBatchTmp1, reqBatchTmp2, reqBatchTmp3}

	mockTxpool := txpoolmock.NewMockTxPool(ctrl)
	// Special usage for mockTxpool
	mockTxpool.EXPECT().AddNewRequest(gomock.Any(), gomock.Any(), gomock.Any()).Return(batch).AnyTimes()
	// General usage for mockTxpool
	mockTxpool.EXPECT().GenerateRequestBatch().Return(nil).AnyTimes()
	mockTxpool.EXPECT().RemoveBatches(gomock.Any()).Return().AnyTimes()
	mockTxpool.EXPECT().IsPoolFull().Return(false).AnyTimes()
	mockTxpool.EXPECT().HasPendingRequestInPool().Return(false).AnyTimes()
	mockTxpool.EXPECT().RestoreOneBatch(gomock.Any()).Return(nil).AnyTimes()
	mockTxpool.EXPECT().GetRequestsByHashList(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, nil, nil).AnyTimes()
	mockTxpool.EXPECT().SendMissingRequests(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	mockTxpool.EXPECT().ReceiveMissingRequests(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockTxpool.EXPECT().FilterOutOfDateRequests().Return(nil, nil).AnyTimes()
	mockTxpool.EXPECT().RestorePool().Return().AnyTimes()
	mockTxpool.EXPECT().ReConstructBatchByOrder(gomock.Any()).Return(nil, nil).AnyTimes()
	mockTxpool.EXPECT().Reset().Return().AnyTimes()
	mockTxpool.EXPECT().IsConfigBatch(gomock.Any()).Return(false).AnyTimes()
	pool := mockTxpool
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)

	conf := Config{
		ID:                      1,
		Hash:                    "node1",
		IsNew:                   false,
		Peers:                   peerSet,
		K:                       10,
		LogMultiplier:           4,
		SetSize:                 25,
		SetTimeout:              100 * time.Millisecond,
		BatchTimeout:            500 * time.Millisecond,
		RequestTimeout:          6 * time.Second,
		NullRequestTimeout:      9 * time.Second,
		VcResendTimeout:         10 * time.Second,
		CleanVCTimeout:          60 * time.Second,
		NewViewTimeout:          8 * time.Second,
		FirstRequestTimeout:     30 * time.Second,
		SyncStateTimeout:        1 * time.Second,
		SyncStateRestartTimeout: 10 * time.Second,
		RecoveryTimeout:         10 * time.Second,
		FetchCheckpointTimeout:  5 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger:      log,
		External:    external,
		RequestPool: pool,

		EpochInit: uint64(0),
	}

	node, _ := newNode(conf)

	return node.rbft, conf
}

func newTestRBFTReplica(ctrl *gomock.Controller) (*rbftImpl, Config) {
	reqBatchTmp1 := &txpool.RequestHashBatch{
		BatchHash:  calculateMD5Hash([]string{"Hash11", "Hash12"}, 1),
		TxHashList: []string{"Hash11", "Hash12"},
		TxList:     []*protos.Transaction{mockTx1, mockTx2},
		LocalList:  []bool{true, true},
		Timestamp:  1,
	}
	reqBatchTmp2 := &txpool.RequestHashBatch{
		BatchHash:  calculateMD5Hash([]string{"Hash21", "Hash23"}, 2),
		TxHashList: []string{"Hash21", "Hash23"},
		TxList:     []*protos.Transaction{mockTx1, mockTx3},
		LocalList:  []bool{true, true},
		Timestamp:  2,
	}
	reqBatchTmp3 := &txpool.RequestHashBatch{
		BatchHash:  calculateMD5Hash([]string{"Hash32", "Hash33"}, 3),
		TxHashList: []string{"Hash32", "Hash33"},
		TxList:     []*protos.Transaction{mockTx2, mockTx3},
		LocalList:  []bool{true, true},
		Timestamp:  3,
	}
	batch := []*txpool.RequestHashBatch{reqBatchTmp1, reqBatchTmp2, reqBatchTmp3}

	mockTxpool := txpoolmock.NewMockTxPool(ctrl)
	// Special usage for mockTxpool
	mockTxpool.EXPECT().AddNewRequest(gomock.Any(), gomock.Any(), gomock.Any()).Return(batch).AnyTimes()
	// General usage for mockTxpool
	mockTxpool.EXPECT().GenerateRequestBatch().Return(nil).AnyTimes()
	mockTxpool.EXPECT().RemoveBatches(gomock.Any()).Return().AnyTimes()
	mockTxpool.EXPECT().IsPoolFull().Return(false).AnyTimes()
	mockTxpool.EXPECT().HasPendingRequestInPool().Return(false).AnyTimes()
	mockTxpool.EXPECT().RestoreOneBatch(gomock.Any()).Return(nil).AnyTimes()
	mockTxpool.EXPECT().GetRequestsByHashList(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, nil, nil).AnyTimes()
	mockTxpool.EXPECT().SendMissingRequests(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	mockTxpool.EXPECT().ReceiveMissingRequests(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockTxpool.EXPECT().FilterOutOfDateRequests().Return(nil, nil).AnyTimes()
	mockTxpool.EXPECT().RestorePool().Return().AnyTimes()
	mockTxpool.EXPECT().ReConstructBatchByOrder(gomock.Any()).Return(nil, nil).AnyTimes()
	mockTxpool.EXPECT().Reset().Return().AnyTimes()
	mockTxpool.EXPECT().IsConfigBatch(gomock.Any()).Return(false).AnyTimes()
	pool := mockTxpool
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)

	conf := Config{
		ID:                      2,
		Hash:                    "node2",
		IsNew:                   false,
		Peers:                   peerSet,
		K:                       10,
		LogMultiplier:           4,
		SetSize:                 25,
		SetTimeout:              100 * time.Millisecond,
		BatchTimeout:            500 * time.Millisecond,
		RequestTimeout:          6 * time.Second,
		NullRequestTimeout:      9 * time.Second,
		VcResendTimeout:         10 * time.Second,
		CleanVCTimeout:          60 * time.Second,
		NewViewTimeout:          8 * time.Second,
		FirstRequestTimeout:     30 * time.Second,
		SyncStateTimeout:        1 * time.Second,
		SyncStateRestartTimeout: 10 * time.Second,
		RecoveryTimeout:         10 * time.Second,
		FetchCheckpointTimeout:  5 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger:      log,
		External:    external,
		RequestPool: pool,

		EpochInit: uint64(0),
	}

	node, _ := newNode(conf)

	return node.rbft, conf
}

// NewRawLogger returns a raw logger used in test in which logger messages
// will be printed to Stdout with string formatter.
func NewRawLogger() *fancylogger.Logger {
	rawLogger := fancylogger.NewLogger("test", fancylogger.DEBUG)

	consoleFormatter := &fancylogger.StringFormatter{
		EnableColors:    true,
		TimestampFormat: "2006-01-02T15:04:05.000",
		IsTerminal:      true,
	}
	consoleBackend := fancylogger.NewIOBackend(consoleFormatter, os.Stdout)

	rawLogger.SetBackends(consoleBackend)
	rawLogger.SetEnableCaller(true)

	return rawLogger
}

// checkNilElems checks if provided struct has nil elements, returns error if provided
// param is not a struct pointer and returns all nil elements' name if has.
func checkNilElems(i interface{}) (string, []string, error) {
	typ := reflect.TypeOf(i)
	value := reflect.Indirect(reflect.ValueOf(i))

	if typ.Kind() != reflect.Ptr {
		return "", nil, errors.New("got a non-ptr to check if has nil elements")
	}
	typ = typ.Elem()
	if typ.Kind() != reflect.Struct {
		return "", nil, errors.New("got a non-struct to check if has nil elements")
	}

	structName := typ.Name()
	nilElems := make([]string, 0)
	hasNil := false

	for i := 0; i < typ.NumField(); i++ {
		kind := typ.Field(i).Type.Kind()
		if kind == reflect.Chan || kind == reflect.Map {
			elemName := typ.Field(i).Name
			if value.FieldByName(elemName).IsNil() {
				nilElems = append(nilElems, elemName)
				hasNil = true
			}
		}
	}
	if hasNil {
		return structName, nilElems, nil
	}
	return structName, nil, nil
}
