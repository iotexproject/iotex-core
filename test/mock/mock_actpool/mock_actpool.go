// Code generated by MockGen. DO NOT EDIT.
// Source: ./actpool/actpool.go

// Package mock_actpool is a generated GoMock package.
package mock_actpool

import (
	gomock "github.com/golang/mock/gomock"
	action "github.com/iotexproject/iotex-core/action"
	protocol "github.com/iotexproject/iotex-core/action/protocol"
	hash "github.com/iotexproject/iotex-core/pkg/hash"
	reflect "reflect"
)

// MockActPool is a mock of ActPool interface
type MockActPool struct {
	ctrl     *gomock.Controller
	recorder *MockActPoolMockRecorder
}

// MockActPoolMockRecorder is the mock recorder for MockActPool
type MockActPoolMockRecorder struct {
	mock *MockActPool
}

// NewMockActPool creates a new mock instance
func NewMockActPool(ctrl *gomock.Controller) *MockActPool {
	mock := &MockActPool{ctrl: ctrl}
	mock.recorder = &MockActPoolMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockActPool) EXPECT() *MockActPoolMockRecorder {
	return m.recorder
}

// Reset mocks base method
func (m *MockActPool) Reset() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Reset")
}

// Reset indicates an expected call of Reset
func (mr *MockActPoolMockRecorder) Reset() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Reset", reflect.TypeOf((*MockActPool)(nil).Reset))
}

// PickActs mocks base method
func (m *MockActPool) PickActs() []action.SealedEnvelope {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PickActs")
	ret0, _ := ret[0].([]action.SealedEnvelope)
	return ret0
}

// PickActs indicates an expected call of PickActs
func (mr *MockActPoolMockRecorder) PickActs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PickActs", reflect.TypeOf((*MockActPool)(nil).PickActs))
}

// PendingActionMap mocks base method
func (m *MockActPool) PendingActionMap() map[string][]action.SealedEnvelope {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PendingActionMap")
	ret0, _ := ret[0].(map[string][]action.SealedEnvelope)
	return ret0
}

// PendingActionMap indicates an expected call of PendingActionMap
func (mr *MockActPoolMockRecorder) PendingActionMap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PendingActionMap", reflect.TypeOf((*MockActPool)(nil).PendingActionMap))
}

// Add mocks base method
func (m *MockActPool) Add(act action.SealedEnvelope) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Add", act)
	ret0, _ := ret[0].(error)
	return ret0
}

// Add indicates an expected call of Add
func (mr *MockActPoolMockRecorder) Add(act interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Add", reflect.TypeOf((*MockActPool)(nil).Add), act)
}

// GetPendingNonce mocks base method
func (m *MockActPool) GetPendingNonce(addr string) (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPendingNonce", addr)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPendingNonce indicates an expected call of GetPendingNonce
func (mr *MockActPoolMockRecorder) GetPendingNonce(addr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPendingNonce", reflect.TypeOf((*MockActPool)(nil).GetPendingNonce), addr)
}

// GetUnconfirmedActs mocks base method
func (m *MockActPool) GetUnconfirmedActs(addr string) []action.SealedEnvelope {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUnconfirmedActs", addr)
	ret0, _ := ret[0].([]action.SealedEnvelope)
	return ret0
}

// GetUnconfirmedActs indicates an expected call of GetUnconfirmedActs
func (mr *MockActPoolMockRecorder) GetUnconfirmedActs(addr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUnconfirmedActs", reflect.TypeOf((*MockActPool)(nil).GetUnconfirmedActs), addr)
}

// GetActionByHash mocks base method
func (m *MockActPool) GetActionByHash(hash hash.Hash256) (action.SealedEnvelope, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetActionByHash", hash)
	ret0, _ := ret[0].(action.SealedEnvelope)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetActionByHash indicates an expected call of GetActionByHash
func (mr *MockActPoolMockRecorder) GetActionByHash(hash interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetActionByHash", reflect.TypeOf((*MockActPool)(nil).GetActionByHash), hash)
}

// GetSize mocks base method
func (m *MockActPool) GetSize() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSize")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// GetSize indicates an expected call of GetSize
func (mr *MockActPoolMockRecorder) GetSize() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSize", reflect.TypeOf((*MockActPool)(nil).GetSize))
}

// GetCapacity mocks base method
func (m *MockActPool) GetCapacity() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCapacity")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// GetCapacity indicates an expected call of GetCapacity
func (mr *MockActPoolMockRecorder) GetCapacity() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCapacity", reflect.TypeOf((*MockActPool)(nil).GetCapacity))
}

// AddActionValidators mocks base method
func (m *MockActPool) AddActionValidators(arg0 ...protocol.ActionValidator) {
	m.ctrl.T.Helper()
	varargs := []interface{}{}
	for _, a := range arg0 {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "AddActionValidators", varargs...)
}

// AddActionValidators indicates an expected call of AddActionValidators
func (mr *MockActPoolMockRecorder) AddActionValidators(arg0 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddActionValidators", reflect.TypeOf((*MockActPool)(nil).AddActionValidators), arg0...)
}

// AddActionEnvelopeValidators mocks base method
func (m *MockActPool) AddActionEnvelopeValidators(arg0 ...protocol.ActionEnvelopeValidator) {
	m.ctrl.T.Helper()
	varargs := []interface{}{}
	for _, a := range arg0 {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "AddActionEnvelopeValidators", varargs...)
}

// AddActionEnvelopeValidators indicates an expected call of AddActionEnvelopeValidators
func (mr *MockActPoolMockRecorder) AddActionEnvelopeValidators(arg0 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddActionEnvelopeValidators", reflect.TypeOf((*MockActPool)(nil).AddActionEnvelopeValidators), arg0...)
}
