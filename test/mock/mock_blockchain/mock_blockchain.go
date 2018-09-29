// Code generated by MockGen. DO NOT EDIT.
// Source: ./blockchain/blockchain.go

// Package mock_blockchain is a generated GoMock package.
package mock_blockchain

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	blockchain "github.com/iotexproject/iotex-core/blockchain"
	action "github.com/iotexproject/iotex-core/blockchain/action"
	iotxaddress "github.com/iotexproject/iotex-core/iotxaddress"
	hash "github.com/iotexproject/iotex-core/pkg/hash"
	state "github.com/iotexproject/iotex-core/state"
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
func (m *MockBlockchain) CreateState(addr string, init uint64) (*state.State, error) {
	ret := m.ctrl.Call(m, "CreateState", addr, init)
	ret0, _ := ret[0].(*state.State)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateState indicates an expected call of CreateState
func (mr *MockBlockchainMockRecorder) CreateState(addr, init interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateState", reflect.TypeOf((*MockBlockchain)(nil).CreateState), addr, init)
}

// Candidates mocks base method
func (m *MockBlockchain) Candidates() (uint64, []*state.Candidate) {
	ret := m.ctrl.Call(m, "Candidates")
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].([]*state.Candidate)
	return ret0, ret1
}

// Candidates indicates an expected call of Candidates
func (mr *MockBlockchainMockRecorder) Candidates() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Candidates", reflect.TypeOf((*MockBlockchain)(nil).Candidates))
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

