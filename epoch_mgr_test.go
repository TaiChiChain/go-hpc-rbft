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

	rbft.atomicOff(InEpochSync)
	rbft.checkIfOutOfEpoch(msg2)
	rbft.checkIfOutOfEpoch(msg3)
	rbft.checkIfOutOfEpoch(msg4)

	assert.Equal(t, true, rbft.atomicIn(InEpochSync))
}

func TestEpoch_initEpochCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)
	rbft.atomicOff(Pending)

	rbft.on(InEpochCheck)
	rbft.initEpochCheck(uint64(0))
	assert.Equal(t, true, rbft.in(InEpochCheck))
}

func TestEpoch_initEpochCheck2(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)
	rbft.atomicOff(Pending)

	rbft.atomicOn(InEpochSync)
	rbft.initEpochCheck(uint64(0))
	assert.Equal(t, true, rbft.in(InEpochCheck))
}

func TestEpoch_tryEpochSync(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	rbft, _ := newTestRBFT(ctrl)
	rbft.atomicOff(Pending)

	rbft.atomicOff(InEpochSync)
	rbft.atomicOn(InConfChange)
	rbft.atomicOn(InRecovery)
	assert.Equal(t, nil, rbft.tryEpochSync())
	assert.Equal(t, true, rbft.atomicIn(InRecovery))

	rbft.atomicOff(InConfChange)
	rbft.atomicOn(InEpochSync)
	assert.Equal(t, nil, rbft.tryEpochSync())
	assert.Equal(t, true, rbft.atomicIn(InRecovery))

	rbft.atomicOff(InEpochSync)
	rbft.atomicOn(InViewChange)
	rbft.on(InEpochCheck)
	rbft.tryEpochSync()
	assert.Equal(t, false, rbft.atomicIn(InRecovery))
	assert.Equal(t, false, rbft.atomicIn(InViewChange))
	assert.Equal(t, false, rbft.in(InEpochCheck))
	assert.Equal(t, true, rbft.atomicIn(InEpochSync))

	rbft.atomicOn(InConfChange)
	rbft.atomicOn(InEpochSync)
	rbft.restartEpochSync()
	assert.Equal(t, false, rbft.atomicIn(InEpochSync))
}
