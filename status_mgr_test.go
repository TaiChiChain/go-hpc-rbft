package rbft

import (
	"testing"

	"github.com/ultramesh/flato-common/metrics/disabled"
	mockexternal "github.com/ultramesh/flato-rbft/mock/mock_external"
	pb "github.com/ultramesh/flato-rbft/rbftpb"
	txpoolmock "github.com/ultramesh/flato-txpool/mock"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestStatusMgr_inOne(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log := FrameworkNewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	tx := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:          1,
		Hash:        calHash("node1"),
		IsNew:       false,
		Peers:       peerSet,
		Logger:      log,
		External:    external,
		RequestPool: tx,
		MetricsProv: &disabled.Provider{},

		EpochInit:    uint64(0),
		LatestConfig: nil,
	}

	cpChan := make(chan *pb.ServiceState)
	confC := make(chan *pb.ReloadFinished)
	rbft, _ := newRBFT(cpChan, confC, conf)

	rbft.atomicOn(InViewChange)
	assert.Equal(t, true, rbft.atomicInOne(InViewChange, InRecovery))
}

func TestStatusMgr_setState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log := FrameworkNewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	tx := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:          1,
		Hash:        calHash("node1"),
		IsNew:       false,
		Peers:       peerSet,
		Logger:      log,
		External:    external,
		RequestPool: tx,
		MetricsProv: &disabled.Provider{},

		EpochInit:    uint64(0),
		LatestConfig: nil,
	}

	cpChan := make(chan *pb.ServiceState)
	confC := make(chan *pb.ReloadFinished)
	rbft, _ := newRBFT(cpChan, confC, conf)

	rbft.setNormal()
	assert.Equal(t, true, rbft.in(Normal))

	rbft.setFull()
	assert.Equal(t, true, rbft.atomicIn(PoolFull))
}

func TestStatusMgr_maybeSetNormal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	log := FrameworkNewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	tx := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:          1,
		Hash:        calHash("node1"),
		IsNew:       false,
		Peers:       peerSet,
		Logger:      log,
		External:    external,
		RequestPool: tx,
		MetricsProv: &disabled.Provider{},

		EpochInit:    uint64(0),
		LatestConfig: nil,
	}

	cpChan := make(chan *pb.ServiceState)
	confC := make(chan *pb.ReloadFinished)
	rbft, _ := newRBFT(cpChan, confC, conf)

	rbft.atomicOff(InRecovery)
	rbft.atomicOff(InConfChange)
	rbft.atomicOff(InViewChange)
	rbft.atomicOff(StateTransferring)
	rbft.atomicOff(Pending)
	rbft.maybeSetNormal()
	assert.Equal(t, true, rbft.in(Normal))

	rbft.atomicOn(InRecovery)
	rbft.maybeSetNormal()
	assert.Equal(t, true, rbft.in(Normal))
}
