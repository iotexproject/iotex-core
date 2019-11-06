// Code generated by MockGen. DO NOT EDIT.
// Source: ./blockchain/blockchain.go

// Package mock_blockchain is a generated GoMock package.
package mock_blockchain

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	hash "github.com/iotexproject/go-pkgs/hash"
	address "github.com/iotexproject/iotex-address/address"
	action "github.com/iotexproject/iotex-core/action"
	blockchain "github.com/iotexproject/iotex-core/blockchain"
	block "github.com/iotexproject/iotex-core/blockchain/block"
	state "github.com/iotexproject/iotex-core/state"
	big "math/big"
	reflect "reflect"
	time "time"
)

// MockBlockchain is a mock of Blockchain interface
type MockBlockchain struct {
	ctrl     *gomock.Controller
	recorder *MockBlockchainMockRecorder
}

// MockBlockchainMockRecorder is the mock recorder for MockBlockchain
type MockBlockchainMockRecorder struct {
	mock *MockBlockchain
}

// NewMockBlockchain creates a new mock instance
func NewMockBlockchain(ctrl *gomock.Controller) *MockBlockchain {
	mock := &MockBlockchain{ctrl: ctrl}
	mock.recorder = &MockBlockchainMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockBlockchain) EXPECT() *MockBlockchainMockRecorder {
	return m.recorder
}

// Start mocks base method
func (m *MockBlockchain) Start(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start
func (mr *MockBlockchainMockRecorder) Start(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockBlockchain)(nil).Start), arg0)
}

// Stop mocks base method
func (m *MockBlockchain) Stop(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop
func (mr *MockBlockchainMockRecorder) Stop(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockBlockchain)(nil).Stop), arg0)
}

