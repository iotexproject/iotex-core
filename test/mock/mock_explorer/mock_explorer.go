// Code generated by MockGen. DO NOT EDIT.
// Source: ./explorer/idl/explorer/explorer.go

// Package mock_explorer is a generated GoMock package.
package mock_explorer

import (
	gomock "github.com/golang/mock/gomock"
	explorer "github.com/iotexproject/iotex-core/explorer/idl/explorer"
	reflect "reflect"
)

// MockExplorer is a mock of Explorer interface
type MockExplorer struct {
	ctrl     *gomock.Controller
	recorder *MockExplorerMockRecorder
}

// MockExplorerMockRecorder is the mock recorder for MockExplorer
type MockExplorerMockRecorder struct {
	mock *MockExplorer
}

// NewMockExplorer creates a new mock instance
func NewMockExplorer(ctrl *gomock.Controller) *MockExplorer {
	mock := &MockExplorer{ctrl: ctrl}
	mock.recorder = &MockExplorerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockExplorer) EXPECT() *MockExplorerMockRecorder {
	return m.recorder
}

// GetBlockchainHeight mocks base method
func (m *MockExplorer) GetBlockchainHeight() (int64, error) {
	ret := m.ctrl.Call(m, "GetBlockchainHeight")
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlockchainHeight indicates an expected call of GetBlockchainHeight
func (mr *MockExplorerMockRecorder) GetBlockchainHeight() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockchainHeight", reflect.TypeOf((*MockExplorer)(nil).GetBlockchainHeight))
}

