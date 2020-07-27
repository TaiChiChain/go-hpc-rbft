package rbft

import (
	"errors"
	"testing"

	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNode_NewNode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, conf := newTestRBFTReplica(ctrl)
	n := rbft.node

	structName, nilElems, err := checkNilElems(n)
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
	rbft, _ := newTestRBFTReplica(ctrl)
	n := rbft.node
	n.rbft.atomicOn(Pending)

	n.currentState = &pb.ServiceState{
		MetaState: &pb.MetaState{
			Applied: uint64(0),
			Digest:  "GENESIS XXX",
		},
	}
	// Test Normal Case
	_ = n.Start()
	assert.Equal(t, false, n.rbft.atomicIn(Pending))
}

func TestNode_Stop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFTReplica(ctrl)
	n := rbft.node
	n.currentState = &pb.ServiceState{
		MetaState: &pb.MetaState{
			Applied: uint64(0),
			Digest:  "GENESIS XXX",
		},
	}

	n.Stop()
	assert.Nil(t, n.currentState)
}

func TestNode_Propose(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFTReplica(ctrl)
	n := rbft.node

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

func TestNode_Step(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFTReplica(ctrl)
	n := rbft.node

	// Post a Type_NULL_REQUEST Msg
	// Type/Payload
	nullRequest := &pb.NullRequest{
		ReplicaId: n.rbft.peerPool.ID,
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
	rbft, _ := newTestRBFTReplica(ctrl)
	n := rbft.node

	r := &pb.Router{Peers: peerSet}
	cc := &pb.ConfState{QuorumRouter: r}
	n.ApplyConfChange(cc)
	assert.Equal(t, len(peerSet), len(n.rbft.peerPool.routerMap.HashMap))
}

func TestNode_ReportExecuted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFTReplica(ctrl)
	n := rbft.node

	state1 := &pb.ServiceState{
		MetaState: &pb.MetaState{
			Applied: 2,
			Digest:  "msg",
		},
	}

	n.currentState = nil
	n.ReportExecuted(state1)
	assert.Equal(t, state1, n.currentState)

	state2 := &pb.ServiceState{
		MetaState: &pb.MetaState{
			Applied: 2,
			Digest:  "test",
		},
	}
	n.ReportExecuted(state2)
	assert.Equal(t, "msg", n.currentState.MetaState.Digest)

	state3 := &pb.ServiceState{
		MetaState: &pb.MetaState{
			Applied: 5,
			Digest:  "test",
		},
	}
	n.ReportExecuted(state3)
	assert.Equal(t, "test", n.currentState.MetaState.Digest)

	state4 := &pb.ServiceState{
		MetaState: &pb.MetaState{
			Applied: 40,
			Digest:  "test",
		},
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
	rbft, _ := newTestRBFTReplica(ctrl)
	n := rbft.node

	state := &pb.ServiceState{
		MetaState: &pb.MetaState{
			Applied: 2,
			Digest:  "state1",
		},
	}

	state2 := &pb.ServiceState{
		MetaState: &pb.MetaState{
			Applied: 2,
			Digest:  "state2",
		},
	}

	n.currentState = state
	n.ReportStateUpdated(state2)
	assert.Equal(t, "state2", n.currentState.MetaState.Digest)

	state3 := &pb.ServiceState{
		MetaState: &pb.MetaState{
			Applied: 4,
			Digest:  "state3",
		},
	}
	n.ReportStateUpdated(state3)
	assert.Equal(t, "state3", n.currentState.MetaState.Digest)
}

func TestNode_getCurrentState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFTReplica(ctrl)
	n := rbft.node

	n.currentState = &pb.ServiceState{
		MetaState: &pb.MetaState{
			Applied: 4,
			Digest:  "test",
		},
	}

	expState := &pb.ServiceState{
		MetaState: &pb.MetaState{
			Applied: 4,
			Digest:  "test",
		},
	}
	assert.Equal(t, expState, n.getCurrentState())
}
