package rbft

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/common/metrics/disabled"
	mockexternal "github.com/axiomesh/axiom-bft/mock/mock_external"

	txpoolmock "github.com/axiomesh/axiom-bft/txpool/mock"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func newPersistTestReplica[T any, Constraint consensus.TXConstraint[T]](ctrl *gomock.Controller) (*node[T, Constraint], *mockexternal.MockExternalStack[T, Constraint]) {
	pool := txpoolmock.NewMockMinimalTxPool[T, Constraint](ctrl)
	log := newRawLogger()
	ext := mockexternal.NewMockExternalStack[T, Constraint](ctrl)
	ext.EXPECT().Sign(gomock.Any()).Return([]byte("sig"), nil).AnyTimes()

	conf := Config[T, Constraint]{
		ID:                      2,
		Hash:                    calHash("node2"),
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
		SyncStateTimeout:        1 * time.Second,
		SyncStateRestartTimeout: 10 * time.Second,
		FetchCheckpointTimeout:  5 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger:      log,
		External:    ext,
		RequestPool: pool,
		MetricsProv: &disabled.Provider{},
		DelFlag:     make(chan bool),

		EpochInit:    uint64(0),
		LatestConfig: nil,
	}

	node, _ := newNode[T, Constraint](conf)
	return node, ext
}

func TestPersist_restoreView(t *testing.T) {
	ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	node, ext := newPersistTestReplica[consensus.Transaction](ctrl)
	ext.EXPECT().ReadState("new-view").Return(nil, errors.New("err"))
	node.rbft.restoreView()
	assert.Equal(t, uint64(0), node.rbft.view)

	nv := &consensus.NewView{
		View: 1,
	}
	nvb, _ := proto.Marshal(nv)
	ext.EXPECT().ReadState("new-view").Return(nvb, nil)

	node.rbft.restoreView()
	assert.Equal(t, uint64(1), node.rbft.view)
}

func TestPersist_restoreQList(t *testing.T) {
	ctrl := gomock.NewController(t)
	//defer ctrl.Finish()
	node, ext := newPersistTestReplica[consensus.Transaction](ctrl)

	var ret map[string][]byte
	var err error

	ret = map[string][]byte{"qlist.": []byte("test")}
	ext.EXPECT().ReadStateSet("qlist.").Return(ret, nil)
	_, err = node.rbft.restoreQList()
	assert.Equal(t, errors.New("incorrect format"), err)

	ret = map[string][]byte{"2.qlist.1": []byte("test")}
	ext.EXPECT().ReadStateSet("qlist.").Return(ret, nil)
	_, err = node.rbft.restoreQList()
	assert.Equal(t, errors.New("incorrect prefix"), err)

	ret = map[string][]byte{"qlist.one.test": []byte("test")}
	ext.EXPECT().ReadStateSet("qlist.").Return(ret, nil)
	_, err = node.rbft.restoreQList()
	assert.Equal(t, errors.New("parse failed"), err)

	ret = map[string][]byte{"qlist.1.test": []byte("test")}
	ext.EXPECT().ReadStateSet("qlist.").Return(ret, nil)
	_, err = node.rbft.restoreQList()
	assert.Equal(t, errors.New("proto: vc_PQ: wiretype end group for non-group"), err)

	ret = map[string][]byte{"qlist.1.test": {24, 10}}
	ext.EXPECT().ReadStateSet("qlist.").Return(ret, nil)
	qlist, _ := node.rbft.restoreQList()
	assert.Equal(t, uint64(10), qlist[qidx{"test", 1}].View)
}

