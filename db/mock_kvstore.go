// Code generated by MockGen. DO NOT EDIT.
// Source: ./db/kvstore.go

// Package db is a generated GoMock package.
package db

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	batch "github.com/iotexproject/iotex-core/db/batch"
	reflect "reflect"
)

// MockKVStoreBasic is a mock of KVStoreBasic interface
type MockKVStoreBasic struct {
	ctrl     *gomock.Controller
	recorder *MockKVStoreBasicMockRecorder
}

// MockKVStoreBasicMockRecorder is the mock recorder for MockKVStoreBasic
type MockKVStoreBasicMockRecorder struct {
	mock *MockKVStoreBasic
}

// NewMockKVStoreBasic creates a new mock instance
func NewMockKVStoreBasic(ctrl *gomock.Controller) *MockKVStoreBasic {
	mock := &MockKVStoreBasic{ctrl: ctrl}
	mock.recorder = &MockKVStoreBasicMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockKVStoreBasic) EXPECT() *MockKVStoreBasicMockRecorder {
	return m.recorder
}

// Start mocks base method
func (m *MockKVStoreBasic) Start(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start
func (mr *MockKVStoreBasicMockRecorder) Start(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockKVStoreBasic)(nil).Start), arg0)
}

// Stop mocks base method
func (m *MockKVStoreBasic) Stop(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop
func (mr *MockKVStoreBasicMockRecorder) Stop(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockKVStoreBasic)(nil).Stop), arg0)
}

// Put mocks base method
func (m *MockKVStoreBasic) Put(arg0 string, arg1, arg2 []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Put", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Put indicates an expected call of Put
func (mr *MockKVStoreBasicMockRecorder) Put(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Put", reflect.TypeOf((*MockKVStoreBasic)(nil).Put), arg0, arg1, arg2)
}

// Get mocks base method
func (m *MockKVStoreBasic) Get(arg0 string, arg1 []byte) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get
func (mr *MockKVStoreBasicMockRecorder) Get(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockKVStoreBasic)(nil).Get), arg0, arg1)
}

// Delete mocks base method
func (m *MockKVStoreBasic) Delete(arg0 string, arg1 []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete
func (mr *MockKVStoreBasicMockRecorder) Delete(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockKVStoreBasic)(nil).Delete), arg0, arg1)
}

// MockKVStore is a mock of KVStore interface
type MockKVStore struct {
	ctrl     *gomock.Controller
	recorder *MockKVStoreMockRecorder
}

// MockKVStoreMockRecorder is the mock recorder for MockKVStore
type MockKVStoreMockRecorder struct {
	mock *MockKVStore
}

// NewMockKVStore creates a new mock instance
func NewMockKVStore(ctrl *gomock.Controller) *MockKVStore {
	mock := &MockKVStore{ctrl: ctrl}
	mock.recorder = &MockKVStoreMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockKVStore) EXPECT() *MockKVStoreMockRecorder {
	return m.recorder
}

// Start mocks base method
func (m *MockKVStore) Start(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start
func (mr *MockKVStoreMockRecorder) Start(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockKVStore)(nil).Start), arg0)
}

// Stop mocks base method
func (m *MockKVStore) Stop(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop
func (mr *MockKVStoreMockRecorder) Stop(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockKVStore)(nil).Stop), arg0)
}

// Put mocks base method
func (m *MockKVStore) Put(arg0 string, arg1, arg2 []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Put", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Put indicates an expected call of Put
func (mr *MockKVStoreMockRecorder) Put(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Put", reflect.TypeOf((*MockKVStore)(nil).Put), arg0, arg1, arg2)
}

// Get mocks base method
func (m *MockKVStore) Get(arg0 string, arg1 []byte) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get
func (mr *MockKVStoreMockRecorder) Get(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockKVStore)(nil).Get), arg0, arg1)
}

// Delete mocks base method
func (m *MockKVStore) Delete(arg0 string, arg1 []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete
func (mr *MockKVStoreMockRecorder) Delete(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockKVStore)(nil).Delete), arg0, arg1)
}

// WriteBatch mocks base method
func (m *MockKVStore) WriteBatch(arg0 batch.KVStoreBatch) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteBatch", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteBatch indicates an expected call of WriteBatch
func (mr *MockKVStoreMockRecorder) WriteBatch(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteBatch", reflect.TypeOf((*MockKVStore)(nil).WriteBatch), arg0)
}

