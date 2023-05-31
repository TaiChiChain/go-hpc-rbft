package rbft

import (
	"sync"
	"testing"

	"github.com/hyperchain/go-hpc-common/types/protos"
	pb "github.com/hyperchain/go-hpc-rbft/rbftpb"
	"github.com/hyperchain/go-hpc-rbft/types"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNode_Start(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()
	n := rbfts[0].node
	n.rbft.atomicOn(Pending)

	n.currentState = &types.ServiceState{
		MetaState: &types.MetaState{
			Height: uint64(0),
			Digest: "XXX GENESIS",
		},
	}
	// Test Normal Case
	_ = n.Start()
	assert.Equal(t, false, n.rbft.atomicIn(Pending))
}

func TestNode_Stop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()
	n := rbfts[0].node
	r := rbfts[0]
	n.currentState = &types.ServiceState{
		MetaState: &types.MetaState{
			Height: uint64(0),
			Digest: "XXX GENESIS",
		},
	}
	_ = n.Start()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		for i := 1; i < 100; i++ {
			txs := &pb.RequestSet{
				Requests: []*protos.Transaction{newTx(), newTx()},
				Local:    true,
			}
			_ = n.Propose(txs)
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		for i := 1; i < 100; i++ {
			con := &pb.ConsensusMessage{}
			n.Step(con)
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		<-r.cpChan
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		<-r.confChan
		wg.Done()
	}()

	n.Stop()
	wg.Wait()

	assert.Nil(t, n.currentState)
}

func TestNode_Propose(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()
	unlockCluster(rbfts)

	n := rbfts[0].node
	requestsTmp := &pb.RequestSet{
		Requests: []*protos.Transaction{newTx()},
		Local:    true,
	}
	_ = n.Propose(requestsTmp)
	obj := <-n.rbft.recvChan
	assert.Equal(t, requestsTmp, obj)
}

func TestNode_Step(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()
	n := rbfts[0].node

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

	_, rbfts := newBasicClusterInstance()
	n := rbfts[0].node

	r := &types.Router{Peers: peerSet}
	cc := &types.ConfState{QuorumRouter: r}
	n.ApplyConfChange(cc)
	assert.Equal(t, len(peerSet), len(n.rbft.peerPool.routerMap.HostMap))
}

func TestNode_ReportExecuted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()
	n := rbfts[0].node

	state1 := &types.ServiceState{
		MetaState: &types.MetaState{
			Height: 2,
			Digest: "block-hash-2",
		},
	}

	n.currentState = nil
	n.ReportExecuted(state1)
	assert.Equal(t, state1, n.currentState)

	state2 := &types.ServiceState{
		MetaState: &types.MetaState{
			Height: 2,
			Digest: "block-hash-2",
		},
	}
	n.ReportExecuted(state2)
	assert.Equal(t, "block-hash-2", n.currentState.MetaState.Digest)

	state3 := &types.ServiceState{
		MetaState: &types.MetaState{
			Height: 5,
			Digest: "block-hash-5",
		},
	}
	n.ReportExecuted(state3)
	assert.Equal(t, "block-hash-5", n.currentState.MetaState.Digest)

	state4 := &types.ServiceState{
		MetaState: &types.MetaState{
			Height: 40,
			Digest: "block-hash-40",
		},
	}
	go func() {
		n.ReportExecuted(state4)
		obj := <-n.rbft.cpChan
		assert.Equal(t, state4, obj)
	}()
}

func TestNode_ReportStateUpdated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()
	n := rbfts[0].node

	state := &types.ServiceState{
		MetaState: &types.MetaState{
			Height: 2,
			Digest: "state1",
		},
	}

	state2 := &types.ServiceState{
		MetaState: &types.MetaState{
			Height: 2,
			Digest: "state2",
		},
	}

	n.currentState = state
	n.ReportStateUpdated(state2)
	assert.Equal(t, "state2", n.currentState.MetaState.Digest)

	state3 := &types.ServiceState{
		MetaState: &types.MetaState{
			Height: 4,
			Digest: "state3",
		},
	}
	n.ReportStateUpdated(state3)
	assert.Equal(t, "state3", n.currentState.MetaState.Digest)
}

func TestNode_getCurrentState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()
	n := rbfts[0].node

	n.currentState = &types.ServiceState{
		MetaState: &types.MetaState{
			Height: 4,
			Digest: "test",
		},
	}

	expState := &types.ServiceState{
		MetaState: &types.MetaState{
			Height: 4,
			Digest: "test",
		},
	}
	assert.Equal(t, expState, n.getCurrentState())
}
