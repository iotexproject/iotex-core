// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/iotexproject/iotex-core/v2/blockchain/blockdao (interfaces: BlockIndexer)

// Package mock_blockdao is a generated GoMock package.
package mock_blockdao

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	block "github.com/iotexproject/iotex-core/v2/blockchain/block"
)

// MockBlockIndexer is a mock of BlockIndexer interface.
type MockBlockIndexer struct {
	ctrl     *gomock.Controller
	recorder *MockBlockIndexerMockRecorder
}

// MockBlockIndexerMockRecorder is the mock recorder for MockBlockIndexer.
type MockBlockIndexerMockRecorder struct {
	mock *MockBlockIndexer
}

// NewMockBlockIndexer creates a new mock instance.
func NewMockBlockIndexer(ctrl *gomock.Controller) *MockBlockIndexer {
	mock := &MockBlockIndexer{ctrl: ctrl}
	mock.recorder = &MockBlockIndexerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockBlockIndexer) EXPECT() *MockBlockIndexerMockRecorder {
	return m.recorder
}

// DeleteTipBlock mocks base method.
func (m *MockBlockIndexer) DeleteTipBlock(arg0 context.Context, arg1 *block.Block) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteTipBlock", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteTipBlock indicates an expected call of DeleteTipBlock.
func (mr *MockBlockIndexerMockRecorder) DeleteTipBlock(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteTipBlock", reflect.TypeOf((*MockBlockIndexer)(nil).DeleteTipBlock), arg0, arg1)
}

// Height mocks base method.
func (m *MockBlockIndexer) Height() (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Height")
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Height indicates an expected call of Height.
func (mr *MockBlockIndexerMockRecorder) Height() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Height", reflect.TypeOf((*MockBlockIndexer)(nil).Height))
}

// PutBlock mocks base method.
func (m *MockBlockIndexer) PutBlock(arg0 context.Context, arg1 *block.Block) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PutBlock", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// PutBlock indicates an expected call of PutBlock.
func (mr *MockBlockIndexerMockRecorder) PutBlock(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PutBlock", reflect.TypeOf((*MockBlockIndexer)(nil).PutBlock), arg0, arg1)
}

// Start mocks base method.
func (m *MockBlockIndexer) Start(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start.
func (mr *MockBlockIndexerMockRecorder) Start(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockBlockIndexer)(nil).Start), arg0)
}

// Stop mocks base method.
func (m *MockBlockIndexer) Stop(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop.
func (mr *MockBlockIndexerMockRecorder) Stop(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockBlockIndexer)(nil).Stop), arg0)
}
