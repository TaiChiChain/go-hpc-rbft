package rbft

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/axiomesh/axiom-bft/types"
)

func TestBlockProcessorTracker_ResetRecentBlockNum(t *testing.T) {
	var validatorSetNum uint64 = 8

	blockProcessorTracker := NewBlockProcessorTracker(func(u uint64) (*types.BlockMeta, error) {
		return &types.BlockMeta{
			ProcessorNodeID: u%validatorSetNum + 1,
			BlockNum:        u,
		}, nil
	})
	type args struct {
		epochStartBlockNum   uint64
		lastExecutedBlockNum uint64
		recentBlockNum       uint64
	}
	type expected struct {
		startBlockNum uint64
		endBlockNum   uint64
	}
	tests := []struct {
		name     string
		args     args
		expected expected
	}{
		{
			name: "first-epoch-start",
			args: args{
				epochStartBlockNum:   1,
				lastExecutedBlockNum: 1,
				recentBlockNum:       2,
			},
			expected: expected{
				startBlockNum: 1,
				endBlockNum:   1,
			},
		},
		{
			name: "first-epoch-with-some-blocks",
			args: args{
				epochStartBlockNum:   1,
				lastExecutedBlockNum: 2,
				recentBlockNum:       2,
			},
			expected: expected{
				startBlockNum: 1,
				endBlockNum:   2,
			},
		},
		{
			name: "first-epoch-with-some-blocks2",
			args: args{
				epochStartBlockNum:   1,
				lastExecutedBlockNum: 3,
				recentBlockNum:       2,
			},
			expected: expected{
				startBlockNum: 2,
				endBlockNum:   3,
			},
		},
		{
			name: "cross-epoch-start",
			args: args{
				epochStartBlockNum:   101,
				lastExecutedBlockNum: 101,
				recentBlockNum:       2,
			},
			expected: expected{
				startBlockNum: 101,
				endBlockNum:   101,
			},
		},
		{
			name: "cross-epoch-with-some-blocks",
			args: args{
				epochStartBlockNum:   101,
				lastExecutedBlockNum: 102,
				recentBlockNum:       2,
			},
			expected: expected{
				startBlockNum: 101,
				endBlockNum:   102,
			},
		},
		{
			name: "cross-epoch-with-some-blocks",
			args: args{
				epochStartBlockNum:   102,
				lastExecutedBlockNum: 103,
				recentBlockNum:       2,
			},
			expected: expected{
				startBlockNum: 102,
				endBlockNum:   103,
			},
		},
	}

	for _, tt := range tests {
		ch := make(chan struct{}, 1)
		t.Run(tt.name, func(t *testing.T) {
			blockProcessorTracker.ResetRecentBlockNum(tt.args.epochStartBlockNum, tt.args.lastExecutedBlockNum, tt.args.recentBlockNum)
			assert.Equal(t, tt.expected.startBlockNum, blockProcessorTracker.StartBlockNum)
			assert.Equal(t, tt.expected.endBlockNum, blockProcessorTracker.EndBlockNum)
			blockProcessorIDSet := blockProcessorTracker.GetRecentProcessorSet()
			assert.Equal(t, tt.expected.endBlockNum-tt.expected.startBlockNum+1, uint64(len(blockProcessorIDSet)))
			for i := tt.expected.startBlockNum; i <= tt.expected.endBlockNum; i++ {
				_, ok := blockProcessorIDSet[i%validatorSetNum+1]
				assert.True(t, ok)
			}
			ch <- struct{}{}
		})
		<-ch
	}
}
