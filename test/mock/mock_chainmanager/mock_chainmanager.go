// Code generated by MockGen. DO NOT EDIT.
// Source: ./action/protocol/protocol.go

// Package mock_chainmanager is a generated GoMock package.
package mock_chainmanager

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	action "github.com/iotexproject/iotex-core/action"
	protocol "github.com/iotexproject/iotex-core/action/protocol"
	db "github.com/iotexproject/iotex-core/db"
	hash "github.com/iotexproject/iotex-core/pkg/hash"
	state "github.com/iotexproject/iotex-core/state"
	reflect "reflect"
)

// MockProtocol is a mock of Protocol interface
type MockProtocol struct {
	ctrl     *gomock.Controller
	recorder *MockProtocolMockRecorder
}

// MockProtocolMockRecorder is the mock recorder for MockProtocol
type MockProtocolMockRecorder struct {
	mock *MockProtocol
}

// NewMockProtocol creates a new mock instance
func NewMockProtocol(ctrl *gomock.Controller) *MockProtocol {
	mock := &MockProtocol{ctrl: ctrl}
	mock.recorder = &MockProtocolMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockProtocol) EXPECT() *MockProtocolMockRecorder {
	return m.recorder
}

// Validate mocks base method
func (m *MockProtocol) Validate(arg0 context.Context, arg1 action.Action) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Validate", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Validate indicates an expected call of Validate
func (mr *MockProtocolMockRecorder) Validate(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Validate", reflect.TypeOf((*MockProtocol)(nil).Validate), arg0, arg1)
}

