package rbft

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEpochInfo_ElectValidators(t *testing.T) {
	e := &EpochInfo{
		Version:                   1,
		Epoch:                     1,
		EpochPeriod:               100,
		StartBlock:                1,
		P2PBootstrapNodeAddresses: []string{"1", "2"},
		ConsensusParams: ConsensusParams{
			ValidatorElectionType:                ValidatorElectionTypeWRF,
			ProposerElectionType:                 ProposerElectionTypeWRF,
			CheckpointPeriod:                     1,
			HighWatermarkCheckpointPeriod:        10,
			MaxValidatorNum:                      4,
			BlockMaxTxNum:                        500,
			EnableTimedGenEmptyBlock:             false,
			NotActiveWeight:                      1,
			AbnormalNodeExcludeView:              100,
			AgainProposeIntervalBlock:            1,
			ContinuousNullRequestToleranceNumber: 1,
		},
		CandidateSet: []NodeInfo{
			{
				ID:                   5,
				ConsensusVotingPower: 1000,
			},
			{
				ID:                   6,
				ConsensusVotingPower: 0,
			},
			{
				ID:                   7,
				ConsensusVotingPower: 0,
			},
			{
				ID:                   8,
				ConsensusVotingPower: 0,
			},
		},
		ValidatorSet: []NodeInfo{
			{
				ID:                   1,
				ConsensusVotingPower: 1000,
			},
			{
				ID:                   2,
				ConsensusVotingPower: 1000,
			},
			{
				ID:                   3,
				ConsensusVotingPower: 1000,
			},
			{
				ID:                   4,
				ConsensusVotingPower: 0,
			},
		},
		DataSyncerSet: []NodeInfo{
			{
				ID:                   9,
				ConsensusVotingPower: 1000,
			},
		},
		FinanceParams: FinanceParams{
			GasLimit:              0x5f5e100,
			MaxGasPrice:           10000000000000,
			MinGasPrice:           1000000000000,
			GasChangeRateValue:    1250,
			GasChangeRateDecimals: 4,
		},
		MiscParams: MiscParams{
			TxMaxSize: 1000,
		},
	}
	assert.EqualValues(t, e, e.Clone())

	err := e.ElectValidators(e.Clone(), []byte("test seed"))
	require.Nil(t, err)
	require.Equal(t, uint64(1), e.ValidatorSet[0].ID)
	require.Equal(t, uint64(2), e.ValidatorSet[1].ID)
	require.Equal(t, uint64(3), e.ValidatorSet[2].ID)
	require.Equal(t, uint64(5), e.ValidatorSet[3].ID)

	require.Equal(t, uint64(4), e.CandidateSet[0].ID)
	require.Equal(t, uint64(6), e.CandidateSet[1].ID)
	require.Equal(t, uint64(7), e.CandidateSet[2].ID)
	require.Equal(t, uint64(8), e.CandidateSet[3].ID)
}

func TestEpochInfo_ElectValidatorsAfterAddNewNodes(t *testing.T) {
	oldEpoch := &EpochInfo{
		Version:                   1,
		Epoch:                     1,
		EpochPeriod:               100,
		StartBlock:                1,
		P2PBootstrapNodeAddresses: []string{"1", "2"},
		ConsensusParams: ConsensusParams{
			ValidatorElectionType:                ValidatorElectionTypeWRF,
			ProposerElectionType:                 ProposerElectionTypeWRF,
			CheckpointPeriod:                     1,
			HighWatermarkCheckpointPeriod:        10,
			MaxValidatorNum:                      4,
			BlockMaxTxNum:                        500,
			EnableTimedGenEmptyBlock:             false,
			NotActiveWeight:                      1,
			AbnormalNodeExcludeView:              100,
			AgainProposeIntervalBlock:            1,
			ContinuousNullRequestToleranceNumber: 1,
		},
		CandidateSet: []NodeInfo{},
		ValidatorSet: []NodeInfo{
			{
				ID:                   1,
				ConsensusVotingPower: 1000,
			},
			{
				ID:                   2,
				ConsensusVotingPower: 1000,
			},
			{
				ID:                   3,
				ConsensusVotingPower: 1000,
			},
			{
				ID:                   4,
				ConsensusVotingPower: 1000,
			},
		},
		DataSyncerSet: []NodeInfo{
			{
				ID:                   5,
				ConsensusVotingPower: 1000,
			},
		},
		FinanceParams: FinanceParams{
			GasLimit:              0x5f5e100,
			MaxGasPrice:           10000000000000,
			MinGasPrice:           1000000000000,
			GasChangeRateValue:    1250,
			GasChangeRateDecimals: 4,
		},
		MiscParams: MiscParams{
			TxMaxSize: 1000,
		},
	}

	newEpoch := oldEpoch.Clone()
	newEpoch.CandidateSet = append(newEpoch.CandidateSet, NodeInfo{
		ID:                   6,
		ConsensusVotingPower: 1000,
	})

	t.Run("newly added nodes will not be selected immediately", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			tempEpoch := newEpoch.Clone()
			err := tempEpoch.ElectValidators(oldEpoch, []byte(fmt.Sprintf("test_%d", i)))
			require.Nil(t, err)
			require.Equal(t, 4, len(tempEpoch.ValidatorSet))
			require.Equal(t, uint64(1), tempEpoch.ValidatorSet[0].ID)
			require.Equal(t, uint64(2), tempEpoch.ValidatorSet[1].ID)
			require.Equal(t, uint64(3), tempEpoch.ValidatorSet[2].ID)
			require.Equal(t, uint64(4), tempEpoch.ValidatorSet[3].ID)

			require.Equal(t, 1, len(tempEpoch.CandidateSet))
			require.Equal(t, 1, len(tempEpoch.DataSyncerSet))
			require.Equal(t, uint64(6), newEpoch.CandidateSet[0].ID)
		}
	})

	t.Run("newly added nodes may be selected in the next round", func(t *testing.T) {
		tempEpoch := newEpoch.Clone()

		err := tempEpoch.ElectValidators(tempEpoch.Clone(), []byte("test"))
		require.Nil(t, err)
		require.Equal(t, 4, len(tempEpoch.ValidatorSet))
		require.Equal(t, uint64(2), tempEpoch.ValidatorSet[0].ID)
		require.Equal(t, uint64(3), tempEpoch.ValidatorSet[1].ID)
		require.Equal(t, uint64(4), tempEpoch.ValidatorSet[2].ID)
		require.Equal(t, uint64(6), tempEpoch.ValidatorSet[3].ID)

		require.Equal(t, uint64(1), tempEpoch.CandidateSet[0].ID)
	})

}
