package rbft

import (
	"context"
	"sync"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"

	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/types"
)

func TestNode_Start(t *testing.T) {
	_, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
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
	_, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	n := rbfts[0].node
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
			tx1 := newTx()
			tx2 := newTx()
			_ = n.Propose([]*consensus.FltTransaction{tx1, tx2}, true)
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		for i := 1; i < 100; i++ {
			con := &consensus.ConsensusMessage{}
			n.Step(context.TODO(), con)
		}
		wg.Done()
	}()
	n.Stop()
	wg.Wait()

	assert.Nil(t, n.currentState)
}

func TestNode_Propose(t *testing.T) {
	_, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	unlockCluster(rbfts)

	n := rbfts[0].node
	tx1 := newTx()
	requestsTmp := &RequestSet[consensus.FltTransaction, *consensus.FltTransaction]{
		Requests: []*consensus.FltTransaction{tx1},
		Local:    true,
	}
	_ = n.Propose([]*consensus.FltTransaction{tx1}, true)
	obj := <-n.rbft.recvChan
	assert.Equal(t, requestsTmp, obj)
}

func TestNode_Step(t *testing.T) {
	_, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	n := rbfts[0].node

	// Post a Type_NULL_REQUEST Msg
	// Type/Payload
	nullRequest := &consensus.NullRequest{
		ReplicaId: n.rbft.peerMgr.selfID,
	}
	payload, _ := proto.Marshal(nullRequest)
	msgTmp := &consensus.ConsensusMessage{
		Type:    consensus.Type_NULL_REQUEST,
		Payload: payload,
	}
	go func() {
		n.Step(context.TODO(), msgTmp)
		obj := <-n.rbft.recvChan
		assert.Equal(t, msgTmp, obj)
	}()
}

func TestNode_ReportExecuted(t *testing.T) {
	_, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
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
	_, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
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
	_, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
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
