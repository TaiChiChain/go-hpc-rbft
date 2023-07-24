package rbft

import (
	"testing"

	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/common/metrics/disabled"
	mempoolmock "github.com/axiomesh/axiom-bft/mempool/mock"
	mockexternal "github.com/axiomesh/axiom-bft/mock/mock_external"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func newTestStatusNode[T any, Constraint consensus.TXConstraint[T]](ctrl *gomock.Controller) *rbftImpl[T, Constraint] {
	log := newRawLogger()
	external := mockexternal.NewMockMinimalExternal[T, Constraint](ctrl)
	pool := mempoolmock.NewMockMinimalMemPool[T, Constraint](ctrl)

	conf := Config[T, Constraint]{
		ID:          1,
		Hash:        calHash("node1"),
		Peers:       peerSet,
		Logger:      log,
		External:    external,
		RequestPool: pool,
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

	rbft := newTestStatusNode[consensus.FltTransaction](ctrl)

	rbft.atomicOn(InViewChange)
	assert.Equal(t, true, rbft.atomicInOne(InViewChange, InRecovery))
}

func TestStatusMgr_setState(t *testing.T) {
	ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	rbft := newTestStatusNode[consensus.FltTransaction](ctrl)

	rbft.setNormal()
	assert.Equal(t, true, rbft.in(Normal))

	rbft.setFull()
	assert.Equal(t, true, rbft.atomicIn(PoolFull))
}

func TestStatusMgr_maybeSetNormal(t *testing.T) {
	ctrl := gomock.NewController(t)
	//defer ctrl.Finish()

	rbft := newTestStatusNode[consensus.FltTransaction](ctrl)

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
