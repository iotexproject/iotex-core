// Code generated by MockGen. DO NOT EDIT.
// Source: ./consensus/consensusfsm/endorsement.go

// Package consensusfsm is a generated GoMock package.
package consensusfsm

import (
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
)

// MockEndorsement is a mock of Endorsement interface
type MockEndorsement struct {
	ctrl     *gomock.Controller
	recorder *MockEndorsementMockRecorder
}

// MockEndorsementMockRecorder is the mock recorder for MockEndorsement
type MockEndorsementMockRecorder struct {
	mock *MockEndorsement
}

// NewMockEndorsement creates a new mock instance
func NewMockEndorsement(ctrl *gomock.Controller) *MockEndorsement {
	mock := &MockEndorsement{ctrl: ctrl}
	mock.recorder = &MockEndorsementMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockEndorsement) EXPECT() *MockEndorsementMockRecorder {
	return m.recorder
}

// Hash mocks base method
func (m *MockEndorsement) Hash() []byte {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Hash")
	ret0, _ := ret[0].([]byte)
	return ret0
}

// Hash indicates an expected call of Hash
func (mr *MockEndorsementMockRecorder) Hash() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Hash", reflect.TypeOf((*MockEndorsement)(nil).Hash))
}

// Height mocks base method
func (m *MockEndorsement) Height() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Height")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// Height indicates an expected call of Height
func (mr *MockEndorsementMockRecorder) Height() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Height", reflect.TypeOf((*MockEndorsement)(nil).Height))
}

// Round mocks base method
func (m *MockEndorsement) Round() uint32 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Round")
	ret0, _ := ret[0].(uint32)
	return ret0
}

// Round indicates an expected call of Round
func (mr *MockEndorsementMockRecorder) Round() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Round", reflect.TypeOf((*MockEndorsement)(nil).Round))
}

// Endorser mocks base method
func (m *MockEndorsement) Endorser() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Endorser")
	ret0, _ := ret[0].(string)
	return ret0
}

// Endorser indicates an expected call of Endorser
func (mr *MockEndorsementMockRecorder) Endorser() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Endorser", reflect.TypeOf((*MockEndorsement)(nil).Endorser))
}

// Serialize mocks base method
func (m *MockEndorsement) Serialize() ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Serialize")
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Serialize indicates an expected call of Serialize
func (mr *MockEndorsementMockRecorder) Serialize() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Serialize", reflect.TypeOf((*MockEndorsement)(nil).Serialize))
}

// Deserialize mocks base method
func (m *MockEndorsement) Deserialize(arg0 []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Deserialize", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Deserialize indicates an expected call of Deserialize
func (mr *MockEndorsementMockRecorder) Deserialize(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Deserialize", reflect.TypeOf((*MockEndorsement)(nil).Deserialize), arg0)
}
