// Code generated by MockGen. DO NOT EDIT.
// Source: ./db/batch/batch.go

// Package batch is a generated GoMock package.
package batch

import (
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockKVStoreBatch is a mock of KVStoreBatch interface
type MockKVStoreBatch struct {
	ctrl     *gomock.Controller
	recorder *MockKVStoreBatchMockRecorder
}

// MockKVStoreBatchMockRecorder is the mock recorder for MockKVStoreBatch
type MockKVStoreBatchMockRecorder struct {
	mock *MockKVStoreBatch
}

// NewMockKVStoreBatch creates a new mock instance
func NewMockKVStoreBatch(ctrl *gomock.Controller) *MockKVStoreBatch {
	mock := &MockKVStoreBatch{ctrl: ctrl}
	mock.recorder = &MockKVStoreBatchMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockKVStoreBatch) EXPECT() *MockKVStoreBatchMockRecorder {
	return m.recorder
}

// Lock mocks base method
func (m *MockKVStoreBatch) Lock() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Lock")
}

// Lock indicates an expected call of Lock
func (mr *MockKVStoreBatchMockRecorder) Lock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Lock", reflect.TypeOf((*MockKVStoreBatch)(nil).Lock))
}

// Unlock mocks base method
func (m *MockKVStoreBatch) Unlock() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Unlock")
}

// Unlock indicates an expected call of Unlock
func (mr *MockKVStoreBatchMockRecorder) Unlock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Unlock", reflect.TypeOf((*MockKVStoreBatch)(nil).Unlock))
}

// ClearAndUnlock mocks base method
func (m *MockKVStoreBatch) ClearAndUnlock() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ClearAndUnlock")
}

// ClearAndUnlock indicates an expected call of ClearAndUnlock
func (mr *MockKVStoreBatchMockRecorder) ClearAndUnlock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClearAndUnlock", reflect.TypeOf((*MockKVStoreBatch)(nil).ClearAndUnlock))
}

// Put mocks base method
func (m *MockKVStoreBatch) Put(arg0 string, arg1, arg2 []byte, arg3 string, arg4 ...interface{}) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1, arg2, arg3}
	for _, a := range arg4 {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Put", varargs...)
}

// Put indicates an expected call of Put
func (mr *MockKVStoreBatchMockRecorder) Put(arg0, arg1, arg2, arg3 interface{}, arg4 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1, arg2, arg3}, arg4...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Put", reflect.TypeOf((*MockKVStoreBatch)(nil).Put), varargs...)
}

// Delete mocks base method
func (m *MockKVStoreBatch) Delete(arg0 string, arg1 []byte, arg2 string, arg3 ...interface{}) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1, arg2}
	for _, a := range arg3 {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Delete", varargs...)
}

// Delete indicates an expected call of Delete
func (mr *MockKVStoreBatchMockRecorder) Delete(arg0, arg1, arg2 interface{}, arg3 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1, arg2}, arg3...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockKVStoreBatch)(nil).Delete), varargs...)
}

// Size mocks base method
func (m *MockKVStoreBatch) Size() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Size")
	ret0, _ := ret[0].(int)
	return ret0
}

// Size indicates an expected call of Size
func (mr *MockKVStoreBatchMockRecorder) Size() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Size", reflect.TypeOf((*MockKVStoreBatch)(nil).Size))
}

// Entry mocks base method
func (m *MockKVStoreBatch) Entry(arg0 int) (*WriteInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Entry", arg0)
	ret0, _ := ret[0].(*WriteInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Entry indicates an expected call of Entry
func (mr *MockKVStoreBatchMockRecorder) Entry(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Entry", reflect.TypeOf((*MockKVStoreBatch)(nil).Entry), arg0)
}

// SerializeQueue mocks base method
func (m *MockKVStoreBatch) SerializeQueue() []byte {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SerializeQueue")
	ret0, _ := ret[0].([]byte)
	return ret0
}

// SerializeQueue indicates an expected call of SerializeQueue
func (mr *MockKVStoreBatchMockRecorder) SerializeQueue() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SerializeQueue", reflect.TypeOf((*MockKVStoreBatch)(nil).SerializeQueue))
}

// ExcludeEntries mocks base method
func (m *MockKVStoreBatch) ExcludeEntries(arg0 string, arg1 WriteType) KVStoreBatch {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ExcludeEntries", arg0, arg1)
	ret0, _ := ret[0].(KVStoreBatch)
	return ret0
}

// ExcludeEntries indicates an expected call of ExcludeEntries
func (mr *MockKVStoreBatchMockRecorder) ExcludeEntries(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExcludeEntries", reflect.TypeOf((*MockKVStoreBatch)(nil).ExcludeEntries), arg0, arg1)
}

// Clear mocks base method
func (m *MockKVStoreBatch) Clear() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Clear")
}

