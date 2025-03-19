// Code generated by MockGen. DO NOT EDIT.
// Source: ./consensus/consensusfsm/context.go

// Package consensusfsm is a generated GoMock package.
package consensusfsm

import (
	context "context"
	reflect "reflect"
	time "time"

	gomock "github.com/golang/mock/gomock"
	go_fsm "github.com/iotexproject/go-fsm"
	zap "go.uber.org/zap"
)

// MockContext is a mock of Context interface.
type MockContext struct {
	ctrl     *gomock.Controller
	recorder *MockContextMockRecorder
}

// MockContextMockRecorder is the mock recorder for MockContext.
type MockContextMockRecorder struct {
	mock *MockContext
}

// NewMockContext creates a new mock instance.
func NewMockContext(ctrl *gomock.Controller) *MockContext {
	mock := &MockContext{ctrl: ctrl}
	mock.recorder = &MockContextMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockContext) EXPECT() *MockContextMockRecorder {
	return m.recorder
}

// AcceptBlockTTL mocks base method.
func (m *MockContext) AcceptBlockTTL(arg0 uint64) time.Duration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AcceptBlockTTL", arg0)
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

// AcceptBlockTTL indicates an expected call of AcceptBlockTTL.
func (mr *MockContextMockRecorder) AcceptBlockTTL(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AcceptBlockTTL", reflect.TypeOf((*MockContext)(nil).AcceptBlockTTL), arg0)
}

// AcceptLockEndorsementTTL mocks base method.
func (m *MockContext) AcceptLockEndorsementTTL(arg0 uint64) time.Duration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AcceptLockEndorsementTTL", arg0)
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

// AcceptLockEndorsementTTL indicates an expected call of AcceptLockEndorsementTTL.
func (mr *MockContextMockRecorder) AcceptLockEndorsementTTL(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AcceptLockEndorsementTTL", reflect.TypeOf((*MockContext)(nil).AcceptLockEndorsementTTL), arg0)
}

// AcceptProposalEndorsementTTL mocks base method.
func (m *MockContext) AcceptProposalEndorsementTTL(arg0 uint64) time.Duration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AcceptProposalEndorsementTTL", arg0)
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

// AcceptProposalEndorsementTTL indicates an expected call of AcceptProposalEndorsementTTL.
func (mr *MockContextMockRecorder) AcceptProposalEndorsementTTL(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AcceptProposalEndorsementTTL", reflect.TypeOf((*MockContext)(nil).AcceptProposalEndorsementTTL), arg0)
}

// Activate mocks base method.
func (m *MockContext) Activate(arg0 bool) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Activate", arg0)
}

// Activate indicates an expected call of Activate.
func (mr *MockContextMockRecorder) Activate(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Activate", reflect.TypeOf((*MockContext)(nil).Activate), arg0)
}

// Active mocks base method.
func (m *MockContext) Active() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Active")
	ret0, _ := ret[0].(bool)
	return ret0
}

// Active indicates an expected call of Active.
func (mr *MockContextMockRecorder) Active() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Active", reflect.TypeOf((*MockContext)(nil).Active))
}

// BlockInterval mocks base method.
func (m *MockContext) BlockInterval(arg0 uint64) time.Duration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlockInterval", arg0)
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

// BlockInterval indicates an expected call of BlockInterval.
func (mr *MockContextMockRecorder) BlockInterval(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlockInterval", reflect.TypeOf((*MockContext)(nil).BlockInterval), arg0)
}

// Broadcast mocks base method.
func (m *MockContext) Broadcast(arg0 interface{}) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Broadcast", arg0)
}

// Broadcast indicates an expected call of Broadcast.
func (mr *MockContextMockRecorder) Broadcast(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Broadcast", reflect.TypeOf((*MockContext)(nil).Broadcast), arg0)
}

// Commit mocks base method.
func (m *MockContext) Commit(arg0 interface{}) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Commit", arg0)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Commit indicates an expected call of Commit.
func (mr *MockContextMockRecorder) Commit(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Commit", reflect.TypeOf((*MockContext)(nil).Commit), arg0)
}

// CommitTTL mocks base method.
func (m *MockContext) CommitTTL(arg0 uint64) time.Duration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CommitTTL", arg0)
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