func TestPersist_restorePList(t *testing.T) {
	ctrl := gomock.NewController(t)
	//defer ctrl.Finish()
	node, ext := newPersistTestReplica[consensus.Transaction](ctrl)

	var ret map[string][]byte
	var err error

	ret = map[string][]byte{"plist.1.1": []byte("test")}
	ext.EXPECT().ReadStateSet("plist.").Return(ret, nil)
	_, err = node.rbft.restorePList()
	assert.Equal(t, errors.New("incorrect format"), err)

	ret = map[string][]byte{"1.plist": []byte("test")}
	ext.EXPECT().ReadStateSet("plist.").Return(ret, nil)
	_, err = node.rbft.restorePList()
	assert.Equal(t, errors.New("incorrect prefix"), err)

	ret = map[string][]byte{"plist.test": []byte("test")}
	ext.EXPECT().ReadStateSet("plist.").Return(ret, nil)
	_, err = node.rbft.restorePList()
	assert.Equal(t, errors.New("parse failed"), err)

	ret = map[string][]byte{"plist.1": {24, 9}}
	ext.EXPECT().ReadStateSet("plist.").Return(ret, nil)
	plist, _ := node.rbft.restorePList()
	assert.Equal(t, uint64(9), plist[uint64(1)].View)
}

func TestPersist_restoreBatchStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	node, ext := newPersistTestReplica[consensus.Transaction](ctrl)

	var ret map[string][]byte
	ret = map[string][]byte{"batch.msg": {24, 10}}
	ext.EXPECT().ReadStateSet("batch.").Return(ret, nil)
	node.rbft.restoreBatchStore()

	assert.Equal(t, int64(10), node.rbft.storeMgr.batchStore["msg"].Timestamp)
}

func TestPersist_restoreQSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	node, ext := newPersistTestReplica[consensus.Transaction](ctrl)

	q := &consensus.PrePrepare{
		ReplicaId:      1,
		View:           1,
		SequenceNumber: 2,
		BatchDigest:    "msg",
		HashBatch:      nil,
	}
	prePrepareByte, _ := proto.Marshal(q)
	retQset := map[string][]byte{
		"qset.1.2.msg": prePrepareByte,
	}
	ext.EXPECT().DelState(gomock.Any()).Return(nil).AnyTimes()
	ext.EXPECT().ReadState(gomock.Any()).Return(nil, errors.New("ReadState Error")).AnyTimes()
	ext.EXPECT().ReadStateSet("qset.").Return(retQset, nil)

	qset, _ := node.rbft.restoreQSet()
	assert.Equal(t, map[msgID]*consensus.PrePrepare{{1, 2, "msg"}: q}, qset)
}

func TestPersist_restorePSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	node, ext := newPersistTestReplica[consensus.Transaction](ctrl)

	p := &consensus.Prepare{
		ReplicaId:      1,
		View:           1,
		SequenceNumber: 2,
		BatchDigest:    "msg",
	}
	set := &consensus.Pset{Set: []*consensus.Prepare{p}}
	PrepareByte, _ := proto.Marshal(set)
	retPset := map[string][]byte{
		"pset.1.2.msg": PrepareByte,
	}
	ext.EXPECT().DelState(gomock.Any()).Return(nil).AnyTimes()
	ext.EXPECT().ReadState(gomock.Any()).Return(nil, errors.New("ReadState Error")).AnyTimes()
	ext.EXPECT().ReadStateSet("pset.").Return(retPset, nil)

	pset, _ := node.rbft.restorePSet()
	assert.Equal(t, map[msgID]*consensus.Pset{{1, 2, "msg"}: set}, pset)
}

func TestPersist_restoreCSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	node, ext := newPersistTestReplica[consensus.Transaction](ctrl)

	c := &consensus.Commit{
		ReplicaId:      1,
		View:           1,
		SequenceNumber: 2,
		BatchDigest:    "msg",
	}
	set := &consensus.Cset{Set: []*consensus.Commit{c}}
	CommitByte, _ := proto.Marshal(set)
	retCset := map[string][]byte{
		"cset.1.2.msg": CommitByte,
	}
	ext.EXPECT().DelState(gomock.Any()).Return(nil).AnyTimes()
	ext.EXPECT().ReadState(gomock.Any()).Return(nil, errors.New("ReadState Error")).AnyTimes()
	ext.EXPECT().ReadStateSet("cset.").Return(retCset, nil)

	cset, _ := node.rbft.restoreCSet()
	assert.Equal(t, map[msgID]*consensus.Cset{{1, 2, "msg"}: set}, cset)
}

