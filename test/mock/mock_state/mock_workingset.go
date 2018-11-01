// Code generated by MockGen. DO NOT EDIT.
// Source: ./state/workingset.go

// Package mock_state is a generated GoMock package.
package mock_state

import (
	gomock "github.com/golang/mock/gomock"
	action "github.com/iotexproject/iotex-core/action"
	hash "github.com/iotexproject/iotex-core/pkg/hash"
	state "github.com/iotexproject/iotex-core/state"
	big "math/big"
	reflect "reflect"
)

// MockWorkingSet is a mock of WorkingSet interface
type MockWorkingSet struct {
	ctrl     *gomock.Controller
	recorder *MockWorkingSetMockRecorder
}

// MockWorkingSetMockRecorder is the mock recorder for MockWorkingSet
type MockWorkingSetMockRecorder struct {
	mock *MockWorkingSet
}

// NewMockWorkingSet creates a new mock instance
func NewMockWorkingSet(ctrl *gomock.Controller) *MockWorkingSet {
	mock := &MockWorkingSet{ctrl: ctrl}
	mock.recorder = &MockWorkingSetMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockWorkingSet) EXPECT() *MockWorkingSetMockRecorder {
	return m.recorder
}

// LoadOrCreateAccountState mocks base method
func (m *MockWorkingSet) LoadOrCreateAccountState(arg0 string, arg1 *big.Int) (*state.Account, error) {
	ret := m.ctrl.Call(m, "LoadOrCreateAccountState", arg0, arg1)
	ret0, _ := ret[0].(*state.Account)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LoadOrCreateAccountState indicates an expected call of LoadOrCreateAccountState
func (mr *MockWorkingSetMockRecorder) LoadOrCreateAccountState(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LoadOrCreateAccountState", reflect.TypeOf((*MockWorkingSet)(nil).LoadOrCreateAccountState), arg0, arg1)
}

// Nonce mocks base method
func (m *MockWorkingSet) Nonce(arg0 string) (uint64, error) {
	ret := m.ctrl.Call(m, "Nonce", arg0)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Nonce indicates an expected call of Nonce
func (mr *MockWorkingSetMockRecorder) Nonce(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Nonce", reflect.TypeOf((*MockWorkingSet)(nil).Nonce), arg0)
}

// CachedAccountState mocks base method
func (m *MockWorkingSet) CachedAccountState(arg0 string) (*state.Account, error) {
	ret := m.ctrl.Call(m, "CachedAccountState", arg0)
	ret0, _ := ret[0].(*state.Account)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CachedAccountState indicates an expected call of CachedAccountState
func (mr *MockWorkingSetMockRecorder) CachedAccountState(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CachedAccountState", reflect.TypeOf((*MockWorkingSet)(nil).CachedAccountState), arg0)
}

// RunActions mocks base method
func (m *MockWorkingSet) RunActions(arg0 string, arg1 uint64, arg2 []action.Action) (hash.Hash32B, error) {
	ret := m.ctrl.Call(m, "RunActions", arg0, arg1, arg2)
	ret0, _ := ret[0].(hash.Hash32B)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RunActions indicates an expected call of RunActions
func (mr *MockWorkingSetMockRecorder) RunActions(arg0, arg1, arg2 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RunActions", reflect.TypeOf((*MockWorkingSet)(nil).RunActions), arg0, arg1, arg2)
}

// Commit mocks base method
func (m *MockWorkingSet) Commit() error {
	ret := m.ctrl.Call(m, "Commit")
	ret0, _ := ret[0].(error)
	return ret0
}

// Commit indicates an expected call of Commit
func (mr *MockWorkingSetMockRecorder) Commit() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Commit", reflect.TypeOf((*MockWorkingSet)(nil).Commit))
}

// GetCodeHash mocks base method
func (m *MockWorkingSet) GetCodeHash(arg0 hash.PKHash) (hash.Hash32B, error) {
	ret := m.ctrl.Call(m, "GetCodeHash", arg0)
	ret0, _ := ret[0].(hash.Hash32B)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCodeHash indicates an expected call of GetCodeHash
func (mr *MockWorkingSetMockRecorder) GetCodeHash(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCodeHash", reflect.TypeOf((*MockWorkingSet)(nil).GetCodeHash), arg0)
}

// GetCode mocks base method
func (m *MockWorkingSet) GetCode(arg0 hash.PKHash) ([]byte, error) {
	ret := m.ctrl.Call(m, "GetCode", arg0)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCode indicates an expected call of GetCode
func (mr *MockWorkingSetMockRecorder) GetCode(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCode", reflect.TypeOf((*MockWorkingSet)(nil).GetCode), arg0)
}

// SetCode mocks base method
func (m *MockWorkingSet) SetCode(arg0 hash.PKHash, arg1 []byte) error {
	ret := m.ctrl.Call(m, "SetCode", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetCode indicates an expected call of SetCode
func (mr *MockWorkingSetMockRecorder) SetCode(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetCode", reflect.TypeOf((*MockWorkingSet)(nil).SetCode), arg0, arg1)
}

// GetContractState mocks base method
func (m *MockWorkingSet) GetContractState(arg0 hash.PKHash, arg1 hash.Hash32B) (hash.Hash32B, error) {
	ret := m.ctrl.Call(m, "GetContractState", arg0, arg1)
	ret0, _ := ret[0].(hash.Hash32B)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetContractState indicates an expected call of GetContractState
func (mr *MockWorkingSetMockRecorder) GetContractState(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetContractState", reflect.TypeOf((*MockWorkingSet)(nil).GetContractState), arg0, arg1)
}

// SetContractState mocks base method
func (m *MockWorkingSet) SetContractState(arg0 hash.PKHash, arg1, arg2 hash.Hash32B) error {
	ret := m.ctrl.Call(m, "SetContractState", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetContractState indicates an expected call of SetContractState
func (mr *MockWorkingSetMockRecorder) SetContractState(arg0, arg1, arg2 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetContractState", reflect.TypeOf((*MockWorkingSet)(nil).SetContractState), arg0, arg1, arg2)
}

// RootHash mocks base method
func (m *MockWorkingSet) RootHash() hash.Hash32B {
	ret := m.ctrl.Call(m, "RootHash")
	ret0, _ := ret[0].(hash.Hash32B)
	return ret0
}

// RootHash indicates an expected call of RootHash
func (mr *MockWorkingSetMockRecorder) RootHash() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RootHash", reflect.TypeOf((*MockWorkingSet)(nil).RootHash))
}

// Version mocks base method
func (m *MockWorkingSet) Version() uint64 {
	ret := m.ctrl.Call(m, "Version")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// Version indicates an expected call of Version
func (mr *MockWorkingSetMockRecorder) Version() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Version", reflect.TypeOf((*MockWorkingSet)(nil).Version))
}

// Height mocks base method
func (m *MockWorkingSet) Height() uint64 {
	ret := m.ctrl.Call(m, "Height")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// Height indicates an expected call of Height
func (mr *MockWorkingSetMockRecorder) Height() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Height", reflect.TypeOf((*MockWorkingSet)(nil).Height))
}

// State mocks base method
func (m *MockWorkingSet) State(arg0 hash.PKHash, arg1 state.State) (state.State, error) {
	ret := m.ctrl.Call(m, "State", arg0, arg1)
	ret0, _ := ret[0].(state.State)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// State indicates an expected call of State
func (mr *MockWorkingSetMockRecorder) State(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "State", reflect.TypeOf((*MockWorkingSet)(nil).State), arg0, arg1)
}

// CachedState mocks base method
func (m *MockWorkingSet) CachedState(arg0 hash.PKHash, arg1 state.State) (state.State, error) {
	ret := m.ctrl.Call(m, "CachedState", arg0, arg1)
	ret0, _ := ret[0].(state.State)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CachedState indicates an expected call of CachedState
func (mr *MockWorkingSetMockRecorder) CachedState(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CachedState", reflect.TypeOf((*MockWorkingSet)(nil).CachedState), arg0, arg1)
}

// PutState mocks base method
func (m *MockWorkingSet) PutState(arg0 hash.PKHash, arg1 state.State) error {
	ret := m.ctrl.Call(m, "PutState", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// PutState indicates an expected call of PutState
func (mr *MockWorkingSetMockRecorder) PutState(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PutState", reflect.TypeOf((*MockWorkingSet)(nil).PutState), arg0, arg1)
}
