package rbft

import (
	"context"
	"encoding/binary"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/axiomesh/axiom-bft/common"
	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/common/metrics/disabled"
	"github.com/axiomesh/axiom-bft/txpool"
)

func newPersistTestReplica[T any, Constraint consensus.TXConstraint[T]](ctrl *gomock.Controller) (*node[T, Constraint], *MockExternalStack[T, Constraint]) {
	pool := txpool.NewMockMinimalTxPool[T, Constraint](ctrl)
	log := common.NewSimpleLogger()
	ext := NewMockExternalStack[T, Constraint](ctrl)
	ext.EXPECT().Sign(gomock.Any()).Return(nil, nil).AnyTimes()
	ext.EXPECT().Verify(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	conf := Config{
		SelfAccountAddress: "node2",
		GenesisEpochInfo: &EpochInfo{
			Version:                   1,
			Epoch:                     1,
			EpochPeriod:               1000,
			CandidateSet:              []NodeInfo{},
			ValidatorSet:              peerSet,
			StartBlock:                1,
			P2PBootstrapNodeAddresses: []string{"1"},
			ConsensusParams: ConsensusParams{
				ValidatorElectionType:         ValidatorElectionTypeWRF,
				ProposerElectionType:          ProposerElectionTypeAbnormalRotation,
				CheckpointPeriod:              10,
				HighWatermarkCheckpointPeriod: 4,
				MaxValidatorNum:               10,
				BlockMaxTxNum:                 500,
				NotActiveWeight:               1,
				ExcludeView:                   10,
			},
		},
		SetSize:                 25,
		SetTimeout:              100 * time.Millisecond,
		BatchTimeout:            500 * time.Millisecond,
		RequestTimeout:          6 * time.Second,
		NullRequestTimeout:      9 * time.Second,
		VcResendTimeout:         10 * time.Second,
		CleanVCTimeout:          60 * time.Second,
		NewViewTimeout:          8 * time.Second,
		SyncStateTimeout:        1 * time.Second,
		SyncStateRestartTimeout: 10 * time.Second,
		FetchCheckpointTimeout:  5 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger:      log,
		MetricsProv: &disabled.Provider{},
		DelFlag:     make(chan bool),
	}

	ext.EXPECT().GetEpochInfo(gomock.Any()).Return(conf.GenesisEpochInfo, nil).AnyTimes()
	ext.EXPECT().GetCurrentEpochInfo().Return(conf.GenesisEpochInfo, nil).AnyTimes()

	node, err := newNode[T, Constraint](conf, ext, pool, true)
	if err != nil {
		panic(err)
	}
	return node, ext
}

func TestPersist_restoreView(t *testing.T) {
	ctrl := gomock.NewController(t)

	node, ext := newPersistTestReplica[consensus.FltTransaction, *consensus.FltTransaction](ctrl)
	ext.EXPECT().ReadState("new-view").Return(nil, errors.New("not mock"))
	node.rbft.restoreView()
	assert.Equal(t, uint64(0), node.rbft.chainConfig.View)

	nv := &consensus.NewView{
		View: 1,
	}
	nvb, _ := nv.MarshalVTStrict()
	ext.EXPECT().ReadState("new-view").Return(nvb, nil)
	node.rbft.restoreView()
	assert.Equal(t, uint64(1), node.rbft.chainConfig.View)
}

func TestPersist_restoreQList(t *testing.T) {
	ctrl := gomock.NewController(t)

	node, ext := newPersistTestReplica[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	var ret map[string][]byte
	var err error

	ret = map[string][]byte{"qlist.": []byte("test")}
	ext.EXPECT().ReadStateSet("qlist.").Return(ret, nil)
	_, err = node.rbft.restoreQList()
	assert.Equal(t, "incorrect format", err.Error())

	ret = map[string][]byte{"2.qlist.1": []byte("test")}
	ext.EXPECT().ReadStateSet("qlist.").Return(ret, nil)
	_, err = node.rbft.restoreQList()
	assert.Equal(t, "incorrect prefix", err.Error())

	ret = map[string][]byte{"qlist.one.test": []byte("test")}
	ext.EXPECT().ReadStateSet("qlist.").Return(ret, nil)
	_, err = node.rbft.restoreQList()
	assert.Equal(t, "parse failed", err.Error())

	ret = map[string][]byte{"qlist.1.test": []byte("test")}
	ext.EXPECT().ReadStateSet("qlist.").Return(ret, nil)
	_, err = node.rbft.restoreQList()
	assert.Equal(t, "proto: VcPq: wiretype end group for non-group", err.Error())

	ret = map[string][]byte{"qlist.1.test": {24, 10}}
	ext.EXPECT().ReadStateSet("qlist.").Return(ret, nil)
	qlist, _ := node.rbft.restoreQList()
	assert.Equal(t, uint64(10), qlist[qidx{d: "test", n: 1}].View)
}

func TestPersist_restorePList(t *testing.T) {
	ctrl := gomock.NewController(t)
	node, ext := newPersistTestReplica[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	var ret map[string][]byte
	var err error

	ret = map[string][]byte{"plist.1.1": []byte("test")}
	ext.EXPECT().ReadStateSet("plist.").Return(ret, nil)
	_, err = node.rbft.restorePList()
	assert.Equal(t, "incorrect format", err.Error())

	ret = map[string][]byte{"1.plist": []byte("test")}
	ext.EXPECT().ReadStateSet("plist.").Return(ret, nil)
	_, err = node.rbft.restorePList()
	assert.Equal(t, "incorrect prefix", err.Error())

	ret = map[string][]byte{"plist.test": []byte("test")}
	ext.EXPECT().ReadStateSet("plist.").Return(ret, nil)
	_, err = node.rbft.restorePList()
	assert.Equal(t, "parse failed", err.Error())

	ret = map[string][]byte{"plist.1": {24, 9}}
	ext.EXPECT().ReadStateSet("plist.").Return(ret, nil)
	plist, _ := node.rbft.restorePList()
	assert.Equal(t, uint64(9), plist[uint64(1)].View)
}

func TestPersist_restoreBatchStore(t *testing.T) {
	ctrl := gomock.NewController(t)

	node, ext := newPersistTestReplica[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	ret := map[string][]byte{"batch.msg": {24, 10}}
	ext.EXPECT().ReadStateSet("batch.").Return(ret, nil)
	node.rbft.restoreBatchStore()

	assert.Equal(t, int64(10), node.rbft.storeMgr.batchStore["msg"].Timestamp)
}

func TestPersist_restoreQSet(t *testing.T) {
	ctrl := gomock.NewController(t)

	node, ext := newPersistTestReplica[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	q := &consensus.PrePrepare{
		ReplicaId:      1,
		View:           1,
		SequenceNumber: 2,
		BatchDigest:    "msg",
		HashBatch:      nil,
	}
	prePrepareByte, _ := q.MarshalVTStrict()
	retQset := map[string][]byte{
		"qset.1.2.msg": prePrepareByte,
	}
	ext.EXPECT().DelState(gomock.Any()).Return(nil).AnyTimes()
	ext.EXPECT().ReadState(gomock.Any()).Return(nil, errors.New("ReadState Error")).AnyTimes()
	ext.EXPECT().ReadStateSet("qset.").Return(retQset, nil)

	qset, _ := node.rbft.restoreQSet()
	assert.Equal(t, map[msgID]*consensus.PrePrepare{{v: 1, n: 2, d: "msg"}: q}, qset)
}

func TestPersist_restorePSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	node, ext := newPersistTestReplica[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	p := &consensus.Prepare{
		ReplicaId:      1,
		View:           1,
		SequenceNumber: 2,
		BatchDigest:    "msg",
	}
	set := &consensus.Pset{Set: []*consensus.Prepare{p}}
	PrepareByte, _ := set.MarshalVTStrict()
	retPset := map[string][]byte{
		"pset.1.2.msg": PrepareByte,
	}
	ext.EXPECT().DelState(gomock.Any()).Return(nil).AnyTimes()
	ext.EXPECT().ReadState(gomock.Any()).Return(nil, errors.New("ReadState Error")).AnyTimes()
	ext.EXPECT().ReadStateSet("pset.").Return(retPset, nil)

	pset, _ := node.rbft.restorePSet()
	assert.Equal(t, map[msgID]*consensus.Pset{{v: 1, n: 2, d: "msg"}: set}, pset)
}

func TestPersist_restoreCSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	node, ext := newPersistTestReplica[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	c := &consensus.Commit{
		ReplicaId:      1,
		View:           1,
		SequenceNumber: 2,
		BatchDigest:    "msg",
	}
	set := &consensus.Cset{Set: []*consensus.Commit{c}}
	CommitByte, _ := set.MarshalVTStrict()
	retCset := map[string][]byte{
		"cset.1.2.msg": CommitByte,
	}
	ext.EXPECT().DelState(gomock.Any()).Return(nil).AnyTimes()
	ext.EXPECT().ReadState(gomock.Any()).Return(nil, errors.New("ReadState Error")).AnyTimes()
	ext.EXPECT().ReadStateSet("cset.").Return(retCset, nil)

	cset, _ := node.rbft.restoreCSet()
	assert.Equal(t, map[msgID]*consensus.Cset{{v: 1, n: 2, d: "msg"}: set}, cset)
}

func TestPersist_restoreCert(t *testing.T) {
	ctrl := gomock.NewController(t)
	node, ext := newPersistTestReplica[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	q := &consensus.PrePrepare{
		ReplicaId:      1,
		View:           1,
		SequenceNumber: 2,
		BatchDigest:    "msg",
		HashBatch:      nil,
	}
	prePrepareByte, _ := q.MarshalVTStrict()
	retQset := map[string][]byte{
		"qset.1.2.msg": prePrepareByte,
	}
	ext.EXPECT().ReadStateSet("qset.").Return(retQset, nil)

	p := &consensus.Prepare{
		ReplicaId:      1,
		View:           1,
		SequenceNumber: 2,
		BatchDigest:    "msg",
	}
	pset := &consensus.Pset{Set: []*consensus.Prepare{p}}
	PrepareByte, _ := pset.MarshalVTStrict()
	retPset := map[string][]byte{
		"pset.1.2.msg": PrepareByte,
	}
	ext.EXPECT().ReadStateSet("pset.").Return(retPset, nil)

	c := &consensus.Commit{
		ReplicaId:      1,
		View:           1,
		SequenceNumber: 2,
		BatchDigest:    "msg",
	}
	cset := &consensus.Cset{Set: []*consensus.Commit{c}}
	CommitByte, _ := cset.MarshalVTStrict()
	retCset := map[string][]byte{
		"cset.1.2.msg": CommitByte,
	}
	ext.EXPECT().ReadStateSet("cset.").Return(retCset, nil)

	ext.EXPECT().DelState(gomock.Any()).Return(nil).AnyTimes()
	ext.EXPECT().ReadStateSet("qlist.").Return(map[string][]byte{"qlist.": []byte("QList")}, nil).AnyTimes()
	ext.EXPECT().ReadStateSet("plist.").Return(map[string][]byte{"plist.": []byte("PList")}, nil).AnyTimes()

	node.rbft.restoreCert()
	exp := &msgCert{prePrepare: q, prePrepareCtx: context.TODO(), prepare: map[string]*consensus.Prepare{p.ID(): p}, commit: map[string]*consensus.Commit{c.ID(): c}}
	assert.Equal(t, exp, node.rbft.storeMgr.certStore[msgID{v: 1, n: 2, d: "msg"}])
}

func TestPersist_restoreState(t *testing.T) {
	ctrl := gomock.NewController(t)

	node, ext := newPersistTestReplica[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	ret := map[string][]byte{
		"chkpt.1.wang": {24, 10},
		"chkpt.2.wang": {24, 9},
		"chkpt.3.wang": {24, 8},
	}

	var buff = make([]byte, 8)
	binary.LittleEndian.PutUint64(buff, uint64(1))
	nv := &consensus.NewView{
		View: 1,
	}
	nvb, _ := nv.MarshalVTStrict()
	ext.EXPECT().SendFilterEvent(gomock.Any(), gomock.Any()).Return().AnyTimes()
	ext.EXPECT().DelState(gomock.Any()).Return(nil).AnyTimes()

	ext.EXPECT().ReadState("epoch-info").Return(nil, errors.New("empty")).AnyTimes()
	ext.EXPECT().ReadState("new-view").Return(nvb, nil).AnyTimes()
	ext.EXPECT().ReadState("rbft.h").Return([]byte("10"), nil)
	ext.EXPECT().ReadState("latestConfigBatchHeight").Return(buff, nil).AnyTimes()
	ext.EXPECT().ReadState("stableC").Return(buff, nil).AnyTimes()

	ext.EXPECT().StoreState(gomock.Any(), gomock.Any())

	ext.EXPECT().ReadStateSet("qset.").Return(map[string][]byte{"qset.": []byte("QSet")}, nil).AnyTimes()
	ext.EXPECT().ReadStateSet("qlist.").Return(map[string][]byte{"qset.": []byte("QSet")}, nil).AnyTimes()
	ext.EXPECT().ReadStateSet("pset.").Return(map[string][]byte{"pset.": []byte("PSet")}, nil).AnyTimes()
	ext.EXPECT().ReadStateSet("plist.").Return(map[string][]byte{"pset.": []byte("PSet")}, nil).AnyTimes()
	ext.EXPECT().ReadStateSet("cset.").Return(map[string][]byte{"pset.": []byte("PSet")}, nil).AnyTimes()
	ext.EXPECT().ReadStateSet("batch.").Return(map[string][]byte{"cset.": []byte("CSet")}, nil).AnyTimes()
	ext.EXPECT().ReadStateSet("chkpt.").Return(ret, nil)

	// move h from 0 to 10
	assert.Nil(t, node.rbft.restoreState())
	assert.Equal(t, uint64(10), node.rbft.chainConfig.H)
}

func TestPersist_parseQPCKey(t *testing.T) {
	_, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	var u1, u2 uint64
	var str string
	var err error

	u1, u2, str, err = rbfts[0].parseQPCKey("test.1.wang", "test")
	assert.Equal(t, uint64(0), u1)
	assert.Equal(t, uint64(0), u2)
	assert.Equal(t, "", str)
	assert.Equal(t, "incorrect format", err.Error())

	u1, u2, str, err = rbfts[0].parseQPCKey("test.1.wang.2", "msg")
	assert.Equal(t, uint64(0), u1)
	assert.Equal(t, uint64(0), u2)
	assert.Equal(t, "", str)
	assert.Equal(t, "incorrect prefix", err.Error())

	u1, u2, str, err = rbfts[0].parseQPCKey("test.a.wang.1", "test")
	assert.Equal(t, uint64(0), u1)
	assert.Equal(t, uint64(0), u2)
	assert.Equal(t, "", str)
	assert.Equal(t, "parse failed", err.Error())

	u1, u2, str, err = rbfts[0].parseQPCKey("test.1.b.wang", "test")
	assert.Equal(t, uint64(0), u1)
	assert.Equal(t, uint64(0), u2)
	assert.Equal(t, "", str)
	assert.Equal(t, "parse failed", err.Error())

	u1, u2, str, err = rbfts[0].parseQPCKey("test.1.2.wang", "test")
	assert.Equal(t, uint64(1), u1)
	assert.Equal(t, uint64(2), u2)
	assert.Equal(t, "wang", str)
	assert.Equal(t, nil, err)
}
