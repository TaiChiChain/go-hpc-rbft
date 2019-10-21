package rbft

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	mockexternal "github.com/ultramesh/flato-rbft/mock/mock_external"
	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestExec_newExecutor(t *testing.T) {
	fmt.Println()
	exec := newExecutor()
	assert.Equal(t, reflect.TypeOf(&executor{}), reflect.TypeOf(exec))
}

func TestExec_setLastExec(t *testing.T) {
	exec := newExecutor()
	exec.setLastExec(uint64(98))
	assert.Equal(t, uint64(98), exec.lastExec)
}

func TestExec_setCurrentExec(t *testing.T) {
	exec := newExecutor()
	i := uint64(98)
	exec.setCurrentExec(&i)
	assert.Equal(t, &i, exec.currentExec)
}

func TestExec_msgToEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	event := &pb.NullRequest{ReplicaId: uint64(2)}
	payload, _ := proto.Marshal(event)
	msg := &pb.ConsensusMessage{
		Type:    pb.Type_NULL_REQUEST,
		From:    uint64(1),
		To:      uint64(2),
		Payload: payload,
	}

	ret, err := rbft.msgToEvent(msg)
	assert.Equal(t, event, ret)
	assert.Equal(t, nil, err)

	go func() {
		msg.Payload = []byte("1")
		ret, err = rbft.msgToEvent(msg)
		assert.Nil(t, ret)
		assert.Equal(t, errors.New("unexpected EOF"), err)
	}()
}

func TestExec_initMsgEventMap(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	rbft.initMsgEventMap()

	ee := eventCreators[pb.Type_NOTIFICATION_RESPONSE]
	assert.Equal(t, reflect.TypeOf(&pb.NotificationResponse{}), reflect.TypeOf(ee()))
}

func TestExec_dispatchConsensusMsg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	var e consensusEvent

	// Test for CoreMsg - change needSyncState Mode
	rbft.off(NeedSyncState)
	rbft.on(Normal)
	rbft.off(InRecovery)
	rbft.off(InViewChange)
	e = &pb.NullRequest{ReplicaId: uint64(1)}
	assert.Nil(t, rbft.dispatchConsensusMsg(e))
	assert.Equal(t, true, rbft.in(NeedSyncState))
	rbft.off(NeedSyncState)

	// Test for ViewChangeMsg - view++
	rbft.on(Normal)
	e = &pb.ViewChange{
		Basis:     &pb.VcBasis{ReplicaId: 1, View: 2},
		Signature: []byte("sig"),
		Timestamp: 10086,
	}
	assert.Nil(t, rbft.dispatchConsensusMsg(e))
	assert.Equal(t, uint64(1), rbft.view)
	rbft.off(InViewChange)
	rbft.view--

	// Test for NodeMgrMsg of recv - change rbft.nodeMgr.agreeUpdateStore
	rbft.off(InRecovery)
	rbft.off(InViewChange)
	e = &pb.AgreeUpdateN{
		Basis:   &pb.VcBasis{ReplicaId: 1, View: 5},
		Flag:    true,
		Id:      6,
		Info:    "call me",
		N:       5,
		ExpectN: 2,
	}
	key := aidx{
		v:    5,
		n:    5,
		flag: true,
		id:   1,
	}
	assert.Nil(t, rbft.dispatchConsensusMsg(e))
	assert.Equal(t, "call me", rbft.nodeMgr.agreeUpdateStore[key].Info)

	// Test for RecoveryMsg - change for rbft.recoveryMgr.notificationStore[NID]
	e = &pb.Notification{
		Basis:     &pb.VcBasis{ReplicaId: 10, View: 10},
		Signature: []byte("sig"),
		Timestamp: 10086,
		ReplicaId: 10,
	}
	NID := ntfIdx{
		v:      10,
		nodeID: 10,
	}
	assert.Nil(t, rbft.dispatchConsensusMsg(e))
	assert.Equal(t, e, rbft.recoveryMgr.notificationStore[NID])

	e = nil
	assert.Nil(t, rbft.dispatchConsensusMsg(e))
}

