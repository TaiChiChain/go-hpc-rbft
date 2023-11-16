package rbft

import (
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

	mock.EXPECT().GetEpochInfo(gomock.Any()).Return(nil, nil).AnyTimes()
	mock.EXPECT().GetCurrentEpochInfo().Return(nil, errors.New("not found epoch info for mock")).AnyTimes()
	mock.EXPECT().StoreEpochState(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mock.EXPECT().ReadEpochState(gomock.Any()).Return(nil, errors.New("ReadEpochState Error")).AnyTimes()

	return mock
}
