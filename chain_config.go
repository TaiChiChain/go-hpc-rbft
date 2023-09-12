package rbft

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"

	"github.com/pkg/errors"
	"github.com/samber/lo"
)

const (
	ProposerElectionTypeWRF      uint64 = 0
	ProposerElectionTypeRotating uint64 = 1
)

// Candidate/Validator node info
type NodeInfo struct {
	// The node serial number is unique in the entire network.
	// Once allocated, it will not change.
	// It is allocated through the governance contract in a manner similar to the self-incrementing primary key.
	ID uint64 `mapstructure:"id" toml:"id" json:"id"`

	// The address of the node staking account address.
	AccountAddress string `mapstructure:"account_address" toml:"account_address" json:"account_address"`

	// P2P node ID(encode by p2p public key).
	P2PNodeID string `mapstructure:"p2p_node_id" toml:"p2p_node_id" json:"p2p_node_id"`

	// Consensus voting weight.
	ConsensusVotingPower int64 `mapstructure:"consensus_voting_power" toml:"consensus_voting_power" json:"consensus_voting_power"`
}

func (n *NodeInfo) Clone() *NodeInfo {
	return &NodeInfo{
		ID:                   n.ID,
		AccountAddress:       n.AccountAddress,
		P2PNodeID:            n.P2PNodeID,
		ConsensusVotingPower: n.ConsensusVotingPower,
	}
}

type ConsensusParams struct {
	// The number of sustained blocks per Checkpoint.
	CheckpointPeriod uint64 `mapstructure:"checkpoint_period" toml:"checkpoint_period" json:"checkpoint_period"`

	// Used to calculate max log size in memory: CheckpointPeriod*HighWatermarkCheckpointPeriod.
	HighWatermarkCheckpointPeriod uint64 `mapstructure:"high_watermark_checkpoint_period" toml:"high_watermark_checkpoint_period" json:"high_watermark_checkpoint_period"`

	// The maximum number of validators in the network.
	MaxValidatorNum uint64 `mapstructure:"max_validator_num" toml:"max_validator_num" json:"max_validator_num"`

	// The maximum number of packaged transactions per block.
	BlockMaxTxNum uint64 `mapstructure:"block_max_tx_num" toml:"block_max_tx_num" json:"block_max_tx_num"`

	// Enable timed gen empty block feature.
	EnableTimedGenEmptyBlock bool `mapstructure:"enable_timed_gen_empty_block" toml:"enable_timed_gen_empty_block" json:"enable_timed_gen_empty_block"`

	// The weight of the faulty node after viewchange is triggered is set to the weight so that the node has a low probability of blocking.
	NotActiveWeight int64 `mapstructure:"not_active_weight" toml:"not_active_weight" json:"not_active_weight"`

	// The low weight of the viewchange node is restored to normal after the specified number of rounds.
	ExcludeView uint64 `mapstructure:"exclude_view" toml:"exclude_view" json:"exclude_view"`

	// The proposer election type, default is wrf
	// 0: WRF
	// 1: Rotating by view(pbft logic, disable auto change proposer)
	ProposerElectionType uint64 `mapstructure:"proposer_election_type" toml:"proposer_election_type" json:"proposer_election_type"`
}

type EpochInfo struct {
	// version.
	Version uint64 `mapstructure:"version" toml:"version" json:"version"`

	// Epoch number.
	Epoch uint64 `mapstructure:"epoch" toml:"epoch" json:"epoch"`

	// The number of blocks lasting per Epoch (must be a multiple of the CheckpointPeriod).
	EpochPeriod uint64 `mapstructure:"epoch_period" toml:"epoch_period" json:"epoch_period"`

	// Epoch start block.
	StartBlock uint64 `mapstructure:"start_block" toml:"start_block" json:"start_block"`

	// List of seed node addresses in a p2p DHT network.
	P2PBootstrapNodeAddresses []string `mapstructure:"p2p_bootstrap_node_addresses" toml:"p2p_bootstrap_node_addresses" json:"p2p_bootstrap_node_addresses"`

	// Consensus params.
	ConsensusParams *ConsensusParams `mapstructure:"consensus_params" toml:"consensus_params" json:"consensus_params"`

	// Candidate set(Do not participate in consensus, only synchronize consensus results).
	CandidateSet []*NodeInfo `mapstructure:"candidate_set" toml:"candidate_set" json:"candidate_set"`

	// Validator set(Participation consensus)
	ValidatorSet []*NodeInfo `mapstructure:"validator_set" toml:"validator_set" json:"validator_set"`
}

