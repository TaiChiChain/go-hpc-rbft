package rbft

import (
	"testing"

	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestPeer_newPeerPool(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// newPeerPool running in newRBFT
	// return is rbft.peerPool
	rbft, _ := newTestRBFT(ctrl)

	structName, nilElems, err := checkNilElems(rbft.peerPool)
	if err == nil {
		assert.Equal(t, "peerPool", structName)
		assert.Nil(t, nilElems)
	}
}

func TestPeer_serializeRouterInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	reInfo := &pb.Router{}
	_ = proto.Unmarshal(rbft.peerPool.serializeRouterInfo(), reInfo)
	assert.Equal(t, peerSet, reInfo.Peers)
}

func TestPeer_unSerializeRouterInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)
	reInfo := &pb.Router{}
	_ = proto.Unmarshal(rbft.peerPool.serializeRouterInfo(), reInfo)
	assert.Equal(t, reInfo, rbft.peerPool.unSerializeRouterInfo(rbft.peerPool.serializeRouterInfo()))
}

func TestPeer_isInRoutingTable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	assert.Equal(t, false, rbft.peerPool.isInRoutingTable(uint64(5)))
	assert.Equal(t, true, rbft.peerPool.isInRoutingTable(uint64(2)))
}

func TestPeer_getSelfInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	reInfo := &pb.Peer{}
	_ = proto.Unmarshal(rbft.peerPool.getSelfInfo(), reInfo)
	assert.Equal(t, rbft.peerPool.self, reInfo)
}

func TestPeer_getPeerByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	assert.Nil(t, rbft.peerPool.getPeerByID(uint64(5)))
	assert.Equal(t, rbft.peerPool.router.Peers[1], rbft.peerPool.getPeerByID(uint64(2)))
}
