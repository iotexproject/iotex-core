// Code generated by MockGen. DO NOT EDIT.
// Source: ./action/protocol/poll/protocol.go
//
// Generated by this command:
//
//	mockgen -destination=./test/mock/mock_poll/mock_poll.go -source=./action/protocol/poll/protocol.go -package=mock_poll Protocol
//

// Package mock_poll is a generated GoMock package.
package mock_poll

import (
	context "context"
	reflect "reflect"

	action "github.com/iotexproject/iotex-core/v2/action"
	protocol "github.com/iotexproject/iotex-core/v2/action/protocol"
	state "github.com/iotexproject/iotex-core/v2/state"
	gomock "go.uber.org/mock/gomock"
)

// MockProtocol is a mock of Protocol interface.
type MockProtocol struct {
	ctrl     *gomock.Controller
	recorder *MockProtocolMockRecorder
	isgomock struct{}
}

// MockProtocolMockRecorder is the mock recorder for MockProtocol.
type MockProtocolMockRecorder struct {
	mock *MockProtocol
}

// NewMockProtocol creates a new mock instance.
func NewMockProtocol(ctrl *gomock.Controller) *MockProtocol {
	mock := &MockProtocol{ctrl: ctrl}
	mock.recorder = &MockProtocolMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockProtocol) EXPECT() *MockProtocolMockRecorder {
	return m.recorder
}

// CalculateCandidatesByHeight mocks base method.
func (m *MockProtocol) CalculateCandidatesByHeight(arg0 context.Context, arg1 protocol.StateReader, arg2 uint64) (state.CandidateList, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CalculateCandidatesByHeight", arg0, arg1, arg2)
	ret0, _ := ret[0].(state.CandidateList)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CalculateCandidatesByHeight indicates an expected call of CalculateCandidatesByHeight.
func (mr *MockProtocolMockRecorder) CalculateCandidatesByHeight(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CalculateCandidatesByHeight", reflect.TypeOf((*MockProtocol)(nil).CalculateCandidatesByHeight), arg0, arg1, arg2)
}

// CalculateUnproductiveDelegates mocks base method.
func (m *MockProtocol) CalculateUnproductiveDelegates(arg0 context.Context, arg1 protocol.StateReader) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CalculateUnproductiveDelegates", arg0, arg1)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CalculateUnproductiveDelegates indicates an expected call of CalculateUnproductiveDelegates.
func (mr *MockProtocolMockRecorder) CalculateUnproductiveDelegates(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CalculateUnproductiveDelegates", reflect.TypeOf((*MockProtocol)(nil).CalculateUnproductiveDelegates), arg0, arg1)
}

// Candidates mocks base method.
func (m *MockProtocol) Candidates(arg0 context.Context, arg1 protocol.StateReader) (state.CandidateList, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Candidates", arg0, arg1)
	ret0, _ := ret[0].(state.CandidateList)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Candidates indicates an expected call of Candidates.
func (mr *MockProtocolMockRecorder) Candidates(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Candidates", reflect.TypeOf((*MockProtocol)(nil).Candidates), arg0, arg1)
}

// CreateGenesisStates mocks base method.
func (m *MockProtocol) CreateGenesisStates(arg0 context.Context, arg1 protocol.StateManager) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateGenesisStates", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateGenesisStates indicates an expected call of CreateGenesisStates.
func (mr *MockProtocolMockRecorder) CreateGenesisStates(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateGenesisStates", reflect.TypeOf((*MockProtocol)(nil).CreateGenesisStates), arg0, arg1)
}

// Delegates mocks base method.
func (m *MockProtocol) Delegates(arg0 context.Context, arg1 protocol.StateReader) (state.CandidateList, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delegates", arg0, arg1)
	ret0, _ := ret[0].(state.CandidateList)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Delegates indicates an expected call of Delegates.
func (mr *MockProtocolMockRecorder) Delegates(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delegates", reflect.TypeOf((*MockProtocol)(nil).Delegates), arg0, arg1)
}

// ForceRegister mocks base method.
func (m *MockProtocol) ForceRegister(arg0 *protocol.Registry) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ForceRegister", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// ForceRegister indicates an expected call of ForceRegister.
func (mr *MockProtocolMockRecorder) ForceRegister(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ForceRegister", reflect.TypeOf((*MockProtocol)(nil).ForceRegister), arg0)
}

// Handle mocks base method.
func (m *MockProtocol) Handle(arg0 context.Context, arg1 action.Envelope, arg2 protocol.StateManager) (*action.Receipt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Handle", arg0, arg1, arg2)
	ret0, _ := ret[0].(*action.Receipt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Handle indicates an expected call of Handle.
func (mr *MockProtocolMockRecorder) Handle(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Handle", reflect.TypeOf((*MockProtocol)(nil).Handle), arg0, arg1, arg2)
}

// Name mocks base method.
func (m *MockProtocol) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockProtocolMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockProtocol)(nil).Name))
}

// NextCandidates mocks base method.
func (m *MockProtocol) NextCandidates(arg0 context.Context, arg1 protocol.StateReader) (state.CandidateList, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NextCandidates", arg0, arg1)
	ret0, _ := ret[0].(state.CandidateList)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NextCandidates indicates an expected call of NextCandidates.
func (mr *MockProtocolMockRecorder) NextCandidates(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NextCandidates", reflect.TypeOf((*MockProtocol)(nil).NextCandidates), arg0, arg1)
}

// NextDelegates mocks base method.
func (m *MockProtocol) NextDelegates(arg0 context.Context, arg1 protocol.StateReader) (state.CandidateList, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NextDelegates", arg0, arg1)
	ret0, _ := ret[0].(state.CandidateList)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NextDelegates indicates an expected call of NextDelegates.
func (mr *MockProtocolMockRecorder) NextDelegates(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NextDelegates", reflect.TypeOf((*MockProtocol)(nil).NextDelegates), arg0, arg1)
}

// ReadState mocks base method.
func (m *MockProtocol) ReadState(arg0 context.Context, arg1 protocol.StateReader, arg2 []byte, arg3 ...[]byte) ([]byte, uint64, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1, arg2}
	for _, a := range arg3 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ReadState", varargs...)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(uint64)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ReadState indicates an expected call of ReadState.
func (mr *MockProtocolMockRecorder) ReadState(arg0, arg1, arg2 any, arg3 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1, arg2}, arg3...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadState", reflect.TypeOf((*MockProtocol)(nil).ReadState), varargs...)
}

// Register mocks base method.
func (m *MockProtocol) Register(arg0 *protocol.Registry) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Register", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Register indicates an expected call of Register.
func (mr *MockProtocolMockRecorder) Register(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Register", reflect.TypeOf((*MockProtocol)(nil).Register), arg0)
}

// Validate mocks base method.
func (m *MockProtocol) Validate(arg0 context.Context, arg1 action.Envelope, arg2 protocol.StateReader) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Validate", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Validate indicates an expected call of Validate.
func (mr *MockProtocolMockRecorder) Validate(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Validate", reflect.TypeOf((*MockProtocol)(nil).Validate), arg0, arg1, arg2)
}
