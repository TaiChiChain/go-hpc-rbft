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
