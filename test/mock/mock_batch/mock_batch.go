// Code generated by MockGen. DO NOT EDIT.
// Source: ./db/batch/batch.go

// Package mock_batch is a generated GoMock package.
package mock_batch

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	batch "github.com/iotexproject/iotex-core/v2/db/batch"
)

// MockKVStoreBatch is a mock of KVStoreBatch interface.
type MockKVStoreBatch struct {
	ctrl     *gomock.Controller
	recorder *MockKVStoreBatchMockRecorder
}

// MockKVStoreBatchMockRecorder is the mock recorder for MockKVStoreBatch.
type MockKVStoreBatchMockRecorder struct {
	mock *MockKVStoreBatch
}

// NewMockKVStoreBatch creates a new mock instance.
func NewMockKVStoreBatch(ctrl *gomock.Controller) *MockKVStoreBatch {
	mock := &MockKVStoreBatch{ctrl: ctrl}
	mock.recorder = &MockKVStoreBatchMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockKVStoreBatch) EXPECT() *MockKVStoreBatchMockRecorder {
	return m.recorder
}

// AddFillPercent mocks base method.
func (m *MockKVStoreBatch) AddFillPercent(arg0 string, arg1 float64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddFillPercent", arg0, arg1)
}

// AddFillPercent indicates an expected call of AddFillPercent.
func (mr *MockKVStoreBatchMockRecorder) AddFillPercent(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddFillPercent", reflect.TypeOf((*MockKVStoreBatch)(nil).AddFillPercent), arg0, arg1)
}

// Append mocks base method.
func (m *MockKVStoreBatch) Append(arg0 batch.KVStoreBatch) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Append", arg0)
}

// Append indicates an expected call of Append.
func (mr *MockKVStoreBatchMockRecorder) Append(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Append", reflect.TypeOf((*MockKVStoreBatch)(nil).Append), arg0)
}

// CheckFillPercent mocks base method.
func (m *MockKVStoreBatch) CheckFillPercent(arg0 string) (float64, bool) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CheckFillPercent", arg0)
	ret0, _ := ret[0].(float64)
	ret1, _ := ret[1].(bool)
	return ret0, ret1
}

// CheckFillPercent indicates an expected call of CheckFillPercent.
func (mr *MockKVStoreBatchMockRecorder) CheckFillPercent(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CheckFillPercent", reflect.TypeOf((*MockKVStoreBatch)(nil).CheckFillPercent), arg0)
}

// Clear mocks base method.
func (m *MockKVStoreBatch) Clear() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Clear")
}

// Clear indicates an expected call of Clear.
func (mr *MockKVStoreBatchMockRecorder) Clear() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Clear", reflect.TypeOf((*MockKVStoreBatch)(nil).Clear))
}

// ClearAndUnlock mocks base method.
func (m *MockKVStoreBatch) ClearAndUnlock() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ClearAndUnlock")
}

// ClearAndUnlock indicates an expected call of ClearAndUnlock.
func (mr *MockKVStoreBatchMockRecorder) ClearAndUnlock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClearAndUnlock", reflect.TypeOf((*MockKVStoreBatch)(nil).ClearAndUnlock))
}

// Delete mocks base method.
func (m *MockKVStoreBatch) Delete(arg0 string, arg1 []byte, arg2 string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Delete", arg0, arg1, arg2)
}

// Delete indicates an expected call of Delete.
func (mr *MockKVStoreBatchMockRecorder) Delete(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockKVStoreBatch)(nil).Delete), arg0, arg1, arg2)
}

// Entry mocks base method.
func (m *MockKVStoreBatch) Entry(arg0 int) (*batch.WriteInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Entry", arg0)
	ret0, _ := ret[0].(*batch.WriteInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Entry indicates an expected call of Entry.
func (mr *MockKVStoreBatchMockRecorder) Entry(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Entry", reflect.TypeOf((*MockKVStoreBatch)(nil).Entry), arg0)
}

// Lock mocks base method.
func (m *MockKVStoreBatch) Lock() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Lock")
}

// Lock indicates an expected call of Lock.
func (mr *MockKVStoreBatchMockRecorder) Lock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Lock", reflect.TypeOf((*MockKVStoreBatch)(nil).Lock))
}

// Put mocks base method.
func (m *MockKVStoreBatch) Put(arg0 string, arg1, arg2 []byte, arg3 string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Put", arg0, arg1, arg2, arg3)
}

// Put indicates an expected call of Put.
func (mr *MockKVStoreBatchMockRecorder) Put(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Put", reflect.TypeOf((*MockKVStoreBatch)(nil).Put), arg0, arg1, arg2, arg3)
}

// SerializeQueue mocks base method.
func (m *MockKVStoreBatch) SerializeQueue(arg0 batch.WriteInfoSerialize, arg1 batch.WriteInfoFilter) []byte {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SerializeQueue", arg0, arg1)
	ret0, _ := ret[0].([]byte)
	return ret0
}

// SerializeQueue indicates an expected call of SerializeQueue.
func (mr *MockKVStoreBatchMockRecorder) SerializeQueue(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SerializeQueue", reflect.TypeOf((*MockKVStoreBatch)(nil).SerializeQueue), arg0, arg1)
}