// CommitTTL indicates an expected call of CommitTTL.
func (mr *MockContextMockRecorder) CommitTTL(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommitTTL", reflect.TypeOf((*MockContext)(nil).CommitTTL), arg0)
}

// Delay mocks base method.
func (m *MockContext) Delay(arg0 uint64) time.Duration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delay", arg0)
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

// Delay indicates an expected call of Delay.
func (mr *MockContextMockRecorder) Delay(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delay", reflect.TypeOf((*MockContext)(nil).Delay), arg0)
}

// EventChanSize mocks base method.
func (m *MockContext) EventChanSize() uint {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EventChanSize")
	ret0, _ := ret[0].(uint)
	return ret0
}

// EventChanSize indicates an expected call of EventChanSize.
func (mr *MockContextMockRecorder) EventChanSize() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EventChanSize", reflect.TypeOf((*MockContext)(nil).EventChanSize))
}

// HasDelegate mocks base method.
func (m *MockContext) HasDelegate() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasDelegate")
	ret0, _ := ret[0].(bool)
	return ret0
}

// HasDelegate indicates an expected call of HasDelegate.
func (mr *MockContextMockRecorder) HasDelegate() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasDelegate", reflect.TypeOf((*MockContext)(nil).HasDelegate))
}

// Height mocks base method.
func (m *MockContext) Height() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Height")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// Height indicates an expected call of Height.
func (mr *MockContextMockRecorder) Height() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Height", reflect.TypeOf((*MockContext)(nil).Height))
}

// IsFutureEvent mocks base method.
func (m *MockContext) IsFutureEvent(arg0 *ConsensusEvent) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsFutureEvent", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsFutureEvent indicates an expected call of IsFutureEvent.
func (mr *MockContextMockRecorder) IsFutureEvent(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsFutureEvent", reflect.TypeOf((*MockContext)(nil).IsFutureEvent), arg0)
}

// IsStaleEvent mocks base method.
func (m *MockContext) IsStaleEvent(arg0 *ConsensusEvent) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsStaleEvent", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsStaleEvent indicates an expected call of IsStaleEvent.
func (mr *MockContextMockRecorder) IsStaleEvent(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsStaleEvent", reflect.TypeOf((*MockContext)(nil).IsStaleEvent), arg0)
}

// IsStaleUnmatchedEvent mocks base method.
func (m *MockContext) IsStaleUnmatchedEvent(arg0 *ConsensusEvent) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsStaleUnmatchedEvent", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsStaleUnmatchedEvent indicates an expected call of IsStaleUnmatchedEvent.
func (mr *MockContextMockRecorder) IsStaleUnmatchedEvent(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsStaleUnmatchedEvent", reflect.TypeOf((*MockContext)(nil).IsStaleUnmatchedEvent), arg0)
}

// Logger mocks base method.
func (m *MockContext) Logger() *zap.Logger {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Logger")
	ret0, _ := ret[0].(*zap.Logger)
	return ret0
}

// Logger indicates an expected call of Logger.
func (mr *MockContextMockRecorder) Logger() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Logger", reflect.TypeOf((*MockContext)(nil).Logger))
}

// NewBackdoorEvt mocks base method.
func (m *MockContext) NewBackdoorEvt(arg0 go_fsm.State) []*ConsensusEvent {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewBackdoorEvt", arg0)
	ret0, _ := ret[0].([]*ConsensusEvent)
	return ret0
}

// NewBackdoorEvt indicates an expected call of NewBackdoorEvt.
func (mr *MockContextMockRecorder) NewBackdoorEvt(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewBackdoorEvt", reflect.TypeOf((*MockContext)(nil).NewBackdoorEvt), arg0)
}

// NewConsensusEvent mocks base method.
func (m *MockContext) NewConsensusEvent(arg0 go_fsm.EventType, arg1 interface{}) []*ConsensusEvent {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewConsensusEvent", arg0, arg1)
	ret0, _ := ret[0].([]*ConsensusEvent)
	return ret0
}

// NewConsensusEvent indicates an expected call of NewConsensusEvent.
func (mr *MockContextMockRecorder) NewConsensusEvent(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewConsensusEvent", reflect.TypeOf((*MockContext)(nil).NewConsensusEvent), arg0, arg1)
}