// Filter mocks base method
func (m *MockKVStore) Filter(arg0 string, arg1 Condition, arg2, arg3 []byte) ([][]byte, [][]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Filter", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].([][]byte)
	ret1, _ := ret[1].([][]byte)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Filter indicates an expected call of Filter
func (mr *MockKVStoreMockRecorder) Filter(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Filter", reflect.TypeOf((*MockKVStore)(nil).Filter), arg0, arg1, arg2, arg3)
}

// MockKVStoreWithRange is a mock of KVStoreWithRange interface
type MockKVStoreWithRange struct {
	ctrl     *gomock.Controller
	recorder *MockKVStoreWithRangeMockRecorder
}

// MockKVStoreWithRangeMockRecorder is the mock recorder for MockKVStoreWithRange
type MockKVStoreWithRangeMockRecorder struct {
	mock *MockKVStoreWithRange
}

// NewMockKVStoreWithRange creates a new mock instance
func NewMockKVStoreWithRange(ctrl *gomock.Controller) *MockKVStoreWithRange {
	mock := &MockKVStoreWithRange{ctrl: ctrl}
	mock.recorder = &MockKVStoreWithRangeMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockKVStoreWithRange) EXPECT() *MockKVStoreWithRangeMockRecorder {
	return m.recorder
}

// Start mocks base method
func (m *MockKVStoreWithRange) Start(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start
func (mr *MockKVStoreWithRangeMockRecorder) Start(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockKVStoreWithRange)(nil).Start), arg0)
}

// Stop mocks base method
func (m *MockKVStoreWithRange) Stop(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop
func (mr *MockKVStoreWithRangeMockRecorder) Stop(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockKVStoreWithRange)(nil).Stop), arg0)
}

// Put mocks base method
func (m *MockKVStoreWithRange) Put(arg0 string, arg1, arg2 []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Put", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Put indicates an expected call of Put
func (mr *MockKVStoreWithRangeMockRecorder) Put(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Put", reflect.TypeOf((*MockKVStoreWithRange)(nil).Put), arg0, arg1, arg2)
}

// Get mocks base method
func (m *MockKVStoreWithRange) Get(arg0 string, arg1 []byte) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get
func (mr *MockKVStoreWithRangeMockRecorder) Get(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockKVStoreWithRange)(nil).Get), arg0, arg1)
}

// Delete mocks base method
func (m *MockKVStoreWithRange) Delete(arg0 string, arg1 []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete
func (mr *MockKVStoreWithRangeMockRecorder) Delete(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockKVStoreWithRange)(nil).Delete), arg0, arg1)
}

// WriteBatch mocks base method
func (m *MockKVStoreWithRange) WriteBatch(arg0 batch.KVStoreBatch) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteBatch", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteBatch indicates an expected call of WriteBatch
func (mr *MockKVStoreWithRangeMockRecorder) WriteBatch(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteBatch", reflect.TypeOf((*MockKVStoreWithRange)(nil).WriteBatch), arg0)
}

// Filter mocks base method
func (m *MockKVStoreWithRange) Filter(arg0 string, arg1 Condition, arg2, arg3 []byte) ([][]byte, [][]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Filter", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].([][]byte)
	ret1, _ := ret[1].([][]byte)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Filter indicates an expected call of Filter
func (mr *MockKVStoreWithRangeMockRecorder) Filter(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Filter", reflect.TypeOf((*MockKVStoreWithRange)(nil).Filter), arg0, arg1, arg2, arg3)
}

// Range mocks base method
func (m *MockKVStoreWithRange) Range(arg0 string, arg1 []byte, arg2 uint64) ([][]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Range", arg0, arg1, arg2)
	ret0, _ := ret[0].([][]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Range indicates an expected call of Range
func (mr *MockKVStoreWithRangeMockRecorder) Range(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Range", reflect.TypeOf((*MockKVStoreWithRange)(nil).Range), arg0, arg1, arg2)
}

// MockKVStoreWithBucketFillPercent is a mock of KVStoreWithBucketFillPercent interface
type MockKVStoreWithBucketFillPercent struct {
	ctrl     *gomock.Controller
	recorder *MockKVStoreWithBucketFillPercentMockRecorder
}

// MockKVStoreWithBucketFillPercentMockRecorder is the mock recorder for MockKVStoreWithBucketFillPercent
type MockKVStoreWithBucketFillPercentMockRecorder struct {
	mock *MockKVStoreWithBucketFillPercent
}

// NewMockKVStoreWithBucketFillPercent creates a new mock instance
func NewMockKVStoreWithBucketFillPercent(ctrl *gomock.Controller) *MockKVStoreWithBucketFillPercent {
	mock := &MockKVStoreWithBucketFillPercent{ctrl: ctrl}
	mock.recorder = &MockKVStoreWithBucketFillPercentMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockKVStoreWithBucketFillPercent) EXPECT() *MockKVStoreWithBucketFillPercentMockRecorder {
	return m.recorder
}

// Start mocks base method
func (m *MockKVStoreWithBucketFillPercent) Start(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start
func (mr *MockKVStoreWithBucketFillPercentMockRecorder) Start(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockKVStoreWithBucketFillPercent)(nil).Start), arg0)
}

// Stop mocks base method
func (m *MockKVStoreWithBucketFillPercent) Stop(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop
func (mr *MockKVStoreWithBucketFillPercentMockRecorder) Stop(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockKVStoreWithBucketFillPercent)(nil).Stop), arg0)
}

