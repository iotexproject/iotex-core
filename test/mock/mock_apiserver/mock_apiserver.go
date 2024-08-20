// Code generated by MockGen. DO NOT EDIT.
// Source: ./api/apitestserver.go

// Package mock_apiserver is a generated GoMock package.
package mock_apiserver

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	iotexapi "github.com/iotexproject/iotex-proto/golang/iotexapi"
	metadata "google.golang.org/grpc/metadata"
)

// MockStreamBlocksServer is a mock of StreamBlocksServer interface.
type MockStreamBlocksServer struct {
	ctrl     *gomock.Controller
	recorder *MockStreamBlocksServerMockRecorder
}

// MockStreamBlocksServerMockRecorder is the mock recorder for MockStreamBlocksServer.
type MockStreamBlocksServerMockRecorder struct {
	mock *MockStreamBlocksServer
}

// NewMockStreamBlocksServer creates a new mock instance.
func NewMockStreamBlocksServer(ctrl *gomock.Controller) *MockStreamBlocksServer {
	mock := &MockStreamBlocksServer{ctrl: ctrl}
	mock.recorder = &MockStreamBlocksServerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStreamBlocksServer) EXPECT() *MockStreamBlocksServerMockRecorder {
	return m.recorder
}

// Context mocks base method.
func (m *MockStreamBlocksServer) Context() context.Context {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Context")
	ret0, _ := ret[0].(context.Context)
	return ret0
}

// Context indicates an expected call of Context.
func (mr *MockStreamBlocksServerMockRecorder) Context() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Context", reflect.TypeOf((*MockStreamBlocksServer)(nil).Context))
}

// RecvMsg mocks base method.
func (m_2 *MockStreamBlocksServer) RecvMsg(m any) error {
	m_2.ctrl.T.Helper()
	ret := m_2.ctrl.Call(m_2, "RecvMsg", m)
	ret0, _ := ret[0].(error)
	return ret0
}

// RecvMsg indicates an expected call of RecvMsg.
func (mr *MockStreamBlocksServerMockRecorder) RecvMsg(m interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecvMsg", reflect.TypeOf((*MockStreamBlocksServer)(nil).RecvMsg), m)
}

// Send mocks base method.
func (m *MockStreamBlocksServer) Send(arg0 *iotexapi.StreamBlocksResponse) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Send", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Send indicates an expected call of Send.
func (mr *MockStreamBlocksServerMockRecorder) Send(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Send", reflect.TypeOf((*MockStreamBlocksServer)(nil).Send), arg0)
}

// SendHeader mocks base method.
func (m *MockStreamBlocksServer) SendHeader(arg0 metadata.MD) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendHeader", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendHeader indicates an expected call of SendHeader.
func (mr *MockStreamBlocksServerMockRecorder) SendHeader(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendHeader", reflect.TypeOf((*MockStreamBlocksServer)(nil).SendHeader), arg0)
}

// SendMsg mocks base method.
func (m_2 *MockStreamBlocksServer) SendMsg(m any) error {
	m_2.ctrl.T.Helper()
	ret := m_2.ctrl.Call(m_2, "SendMsg", m)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendMsg indicates an expected call of SendMsg.
func (mr *MockStreamBlocksServerMockRecorder) SendMsg(m interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendMsg", reflect.TypeOf((*MockStreamBlocksServer)(nil).SendMsg), m)
}

// SetHeader mocks base method.
func (m *MockStreamBlocksServer) SetHeader(arg0 metadata.MD) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetHeader", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetHeader indicates an expected call of SetHeader.
func (mr *MockStreamBlocksServerMockRecorder) SetHeader(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetHeader", reflect.TypeOf((*MockStreamBlocksServer)(nil).SetHeader), arg0)
}

// SetTrailer mocks base method.
func (m *MockStreamBlocksServer) SetTrailer(arg0 metadata.MD) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetTrailer", arg0)
}

// SetTrailer indicates an expected call of SetTrailer.
func (mr *MockStreamBlocksServerMockRecorder) SetTrailer(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetTrailer", reflect.TypeOf((*MockStreamBlocksServer)(nil).SetTrailer), arg0)
}
