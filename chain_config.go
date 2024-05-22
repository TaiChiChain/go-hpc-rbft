package rbft

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"
	"sort"

	"github.com/pkg/errors"
	"github.com/samber/lo"

	"github.com/axiomesh/axiom-bft/common"
	"github.com/axiomesh/axiom-bft/types"
	kittypes "github.com/axiomesh/axiom-kit/types"
)

const (
	ValidatorElectionTypeWRF                 = "wrf"
	ValidatorElectionTypeVotingPowerPriority = "voting-power-priority"
)

const (
	ProposerElectionTypeWRF              = "wrf"
	ProposerElectionTypeAbnormalRotation = "abnormal-rotation"
)

type ValidatorInfo struct {
	ID uint64

	// current consensus voting power
	ConsensusVotingPower int64

	ConsensusVotingPowerReduced    bool
	ConsensusVotingPowerReduceView uint64
}

// NodeInfo node info
type NodeInfo struct {
	// The node serial number is unique in the entire network.
	// Once allocated, it will not change.
	// It is allocated through the governance contract in a manner similar to the self-incrementing primary key.
	ID uint64

	// P2P node ID(encode by p2p public key).
	P2PNodeID string
}

func wrfSelectNodeByVotingPower(seed []byte, nodeID2VotingPower map[uint64]int64) uint64 {
	h := sha256.New()
	_, err := h.Write(seed)
	if err != nil {
		panic(err)
	}
	seedHash := h.Sum(nil)

	// clone
	nodeSet := lo.MapToSlice(nodeID2VotingPower, func(id uint64, votingPower int64) ValidatorInfo {
		return ValidatorInfo{
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
	cumulativeVotingPowers := lo.Map(nodeSet, func(item ValidatorInfo, idx int) uint64 {
		totalVotingPower += uint64(item.ConsensusVotingPower)
		return totalVotingPower
	})

	seedInt := big.NewInt(0).SetBytes(seedHash)
	selectedCumulativeVotingPower := seedInt.Mod(seedInt, big.NewInt(int64(totalVotingPower))).Uint64()
	selectedIndex := binarySearch(cumulativeVotingPowers, selectedCumulativeVotingPower)
	return nodeSet[selectedIndex].ID
}

type EpochDerivedData struct {
	// Validator set size.
	N int

	// The maximum number of Byzantine nodes that can be tolerated in the verifier node set.
	F int

	// High watermark perid block number.
	L uint64

	SelfID uint64

	nodeInfoMap map[uint64]NodeInfo

	ValidatorSet map[uint64]int64
}

type DynamicChainConfig struct {
	// will track validator consensusVotingPower
	ValidatorDynamicInfoMap map[uint64]*ValidatorInfo

	// Low watermark block number.
	H uint64

	LastStableValidatorDynamicInfoMap map[uint64]*ValidatorInfo

	// Last stable view, change by ViewChangeDone and Checkpoint.
	LastStableView uint64

	// Current view(auto-increment), change by ViewChange and Checkpoint.
	View uint64

	// last checkpoint block hash
	LastCheckpointExecBlockHash string

	LastCheckpointExecBlockHeight uint64

	// Proposer node id of the current View period.
	PrimaryID uint64

	RecentBlockProcessorTracker *BlockProcessorTracker
}

// BlockProcessorTracker use rings to track recent block proposers
type BlockProcessorTracker struct {
	getBlockFunc        func(uint64) (*types.BlockMeta, error)
	BlockProcessors     []*types.BlockMeta
	RecentBlockNum      uint64
	NextIdx             uint64
	BlockProcessorIDSet map[uint64]struct{}
	StartBlockNum       uint64
	EndBlockNum         uint64
}

func NewBlockProcessorTracker(getBlockFunc func(uint64) (*types.BlockMeta, error)) *BlockProcessorTracker {
	return &BlockProcessorTracker{
		getBlockFunc:        getBlockFunc,
		BlockProcessors:     []*types.BlockMeta{},
		RecentBlockNum:      0,
		NextIdx:             0,
		BlockProcessorIDSet: map[uint64]struct{}{},
		StartBlockNum:       0,
		EndBlockNum:         0,
	}
}

func (t *BlockProcessorTracker) ResetRecentBlockNum(epochStartBlockNum uint64, lastCheckpointExecBlockHeight uint64, recentBlockNum uint64) {
	oldBlockProcessors := make(map[uint64]*types.BlockMeta, len(t.BlockProcessors))
	for _, oldBlockProcessor := range t.BlockProcessors {
		if oldBlockProcessor != nil {
			oldBlockProcessors[oldBlockProcessor.BlockNum] = oldBlockProcessor
		}
	}
	t.BlockProcessors = make([]*types.BlockMeta, recentBlockNum)
	t.RecentBlockNum = recentBlockNum
	t.NextIdx = 0
	t.BlockProcessorIDSet = make(map[uint64]struct{})
	t.StartBlockNum = 0
	t.EndBlockNum = 0
	startBlockNum := epochStartBlockNum
	endBlockNum := lastCheckpointExecBlockHeight
	if endBlockNum > epochStartBlockNum+recentBlockNum-1 {
		startBlockNum = endBlockNum - recentBlockNum + 1
	}

	if startBlockNum <= endBlockNum {
		for i := startBlockNum; i <= endBlockNum; i++ {
			if oldBlockProcessor, ok := oldBlockProcessors[i]; ok {
				t.AddBlock(*oldBlockProcessor)
			} else {
				m, err := t.getBlockFunc(i)
				if err != nil {
					panic(fmt.Sprintf("failed to get block %d when ResetRecentBlockNum: %v", i, err))
				}
				t.AddBlock(*m)
			}
		}
	}
}

func (t *BlockProcessorTracker) AddBlock(lastCheckpointExecBlockMeta types.BlockMeta) {
	t.BlockProcessors[t.NextIdx] = &lastCheckpointExecBlockMeta
	blockProcessorIDSet := make(map[uint64]struct{})
	for _, item := range t.BlockProcessors {
		if item != nil {
			blockProcessorIDSet[item.ProcessorNodeID] = struct{}{}
		}
	}
	t.BlockProcessorIDSet = blockProcessorIDSet
	t.NextIdx = (t.NextIdx + 1) % t.RecentBlockNum
	if t.StartBlockNum == 0 && t.EndBlockNum == 0 {
		t.StartBlockNum = lastCheckpointExecBlockMeta.BlockNum
		t.EndBlockNum = lastCheckpointExecBlockMeta.BlockNum
	} else {
		t.EndBlockNum++
		if t.EndBlockNum-t.StartBlockNum == t.RecentBlockNum {
			t.StartBlockNum++
		}
	}
}

func (t *BlockProcessorTracker) GetRecentProcessorSet() map[uint64]struct{} {
	return t.BlockProcessorIDSet
}

// ChainConfig tracking each view.
type ChainConfig struct {
	// Get from EpochManager system contract.
	EpochInfo *kittypes.EpochInfo

	EpochDerivedData

	DynamicChainConfig

	SelfP2PNodeID string

	logger           common.Logger
	getNodeInfoFn    func(nodeID uint64) (*NodeInfo, error)
	getNodeIDByP2PID func(p2pID string) (uint64, error)
}

func (c *ChainConfig) isProposerElectionTypeWRF() bool {
	return c.EpochInfo.ConsensusParams.ProposerElectionType == ProposerElectionTypeWRF
}

func (c *ChainConfig) isValidator() bool {
	return c.CheckValidator(c.SelfID)
}

func (c *ChainConfig) getNodeInfo(nodeID uint64) (NodeInfo, error) {
	if nodeInfo, ok := c.nodeInfoMap[nodeID]; ok {
		return nodeInfo, nil
	}
	nodeInfo, err := c.getNodeInfoFn(nodeID)
	if err != nil {
		return NodeInfo{}, err
	}
	c.nodeInfoMap[nodeID] = NodeInfo{
		ID:        nodeInfo.ID,
		P2PNodeID: nodeInfo.P2PNodeID,
	}
	return NodeInfo{
		ID:        nodeInfo.ID,
		P2PNodeID: nodeInfo.P2PNodeID,
	}, nil
}

func (c *ChainConfig) updateDerivedData(newValidatorSet map[uint64]int64) error {
	if len(newValidatorSet) < 4 {
		return errors.New("at least 4 validators")
	}

	selfID, err := c.getNodeIDByP2PID(c.SelfP2PNodeID)
	if err == nil {
		c.SelfID = selfID
	}

	c.ValidatorSet = lo.MapEntries(newValidatorSet, func(key uint64, value int64) (uint64, int64) {
		return key, value
	})

	c.ValidatorDynamicInfoMap = lo.MapEntries(newValidatorSet, func(id uint64, consensusVotingPower int64) (uint64, *ValidatorInfo) {
		return id, &ValidatorInfo{
			ID:                             id,
			ConsensusVotingPower:           consensusVotingPower,
			ConsensusVotingPowerReduced:    false,
			ConsensusVotingPowerReduceView: 0,
		}
	})
	c.LastStableValidatorDynamicInfoMap = lo.MapEntries(c.ValidatorDynamicInfoMap, func(id uint64, validatorInfo *ValidatorInfo) (uint64, *ValidatorInfo) {
		return id, &ValidatorInfo{
			ID:                             id,
			ConsensusVotingPower:           validatorInfo.ConsensusVotingPower,
			ConsensusVotingPowerReduced:    false,
			ConsensusVotingPowerReduceView: 0,
		}
	})

	c.N = len(c.ValidatorDynamicInfoMap)
	c.F = (c.N - 1) / 3
	c.L = c.EpochInfo.ConsensusParams.CheckpointPeriod * c.EpochInfo.ConsensusParams.HighWatermarkCheckpointPeriod

	return nil
}

func (c *ChainConfig) wrfCalPrimaryIDByView(v uint64, validatorDynamicInfoMap map[uint64]*ValidatorInfo) uint64 {
	// generate random number by last blockhash + view + epoch
	var seed = []byte(c.LastCheckpointExecBlockHash)
	seed = binary.BigEndian.AppendUint64(seed, c.EpochInfo.Epoch)
	seed = binary.BigEndian.AppendUint64(seed, v)

	nodeID2VotingPower := make(map[uint64]int64)
	for nodeID, info := range validatorDynamicInfoMap {
		// exclude nodes that have recently produced blocks
		if _, ok := c.RecentBlockProcessorTracker.GetRecentProcessorSet()[nodeID]; !ok {
			nodeID2VotingPower[nodeID] = info.ConsensusVotingPower
		}
	}
	return wrfSelectNodeByVotingPower(seed, nodeID2VotingPower)
}

// primaryID returns the expected primary id with the given view v
func (c *ChainConfig) calPrimaryIDByView(v uint64, validatorDynamicInfoMap map[uint64]*ValidatorInfo) uint64 {
	validatorDynamicInfo := c.validatorDynamicInfo()

	var primaryID uint64
	switch c.EpochInfo.ConsensusParams.ProposerElectionType {
	case ProposerElectionTypeWRF:
		primaryID = c.wrfCalPrimaryIDByView(v, validatorDynamicInfoMap)
	case ProposerElectionTypeAbnormalRotation:
		primaryID = v%uint64(c.N) + 1
	default:
		primaryID = c.wrfCalPrimaryIDByView(v, validatorDynamicInfoMap)
	}
	excludedNodes := lo.MapToSlice(c.RecentBlockProcessorTracker.GetRecentProcessorSet(), func(id uint64, _ struct{}) uint64 {
		return id
	})
	sort.Slice(excludedNodes, func(i, j int) bool {
		return excludedNodes[i] < excludedNodes[j]
	})
	c.logger.Debugf("calPrimaryIDByView, view: %d, primary id: %d, validatorDynamicInfo: %v, excludedNodes: %v", v, primaryID, validatorDynamicInfo, excludedNodes)
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
	c.PrimaryID = c.calPrimaryIDByView(c.View, c.ValidatorDynamicInfoMap)
}

func (c *ChainConfig) validatorDynamicInfo() []ValidatorInfo {
	res := lo.MapToSlice(c.ValidatorDynamicInfoMap, func(id uint64, nodeInfo *ValidatorInfo) ValidatorInfo {
		return *nodeInfo
	})
	sort.Slice(res, func(i, j int) bool {
		return res[i].ID < res[j].ID
	})
	return res
}

func (c *ChainConfig) ResetRecentBlockNum(lastCheckpointExecBlockHeight uint64) {
	validatorSetNum := uint64(len(c.ValidatorDynamicInfoMap))
	recentBlockNum := validatorSetNum * c.EpochInfo.ConsensusParams.AgainProposeIntervalBlockInValidatorsNumPercentage / 100
	if recentBlockNum == 0 {
		recentBlockNum = 1
	} else if recentBlockNum == validatorSetNum {
		recentBlockNum = validatorSetNum - 1
	}
	c.RecentBlockProcessorTracker.ResetRecentBlockNum(c.EpochInfo.StartBlock, lastCheckpointExecBlockHeight, recentBlockNum)
}

func (c *ChainConfig) CheckValidator(nodeID uint64) bool {
	_, ok := c.ValidatorDynamicInfoMap[nodeID]
	return ok
}
