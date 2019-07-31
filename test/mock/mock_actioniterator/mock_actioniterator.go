// Code generated by MockGen. DO NOT EDIT.
// Source: ./actpool/actioniterator/actioniterator.go

// Package mock_actioniterator is a generated GoMock package.
package mock_actioniterator

import (
	gomock "github.com/golang/mock/gomock"
	action "github.com/iotexproject/iotex-core/action"
	reflect "reflect"
)

// MockActionIterator is a mock of ActionIterator interface
type MockActionIterator struct {
	ctrl     *gomock.Controller
	recorder *MockActionIteratorMockRecorder
}

// MockActionIteratorMockRecorder is the mock recorder for MockActionIterator
type MockActionIteratorMockRecorder struct {
	mock *MockActionIterator
}

// NewMockActionIterator creates a new mock instance
func NewMockActionIterator(ctrl *gomock.Controller) *MockActionIterator {
	mock := &MockActionIterator{ctrl: ctrl}
	mock.recorder = &MockActionIteratorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockActionIterator) EXPECT() *MockActionIteratorMockRecorder {
	return m.recorder
}

// Next mocks base method
func (m *MockActionIterator) Next() (action.SealedEnvelope, bool) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Next")
	ret0, _ := ret[0].(action.SealedEnvelope)
	ret1, _ := ret[1].(bool)
	return ret0, ret1
}

// Next indicates an expected call of Next
func (mr *MockActionIteratorMockRecorder) Next() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Next", reflect.TypeOf((*MockActionIterator)(nil).Next))
}

// PopAccount mocks base method
func (m *MockActionIterator) PopAccount() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "PopAccount")
}

// PopAccount indicates an expected call of PopAccount
func (mr *MockActionIteratorMockRecorder) PopAccount() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PopAccount", reflect.TypeOf((*MockActionIterator)(nil).PopAccount))
}