// Size mocks base method.
func (m *MockKVStoreBatch) Size() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Size")
	ret0, _ := ret[0].(int)
	return ret0
}

// Size indicates an expected call of Size.
func (mr *MockKVStoreBatchMockRecorder) Size() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Size", reflect.TypeOf((*MockKVStoreBatch)(nil).Size))
}

// Translate mocks base method.
func (m *MockKVStoreBatch) Translate(arg0 batch.WriteInfoTranslate) batch.KVStoreBatch {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Translate", arg0)
	ret0, _ := ret[0].(batch.KVStoreBatch)
	return ret0
}

// Translate indicates an expected call of Translate.
func (mr *MockKVStoreBatchMockRecorder) Translate(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Translate", reflect.TypeOf((*MockKVStoreBatch)(nil).Translate), arg0)
}

// Unlock mocks base method.
func (m *MockKVStoreBatch) Unlock() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Unlock")
}

// Unlock indicates an expected call of Unlock.
func (mr *MockKVStoreBatchMockRecorder) Unlock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Unlock", reflect.TypeOf((*MockKVStoreBatch)(nil).Unlock))
}

// MockCachedBatch is a mock of CachedBatch interface.
type MockCachedBatch struct {
	ctrl     *gomock.Controller
	recorder *MockCachedBatchMockRecorder
}

// MockCachedBatchMockRecorder is the mock recorder for MockCachedBatch.
type MockCachedBatchMockRecorder struct {
	mock *MockCachedBatch
}

// NewMockCachedBatch creates a new mock instance.
func NewMockCachedBatch(ctrl *gomock.Controller) *MockCachedBatch {
	mock := &MockCachedBatch{ctrl: ctrl}
	mock.recorder = &MockCachedBatchMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCachedBatch) EXPECT() *MockCachedBatchMockRecorder {
	return m.recorder
}

// AddFillPercent mocks base method.
func (m *MockCachedBatch) AddFillPercent(arg0 string, arg1 float64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddFillPercent", arg0, arg1)
}

// AddFillPercent indicates an expected call of AddFillPercent.
func (mr *MockCachedBatchMockRecorder) AddFillPercent(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddFillPercent", reflect.TypeOf((*MockCachedBatch)(nil).AddFillPercent), arg0, arg1)
}

// Append mocks base method.
func (m *MockCachedBatch) Append(arg0 batch.KVStoreBatch) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Append", arg0)
}

// Append indicates an expected call of Append.
func (mr *MockCachedBatchMockRecorder) Append(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Append", reflect.TypeOf((*MockCachedBatch)(nil).Append), arg0)
}

// CheckFillPercent mocks base method.
func (m *MockCachedBatch) CheckFillPercent(arg0 string) (float64, bool) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CheckFillPercent", arg0)
	ret0, _ := ret[0].(float64)
	ret1, _ := ret[1].(bool)
	return ret0, ret1
}

// CheckFillPercent indicates an expected call of CheckFillPercent.
func (mr *MockCachedBatchMockRecorder) CheckFillPercent(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CheckFillPercent", reflect.TypeOf((*MockCachedBatch)(nil).CheckFillPercent), arg0)
}

// Clear mocks base method.
func (m *MockCachedBatch) Clear() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Clear")
}

// Clear indicates an expected call of Clear.
func (mr *MockCachedBatchMockRecorder) Clear() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Clear", reflect.TypeOf((*MockCachedBatch)(nil).Clear))
}

// ClearAndUnlock mocks base method.
func (m *MockCachedBatch) ClearAndUnlock() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ClearAndUnlock")
}

// ClearAndUnlock indicates an expected call of ClearAndUnlock.
func (mr *MockCachedBatchMockRecorder) ClearAndUnlock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClearAndUnlock", reflect.TypeOf((*MockCachedBatch)(nil).ClearAndUnlock))
}

// Delete mocks base method.
func (m *MockCachedBatch) Delete(arg0 string, arg1 []byte, arg2 string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Delete", arg0, arg1, arg2)
}

// Delete indicates an expected call of Delete.
func (mr *MockCachedBatchMockRecorder) Delete(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockCachedBatch)(nil).Delete), arg0, arg1, arg2)
}

// Entry mocks base method.
func (m *MockCachedBatch) Entry(arg0 int) (*batch.WriteInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Entry", arg0)
	ret0, _ := ret[0].(*batch.WriteInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Entry indicates an expected call of Entry.
func (mr *MockCachedBatchMockRecorder) Entry(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Entry", reflect.TypeOf((*MockCachedBatch)(nil).Entry), arg0)
}

// Get mocks base method.
func (m *MockCachedBatch) Get(arg0 string, arg1 []byte) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockCachedBatchMockRecorder) Get(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockCachedBatch)(nil).Get), arg0, arg1)
}

// Lock mocks base method.
func (m *MockCachedBatch) Lock() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Lock")
}

// Lock indicates an expected call of Lock.
func (mr *MockCachedBatchMockRecorder) Lock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Lock", reflect.TypeOf((*MockCachedBatch)(nil).Lock))
}