func TestExec_dispatchLocalEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	e := &LocalEvent{}

	// Send Core LocalEvent - stop batchTimer
	e.Service = CoreRbftService
	e.EventType = CoreBatchTimerEvent
	rbft.on(Normal)
	rbft.batchMgr.batchTimerActive = true
	assert.Nil(t, rbft.dispatchLocalEvent(e))
	assert.Equal(t, false, rbft.batchMgr.batchTimerActive)

	// Send View Change LocalEvent - sendViewChange
	e.Service = ViewChangeService
	e.EventType = ViewChangeTimerEvent
	assert.Equal(t, uint64(0), rbft.view)
	assert.Nil(t, rbft.dispatchLocalEvent(e))
	assert.Equal(t, uint64(1), rbft.view)
	rbft.view--

	// Send NodeMgr Msg - rbft.nodeMgr.updateHandled from true to false
	e.Service = NodeMgrService
	e.EventType = NodeMgrUpdatedEvent
	rbft.nodeMgr.updateHandled = true
	assert.Nil(t, rbft.dispatchLocalEvent(e))
	assert.Equal(t, false, rbft.nodeMgr.updateHandled)

	// Send RecoveryMsg - close recovery process
	e.Service = RecoveryService
	e.EventType = RecoveryDoneEvent
	rbft.on(InRecovery)
	assert.Nil(t, rbft.dispatchLocalEvent(e))
	assert.Equal(t, false, rbft.in(InRecovery))

	// Default
	e.Service = NotSupportService
	assert.Nil(t, rbft.dispatchLocalEvent(e))
}

func TestExec_handleCoreRbftEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	e := &LocalEvent{Service: CoreRbftService}

	e.EventType = CoreBatchTimerEvent
	// if !rbft.isNormal() - finish
	rbft.off(Normal)
	assert.Nil(t, rbft.handleCoreRbftEvent(e))

	// if len(rbft.batchMgr.cacheBatch) > 0, run rbft.maybeSendPrePrepare
	// change rbft.storeMgr.batchStore["hashMsg"]
	rbft.on(Normal)
	batchTmp := &pb.RequestBatch{
		RequestHashList: []string{"hash"},
		RequestList:     mockRequestList,
		Timestamp:       10086,
		SeqNo:           5,
		LocalList:       []bool{false},
		BatchHash:       "hashMsg",
	}
	rbft.batchMgr.cacheBatch = []*pb.RequestBatch{batchTmp}
	assert.Nil(t, rbft.handleCoreRbftEvent(e))
	assert.Equal(t, batchTmp, rbft.storeMgr.batchStore["hashMsg"])

	// rbft.batchMgr.cacheBatch is empty
	// stop batch timer
	rbft.batchMgr.batchTimerActive = true
	rbft.batchMgr.cacheBatch = []*pb.RequestBatch{}
	assert.Nil(t, rbft.handleCoreRbftEvent(e))
	assert.Equal(t, false, rbft.batchMgr.batchTimerActive)

	// Replica handle Null request to a viewChange
	rbft.peerPool.localID = uint64(2)
	rbft.off(InRecovery)
	rbft.off(InViewChange)
	e.EventType = CoreNullRequestTimerEvent
	assert.Equal(t, uint64(0), rbft.view)
	assert.Nil(t, rbft.handleCoreRbftEvent(e))
	assert.Equal(t, uint64(1), rbft.view)
	rbft.view--

	// First Req to a ViewChange
	rbft.peerPool.localID = uint64(1)
	e.EventType = CoreFirstRequestTimerEvent
	assert.Equal(t, uint64(0), rbft.view)
	assert.Nil(t, rbft.handleCoreRbftEvent(e))
	assert.Equal(t, uint64(1), rbft.view)
	rbft.view--

	// Trigger processOutOfDateReqs
	// Check the pool is not full
	// if !rbft.isNormal(), cannot check
	e.EventType = CoreCheckPoolTimerEvent
	rbft.on(PoolFull)
	rbft.off(Normal)
	assert.Nil(t, rbft.handleCoreRbftEvent(e))
	assert.Equal(t, true, rbft.in(PoolFull))
	// Else success
	rbft.on(Normal)
	assert.Nil(t, rbft.handleCoreRbftEvent(e))
	assert.Equal(t, false, rbft.in(PoolFull))

	// Trigger rbft.recvStateUpdatedEvent(e.Event.(uint64))
	rbft.off(InRecovery)
	e.EventType = CoreStateUpdatedEvent
	e.Event = uint64(5)
	rbft.exec.setLastExec(uint64(3))
	assert.Nil(t, rbft.handleCoreRbftEvent(e))
	assert.Equal(t, uint64(5), rbft.exec.lastExec)

	e.EventType = CoreResendMissingTxsEvent
	e.Event = &pb.FetchMissingRequests{}
	assert.Nil(t, rbft.handleCoreRbftEvent(e))

	e.EventType = CoreResendFetchMissingEvent
	assert.Nil(t, rbft.handleCoreRbftEvent(e))

	// as for update conf msg
	// found localId, initPeers to refresh the peerPool
	rbft.off(InViewChange)
	e.Event = nil
	e.EventType = CoreUpdateConfStateEvent
	peerTmp := []*pb.Peer{
		{
			Id:      1,
			Context: []byte("Test1"),
		},
		{
			Id:      5,
			Context: []byte("Test5"),
		},
		{
			Id:      6,
			Context: []byte("Test6"),
		},
	}
	e.Event = &pb.ConfState{QuorumRouter: &pb.Router{Peers: peerTmp}}
	rbft.off(Pending)
	rbft.handleCoreRbftEvent(e)
	assert.Equal(t, false, rbft.in(Pending))
	assert.Equal(t, peerTmp, rbft.peerPool.router.Peers)

	// Not found localId, Pending the peer
	e.Event = &pb.ConfState{
		QuorumRouter: &pb.Router{
			Peers: []*pb.Peer{
				{
					Id:      6,
					Context: []byte("NOT EXIST"),
				},
			},
		},
	}
	rbft.off(Pending)
	rbft.handleCoreRbftEvent(e)
	assert.Equal(t, true, rbft.in(Pending))

	// Default
	e.EventType = ViewChangeTimerEvent
	assert.Nil(t, rbft.handleCoreRbftEvent(e))

	go func() {
		rbft.on(InViewChange)
		e.EventType = CoreRetrieveStatusEvent
		stateChan := make(chan NodeStatus)
		e.Event = stateChan
		assert.Nil(t, rbft.handleCoreRbftEvent(e))
		obj := <-stateChan
		assert.Equal(t, InViewChange, obj)
	}()
}

