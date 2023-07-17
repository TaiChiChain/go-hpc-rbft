package rbft

import (
	"testing"

	"github.com/hyperchain/go-hpc-rbft/v2/common/consensus"
	"github.com/hyperchain/go-hpc-rbft/v2/common/metrics/disabled"
	mockexternal "github.com/hyperchain/go-hpc-rbft/v2/mock/mock_external"
	txpoolmock "github.com/hyperchain/go-hpc-rbft/v2/txpool/mock"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func newTestStatusNode[T any, Constraint consensus.TXConstraint[T]](ctrl *gomock.Controller) *rbftImpl[T, Constraint] {
	log := newRawLogger()
	external := mockexternal.NewMockMinimalExternal[T, Constraint](ctrl)
	tx := txpoolmock.NewMockMinimalTxPool[T, Constraint](ctrl)

	conf := Config[T, Constraint]{
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

	rbft, _ := newRBFT(conf)
	return rbft
}

func TestStatusMgr_inOne(t *testing.T) {
	ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	rbft := newTestStatusNode[consensus.Transaction](ctrl)

	rbft.atomicOn(InViewChange)
	assert.Equal(t, true, rbft.atomicInOne(InViewChange, InRecovery))
}

func TestStatusMgr_setState(t *testing.T) {
	ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	rbft := newTestStatusNode[consensus.Transaction](ctrl)

	rbft.setNormal()
	assert.Equal(t, true, rbft.in(Normal))

	rbft.setFull()
	assert.Equal(t, true, rbft.atomicIn(PoolFull))
}

func TestStatusMgr_maybeSetNormal(t *testing.T) {
	ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	rbft := newTestStatusNode[consensus.Transaction](ctrl)

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