// Put mocks base method.
func (m *MockCachedBatch) Put(arg0 string, arg1, arg2 []byte, arg3 string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Put", arg0, arg1, arg2, arg3)
}

// Put indicates an expected call of Put.
func (mr *MockCachedBatchMockRecorder) Put(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Put", reflect.TypeOf((*MockCachedBatch)(nil).Put), arg0, arg1, arg2, arg3)
}

// ResetSnapshots mocks base method.
func (m *MockCachedBatch) ResetSnapshots() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ResetSnapshots")
}

// ResetSnapshots indicates an expected call of ResetSnapshots.
func (mr *MockCachedBatchMockRecorder) ResetSnapshots() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResetSnapshots", reflect.TypeOf((*MockCachedBatch)(nil).ResetSnapshots))
}

// RevertSnapshot mocks base method.
func (m *MockCachedBatch) RevertSnapshot(arg0 int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RevertSnapshot", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// RevertSnapshot indicates an expected call of RevertSnapshot.
func (mr *MockCachedBatchMockRecorder) RevertSnapshot(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RevertSnapshot", reflect.TypeOf((*MockCachedBatch)(nil).RevertSnapshot), arg0)
}

// SerializeQueue mocks base method.
func (m *MockCachedBatch) SerializeQueue(arg0 batch.WriteInfoSerialize, arg1 batch.WriteInfoFilter) []byte {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SerializeQueue", arg0, arg1)
	ret0, _ := ret[0].([]byte)
	return ret0
}

// SerializeQueue indicates an expected call of SerializeQueue.
func (mr *MockCachedBatchMockRecorder) SerializeQueue(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SerializeQueue", reflect.TypeOf((*MockCachedBatch)(nil).SerializeQueue), arg0, arg1)
}

// Size mocks base method.
func (m *MockCachedBatch) Size() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Size")
	ret0, _ := ret[0].(int)
	return ret0
}

// Size indicates an expected call of Size.
func (mr *MockCachedBatchMockRecorder) Size() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Size", reflect.TypeOf((*MockCachedBatch)(nil).Size))
}

// Snapshot mocks base method.
func (m *MockCachedBatch) Snapshot() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Snapshot")
	ret0, _ := ret[0].(int)
	return ret0
}

// Snapshot indicates an expected call of Snapshot.
func (mr *MockCachedBatchMockRecorder) Snapshot() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Snapshot", reflect.TypeOf((*MockCachedBatch)(nil).Snapshot))
}

// Translate mocks base method.
func (m *MockCachedBatch) Translate(arg0 batch.WriteInfoTranslate) batch.KVStoreBatch {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Translate", arg0)
	ret0, _ := ret[0].(batch.KVStoreBatch)
	return ret0
}

// Translate indicates an expected call of Translate.
func (mr *MockCachedBatchMockRecorder) Translate(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Translate", reflect.TypeOf((*MockCachedBatch)(nil).Translate), arg0)
}

// Unlock mocks base method.
func (m *MockCachedBatch) Unlock() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Unlock")
}

// Unlock indicates an expected call of Unlock.
func (mr *MockCachedBatchMockRecorder) Unlock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Unlock", reflect.TypeOf((*MockCachedBatch)(nil).Unlock))
}

// MockSnapshot is a mock of Snapshot interface.
type MockSnapshot struct {
	ctrl     *gomock.Controller
	recorder *MockSnapshotMockRecorder
}

// MockSnapshotMockRecorder is the mock recorder for MockSnapshot.
type MockSnapshotMockRecorder struct {
	mock *MockSnapshot
}

// NewMockSnapshot creates a new mock instance.
func NewMockSnapshot(ctrl *gomock.Controller) *MockSnapshot {
	mock := &MockSnapshot{ctrl: ctrl}
	mock.recorder = &MockSnapshotMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSnapshot) EXPECT() *MockSnapshotMockRecorder {
	return m.recorder
}

// ResetSnapshots mocks base method.
func (m *MockSnapshot) ResetSnapshots() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ResetSnapshots")
}

// ResetSnapshots indicates an expected call of ResetSnapshots.
func (mr *MockSnapshotMockRecorder) ResetSnapshots() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResetSnapshots", reflect.TypeOf((*MockSnapshot)(nil).ResetSnapshots))
}

// RevertSnapshot mocks base method.
func (m *MockSnapshot) RevertSnapshot(arg0 int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RevertSnapshot", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// RevertSnapshot indicates an expected call of RevertSnapshot.
func (mr *MockSnapshotMockRecorder) RevertSnapshot(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RevertSnapshot", reflect.TypeOf((*MockSnapshot)(nil).RevertSnapshot), arg0)
}

// Snapshot mocks base method.
func (m *MockSnapshot) Snapshot() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Snapshot")
	ret0, _ := ret[0].(int)
	return ret0
}

// Snapshot indicates an expected call of Snapshot.
func (mr *MockSnapshotMockRecorder) Snapshot() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Snapshot", reflect.TypeOf((*MockSnapshot)(nil).Snapshot))
}
