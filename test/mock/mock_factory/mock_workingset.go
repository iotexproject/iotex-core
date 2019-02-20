// Code generated by MockGen. DO NOT EDIT.
// Source: ./state/factory/workingset.go

// Package mock_factory is a generated GoMock package.
package mock_factory

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	action "github.com/iotexproject/iotex-core/action"
	db "github.com/iotexproject/iotex-core/db"
	hash "github.com/iotexproject/iotex-core/pkg/hash"
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

// RunAction mocks base method
func (m *MockWorkingSet) RunAction(arg0 context.Context, arg1 action.SealedEnvelope) (*action.Receipt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RunAction", arg0, arg1)
	ret0, _ := ret[0].(*action.Receipt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RunAction indicates an expected call of RunAction
func (mr *MockWorkingSetMockRecorder) RunAction(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RunAction", reflect.TypeOf((*MockWorkingSet)(nil).RunAction), arg0, arg1)
}

// UpdateBlockLevelInfo mocks base method
func (m *MockWorkingSet) UpdateBlockLevelInfo(blockHeight uint64) hash.Hash256 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateBlockLevelInfo", blockHeight)
	ret0, _ := ret[0].(hash.Hash256)
	return ret0
}

// UpdateBlockLevelInfo indicates an expected call of UpdateBlockLevelInfo
func (mr *MockWorkingSetMockRecorder) UpdateBlockLevelInfo(blockHeight interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateBlockLevelInfo", reflect.TypeOf((*MockWorkingSet)(nil).UpdateBlockLevelInfo), blockHeight)
}

// RunActions mocks base method
func (m *MockWorkingSet) RunActions(arg0 context.Context, arg1 uint64, arg2 []action.SealedEnvelope) (hash.Hash256, []*action.Receipt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RunActions", arg0, arg1, arg2)
	ret0, _ := ret[0].(hash.Hash256)
	ret1, _ := ret[1].([]*action.Receipt)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// RunActions indicates an expected call of RunActions
func (mr *MockWorkingSetMockRecorder) RunActions(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RunActions", reflect.TypeOf((*MockWorkingSet)(nil).RunActions), arg0, arg1, arg2)
}

// Snapshot mocks base method
func (m *MockWorkingSet) Snapshot() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Snapshot")
	ret0, _ := ret[0].(int)
	return ret0
}

// Snapshot indicates an expected call of Snapshot
func (mr *MockWorkingSetMockRecorder) Snapshot() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Snapshot", reflect.TypeOf((*MockWorkingSet)(nil).Snapshot))
}

// Revert mocks base method
func (m *MockWorkingSet) Revert(arg0 int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Revert", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Revert indicates an expected call of Revert
func (mr *MockWorkingSetMockRecorder) Revert(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Revert", reflect.TypeOf((*MockWorkingSet)(nil).Revert), arg0)
}

// Commit mocks base method
func (m *MockWorkingSet) Commit() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Commit")
	ret0, _ := ret[0].(error)
	return ret0
}

// Commit indicates an expected call of Commit
func (mr *MockWorkingSetMockRecorder) Commit() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Commit", reflect.TypeOf((*MockWorkingSet)(nil).Commit))
}

// RootHash mocks base method
func (m *MockWorkingSet) RootHash() hash.Hash256 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RootHash")
	ret0, _ := ret[0].(hash.Hash256)
	return ret0
}

// RootHash indicates an expected call of RootHash
func (mr *MockWorkingSetMockRecorder) RootHash() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RootHash", reflect.TypeOf((*MockWorkingSet)(nil).RootHash))
}

// Digest mocks base method
func (m *MockWorkingSet) Digest() hash.Hash256 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Digest")
	ret0, _ := ret[0].(hash.Hash256)
	return ret0
}

// Digest indicates an expected call of Digest
func (mr *MockWorkingSetMockRecorder) Digest() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Digest", reflect.TypeOf((*MockWorkingSet)(nil).Digest))
}

// Version mocks base method
func (m *MockWorkingSet) Version() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Version")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// Version indicates an expected call of Version
func (mr *MockWorkingSetMockRecorder) Version() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Version", reflect.TypeOf((*MockWorkingSet)(nil).Version))
}

// Height mocks base method
func (m *MockWorkingSet) Height() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Height")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// Height indicates an expected call of Height
func (mr *MockWorkingSetMockRecorder) Height() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Height", reflect.TypeOf((*MockWorkingSet)(nil).Height))
}

// State mocks base method
func (m *MockWorkingSet) State(arg0 hash.Hash160, arg1 interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "State", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// State indicates an expected call of State
func (mr *MockWorkingSetMockRecorder) State(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "State", reflect.TypeOf((*MockWorkingSet)(nil).State), arg0, arg1)
}

// PutState mocks base method
func (m *MockWorkingSet) PutState(arg0 hash.Hash160, arg1 interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PutState", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// PutState indicates an expected call of PutState
func (mr *MockWorkingSetMockRecorder) PutState(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PutState", reflect.TypeOf((*MockWorkingSet)(nil).PutState), arg0, arg1)
}

// DelState mocks base method
func (m *MockWorkingSet) DelState(pkHash hash.Hash160) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DelState", pkHash)
	ret0, _ := ret[0].(error)
	return ret0
}

// DelState indicates an expected call of DelState
func (mr *MockWorkingSetMockRecorder) DelState(pkHash interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DelState", reflect.TypeOf((*MockWorkingSet)(nil).DelState), pkHash)
}

// GetDB mocks base method
func (m *MockWorkingSet) GetDB() db.KVStore {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDB")
	ret0, _ := ret[0].(db.KVStore)
	return ret0
}

// GetDB indicates an expected call of GetDB
func (mr *MockWorkingSetMockRecorder) GetDB() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDB", reflect.TypeOf((*MockWorkingSet)(nil).GetDB))
}

// GetCachedBatch mocks base method
func (m *MockWorkingSet) GetCachedBatch() db.CachedBatch {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCachedBatch")
	ret0, _ := ret[0].(db.CachedBatch)
	return ret0
}

// GetCachedBatch indicates an expected call of GetCachedBatch
func (mr *MockWorkingSetMockRecorder) GetCachedBatch() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCachedBatch", reflect.TypeOf((*MockWorkingSet)(nil).GetCachedBatch))
}