// Balance mocks base method
func (m *MockBlockchain) Balance(addr string) (*big.Int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Balance", addr)
	ret0, _ := ret[0].(*big.Int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Balance indicates an expected call of Balance
func (mr *MockBlockchainMockRecorder) Balance(addr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Balance", reflect.TypeOf((*MockBlockchain)(nil).Balance), addr)
}

// Nonce mocks base method
func (m *MockBlockchain) Nonce(addr string) (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Nonce", addr)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Nonce indicates an expected call of Nonce
func (mr *MockBlockchainMockRecorder) Nonce(addr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Nonce", reflect.TypeOf((*MockBlockchain)(nil).Nonce), addr)
}

// CreateState mocks base method
func (m *MockBlockchain) CreateState(addr string, init *big.Int) (*state.Account, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateState", addr, init)
	ret0, _ := ret[0].(*state.Account)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateState indicates an expected call of CreateState
func (mr *MockBlockchainMockRecorder) CreateState(addr, init interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateState", reflect.TypeOf((*MockBlockchain)(nil).CreateState), addr, init)
}

// CandidatesByHeight mocks base method
func (m *MockBlockchain) CandidatesByHeight(height uint64) ([]*state.Candidate, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CandidatesByHeight", height)
	ret0, _ := ret[0].([]*state.Candidate)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CandidatesByHeight indicates an expected call of CandidatesByHeight
func (mr *MockBlockchainMockRecorder) CandidatesByHeight(height interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CandidatesByHeight", reflect.TypeOf((*MockBlockchain)(nil).CandidatesByHeight), height)
}

// ProductivityByEpoch mocks base method
func (m *MockBlockchain) ProductivityByEpoch(epochNum uint64) (uint64, map[string]uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ProductivityByEpoch", epochNum)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(map[string]uint64)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ProductivityByEpoch indicates an expected call of ProductivityByEpoch
func (mr *MockBlockchainMockRecorder) ProductivityByEpoch(epochNum interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ProductivityByEpoch", reflect.TypeOf((*MockBlockchain)(nil).ProductivityByEpoch), epochNum)
}

// GetHeightByHash mocks base method
func (m *MockBlockchain) GetHeightByHash(h hash.Hash256) (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetHeightByHash", h)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetHeightByHash indicates an expected call of GetHeightByHash
func (mr *MockBlockchainMockRecorder) GetHeightByHash(h interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetHeightByHash", reflect.TypeOf((*MockBlockchain)(nil).GetHeightByHash), h)
}

// GetHashByHeight mocks base method
func (m *MockBlockchain) GetHashByHeight(height uint64) (hash.Hash256, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetHashByHeight", height)
	ret0, _ := ret[0].(hash.Hash256)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetHashByHeight indicates an expected call of GetHashByHeight
func (mr *MockBlockchainMockRecorder) GetHashByHeight(height interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetHashByHeight", reflect.TypeOf((*MockBlockchain)(nil).GetHashByHeight), height)
}

// GetBlockByHeight mocks base method
func (m *MockBlockchain) GetBlockByHeight(height uint64) (*block.Block, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBlockByHeight", height)
	ret0, _ := ret[0].(*block.Block)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlockByHeight indicates an expected call of GetBlockByHeight
func (mr *MockBlockchainMockRecorder) GetBlockByHeight(height interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockByHeight", reflect.TypeOf((*MockBlockchain)(nil).GetBlockByHeight), height)
}

// GetBlockByHash mocks base method
func (m *MockBlockchain) GetBlockByHash(h hash.Hash256) (*block.Block, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBlockByHash", h)
	ret0, _ := ret[0].(*block.Block)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlockByHash indicates an expected call of GetBlockByHash
func (mr *MockBlockchainMockRecorder) GetBlockByHash(h interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockByHash", reflect.TypeOf((*MockBlockchain)(nil).GetBlockByHash), h)
}

// BlockHeaderByHeight mocks base method
func (m *MockBlockchain) BlockHeaderByHeight(height uint64) (*block.Header, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlockHeaderByHeight", height)
	ret0, _ := ret[0].(*block.Header)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// BlockHeaderByHeight indicates an expected call of BlockHeaderByHeight
func (mr *MockBlockchainMockRecorder) BlockHeaderByHeight(height interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlockHeaderByHeight", reflect.TypeOf((*MockBlockchain)(nil).BlockHeaderByHeight), height)
}

// BlockHeaderByHash mocks base method
func (m *MockBlockchain) BlockHeaderByHash(h hash.Hash256) (*block.Header, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlockHeaderByHash", h)
	ret0, _ := ret[0].(*block.Header)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// BlockHeaderByHash indicates an expected call of BlockHeaderByHash
func (mr *MockBlockchainMockRecorder) BlockHeaderByHash(h interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlockHeaderByHash", reflect.TypeOf((*MockBlockchain)(nil).BlockHeaderByHash), h)
}

// BlockFooterByHeight mocks base method
func (m *MockBlockchain) BlockFooterByHeight(height uint64) (*block.Footer, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlockFooterByHeight", height)
	ret0, _ := ret[0].(*block.Footer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// BlockFooterByHeight indicates an expected call of BlockFooterByHeight
func (mr *MockBlockchainMockRecorder) BlockFooterByHeight(height interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlockFooterByHeight", reflect.TypeOf((*MockBlockchain)(nil).BlockFooterByHeight), height)
}

// BlockFooterByHash mocks base method
func (m *MockBlockchain) BlockFooterByHash(h hash.Hash256) (*block.Footer, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlockFooterByHash", h)
	ret0, _ := ret[0].(*block.Footer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// BlockFooterByHash indicates an expected call of BlockFooterByHash
func (mr *MockBlockchainMockRecorder) BlockFooterByHash(h interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlockFooterByHash", reflect.TypeOf((*MockBlockchain)(nil).BlockFooterByHash), h)
}

// ChainID mocks base method
func (m *MockBlockchain) ChainID() uint32 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ChainID")
	ret0, _ := ret[0].(uint32)
	return ret0
}

// ChainID indicates an expected call of ChainID
func (mr *MockBlockchainMockRecorder) ChainID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ChainID", reflect.TypeOf((*MockBlockchain)(nil).ChainID))
}

// ChainAddress mocks base method
func (m *MockBlockchain) ChainAddress() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ChainAddress")
	ret0, _ := ret[0].(string)
	return ret0
}

// ChainAddress indicates an expected call of ChainAddress
func (mr *MockBlockchainMockRecorder) ChainAddress() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ChainAddress", reflect.TypeOf((*MockBlockchain)(nil).ChainAddress))
}

// TipHash mocks base method
func (m *MockBlockchain) TipHash() hash.Hash256 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TipHash")
	ret0, _ := ret[0].(hash.Hash256)
	return ret0
}

// TipHash indicates an expected call of TipHash
func (mr *MockBlockchainMockRecorder) TipHash() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TipHash", reflect.TypeOf((*MockBlockchain)(nil).TipHash))
}

// TipHeight mocks base method
func (m *MockBlockchain) TipHeight() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TipHeight")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// TipHeight indicates an expected call of TipHeight
func (mr *MockBlockchainMockRecorder) TipHeight() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TipHeight", reflect.TypeOf((*MockBlockchain)(nil).TipHeight))
}

// StateByAddr mocks base method
func (m *MockBlockchain) StateByAddr(address string) (*state.Account, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StateByAddr", address)
	ret0, _ := ret[0].(*state.Account)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StateByAddr indicates an expected call of StateByAddr
func (mr *MockBlockchainMockRecorder) StateByAddr(address interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StateByAddr", reflect.TypeOf((*MockBlockchain)(nil).StateByAddr), address)
}

// RecoverChainAndState mocks base method
func (m *MockBlockchain) RecoverChainAndState(targetHeight uint64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RecoverChainAndState", targetHeight)
	ret0, _ := ret[0].(error)
	return ret0
}

// RecoverChainAndState indicates an expected call of RecoverChainAndState
func (mr *MockBlockchainMockRecorder) RecoverChainAndState(targetHeight interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecoverChainAndState", reflect.TypeOf((*MockBlockchain)(nil).RecoverChainAndState), targetHeight)
}

// GenesisTimestamp mocks base method
func (m *MockBlockchain) GenesisTimestamp() int64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GenesisTimestamp")
	ret0, _ := ret[0].(int64)
	return ret0
}

// GenesisTimestamp indicates an expected call of GenesisTimestamp
func (mr *MockBlockchainMockRecorder) GenesisTimestamp() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GenesisTimestamp", reflect.TypeOf((*MockBlockchain)(nil).GenesisTimestamp))
}

// MintNewBlock mocks base method
func (m *MockBlockchain) MintNewBlock(actionMap map[string][]action.SealedEnvelope, timestamp time.Time) (*block.Block, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MintNewBlock", actionMap, timestamp)
	ret0, _ := ret[0].(*block.Block)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MintNewBlock indicates an expected call of MintNewBlock
func (mr *MockBlockchainMockRecorder) MintNewBlock(actionMap, timestamp interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MintNewBlock", reflect.TypeOf((*MockBlockchain)(nil).MintNewBlock), actionMap, timestamp)
}

// CommitBlock mocks base method
func (m *MockBlockchain) CommitBlock(blk *block.Block) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CommitBlock", blk)
	ret0, _ := ret[0].(error)
	return ret0
}

// CommitBlock indicates an expected call of CommitBlock
func (mr *MockBlockchainMockRecorder) CommitBlock(blk interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommitBlock", reflect.TypeOf((*MockBlockchain)(nil).CommitBlock), blk)
}

// ValidateBlock mocks base method
func (m *MockBlockchain) ValidateBlock(blk *block.Block) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ValidateBlock", blk)
	ret0, _ := ret[0].(error)
	return ret0
}

// ValidateBlock indicates an expected call of ValidateBlock
func (mr *MockBlockchainMockRecorder) ValidateBlock(blk interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidateBlock", reflect.TypeOf((*MockBlockchain)(nil).ValidateBlock), blk)
}

// Validator mocks base method
func (m *MockBlockchain) Validator() blockchain.Validator {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Validator")
	ret0, _ := ret[0].(blockchain.Validator)
	return ret0
}

// Validator indicates an expected call of Validator
func (mr *MockBlockchainMockRecorder) Validator() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Validator", reflect.TypeOf((*MockBlockchain)(nil).Validator))
}

// SetValidator mocks base method
func (m *MockBlockchain) SetValidator(val blockchain.Validator) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetValidator", val)
}

// SetValidator indicates an expected call of SetValidator
func (mr *MockBlockchainMockRecorder) SetValidator(val interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetValidator", reflect.TypeOf((*MockBlockchain)(nil).SetValidator), val)
}

// ExecuteContractRead mocks base method
func (m *MockBlockchain) ExecuteContractRead(caller address.Address, ex *action.Execution) ([]byte, *action.Receipt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ExecuteContractRead", caller, ex)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(*action.Receipt)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ExecuteContractRead indicates an expected call of ExecuteContractRead
func (mr *MockBlockchainMockRecorder) ExecuteContractRead(caller, ex interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExecuteContractRead", reflect.TypeOf((*MockBlockchain)(nil).ExecuteContractRead), caller, ex)
}

// AddSubscriber mocks base method
func (m *MockBlockchain) AddSubscriber(arg0 blockchain.BlockCreationSubscriber) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddSubscriber", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddSubscriber indicates an expected call of AddSubscriber
func (mr *MockBlockchainMockRecorder) AddSubscriber(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddSubscriber", reflect.TypeOf((*MockBlockchain)(nil).AddSubscriber), arg0)
}

// RemoveSubscriber mocks base method
func (m *MockBlockchain) RemoveSubscriber(arg0 blockchain.BlockCreationSubscriber) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveSubscriber", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveSubscriber indicates an expected call of RemoveSubscriber
func (mr *MockBlockchainMockRecorder) RemoveSubscriber(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveSubscriber", reflect.TypeOf((*MockBlockchain)(nil).RemoveSubscriber), arg0)
}

// MockActPoolManager is a mock of ActPoolManager interface
type MockActPoolManager struct {
	ctrl     *gomock.Controller
	recorder *MockActPoolManagerMockRecorder
}

// MockActPoolManagerMockRecorder is the mock recorder for MockActPoolManager
type MockActPoolManagerMockRecorder struct {
	mock *MockActPoolManager
}

// NewMockActPoolManager creates a new mock instance
func NewMockActPoolManager(ctrl *gomock.Controller) *MockActPoolManager {
	mock := &MockActPoolManager{ctrl: ctrl}
	mock.recorder = &MockActPoolManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockActPoolManager) EXPECT() *MockActPoolManagerMockRecorder {
	return m.recorder
}

// GetActionByHash mocks base method
func (m *MockActPoolManager) GetActionByHash(hash hash.Hash256) (action.SealedEnvelope, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetActionByHash", hash)
	ret0, _ := ret[0].(action.SealedEnvelope)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetActionByHash indicates an expected call of GetActionByHash
func (mr *MockActPoolManagerMockRecorder) GetActionByHash(hash interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetActionByHash", reflect.TypeOf((*MockActPoolManager)(nil).GetActionByHash), hash)
}