func TestExec_handleRecoveryEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	e := &LocalEvent{}

	e.EventType = RecoveryDoneEvent
	rbft.on(InRecovery)
	assert.Nil(t, rbft.handleRecoveryEvent(e))
	assert.Equal(t, false, rbft.in(InRecovery))
	assert.Equal(t, false, rbft.recoveryMgr.recoveryHandled)

	rbft.on(InRecovery)
	rbft.peerPool.localID = 1
	rbft.timerMgr.tTimers[nullRequestTimer].isActive.Store("tag", "1")
	assert.Nil(t, rbft.handleRecoveryEvent(e))
	_, flag := rbft.timerMgr.tTimers[nullRequestTimer].isActive.Load("tag")
	assert.Equal(t, false, flag)

	rbft.peerPool.localID = 4
	rbft.on(InRecovery)
	rbft.on(isNewNode)
	rbft.off(InUpdatingN)
	assert.Nil(t, rbft.handleRecoveryEvent(e))
	assert.Equal(t, true, rbft.in(InUpdatingN))
	rbft.peerPool.localID = 2

	e.EventType = RecoveryRestartTimerEvent
	rbft.recoveryMgr.recoveryHandled = true
	assert.Nil(t, rbft.handleRecoveryEvent(e))
	assert.Equal(t, false, rbft.recoveryMgr.recoveryHandled)

	e.EventType = RecoverySyncStateRspTimerEvent
	rbft.on(InSyncState)
	rbft.on(Normal)
	assert.Nil(t, rbft.handleRecoveryEvent(e))
	assert.Equal(t, false, rbft.in(InSyncState))

	e.EventType = RecoverySyncStateRestartTimerEvent
	rbft.recoveryMgr.syncRspStore = make(map[uint64]*pb.SyncStateResponse)
	rbft.recoveryMgr.syncRspStore[uint64(1)] = &pb.SyncStateResponse{ReplicaId: 5}
	assert.Nil(t, rbft.handleRecoveryEvent(e))
	assert.Nil(t, rbft.recoveryMgr.syncRspStore[uint64(1)])

	e.EventType = NotificationQuorumEvent
	assert.Nil(t, rbft.handleRecoveryEvent(e))
	rbft.peerPool.localID = 1
	rbft.on(SkipInProgress)
	assert.Nil(t, rbft.handleRecoveryEvent(e))
	rbft.off(SkipInProgress)
	assert.Nil(t, rbft.handleRecoveryEvent(e))

	e.EventType = CoreRbftService
	assert.Nil(t, rbft.handleRecoveryEvent(e))
}

