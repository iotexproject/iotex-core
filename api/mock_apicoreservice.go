// Code generated by MockGen. DO NOT EDIT.
// Source: ./api/coreservice.go
//
// Generated by this command:
//
//	mockgen -destination=./api/mock_apicoreservice.go -source=./api/coreservice.go -package=api CoreService
//

// Package api is a generated GoMock package.
package api

import (
	context "context"
	big "math/big"
	reflect "reflect"
	time "time"

	tracers "github.com/ethereum/go-ethereum/eth/tracers"
	hash "github.com/iotexproject/go-pkgs/hash"
	address "github.com/iotexproject/iotex-address/address"
	action "github.com/iotexproject/iotex-core/v2/action"
	protocol "github.com/iotexproject/iotex-core/v2/action/protocol"
	logfilter "github.com/iotexproject/iotex-core/v2/api/logfilter"
	apitypes "github.com/iotexproject/iotex-core/v2/api/types"
	block "github.com/iotexproject/iotex-core/v2/blockchain/block"
	genesis "github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	iotexapi "github.com/iotexproject/iotex-proto/golang/iotexapi"
	iotextypes "github.com/iotexproject/iotex-proto/golang/iotextypes"
	gomock "go.uber.org/mock/gomock"
)

// MockCoreService is a mock of CoreService interface.
type MockCoreService struct {
	ctrl     *gomock.Controller
	recorder *MockCoreServiceMockRecorder
	isgomock struct{}
}

// MockCoreServiceMockRecorder is the mock recorder for MockCoreService.
type MockCoreServiceMockRecorder struct {
	mock *MockCoreService
}

// NewMockCoreService creates a new mock instance.
func NewMockCoreService(ctrl *gomock.Controller) *MockCoreService {
	mock := &MockCoreService{ctrl: ctrl}
	mock.recorder = &MockCoreServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCoreService) EXPECT() *MockCoreServiceMockRecorder {
	return m.recorder
}

