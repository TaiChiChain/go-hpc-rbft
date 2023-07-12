package rbft

import (
	"testing"

	consensus "github.com/hyperchain/go-hpc-rbft/v2/common/consensus"
	"github.com/hyperchain/go-hpc-rbft/v2/types"

	"github.com/stretchr/testify/assert"
)

func TestEpoch_fetchCheckpoint_and_recv(t *testing.T) {
	//ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()

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
	//ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	fetch := &consensus.FetchCheckpoint{
		ReplicaId:      2,
		SequenceNumber: uint64(12),
	}
	signedC := &consensus.SignedCheckpoint{
		Author: rbfts[0].peerPool.hostname,
		Checkpoint: &consensus.Checkpoint{
			Epoch: rbfts[0].epoch,
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
	//ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance[consensus.Transaction]()
	fetch := &consensus.FetchCheckpoint{
		ReplicaId:      2,
		SequenceNumber: uint64(12),
	}
	rbfts[0].h = uint64(50)
	signedC := &consensus.SignedCheckpoint{
		Author: rbfts[0].peerPool.hostname,
		Checkpoint: &consensus.Checkpoint{
			Epoch: rbfts[0].epoch,
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