// Clear indicates an expected call of Clear
func (mr *MockKVStoreBatchMockRecorder) Clear() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Clear", reflect.TypeOf((*MockKVStoreBatch)(nil).Clear))
}

// CloneBatch mocks base method
func (m *MockKVStoreBatch) CloneBatch() KVStoreBatch {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloneBatch")
	ret0, _ := ret[0].(KVStoreBatch)
	return ret0
}

// CloneBatch indicates an expected call of CloneBatch
func (mr *MockKVStoreBatchMockRecorder) CloneBatch() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloneBatch", reflect.TypeOf((*MockKVStoreBatch)(nil).CloneBatch))
}

// batch mocks base method
func (m *MockKVStoreBatch) batch(op WriteType, namespace string, key, value []byte, errorFormat string, errorArgs ...interface{}) {
	m.ctrl.T.Helper()
	varargs := []interface{}{op, namespace, key, value, errorFormat}
	for _, a := range errorArgs {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "batch", varargs...)
}

// batch indicates an expected call of batch
func (mr *MockKVStoreBatchMockRecorder) batch(op, namespace, key, value, errorFormat interface{}, errorArgs ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{op, namespace, key, value, errorFormat}, errorArgs...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "batch", reflect.TypeOf((*MockKVStoreBatch)(nil).batch), varargs...)
}

// truncate mocks base method
func (m *MockKVStoreBatch) truncate(arg0 int) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "truncate", arg0)
}

// truncate indicates an expected call of truncate
func (mr *MockKVStoreBatchMockRecorder) truncate(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "truncate", reflect.TypeOf((*MockKVStoreBatch)(nil).truncate), arg0)
}

// MockCachedBatch is a mock of CachedBatch interface
type MockCachedBatch struct {
	ctrl     *gomock.Controller
	recorder *MockCachedBatchMockRecorder
}

// MockCachedBatchMockRecorder is the mock recorder for MockCachedBatch
type MockCachedBatchMockRecorder struct {
	mock *MockCachedBatch
}

// NewMockCachedBatch creates a new mock instance
func NewMockCachedBatch(ctrl *gomock.Controller) *MockCachedBatch {
	mock := &MockCachedBatch{ctrl: ctrl}
	mock.recorder = &MockCachedBatchMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockCachedBatch) EXPECT() *MockCachedBatchMockRecorder {
	return m.recorder
}

// Lock mocks base method
func (m *MockCachedBatch) Lock() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Lock")
}

// Lock indicates an expected call of Lock
func (mr *MockCachedBatchMockRecorder) Lock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Lock", reflect.TypeOf((*MockCachedBatch)(nil).Lock))
}

// Unlock mocks base method
func (m *MockCachedBatch) Unlock() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Unlock")
}

// Unlock indicates an expected call of Unlock
func (mr *MockCachedBatchMockRecorder) Unlock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Unlock", reflect.TypeOf((*MockCachedBatch)(nil).Unlock))
}

// ClearAndUnlock mocks base method
func (m *MockCachedBatch) ClearAndUnlock() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ClearAndUnlock")
}

// ClearAndUnlock indicates an expected call of ClearAndUnlock
func (mr *MockCachedBatchMockRecorder) ClearAndUnlock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClearAndUnlock", reflect.TypeOf((*MockCachedBatch)(nil).ClearAndUnlock))
}

// Put mocks base method
func (m *MockCachedBatch) Put(arg0 string, arg1, arg2 []byte, arg3 string, arg4 ...interface{}) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1, arg2, arg3}
	for _, a := range arg4 {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Put", varargs...)
}

// Put indicates an expected call of Put
func (mr *MockCachedBatchMockRecorder) Put(arg0, arg1, arg2, arg3 interface{}, arg4 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1, arg2, arg3}, arg4...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Put", reflect.TypeOf((*MockCachedBatch)(nil).Put), varargs...)
}

// Delete mocks base method
func (m *MockCachedBatch) Delete(arg0 string, arg1 []byte, arg2 string, arg3 ...interface{}) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1, arg2}
	for _, a := range arg3 {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Delete", varargs...)
}

