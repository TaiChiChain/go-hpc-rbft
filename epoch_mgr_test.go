package rbft

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/types"
)

func TestEpoch_fetchCheckpoint_and_recv(t *testing.T) {
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()

	rbfts[0].epochMgr.configBatchToCheck = &types.MetaState{
		Height: 10,
		Digest: "test-block-10",
	}

	rbfts[0].fetchCheckpoint()
	msg := nodes[0].broadcastMessageCache
	assert.Equal(t, consensus.Type_FETCH_CHECKPOINT, msg.Type)

	rbfts[1].processEvent(msg)
	msg2 := nodes[1].unicastMessageCache
	assert.Equal(t, consensus.Type_SIGNED_CHECKPOINT, msg2.Type)
}

func TestEpoch_recvFetchCheckpoint_SendBackNormal(t *testing.T) {
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	fetch := &consensus.FetchCheckpoint{
		ReplicaId:      2,
		SequenceNumber: uint64(12),
	}
	signedC := &consensus.SignedCheckpoint{
		Author: rbfts[0].peerMgr.selfID,
		Checkpoint: &consensus.Checkpoint{
			Epoch: rbfts[0].chainConfig.EpochInfo.Epoch,
			ExecuteState: &consensus.Checkpoint_ExecuteState{
				Height: uint64(12),
				Digest: "block-number-12",
			},
		},
	}
	rbfts[0].storeMgr.saveCheckpoint(uint64(12), signedC)
	consensusMsg := rbfts[0].consensusMessagePacker(signedC)
	ret := rbfts[0].recvFetchCheckpoint(fetch)
	assert.Nil(t, ret)
	assert.Equal(t, consensusMsg, nodes[0].unicastMessageCache.ConsensusMessage)
}

func TestEpoch_recvFetchCheckpoint_SendBackStableCheckpoint(t *testing.T) {
	nodes, rbfts := newBasicClusterInstance[consensus.FltTransaction, *consensus.FltTransaction]()
	fetch := &consensus.FetchCheckpoint{
		ReplicaId:      2,
		SequenceNumber: uint64(12),
	}
	rbfts[0].chainConfig.H = uint64(50)
	signedC := &consensus.SignedCheckpoint{
		Author: rbfts[0].peerMgr.selfID,
		Checkpoint: &consensus.Checkpoint{
			Epoch: rbfts[0].chainConfig.EpochInfo.Epoch,
			ExecuteState: &consensus.Checkpoint_ExecuteState{
				Height: uint64(50),
				Digest: "block-number-50",
			},
		},
	}
	rbfts[0].storeMgr.saveCheckpoint(uint64(50), signedC)
	consensusMsg := rbfts[0].consensusMessagePacker(signedC)
	ret := rbfts[0].recvFetchCheckpoint(fetch)
	assert.Nil(t, ret)
	assert.Equal(t, consensusMsg, nodes[0].unicastMessageCache.ConsensusMessage)
}
