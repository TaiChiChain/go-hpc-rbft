package rbftmock

import "github.com/golang/mock/gomock"

// NewMockMinimalNode returns a minimal implement of MockNode which accepts
// any kinds of input and returns 'zero value' as all outputs.
// Users can defines custom MockNode like this:
// func NewMockCustomNode(ctrl *gomock.Controller) *MockNode {...}
// in which users must specify output for all functions.
func NewMockMinimalNode(ctrl *gomock.Controller) *MockNode {
	mock := NewMockNode(ctrl)
	mock.EXPECT().Start().Return(nil).AnyTimes()
	mock.EXPECT().Propose(gomock.Any()).Return(nil).AnyTimes()
	mock.EXPECT().ProposeConfChange(gomock.Any()).Return(nil).AnyTimes()
	mock.EXPECT().Step(gomock.Any()).Return().AnyTimes()
	mock.EXPECT().ApplyConfChange(gomock.Any()).Return().AnyTimes()
	mock.EXPECT().Status().Return(nil).AnyTimes()
	mock.EXPECT().Stop().Return().AnyTimes()
	mock.EXPECT().ReportExecuted(gomock.Any()).Return().AnyTimes()
	mock.EXPECT().ReportStateUpdated(gomock.Any()).Return().AnyTimes()
	return mock
}