// Handle mocks base method
func (m *MockProtocol) Handle(arg0 context.Context, arg1 action.Action, arg2 protocol.StateManager) (*action.Receipt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Handle", arg0, arg1, arg2)
	ret0, _ := ret[0].(*action.Receipt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Handle indicates an expected call of Handle
func (mr *MockProtocolMockRecorder) Handle(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Handle", reflect.TypeOf((*MockProtocol)(nil).Handle), arg0, arg1, arg2)
}

// ReadState mocks base method
func (m *MockProtocol) ReadState(arg0 context.Context, arg1 protocol.StateManager, arg2 []byte, arg3 ...[]byte) ([]byte, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1, arg2}
	for _, a := range arg3 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ReadState", varargs...)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadState indicates an expected call of ReadState
func (mr *MockProtocolMockRecorder) ReadState(arg0, arg1, arg2 interface{}, arg3 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1, arg2}, arg3...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadState", reflect.TypeOf((*MockProtocol)(nil).ReadState), varargs...)
}

// MockActionValidator is a mock of ActionValidator interface
type MockActionValidator struct {
	ctrl     *gomock.Controller
	recorder *MockActionValidatorMockRecorder
}

// MockActionValidatorMockRecorder is the mock recorder for MockActionValidator
type MockActionValidatorMockRecorder struct {
	mock *MockActionValidator
}

// NewMockActionValidator creates a new mock instance
func NewMockActionValidator(ctrl *gomock.Controller) *MockActionValidator {
	mock := &MockActionValidator{ctrl: ctrl}
	mock.recorder = &MockActionValidatorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockActionValidator) EXPECT() *MockActionValidatorMockRecorder {
	return m.recorder
}

// Validate mocks base method
func (m *MockActionValidator) Validate(arg0 context.Context, arg1 action.Action) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Validate", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Validate indicates an expected call of Validate
func (mr *MockActionValidatorMockRecorder) Validate(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Validate", reflect.TypeOf((*MockActionValidator)(nil).Validate), arg0, arg1)
}

// MockActionEnvelopeValidator is a mock of ActionEnvelopeValidator interface
type MockActionEnvelopeValidator struct {
	ctrl     *gomock.Controller
	recorder *MockActionEnvelopeValidatorMockRecorder
}

// MockActionEnvelopeValidatorMockRecorder is the mock recorder for MockActionEnvelopeValidator
type MockActionEnvelopeValidatorMockRecorder struct {
	mock *MockActionEnvelopeValidator
}

// NewMockActionEnvelopeValidator creates a new mock instance
func NewMockActionEnvelopeValidator(ctrl *gomock.Controller) *MockActionEnvelopeValidator {
	mock := &MockActionEnvelopeValidator{ctrl: ctrl}
	mock.recorder = &MockActionEnvelopeValidatorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockActionEnvelopeValidator) EXPECT() *MockActionEnvelopeValidatorMockRecorder {
	return m.recorder
}

// Validate mocks base method
func (m *MockActionEnvelopeValidator) Validate(arg0 context.Context, arg1 action.SealedEnvelope) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Validate", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Validate indicates an expected call of Validate
func (mr *MockActionEnvelopeValidatorMockRecorder) Validate(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Validate", reflect.TypeOf((*MockActionEnvelopeValidator)(nil).Validate), arg0, arg1)
}

// MockActionHandler is a mock of ActionHandler interface
type MockActionHandler struct {
	ctrl     *gomock.Controller
	recorder *MockActionHandlerMockRecorder
}

// MockActionHandlerMockRecorder is the mock recorder for MockActionHandler
type MockActionHandlerMockRecorder struct {
	mock *MockActionHandler
}

// NewMockActionHandler creates a new mock instance
func NewMockActionHandler(ctrl *gomock.Controller) *MockActionHandler {
	mock := &MockActionHandler{ctrl: ctrl}
	mock.recorder = &MockActionHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockActionHandler) EXPECT() *MockActionHandlerMockRecorder {
	return m.recorder
}

// Handle mocks base method
func (m *MockActionHandler) Handle(arg0 context.Context, arg1 action.Action, arg2 protocol.StateManager) (*action.Receipt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Handle", arg0, arg1, arg2)
	ret0, _ := ret[0].(*action.Receipt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Handle indicates an expected call of Handle
func (mr *MockActionHandlerMockRecorder) Handle(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Handle", reflect.TypeOf((*MockActionHandler)(nil).Handle), arg0, arg1, arg2)
}

// MockChainManager is a mock of ChainManager interface
type MockChainManager struct {
	ctrl     *gomock.Controller
	recorder *MockChainManagerMockRecorder
}

// MockChainManagerMockRecorder is the mock recorder for MockChainManager
type MockChainManagerMockRecorder struct {
	mock *MockChainManager
}

// NewMockChainManager creates a new mock instance
func NewMockChainManager(ctrl *gomock.Controller) *MockChainManager {
	mock := &MockChainManager{ctrl: ctrl}
	mock.recorder = &MockChainManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockChainManager) EXPECT() *MockChainManagerMockRecorder {
	return m.recorder
}

// ChainID mocks base method
func (m *MockChainManager) ChainID() uint32 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ChainID")
	ret0, _ := ret[0].(uint32)
	return ret0
}

// ChainID indicates an expected call of ChainID
func (mr *MockChainManagerMockRecorder) ChainID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ChainID", reflect.TypeOf((*MockChainManager)(nil).ChainID))
}

// GetHashByHeight mocks base method
func (m *MockChainManager) GetHashByHeight(height uint64) (hash.Hash256, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetHashByHeight", height)
	ret0, _ := ret[0].(hash.Hash256)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetHashByHeight indicates an expected call of GetHashByHeight
func (mr *MockChainManagerMockRecorder) GetHashByHeight(height interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetHashByHeight", reflect.TypeOf((*MockChainManager)(nil).GetHashByHeight), height)
}

// StateByAddr mocks base method
func (m *MockChainManager) StateByAddr(address string) (*state.Account, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StateByAddr", address)
	ret0, _ := ret[0].(*state.Account)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StateByAddr indicates an expected call of StateByAddr
func (mr *MockChainManagerMockRecorder) StateByAddr(address interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StateByAddr", reflect.TypeOf((*MockChainManager)(nil).StateByAddr), address)
}

// Nonce mocks base method
func (m *MockChainManager) Nonce(addr string) (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Nonce", addr)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Nonce indicates an expected call of Nonce
func (mr *MockChainManagerMockRecorder) Nonce(addr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Nonce", reflect.TypeOf((*MockChainManager)(nil).Nonce), addr)
}

// CandidatesByHeight mocks base method
func (m *MockChainManager) CandidatesByHeight(height uint64) ([]*state.Candidate, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CandidatesByHeight", height)
	ret0, _ := ret[0].([]*state.Candidate)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CandidatesByHeight indicates an expected call of CandidatesByHeight
func (mr *MockChainManagerMockRecorder) CandidatesByHeight(height interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CandidatesByHeight", reflect.TypeOf((*MockChainManager)(nil).CandidatesByHeight), height)
}

// MockStateManager is a mock of StateManager interface
type MockStateManager struct {
	ctrl     *gomock.Controller
	recorder *MockStateManagerMockRecorder
}

// MockStateManagerMockRecorder is the mock recorder for MockStateManager
type MockStateManagerMockRecorder struct {
	mock *MockStateManager
}

// NewMockStateManager creates a new mock instance
func NewMockStateManager(ctrl *gomock.Controller) *MockStateManager {
	mock := &MockStateManager{ctrl: ctrl}
	mock.recorder = &MockStateManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockStateManager) EXPECT() *MockStateManagerMockRecorder {
	return m.recorder
}

// Height mocks base method
func (m *MockStateManager) Height() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Height")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// Height indicates an expected call of Height
func (mr *MockStateManagerMockRecorder) Height() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Height", reflect.TypeOf((*MockStateManager)(nil).Height))
}

// Snapshot mocks base method
func (m *MockStateManager) Snapshot() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Snapshot")
	ret0, _ := ret[0].(int)
	return ret0
}

// Snapshot indicates an expected call of Snapshot
func (mr *MockStateManagerMockRecorder) Snapshot() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Snapshot", reflect.TypeOf((*MockStateManager)(nil).Snapshot))
}

// Revert mocks base method
func (m *MockStateManager) Revert(arg0 int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Revert", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Revert indicates an expected call of Revert
func (mr *MockStateManagerMockRecorder) Revert(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Revert", reflect.TypeOf((*MockStateManager)(nil).Revert), arg0)
}

// State mocks base method
func (m *MockStateManager) State(arg0 hash.Hash160, arg1 interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "State", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// State indicates an expected call of State
func (mr *MockStateManagerMockRecorder) State(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "State", reflect.TypeOf((*MockStateManager)(nil).State), arg0, arg1)
}

// PutState mocks base method
func (m *MockStateManager) PutState(arg0 hash.Hash160, arg1 interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PutState", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// PutState indicates an expected call of PutState
func (mr *MockStateManagerMockRecorder) PutState(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PutState", reflect.TypeOf((*MockStateManager)(nil).PutState), arg0, arg1)
}

// DelState mocks base method
func (m *MockStateManager) DelState(pkHash hash.Hash160) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DelState", pkHash)
	ret0, _ := ret[0].(error)
	return ret0
}

// DelState indicates an expected call of DelState
func (mr *MockStateManagerMockRecorder) DelState(pkHash interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DelState", reflect.TypeOf((*MockStateManager)(nil).DelState), pkHash)
}

// GetDB mocks base method
func (m *MockStateManager) GetDB() db.KVStore {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDB")
	ret0, _ := ret[0].(db.KVStore)
	return ret0
}

// GetDB indicates an expected call of GetDB
func (mr *MockStateManagerMockRecorder) GetDB() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDB", reflect.TypeOf((*MockStateManager)(nil).GetDB))
}

// GetCachedBatch mocks base method
func (m *MockStateManager) GetCachedBatch() db.CachedBatch {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCachedBatch")
	ret0, _ := ret[0].(db.CachedBatch)
	return ret0
}

// GetCachedBatch indicates an expected call of GetCachedBatch
func (mr *MockStateManagerMockRecorder) GetCachedBatch() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCachedBatch", reflect.TypeOf((*MockStateManager)(nil).GetCachedBatch))
}