func (e *EpochInfo) Clone() *EpochInfo {
	return &EpochInfo{
		Version:     e.Version,
		Epoch:       e.Epoch,
		EpochPeriod: e.EpochPeriod,
		StartBlock:  e.StartBlock,
		P2PBootstrapNodeAddresses: lo.Map(e.P2PBootstrapNodeAddresses, func(item string, idx int) string {
			return item
		}),
		ConsensusParams: &ConsensusParams{
			CheckpointPeriod:              e.ConsensusParams.CheckpointPeriod,
			HighWatermarkCheckpointPeriod: e.ConsensusParams.HighWatermarkCheckpointPeriod,
			MaxValidatorNum:               e.ConsensusParams.MaxValidatorNum,
			BlockMaxTxNum:                 e.ConsensusParams.BlockMaxTxNum,
			EnableTimedGenEmptyBlock:      e.ConsensusParams.EnableTimedGenEmptyBlock,
			NotActiveWeight:               e.ConsensusParams.NotActiveWeight,
			ExcludeView:                   e.ConsensusParams.ExcludeView,
			ProposerElectionType:          e.ConsensusParams.ProposerElectionType,
		},
		CandidateSet: lo.Map(e.CandidateSet, func(item *NodeInfo, idx int) *NodeInfo {
			return &NodeInfo{
				ID:                   item.ID,
				AccountAddress:       item.AccountAddress,
				P2PNodeID:            item.P2PNodeID,
				ConsensusVotingPower: item.ConsensusVotingPower,
			}
		}),
		ValidatorSet: lo.Map(e.ValidatorSet, func(item *NodeInfo, idx int) *NodeInfo {
			return &NodeInfo{
				ID:                   item.ID,
				AccountAddress:       item.AccountAddress,
				P2PNodeID:            item.P2PNodeID,
				ConsensusVotingPower: item.ConsensusVotingPower,
			}
		}),
	}
}

func (e *EpochInfo) Check() error {
	if e.ConsensusParams.CheckpointPeriod == 0 {
		return errors.New("epoch info error: checkpoint_period cannot be 0")
	}

	if e.EpochPeriod == 0 {
		return errors.New("epoch info error: epoch_period cannot be 0")
	} else if e.EpochPeriod%e.ConsensusParams.CheckpointPeriod != 0 {
		return errors.New("epoch info error: epoch_period must be an integral multiple of checkpoint_period")
	}

	if e.ConsensusParams.HighWatermarkCheckpointPeriod == 0 {
		return errors.New("epoch info error: high_watermark_checkpoint_period cannot be 0")
	}

	if e.ConsensusParams.MaxValidatorNum == 0 {
		return errors.New("epoch info error: max_validator_num cannot be 0")
	}

	if e.ConsensusParams.BlockMaxTxNum == 0 {
		return errors.New("epoch info error: block_max_tx_num cannot be 0")
	}

	if e.ConsensusParams.ExcludeView == 0 {
		return errors.New("epoch info error: exclude_view cannot be 0")
	}

	if len(e.P2PBootstrapNodeAddresses) == 0 {
		return errors.New("epoch info error: p2p_bootstrap_node_addresses cannot be empty")
	}

	if len(e.ValidatorSet) < 4 {
		return errors.New("epoch info error: validator_set need at least 4")
	}

	for _, nodeInfo := range e.ValidatorSet {
		if nodeInfo.ConsensusVotingPower < 0 {
			return errors.Errorf("epoch info error: validator(%d) consensus_voting_power cannot be negative", nodeInfo.ID)
		}
	}

	if e.ConsensusParams.ProposerElectionType != ProposerElectionTypeWRF && e.ConsensusParams.ProposerElectionType != ProposerElectionTypeRotating {
		return fmt.Errorf("epoch info error: unsupported proposer_election_type: %d", e.ConsensusParams.ProposerElectionType)
	}

	return nil
}