// GetHeightByHash mocks base method
func (m *MockBlockchain) GetHeightByHash(h hash.Hash32B) (uint64, error) {
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
func (m *MockBlockchain) GetHashByHeight(height uint64) (hash.Hash32B, error) {
	ret := m.ctrl.Call(m, "GetHashByHeight", height)
	ret0, _ := ret[0].(hash.Hash32B)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetHashByHeight indicates an expected call of GetHashByHeight
func (mr *MockBlockchainMockRecorder) GetHashByHeight(height interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetHashByHeight", reflect.TypeOf((*MockBlockchain)(nil).GetHashByHeight), height)
}

// GetBlockByHeight mocks base method
func (m *MockBlockchain) GetBlockByHeight(height uint64) (*blockchain.Block, error) {
	ret := m.ctrl.Call(m, "GetBlockByHeight", height)
	ret0, _ := ret[0].(*blockchain.Block)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlockByHeight indicates an expected call of GetBlockByHeight
func (mr *MockBlockchainMockRecorder) GetBlockByHeight(height interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockByHeight", reflect.TypeOf((*MockBlockchain)(nil).GetBlockByHeight), height)
}

// GetBlockByHash mocks base method
func (m *MockBlockchain) GetBlockByHash(h hash.Hash32B) (*blockchain.Block, error) {
	ret := m.ctrl.Call(m, "GetBlockByHash", h)
	ret0, _ := ret[0].(*blockchain.Block)
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

// GetTransfersFromAddress mocks base method
func (m *MockBlockchain) GetTransfersFromAddress(address string) ([]hash.Hash32B, error) {
	ret := m.ctrl.Call(m, "GetTransfersFromAddress", address)
	ret0, _ := ret[0].([]hash.Hash32B)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTransfersFromAddress indicates an expected call of GetTransfersFromAddress
func (mr *MockBlockchainMockRecorder) GetTransfersFromAddress(address interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTransfersFromAddress", reflect.TypeOf((*MockBlockchain)(nil).GetTransfersFromAddress), address)
}

// GetTransfersToAddress mocks base method
func (m *MockBlockchain) GetTransfersToAddress(address string) ([]hash.Hash32B, error) {
	ret := m.ctrl.Call(m, "GetTransfersToAddress", address)
	ret0, _ := ret[0].([]hash.Hash32B)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTransfersToAddress indicates an expected call of GetTransfersToAddress
func (mr *MockBlockchainMockRecorder) GetTransfersToAddress(address interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTransfersToAddress", reflect.TypeOf((*MockBlockchain)(nil).GetTransfersToAddress), address)
}

// GetTransferByTransferHash mocks base method
func (m *MockBlockchain) GetTransferByTransferHash(h hash.Hash32B) (*action.Transfer, error) {
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
func (m *MockBlockchain) GetBlockHashByTransferHash(h hash.Hash32B) (hash.Hash32B, error) {
	ret := m.ctrl.Call(m, "GetBlockHashByTransferHash", h)
	ret0, _ := ret[0].(hash.Hash32B)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlockHashByTransferHash indicates an expected call of GetBlockHashByTransferHash
func (mr *MockBlockchainMockRecorder) GetBlockHashByTransferHash(h interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockHashByTransferHash", reflect.TypeOf((*MockBlockchain)(nil).GetBlockHashByTransferHash), h)
}

// GetVotesFromAddress mocks base method
func (m *MockBlockchain) GetVotesFromAddress(address string) ([]hash.Hash32B, error) {
	ret := m.ctrl.Call(m, "GetVotesFromAddress", address)
	ret0, _ := ret[0].([]hash.Hash32B)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetVotesFromAddress indicates an expected call of GetVotesFromAddress
func (mr *MockBlockchainMockRecorder) GetVotesFromAddress(address interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVotesFromAddress", reflect.TypeOf((*MockBlockchain)(nil).GetVotesFromAddress), address)
}

// GetVotesToAddress mocks base method
func (m *MockBlockchain) GetVotesToAddress(address string) ([]hash.Hash32B, error) {
	ret := m.ctrl.Call(m, "GetVotesToAddress", address)
	ret0, _ := ret[0].([]hash.Hash32B)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetVotesToAddress indicates an expected call of GetVotesToAddress
func (mr *MockBlockchainMockRecorder) GetVotesToAddress(address interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVotesToAddress", reflect.TypeOf((*MockBlockchain)(nil).GetVotesToAddress), address)
}

// GetVoteByVoteHash mocks base method
func (m *MockBlockchain) GetVoteByVoteHash(h hash.Hash32B) (*action.Vote, error) {
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
func (m *MockBlockchain) GetBlockHashByVoteHash(h hash.Hash32B) (hash.Hash32B, error) {
	ret := m.ctrl.Call(m, "GetBlockHashByVoteHash", h)
	ret0, _ := ret[0].(hash.Hash32B)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlockHashByVoteHash indicates an expected call of GetBlockHashByVoteHash
func (mr *MockBlockchainMockRecorder) GetBlockHashByVoteHash(h interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockHashByVoteHash", reflect.TypeOf((*MockBlockchain)(nil).GetBlockHashByVoteHash), h)
}

// GetExecutionsFromAddress mocks base method
func (m *MockBlockchain) GetExecutionsFromAddress(address string) ([]hash.Hash32B, error) {
	ret := m.ctrl.Call(m, "GetExecutionsFromAddress", address)
	ret0, _ := ret[0].([]hash.Hash32B)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetExecutionsFromAddress indicates an expected call of GetExecutionsFromAddress
func (mr *MockBlockchainMockRecorder) GetExecutionsFromAddress(address interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetExecutionsFromAddress", reflect.TypeOf((*MockBlockchain)(nil).GetExecutionsFromAddress), address)
}

// GetExecutionsToAddress mocks base method
func (m *MockBlockchain) GetExecutionsToAddress(address string) ([]hash.Hash32B, error) {
	ret := m.ctrl.Call(m, "GetExecutionsToAddress", address)
	ret0, _ := ret[0].([]hash.Hash32B)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetExecutionsToAddress indicates an expected call of GetExecutionsToAddress
func (mr *MockBlockchainMockRecorder) GetExecutionsToAddress(address interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetExecutionsToAddress", reflect.TypeOf((*MockBlockchain)(nil).GetExecutionsToAddress), address)
}

// GetExecutionByExecutionHash mocks base method
func (m *MockBlockchain) GetExecutionByExecutionHash(h hash.Hash32B) (*action.Execution, error) {
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
func (m *MockBlockchain) GetBlockHashByExecutionHash(h hash.Hash32B) (hash.Hash32B, error) {
	ret := m.ctrl.Call(m, "GetBlockHashByExecutionHash", h)
	ret0, _ := ret[0].(hash.Hash32B)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlockHashByExecutionHash indicates an expected call of GetBlockHashByExecutionHash
func (mr *MockBlockchainMockRecorder) GetBlockHashByExecutionHash(h interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockHashByExecutionHash", reflect.TypeOf((*MockBlockchain)(nil).GetBlockHashByExecutionHash), h)
}

// GetReceiptByExecutionHash mocks base method
func (m *MockBlockchain) GetReceiptByExecutionHash(h hash.Hash32B) (*blockchain.Receipt, error) {
	ret := m.ctrl.Call(m, "GetReceiptByExecutionHash", h)
	ret0, _ := ret[0].(*blockchain.Receipt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetReceiptByExecutionHash indicates an expected call of GetReceiptByExecutionHash
func (mr *MockBlockchainMockRecorder) GetReceiptByExecutionHash(h interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetReceiptByExecutionHash", reflect.TypeOf((*MockBlockchain)(nil).GetReceiptByExecutionHash), h)
}

// GetFactory mocks base method
func (m *MockBlockchain) GetFactory() state.Factory {
	ret := m.ctrl.Call(m, "GetFactory")
	ret0, _ := ret[0].(state.Factory)
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

// TipHash mocks base method
func (m *MockBlockchain) TipHash() hash.Hash32B {
	ret := m.ctrl.Call(m, "TipHash")
	ret0, _ := ret[0].(hash.Hash32B)
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
func (m *MockBlockchain) StateByAddr(address string) (*state.State, error) {
	ret := m.ctrl.Call(m, "StateByAddr", address)
	ret0, _ := ret[0].(*state.State)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StateByAddr indicates an expected call of StateByAddr
func (mr *MockBlockchainMockRecorder) StateByAddr(address interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StateByAddr", reflect.TypeOf((*MockBlockchain)(nil).StateByAddr), address)
}

// MintNewBlock mocks base method
func (m *MockBlockchain) MintNewBlock(tsf []*action.Transfer, vote []*action.Vote, executions []*action.Execution, address *iotxaddress.Address, data string) (*blockchain.Block, error) {
	ret := m.ctrl.Call(m, "MintNewBlock", tsf, vote, executions, address, data)
	ret0, _ := ret[0].(*blockchain.Block)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MintNewBlock indicates an expected call of MintNewBlock
func (mr *MockBlockchainMockRecorder) MintNewBlock(tsf, vote, executions, address, data interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MintNewBlock", reflect.TypeOf((*MockBlockchain)(nil).MintNewBlock), tsf, vote, executions, address, data)
}

// MintNewDKGBlock mocks base method
func (m *MockBlockchain) MintNewDKGBlock(tsf []*action.Transfer, vote []*action.Vote, executions []*action.Execution, producer *iotxaddress.Address, dkgAddress *iotxaddress.DKGAddress, seed []byte, data string) (*blockchain.Block, error) {
	ret := m.ctrl.Call(m, "MintNewDKGBlock", tsf, vote, executions, producer, dkgAddress, seed, data)
	ret0, _ := ret[0].(*blockchain.Block)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MintNewDKGBlock indicates an expected call of MintNewDKGBlock
func (mr *MockBlockchainMockRecorder) MintNewDKGBlock(tsf, vote, executions, producer, dkgAddress, seed, data interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MintNewDKGBlock", reflect.TypeOf((*MockBlockchain)(nil).MintNewDKGBlock), tsf, vote, executions, producer, dkgAddress, seed, data)
}

// MintNewSecretBlock mocks base method
func (m *MockBlockchain) MintNewSecretBlock(secretProposals []*action.SecretProposal, secretWitness *action.SecretWitness, producer *iotxaddress.Address) (*blockchain.Block, error) {
	ret := m.ctrl.Call(m, "MintNewSecretBlock", secretProposals, secretWitness, producer)
	ret0, _ := ret[0].(*blockchain.Block)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MintNewSecretBlock indicates an expected call of MintNewSecretBlock
func (mr *MockBlockchainMockRecorder) MintNewSecretBlock(secretProposals, secretWitness, producer interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MintNewSecretBlock", reflect.TypeOf((*MockBlockchain)(nil).MintNewSecretBlock), secretProposals, secretWitness, producer)
}

// MintNewDummyBlock mocks base method
func (m *MockBlockchain) MintNewDummyBlock() *blockchain.Block {
	ret := m.ctrl.Call(m, "MintNewDummyBlock")
	ret0, _ := ret[0].(*blockchain.Block)
	return ret0
}

// MintNewDummyBlock indicates an expected call of MintNewDummyBlock
func (mr *MockBlockchainMockRecorder) MintNewDummyBlock() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MintNewDummyBlock", reflect.TypeOf((*MockBlockchain)(nil).MintNewDummyBlock))
}

// CommitBlock mocks base method
func (m *MockBlockchain) CommitBlock(blk *blockchain.Block) error {
	ret := m.ctrl.Call(m, "CommitBlock", blk)
	ret0, _ := ret[0].(error)
	return ret0
}

// CommitBlock indicates an expected call of CommitBlock
func (mr *MockBlockchainMockRecorder) CommitBlock(blk interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommitBlock", reflect.TypeOf((*MockBlockchain)(nil).CommitBlock), blk)
}

// ValidateBlock mocks base method
func (m *MockBlockchain) ValidateBlock(blk *blockchain.Block) error {
	ret := m.ctrl.Call(m, "ValidateBlock", blk)
	ret0, _ := ret[0].(error)
	return ret0
}

// ValidateBlock indicates an expected call of ValidateBlock
func (mr *MockBlockchainMockRecorder) ValidateBlock(blk interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidateBlock", reflect.TypeOf((*MockBlockchain)(nil).ValidateBlock), blk)
}

// ValidateSecretBlock mocks base method
func (m *MockBlockchain) ValidateSecretBlock(blk *blockchain.Block) error {
	ret := m.ctrl.Call(m, "ValidateSecretBlock", blk)
	ret0, _ := ret[0].(error)
	return ret0
}

// ValidateSecretBlock indicates an expected call of ValidateSecretBlock
func (mr *MockBlockchainMockRecorder) ValidateSecretBlock(blk interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidateSecretBlock", reflect.TypeOf((*MockBlockchain)(nil).ValidateSecretBlock), blk)
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

// SecretValidator mocks base method
func (m *MockBlockchain) SecretValidator() blockchain.Validator {
	ret := m.ctrl.Call(m, "SecretValidator")
	ret0, _ := ret[0].(blockchain.Validator)
	return ret0
}

// SecretValidator indicates an expected call of SecretValidator
func (mr *MockBlockchainMockRecorder) SecretValidator() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SecretValidator", reflect.TypeOf((*MockBlockchain)(nil).SecretValidator))
}

// SetSecretValidator mocks base method
func (m *MockBlockchain) SetSecretValidator(val blockchain.Validator) {
	m.ctrl.Call(m, "SetSecretValidator", val)
}

// SetSecretValidator indicates an expected call of SetSecretValidator
func (mr *MockBlockchainMockRecorder) SetSecretValidator(val interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetSecretValidator", reflect.TypeOf((*MockBlockchain)(nil).SetSecretValidator), val)
}

// ExecuteContractRead mocks base method
func (m *MockBlockchain) ExecuteContractRead(arg0 *action.Execution) ([]byte, error) {
	ret := m.ctrl.Call(m, "ExecuteContractRead", arg0)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ExecuteContractRead indicates an expected call of ExecuteContractRead
func (mr *MockBlockchainMockRecorder) ExecuteContractRead(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExecuteContractRead", reflect.TypeOf((*MockBlockchain)(nil).ExecuteContractRead), arg0)
}