// Put mocks base method
func (m *MockKVStoreWithBucketFillPercent) Put(arg0 string, arg1, arg2 []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Put", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Put indicates an expected call of Put
func (mr *MockKVStoreWithBucketFillPercentMockRecorder) Put(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Put", reflect.TypeOf((*MockKVStoreWithBucketFillPercent)(nil).Put), arg0, arg1, arg2)
}

// Get mocks base method
func (m *MockKVStoreWithBucketFillPercent) Get(arg0 string, arg1 []byte) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get
func (mr *MockKVStoreWithBucketFillPercentMockRecorder) Get(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockKVStoreWithBucketFillPercent)(nil).Get), arg0, arg1)
}

// Delete mocks base method
func (m *MockKVStoreWithBucketFillPercent) Delete(arg0 string, arg1 []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete
func (mr *MockKVStoreWithBucketFillPercentMockRecorder) Delete(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockKVStoreWithBucketFillPercent)(nil).Delete), arg0, arg1)
}

// WriteBatch mocks base method
func (m *MockKVStoreWithBucketFillPercent) WriteBatch(arg0 batch.KVStoreBatch) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteBatch", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteBatch indicates an expected call of WriteBatch
func (mr *MockKVStoreWithBucketFillPercentMockRecorder) WriteBatch(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteBatch", reflect.TypeOf((*MockKVStoreWithBucketFillPercent)(nil).WriteBatch), arg0)
}

// Filter mocks base method
func (m *MockKVStoreWithBucketFillPercent) Filter(arg0 string, arg1 Condition, arg2, arg3 []byte) ([][]byte, [][]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Filter", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].([][]byte)
	ret1, _ := ret[1].([][]byte)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Filter indicates an expected call of Filter
func (mr *MockKVStoreWithBucketFillPercentMockRecorder) Filter(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Filter", reflect.TypeOf((*MockKVStoreWithBucketFillPercent)(nil).Filter), arg0, arg1, arg2, arg3)
}

// SetBucketFillPercent mocks base method
func (m *MockKVStoreWithBucketFillPercent) SetBucketFillPercent(arg0 string, arg1 float64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetBucketFillPercent", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetBucketFillPercent indicates an expected call of SetBucketFillPercent
func (mr *MockKVStoreWithBucketFillPercentMockRecorder) SetBucketFillPercent(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetBucketFillPercent", reflect.TypeOf((*MockKVStoreWithBucketFillPercent)(nil).SetBucketFillPercent), arg0, arg1)
}

// MockKVStoreForRangeIndex is a mock of KVStoreForRangeIndex interface
type MockKVStoreForRangeIndex struct {
	ctrl     *gomock.Controller
	recorder *MockKVStoreForRangeIndexMockRecorder
}

// MockKVStoreForRangeIndexMockRecorder is the mock recorder for MockKVStoreForRangeIndex
type MockKVStoreForRangeIndexMockRecorder struct {
	mock *MockKVStoreForRangeIndex
}

// NewMockKVStoreForRangeIndex creates a new mock instance
func NewMockKVStoreForRangeIndex(ctrl *gomock.Controller) *MockKVStoreForRangeIndex {
	mock := &MockKVStoreForRangeIndex{ctrl: ctrl}
	mock.recorder = &MockKVStoreForRangeIndexMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockKVStoreForRangeIndex) EXPECT() *MockKVStoreForRangeIndexMockRecorder {
	return m.recorder
}

// Start mocks base method
func (m *MockKVStoreForRangeIndex) Start(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start
func (mr *MockKVStoreForRangeIndexMockRecorder) Start(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockKVStoreForRangeIndex)(nil).Start), arg0)
}

// Stop mocks base method
func (m *MockKVStoreForRangeIndex) Stop(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop
func (mr *MockKVStoreForRangeIndexMockRecorder) Stop(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockKVStoreForRangeIndex)(nil).Stop), arg0)
}