func (e *EpochInfo) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

func (e *EpochInfo) Unmarshal(raw []byte) error {
	return json.Unmarshal(raw, e)
}

// Chain config, tracking each view.
type ChainConfig struct {
	// Epoch info.
	EpochInfo *EpochInfo

	// Validator set size.
	N int

	// The maximum number of Byzantine nodes that can be tolerated in the verifier node set.
	F int

	// Low watermark block number.
	H uint64

	// High watermark perid block number.
	L uint64

	// Current view(auto-increment), change by ViewChange and Checkpoint.
	View uint64

	// last checkpoint block hash
	LastCheckpointExecBlockHash string

	// Proposer node id of the current View period.
	PrimaryID uint64
}

func (c *ChainConfig) isWRF() bool {
	return c.EpochInfo.ConsensusParams.ProposerElectionType == ProposerElectionTypeWRF
}

func (c *ChainConfig) updateDerivedData() {
	c.N = len(c.EpochInfo.ValidatorSet)
	c.F = (c.N - 1) / 3
	c.L = c.EpochInfo.ConsensusParams.CheckpointPeriod * c.EpochInfo.ConsensusParams.HighWatermarkCheckpointPeriod
}

func (c *ChainConfig) wrfCalPrimaryIDByView(v uint64) uint64 {
	// clone validatorSet
	validatorSet := lo.Map(c.EpochInfo.ValidatorSet, func(item *NodeInfo, idx int) *NodeInfo {
		return item.Clone()
	})
	// sort validators by id
	sort.Slice(validatorSet, func(i, j int) bool {
		return validatorSet[i].ID < validatorSet[j].ID
	})

	var totalVotingPower uint64
	// calculate totalVotingPower and cumulative VotingPower
	// |    a(VotingPower 2)    |    b(VotingPower 3)    |    c(VotingPower 1)    |     b(VotingPower 4)    |
	// 0                        2                        5                        6                         10
	// rand select from 0 - totalVotingPower
	cumulativeVotingPowers := lo.Map(validatorSet, func(item *NodeInfo, idx int) uint64 {
		totalVotingPower += uint64(item.ConsensusVotingPower)
		return totalVotingPower
	})

	// generate random number by last blockhash + view + epoch
	var state = []byte(c.LastCheckpointExecBlockHash)
	state = binary.BigEndian.AppendUint64(state, c.EpochInfo.Epoch)
	state = binary.BigEndian.AppendUint64(state, v)
	h := sha256.New()
	_, err := h.Write(state)
	if err != nil {
		panic(err)
	}

	seed := big.NewInt(0).SetBytes(h.Sum(nil))
	selectedCumulativeVotingPower := seed.Mod(seed, big.NewInt(int64(totalVotingPower))).Uint64()
	selectedIndex := binarySearch(cumulativeVotingPowers, selectedCumulativeVotingPower)
	return validatorSet[selectedIndex].ID
}

// primaryID returns the expected primary id with the given view v
func (c *ChainConfig) calPrimaryIDByView(v uint64) uint64 {
	switch c.EpochInfo.ConsensusParams.ProposerElectionType {
	case ProposerElectionTypeWRF:
		return c.wrfCalPrimaryIDByView(v)
	case ProposerElectionTypeRotating:
		return v%uint64(c.N) + 1
	default:
		return c.wrfCalPrimaryIDByView(v)
	}
}

func binarySearch(nums []uint64, target uint64) int {
	left, right := 0, len(nums)-1
	for left <= right {
		mid := left + (right-left)/2
		if nums[mid] == target {
			return mid
		} else if nums[mid] < target {
			left = mid + 1
		} else {
			right = mid - 1
		}
	}
	return left
}

func (c *ChainConfig) updatePrimaryID() {
	c.PrimaryID = c.calPrimaryIDByView(c.View)
}
