package rbft

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEpochInfo_ElectValidators(t *testing.T) {
	e := &EpochInfo{
		ConsensusParams: &ConsensusParams{
			ValidatorElectionType: ValidatorElectionTypeVotingPowerPriority,
			MaxValidatorNum:       4,
		},
		CandidateSet: []*NodeInfo{
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
		ValidatorSet: []*NodeInfo{
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
		DataSyncerSet: nil,
	}

	err := e.ElectValidators([]byte("test seed"))
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