func TestPersist_restoreCert(t *testing.T) {
	ctrl := gomock.NewController(t)
	//defer ctrl.Finish()
	node, ext := newPersistTestReplica[consensus.Transaction](ctrl)

	q := &consensus.PrePrepare{
		ReplicaId:      1,
		View:           1,
		SequenceNumber: 2,
		BatchDigest:    "msg",
		HashBatch:      nil,
	}
	prePrepareByte, _ := proto.Marshal(q)
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
	PrepareByte, _ := proto.Marshal(pset)
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
	CommitByte, _ := proto.Marshal(cset)
	retCset := map[string][]byte{
		"cset.1.2.msg": CommitByte,
	}
	ext.EXPECT().ReadStateSet("cset.").Return(retCset, nil)

	ext.EXPECT().DelState(gomock.Any()).Return(nil).AnyTimes()
	ext.EXPECT().ReadStateSet("qlist.").Return(map[string][]byte{"qlist.": []byte("QList")}, nil).AnyTimes()
	ext.EXPECT().ReadStateSet("plist.").Return(map[string][]byte{"plist.": []byte("PList")}, nil).AnyTimes()

	node.rbft.restoreCert()
	exp := &msgCert{prePrepare: q, prePrepareCtx: context.TODO(), prepare: map[consensus.Prepare]bool{*p: true}, commit: map[consensus.Commit]bool{*c: true}}
	assert.Equal(t, exp, node.rbft.storeMgr.certStore[msgID{1, 2, "msg"}])
}

func TestPersist_restoreState(t *testing.T) {
	ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	node, ext := newPersistTestReplica[consensus.Transaction](ctrl)

	var ret map[string][]byte
	ret = map[string][]byte{
		"chkpt.1.wang": {24, 10},
		"chkpt.2.wang": {24, 9},
		"chkpt.3.wang": {24, 8},
	}

	var buff = make([]byte, 8)
	binary.LittleEndian.PutUint64(buff, uint64(1))
	nv := &consensus.NewView{
		View: 1,
	}
	nvb, _ := proto.Marshal(nv)
	ext.EXPECT().SendFilterEvent(gomock.Any(), gomock.Any()).Return().AnyTimes()
	ext.EXPECT().DelState(gomock.Any()).Return(nil).AnyTimes()

	ext.EXPECT().ReadState("new-view").Return(nvb, nil).AnyTimes()
	ext.EXPECT().ReadState("nodes").Return(buff, nil)
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

	ext.EXPECT().IsConfigBlock(gomock.Any()).Return(false).AnyTimes()

	// move h from 0 to 10
	assert.Nil(t, node.rbft.restoreState())
	assert.Equal(t, uint64(10), node.rbft.h)
}

func TestPersist_parseQPCKey(t *testing.T) {

	_, rbfts := newBasicClusterInstance[consensus.Transaction]()
	var u1, u2 uint64
	var str string
	var err error

	u1, u2, str, err = rbfts[0].parseQPCKey("test.1.wang", "test")
	assert.Equal(t, uint64(0), u1)
	assert.Equal(t, uint64(0), u2)
	assert.Equal(t, "", str)
	assert.Equal(t, errors.New("incorrect format"), err)

	u1, u2, str, err = rbfts[0].parseQPCKey("test.1.wang.2", "msg")
	assert.Equal(t, uint64(0), u1)
	assert.Equal(t, uint64(0), u2)
	assert.Equal(t, "", str)
	assert.Equal(t, errors.New("incorrect prefix"), err)

	u1, u2, str, err = rbfts[0].parseQPCKey("test.a.wang.1", "test")
	assert.Equal(t, uint64(0), u1)
	assert.Equal(t, uint64(0), u2)
	assert.Equal(t, "", str)
	assert.Equal(t, errors.New("parse failed"), err)

	u1, u2, str, err = rbfts[0].parseQPCKey("test.1.b.wang", "test")
	assert.Equal(t, uint64(0), u1)
	assert.Equal(t, uint64(0), u2)
	assert.Equal(t, "", str)
	assert.Equal(t, errors.New("parse failed"), err)

	u1, u2, str, err = rbfts[0].parseQPCKey("test.1.2.wang", "test")
	assert.Equal(t, uint64(1), u1)
	assert.Equal(t, uint64(2), u2)
	assert.Equal(t, "wang", str)
	assert.Equal(t, nil, err)
}
