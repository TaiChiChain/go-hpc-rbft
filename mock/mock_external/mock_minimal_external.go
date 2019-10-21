package mockexternal

import (
	"errors"
	"github.com/golang/mock/gomock"
)

// NewMockMinimalExternal returns a minimal implement of MockExternalStack which accepts
// any kinds of input and returns 'zero value' as all outputs.
// Users can defines custom MockExternalStack like this:
// func NewMockCustomMockExternalStack(ctrl *gomock.Controller) *MockExternalStack {...}
// in which users must specify output for all functions.
func NewMockMinimalExternal(ctrl *gomock.Controller) *MockExternalStack {
	mock := NewMockExternalStack(ctrl)
	mock.EXPECT().StoreState(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mock.EXPECT().DelState(gomock.Any()).Return(nil).AnyTimes()

	mock.EXPECT().ReadState(gomock.Any()).Return(nil, errors.New("ReadState Error")).AnyTimes()
	mock.EXPECT().ReadStateSet(gomock.Any()).Return(nil, nil).AnyTimes()
	mock.EXPECT().Destroy().Return(nil).AnyTimes()

	mock.EXPECT().Broadcast(gomock.Any()).Return(nil).AnyTimes()
	mock.EXPECT().Unicast(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mock.EXPECT().UpdateTable(gomock.Any()).Return().AnyTimes()

	mock.EXPECT().Sign(gomock.Any()).Return(nil, nil).AnyTimes()
	mock.EXPECT().Verify(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	mock.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return().AnyTimes()
	mock.EXPECT().StateUpdate(gomock.Any(), gomock.Any(), gomock.Any()).Return().AnyTimes()
	mock.EXPECT().SendFilterEvent(gomock.Any(), gomock.Any()).Return().AnyTimes()

	return mock
}
