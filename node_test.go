package rbft

import (
	"errors"
	"testing"
	"time"

	mockexternal "github.com/ultramesh/flato-rbft/mock/mock_external"
	pb "github.com/ultramesh/flato-rbft/rbftpb"
	txpoolmock "github.com/ultramesh/flato-txpool/mock"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNode_NewNode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	pool := txpoolmock.NewMockMinimalTxPool(ctrl)
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)

	conf := Config{
		ID:                      2,
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
		UpdateTimeout:           4 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger:      log,
		External:    external,
		RequestPool: pool,
	}
	node, _ := NewNode(conf)

	structName, nilElems, err := checkNilElems(node)
	if err == nil {
		assert.Equal(t, "node", structName)
		assert.Nil(t, nilElems)
	}

	_, err = NewNode(conf)
	assert.Equal(t, nil, err)

	conf.Peers = nil
	_, err = NewNode(conf)
	expErr := errors.New("nil peers")
	assert.Equal(t, expErr, err)
}

func TestNode_Start(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	pool := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:                      2,
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
		UpdateTimeout:           4 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger:      log,
		External:    external,
		RequestPool: pool,
	}
	n, _ := newNode(conf)
	n.rbft.on(Pending)

	// Test Normal Case
	_ = n.Start()
	assert.Equal(t, false, n.rbft.in(Pending))
}

func TestNode_Stop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	pool := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:                      2,
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
		UpdateTimeout:           4 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger:      log,
		External:    external,
		RequestPool: pool,
	}
	n, _ := newNode(conf)
	n.currentState = &pb.ServiceState{Digest: "digest"}

	n.Stop()
	assert.Nil(t, n.currentState)
}

func TestNode_Propose(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	pool := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:                      2,
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
		UpdateTimeout:           4 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger:      log,
		External:    external,
		RequestPool: pool,
	}
	n, _ := newNode(conf)

	go func() {
		requestsTmp := mockRequestList
		_ = n.Propose(requestsTmp)
		obj := <-n.rbft.recvChan
		rSet := &pb.RequestSet{
			Requests: requestsTmp,
			Local:    true,
		}
		assert.Equal(t, rSet, obj)
	}()
}

func TestNode_ProposeConfChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	pool := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:                      2,
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
		UpdateTimeout:           4 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger:      log,
		External:    external,
		RequestPool: pool,
	}
	n, _ := newNode(conf)

	go func() {
		ccRemove := &pb.ConfChange{
			NodeID:  3,
			Type:    pb.ConfChangeType_ConfChangeRemoveNode,
			Context: []byte("blank"),
		}
		deleteEvent := &LocalEvent{
			Service:   NodeMgrService,
			EventType: NodeMgrDelNodeEvent,
			Event:     uint64(3),
		}
		_ = n.ProposeConfChange(ccRemove)
		obj := <-n.rbft.recvChan
		assert.Equal(t, deleteEvent, obj)
	}()

	ccDefault := &pb.ConfChange{
		NodeID:  3,
		Type:    pb.ConfChangeType_ConfChangeAddNode,
		Context: []byte("blank"),
	}
	assert.Equal(t, errors.New("invalid confChange propose"), n.ProposeConfChange(ccDefault))
}

func TestNode_Step(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	pool := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:                      2,
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
		UpdateTimeout:           4 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger:      log,
		External:    external,
		RequestPool: pool,
	}
	n, _ := newNode(conf)

	// Post a Type_NULL_REQUEST Msg
	// Type/Payload
	nullRequest := &pb.NullRequest{
		ReplicaId: n.rbft.peerPool.localID,
	}
	payload, _ := proto.Marshal(nullRequest)
	msgTmp := &pb.ConsensusMessage{
		Type:    pb.Type_NULL_REQUEST,
		Payload: payload,
	}
	go func() {
		n.Step(msgTmp)
		obj := <-n.rbft.recvChan
		assert.Equal(t, msgTmp, obj)
	}()
}

func TestNode_ApplyConfChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	pool := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:                      2,
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
		UpdateTimeout:           4 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger:      log,
		External:    external,
		RequestPool: pool,
	}
	n, _ := newNode(conf)

	r := &pb.Router{Peers: peerSet}
	cc := &pb.ConfState{QuorumRouter: r}
	n.ApplyConfChange(cc)
	assert.Equal(t, len(peerSet), len(n.rbft.peerPool.router.Peers))
}

func TestNode_ReportExecuted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	pool := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:                      2,
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
		UpdateTimeout:           4 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger:      log,
		External:    external,
		RequestPool: pool,
	}
	n, _ := newNode(conf)

	state1 := &pb.ServiceState{
		Applied: 2,
		Digest:  "msg",
	}

	n.currentState = nil
	n.ReportExecuted(state1)
	assert.Equal(t, state1, n.currentState)

	state2 := &pb.ServiceState{
		Applied: 2,
		Digest:  "test",
	}
	n.ReportExecuted(state2)
	assert.Equal(t, "msg", n.currentState.Digest)

	state3 := &pb.ServiceState{
		Applied: 5,
		Digest:  "test",
	}
	n.ReportExecuted(state3)
	assert.Equal(t, "test", n.currentState.Digest)

	state4 := &pb.ServiceState{
		Applied: 40,
		Digest:  "test",
	}
	go func() {
		n.ReportExecuted(state4)
		obj := <-n.cpChan
		assert.Equal(t, state4, obj)
	}()
}

func TestNode_ReportStateUpdated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	pool := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:                      2,
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
		UpdateTimeout:           4 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger:      log,
		External:    external,
		RequestPool: pool,
	}
	n, _ := newNode(conf)

	state := &pb.ServiceState{
		Applied: 2,
		Digest:  "state1",
	}

	state2 := &pb.ServiceState{
		Applied: 2,
		Digest:  "state2",
	}

	n.currentState = state
	n.ReportStateUpdated(state2)
	assert.Equal(t, "state2", n.currentState.Digest)

	state3 := &pb.ServiceState{
		Applied: 4,
		Digest:  "state3",
	}
	n.ReportStateUpdated(state3)
	assert.Equal(t, "state3", n.currentState.Digest)
}

func TestNode_Status(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	pool := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:                      2,
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
		UpdateTimeout:           4 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger:      log,
		External:    external,
		RequestPool: pool,
	}
	n, _ := NewNode(conf)
	_ = n.Start()

	// When started, the node need recovery process
	// So that here is a InRecovery Status
	go func() {
		ret := n.Status()
		expStatus := NodeStatus{
			ID:     2,
			View:   1,
			Status: InRecovery,
		}
		assert.Equal(t, expStatus, ret)
	}()
}

func TestNode_getCurrentState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	pool := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:                      2,
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
		UpdateTimeout:           4 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,

		Logger:      log,
		External:    external,
		RequestPool: pool,
	}
	n, _ := newNode(conf)
	n.currentState = &pb.ServiceState{
		Applied: 4,
		Digest:  "test",
	}

	expState := &pb.ServiceState{
		Applied: 4,
		Digest:  "test",
	}
	assert.Equal(t, expState, n.getCurrentState())
}
