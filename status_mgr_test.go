package rbft

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/common/metrics/disabled"
	"github.com/axiomesh/axiom-bft/mempool"
)

func newTestStatusNode[T any, Constraint consensus.TXConstraint[T]](ctrl *gomock.Controller) *rbftImpl[T, Constraint] {
	log := newRawLogger()
	external := NewMockMinimalExternal[T, Constraint](ctrl)
	pool := mempool.NewMockMinimalMemPool[T, Constraint](ctrl)
	conf := Config{
		SelfAccountAddress: "node1",
		GenesisEpochInfo: &EpochInfo{
			Version:                   1,
			Epoch:                     1,
			EpochPeriod:               1000,
			CandidateSet:              []*NodeInfo{},
			ValidatorSet:              peerSet,
			StartBlock:                1,
			P2PBootstrapNodeAddresses: []string{},
			ConsensusParams: &ConsensusParams{
				CheckpointPeriod:              10,
				HighWatermarkCheckpointPeriod: 4,
				MaxValidatorNum:               10,
				BlockMaxTxNum:                 500,
				NotActiveWeight:               1,
				ExcludeView:                   10,
			},
		},
		Logger:      log,
		MetricsProv: &disabled.Provider{},
		DelFlag:     make(chan bool),
	}

	rbft, err := newRBFT[T, Constraint](conf, external, pool)
	if err != nil {
		panic(err)
	}
	err = rbft.init()
	if err != nil {
		panic(err)
	}
	return rbft
}

func TestStatusMgr_inOne(t *testing.T) {
	ctrl := gomock.NewController(t)
	// defer ctrl.Finish()

	rbft := newTestStatusNode[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	rbft.atomicOn(InViewChange)
	assert.Equal(t, true, rbft.atomicInOne(InViewChange, InRecovery))
}

func TestStatusMgr_setState(t *testing.T) {
	ctrl := gomock.NewController(t)
	// defer ctrl.Finish()

	rbft := newTestStatusNode[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	rbft.setNormal()
	assert.Equal(t, true, rbft.in(Normal))

	rbft.setFull()
	assert.Equal(t, true, rbft.atomicIn(PoolFull))
}

func TestStatusMgr_maybeSetNormal(t *testing.T) {
	ctrl := gomock.NewController(t)
	// defer ctrl.Finish()

	rbft := newTestStatusNode[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

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
