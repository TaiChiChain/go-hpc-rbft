package rbft

import (
	"errors"
	"testing"

	mockexternal "github.com/ultramesh/flato-rbft/mock/mock_external"
	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestExec_msgToEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	event := &pb.NullRequest{ReplicaId: uint64(2)}
	payload, _ := proto.Marshal(event)
	msg := &pb.ConsensusMessage{
		Type:    pb.Type_NULL_REQUEST,
		From:    uint64(1),
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

func TestExec_handleCoreRbftEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	e := &LocalEvent{Service: CoreRbftService}

	e.EventType = CoreBatchTimerEvent
	rbft.off(Normal)
	assert.Nil(t, rbft.handleCoreRbftEvent(e))

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
	rbft.peerPool.ID = uint64(2)
	rbft.atomicOff(InRecovery)
	rbft.atomicOff(InViewChange)
	e.EventType = CoreNullRequestTimerEvent
	assert.Equal(t, uint64(0), rbft.view)
	assert.Nil(t, rbft.handleCoreRbftEvent(e))
	assert.Equal(t, uint64(1), rbft.view)
	newView := rbft.view - uint64(1)
	rbft.setView(newView)

	// First Req to a ViewChange
	rbft.atomicOff(InViewChange)
	rbft.peerPool.ID = uint64(1)
	e.EventType = CoreFirstRequestTimerEvent
	assert.Equal(t, uint64(0), rbft.view)
	assert.Nil(t, rbft.handleCoreRbftEvent(e))
	assert.Equal(t, uint64(1), rbft.view)
	newView = rbft.view - uint64(1)
	rbft.setView(newView)

	// Trigger processOutOfDateReqs
	// Check the pool is not full
	// if !rbft.isNormal(), cannot check
	e.EventType = CoreCheckPoolTimerEvent
	rbft.atomicOn(PoolFull)
	rbft.off(Normal)
	assert.Nil(t, rbft.handleCoreRbftEvent(e))
	assert.Equal(t, true, rbft.atomicIn(PoolFull))
	// Else success
	rbft.on(Normal)
	assert.Nil(t, rbft.handleCoreRbftEvent(e))
	assert.Equal(t, false, rbft.atomicIn(PoolFull))

	// Trigger rbft.recvStateUpdatedEvent(e.Event.(uint64))
	rbft.atomicOff(InRecovery)
	e.EventType = CoreStateUpdatedEvent
	e.Event = &pb.ServiceState{
		MetaState: &pb.MetaState{
			Applied: uint64(5),
			Digest:  "block-number-5",
		},
	}
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
	rbft.atomicOff(InViewChange)
	peerTmp := []*pb.Peer{
		{
			Id:   1,
			Hash: "node1",
		},
		{
			Id:   2,
			Hash: "node2",
		},
		{
			Id:   3,
			Hash: "node3",
		},
	}
	confState := &pb.ConfState{QuorumRouter: &pb.Router{Peers: peerTmp}}
	rbft.atomicOff(Pending)
	rbft.postConfState(confState)
	assert.Equal(t, false, rbft.atomicIn(Pending))
	assert.Equal(t, 3, len(rbft.peerPool.routerMap.HashMap))

	// Not found localId, Pending the peer
	confState = &pb.ConfState{
		QuorumRouter: &pb.Router{
			Peers: []*pb.Peer{
				{
					Id:   6,
					Hash: "node6",
				},
			},
		},
	}
	rbft.atomicOff(Pending)
	rbft.postConfState(confState)
	assert.Equal(t, true, rbft.atomicIn(Pending))

	// Default
	e.EventType = ViewChangeTimerEvent
	assert.Nil(t, rbft.handleCoreRbftEvent(e))
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
	newView := rbft.view - uint64(1)
	rbft.setView(newView)

	rbft.setView(10)
	rbft.handleViewChangeEvent(e)
	assert.Equal(t, uint64(10), rbft.view)

	rbft.setView(0)
	rbft.atomicOn(InRecovery)
	rbft.atomicOn(InViewChange)
	rbft.handleViewChangeEvent(e)
	assert.Equal(t, false, rbft.atomicIn(InViewChange))

	e.EventType = ViewChangedEvent
	rbft.setView(0)
	rbft.atomicOff(InRecovery)
	rbft.atomicOn(InViewChange)
	rbft.handleViewChangeEvent(e)
	assert.Equal(t, false, rbft.atomicIn(InViewChange))

	e.EventType = ViewChangeResendTimerEvent
	rbft.atomicOff(InViewChange)
	rbft.timerMgr.tTimers[newViewTimer].isActive.Store("tag", "1")
	rbft.handleViewChangeEvent(e)
	_, flag := rbft.timerMgr.tTimers[newViewTimer].isActive.Load("tag")
	assert.Equal(t, true, flag)

	rbft.atomicOn(InViewChange)
	rbft.handleViewChangeEvent(e)
	assert.Equal(t, false, rbft.atomicIn(InViewChange))
	_, flag = rbft.timerMgr.tTimers[newViewTimer].isActive.Load("tag")
	assert.Equal(t, false, flag)

	e.EventType = CoreRbftService
	assert.Nil(t, rbft.handleViewChangeEvent(e))
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
	confC := make(chan *pb.ReloadFinished)
	rbft, _ := newRBFT(cpChan, confC, conf)

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

	e = nil
	i = rbft.dispatchMsgToService(e)
	assert.Equal(t, NotSupportService, i)

}
