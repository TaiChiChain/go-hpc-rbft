package rbft

import (
	"github.com/golang/mock/gomock"

	"github.com/axiomesh/axiom-bft/common/consensus"
)

// NewMockMinimalNode returns a minimal implement of MockNode which accepts
// any kinds of input and returns 'zero value' as all outputs.
// Users can define custom MockNode like this:
// func NewMockCustomNode(ctrl *gomock.Controller) *MockNode {...}
// in which users must specify output for all functions.
func NewMockMinimalNode[T any, Constraint consensus.TXConstraint[T]](ctrl *gomock.Controller) *MockNode[T, Constraint] {
	mock := NewMockNode[T, Constraint](ctrl)
	mock.EXPECT().Start().Return(nil).AnyTimes()
	mock.EXPECT().Step(gomock.Any(), gomock.Any()).Return().AnyTimes()
	mock.EXPECT().Stop().Return([][]byte{}).AnyTimes()
	mock.EXPECT().ReportExecuted(gomock.Any()).Return().AnyTimes()
	mock.EXPECT().ReportStateUpdated(gomock.Any()).Return().AnyTimes()
	return mock
}
