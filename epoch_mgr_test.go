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
		ReplicaHash:    calHash("node5"),
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
		ReplicaHash:    calHash("node2"),
		SequenceNumber: uint64(12),
	}
	signedC := &pb.SignedCheckpoint{
		NodeInfo: rbfts[0].getNodeInfo(),
		Checkpoint: &protos.Checkpoint{
			Epoch:   rbfts[0].epoch,
			Height:  uint64(12),
			Digest:  "block-number-12",
			NextSet: nil,
		},
		Signature: nil,
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
		ReplicaHash:    calHash("node2"),
		SequenceNumber: uint64(12),
	}
	rbfts[0].h = uint64(50)
	signedC := &pb.SignedCheckpoint{
		NodeInfo: rbfts[0].getNodeInfo(),
		Checkpoint: &protos.Checkpoint{
			Epoch:   rbfts[0].epoch,
			Height:  uint64(50),
			Digest:  "block-number-50",
			NextSet: nil,
		},
		Signature: nil,
	}
	rbfts[0].storeMgr.saveCheckpoint(uint64(50), signedC)
	consensusMsg := rbfts[0].consensusMessagePacker(signedC)
	ret := rbfts[0].recvFetchCheckpoint(fetch)
	assert.Nil(t, ret)
	assert.Equal(t, consensusMsg, nodes[0].unicastMessageCache)
}

func TestEpoch_checkIfOutOfEpoch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nodes, rbfts := newBasicClusterInstance()
	rbfts[3].on(isNewNode)
	conFrom1 := &pb.ConsensusMessage{
		From:  uint64(1),
		Epoch: uint64(12),
	}
	conFrom2 := &pb.ConsensusMessage{
		From:  uint64(2),
		Epoch: uint64(12),
	}
	conFrom3 := &pb.ConsensusMessage{
		From:  uint64(3),
		Epoch: uint64(12),
	}

	rbfts[3].checkIfOutOfEpoch(conFrom1)
	rbfts[3].checkIfOutOfEpoch(conFrom2)
	rbfts[3].checkIfOutOfEpoch(conFrom3)
	assert.Nil(t, nodes[3].broadcastMessageCache)

	rbfts[3].off(isNewNode)
	rbfts[3].checkIfOutOfEpoch(conFrom1)
	rbfts[3].checkIfOutOfEpoch(conFrom2)
	rbfts[3].checkIfOutOfEpoch(conFrom3)
	assert.Equal(t, pb.Type_NOTIFICATION, nodes[3].broadcastMessageCache.Type)
}

func TestEpoch_turnIntoEpoch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, rbfts := newBasicClusterInstance()

	addNode5 := append(defaultValidatorSet, &protos.NodeInfo{Hostname: "node5", PubKey: "pub-5"})
	router := vSetToRouters(addNode5)

	rbfts[0].turnIntoEpoch(&router, uint64(8))

	assert.Equal(t, uint64(8), rbfts[0].epoch)
	assert.Equal(t, 5, rbfts[0].N)
	assert.Equal(t, 5, len(rbfts[0].peerPool.routerMap.HashMap))
}
