// Code generated by MockGen. DO NOT EDIT.
// Source: ./blockchain/blockchain.go

// Package mock_blockchain is a generated GoMock package.
package mock_blockchain

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	action "github.com/iotexproject/iotex-core/action"
	address "github.com/iotexproject/iotex-core/address"
	blockchain "github.com/iotexproject/iotex-core/blockchain"
	block "github.com/iotexproject/iotex-core/blockchain/block"
	hash "github.com/iotexproject/iotex-core/pkg/hash"
	state "github.com/iotexproject/iotex-core/state"
	factory "github.com/iotexproject/iotex-core/state/factory"
	big "math/big"
	reflect "reflect"
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
	ret := m.ctrl.Call(m, "Start", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start
func (mr *MockBlockchainMockRecorder) Start(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockBlockchain)(nil).Start), arg0)
}

// Stop mocks base method
func (m *MockBlockchain) Stop(arg0 context.Context) error {
	ret := m.ctrl.Call(m, "Stop", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop
func (mr *MockBlockchainMockRecorder) Stop(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockBlockchain)(nil).Stop), arg0)
}

// Balance mocks base method
func (m *MockBlockchain) Balance(addr string) (*big.Int, error) {
	ret := m.ctrl.Call(m, "Balance", addr)
	ret0, _ := ret[0].(*big.Int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Balance indicates an expected call of Balance
func (mr *MockBlockchainMockRecorder) Balance(addr interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Balance", reflect.TypeOf((*MockBlockchain)(nil).Balance), addr)
}

// Nonce mocks base method
func (m *MockBlockchain) Nonce(addr string) (uint64, error) {
	ret := m.ctrl.Call(m, "Nonce", addr)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Nonce indicates an expected call of Nonce
func (mr *MockBlockchainMockRecorder) Nonce(addr interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Nonce", reflect.TypeOf((*MockBlockchain)(nil).Nonce), addr)
}

// CreateState mocks base method
func (m *MockBlockchain) CreateState(addr string, init *big.Int) (*state.Account, error) {
	ret := m.ctrl.Call(m, "CreateState", addr, init)
	ret0, _ := ret[0].(*state.Account)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateState indicates an expected call of CreateState
func (mr *MockBlockchainMockRecorder) CreateState(addr, init interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateState", reflect.TypeOf((*MockBlockchain)(nil).CreateState), addr, init)
}

// CandidatesByHeight mocks base method
func (m *MockBlockchain) CandidatesByHeight(height uint64) ([]*state.Candidate, error) {
	ret := m.ctrl.Call(m, "CandidatesByHeight", height)
	ret0, _ := ret[0].([]*state.Candidate)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CandidatesByHeight indicates an expected call of CandidatesByHeight
func (mr *MockBlockchainMockRecorder) CandidatesByHeight(height interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CandidatesByHeight", reflect.TypeOf((*MockBlockchain)(nil).CandidatesByHeight), height)
}

// BlockProducersByHeight mocks base method
func (m *MockBlockchain) BlockProducersByHeight(height uint64) ([]string, error) {
	ret := m.ctrl.Call(m, "BlockProducersByHeight", height)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// BlockProducersByHeight indicates an expected call of BlockProducersByHeight
func (mr *MockBlockchainMockRecorder) BlockProducersByHeight(height interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlockProducersByHeight", reflect.TypeOf((*MockBlockchain)(nil).BlockProducersByHeight), height)
}

// ActiveBlockProducersByHeight mocks base method
func (m *MockBlockchain) ActiveBlockProducersByHeight(height uint64) ([]string, error) {
	ret := m.ctrl.Call(m, "ActiveBlockProducersByHeight", height)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ActiveBlockProducersByHeight indicates an expected call of ActiveBlockProducersByHeight
func (mr *MockBlockchainMockRecorder) ActiveBlockProducersByHeight(height interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ActiveBlockProducersByHeight", reflect.TypeOf((*MockBlockchain)(nil).ActiveBlockProducersByHeight), height)
}

// GetHeightByHash mocks base method
func (m *MockBlockchain) GetHeightByHash(h hash.Hash256) (uint64, error) {
	ret := m.ctrl.Call(m, "GetHeightByHash", h)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetHeightByHash indicates an expected call of GetHeightByHash
func (mr *MockBlockchainMockRecorder) GetHeightByHash(h interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetHeightByHash", reflect.TypeOf((*MockBlockchain)(nil).GetHeightByHash), h)
}

// GetHashByHeight mocks base method
func (m *MockBlockchain) GetHashByHeight(height uint64) (hash.Hash256, error) {
	ret := m.ctrl.Call(m, "GetHashByHeight", height)
	ret0, _ := ret[0].(hash.Hash256)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetHashByHeight indicates an expected call of GetHashByHeight
func (mr *MockBlockchainMockRecorder) GetHashByHeight(height interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetHashByHeight", reflect.TypeOf((*MockBlockchain)(nil).GetHashByHeight), height)
}

// GetBlockByHeight mocks base method
func (m *MockBlockchain) GetBlockByHeight(height uint64) (*block.Block, error) {
	ret := m.ctrl.Call(m, "GetBlockByHeight", height)
	ret0, _ := ret[0].(*block.Block)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlockByHeight indicates an expected call of GetBlockByHeight
func (mr *MockBlockchainMockRecorder) GetBlockByHeight(height interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockByHeight", reflect.TypeOf((*MockBlockchain)(nil).GetBlockByHeight), height)
}

// GetBlockByHash mocks base method
func (m *MockBlockchain) GetBlockByHash(h hash.Hash256) (*block.Block, error) {
	ret := m.ctrl.Call(m, "GetBlockByHash", h)
	ret0, _ := ret[0].(*block.Block)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlockByHash indicates an expected call of GetBlockByHash
func (mr *MockBlockchainMockRecorder) GetBlockByHash(h interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockByHash", reflect.TypeOf((*MockBlockchain)(nil).GetBlockByHash), h)
}

// GetTotalTransfers mocks base method
func (m *MockBlockchain) GetTotalTransfers() (uint64, error) {
	ret := m.ctrl.Call(m, "GetTotalTransfers")
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTotalTransfers indicates an expected call of GetTotalTransfers
func (mr *MockBlockchainMockRecorder) GetTotalTransfers() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTotalTransfers", reflect.TypeOf((*MockBlockchain)(nil).GetTotalTransfers))
}

// GetTotalVotes mocks base method
func (m *MockBlockchain) GetTotalVotes() (uint64, error) {
	ret := m.ctrl.Call(m, "GetTotalVotes")
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTotalVotes indicates an expected call of GetTotalVotes
func (mr *MockBlockchainMockRecorder) GetTotalVotes() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTotalVotes", reflect.TypeOf((*MockBlockchain)(nil).GetTotalVotes))
}

// GetTotalExecutions mocks base method
func (m *MockBlockchain) GetTotalExecutions() (uint64, error) {
	ret := m.ctrl.Call(m, "GetTotalExecutions")
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTotalExecutions indicates an expected call of GetTotalExecutions
func (mr *MockBlockchainMockRecorder) GetTotalExecutions() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTotalExecutions", reflect.TypeOf((*MockBlockchain)(nil).GetTotalExecutions))
}

// GetTotalActions mocks base method
func (m *MockBlockchain) GetTotalActions() (uint64, error) {
	ret := m.ctrl.Call(m, "GetTotalActions")
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTotalActions indicates an expected call of GetTotalActions
func (mr *MockBlockchainMockRecorder) GetTotalActions() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTotalActions", reflect.TypeOf((*MockBlockchain)(nil).GetTotalActions))
}

// GetTransfersFromAddress mocks base method
func (m *MockBlockchain) GetTransfersFromAddress(address string) ([]hash.Hash256, error) {
	ret := m.ctrl.Call(m, "GetTransfersFromAddress", address)
	ret0, _ := ret[0].([]hash.Hash256)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTransfersFromAddress indicates an expected call of GetTransfersFromAddress
func (mr *MockBlockchainMockRecorder) GetTransfersFromAddress(address interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTransfersFromAddress", reflect.TypeOf((*MockBlockchain)(nil).GetTransfersFromAddress), address)
}

// GetTransfersToAddress mocks base method
func (m *MockBlockchain) GetTransfersToAddress(address string) ([]hash.Hash256, error) {
	ret := m.ctrl.Call(m, "GetTransfersToAddress", address)
	ret0, _ := ret[0].([]hash.Hash256)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTransfersToAddress indicates an expected call of GetTransfersToAddress
func (mr *MockBlockchainMockRecorder) GetTransfersToAddress(address interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTransfersToAddress", reflect.TypeOf((*MockBlockchain)(nil).GetTransfersToAddress), address)
}

// GetTransferByTransferHash mocks base method
func (m *MockBlockchain) GetTransferByTransferHash(h hash.Hash256) (*action.Transfer, error) {
	ret := m.ctrl.Call(m, "GetTransferByTransferHash", h)
	ret0, _ := ret[0].(*action.Transfer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTransferByTransferHash indicates an expected call of GetTransferByTransferHash
func (mr *MockBlockchainMockRecorder) GetTransferByTransferHash(h interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTransferByTransferHash", reflect.TypeOf((*MockBlockchain)(nil).GetTransferByTransferHash), h)
}

// GetBlockHashByTransferHash mocks base method
func (m *MockBlockchain) GetBlockHashByTransferHash(h hash.Hash256) (hash.Hash256, error) {
	ret := m.ctrl.Call(m, "GetBlockHashByTransferHash", h)
	ret0, _ := ret[0].(hash.Hash256)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlockHashByTransferHash indicates an expected call of GetBlockHashByTransferHash
func (mr *MockBlockchainMockRecorder) GetBlockHashByTransferHash(h interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockHashByTransferHash", reflect.TypeOf((*MockBlockchain)(nil).GetBlockHashByTransferHash), h)
}

// GetVotesFromAddress mocks base method
func (m *MockBlockchain) GetVotesFromAddress(address string) ([]hash.Hash256, error) {
	ret := m.ctrl.Call(m, "GetVotesFromAddress", address)
	ret0, _ := ret[0].([]hash.Hash256)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetVotesFromAddress indicates an expected call of GetVotesFromAddress
func (mr *MockBlockchainMockRecorder) GetVotesFromAddress(address interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVotesFromAddress", reflect.TypeOf((*MockBlockchain)(nil).GetVotesFromAddress), address)
}

// GetVotesToAddress mocks base method
func (m *MockBlockchain) GetVotesToAddress(address string) ([]hash.Hash256, error) {
	ret := m.ctrl.Call(m, "GetVotesToAddress", address)
	ret0, _ := ret[0].([]hash.Hash256)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetVotesToAddress indicates an expected call of GetVotesToAddress
func (mr *MockBlockchainMockRecorder) GetVotesToAddress(address interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVotesToAddress", reflect.TypeOf((*MockBlockchain)(nil).GetVotesToAddress), address)
}

// GetVoteByVoteHash mocks base method
func (m *MockBlockchain) GetVoteByVoteHash(h hash.Hash256) (*action.Vote, error) {
	ret := m.ctrl.Call(m, "GetVoteByVoteHash", h)
	ret0, _ := ret[0].(*action.Vote)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetVoteByVoteHash indicates an expected call of GetVoteByVoteHash
func (mr *MockBlockchainMockRecorder) GetVoteByVoteHash(h interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVoteByVoteHash", reflect.TypeOf((*MockBlockchain)(nil).GetVoteByVoteHash), h)
}

// GetBlockHashByVoteHash mocks base method
func (m *MockBlockchain) GetBlockHashByVoteHash(h hash.Hash256) (hash.Hash256, error) {
	ret := m.ctrl.Call(m, "GetBlockHashByVoteHash", h)
	ret0, _ := ret[0].(hash.Hash256)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlockHashByVoteHash indicates an expected call of GetBlockHashByVoteHash
func (mr *MockBlockchainMockRecorder) GetBlockHashByVoteHash(h interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockHashByVoteHash", reflect.TypeOf((*MockBlockchain)(nil).GetBlockHashByVoteHash), h)
}

// GetExecutionsFromAddress mocks base method
func (m *MockBlockchain) GetExecutionsFromAddress(address string) ([]hash.Hash256, error) {
	ret := m.ctrl.Call(m, "GetExecutionsFromAddress", address)
	ret0, _ := ret[0].([]hash.Hash256)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetExecutionsFromAddress indicates an expected call of GetExecutionsFromAddress
func (mr *MockBlockchainMockRecorder) GetExecutionsFromAddress(address interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetExecutionsFromAddress", reflect.TypeOf((*MockBlockchain)(nil).GetExecutionsFromAddress), address)
}

// GetExecutionsToAddress mocks base method
func (m *MockBlockchain) GetExecutionsToAddress(address string) ([]hash.Hash256, error) {
	ret := m.ctrl.Call(m, "GetExecutionsToAddress", address)
	ret0, _ := ret[0].([]hash.Hash256)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetExecutionsToAddress indicates an expected call of GetExecutionsToAddress
func (mr *MockBlockchainMockRecorder) GetExecutionsToAddress(address interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetExecutionsToAddress", reflect.TypeOf((*MockBlockchain)(nil).GetExecutionsToAddress), address)
}

// GetExecutionByExecutionHash mocks base method
func (m *MockBlockchain) GetExecutionByExecutionHash(h hash.Hash256) (*action.Execution, error) {
	ret := m.ctrl.Call(m, "GetExecutionByExecutionHash", h)
	ret0, _ := ret[0].(*action.Execution)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetExecutionByExecutionHash indicates an expected call of GetExecutionByExecutionHash
func (mr *MockBlockchainMockRecorder) GetExecutionByExecutionHash(h interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetExecutionByExecutionHash", reflect.TypeOf((*MockBlockchain)(nil).GetExecutionByExecutionHash), h)
}

// GetBlockHashByExecutionHash mocks base method
func (m *MockBlockchain) GetBlockHashByExecutionHash(h hash.Hash256) (hash.Hash256, error) {
	ret := m.ctrl.Call(m, "GetBlockHashByExecutionHash", h)
	ret0, _ := ret[0].(hash.Hash256)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlockHashByExecutionHash indicates an expected call of GetBlockHashByExecutionHash
func (mr *MockBlockchainMockRecorder) GetBlockHashByExecutionHash(h interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockHashByExecutionHash", reflect.TypeOf((*MockBlockchain)(nil).GetBlockHashByExecutionHash), h)
}

// GetReceiptByActionHash mocks base method
func (m *MockBlockchain) GetReceiptByActionHash(h hash.Hash256) (*action.Receipt, error) {
	ret := m.ctrl.Call(m, "GetReceiptByActionHash", h)
	ret0, _ := ret[0].(*action.Receipt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetReceiptByActionHash indicates an expected call of GetReceiptByActionHash
func (mr *MockBlockchainMockRecorder) GetReceiptByActionHash(h interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetReceiptByActionHash", reflect.TypeOf((*MockBlockchain)(nil).GetReceiptByActionHash), h)
}

// GetActionsFromAddress mocks base method
func (m *MockBlockchain) GetActionsFromAddress(address string) ([]hash.Hash256, error) {
	ret := m.ctrl.Call(m, "GetActionsFromAddress", address)
	ret0, _ := ret[0].([]hash.Hash256)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetActionsFromAddress indicates an expected call of GetActionsFromAddress
func (mr *MockBlockchainMockRecorder) GetActionsFromAddress(address interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetActionsFromAddress", reflect.TypeOf((*MockBlockchain)(nil).GetActionsFromAddress), address)
}

// GetActionsToAddress mocks base method
func (m *MockBlockchain) GetActionsToAddress(address string) ([]hash.Hash256, error) {
	ret := m.ctrl.Call(m, "GetActionsToAddress", address)
	ret0, _ := ret[0].([]hash.Hash256)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetActionsToAddress indicates an expected call of GetActionsToAddress
func (mr *MockBlockchainMockRecorder) GetActionsToAddress(address interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetActionsToAddress", reflect.TypeOf((*MockBlockchain)(nil).GetActionsToAddress), address)
}

// GetActionByActionHash mocks base method
func (m *MockBlockchain) GetActionByActionHash(h hash.Hash256) (action.SealedEnvelope, error) {
	ret := m.ctrl.Call(m, "GetActionByActionHash", h)
	ret0, _ := ret[0].(action.SealedEnvelope)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetActionByActionHash indicates an expected call of GetActionByActionHash
func (mr *MockBlockchainMockRecorder) GetActionByActionHash(h interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetActionByActionHash", reflect.TypeOf((*MockBlockchain)(nil).GetActionByActionHash), h)
}

// GetBlockHashByActionHash mocks base method
func (m *MockBlockchain) GetBlockHashByActionHash(h hash.Hash256) (hash.Hash256, error) {
	ret := m.ctrl.Call(m, "GetBlockHashByActionHash", h)
	ret0, _ := ret[0].(hash.Hash256)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlockHashByActionHash indicates an expected call of GetBlockHashByActionHash
func (mr *MockBlockchainMockRecorder) GetBlockHashByActionHash(h interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockHashByActionHash", reflect.TypeOf((*MockBlockchain)(nil).GetBlockHashByActionHash), h)
}

// GetFactory mocks base method
func (m *MockBlockchain) GetFactory() factory.Factory {
	ret := m.ctrl.Call(m, "GetFactory")
	ret0, _ := ret[0].(factory.Factory)
	return ret0
}

// GetFactory indicates an expected call of GetFactory
func (mr *MockBlockchainMockRecorder) GetFactory() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetFactory", reflect.TypeOf((*MockBlockchain)(nil).GetFactory))
}

// ChainID mocks base method
func (m *MockBlockchain) ChainID() uint32 {
	ret := m.ctrl.Call(m, "ChainID")
	ret0, _ := ret[0].(uint32)
	return ret0
}

// ChainID indicates an expected call of ChainID
func (mr *MockBlockchainMockRecorder) ChainID() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ChainID", reflect.TypeOf((*MockBlockchain)(nil).ChainID))
}

// ChainAddress mocks base method
func (m *MockBlockchain) ChainAddress() string {
	ret := m.ctrl.Call(m, "ChainAddress")
	ret0, _ := ret[0].(string)
	return ret0
}

// ChainAddress indicates an expected call of ChainAddress
func (mr *MockBlockchainMockRecorder) ChainAddress() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ChainAddress", reflect.TypeOf((*MockBlockchain)(nil).ChainAddress))
}

// TipHash mocks base method
func (m *MockBlockchain) TipHash() hash.Hash256 {
	ret := m.ctrl.Call(m, "TipHash")
	ret0, _ := ret[0].(hash.Hash256)
	return ret0
}

// TipHash indicates an expected call of TipHash
func (mr *MockBlockchainMockRecorder) TipHash() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TipHash", reflect.TypeOf((*MockBlockchain)(nil).TipHash))
}

// TipHeight mocks base method
func (m *MockBlockchain) TipHeight() uint64 {
	ret := m.ctrl.Call(m, "TipHeight")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// TipHeight indicates an expected call of TipHeight
func (mr *MockBlockchainMockRecorder) TipHeight() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TipHeight", reflect.TypeOf((*MockBlockchain)(nil).TipHeight))
}

// StateByAddr mocks base method
func (m *MockBlockchain) StateByAddr(address string) (*state.Account, error) {
	ret := m.ctrl.Call(m, "StateByAddr", address)
	ret0, _ := ret[0].(*state.Account)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StateByAddr indicates an expected call of StateByAddr
func (mr *MockBlockchainMockRecorder) StateByAddr(address interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StateByAddr", reflect.TypeOf((*MockBlockchain)(nil).StateByAddr), address)
}

// RecoverChainAndState mocks base method
func (m *MockBlockchain) RecoverChainAndState(targetHeight uint64) error {
	ret := m.ctrl.Call(m, "RecoverChainAndState", targetHeight)
	ret0, _ := ret[0].(error)
	return ret0
}

// RecoverChainAndState indicates an expected call of RecoverChainAndState
func (mr *MockBlockchainMockRecorder) RecoverChainAndState(targetHeight interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecoverChainAndState", reflect.TypeOf((*MockBlockchain)(nil).RecoverChainAndState), targetHeight)
}

// MintNewBlock mocks base method
func (m *MockBlockchain) MintNewBlock(actionMap map[string][]action.SealedEnvelope, timestamp int64) (*block.Block, error) {
	ret := m.ctrl.Call(m, "MintNewBlock", actionMap, timestamp)
	ret0, _ := ret[0].(*block.Block)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MintNewBlock indicates an expected call of MintNewBlock
func (mr *MockBlockchainMockRecorder) MintNewBlock(actionMap, timestamp interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MintNewBlock", reflect.TypeOf((*MockBlockchain)(nil).MintNewBlock), actionMap, timestamp)
}

// CommitBlock mocks base method
func (m *MockBlockchain) CommitBlock(blk *block.Block) error {
	ret := m.ctrl.Call(m, "CommitBlock", blk)
	ret0, _ := ret[0].(error)
	return ret0
}

// CommitBlock indicates an expected call of CommitBlock
func (mr *MockBlockchainMockRecorder) CommitBlock(blk interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommitBlock", reflect.TypeOf((*MockBlockchain)(nil).CommitBlock), blk)
}

// ValidateBlock mocks base method
func (m *MockBlockchain) ValidateBlock(blk *block.Block) error {
	ret := m.ctrl.Call(m, "ValidateBlock", blk)
	ret0, _ := ret[0].(error)
	return ret0
}

// ValidateBlock indicates an expected call of ValidateBlock
func (mr *MockBlockchainMockRecorder) ValidateBlock(blk interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidateBlock", reflect.TypeOf((*MockBlockchain)(nil).ValidateBlock), blk)
}

// Validator mocks base method
func (m *MockBlockchain) Validator() blockchain.Validator {
	ret := m.ctrl.Call(m, "Validator")
	ret0, _ := ret[0].(blockchain.Validator)
	return ret0
}

// Validator indicates an expected call of Validator
func (mr *MockBlockchainMockRecorder) Validator() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Validator", reflect.TypeOf((*MockBlockchain)(nil).Validator))
}

// SetValidator mocks base method
func (m *MockBlockchain) SetValidator(val blockchain.Validator) {
	m.ctrl.Call(m, "SetValidator", val)
}

// SetValidator indicates an expected call of SetValidator
func (mr *MockBlockchainMockRecorder) SetValidator(val interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetValidator", reflect.TypeOf((*MockBlockchain)(nil).SetValidator), val)
}

// ExecuteContractRead mocks base method
func (m *MockBlockchain) ExecuteContractRead(caller address.Address, ex *action.Execution) (*action.Receipt, error) {
	ret := m.ctrl.Call(m, "ExecuteContractRead", caller, ex)
	ret0, _ := ret[0].(*action.Receipt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ExecuteContractRead indicates an expected call of ExecuteContractRead
func (mr *MockBlockchainMockRecorder) ExecuteContractRead(caller, ex interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExecuteContractRead", reflect.TypeOf((*MockBlockchain)(nil).ExecuteContractRead), caller, ex)
}

// AddSubscriber mocks base method
func (m *MockBlockchain) AddSubscriber(arg0 blockchain.BlockCreationSubscriber) error {
	ret := m.ctrl.Call(m, "AddSubscriber", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddSubscriber indicates an expected call of AddSubscriber
func (mr *MockBlockchainMockRecorder) AddSubscriber(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddSubscriber", reflect.TypeOf((*MockBlockchain)(nil).AddSubscriber), arg0)
}

// RemoveSubscriber mocks base method
func (m *MockBlockchain) RemoveSubscriber(arg0 blockchain.BlockCreationSubscriber) error {
	ret := m.ctrl.Call(m, "RemoveSubscriber", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveSubscriber indicates an expected call of RemoveSubscriber
func (mr *MockBlockchainMockRecorder) RemoveSubscriber(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveSubscriber", reflect.TypeOf((*MockBlockchain)(nil).RemoveSubscriber), arg0)
}
