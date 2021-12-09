package rbft

import (
	"testing"
	"time"

	"github.com/ultramesh/flato-common/metrics/disabled"
	mockexternal "github.com/ultramesh/flato-rbft/mock/mock_external"
	pb "github.com/ultramesh/flato-rbft/rbftpb"
	txpoolmock "github.com/ultramesh/flato-txpool/mock"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func newStorageTestNode(ctrl *gomock.Controller) (*storeManager, Config) {
	pool := txpoolmock.NewMockMinimalTxPool(ctrl)
	log := FrameworkNewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)

	conf := Config{
		ID:                      2,
		Hash:                    "hash-node2",
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
		MetricsProv: &disabled.Provider{},
		DelFlag:     make(chan bool),

		EpochInit:    uint64(0),
		LatestConfig: nil,
	}

	return newStoreMgr(conf), conf
}

func TestStoreMgr_getCert(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s, _ := newStorageTestNode(ctrl)

	var retCert *msgCert
	// get default cert
	certDefault := &msgCert{
		prePrepare:  nil,
		sentPrepare: false,
		prepare:     make(map[pb.Prepare]bool),
		sentCommit:  false,
		commit:      make(map[pb.Commit]bool),
		sentExecute: false,
	}

	retCert = s.getCert(1, 10, "default")
	assert.Equal(t, certDefault, retCert)

	msgIDTmp := msgID{
		v: 1,
		n: 20,
		d: "tmp",
	}
	certTmp := &msgCert{
		prePrepare:  nil,
		sentPrepare: true,
		prepare:     make(map[pb.Prepare]bool),
		sentCommit:  true,
		commit:      make(map[pb.Commit]bool),
		sentExecute: true,
	}
	s.certStore[msgIDTmp] = certTmp
	retCert = s.getCert(1, 20, "tmp")
	assert.Equal(t, certTmp, retCert)
}

func TestStoreMgr_existedDigest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s, _ := newStorageTestNode(ctrl)

	msgIDTmp := msgID{
		v: 1,
		n: 20,
		d: "tmp",
	}
	prePrepareTmp := &pb.PrePrepare{
		ReplicaId:      2,
		View:           0,
		SequenceNumber: 20,
		BatchDigest:    "tmp",
		HashBatch:      nil,
	}
	certTmp := &msgCert{
		prePrepare:  prePrepareTmp,
		sentPrepare: true,
		prepare:     make(map[pb.Prepare]bool),
		sentCommit:  true,
		commit:      make(map[pb.Commit]bool),
		sentExecute: true,
	}
	s.certStore[msgIDTmp] = certTmp

	assert.Equal(t, false, s.existedDigest(20, 0, "tmp"))
	assert.Equal(t, true, s.existedDigest(10, 0, "tmp"))
}