// NewLockEndorsement mocks base method.
func (m *MockContext) NewLockEndorsement(arg0 interface{}) (interface{}, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewLockEndorsement", arg0)
	ret0, _ := ret[0].(interface{})
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewLockEndorsement indicates an expected call of NewLockEndorsement.
func (mr *MockContextMockRecorder) NewLockEndorsement(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewLockEndorsement", reflect.TypeOf((*MockContext)(nil).NewLockEndorsement), arg0)
}

// NewPreCommitEndorsement mocks base method.
func (m *MockContext) NewPreCommitEndorsement(arg0 interface{}) (interface{}, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewPreCommitEndorsement", arg0)
	ret0, _ := ret[0].(interface{})
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewPreCommitEndorsement indicates an expected call of NewPreCommitEndorsement.
func (mr *MockContextMockRecorder) NewPreCommitEndorsement(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewPreCommitEndorsement", reflect.TypeOf((*MockContext)(nil).NewPreCommitEndorsement), arg0)
}

// NewProposalEndorsement mocks base method.
func (m *MockContext) NewProposalEndorsement(arg0 interface{}) (interface{}, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewProposalEndorsement", arg0)
	ret0, _ := ret[0].(interface{})
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewProposalEndorsement indicates an expected call of NewProposalEndorsement.
func (mr *MockContextMockRecorder) NewProposalEndorsement(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewProposalEndorsement", reflect.TypeOf((*MockContext)(nil).NewProposalEndorsement), arg0)
}

// PreCommitEndorsement mocks base method.
func (m *MockContext) PreCommitEndorsement() interface{} {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PreCommitEndorsement")
	ret0, _ := ret[0].(interface{})
	return ret0
}

// PreCommitEndorsement indicates an expected call of PreCommitEndorsement.
func (mr *MockContextMockRecorder) PreCommitEndorsement() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PreCommitEndorsement", reflect.TypeOf((*MockContext)(nil).PreCommitEndorsement))
}

// Prepare mocks base method.
func (m *MockContext) Prepare() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Prepare")
	ret0, _ := ret[0].(error)
	return ret0
}

// Prepare indicates an expected call of Prepare.
func (mr *MockContextMockRecorder) Prepare() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Prepare", reflect.TypeOf((*MockContext)(nil).Prepare))
}

// Proposal mocks base method.
func (m *MockContext) Proposal() (interface{}, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Proposal")
	ret0, _ := ret[0].(interface{})
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Proposal indicates an expected call of Proposal.
func (mr *MockContextMockRecorder) Proposal() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Proposal", reflect.TypeOf((*MockContext)(nil).Proposal))
}

// Start mocks base method.
func (m *MockContext) Start(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start.
func (mr *MockContextMockRecorder) Start(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockContext)(nil).Start), arg0)
}

// Stop mocks base method.
func (m *MockContext) Stop(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop.
func (mr *MockContextMockRecorder) Stop(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockContext)(nil).Stop), arg0)
}

// UnmatchedEventInterval mocks base method.
func (m *MockContext) UnmatchedEventInterval(arg0 uint64) time.Duration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UnmatchedEventInterval", arg0)
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

// UnmatchedEventInterval indicates an expected call of UnmatchedEventInterval.
func (mr *MockContextMockRecorder) UnmatchedEventInterval(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnmatchedEventInterval", reflect.TypeOf((*MockContext)(nil).UnmatchedEventInterval), arg0)
}

// UnmatchedEventTTL mocks base method.
func (m *MockContext) UnmatchedEventTTL(arg0 uint64) time.Duration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UnmatchedEventTTL", arg0)
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

// UnmatchedEventTTL indicates an expected call of UnmatchedEventTTL.
func (mr *MockContextMockRecorder) UnmatchedEventTTL(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnmatchedEventTTL", reflect.TypeOf((*MockContext)(nil).UnmatchedEventTTL), arg0)
}

// WaitUntilRoundStart mocks base method.
func (m *MockContext) WaitUntilRoundStart() time.Duration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WaitUntilRoundStart")
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

// WaitUntilRoundStart indicates an expected call of WaitUntilRoundStart.
func (mr *MockContextMockRecorder) WaitUntilRoundStart() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WaitUntilRoundStart", reflect.TypeOf((*MockContext)(nil).WaitUntilRoundStart))
}
