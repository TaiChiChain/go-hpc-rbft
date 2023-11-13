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

	"github.com/axiomesh/axiom-bft/common"
)

const (
	ValidatorElectionTypeWRF                 = "wrf"
	ValidatorElectionTypeVotingPowerPriority = "voting-power-priority"
)

const (
	ProposerElectionTypeWRF              = "wrf"
	ProposerElectionTypeAbnormalRotation = "abnormal-rotation"
)

type NodeDynamicInfo struct {
	ID                             uint64
	ConsensusVotingPower           int64
	ConsensusVotingPowerReduced    bool
	ConsensusVotingPowerReduceView uint64
}

// NodeInfo node info
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

func wrfSelectNodeByVotingPower(seed []byte, nodeID2VotingPower map[uint64]int64) uint64 {
	h := sha256.New()
	_, err := h.Write(seed)
	if err != nil {
		panic(err)
	}
	seedHash := h.Sum(nil)

	// clone
	nodeSet := lo.MapToSlice(nodeID2VotingPower, func(id uint64, votingPower int64) NodeInfo {
		return NodeInfo{
			ID:                   id,
			ConsensusVotingPower: votingPower,
		}
	})
	// sort by id
	sort.Slice(nodeSet, func(i, j int) bool {
		return nodeSet[i].ID < nodeSet[j].ID
	})

	var totalVotingPower uint64
	// calculate totalVotingPower and cumulative VotingPower
	// |    a(VotingPower 2)    |    b(VotingPower 3)    |    c(VotingPower 1)    |     b(VotingPower 4)    |
	// 0                        2                        5                        6                         10
	// rand select from 0 - totalVotingPower
	cumulativeVotingPowers := lo.Map(nodeSet, func(item NodeInfo, idx int) uint64 {
		totalVotingPower += uint64(item.ConsensusVotingPower)
		return totalVotingPower
	})

	seedInt := big.NewInt(0).SetBytes(seedHash)
	selectedCumulativeVotingPower := seedInt.Mod(seedInt, big.NewInt(int64(totalVotingPower))).Uint64()
	selectedIndex := binarySearch(cumulativeVotingPowers, selectedCumulativeVotingPower)
	return nodeSet[selectedIndex].ID
}

func sortNodesByVotingPower(nodes []NodeInfo) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].ConsensusVotingPower != nodes[j].ConsensusVotingPower {
			return nodes[i].ConsensusVotingPower > nodes[j].ConsensusVotingPower
		}
		return nodes[i].ID < nodes[j].ID
	})
}

func (n *NodeInfo) Clone() NodeInfo {
	return NodeInfo{
		ID:                   n.ID,
		AccountAddress:       n.AccountAddress,
		P2PNodeID:            n.P2PNodeID,
		ConsensusVotingPower: n.ConsensusVotingPower,
	}
}

type ConsensusParams struct {
	// The validator election type, default is wrf
	// wrf: WRF
	// voting-power-priority: the greater the voting weight, the more likely it is to be selected
	ValidatorElectionType string `mapstructure:"validator_election_type" toml:"validator_election_type" json:"validator_election_type"`

	// The proposer election type, default is wrf
	// wrf: WRF
	// abnormal-rotation: rotating by view(pbft logic, disable auto change proposer)
	ProposerElectionType string `mapstructure:"proposer_election_type" toml:"proposer_election_type" json:"proposer_election_type"`

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
	AbnormalNodeExcludeView uint64 `mapstructure:"abnormal_node_exclude_view" toml:"abnormal_node_exclude_view" json:"abnormal_node_exclude_view"`

	// The block interval for node to propose again, Ensure that a node cannot continuously produce blocks
	AgainProposeIntervalBlock uint64 `mapstructure:"again_propose_interval_block" toml:"again_propose_interval_block" json:"again_propose_interval_block"`

	// ContinuousNullRequestToleranceNumber Viewchange will be sent when there is a packageable transaction locally and n nullrequests are received consecutively.
	ContinuousNullRequestToleranceNumber uint64 `mapstructure:"continuous_null_request_tolerance_number" toml:"continuous_null_request_tolerance_number" json:"continuous_null_request_tolerance_number"`
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
	ConsensusParams ConsensusParams `mapstructure:"consensus_params" toml:"consensus_params" json:"consensus_params"`

	// FinanceParams params about gas
	FinanceParams FinanceParams `mapstructure:"finance_params" toml:"finance_params" json:"finance_params"`

	MiscParams MiscParams `mapstructure:"misc_params" toml:"misc_params" json:"misc_params"`

	// Validator set(Participation consensus)
	ValidatorSet []NodeInfo `mapstructure:"validator_set" toml:"validator_set" json:"validator_set"`

	// Candidate set(Do not participate in consensus, only synchronize consensus results).
	CandidateSet []NodeInfo `mapstructure:"candidate_set" toml:"candidate_set" json:"candidate_set"`

	DataSyncerSet []NodeInfo `mapstructure:"data_syncer_set" toml:"data_syncer_set" json:"data_syncer_set"`
}