func TestExec_handleViewChangeEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	e := &LocalEvent{}

	e.EventType = ViewChangeTimerEvent
	e.Event = nextDemandNewView(uint64(1))
	rbft.handleViewChangeEvent(e)
	assert.Equal(t, uint64(1), rbft.view)
	rbft.view--

	rbft.view = 10
	rbft.handleViewChangeEvent(e)
	assert.Equal(t, uint64(10), rbft.view)

	rbft.view = 0
	rbft.on(InRecovery)
	rbft.on(InViewChange)
	rbft.handleViewChangeEvent(e)
	assert.Equal(t, false, rbft.in(InViewChange))

	e.EventType = ViewChangedEvent
	rbft.view = 0
	rbft.off(InRecovery)
	rbft.on(InViewChange)
	rbft.handleViewChangeEvent(e)
	assert.Equal(t, false, rbft.in(InViewChange))

	rbft.on(isNewNode)
	rbft.off(InUpdatingN)
	rbft.handleViewChangeEvent(e)
	assert.Equal(t, true, rbft.in(InUpdatingN))

	e.EventType = ViewChangeResendTimerEvent
	rbft.off(InViewChange)
	rbft.timerMgr.tTimers[newViewTimer].isActive.Store("tag", "1")
	rbft.handleViewChangeEvent(e)
	_, flag := rbft.timerMgr.tTimers[newViewTimer].isActive.Load("tag")
	assert.Equal(t, true, flag)

	rbft.on(InViewChange)
	rbft.handleViewChangeEvent(e)
	assert.Equal(t, false, rbft.in(InViewChange))
	_, flag = rbft.timerMgr.tTimers[newViewTimer].isActive.Load("tag")
	assert.Equal(t, false, flag)

	e.EventType = CoreRbftService
	assert.Nil(t, rbft.handleViewChangeEvent(e))
}

func TestExec_handleNodeMgrEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	e := &LocalEvent{}

	e.EventType = NodeMgrUpdatedEvent
	rbft.nodeMgr.updateHandled = true
	rbft.handleNodeMgrEvent(e)
	assert.Equal(t, false, rbft.nodeMgr.updateHandled)

	e.EventType = NodeMgrUpdatedEvent
	rbft.on(isNewNode)
	rbft.handleNodeMgrEvent(e)
	assert.Equal(t, false, rbft.in(isNewNode))

	e.EventType = NodeMgrUpdateTimerEvent
	rbft.on(InUpdatingN)
	assert.Nil(t, rbft.handleNodeMgrEvent(e))
	assert.Equal(t, false, rbft.in(InUpdatingN))
}

func TestExec_dispatchMsgToService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)

	conf := Config{
		ID:       1,
		IsNew:    false,
		Peers:    peerSet,
		Logger:   log,
		External: external,
	}

	cpChan := make(chan *pb.ServiceState)
	rbft, _ := newRBFT(cpChan, conf)

	var e consensusEvent
	var i int

	e = &pb.NullRequest{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, CoreRbftService, i)

	e = &pb.PrePrepare{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, CoreRbftService, i)

	e = &pb.Prepare{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, CoreRbftService, i)

	e = &pb.Commit{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, CoreRbftService, i)

	e = &pb.Checkpoint{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, CoreRbftService, i)

	e = &pb.FetchMissingRequests{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, CoreRbftService, i)

	e = &pb.SendMissingRequests{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, CoreRbftService, i)

	e = &pb.ViewChange{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, ViewChangeService, i)

	e = &pb.NewView{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, ViewChangeService, i)

	e = &pb.FetchRequestBatch{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, ViewChangeService, i)

	e = &pb.SendRequestBatch{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, ViewChangeService, i)

	e = &pb.RecoveryFetchPQC{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, RecoveryService, i)

	e = &pb.RecoveryReturnPQC{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, RecoveryService, i)

	e = &pb.SyncState{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, RecoveryService, i)

	e = &pb.SyncStateResponse{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, RecoveryService, i)

	e = &pb.Notification{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, RecoveryService, i)

	e = &pb.NotificationResponse{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, RecoveryService, i)

	e = &pb.ReadyForN{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, NodeMgrService, i)

	e = &pb.UpdateN{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, NodeMgrService, i)

	e = &pb.AgreeUpdateN{}
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, NodeMgrService, i)

	e = nil
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, NotSupportService, i)

}
