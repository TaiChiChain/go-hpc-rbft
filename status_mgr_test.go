package rbft

import (
	"testing"

	"github.com/ultramesh/flato-common/metrics/disabled"
	mockexternal "github.com/ultramesh/flato-rbft/mock/mock_external"
	"github.com/ultramesh/flato-rbft/types"
	txpoolmock "github.com/ultramesh/flato-txpool/mock"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func newTestStatusNode(ctrl *gomock.Controller) *rbftImpl {
	log := FrameworkNewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	tx := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:          1,
		Hash:        calHash("node1"),
		Peers:       peerSet,
		Logger:      log,
		External:    external,
		RequestPool: tx,
		MetricsProv: &disabled.Provider{},
		DelFlag:     make(chan bool),

		EpochInit:    uint64(0),
		LatestConfig: nil,
	}

	cpChan := make(chan *types.ServiceState)
	confC := make(chan *types.ReloadFinished)
	rbft, _ := newRBFT(cpChan, confC, conf)

	return rbft
}

func TestStatusMgr_inOne(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rbft := newTestStatusNode(ctrl)

	rbft.atomicOn(InViewChange)
	assert.Equal(t, true, rbft.atomicInOne(InViewChange, InRecovery))
}

func TestStatusMgr_setState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rbft := newTestStatusNode(ctrl)

	rbft.setNormal()
	assert.Equal(t, true, rbft.in(Normal))

	rbft.setFull()
	assert.Equal(t, true, rbft.atomicIn(PoolFull))
}

func TestStatusMgr_maybeSetNormal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rbft := newTestStatusNode(ctrl)

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
