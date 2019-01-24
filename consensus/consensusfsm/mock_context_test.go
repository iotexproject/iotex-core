// Code generated by MockGen. DO NOT EDIT.
// Source: ./consensus/fsm/context.go

// Package consensusfsm is a generated GoMock package.
package consensusfsm

import (
	gomock "github.com/golang/mock/gomock"
	go_fsm "github.com/iotexproject/go-fsm"
	zap "go.uber.org/zap"
	reflect "reflect"
	time "time"
)

// MockContext is a mock of Context interface
type MockContext struct {
	ctrl     *gomock.Controller
	recorder *MockContextMockRecorder
}

// MockContextMockRecorder is the mock recorder for MockContext
type MockContextMockRecorder struct {
	mock *MockContext
}

// NewMockContext creates a new mock instance
func NewMockContext(ctrl *gomock.Controller) *MockContext {
	mock := &MockContext{ctrl: ctrl}
	mock.recorder = &MockContextMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockContext) EXPECT() *MockContextMockRecorder {
	return m.recorder
}

// IsStaleEvent mocks base method
func (m *MockContext) IsStaleEvent(arg0 *ConsensusEvent) bool {
	ret := m.ctrl.Call(m, "IsStaleEvent", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsStaleEvent indicates an expected call of IsStaleEvent
func (mr *MockContextMockRecorder) IsStaleEvent(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsStaleEvent", reflect.TypeOf((*MockContext)(nil).IsStaleEvent), arg0)
}

// IsFutureEvent mocks base method
func (m *MockContext) IsFutureEvent(arg0 *ConsensusEvent) bool {
	ret := m.ctrl.Call(m, "IsFutureEvent", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsFutureEvent indicates an expected call of IsFutureEvent
func (mr *MockContextMockRecorder) IsFutureEvent(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsFutureEvent", reflect.TypeOf((*MockContext)(nil).IsFutureEvent), arg0)
}

// IsStaleUnmatchedEvent mocks base method
func (m *MockContext) IsStaleUnmatchedEvent(arg0 *ConsensusEvent) bool {
	ret := m.ctrl.Call(m, "IsStaleUnmatchedEvent", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsStaleUnmatchedEvent indicates an expected call of IsStaleUnmatchedEvent
func (mr *MockContextMockRecorder) IsStaleUnmatchedEvent(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsStaleUnmatchedEvent", reflect.TypeOf((*MockContext)(nil).IsStaleUnmatchedEvent), arg0)
}

// Logger mocks base method
func (m *MockContext) Logger() *zap.Logger {
	ret := m.ctrl.Call(m, "Logger")
	ret0, _ := ret[0].(*zap.Logger)
	return ret0
}

// Logger indicates an expected call of Logger
func (mr *MockContextMockRecorder) Logger() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Logger", reflect.TypeOf((*MockContext)(nil).Logger))
}

// LoggerWithStats mocks base method
func (m *MockContext) LoggerWithStats() *zap.Logger {
	ret := m.ctrl.Call(m, "LoggerWithStats")
	ret0, _ := ret[0].(*zap.Logger)
	return ret0
}

// LoggerWithStats indicates an expected call of LoggerWithStats
func (mr *MockContextMockRecorder) LoggerWithStats() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LoggerWithStats", reflect.TypeOf((*MockContext)(nil).LoggerWithStats))
}

// NewConsensusEvent mocks base method
func (m *MockContext) NewConsensusEvent(arg0 go_fsm.EventType, arg1 interface{}) *ConsensusEvent {
	ret := m.ctrl.Call(m, "NewConsensusEvent", arg0, arg1)
	ret0, _ := ret[0].(*ConsensusEvent)
	return ret0
}

// NewConsensusEvent indicates an expected call of NewConsensusEvent
func (mr *MockContextMockRecorder) NewConsensusEvent(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewConsensusEvent", reflect.TypeOf((*MockContext)(nil).NewConsensusEvent), arg0, arg1)
}

// NewBackdoorEvt mocks base method
func (m *MockContext) NewBackdoorEvt(arg0 go_fsm.State) *ConsensusEvent {
	ret := m.ctrl.Call(m, "NewBackdoorEvt", arg0)
	ret0, _ := ret[0].(*ConsensusEvent)
	return ret0
}

// NewBackdoorEvt indicates an expected call of NewBackdoorEvt
func (mr *MockContextMockRecorder) NewBackdoorEvt(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewBackdoorEvt", reflect.TypeOf((*MockContext)(nil).NewBackdoorEvt), arg0)
}

// IsDelegate mocks base method
func (m *MockContext) IsDelegate() bool {
	ret := m.ctrl.Call(m, "IsDelegate")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsDelegate indicates an expected call of IsDelegate
func (mr *MockContextMockRecorder) IsDelegate() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsDelegate", reflect.TypeOf((*MockContext)(nil).IsDelegate))
}

// IsProposer mocks base method
func (m *MockContext) IsProposer() bool {
	ret := m.ctrl.Call(m, "IsProposer")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsProposer indicates an expected call of IsProposer
func (mr *MockContextMockRecorder) IsProposer() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsProposer", reflect.TypeOf((*MockContext)(nil).IsProposer))
}

// BroadcastBlockProposal mocks base method
func (m *MockContext) BroadcastBlockProposal(arg0 Endorsement) {
	m.ctrl.Call(m, "BroadcastBlockProposal", arg0)
}

// BroadcastBlockProposal indicates an expected call of BroadcastBlockProposal
func (mr *MockContextMockRecorder) BroadcastBlockProposal(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BroadcastBlockProposal", reflect.TypeOf((*MockContext)(nil).BroadcastBlockProposal), arg0)
}

// BroadcastEndorsement mocks base method
func (m *MockContext) BroadcastEndorsement(arg0 Endorsement) {
	m.ctrl.Call(m, "BroadcastEndorsement", arg0)
}

// BroadcastEndorsement indicates an expected call of BroadcastEndorsement
func (mr *MockContextMockRecorder) BroadcastEndorsement(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BroadcastEndorsement", reflect.TypeOf((*MockContext)(nil).BroadcastEndorsement), arg0)
}

// Prepare mocks base method
func (m *MockContext) Prepare() (time.Duration, error) {
	ret := m.ctrl.Call(m, "Prepare")
	ret0, _ := ret[0].(time.Duration)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Prepare indicates an expected call of Prepare
func (mr *MockContextMockRecorder) Prepare() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Prepare", reflect.TypeOf((*MockContext)(nil).Prepare))
}

// MintBlock mocks base method
func (m *MockContext) MintBlock() (Endorsement, error) {
	ret := m.ctrl.Call(m, "MintBlock")
	ret0, _ := ret[0].(Endorsement)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MintBlock indicates an expected call of MintBlock
func (mr *MockContextMockRecorder) MintBlock() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MintBlock", reflect.TypeOf((*MockContext)(nil).MintBlock))
}

// NewProposalEndorsement mocks base method
func (m *MockContext) NewProposalEndorsement(arg0 Endorsement) (Endorsement, error) {
	ret := m.ctrl.Call(m, "NewProposalEndorsement", arg0)
	ret0, _ := ret[0].(Endorsement)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewProposalEndorsement indicates an expected call of NewProposalEndorsement
func (mr *MockContextMockRecorder) NewProposalEndorsement(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewProposalEndorsement", reflect.TypeOf((*MockContext)(nil).NewProposalEndorsement), arg0)
}

// NewLockEndorsement mocks base method
func (m *MockContext) NewLockEndorsement() (Endorsement, error) {
	ret := m.ctrl.Call(m, "NewLockEndorsement")
	ret0, _ := ret[0].(Endorsement)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewLockEndorsement indicates an expected call of NewLockEndorsement
func (mr *MockContextMockRecorder) NewLockEndorsement() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewLockEndorsement", reflect.TypeOf((*MockContext)(nil).NewLockEndorsement))
}

// NewPreCommitEndorsement mocks base method
func (m *MockContext) NewPreCommitEndorsement() (Endorsement, error) {
	ret := m.ctrl.Call(m, "NewPreCommitEndorsement")
	ret0, _ := ret[0].(Endorsement)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewPreCommitEndorsement indicates an expected call of NewPreCommitEndorsement
func (mr *MockContextMockRecorder) NewPreCommitEndorsement() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewPreCommitEndorsement", reflect.TypeOf((*MockContext)(nil).NewPreCommitEndorsement))
}

// OnConsensusReached mocks base method
func (m *MockContext) OnConsensusReached() {
	m.ctrl.Call(m, "OnConsensusReached")
}

// OnConsensusReached indicates an expected call of OnConsensusReached
func (mr *MockContextMockRecorder) OnConsensusReached() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OnConsensusReached", reflect.TypeOf((*MockContext)(nil).OnConsensusReached))
}

// AddProposalEndorsement mocks base method
func (m *MockContext) AddProposalEndorsement(arg0 Endorsement) error {
	ret := m.ctrl.Call(m, "AddProposalEndorsement", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddProposalEndorsement indicates an expected call of AddProposalEndorsement
func (mr *MockContextMockRecorder) AddProposalEndorsement(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddProposalEndorsement", reflect.TypeOf((*MockContext)(nil).AddProposalEndorsement), arg0)
}

// AddLockEndorsement mocks base method
func (m *MockContext) AddLockEndorsement(arg0 Endorsement) error {
	ret := m.ctrl.Call(m, "AddLockEndorsement", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddLockEndorsement indicates an expected call of AddLockEndorsement
func (mr *MockContextMockRecorder) AddLockEndorsement(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddLockEndorsement", reflect.TypeOf((*MockContext)(nil).AddLockEndorsement), arg0)
}

// AddPreCommitEndorsement mocks base method
func (m *MockContext) AddPreCommitEndorsement(arg0 Endorsement) error {
	ret := m.ctrl.Call(m, "AddPreCommitEndorsement", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddPreCommitEndorsement indicates an expected call of AddPreCommitEndorsement
func (mr *MockContextMockRecorder) AddPreCommitEndorsement(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddPreCommitEndorsement", reflect.TypeOf((*MockContext)(nil).AddPreCommitEndorsement), arg0)
}

// HasReceivedBlock mocks base method
func (m *MockContext) HasReceivedBlock() bool {
	ret := m.ctrl.Call(m, "HasReceivedBlock")
	ret0, _ := ret[0].(bool)
	return ret0
}

// HasReceivedBlock indicates an expected call of HasReceivedBlock
func (mr *MockContextMockRecorder) HasReceivedBlock() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasReceivedBlock", reflect.TypeOf((*MockContext)(nil).HasReceivedBlock))
}

// IsLocked mocks base method
func (m *MockContext) IsLocked() bool {
	ret := m.ctrl.Call(m, "IsLocked")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsLocked indicates an expected call of IsLocked
func (mr *MockContextMockRecorder) IsLocked() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsLocked", reflect.TypeOf((*MockContext)(nil).IsLocked))
}

// ReadyToPreCommit mocks base method
func (m *MockContext) ReadyToPreCommit() bool {
	ret := m.ctrl.Call(m, "ReadyToPreCommit")
	ret0, _ := ret[0].(bool)
	return ret0
}

// ReadyToPreCommit indicates an expected call of ReadyToPreCommit
func (mr *MockContextMockRecorder) ReadyToPreCommit() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadyToPreCommit", reflect.TypeOf((*MockContext)(nil).ReadyToPreCommit))
}

// ReadyToCommit mocks base method
func (m *MockContext) ReadyToCommit() bool {
	ret := m.ctrl.Call(m, "ReadyToCommit")
	ret0, _ := ret[0].(bool)
	return ret0
}

// ReadyToCommit indicates an expected call of ReadyToCommit
func (mr *MockContextMockRecorder) ReadyToCommit() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadyToCommit", reflect.TypeOf((*MockContext)(nil).ReadyToCommit))
}