// Account mocks base method.
func (m *MockCoreService) Account(addr address.Address) (*iotextypes.AccountMeta, *iotextypes.BlockIdentifier, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Account", addr)
	ret0, _ := ret[0].(*iotextypes.AccountMeta)
	ret1, _ := ret[1].(*iotextypes.BlockIdentifier)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Account indicates an expected call of Account.
func (mr *MockCoreServiceMockRecorder) Account(addr any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Account", reflect.TypeOf((*MockCoreService)(nil).Account), addr)
}

// Action mocks base method.
func (m *MockCoreService) Action(actionHash string, checkPending bool) (*iotexapi.ActionInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Action", actionHash, checkPending)
	ret0, _ := ret[0].(*iotexapi.ActionInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Action indicates an expected call of Action.
func (mr *MockCoreServiceMockRecorder) Action(actionHash, checkPending any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Action", reflect.TypeOf((*MockCoreService)(nil).Action), actionHash, checkPending)
}

// ActionByActionHash mocks base method.
func (m *MockCoreService) ActionByActionHash(h hash.Hash256) (*action.SealedEnvelope, *block.Block, uint32, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ActionByActionHash", h)
	ret0, _ := ret[0].(*action.SealedEnvelope)
	ret1, _ := ret[1].(*block.Block)
	ret2, _ := ret[2].(uint32)
	ret3, _ := ret[3].(error)
	return ret0, ret1, ret2, ret3
}

// ActionByActionHash indicates an expected call of ActionByActionHash.
func (mr *MockCoreServiceMockRecorder) ActionByActionHash(h any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ActionByActionHash", reflect.TypeOf((*MockCoreService)(nil).ActionByActionHash), h)
}

// Actions mocks base method.
func (m *MockCoreService) Actions(start, count uint64) ([]*iotexapi.ActionInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Actions", start, count)
	ret0, _ := ret[0].([]*iotexapi.ActionInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Actions indicates an expected call of Actions.
func (mr *MockCoreServiceMockRecorder) Actions(start, count any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Actions", reflect.TypeOf((*MockCoreService)(nil).Actions), start, count)
}

// ActionsByAddress mocks base method.
func (m *MockCoreService) ActionsByAddress(addr address.Address, start, count uint64) ([]*iotexapi.ActionInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ActionsByAddress", addr, start, count)
	ret0, _ := ret[0].([]*iotexapi.ActionInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ActionsByAddress indicates an expected call of ActionsByAddress.
func (mr *MockCoreServiceMockRecorder) ActionsByAddress(addr, start, count any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ActionsByAddress", reflect.TypeOf((*MockCoreService)(nil).ActionsByAddress), addr, start, count)
}

// ActionsInActPool mocks base method.
func (m *MockCoreService) ActionsInActPool(actHashes []string) ([]*action.SealedEnvelope, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ActionsInActPool", actHashes)
	ret0, _ := ret[0].([]*action.SealedEnvelope)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ActionsInActPool indicates an expected call of ActionsInActPool.
func (mr *MockCoreServiceMockRecorder) ActionsInActPool(actHashes any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ActionsInActPool", reflect.TypeOf((*MockCoreService)(nil).ActionsInActPool), actHashes)
}

// BlobSidecarsByHeight mocks base method.
func (m *MockCoreService) BlobSidecarsByHeight(height uint64) ([]*apitypes.BlobSidecarResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobSidecarsByHeight", height)
	ret0, _ := ret[0].([]*apitypes.BlobSidecarResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// BlobSidecarsByHeight indicates an expected call of BlobSidecarsByHeight.
func (mr *MockCoreServiceMockRecorder) BlobSidecarsByHeight(height any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobSidecarsByHeight", reflect.TypeOf((*MockCoreService)(nil).BlobSidecarsByHeight), height)
}

// BlockByHash mocks base method.
func (m *MockCoreService) BlockByHash(arg0 string) (*apitypes.BlockWithReceipts, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlockByHash", arg0)
	ret0, _ := ret[0].(*apitypes.BlockWithReceipts)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// BlockByHash indicates an expected call of BlockByHash.
func (mr *MockCoreServiceMockRecorder) BlockByHash(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlockByHash", reflect.TypeOf((*MockCoreService)(nil).BlockByHash), arg0)
}

// BlockByHeight mocks base method.
func (m *MockCoreService) BlockByHeight(arg0 uint64) (*apitypes.BlockWithReceipts, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlockByHeight", arg0)
	ret0, _ := ret[0].(*apitypes.BlockWithReceipts)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// BlockByHeight indicates an expected call of BlockByHeight.
func (mr *MockCoreServiceMockRecorder) BlockByHeight(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlockByHeight", reflect.TypeOf((*MockCoreService)(nil).BlockByHeight), arg0)
}

// BlockByHeightRange mocks base method.
func (m *MockCoreService) BlockByHeightRange(arg0, arg1 uint64) ([]*apitypes.BlockWithReceipts, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlockByHeightRange", arg0, arg1)
	ret0, _ := ret[0].([]*apitypes.BlockWithReceipts)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// BlockByHeightRange indicates an expected call of BlockByHeightRange.
func (mr *MockCoreServiceMockRecorder) BlockByHeightRange(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlockByHeightRange", reflect.TypeOf((*MockCoreService)(nil).BlockByHeightRange), arg0, arg1)
}

// BlockHashByBlockHeight mocks base method.
func (m *MockCoreService) BlockHashByBlockHeight(blkHeight uint64) (hash.Hash256, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlockHashByBlockHeight", blkHeight)
	ret0, _ := ret[0].(hash.Hash256)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// BlockHashByBlockHeight indicates an expected call of BlockHashByBlockHeight.
func (mr *MockCoreServiceMockRecorder) BlockHashByBlockHeight(blkHeight any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlockHashByBlockHeight", reflect.TypeOf((*MockCoreService)(nil).BlockHashByBlockHeight), blkHeight)
}

// ChainID mocks base method.
func (m *MockCoreService) ChainID() uint32 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ChainID")
	ret0, _ := ret[0].(uint32)
	return ret0
}

// ChainID indicates an expected call of ChainID.
func (mr *MockCoreServiceMockRecorder) ChainID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ChainID", reflect.TypeOf((*MockCoreService)(nil).ChainID))
}

// ChainListener mocks base method.
func (m *MockCoreService) ChainListener() apitypes.Listener {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ChainListener")
	ret0, _ := ret[0].(apitypes.Listener)
	return ret0
}

// ChainListener indicates an expected call of ChainListener.
func (mr *MockCoreServiceMockRecorder) ChainListener() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ChainListener", reflect.TypeOf((*MockCoreService)(nil).ChainListener))
}

// ChainMeta mocks base method.
func (m *MockCoreService) ChainMeta() (*iotextypes.ChainMeta, string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ChainMeta")
	ret0, _ := ret[0].(*iotextypes.ChainMeta)
	ret1, _ := ret[1].(string)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ChainMeta indicates an expected call of ChainMeta.
func (mr *MockCoreServiceMockRecorder) ChainMeta() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ChainMeta", reflect.TypeOf((*MockCoreService)(nil).ChainMeta))
}

// EVMNetworkID mocks base method.
func (m *MockCoreService) EVMNetworkID() uint32 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EVMNetworkID")
	ret0, _ := ret[0].(uint32)
	return ret0
}

// EVMNetworkID indicates an expected call of EVMNetworkID.
func (mr *MockCoreServiceMockRecorder) EVMNetworkID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EVMNetworkID", reflect.TypeOf((*MockCoreService)(nil).EVMNetworkID))
}

// ElectionBuckets mocks base method.
func (m *MockCoreService) ElectionBuckets(epochNum uint64) ([]*iotextypes.ElectionBucket, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ElectionBuckets", epochNum)
	ret0, _ := ret[0].([]*iotextypes.ElectionBucket)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ElectionBuckets indicates an expected call of ElectionBuckets.
func (mr *MockCoreServiceMockRecorder) ElectionBuckets(epochNum any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ElectionBuckets", reflect.TypeOf((*MockCoreService)(nil).ElectionBuckets), epochNum)
}

// EpochMeta mocks base method.
func (m *MockCoreService) EpochMeta(epochNum uint64) (*iotextypes.EpochData, uint64, []*iotexapi.BlockProducerInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EpochMeta", epochNum)
	ret0, _ := ret[0].(*iotextypes.EpochData)
	ret1, _ := ret[1].(uint64)
	ret2, _ := ret[2].([]*iotexapi.BlockProducerInfo)
	ret3, _ := ret[3].(error)
	return ret0, ret1, ret2, ret3
}

// EpochMeta indicates an expected call of EpochMeta.
func (mr *MockCoreServiceMockRecorder) EpochMeta(epochNum any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EpochMeta", reflect.TypeOf((*MockCoreService)(nil).EpochMeta), epochNum)
}

// EstimateExecutionGasConsumption mocks base method.
func (m *MockCoreService) EstimateExecutionGasConsumption(ctx context.Context, sc action.Envelope, callerAddr address.Address, opts ...protocol.SimulateOption) (uint64, []byte, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, sc, callerAddr}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "EstimateExecutionGasConsumption", varargs...)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].([]byte)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// EstimateExecutionGasConsumption indicates an expected call of EstimateExecutionGasConsumption.
func (mr *MockCoreServiceMockRecorder) EstimateExecutionGasConsumption(ctx, sc, callerAddr any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, sc, callerAddr}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EstimateExecutionGasConsumption", reflect.TypeOf((*MockCoreService)(nil).EstimateExecutionGasConsumption), varargs...)
}

// EstimateGasForAction mocks base method.
func (m *MockCoreService) EstimateGasForAction(ctx context.Context, in *iotextypes.Action) (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EstimateGasForAction", ctx, in)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// EstimateGasForAction indicates an expected call of EstimateGasForAction.
func (mr *MockCoreServiceMockRecorder) EstimateGasForAction(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EstimateGasForAction", reflect.TypeOf((*MockCoreService)(nil).EstimateGasForAction), ctx, in)
}

// EstimateGasForNonExecution mocks base method.
func (m *MockCoreService) EstimateGasForNonExecution(arg0 action.Action) (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EstimateGasForNonExecution", arg0)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// EstimateGasForNonExecution indicates an expected call of EstimateGasForNonExecution.
func (mr *MockCoreServiceMockRecorder) EstimateGasForNonExecution(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EstimateGasForNonExecution", reflect.TypeOf((*MockCoreService)(nil).EstimateGasForNonExecution), arg0)
}

// EstimateMigrateStakeGasConsumption mocks base method.
func (m *MockCoreService) EstimateMigrateStakeGasConsumption(arg0 context.Context, arg1 *action.MigrateStake, arg2 address.Address) (uint64, []byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EstimateMigrateStakeGasConsumption", arg0, arg1, arg2)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].([]byte)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// EstimateMigrateStakeGasConsumption indicates an expected call of EstimateMigrateStakeGasConsumption.
func (mr *MockCoreServiceMockRecorder) EstimateMigrateStakeGasConsumption(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EstimateMigrateStakeGasConsumption", reflect.TypeOf((*MockCoreService)(nil).EstimateMigrateStakeGasConsumption), arg0, arg1, arg2)
}

// FeeHistory mocks base method.
func (m *MockCoreService) FeeHistory(ctx context.Context, blocks, lastBlock uint64, rewardPercentiles []float64) (uint64, [][]*big.Int, []*big.Int, []float64, []*big.Int, []float64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FeeHistory", ctx, blocks, lastBlock, rewardPercentiles)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].([][]*big.Int)
	ret2, _ := ret[2].([]*big.Int)
	ret3, _ := ret[3].([]float64)
	ret4, _ := ret[4].([]*big.Int)
	ret5, _ := ret[5].([]float64)
	ret6, _ := ret[6].(error)
	return ret0, ret1, ret2, ret3, ret4, ret5, ret6
}

// FeeHistory indicates an expected call of FeeHistory.
func (mr *MockCoreServiceMockRecorder) FeeHistory(ctx, blocks, lastBlock, rewardPercentiles any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FeeHistory", reflect.TypeOf((*MockCoreService)(nil).FeeHistory), ctx, blocks, lastBlock, rewardPercentiles)
}

// Genesis mocks base method.
func (m *MockCoreService) Genesis() genesis.Genesis {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Genesis")
	ret0, _ := ret[0].(genesis.Genesis)
	return ret0
}

// Genesis indicates an expected call of Genesis.
func (mr *MockCoreServiceMockRecorder) Genesis() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Genesis", reflect.TypeOf((*MockCoreService)(nil).Genesis))
}

// LogsInBlockByHash mocks base method.
func (m *MockCoreService) LogsInBlockByHash(filter *logfilter.LogFilter, blockHash hash.Hash256) ([]*action.Log, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LogsInBlockByHash", filter, blockHash)
	ret0, _ := ret[0].([]*action.Log)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LogsInBlockByHash indicates an expected call of LogsInBlockByHash.
func (mr *MockCoreServiceMockRecorder) LogsInBlockByHash(filter, blockHash any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LogsInBlockByHash", reflect.TypeOf((*MockCoreService)(nil).LogsInBlockByHash), filter, blockHash)
}

// LogsInRange mocks base method.
func (m *MockCoreService) LogsInRange(filter *logfilter.LogFilter, start, end, paginationSize uint64) ([]*action.Log, []hash.Hash256, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LogsInRange", filter, start, end, paginationSize)
	ret0, _ := ret[0].([]*action.Log)
	ret1, _ := ret[1].([]hash.Hash256)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// LogsInRange indicates an expected call of LogsInRange.
func (mr *MockCoreServiceMockRecorder) LogsInRange(filter, start, end, paginationSize any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LogsInRange", reflect.TypeOf((*MockCoreService)(nil).LogsInRange), filter, start, end, paginationSize)
}

// PendingActionByActionHash mocks base method.
func (m *MockCoreService) PendingActionByActionHash(h hash.Hash256) (*action.SealedEnvelope, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PendingActionByActionHash", h)
	ret0, _ := ret[0].(*action.SealedEnvelope)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PendingActionByActionHash indicates an expected call of PendingActionByActionHash.
func (mr *MockCoreServiceMockRecorder) PendingActionByActionHash(h any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PendingActionByActionHash", reflect.TypeOf((*MockCoreService)(nil).PendingActionByActionHash), h)
}

// PendingNonce mocks base method.
func (m *MockCoreService) PendingNonce(arg0 address.Address) (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PendingNonce", arg0)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PendingNonce indicates an expected call of PendingNonce.
func (mr *MockCoreServiceMockRecorder) PendingNonce(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PendingNonce", reflect.TypeOf((*MockCoreService)(nil).PendingNonce), arg0)
}

// RawBlocks mocks base method.
func (m *MockCoreService) RawBlocks(startHeight, count uint64, withReceipts, withTransactionLogs bool) ([]*iotexapi.BlockInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RawBlocks", startHeight, count, withReceipts, withTransactionLogs)
	ret0, _ := ret[0].([]*iotexapi.BlockInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RawBlocks indicates an expected call of RawBlocks.
func (mr *MockCoreServiceMockRecorder) RawBlocks(startHeight, count, withReceipts, withTransactionLogs any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RawBlocks", reflect.TypeOf((*MockCoreService)(nil).RawBlocks), startHeight, count, withReceipts, withTransactionLogs)
}

// ReadContract mocks base method.
func (m *MockCoreService) ReadContract(ctx context.Context, callerAddr address.Address, sc action.Envelope) (string, *iotextypes.Receipt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadContract", ctx, callerAddr, sc)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(*iotextypes.Receipt)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ReadContract indicates an expected call of ReadContract.
func (mr *MockCoreServiceMockRecorder) ReadContract(ctx, callerAddr, sc any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadContract", reflect.TypeOf((*MockCoreService)(nil).ReadContract), ctx, callerAddr, sc)
}

// ReadContractStorage mocks base method.
func (m *MockCoreService) ReadContractStorage(ctx context.Context, addr address.Address, key []byte) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadContractStorage", ctx, addr, key)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadContractStorage indicates an expected call of ReadContractStorage.
func (mr *MockCoreServiceMockRecorder) ReadContractStorage(ctx, addr, key any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadContractStorage", reflect.TypeOf((*MockCoreService)(nil).ReadContractStorage), ctx, addr, key)
}

// ReadState mocks base method.
func (m *MockCoreService) ReadState(protocolID, height string, methodName []byte, arguments [][]byte) (*iotexapi.ReadStateResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadState", protocolID, height, methodName, arguments)
	ret0, _ := ret[0].(*iotexapi.ReadStateResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadState indicates an expected call of ReadState.
func (mr *MockCoreServiceMockRecorder) ReadState(protocolID, height, methodName, arguments any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadState", reflect.TypeOf((*MockCoreService)(nil).ReadState), protocolID, height, methodName, arguments)
}

// ReceiptByActionHash mocks base method.
func (m *MockCoreService) ReceiptByActionHash(h hash.Hash256) (*action.Receipt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReceiptByActionHash", h)
	ret0, _ := ret[0].(*action.Receipt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReceiptByActionHash indicates an expected call of ReceiptByActionHash.
func (mr *MockCoreServiceMockRecorder) ReceiptByActionHash(h any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReceiptByActionHash", reflect.TypeOf((*MockCoreService)(nil).ReceiptByActionHash), h)
}

// ReceiveBlock mocks base method.
func (m *MockCoreService) ReceiveBlock(blk *block.Block) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReceiveBlock", blk)
	ret0, _ := ret[0].(error)
	return ret0
}

// ReceiveBlock indicates an expected call of ReceiveBlock.
func (mr *MockCoreServiceMockRecorder) ReceiveBlock(blk any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReceiveBlock", reflect.TypeOf((*MockCoreService)(nil).ReceiveBlock), blk)
}

// SendAction mocks base method.
func (m *MockCoreService) SendAction(ctx context.Context, in *iotextypes.Action) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendAction", ctx, in)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SendAction indicates an expected call of SendAction.
func (mr *MockCoreServiceMockRecorder) SendAction(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendAction", reflect.TypeOf((*MockCoreService)(nil).SendAction), ctx, in)
}

// ServerMeta mocks base method.
func (m *MockCoreService) ServerMeta() (string, string, string, string, string) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ServerMeta")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(string)
	ret2, _ := ret[2].(string)
	ret3, _ := ret[3].(string)
	ret4, _ := ret[4].(string)
	return ret0, ret1, ret2, ret3, ret4
}

// ServerMeta indicates an expected call of ServerMeta.
func (mr *MockCoreServiceMockRecorder) ServerMeta() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ServerMeta", reflect.TypeOf((*MockCoreService)(nil).ServerMeta))
}

// SimulateExecution mocks base method.
func (m *MockCoreService) SimulateExecution(arg0 context.Context, arg1 address.Address, arg2 action.Envelope) ([]byte, *action.Receipt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SimulateExecution", arg0, arg1, arg2)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(*action.Receipt)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// SimulateExecution indicates an expected call of SimulateExecution.
func (mr *MockCoreServiceMockRecorder) SimulateExecution(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SimulateExecution", reflect.TypeOf((*MockCoreService)(nil).SimulateExecution), arg0, arg1, arg2)
}

// Start mocks base method.
func (m *MockCoreService) Start(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start.
func (mr *MockCoreServiceMockRecorder) Start(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockCoreService)(nil).Start), ctx)
}

// Stop mocks base method.
func (m *MockCoreService) Stop(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop.
func (mr *MockCoreServiceMockRecorder) Stop(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockCoreService)(nil).Stop), ctx)
}

// SuggestGasPrice mocks base method.
func (m *MockCoreService) SuggestGasPrice() (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SuggestGasPrice")
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SuggestGasPrice indicates an expected call of SuggestGasPrice.
func (mr *MockCoreServiceMockRecorder) SuggestGasPrice() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SuggestGasPrice", reflect.TypeOf((*MockCoreService)(nil).SuggestGasPrice))
}

// SuggestGasTipCap mocks base method.
func (m *MockCoreService) SuggestGasTipCap() (*big.Int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SuggestGasTipCap")
	ret0, _ := ret[0].(*big.Int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SuggestGasTipCap indicates an expected call of SuggestGasTipCap.
func (mr *MockCoreServiceMockRecorder) SuggestGasTipCap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SuggestGasTipCap", reflect.TypeOf((*MockCoreService)(nil).SuggestGasTipCap))
}

// SyncingProgress mocks base method.
func (m *MockCoreService) SyncingProgress() (uint64, uint64, uint64) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SyncingProgress")
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(uint64)
	ret2, _ := ret[2].(uint64)
	return ret0, ret1, ret2
}

// SyncingProgress indicates an expected call of SyncingProgress.
func (mr *MockCoreServiceMockRecorder) SyncingProgress() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SyncingProgress", reflect.TypeOf((*MockCoreService)(nil).SyncingProgress))
}

// TipHeight mocks base method.
func (m *MockCoreService) TipHeight() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TipHeight")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// TipHeight indicates an expected call of TipHeight.
func (mr *MockCoreServiceMockRecorder) TipHeight() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TipHeight", reflect.TypeOf((*MockCoreService)(nil).TipHeight))
}

// TraceCall mocks base method.
func (m *MockCoreService) TraceCall(ctx context.Context, callerAddr address.Address, blkNumOrHash any, contractAddress string, nonce uint64, amount *big.Int, gasLimit uint64, data []byte, config *tracers.TraceConfig) ([]byte, *action.Receipt, any, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TraceCall", ctx, callerAddr, blkNumOrHash, contractAddress, nonce, amount, gasLimit, data, config)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(*action.Receipt)
	ret2, _ := ret[2].(any)
	ret3, _ := ret[3].(error)
	return ret0, ret1, ret2, ret3
}

// TraceCall indicates an expected call of TraceCall.
func (mr *MockCoreServiceMockRecorder) TraceCall(ctx, callerAddr, blkNumOrHash, contractAddress, nonce, amount, gasLimit, data, config any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TraceCall", reflect.TypeOf((*MockCoreService)(nil).TraceCall), ctx, callerAddr, blkNumOrHash, contractAddress, nonce, amount, gasLimit, data, config)
}

// TraceTransaction mocks base method.
func (m *MockCoreService) TraceTransaction(ctx context.Context, actHash string, config *tracers.TraceConfig) ([]byte, *action.Receipt, any, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TraceTransaction", ctx, actHash, config)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(*action.Receipt)
	ret2, _ := ret[2].(any)
	ret3, _ := ret[3].(error)
	return ret0, ret1, ret2, ret3
}

// TraceTransaction indicates an expected call of TraceTransaction.
func (mr *MockCoreServiceMockRecorder) TraceTransaction(ctx, actHash, config any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TraceTransaction", reflect.TypeOf((*MockCoreService)(nil).TraceTransaction), ctx, actHash, config)
}

// Track mocks base method.
func (m *MockCoreService) Track(ctx context.Context, start time.Time, method string, size int64, success bool) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Track", ctx, start, method, size, success)
}

// Track indicates an expected call of Track.
func (mr *MockCoreServiceMockRecorder) Track(ctx, start, method, size, success any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Track", reflect.TypeOf((*MockCoreService)(nil).Track), ctx, start, method, size, success)
}

// TransactionLogByActionHash mocks base method.
func (m *MockCoreService) TransactionLogByActionHash(actHash string) (*iotextypes.TransactionLog, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TransactionLogByActionHash", actHash)
	ret0, _ := ret[0].(*iotextypes.TransactionLog)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// TransactionLogByActionHash indicates an expected call of TransactionLogByActionHash.
func (mr *MockCoreServiceMockRecorder) TransactionLogByActionHash(actHash any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TransactionLogByActionHash", reflect.TypeOf((*MockCoreService)(nil).TransactionLogByActionHash), actHash)
}

// TransactionLogByBlockHeight mocks base method.
func (m *MockCoreService) TransactionLogByBlockHeight(blockHeight uint64) (*iotextypes.BlockIdentifier, *iotextypes.TransactionLogs, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TransactionLogByBlockHeight", blockHeight)
	ret0, _ := ret[0].(*iotextypes.BlockIdentifier)
	ret1, _ := ret[1].(*iotextypes.TransactionLogs)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// TransactionLogByBlockHeight indicates an expected call of TransactionLogByBlockHeight.
func (mr *MockCoreServiceMockRecorder) TransactionLogByBlockHeight(blockHeight any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TransactionLogByBlockHeight", reflect.TypeOf((*MockCoreService)(nil).TransactionLogByBlockHeight), blockHeight)
}

// UnconfirmedActionsByAddress mocks base method.
func (m *MockCoreService) UnconfirmedActionsByAddress(arg0 string, start, count uint64) ([]*iotexapi.ActionInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UnconfirmedActionsByAddress", arg0, start, count)
	ret0, _ := ret[0].([]*iotexapi.ActionInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UnconfirmedActionsByAddress indicates an expected call of UnconfirmedActionsByAddress.
func (mr *MockCoreServiceMockRecorder) UnconfirmedActionsByAddress(arg0, start, count any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnconfirmedActionsByAddress", reflect.TypeOf((*MockCoreService)(nil).UnconfirmedActionsByAddress), arg0, start, count)
}

// WithHeight mocks base method.
func (m *MockCoreService) WithHeight(arg0 uint64) CoreServiceReaderWithHeight {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WithHeight", arg0)
	ret0, _ := ret[0].(CoreServiceReaderWithHeight)
	return ret0
}

// WithHeight indicates an expected call of WithHeight.
func (mr *MockCoreServiceMockRecorder) WithHeight(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithHeight", reflect.TypeOf((*MockCoreService)(nil).WithHeight), arg0)
}

// MockintrinsicGasCalculator is a mock of intrinsicGasCalculator interface.
type MockintrinsicGasCalculator struct {
	ctrl     *gomock.Controller
	recorder *MockintrinsicGasCalculatorMockRecorder
	isgomock struct{}
}

// MockintrinsicGasCalculatorMockRecorder is the mock recorder for MockintrinsicGasCalculator.
type MockintrinsicGasCalculatorMockRecorder struct {
	mock *MockintrinsicGasCalculator
}

// NewMockintrinsicGasCalculator creates a new mock instance.
func NewMockintrinsicGasCalculator(ctrl *gomock.Controller) *MockintrinsicGasCalculator {
	mock := &MockintrinsicGasCalculator{ctrl: ctrl}
	mock.recorder = &MockintrinsicGasCalculatorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockintrinsicGasCalculator) EXPECT() *MockintrinsicGasCalculatorMockRecorder {
	return m.recorder
}

// IntrinsicGas mocks base method.
func (m *MockintrinsicGasCalculator) IntrinsicGas() (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IntrinsicGas")
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IntrinsicGas indicates an expected call of IntrinsicGas.
func (mr *MockintrinsicGasCalculatorMockRecorder) IntrinsicGas() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IntrinsicGas", reflect.TypeOf((*MockintrinsicGasCalculator)(nil).IntrinsicGas))
}
