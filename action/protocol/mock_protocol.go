// Code generated by MockGen. DO NOT EDIT.
// Source: ./action/protocol/protocol.go

// Package protocol is a generated GoMock package.
package protocol

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	action "github.com/iotexproject/iotex-core/v2/action"
)

// MockProtocol is a mock of Protocol interface.
type MockProtocol struct {
	ctrl     *gomock.Controller
	recorder *MockProtocolMockRecorder
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

// ForceRegister mocks base method.
func (m *MockProtocol) ForceRegister(arg0 *Registry) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ForceRegister", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// ForceRegister indicates an expected call of ForceRegister.
func (mr *MockProtocolMockRecorder) ForceRegister(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ForceRegister", reflect.TypeOf((*MockProtocol)(nil).ForceRegister), arg0)
}

// Handle mocks base method.
func (m *MockProtocol) Handle(arg0 context.Context, arg1 action.Envelope, arg2 StateManager) (*action.Receipt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Handle", arg0, arg1, arg2)
	ret0, _ := ret[0].(*action.Receipt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Handle indicates an expected call of Handle.
func (mr *MockProtocolMockRecorder) Handle(arg0, arg1, arg2 interface{}) *gomock.Call {
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

// ReadState mocks base method.
func (m *MockProtocol) ReadState(arg0 context.Context, arg1 StateReader, arg2 []byte, arg3 ...[]byte) ([]byte, uint64, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1, arg2}
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
func (mr *MockProtocolMockRecorder) ReadState(arg0, arg1, arg2 interface{}, arg3 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1, arg2}, arg3...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadState", reflect.TypeOf((*MockProtocol)(nil).ReadState), varargs...)
}

// Register mocks base method.
func (m *MockProtocol) Register(arg0 *Registry) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Register", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Register indicates an expected call of Register.
func (mr *MockProtocolMockRecorder) Register(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Register", reflect.TypeOf((*MockProtocol)(nil).Register), arg0)
}

// MockStarter is a mock of Starter interface.
type MockStarter struct {
	ctrl     *gomock.Controller
	recorder *MockStarterMockRecorder
}

// MockStarterMockRecorder is the mock recorder for MockStarter.
type MockStarterMockRecorder struct {
	mock *MockStarter
}

// NewMockStarter creates a new mock instance.
func NewMockStarter(ctrl *gomock.Controller) *MockStarter {
	mock := &MockStarter{ctrl: ctrl}
	mock.recorder = &MockStarterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStarter) EXPECT() *MockStarterMockRecorder {
	return m.recorder
}

// Start mocks base method.
func (m *MockStarter) Start(arg0 context.Context, arg1 StateReader) (interface{}, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start", arg0, arg1)
	ret0, _ := ret[0].(interface{})
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Start indicates an expected call of Start.
func (mr *MockStarterMockRecorder) Start(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockStarter)(nil).Start), arg0, arg1)
}

// MockGenesisStateCreator is a mock of GenesisStateCreator interface.
type MockGenesisStateCreator struct {
	ctrl     *gomock.Controller
	recorder *MockGenesisStateCreatorMockRecorder
}

// MockGenesisStateCreatorMockRecorder is the mock recorder for MockGenesisStateCreator.
type MockGenesisStateCreatorMockRecorder struct {
	mock *MockGenesisStateCreator
}

// NewMockGenesisStateCreator creates a new mock instance.
func NewMockGenesisStateCreator(ctrl *gomock.Controller) *MockGenesisStateCreator {
	mock := &MockGenesisStateCreator{ctrl: ctrl}
	mock.recorder = &MockGenesisStateCreatorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockGenesisStateCreator) EXPECT() *MockGenesisStateCreatorMockRecorder {
	return m.recorder
}

// CreateGenesisStates mocks base method.
func (m *MockGenesisStateCreator) CreateGenesisStates(arg0 context.Context, arg1 StateManager) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateGenesisStates", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateGenesisStates indicates an expected call of CreateGenesisStates.
func (mr *MockGenesisStateCreatorMockRecorder) CreateGenesisStates(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateGenesisStates", reflect.TypeOf((*MockGenesisStateCreator)(nil).CreateGenesisStates), arg0, arg1)
}

// MockPreStatesCreator is a mock of PreStatesCreator interface.
type MockPreStatesCreator struct {
	ctrl     *gomock.Controller
	recorder *MockPreStatesCreatorMockRecorder
}

// MockPreStatesCreatorMockRecorder is the mock recorder for MockPreStatesCreator.
type MockPreStatesCreatorMockRecorder struct {
	mock *MockPreStatesCreator
}

// NewMockPreStatesCreator creates a new mock instance.
func NewMockPreStatesCreator(ctrl *gomock.Controller) *MockPreStatesCreator {
	mock := &MockPreStatesCreator{ctrl: ctrl}
	mock.recorder = &MockPreStatesCreatorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPreStatesCreator) EXPECT() *MockPreStatesCreatorMockRecorder {
	return m.recorder
}

// CreatePreStates mocks base method.
func (m *MockPreStatesCreator) CreatePreStates(arg0 context.Context, arg1 StateManager) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreatePreStates", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreatePreStates indicates an expected call of CreatePreStates.
func (mr *MockPreStatesCreatorMockRecorder) CreatePreStates(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreatePreStates", reflect.TypeOf((*MockPreStatesCreator)(nil).CreatePreStates), arg0, arg1)
}

// MockPreCommitter is a mock of PreCommitter interface.
type MockPreCommitter struct {
	ctrl     *gomock.Controller
	recorder *MockPreCommitterMockRecorder
}

// MockPreCommitterMockRecorder is the mock recorder for MockPreCommitter.
type MockPreCommitterMockRecorder struct {
	mock *MockPreCommitter
}

// NewMockPreCommitter creates a new mock instance.
func NewMockPreCommitter(ctrl *gomock.Controller) *MockPreCommitter {
	mock := &MockPreCommitter{ctrl: ctrl}
	mock.recorder = &MockPreCommitterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPreCommitter) EXPECT() *MockPreCommitterMockRecorder {
	return m.recorder
}

// PreCommit mocks base method.
func (m *MockPreCommitter) PreCommit(arg0 context.Context, arg1 StateManager) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PreCommit", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// PreCommit indicates an expected call of PreCommit.
func (mr *MockPreCommitterMockRecorder) PreCommit(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PreCommit", reflect.TypeOf((*MockPreCommitter)(nil).PreCommit), arg0, arg1)
}

// MockCommitter is a mock of Committer interface.
type MockCommitter struct {
	ctrl     *gomock.Controller
	recorder *MockCommitterMockRecorder
}

// MockCommitterMockRecorder is the mock recorder for MockCommitter.
type MockCommitterMockRecorder struct {
	mock *MockCommitter
}

// NewMockCommitter creates a new mock instance.
func NewMockCommitter(ctrl *gomock.Controller) *MockCommitter {
	mock := &MockCommitter{ctrl: ctrl}
	mock.recorder = &MockCommitterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCommitter) EXPECT() *MockCommitterMockRecorder {
	return m.recorder
}

// Commit mocks base method.
func (m *MockCommitter) Commit(arg0 context.Context, arg1 StateManager) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Commit", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Commit indicates an expected call of Commit.
func (mr *MockCommitterMockRecorder) Commit(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Commit", reflect.TypeOf((*MockCommitter)(nil).Commit), arg0, arg1)
}

// MockPostSystemActionsCreator is a mock of PostSystemActionsCreator interface.
type MockPostSystemActionsCreator struct {
	ctrl     *gomock.Controller
	recorder *MockPostSystemActionsCreatorMockRecorder
}

// MockPostSystemActionsCreatorMockRecorder is the mock recorder for MockPostSystemActionsCreator.
type MockPostSystemActionsCreatorMockRecorder struct {
	mock *MockPostSystemActionsCreator
}

// NewMockPostSystemActionsCreator creates a new mock instance.
func NewMockPostSystemActionsCreator(ctrl *gomock.Controller) *MockPostSystemActionsCreator {
	mock := &MockPostSystemActionsCreator{ctrl: ctrl}
	mock.recorder = &MockPostSystemActionsCreatorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPostSystemActionsCreator) EXPECT() *MockPostSystemActionsCreatorMockRecorder {
	return m.recorder
}

// CreatePostSystemActions mocks base method.
func (m *MockPostSystemActionsCreator) CreatePostSystemActions(arg0 context.Context, arg1 StateReader) ([]action.Envelope, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreatePostSystemActions", arg0, arg1)
	ret0, _ := ret[0].([]action.Envelope)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreatePostSystemActions indicates an expected call of CreatePostSystemActions.
func (mr *MockPostSystemActionsCreatorMockRecorder) CreatePostSystemActions(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreatePostSystemActions", reflect.TypeOf((*MockPostSystemActionsCreator)(nil).CreatePostSystemActions), arg0, arg1)
}

// MockActionValidator is a mock of ActionValidator interface.
type MockActionValidator struct {
	ctrl     *gomock.Controller
	recorder *MockActionValidatorMockRecorder
}

// MockActionValidatorMockRecorder is the mock recorder for MockActionValidator.
type MockActionValidatorMockRecorder struct {
	mock *MockActionValidator
}

// NewMockActionValidator creates a new mock instance.
func NewMockActionValidator(ctrl *gomock.Controller) *MockActionValidator {
	mock := &MockActionValidator{ctrl: ctrl}
	mock.recorder = &MockActionValidatorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockActionValidator) EXPECT() *MockActionValidatorMockRecorder {
	return m.recorder
}

// Validate mocks base method.
func (m *MockActionValidator) Validate(arg0 context.Context, arg1 action.Envelope, arg2 StateReader) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Validate", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Validate indicates an expected call of Validate.
func (mr *MockActionValidatorMockRecorder) Validate(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Validate", reflect.TypeOf((*MockActionValidator)(nil).Validate), arg0, arg1, arg2)
}

// MockActionHandler is a mock of ActionHandler interface.
type MockActionHandler struct {
	ctrl     *gomock.Controller
	recorder *MockActionHandlerMockRecorder
}

// MockActionHandlerMockRecorder is the mock recorder for MockActionHandler.
type MockActionHandlerMockRecorder struct {
	mock *MockActionHandler
}

// NewMockActionHandler creates a new mock instance.
func NewMockActionHandler(ctrl *gomock.Controller) *MockActionHandler {
	mock := &MockActionHandler{ctrl: ctrl}
	mock.recorder = &MockActionHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockActionHandler) EXPECT() *MockActionHandlerMockRecorder {
	return m.recorder
}

// Handle mocks base method.
func (m *MockActionHandler) Handle(arg0 context.Context, arg1 action.Envelope, arg2 StateManager) (*action.Receipt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Handle", arg0, arg1, arg2)
	ret0, _ := ret[0].(*action.Receipt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Handle indicates an expected call of Handle.
func (mr *MockActionHandlerMockRecorder) Handle(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Handle", reflect.TypeOf((*MockActionHandler)(nil).Handle), arg0, arg1, arg2)
}
