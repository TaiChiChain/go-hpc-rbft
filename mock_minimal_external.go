package rbft

import (
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/mock/gomock"

	"github.com/axiomesh/axiom-kit/types"
)

// NewMockMinimalExternal returns a minimal implement of MockExternalStack which accepts
// any kinds of input and returns 'zero value' as all outputs.
// Users can defines custom MockExternalStack like this:
// func NewMockCustomMockExternalStack(ctrl *gomock.Controller) *MockExternalStack {...}
// in which users must specify output for all functions.
func NewMockMinimalExternal[T any, Constraint types.TXConstraint[T]](ctrl *gomock.Controller) *MockExternalStack[T, Constraint] {
	mock := NewMockExternalStack[T, Constraint](ctrl)
	mock.EXPECT().StoreState(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mock.EXPECT().DelState(gomock.Any()).Return(nil).AnyTimes()

	mock.EXPECT().ReadState(gomock.Any()).Return(nil, errors.New("ReadState Error")).AnyTimes()
	mock.EXPECT().ReadStateSet(gomock.Any()).Return(nil, nil).AnyTimes()

	mock.EXPECT().Broadcast(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mock.EXPECT().Unicast(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	mock.EXPECT().Sign(gomock.Any()).Return(nil, nil).AnyTimes()
	mock.EXPECT().Verify(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	mock.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return().AnyTimes()
	mock.EXPECT().StateUpdate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return().AnyTimes()
	mock.EXPECT().SendFilterEvent(gomock.Any(), gomock.Any()).Return().AnyTimes()

	mock.EXPECT().GetCurrentEpochInfo().Return(nil, errors.New("not found epoch info for mock")).AnyTimes()
	mock.EXPECT().StoreEpochState(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mock.EXPECT().ReadEpochState(gomock.Any()).Return(nil, errors.New("ReadEpochState Error")).AnyTimes()

	mock.EXPECT().GetNodeIDByP2PID(gomock.Any()).DoAndReturn(func(p2pID string) (uint64, error) {
		nodeIDStr := strings.TrimPrefix(p2pID, "node")
		if nodeIDStr == p2pID {
			return 0, errors.New("invalid p2p id")
		}
		nodeID, err := strconv.Atoi(nodeIDStr)
		if err != nil {
			return 0, err
		}
		return uint64(nodeID), nil
	}).AnyTimes()

	mock.EXPECT().GetNodeInfo(gomock.Any()).DoAndReturn(func(nodeID uint64) (*NodeInfo, error) {
		return &NodeInfo{
			ID:        nodeID,
			P2PNodeID: "node" + strconv.Itoa(int(nodeID+1)),
		}, nil
	}).AnyTimes()
	mock.EXPECT().GetValidatorSet().DoAndReturn(func() (map[uint64]int64, error) {
		return map[uint64]int64{
			1: 1000,
			2: 1000,
			3: 1000,
			4: 1000,
		}, nil
	}).AnyTimes()

	return mock
}