// Delete indicates an expected call of Delete
func (mr *MockCachedBatchMockRecorder) Delete(arg0, arg1, arg2 interface{}, arg3 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1, arg2}, arg3...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockCachedBatch)(nil).Delete), varargs...)
}

// Size mocks base method
func (m *MockCachedBatch) Size() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Size")
	ret0, _ := ret[0].(int)
	return ret0
}

// Size indicates an expected call of Size
func (mr *MockCachedBatchMockRecorder) Size() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Size", reflect.TypeOf((*MockCachedBatch)(nil).Size))
}

// Entry mocks base method
func (m *MockCachedBatch) Entry(arg0 int) (*WriteInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Entry", arg0)
	ret0, _ := ret[0].(*WriteInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Entry indicates an expected call of Entry
func (mr *MockCachedBatchMockRecorder) Entry(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Entry", reflect.TypeOf((*MockCachedBatch)(nil).Entry), arg0)
}

// SerializeQueue mocks base method
func (m *MockCachedBatch) SerializeQueue() []byte {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SerializeQueue")
	ret0, _ := ret[0].([]byte)
	return ret0
}

// SerializeQueue indicates an expected call of SerializeQueue
func (mr *MockCachedBatchMockRecorder) SerializeQueue() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SerializeQueue", reflect.TypeOf((*MockCachedBatch)(nil).SerializeQueue))
}

// ExcludeEntries mocks base method
func (m *MockCachedBatch) ExcludeEntries(arg0 string, arg1 WriteType) KVStoreBatch {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ExcludeEntries", arg0, arg1)
	ret0, _ := ret[0].(KVStoreBatch)
	return ret0
}

// ExcludeEntries indicates an expected call of ExcludeEntries
func (mr *MockCachedBatchMockRecorder) ExcludeEntries(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExcludeEntries", reflect.TypeOf((*MockCachedBatch)(nil).ExcludeEntries), arg0, arg1)
}

// Clear mocks base method
func (m *MockCachedBatch) Clear() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Clear")
}

// Clear indicates an expected call of Clear
func (mr *MockCachedBatchMockRecorder) Clear() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Clear", reflect.TypeOf((*MockCachedBatch)(nil).Clear))
}

// CloneBatch mocks base method
func (m *MockCachedBatch) CloneBatch() KVStoreBatch {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloneBatch")
	ret0, _ := ret[0].(KVStoreBatch)
	return ret0
}

// CloneBatch indicates an expected call of CloneBatch
func (mr *MockCachedBatchMockRecorder) CloneBatch() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloneBatch", reflect.TypeOf((*MockCachedBatch)(nil).CloneBatch))
}

// batch mocks base method
func (m *MockCachedBatch) batch(op WriteType, namespace string, key, value []byte, errorFormat string, errorArgs ...interface{}) {
	m.ctrl.T.Helper()
	varargs := []interface{}{op, namespace, key, value, errorFormat}
	for _, a := range errorArgs {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "batch", varargs...)
}

// batch indicates an expected call of batch
func (mr *MockCachedBatchMockRecorder) batch(op, namespace, key, value, errorFormat interface{}, errorArgs ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{op, namespace, key, value, errorFormat}, errorArgs...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "batch", reflect.TypeOf((*MockCachedBatch)(nil).batch), varargs...)
}

// truncate mocks base method
func (m *MockCachedBatch) truncate(arg0 int) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "truncate", arg0)
}

// truncate indicates an expected call of truncate
func (mr *MockCachedBatchMockRecorder) truncate(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "truncate", reflect.TypeOf((*MockCachedBatch)(nil).truncate), arg0)
}

// Get mocks base method
func (m *MockCachedBatch) Get(arg0 string, arg1 []byte) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get
func (mr *MockCachedBatchMockRecorder) Get(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockCachedBatch)(nil).Get), arg0, arg1)
}

// Snapshot mocks base method
func (m *MockCachedBatch) Snapshot() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Snapshot")
	ret0, _ := ret[0].(int)
	return ret0
}

// Snapshot indicates an expected call of Snapshot
func (mr *MockCachedBatchMockRecorder) Snapshot() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Snapshot", reflect.TypeOf((*MockCachedBatch)(nil).Snapshot))
}

// Revert mocks base method
func (m *MockCachedBatch) Revert(arg0 int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Revert", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Revert indicates an expected call of Revert
func (mr *MockCachedBatchMockRecorder) Revert(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Revert", reflect.TypeOf((*MockCachedBatch)(nil).Revert), arg0)
}
