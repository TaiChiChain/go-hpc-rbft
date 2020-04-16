package rbft

import (
	"testing"

	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/golang/mock/gomock"
	"github.com/magiconair/properties/assert"
)

func TestEpoch_checkOutOfEpoch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)

	msg2 := &pb.ConsensusMessage{
		From:  uint64(2),
		Epoch: uint64(99),
	}

	msg3 := &pb.ConsensusMessage{
		From:  uint64(3),
		Epoch: uint64(99),
	}

	msg4 := &pb.ConsensusMessage{
		From:  uint64(4),
		Epoch: uint64(99),
	}

	rbft.off(InEpochSync)
	rbft.checkIfOutOfEpoch(msg2)
	rbft.checkIfOutOfEpoch(msg3)
	rbft.checkIfOutOfEpoch(msg4)

	assert.Equal(t, true, rbft.in(InEpochSync))
}

func TestEpoch_initEpochCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)
	rbft.off(Pending)

	rbft.on(InEpochCheck)
	rbft.initEpochCheck()
	assert.Equal(t, true, rbft.in(InEpochCheck))
}

func TestEpoch_initEpochCheck2(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)
	rbft.off(Pending)

	rbft.initEpochCheck()
	assert.Equal(t, true, rbft.in(InEpochCheck))
}