// Put mocks base method
func (m *MockKVStoreForRangeIndex) Put(arg0 string, arg1, arg2 []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Put", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Put indicates an expected call of Put
func (mr *MockKVStoreForRangeIndexMockRecorder) Put(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Put", reflect.TypeOf((*MockKVStoreForRangeIndex)(nil).Put), arg0, arg1, arg2)
}

// Get mocks base method
func (m *MockKVStoreForRangeIndex) Get(arg0 string, arg1 []byte) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get
func (mr *MockKVStoreForRangeIndexMockRecorder) Get(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockKVStoreForRangeIndex)(nil).Get), arg0, arg1)
}

// Delete mocks base method
func (m *MockKVStoreForRangeIndex) Delete(arg0 string, arg1 []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete
func (mr *MockKVStoreForRangeIndexMockRecorder) Delete(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockKVStoreForRangeIndex)(nil).Delete), arg0, arg1)
}

// WriteBatch mocks base method
func (m *MockKVStoreForRangeIndex) WriteBatch(arg0 batch.KVStoreBatch) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteBatch", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteBatch indicates an expected call of WriteBatch
func (mr *MockKVStoreForRangeIndexMockRecorder) WriteBatch(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteBatch", reflect.TypeOf((*MockKVStoreForRangeIndex)(nil).WriteBatch), arg0)
}

// Filter mocks base method
func (m *MockKVStoreForRangeIndex) Filter(arg0 string, arg1 Condition, arg2, arg3 []byte) ([][]byte, [][]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Filter", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].([][]byte)
	ret1, _ := ret[1].([][]byte)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Filter indicates an expected call of Filter
func (mr *MockKVStoreForRangeIndexMockRecorder) Filter(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Filter", reflect.TypeOf((*MockKVStoreForRangeIndex)(nil).Filter), arg0, arg1, arg2, arg3)
}

// Insert mocks base method
func (m *MockKVStoreForRangeIndex) Insert(arg0 []byte, arg1 uint64, arg2 []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Insert", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Insert indicates an expected call of Insert
func (mr *MockKVStoreForRangeIndexMockRecorder) Insert(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Insert", reflect.TypeOf((*MockKVStoreForRangeIndex)(nil).Insert), arg0, arg1, arg2)
}

// Seek mocks base method
func (m *MockKVStoreForRangeIndex) Seek(arg0 []byte, arg1 uint64) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Seek", arg0, arg1)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Seek indicates an expected call of Seek
func (mr *MockKVStoreForRangeIndexMockRecorder) Seek(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Seek", reflect.TypeOf((*MockKVStoreForRangeIndex)(nil).Seek), arg0, arg1)
}

// Remove mocks base method
func (m *MockKVStoreForRangeIndex) Remove(arg0 []byte, arg1 uint64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Remove", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Remove indicates an expected call of Remove
func (mr *MockKVStoreForRangeIndexMockRecorder) Remove(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Remove", reflect.TypeOf((*MockKVStoreForRangeIndex)(nil).Remove), arg0, arg1)
}

// Purge mocks base method
func (m *MockKVStoreForRangeIndex) Purge(arg0 []byte, arg1 uint64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Purge", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Purge indicates an expected call of Purge
func (mr *MockKVStoreForRangeIndexMockRecorder) Purge(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Purge", reflect.TypeOf((*MockKVStoreForRangeIndex)(nil).Purge), arg0, arg1)
}

// GetBucketByPrefix mocks base method
func (m *MockKVStoreForRangeIndex) GetBucketByPrefix(arg0 []byte) ([][]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBucketByPrefix", arg0)
	ret0, _ := ret[0].([][]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBucketByPrefix indicates an expected call of GetBucketByPrefix
func (mr *MockKVStoreForRangeIndexMockRecorder) GetBucketByPrefix(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBucketByPrefix", reflect.TypeOf((*MockKVStoreForRangeIndex)(nil).GetBucketByPrefix), arg0)
}

// GetKeyByPrefix mocks base method
func (m *MockKVStoreForRangeIndex) GetKeyByPrefix(namespace, prefix []byte) ([][]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetKeyByPrefix", namespace, prefix)
	ret0, _ := ret[0].([][]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetKeyByPrefix indicates an expected call of GetKeyByPrefix
func (mr *MockKVStoreForRangeIndexMockRecorder) GetKeyByPrefix(namespace, prefix interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetKeyByPrefix", reflect.TypeOf((*MockKVStoreForRangeIndex)(nil).GetKeyByPrefix), namespace, prefix)
}