type FinanceParams struct {
	GasLimit              uint64 `mapstructure:"gas_limit" toml:"gas_limit" json:"gas_limit"`
	MaxGasPrice           uint64 `mapstructure:"max_gas_price" toml:"max_gas_price" json:"max_gas_price"`
	MinGasPrice           uint64 `mapstructure:"min_gas_price" toml:"min_gas_price" json:"min_gas_price"`
	GasChangeRateValue    uint64 `mapstructure:"gas_change_rate_value" toml:"gas_change_rate_value" json:"gas_change_rate_value"`
	GasChangeRateDecimals uint64 `mapstructure:"gas_change_rate_decimals" toml:"gas_change_rate_decimals" json:"gas_change_rate_decimals"`
}

type MiscParams struct {
	TxMaxSize uint64 `mapstructure:"tx_max_size" toml:"tx_max_size" json:"tx_max_size"`
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
		ConsensusParams: ConsensusParams{
			ValidatorElectionType:                e.ConsensusParams.ValidatorElectionType,
			ProposerElectionType:                 e.ConsensusParams.ProposerElectionType,
			CheckpointPeriod:                     e.ConsensusParams.CheckpointPeriod,
			HighWatermarkCheckpointPeriod:        e.ConsensusParams.HighWatermarkCheckpointPeriod,
			MaxValidatorNum:                      e.ConsensusParams.MaxValidatorNum,
			BlockMaxTxNum:                        e.ConsensusParams.BlockMaxTxNum,
			EnableTimedGenEmptyBlock:             e.ConsensusParams.EnableTimedGenEmptyBlock,
			NotActiveWeight:                      e.ConsensusParams.NotActiveWeight,
			AbnormalNodeExcludeView:              e.ConsensusParams.AbnormalNodeExcludeView,
			AgainProposeIntervalBlock:            e.ConsensusParams.AgainProposeIntervalBlock,
			ContinuousNullRequestToleranceNumber: e.ConsensusParams.ContinuousNullRequestToleranceNumber,
		},
		FinanceParams: FinanceParams{
			GasLimit:              e.FinanceParams.GasLimit,
			MaxGasPrice:           e.FinanceParams.MaxGasPrice,
			MinGasPrice:           e.FinanceParams.MinGasPrice,
			GasChangeRateValue:    e.FinanceParams.GasChangeRateValue,
			GasChangeRateDecimals: e.FinanceParams.GasChangeRateDecimals,
		},
		MiscParams: MiscParams{
			TxMaxSize: e.MiscParams.TxMaxSize,
		},
		ValidatorSet: lo.Map(e.ValidatorSet, func(item NodeInfo, idx int) NodeInfo {
			return NodeInfo{
				ID:                   item.ID,
				AccountAddress:       item.AccountAddress,
				P2PNodeID:            item.P2PNodeID,
				ConsensusVotingPower: item.ConsensusVotingPower,
			}
		}),
		CandidateSet: lo.Map(e.CandidateSet, func(item NodeInfo, idx int) NodeInfo {
			return NodeInfo{
				ID:                   item.ID,
				AccountAddress:       item.AccountAddress,
				P2PNodeID:            item.P2PNodeID,
				ConsensusVotingPower: item.ConsensusVotingPower,
			}
		}),
		DataSyncerSet: lo.Map(e.DataSyncerSet, func(item NodeInfo, idx int) NodeInfo {
			return NodeInfo{
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

	if e.ConsensusParams.MaxValidatorNum < 4 {
		return errors.New("epoch info error: max_validator_num must be greater than or equal to 4")
	}

	if e.ConsensusParams.BlockMaxTxNum == 0 {
		return errors.New("epoch info error: block_max_tx_num cannot be 0")
	}

	if e.ConsensusParams.AbnormalNodeExcludeView == 0 {
		return errors.New("epoch info error: exclude_view cannot be 0")
	}

	if len(e.P2PBootstrapNodeAddresses) == 0 {
		return errors.New("epoch info error: p2p_bootstrap_node_addresses cannot be empty")
	}

	if len(e.ValidatorSet) < 4 {
		return errors.New("epoch info error: validator_set need at least 4")
	}

	isAllZero := true
	for _, nodeInfo := range e.ValidatorSet {
		if nodeInfo.ConsensusVotingPower < 0 {
			return errors.Errorf("epoch info error: validator(%d) consensus_voting_power cannot be negative", nodeInfo.ID)
		} else if nodeInfo.ConsensusVotingPower > 0 {
			isAllZero = false
		}
	}
	if isAllZero {
		return errors.New("epoch info error: validators consensus_voting_power cannot all be zero")
	}

	if e.ConsensusParams.ProposerElectionType != ProposerElectionTypeWRF && e.ConsensusParams.ProposerElectionType != ProposerElectionTypeAbnormalRotation {
		return fmt.Errorf("epoch info error: unsupported proposer_election_type: %s", e.ConsensusParams.ProposerElectionType)
	}
	if e.ConsensusParams.ValidatorElectionType != ValidatorElectionTypeWRF && e.ConsensusParams.ValidatorElectionType != ValidatorElectionTypeVotingPowerPriority {
		return fmt.Errorf("epoch info error: unsupported validator_election_type: %s", e.ConsensusParams.ValidatorElectionType)
	}

	return nil
}

func (e *EpochInfo) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

func (e *EpochInfo) Unmarshal(raw []byte) error {
	return json.Unmarshal(raw, e)
}

func (e *EpochInfo) electValidatorsByWrf(electValidatorsByWrfSeed []byte, allEligibleNodes []NodeInfo) []NodeInfo {
	var i uint64 = 0
	allEligibleNodeDynamicInfoMap := make(map[uint64]NodeDynamicInfo)
	allEligibleNodeMap := make(map[uint64]NodeInfo)
	for _, eligibleNode := range allEligibleNodes {
		allEligibleNodeDynamicInfoMap[eligibleNode.ID] = NodeDynamicInfo{
			ID:                             eligibleNode.ID,
			ConsensusVotingPower:           eligibleNode.ConsensusVotingPower,
			ConsensusVotingPowerReduced:    false,
			ConsensusVotingPowerReduceView: 0,
		}
		allEligibleNodeMap[eligibleNode.ID] = eligibleNode
	}

	var validators []NodeInfo
	for ; i < e.ConsensusParams.MaxValidatorNum; i++ {
		validatorID := wrfSelectNodeByVotingPower(electValidatorsByWrfSeed, lo.SliceToMap(allEligibleNodes, func(item NodeInfo) (uint64, int64) {
			return item.ID, item.ConsensusVotingPower
		}))
		// exclude selected
		delete(allEligibleNodeDynamicInfoMap, validatorID)
		validators = append(validators, allEligibleNodeMap[validatorID])
	}

	return validators
}

func (e *EpochInfo) ElectValidators(electValidatorsByWrfSeed []byte) error {
	err := func() error {
		var allEligibleNodes []NodeInfo
		var allNodes []NodeInfo
		for _, info := range e.ValidatorSet {
			allNodes = append(allNodes, info.Clone())
			if info.ConsensusVotingPower > 0 {
				allEligibleNodes = append(allEligibleNodes, info.Clone())
			}
		}
		for _, info := range e.CandidateSet {
			allNodes = append(allNodes, info.Clone())
			if info.ConsensusVotingPower > 0 {
				allEligibleNodes = append(allEligibleNodes, info.Clone())
			}
		}
		if len(allEligibleNodes) < 4 {
			return errors.New("at least 4 nodes with voting weight greater than 0")
		}

		var validatorSet []NodeInfo
		if len(allEligibleNodes) <= int(e.ConsensusParams.MaxValidatorNum) {
			sortNodesByVotingPower(allEligibleNodes)
			validatorSet = allEligibleNodes
		} else {
			switch e.ConsensusParams.ValidatorElectionType {
			case ValidatorElectionTypeWRF:
				validatorSet = e.electValidatorsByWrf(electValidatorsByWrfSeed, allEligibleNodes)
				sortNodesByVotingPower(validatorSet)
			case ValidatorElectionTypeVotingPowerPriority:
				sortNodesByVotingPower(allEligibleNodes)
				validatorSet = allEligibleNodes[:int(e.ConsensusParams.MaxValidatorNum)]
			default:
				return fmt.Errorf("epoch info error: unsupported validator_election_type: %s", e.ConsensusParams.ValidatorElectionType)
			}
		}
		e.ValidatorSet = validatorSet
		validatorMap := lo.SliceToMap(validatorSet, func(info NodeInfo) (uint64, struct{}) {
			return info.ID, struct{}{}
		})
		e.CandidateSet = lo.Filter(allNodes, func(item NodeInfo, index int) bool {
			_, ok := validatorMap[item.ID]
			return !ok
		})
		sort.Slice(e.CandidateSet, func(i, j int) bool {
			return e.CandidateSet[i].ID < e.CandidateSet[j].ID
		})

		return nil
	}()
	if err != nil {
		return errors.Wrap(err, "failed to elect validators")
	}
	return nil
}

type NodeRole uint8

const (
	NodeRoleUnknown NodeRole = iota

	// NodeRoleDataSyncer only syncer data
	NodeRoleDataSyncer
	NodeRoleCandidate
	NodeRoleValidator
)

func (n NodeRole) String() string {
	return [...]string{"Unknown", "DataSyncer", "Candidate", "Validator"}[n]
}

type EpochDerivedData struct {
	// Validator set size.
	N int

	// The maximum number of Byzantine nodes that can be tolerated in the verifier node set.
	F int

	// High watermark perid block number.
	L uint64

	SelfID uint64

	SelfRole NodeRole

	NodeRoleMap map[uint64]NodeRole

	NodeInfoMap map[uint64]NodeInfo

	// will track validator consensusVotingPower
	ValidatorMap map[uint64]NodeInfo
}

type DynamicChainConfig struct {
	// will track validator consensusVotingPower
	ValidatorDynamicInfoMap map[uint64]*NodeDynamicInfo

	// Low watermark block number.
	H uint64

	// Last stable view, change by ViewChangeDone and Checkpoint.
	LastStableView uint64

	// Current view(auto-increment), change by ViewChange and Checkpoint.
	View uint64

	// last checkpoint block hash
	LastCheckpointExecBlockHash string

	// Proposer node id of the current View period.
	PrimaryID uint64
}

// ChainConfig tracking each view.
type ChainConfig struct {
	// Epoch info.
	EpochInfo *EpochInfo

	EpochDerivedData

	DynamicChainConfig

	SelfAccountAddress string

	logger common.Logger
}

func (c *ChainConfig) isProposerElectionTypeWRF() bool {
	return c.EpochInfo.ConsensusParams.ProposerElectionType == ProposerElectionTypeWRF
}

func (c *ChainConfig) isValidator() bool {
	return c.SelfRole == NodeRoleValidator
}

func (c *ChainConfig) updateDerivedData() error {
	if len(c.EpochInfo.ValidatorSet) < 4 {
		return errors.New("at least 4 validators")
	}
	if len(lo.Intersect(c.EpochInfo.ValidatorSet, c.EpochInfo.CandidateSet)) != 0 ||
		len(lo.Intersect(c.EpochInfo.ValidatorSet, c.EpochInfo.DataSyncerSet)) != 0 ||
		len(lo.Intersect(c.EpochInfo.CandidateSet, c.EpochInfo.DataSyncerSet)) != 0 {
		return errors.New("validator_set, candidate_set, data_syncer_set cannot overlap(A node can only have one role at a time)")
	}

	c.NodeInfoMap = make(map[uint64]NodeInfo)
	c.NodeRoleMap = make(map[uint64]NodeRole)
	fillNodes := func(nodes []NodeInfo, role NodeRole) {
		for _, p := range nodes {
			if p.AccountAddress == c.SelfAccountAddress {
				c.SelfID = p.ID
				c.SelfRole = role
			}
			c.NodeInfoMap[p.ID] = p
			c.NodeRoleMap[p.ID] = role
		}
	}
	fillNodes(c.EpochInfo.ValidatorSet, NodeRoleValidator)
	fillNodes(c.EpochInfo.CandidateSet, NodeRoleCandidate)
	fillNodes(c.EpochInfo.DataSyncerSet, NodeRoleDataSyncer)

	c.ValidatorMap = lo.SliceToMap(c.EpochInfo.ValidatorSet, func(item NodeInfo) (uint64, NodeInfo) {
		return item.ID, item.Clone()
	})

	c.ValidatorDynamicInfoMap = lo.MapEntries(c.ValidatorMap, func(id uint64, nodeInfo NodeInfo) (uint64, *NodeDynamicInfo) {
		return id, &NodeDynamicInfo{
			ID:                             id,
			ConsensusVotingPower:           nodeInfo.ConsensusVotingPower,
			ConsensusVotingPowerReduced:    false,
			ConsensusVotingPowerReduceView: 0,
		}
	})

	c.N = len(c.EpochInfo.ValidatorSet)
	c.F = (c.N - 1) / 3
	c.L = c.EpochInfo.ConsensusParams.CheckpointPeriod * c.EpochInfo.ConsensusParams.HighWatermarkCheckpointPeriod

	return nil
}

func (c *ChainConfig) wrfCalPrimaryIDByView(v uint64) uint64 {
	// generate random number by last blockhash + view + epoch
	var seed = []byte(c.LastCheckpointExecBlockHash)
	seed = binary.BigEndian.AppendUint64(seed, c.EpochInfo.Epoch)
	seed = binary.BigEndian.AppendUint64(seed, v)
	return wrfSelectNodeByVotingPower(seed, lo.MapEntries(c.ValidatorDynamicInfoMap, func(id uint64, nodeInfo *NodeDynamicInfo) (uint64, int64) {
		return id, nodeInfo.ConsensusVotingPower
	}))
}

// primaryID returns the expected primary id with the given view v
func (c *ChainConfig) calPrimaryIDByView(v uint64) uint64 {
	validatorDynamicInfo := c.validatorDynamicInfo()

	var primaryID uint64
	switch c.EpochInfo.ConsensusParams.ProposerElectionType {
	case ProposerElectionTypeWRF:
		primaryID = c.wrfCalPrimaryIDByView(v)
	case ProposerElectionTypeAbnormalRotation:
		primaryID = v%uint64(c.N) + 1
	default:
		primaryID = c.wrfCalPrimaryIDByView(v)
	}
	c.logger.Debugf("calPrimaryIDByView, view: %d, primary id: %d, validatorDynamicInfo: %v", v, primaryID, validatorDynamicInfo)
	return primaryID
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

func (c *ChainConfig) validatorDynamicInfo() []NodeDynamicInfo {
	res := lo.MapToSlice(c.ValidatorDynamicInfoMap, func(id uint64, nodeInfo *NodeDynamicInfo) NodeDynamicInfo {
		return *nodeInfo
	})
	sort.Slice(res, func(i, j int) bool {
		return res[i].ID < res[j].ID
	})
	return lo.MapToSlice(c.ValidatorDynamicInfoMap, func(id uint64, nodeInfo *NodeDynamicInfo) NodeDynamicInfo {
		return *nodeInfo
	})
}
