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

func testNewStorage(ctrl *gomock.Controller) (*storeManager, Config) {
	pool := txpoolmock.NewMockMinimalTxPool(ctrl)
	log := FrameworkNewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)

	conf := Config{
		ID:                      2,
		Hash:                    "hash-node2",
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
		MetricsProv: &disabled.Provider{},

		EpochInit:    uint64(0),
		LatestConfig: nil,
	}

	return newStoreMgr(conf), conf
}

func TestStoreMgr_newStoreMgr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	s, _ := testNewStorage(ctrl)

	structName, nilElems, err := checkNilElems(s)
	if err == nil {
		assert.Equal(t, "storeManager", structName)
		assert.Nil(t, nilElems)
	}
	assert.Equal(t, "XXX GENESIS", s.chkpts[0])
}

func TestStoreMgr_moveWatermarks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s, conf := testNewStorage(ctrl)
	cpChan := make(chan *pb.ServiceState)
	confC := make(chan *pb.ReloadFinished)
	rbft, _ := newRBFT(cpChan, confC, conf)

	QID := qidx{
		d: "useless",
		n: 20,
	}
	PQ := &pb.Vc_PQ{
		SequenceNumber: 20,
		BatchDigest:    "useless",
		View:           1,
	}
	s.chkpts[uint64(20)] = "base64"
	rbft.vcMgr.qlist[QID] = PQ
	rbft.vcMgr.plist[uint64(20)] = PQ

	// h is 30, delete them
	s.moveWatermarks(rbft, uint64(30))
	assert.Equal(t, map[uint64]string{}, s.chkpts)
	assert.Equal(t, map[qidx]*pb.Vc_PQ{}, rbft.vcMgr.qlist)
	assert.Equal(t, map[uint64]*pb.Vc_PQ{}, rbft.vcMgr.plist)
}

func TestStoreMgr_saveCheckpoint(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	s, _ := testNewStorage(ctrl)
	s.saveCheckpoint(uint64(10), "base64")

	assert.Equal(t, "base64", s.chkpts[uint64(10)])
}

func TestStoreMgr_getCert(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s, _ := testNewStorage(ctrl)

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

	s, _ := testNewStorage(ctrl)

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
