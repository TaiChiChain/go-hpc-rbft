package rbft

import (
	"testing"

	"github.com/ultramesh/flato-common/types/protos"
	pb "github.com/ultramesh/flato-rbft/rbftpb"
	"github.com/ultramesh/flato-rbft/types"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestEpoch_fetchCheckpoint_and_recv(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()

	rbfts[0].epochMgr.configBatchToCheck = &types.MetaState{
		Height: 10,
		Digest: "test-block-10",
	}

	rbfts[0].fetchCheckpoint()
	msg := nodes[0].broadcastMessageCache
	assert.Equal(t, pb.Type_FETCH_CHECKPOINT, msg.Type)

	rbfts[1].processEvent(msg)
	msg2 := nodes[1].unicastMessageCache
	assert.Equal(t, pb.Type_SIGNED_CHECKPOINT, msg2.Type)
}

func TestEpoch_recvFetchCheckpoint_RouterNotExist(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	fetch := &pb.FetchCheckpoint{
		ReplicaHost:    "node5",
		SequenceNumber: uint64(12),
	}
	ret := rbfts[0].recvFetchCheckpoint(fetch)
	assert.Nil(t, ret)
	assert.Nil(t, nodes[0].unicastMessageCache)
}

func TestEpoch_recvFetchCheckpoint_SendBackNormal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	fetch := &pb.FetchCheckpoint{
		ReplicaHost:    "node2",
		SequenceNumber: uint64(12),
	}
	signedC := &pb.SignedCheckpoint{
		NodeInfo: rbfts[0].getNodeInfo(),
		Checkpoint: &protos.Checkpoint{
			Epoch: rbfts[0].epoch,
			ExecuteState: &protos.Checkpoint_ExecuteState{
				Height: uint64(12),
				Digest: "block-number-12",
			},
		},
	}
	rbfts[0].storeMgr.saveCheckpoint(uint64(12), signedC)
	consensusMsg := rbfts[0].consensusMessagePacker(signedC)
	ret := rbfts[0].recvFetchCheckpoint(fetch)
	assert.Nil(t, ret)
	assert.Equal(t, consensusMsg, nodes[0].unicastMessageCache)
}

func TestEpoch_recvFetchCheckpoint_SendBackStableCheckpoint(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	fetch := &pb.FetchCheckpoint{
		ReplicaHost:    "node2",
		SequenceNumber: uint64(12),
	}
	rbfts[0].h = uint64(50)
	signedC := &pb.SignedCheckpoint{
		NodeInfo: rbfts[0].getNodeInfo(),
		Checkpoint: &protos.Checkpoint{
			Epoch: rbfts[0].epoch,
			ExecuteState: &protos.Checkpoint_ExecuteState{
				Height: uint64(50),
				Digest: "block-number-50",
			},
		},
	}
	rbfts[0].storeMgr.saveCheckpoint(uint64(50), signedC)
	consensusMsg := rbfts[0].consensusMessagePacker(signedC)
	ret := rbfts[0].recvFetchCheckpoint(fetch)
	assert.Nil(t, ret)
	assert.Equal(t, consensusMsg, nodes[0].unicastMessageCache)
}