// GetAddressBalance mocks base method
func (m *MockExplorer) GetAddressBalance(address string) (string, error) {
	ret := m.ctrl.Call(m, "GetAddressBalance", address)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAddressBalance indicates an expected call of GetAddressBalance
func (mr *MockExplorerMockRecorder) GetAddressBalance(address interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAddressBalance", reflect.TypeOf((*MockExplorer)(nil).GetAddressBalance), address)
}

// GetAddressDetails mocks base method
func (m *MockExplorer) GetAddressDetails(address string) (explorer.AddressDetails, error) {
	ret := m.ctrl.Call(m, "GetAddressDetails", address)
	ret0, _ := ret[0].(explorer.AddressDetails)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAddressDetails indicates an expected call of GetAddressDetails
func (mr *MockExplorerMockRecorder) GetAddressDetails(address interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAddressDetails", reflect.TypeOf((*MockExplorer)(nil).GetAddressDetails), address)
}

// GetLastTransfersByRange mocks base method
func (m *MockExplorer) GetLastTransfersByRange(startBlockHeight, offset, limit int64, showCoinBase bool) ([]explorer.Transfer, error) {
	ret := m.ctrl.Call(m, "GetLastTransfersByRange", startBlockHeight, offset, limit, showCoinBase)
	ret0, _ := ret[0].([]explorer.Transfer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLastTransfersByRange indicates an expected call of GetLastTransfersByRange
func (mr *MockExplorerMockRecorder) GetLastTransfersByRange(startBlockHeight, offset, limit, showCoinBase interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLastTransfersByRange", reflect.TypeOf((*MockExplorer)(nil).GetLastTransfersByRange), startBlockHeight, offset, limit, showCoinBase)
}

// GetTransferByID mocks base method
func (m *MockExplorer) GetTransferByID(transferID string) (explorer.Transfer, error) {
	ret := m.ctrl.Call(m, "GetTransferByID", transferID)
	ret0, _ := ret[0].(explorer.Transfer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTransferByID indicates an expected call of GetTransferByID
func (mr *MockExplorerMockRecorder) GetTransferByID(transferID interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTransferByID", reflect.TypeOf((*MockExplorer)(nil).GetTransferByID), transferID)
}

// GetTransfersByAddress mocks base method
func (m *MockExplorer) GetTransfersByAddress(address string, offset, limit int64) ([]explorer.Transfer, error) {
	ret := m.ctrl.Call(m, "GetTransfersByAddress", address, offset, limit)
	ret0, _ := ret[0].([]explorer.Transfer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTransfersByAddress indicates an expected call of GetTransfersByAddress
func (mr *MockExplorerMockRecorder) GetTransfersByAddress(address, offset, limit interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTransfersByAddress", reflect.TypeOf((*MockExplorer)(nil).GetTransfersByAddress), address, offset, limit)
}

// GetUnconfirmedTransfersByAddress mocks base method
func (m *MockExplorer) GetUnconfirmedTransfersByAddress(address string, offset, limit int64) ([]explorer.Transfer, error) {
	ret := m.ctrl.Call(m, "GetUnconfirmedTransfersByAddress", address, offset, limit)
	ret0, _ := ret[0].([]explorer.Transfer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUnconfirmedTransfersByAddress indicates an expected call of GetUnconfirmedTransfersByAddress
func (mr *MockExplorerMockRecorder) GetUnconfirmedTransfersByAddress(address, offset, limit interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUnconfirmedTransfersByAddress", reflect.TypeOf((*MockExplorer)(nil).GetUnconfirmedTransfersByAddress), address, offset, limit)
}

// GetTransfersByBlockID mocks base method
func (m *MockExplorer) GetTransfersByBlockID(blkID string, offset, limit int64) ([]explorer.Transfer, error) {
	ret := m.ctrl.Call(m, "GetTransfersByBlockID", blkID, offset, limit)
	ret0, _ := ret[0].([]explorer.Transfer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTransfersByBlockID indicates an expected call of GetTransfersByBlockID
func (mr *MockExplorerMockRecorder) GetTransfersByBlockID(blkID, offset, limit interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTransfersByBlockID", reflect.TypeOf((*MockExplorer)(nil).GetTransfersByBlockID), blkID, offset, limit)
}

// GetLastVotesByRange mocks base method
func (m *MockExplorer) GetLastVotesByRange(startBlockHeight, offset, limit int64) ([]explorer.Vote, error) {
	ret := m.ctrl.Call(m, "GetLastVotesByRange", startBlockHeight, offset, limit)
	ret0, _ := ret[0].([]explorer.Vote)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLastVotesByRange indicates an expected call of GetLastVotesByRange
func (mr *MockExplorerMockRecorder) GetLastVotesByRange(startBlockHeight, offset, limit interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLastVotesByRange", reflect.TypeOf((*MockExplorer)(nil).GetLastVotesByRange), startBlockHeight, offset, limit)
}

// GetVoteByID mocks base method
func (m *MockExplorer) GetVoteByID(voteID string) (explorer.Vote, error) {
	ret := m.ctrl.Call(m, "GetVoteByID", voteID)
	ret0, _ := ret[0].(explorer.Vote)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetVoteByID indicates an expected call of GetVoteByID
func (mr *MockExplorerMockRecorder) GetVoteByID(voteID interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVoteByID", reflect.TypeOf((*MockExplorer)(nil).GetVoteByID), voteID)
}

// GetVotesByAddress mocks base method
func (m *MockExplorer) GetVotesByAddress(address string, offset, limit int64) ([]explorer.Vote, error) {
	ret := m.ctrl.Call(m, "GetVotesByAddress", address, offset, limit)
	ret0, _ := ret[0].([]explorer.Vote)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetVotesByAddress indicates an expected call of GetVotesByAddress
func (mr *MockExplorerMockRecorder) GetVotesByAddress(address, offset, limit interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVotesByAddress", reflect.TypeOf((*MockExplorer)(nil).GetVotesByAddress), address, offset, limit)
}

// GetUnconfirmedVotesByAddress mocks base method
func (m *MockExplorer) GetUnconfirmedVotesByAddress(address string, offset, limit int64) ([]explorer.Vote, error) {
	ret := m.ctrl.Call(m, "GetUnconfirmedVotesByAddress", address, offset, limit)
	ret0, _ := ret[0].([]explorer.Vote)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUnconfirmedVotesByAddress indicates an expected call of GetUnconfirmedVotesByAddress
func (mr *MockExplorerMockRecorder) GetUnconfirmedVotesByAddress(address, offset, limit interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUnconfirmedVotesByAddress", reflect.TypeOf((*MockExplorer)(nil).GetUnconfirmedVotesByAddress), address, offset, limit)
}

// GetVotesByBlockID mocks base method
func (m *MockExplorer) GetVotesByBlockID(blkID string, offset, limit int64) ([]explorer.Vote, error) {
	ret := m.ctrl.Call(m, "GetVotesByBlockID", blkID, offset, limit)
	ret0, _ := ret[0].([]explorer.Vote)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetVotesByBlockID indicates an expected call of GetVotesByBlockID
func (mr *MockExplorerMockRecorder) GetVotesByBlockID(blkID, offset, limit interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVotesByBlockID", reflect.TypeOf((*MockExplorer)(nil).GetVotesByBlockID), blkID, offset, limit)
}

// GetLastExecutionsByRange mocks base method
func (m *MockExplorer) GetLastExecutionsByRange(startBlockHeight, offset, limit int64) ([]explorer.Execution, error) {
	ret := m.ctrl.Call(m, "GetLastExecutionsByRange", startBlockHeight, offset, limit)
	ret0, _ := ret[0].([]explorer.Execution)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLastExecutionsByRange indicates an expected call of GetLastExecutionsByRange
func (mr *MockExplorerMockRecorder) GetLastExecutionsByRange(startBlockHeight, offset, limit interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLastExecutionsByRange", reflect.TypeOf((*MockExplorer)(nil).GetLastExecutionsByRange), startBlockHeight, offset, limit)
}

// GetExecutionByID mocks base method
func (m *MockExplorer) GetExecutionByID(executionID string) (explorer.Execution, error) {
	ret := m.ctrl.Call(m, "GetExecutionByID", executionID)
	ret0, _ := ret[0].(explorer.Execution)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetExecutionByID indicates an expected call of GetExecutionByID
func (mr *MockExplorerMockRecorder) GetExecutionByID(executionID interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetExecutionByID", reflect.TypeOf((*MockExplorer)(nil).GetExecutionByID), executionID)
}

// GetExecutionsByAddress mocks base method
func (m *MockExplorer) GetExecutionsByAddress(address string, offset, limit int64) ([]explorer.Execution, error) {
	ret := m.ctrl.Call(m, "GetExecutionsByAddress", address, offset, limit)
	ret0, _ := ret[0].([]explorer.Execution)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetExecutionsByAddress indicates an expected call of GetExecutionsByAddress
func (mr *MockExplorerMockRecorder) GetExecutionsByAddress(address, offset, limit interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetExecutionsByAddress", reflect.TypeOf((*MockExplorer)(nil).GetExecutionsByAddress), address, offset, limit)
}

// GetUnconfirmedExecutionsByAddress mocks base method
func (m *MockExplorer) GetUnconfirmedExecutionsByAddress(address string, offset, limit int64) ([]explorer.Execution, error) {
	ret := m.ctrl.Call(m, "GetUnconfirmedExecutionsByAddress", address, offset, limit)
	ret0, _ := ret[0].([]explorer.Execution)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUnconfirmedExecutionsByAddress indicates an expected call of GetUnconfirmedExecutionsByAddress
func (mr *MockExplorerMockRecorder) GetUnconfirmedExecutionsByAddress(address, offset, limit interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUnconfirmedExecutionsByAddress", reflect.TypeOf((*MockExplorer)(nil).GetUnconfirmedExecutionsByAddress), address, offset, limit)
}

// GetExecutionsByBlockID mocks base method
func (m *MockExplorer) GetExecutionsByBlockID(blkID string, offset, limit int64) ([]explorer.Execution, error) {
	ret := m.ctrl.Call(m, "GetExecutionsByBlockID", blkID, offset, limit)
	ret0, _ := ret[0].([]explorer.Execution)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetExecutionsByBlockID indicates an expected call of GetExecutionsByBlockID
func (mr *MockExplorerMockRecorder) GetExecutionsByBlockID(blkID, offset, limit interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetExecutionsByBlockID", reflect.TypeOf((*MockExplorer)(nil).GetExecutionsByBlockID), blkID, offset, limit)
}

// GetLastBlocksByRange mocks base method
func (m *MockExplorer) GetLastBlocksByRange(offset, limit int64) ([]explorer.Block, error) {
	ret := m.ctrl.Call(m, "GetLastBlocksByRange", offset, limit)
	ret0, _ := ret[0].([]explorer.Block)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLastBlocksByRange indicates an expected call of GetLastBlocksByRange
func (mr *MockExplorerMockRecorder) GetLastBlocksByRange(offset, limit interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLastBlocksByRange", reflect.TypeOf((*MockExplorer)(nil).GetLastBlocksByRange), offset, limit)
}

// GetBlockByID mocks base method
func (m *MockExplorer) GetBlockByID(blkID string) (explorer.Block, error) {
	ret := m.ctrl.Call(m, "GetBlockByID", blkID)
	ret0, _ := ret[0].(explorer.Block)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlockByID indicates an expected call of GetBlockByID
func (mr *MockExplorerMockRecorder) GetBlockByID(blkID interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockByID", reflect.TypeOf((*MockExplorer)(nil).GetBlockByID), blkID)
}

// GetCoinStatistic mocks base method
func (m *MockExplorer) GetCoinStatistic() (explorer.CoinStatistic, error) {
	ret := m.ctrl.Call(m, "GetCoinStatistic")
	ret0, _ := ret[0].(explorer.CoinStatistic)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCoinStatistic indicates an expected call of GetCoinStatistic
func (mr *MockExplorerMockRecorder) GetCoinStatistic() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCoinStatistic", reflect.TypeOf((*MockExplorer)(nil).GetCoinStatistic))
}

// GetConsensusMetrics mocks base method
func (m *MockExplorer) GetConsensusMetrics() (explorer.ConsensusMetrics, error) {
	ret := m.ctrl.Call(m, "GetConsensusMetrics")
	ret0, _ := ret[0].(explorer.ConsensusMetrics)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetConsensusMetrics indicates an expected call of GetConsensusMetrics
func (mr *MockExplorerMockRecorder) GetConsensusMetrics() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetConsensusMetrics", reflect.TypeOf((*MockExplorer)(nil).GetConsensusMetrics))
}

// GetCandidateMetrics mocks base method
func (m *MockExplorer) GetCandidateMetrics() (explorer.CandidateMetrics, error) {
	ret := m.ctrl.Call(m, "GetCandidateMetrics")
	ret0, _ := ret[0].(explorer.CandidateMetrics)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCandidateMetrics indicates an expected call of GetCandidateMetrics
func (mr *MockExplorerMockRecorder) GetCandidateMetrics() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCandidateMetrics", reflect.TypeOf((*MockExplorer)(nil).GetCandidateMetrics))
}

// GetCandidateMetricsByHeight mocks base method
func (m *MockExplorer) GetCandidateMetricsByHeight(h int64) (explorer.CandidateMetrics, error) {
	ret := m.ctrl.Call(m, "GetCandidateMetricsByHeight", h)
	ret0, _ := ret[0].(explorer.CandidateMetrics)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCandidateMetricsByHeight indicates an expected call of GetCandidateMetricsByHeight
func (mr *MockExplorerMockRecorder) GetCandidateMetricsByHeight(h interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCandidateMetricsByHeight", reflect.TypeOf((*MockExplorer)(nil).GetCandidateMetricsByHeight), h)
}

// SendTransfer mocks base method
func (m *MockExplorer) SendTransfer(request explorer.SendTransferRequest) (explorer.SendTransferResponse, error) {
	ret := m.ctrl.Call(m, "SendTransfer", request)
	ret0, _ := ret[0].(explorer.SendTransferResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SendTransfer indicates an expected call of SendTransfer
func (mr *MockExplorerMockRecorder) SendTransfer(request interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendTransfer", reflect.TypeOf((*MockExplorer)(nil).SendTransfer), request)
}

// SendVote mocks base method
func (m *MockExplorer) SendVote(request explorer.SendVoteRequest) (explorer.SendVoteResponse, error) {
	ret := m.ctrl.Call(m, "SendVote", request)
	ret0, _ := ret[0].(explorer.SendVoteResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SendVote indicates an expected call of SendVote
func (mr *MockExplorerMockRecorder) SendVote(request interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendVote", reflect.TypeOf((*MockExplorer)(nil).SendVote), request)
}

// SendSmartContract mocks base method
func (m *MockExplorer) SendSmartContract(request explorer.Execution) (explorer.SendSmartContractResponse, error) {
	ret := m.ctrl.Call(m, "SendSmartContract", request)
	ret0, _ := ret[0].(explorer.SendSmartContractResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SendSmartContract indicates an expected call of SendSmartContract
func (mr *MockExplorerMockRecorder) SendSmartContract(request interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendSmartContract", reflect.TypeOf((*MockExplorer)(nil).SendSmartContract), request)
}

// PutSubChainBlock mocks base method
func (m *MockExplorer) PutSubChainBlock(request explorer.PutSubChainBlockRequest) (explorer.PutSubChainBlockResponse, error) {
	ret := m.ctrl.Call(m, "PutSubChainBlock", request)
	ret0, _ := ret[0].(explorer.PutSubChainBlockResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PutSubChainBlock indicates an expected call of PutSubChainBlock
func (mr *MockExplorerMockRecorder) PutSubChainBlock(request interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PutSubChainBlock", reflect.TypeOf((*MockExplorer)(nil).PutSubChainBlock), request)
}

// SendAction mocks base method
func (m *MockExplorer) SendAction(request explorer.SendActionRequest) (explorer.SendActionResponse, error) {
	ret := m.ctrl.Call(m, "SendAction", request)
	ret0, _ := ret[0].(explorer.SendActionResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SendAction indicates an expected call of SendAction
func (mr *MockExplorerMockRecorder) SendAction(request interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendAction", reflect.TypeOf((*MockExplorer)(nil).SendAction), request)
}

// GetPeers mocks base method
func (m *MockExplorer) GetPeers() (explorer.GetPeersResponse, error) {
	ret := m.ctrl.Call(m, "GetPeers")
	ret0, _ := ret[0].(explorer.GetPeersResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPeers indicates an expected call of GetPeers
func (mr *MockExplorerMockRecorder) GetPeers() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPeers", reflect.TypeOf((*MockExplorer)(nil).GetPeers))
}

// GetReceiptByExecutionID mocks base method
func (m *MockExplorer) GetReceiptByExecutionID(id string) (explorer.Receipt, error) {
	ret := m.ctrl.Call(m, "GetReceiptByExecutionID", id)
	ret0, _ := ret[0].(explorer.Receipt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetReceiptByExecutionID indicates an expected call of GetReceiptByExecutionID
func (mr *MockExplorerMockRecorder) GetReceiptByExecutionID(id interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetReceiptByExecutionID", reflect.TypeOf((*MockExplorer)(nil).GetReceiptByExecutionID), id)
}

// ReadExecutionState mocks base method
func (m *MockExplorer) ReadExecutionState(request explorer.Execution) (string, error) {
	ret := m.ctrl.Call(m, "ReadExecutionState", request)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadExecutionState indicates an expected call of ReadExecutionState
func (mr *MockExplorerMockRecorder) ReadExecutionState(request interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadExecutionState", reflect.TypeOf((*MockExplorer)(nil).ReadExecutionState), request)
}

// GetBlockOrActionByHash mocks base method
func (m *MockExplorer) GetBlockOrActionByHash(hashStr string) (explorer.GetBlkOrActResponse, error) {
	ret := m.ctrl.Call(m, "GetBlockOrActionByHash", hashStr)
	ret0, _ := ret[0].(explorer.GetBlkOrActResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlockOrActionByHash indicates an expected call of GetBlockOrActionByHash
func (mr *MockExplorerMockRecorder) GetBlockOrActionByHash(hashStr interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockOrActionByHash", reflect.TypeOf((*MockExplorer)(nil).GetBlockOrActionByHash), hashStr)
}

// CreateDeposit mocks base method
func (m *MockExplorer) CreateDeposit(request explorer.CreateDepositRequest) (explorer.CreateDepositResponse, error) {
	ret := m.ctrl.Call(m, "CreateDeposit", request)
	ret0, _ := ret[0].(explorer.CreateDepositResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateDeposit indicates an expected call of CreateDeposit
func (mr *MockExplorerMockRecorder) CreateDeposit(request interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateDeposit", reflect.TypeOf((*MockExplorer)(nil).CreateDeposit), request)
}

// GetDeposits mocks base method
func (m *MockExplorer) GetDeposits(subChainID, offset, limit int64) ([]explorer.Deposit, error) {
	ret := m.ctrl.Call(m, "GetDeposits", subChainID, offset, limit)
	ret0, _ := ret[0].([]explorer.Deposit)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDeposits indicates an expected call of GetDeposits
func (mr *MockExplorerMockRecorder) GetDeposits(subChainID, offset, limit interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDeposits", reflect.TypeOf((*MockExplorer)(nil).GetDeposits), subChainID, offset, limit)
}

// SettleDeposit mocks base method
func (m *MockExplorer) SettleDeposit(request explorer.SettleDepositRequest) (explorer.SettleDepositResponse, error) {
	ret := m.ctrl.Call(m, "SettleDeposit", request)
	ret0, _ := ret[0].(explorer.SettleDepositResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SettleDeposit indicates an expected call of SettleDeposit
func (mr *MockExplorerMockRecorder) SettleDeposit(request interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SettleDeposit", reflect.TypeOf((*MockExplorer)(nil).SettleDeposit), request)
}

// SuggestGasPrice mocks base method
func (m *MockExplorer) SuggestGasPrice() (int64, error) {
	ret := m.ctrl.Call(m, "SuggestGasPrice")
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SuggestGasPrice indicates an expected call of SuggestGasPrice
func (mr *MockExplorerMockRecorder) SuggestGasPrice() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SuggestGasPrice", reflect.TypeOf((*MockExplorer)(nil).Deposit))
}

// EstimateGasForTransfer mocks base method
func (m *MockExplorer) EstimateGasForTransfer(request explorer.SendTransferRequest) (int64, error) {
	ret := m.ctrl.Call(m, "EstimateGasForTransfer", request)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// EstimateGasForTransfer indicates an expected call of EstimateGasForTransfer
func (mr *MockExplorerMockRecorder) EstimateGasForTransfer(request interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EstimateGasForTransfer", reflect.TypeOf((*MockExplorer)(nil).EstimateGasForTransfer), request)
}

// EstimateGasForVote mocks base method
func (m *MockExplorer) EstimateGasForVote(request explorer.SendVoteRequest) (int64, error) {
	ret := m.ctrl.Call(m, "EstimateGasForVote", request)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// EstimateGasForVote indicates an expected call of EstimateGasForVote
func (mr *MockExplorerMockRecorder) EstimateGasForVote(request interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EstimateGasForVote", reflect.TypeOf((*MockExplorer)(nil).EstimateGasForVote), request)
}

// EstimateGasForSmartContract mocks base method EstimateGasForSmartContract
func (m *MockExplorer) EstimateGasForSmartContract(request explorer.Execution) (int64, error) {
	ret := m.ctrl.Call(m, "EstimateGasForSmartContract", request)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// EstimateGasForSmartContract indicates an expected call of EstimateGasForSmartContract
func (mr *MockExplorerMockRecorder) EstimateGasForSmartContract(request interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EstimateGasForSmartContract", reflect.TypeOf((*MockExplorer)(nil).EstimateGasForSmartContract), request)
}
