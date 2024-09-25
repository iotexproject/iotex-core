// Code generated by MockGen. DO NOT EDIT.
// Source: ./action/envelope.go

// Package mock_envelope is a generated GoMock package.
package mock_envelope

import (
	big "math/big"
	reflect "reflect"

	common "github.com/ethereum/go-ethereum/common"
	types "github.com/ethereum/go-ethereum/core/types"
	gomock "github.com/golang/mock/gomock"
	action "github.com/iotexproject/iotex-core/action"
	iotextypes "github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// MockEnvelope is a mock of Envelope interface.
type MockEnvelope struct {
	ctrl     *gomock.Controller
	recorder *MockEnvelopeMockRecorder
}

// MockEnvelopeMockRecorder is the mock recorder for MockEnvelope.
type MockEnvelopeMockRecorder struct {
	mock *MockEnvelope
}

// NewMockEnvelope creates a new mock instance.
func NewMockEnvelope(ctrl *gomock.Controller) *MockEnvelope {
	mock := &MockEnvelope{ctrl: ctrl}
	mock.recorder = &MockEnvelopeMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockEnvelope) EXPECT() *MockEnvelopeMockRecorder {
	return m.recorder
}

// AccessList mocks base method.
func (m *MockEnvelope) AccessList() types.AccessList {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AccessList")
	ret0, _ := ret[0].(types.AccessList)
	return ret0
}

// AccessList indicates an expected call of AccessList.
func (mr *MockEnvelopeMockRecorder) AccessList() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AccessList", reflect.TypeOf((*MockEnvelope)(nil).AccessList))
}

// Action mocks base method.
func (m *MockEnvelope) Action() action.Action {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Action")
	ret0, _ := ret[0].(action.Action)
	return ret0
}

// Action indicates an expected call of Action.
func (mr *MockEnvelopeMockRecorder) Action() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Action", reflect.TypeOf((*MockEnvelope)(nil).Action))
}

// BlobGas mocks base method.
func (m *MockEnvelope) BlobGas() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobGas")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// BlobGas indicates an expected call of BlobGas.
func (mr *MockEnvelopeMockRecorder) BlobGas() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobGas", reflect.TypeOf((*MockEnvelope)(nil).BlobGas))
}

// BlobGasFeeCap mocks base method.
func (m *MockEnvelope) BlobGasFeeCap() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobGasFeeCap")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// BlobGasFeeCap indicates an expected call of BlobGasFeeCap.
func (mr *MockEnvelopeMockRecorder) BlobGasFeeCap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobGasFeeCap", reflect.TypeOf((*MockEnvelope)(nil).BlobGasFeeCap))
}

// BlobHashes mocks base method.
func (m *MockEnvelope) BlobHashes() []common.Hash {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobHashes")
	ret0, _ := ret[0].([]common.Hash)
	return ret0
}

// BlobHashes indicates an expected call of BlobHashes.
func (mr *MockEnvelopeMockRecorder) BlobHashes() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobHashes", reflect.TypeOf((*MockEnvelope)(nil).BlobHashes))
}

// BlobTxSidecar mocks base method.
func (m *MockEnvelope) BlobTxSidecar() *types.BlobTxSidecar {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobTxSidecar")
	ret0, _ := ret[0].(*types.BlobTxSidecar)
	return ret0
}

// BlobTxSidecar indicates an expected call of BlobTxSidecar.
func (mr *MockEnvelopeMockRecorder) BlobTxSidecar() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobTxSidecar", reflect.TypeOf((*MockEnvelope)(nil).BlobTxSidecar))
}

// ChainID mocks base method.
func (m *MockEnvelope) ChainID() uint32 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ChainID")
	ret0, _ := ret[0].(uint32)
	return ret0
}

// ChainID indicates an expected call of ChainID.
func (mr *MockEnvelopeMockRecorder) ChainID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ChainID", reflect.TypeOf((*MockEnvelope)(nil).ChainID))
}

// Cost mocks base method.
func (m *MockEnvelope) Cost() (*big.Int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Cost")
	ret0, _ := ret[0].(*big.Int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Cost indicates an expected call of Cost.
func (mr *MockEnvelopeMockRecorder) Cost() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Cost", reflect.TypeOf((*MockEnvelope)(nil).Cost))
}

// Data mocks base method.
func (m *MockEnvelope) Data() []byte {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Data")
	ret0, _ := ret[0].([]byte)
	return ret0
}

// Data indicates an expected call of Data.
func (mr *MockEnvelopeMockRecorder) Data() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Data", reflect.TypeOf((*MockEnvelope)(nil).Data))
}

// Destination mocks base method.
func (m *MockEnvelope) Destination() (string, bool) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Destination")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(bool)
	return ret0, ret1
}

// Destination indicates an expected call of Destination.
func (mr *MockEnvelopeMockRecorder) Destination() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Destination", reflect.TypeOf((*MockEnvelope)(nil).Destination))
}

// EffectiveGasPrice mocks base method.
func (m *MockEnvelope) EffectiveGasPrice(arg0 *big.Int) *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EffectiveGasPrice", arg0)
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// EffectiveGasPrice indicates an expected call of EffectiveGasPrice.
func (mr *MockEnvelopeMockRecorder) EffectiveGasPrice(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EffectiveGasPrice", reflect.TypeOf((*MockEnvelope)(nil).EffectiveGasPrice), arg0)
}

// Gas mocks base method.
func (m *MockEnvelope) Gas() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Gas")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// Gas indicates an expected call of Gas.
func (mr *MockEnvelopeMockRecorder) Gas() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Gas", reflect.TypeOf((*MockEnvelope)(nil).Gas))
}

// GasFeeCap mocks base method.
func (m *MockEnvelope) GasFeeCap() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GasFeeCap")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// GasFeeCap indicates an expected call of GasFeeCap.
func (mr *MockEnvelopeMockRecorder) GasFeeCap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GasFeeCap", reflect.TypeOf((*MockEnvelope)(nil).GasFeeCap))
}

// GasPrice mocks base method.
func (m *MockEnvelope) GasPrice() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GasPrice")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// GasPrice indicates an expected call of GasPrice.
func (mr *MockEnvelopeMockRecorder) GasPrice() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GasPrice", reflect.TypeOf((*MockEnvelope)(nil).GasPrice))
}

// GasTipCap mocks base method.
func (m *MockEnvelope) GasTipCap() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GasTipCap")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// GasTipCap indicates an expected call of GasTipCap.
func (mr *MockEnvelopeMockRecorder) GasTipCap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GasTipCap", reflect.TypeOf((*MockEnvelope)(nil).GasTipCap))
}

// IntrinsicGas mocks base method.
func (m *MockEnvelope) IntrinsicGas() (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IntrinsicGas")
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IntrinsicGas indicates an expected call of IntrinsicGas.
func (mr *MockEnvelopeMockRecorder) IntrinsicGas() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IntrinsicGas", reflect.TypeOf((*MockEnvelope)(nil).IntrinsicGas))
}

// LoadProto mocks base method.
func (m *MockEnvelope) LoadProto(arg0 *iotextypes.ActionCore) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LoadProto", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// LoadProto indicates an expected call of LoadProto.
func (mr *MockEnvelopeMockRecorder) LoadProto(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LoadProto", reflect.TypeOf((*MockEnvelope)(nil).LoadProto), arg0)
}

// Nonce mocks base method.
func (m *MockEnvelope) Nonce() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Nonce")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// Nonce indicates an expected call of Nonce.
func (mr *MockEnvelopeMockRecorder) Nonce() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Nonce", reflect.TypeOf((*MockEnvelope)(nil).Nonce))
}

// Proto mocks base method.
func (m *MockEnvelope) Proto() *iotextypes.ActionCore {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Proto")
	ret0, _ := ret[0].(*iotextypes.ActionCore)
	return ret0
}

// Proto indicates an expected call of Proto.
func (mr *MockEnvelopeMockRecorder) Proto() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Proto", reflect.TypeOf((*MockEnvelope)(nil).Proto))
}

// SanityCheck mocks base method.
func (m *MockEnvelope) SanityCheck() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SanityCheck")
	ret0, _ := ret[0].(error)
	return ret0
}

// SanityCheck indicates an expected call of SanityCheck.
func (mr *MockEnvelopeMockRecorder) SanityCheck() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SanityCheck", reflect.TypeOf((*MockEnvelope)(nil).SanityCheck))
}

// SetChainID mocks base method.
func (m *MockEnvelope) SetChainID(arg0 uint32) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetChainID", arg0)
}

// SetChainID indicates an expected call of SetChainID.
func (mr *MockEnvelopeMockRecorder) SetChainID(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetChainID", reflect.TypeOf((*MockEnvelope)(nil).SetChainID), arg0)
}

// SetGas mocks base method.
func (m *MockEnvelope) SetGas(arg0 uint64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetGas", arg0)
}

// SetGas indicates an expected call of SetGas.
func (mr *MockEnvelopeMockRecorder) SetGas(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetGas", reflect.TypeOf((*MockEnvelope)(nil).SetGas), arg0)
}

// SetNonce mocks base method.
func (m *MockEnvelope) SetNonce(arg0 uint64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetNonce", arg0)
}

// SetNonce indicates an expected call of SetNonce.
func (mr *MockEnvelopeMockRecorder) SetNonce(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetNonce", reflect.TypeOf((*MockEnvelope)(nil).SetNonce), arg0)
}

// Size mocks base method.
func (m *MockEnvelope) Size() uint32 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Size")
	ret0, _ := ret[0].(uint32)
	return ret0
}

// Size indicates an expected call of Size.
func (mr *MockEnvelopeMockRecorder) Size() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Size", reflect.TypeOf((*MockEnvelope)(nil).Size))
}

// To mocks base method.
func (m *MockEnvelope) To() *common.Address {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "To")
	ret0, _ := ret[0].(*common.Address)
	return ret0
}

// To indicates an expected call of To.
func (mr *MockEnvelopeMockRecorder) To() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "To", reflect.TypeOf((*MockEnvelope)(nil).To))
}

// ToEthTx mocks base method.
func (m *MockEnvelope) ToEthTx(arg0 uint32, arg1 iotextypes.Encoding) (*types.Transaction, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ToEthTx", arg0, arg1)
	ret0, _ := ret[0].(*types.Transaction)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ToEthTx indicates an expected call of ToEthTx.
func (mr *MockEnvelopeMockRecorder) ToEthTx(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ToEthTx", reflect.TypeOf((*MockEnvelope)(nil).ToEthTx), arg0, arg1)
}

// Value mocks base method.
func (m *MockEnvelope) Value() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Value")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// Value indicates an expected call of Value.
func (mr *MockEnvelopeMockRecorder) Value() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Value", reflect.TypeOf((*MockEnvelope)(nil).Value))
}

// Version mocks base method.
func (m *MockEnvelope) Version() uint32 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Version")
	ret0, _ := ret[0].(uint32)
	return ret0
}

// Version indicates an expected call of Version.
func (mr *MockEnvelopeMockRecorder) Version() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Version", reflect.TypeOf((*MockEnvelope)(nil).Version))
}

// MockTxData is a mock of TxData interface.
type MockTxData struct {
	ctrl     *gomock.Controller
	recorder *MockTxDataMockRecorder
}

// MockTxDataMockRecorder is the mock recorder for MockTxData.
type MockTxDataMockRecorder struct {
	mock *MockTxData
}

// NewMockTxData creates a new mock instance.
func NewMockTxData(ctrl *gomock.Controller) *MockTxData {
	mock := &MockTxData{ctrl: ctrl}
	mock.recorder = &MockTxDataMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTxData) EXPECT() *MockTxDataMockRecorder {
	return m.recorder
}

// AccessList mocks base method.
func (m *MockTxData) AccessList() types.AccessList {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AccessList")
	ret0, _ := ret[0].(types.AccessList)
	return ret0
}

// AccessList indicates an expected call of AccessList.
func (mr *MockTxDataMockRecorder) AccessList() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AccessList", reflect.TypeOf((*MockTxData)(nil).AccessList))
}

// BlobGas mocks base method.
func (m *MockTxData) BlobGas() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobGas")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// BlobGas indicates an expected call of BlobGas.
func (mr *MockTxDataMockRecorder) BlobGas() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobGas", reflect.TypeOf((*MockTxData)(nil).BlobGas))
}

// BlobGasFeeCap mocks base method.
func (m *MockTxData) BlobGasFeeCap() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobGasFeeCap")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// BlobGasFeeCap indicates an expected call of BlobGasFeeCap.
func (mr *MockTxDataMockRecorder) BlobGasFeeCap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobGasFeeCap", reflect.TypeOf((*MockTxData)(nil).BlobGasFeeCap))
}

// BlobHashes mocks base method.
func (m *MockTxData) BlobHashes() []common.Hash {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobHashes")
	ret0, _ := ret[0].([]common.Hash)
	return ret0
}

// BlobHashes indicates an expected call of BlobHashes.
func (mr *MockTxDataMockRecorder) BlobHashes() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobHashes", reflect.TypeOf((*MockTxData)(nil).BlobHashes))
}

// BlobTxSidecar mocks base method.
func (m *MockTxData) BlobTxSidecar() *types.BlobTxSidecar {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobTxSidecar")
	ret0, _ := ret[0].(*types.BlobTxSidecar)
	return ret0
}

// BlobTxSidecar indicates an expected call of BlobTxSidecar.
func (mr *MockTxDataMockRecorder) BlobTxSidecar() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobTxSidecar", reflect.TypeOf((*MockTxData)(nil).BlobTxSidecar))
}

// Data mocks base method.
func (m *MockTxData) Data() []byte {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Data")
	ret0, _ := ret[0].([]byte)
	return ret0
}

// Data indicates an expected call of Data.
func (mr *MockTxDataMockRecorder) Data() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Data", reflect.TypeOf((*MockTxData)(nil).Data))
}

// EffectiveGasPrice mocks base method.
func (m *MockTxData) EffectiveGasPrice(arg0 *big.Int) *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EffectiveGasPrice", arg0)
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// EffectiveGasPrice indicates an expected call of EffectiveGasPrice.
func (mr *MockTxDataMockRecorder) EffectiveGasPrice(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EffectiveGasPrice", reflect.TypeOf((*MockTxData)(nil).EffectiveGasPrice), arg0)
}

// Gas mocks base method.
func (m *MockTxData) Gas() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Gas")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// Gas indicates an expected call of Gas.
func (mr *MockTxDataMockRecorder) Gas() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Gas", reflect.TypeOf((*MockTxData)(nil).Gas))
}

// GasFeeCap mocks base method.
func (m *MockTxData) GasFeeCap() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GasFeeCap")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// GasFeeCap indicates an expected call of GasFeeCap.
func (mr *MockTxDataMockRecorder) GasFeeCap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GasFeeCap", reflect.TypeOf((*MockTxData)(nil).GasFeeCap))
}

// GasPrice mocks base method.
func (m *MockTxData) GasPrice() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GasPrice")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// GasPrice indicates an expected call of GasPrice.
func (mr *MockTxDataMockRecorder) GasPrice() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GasPrice", reflect.TypeOf((*MockTxData)(nil).GasPrice))
}

// GasTipCap mocks base method.
func (m *MockTxData) GasTipCap() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GasTipCap")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// GasTipCap indicates an expected call of GasTipCap.
func (mr *MockTxDataMockRecorder) GasTipCap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GasTipCap", reflect.TypeOf((*MockTxData)(nil).GasTipCap))
}

// Nonce mocks base method.
func (m *MockTxData) Nonce() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Nonce")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// Nonce indicates an expected call of Nonce.
func (mr *MockTxDataMockRecorder) Nonce() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Nonce", reflect.TypeOf((*MockTxData)(nil).Nonce))
}

// To mocks base method.
func (m *MockTxData) To() *common.Address {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "To")
	ret0, _ := ret[0].(*common.Address)
	return ret0
}

// To indicates an expected call of To.
func (mr *MockTxDataMockRecorder) To() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "To", reflect.TypeOf((*MockTxData)(nil).To))
}

// Value mocks base method.
func (m *MockTxData) Value() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Value")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// Value indicates an expected call of Value.
func (mr *MockTxDataMockRecorder) Value() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Value", reflect.TypeOf((*MockTxData)(nil).Value))
}

// MockTxCommon is a mock of TxCommon interface.
type MockTxCommon struct {
	ctrl     *gomock.Controller
	recorder *MockTxCommonMockRecorder
}

// MockTxCommonMockRecorder is the mock recorder for MockTxCommon.
type MockTxCommonMockRecorder struct {
	mock *MockTxCommon
}

// NewMockTxCommon creates a new mock instance.
func NewMockTxCommon(ctrl *gomock.Controller) *MockTxCommon {
	mock := &MockTxCommon{ctrl: ctrl}
	mock.recorder = &MockTxCommonMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTxCommon) EXPECT() *MockTxCommonMockRecorder {
	return m.recorder
}

// AccessList mocks base method.
func (m *MockTxCommon) AccessList() types.AccessList {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AccessList")
	ret0, _ := ret[0].(types.AccessList)
	return ret0
}

// AccessList indicates an expected call of AccessList.
func (mr *MockTxCommonMockRecorder) AccessList() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AccessList", reflect.TypeOf((*MockTxCommon)(nil).AccessList))
}

// BlobGas mocks base method.
func (m *MockTxCommon) BlobGas() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobGas")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// BlobGas indicates an expected call of BlobGas.
func (mr *MockTxCommonMockRecorder) BlobGas() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobGas", reflect.TypeOf((*MockTxCommon)(nil).BlobGas))
}

// BlobGasFeeCap mocks base method.
func (m *MockTxCommon) BlobGasFeeCap() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobGasFeeCap")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// BlobGasFeeCap indicates an expected call of BlobGasFeeCap.
func (mr *MockTxCommonMockRecorder) BlobGasFeeCap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobGasFeeCap", reflect.TypeOf((*MockTxCommon)(nil).BlobGasFeeCap))
}

// BlobHashes mocks base method.
func (m *MockTxCommon) BlobHashes() []common.Hash {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobHashes")
	ret0, _ := ret[0].([]common.Hash)
	return ret0
}

// BlobHashes indicates an expected call of BlobHashes.
func (mr *MockTxCommonMockRecorder) BlobHashes() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobHashes", reflect.TypeOf((*MockTxCommon)(nil).BlobHashes))
}

// BlobTxSidecar mocks base method.
func (m *MockTxCommon) BlobTxSidecar() *types.BlobTxSidecar {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobTxSidecar")
	ret0, _ := ret[0].(*types.BlobTxSidecar)
	return ret0
}

// BlobTxSidecar indicates an expected call of BlobTxSidecar.
func (mr *MockTxCommonMockRecorder) BlobTxSidecar() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobTxSidecar", reflect.TypeOf((*MockTxCommon)(nil).BlobTxSidecar))
}

// EffectiveGasPrice mocks base method.
func (m *MockTxCommon) EffectiveGasPrice(arg0 *big.Int) *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EffectiveGasPrice", arg0)
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// EffectiveGasPrice indicates an expected call of EffectiveGasPrice.
func (mr *MockTxCommonMockRecorder) EffectiveGasPrice(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EffectiveGasPrice", reflect.TypeOf((*MockTxCommon)(nil).EffectiveGasPrice), arg0)
}

// Gas mocks base method.
func (m *MockTxCommon) Gas() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Gas")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// Gas indicates an expected call of Gas.
func (mr *MockTxCommonMockRecorder) Gas() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Gas", reflect.TypeOf((*MockTxCommon)(nil).Gas))
}

// GasFeeCap mocks base method.
func (m *MockTxCommon) GasFeeCap() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GasFeeCap")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// GasFeeCap indicates an expected call of GasFeeCap.
func (mr *MockTxCommonMockRecorder) GasFeeCap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GasFeeCap", reflect.TypeOf((*MockTxCommon)(nil).GasFeeCap))
}

// GasPrice mocks base method.
func (m *MockTxCommon) GasPrice() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GasPrice")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// GasPrice indicates an expected call of GasPrice.
func (mr *MockTxCommonMockRecorder) GasPrice() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GasPrice", reflect.TypeOf((*MockTxCommon)(nil).GasPrice))
}

// GasTipCap mocks base method.
func (m *MockTxCommon) GasTipCap() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GasTipCap")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// GasTipCap indicates an expected call of GasTipCap.
func (mr *MockTxCommonMockRecorder) GasTipCap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GasTipCap", reflect.TypeOf((*MockTxCommon)(nil).GasTipCap))
}

// Nonce mocks base method.
func (m *MockTxCommon) Nonce() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Nonce")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// Nonce indicates an expected call of Nonce.
func (mr *MockTxCommonMockRecorder) Nonce() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Nonce", reflect.TypeOf((*MockTxCommon)(nil).Nonce))
}

// MockTxCommonInternal is a mock of TxCommonInternal interface.
type MockTxCommonInternal struct {
	ctrl     *gomock.Controller
	recorder *MockTxCommonInternalMockRecorder
}

// MockTxCommonInternalMockRecorder is the mock recorder for MockTxCommonInternal.
type MockTxCommonInternalMockRecorder struct {
	mock *MockTxCommonInternal
}

// NewMockTxCommonInternal creates a new mock instance.
func NewMockTxCommonInternal(ctrl *gomock.Controller) *MockTxCommonInternal {
	mock := &MockTxCommonInternal{ctrl: ctrl}
	mock.recorder = &MockTxCommonInternalMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTxCommonInternal) EXPECT() *MockTxCommonInternalMockRecorder {
	return m.recorder
}

// AccessList mocks base method.
func (m *MockTxCommonInternal) AccessList() types.AccessList {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AccessList")
	ret0, _ := ret[0].(types.AccessList)
	return ret0
}

// AccessList indicates an expected call of AccessList.
func (mr *MockTxCommonInternalMockRecorder) AccessList() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AccessList", reflect.TypeOf((*MockTxCommonInternal)(nil).AccessList))
}

// BlobGas mocks base method.
func (m *MockTxCommonInternal) BlobGas() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobGas")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// BlobGas indicates an expected call of BlobGas.
func (mr *MockTxCommonInternalMockRecorder) BlobGas() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobGas", reflect.TypeOf((*MockTxCommonInternal)(nil).BlobGas))
}

// BlobGasFeeCap mocks base method.
func (m *MockTxCommonInternal) BlobGasFeeCap() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobGasFeeCap")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// BlobGasFeeCap indicates an expected call of BlobGasFeeCap.
func (mr *MockTxCommonInternalMockRecorder) BlobGasFeeCap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobGasFeeCap", reflect.TypeOf((*MockTxCommonInternal)(nil).BlobGasFeeCap))
}

// BlobHashes mocks base method.
func (m *MockTxCommonInternal) BlobHashes() []common.Hash {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobHashes")
	ret0, _ := ret[0].([]common.Hash)
	return ret0
}

// BlobHashes indicates an expected call of BlobHashes.
func (mr *MockTxCommonInternalMockRecorder) BlobHashes() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobHashes", reflect.TypeOf((*MockTxCommonInternal)(nil).BlobHashes))
}

// BlobTxSidecar mocks base method.
func (m *MockTxCommonInternal) BlobTxSidecar() *types.BlobTxSidecar {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobTxSidecar")
	ret0, _ := ret[0].(*types.BlobTxSidecar)
	return ret0
}

// BlobTxSidecar indicates an expected call of BlobTxSidecar.
func (mr *MockTxCommonInternalMockRecorder) BlobTxSidecar() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobTxSidecar", reflect.TypeOf((*MockTxCommonInternal)(nil).BlobTxSidecar))
}

// ChainID mocks base method.
func (m *MockTxCommonInternal) ChainID() uint32 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ChainID")
	ret0, _ := ret[0].(uint32)
	return ret0
}

// ChainID indicates an expected call of ChainID.
func (mr *MockTxCommonInternalMockRecorder) ChainID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ChainID", reflect.TypeOf((*MockTxCommonInternal)(nil).ChainID))
}

// EffectiveGasPrice mocks base method.
func (m *MockTxCommonInternal) EffectiveGasPrice(arg0 *big.Int) *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EffectiveGasPrice", arg0)
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// EffectiveGasPrice indicates an expected call of EffectiveGasPrice.
func (mr *MockTxCommonInternalMockRecorder) EffectiveGasPrice(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EffectiveGasPrice", reflect.TypeOf((*MockTxCommonInternal)(nil).EffectiveGasPrice), arg0)
}

// Gas mocks base method.
func (m *MockTxCommonInternal) Gas() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Gas")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// Gas indicates an expected call of Gas.
func (mr *MockTxCommonInternalMockRecorder) Gas() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Gas", reflect.TypeOf((*MockTxCommonInternal)(nil).Gas))
}

// GasFeeCap mocks base method.
func (m *MockTxCommonInternal) GasFeeCap() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GasFeeCap")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// GasFeeCap indicates an expected call of GasFeeCap.
func (mr *MockTxCommonInternalMockRecorder) GasFeeCap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GasFeeCap", reflect.TypeOf((*MockTxCommonInternal)(nil).GasFeeCap))
}

// GasPrice mocks base method.
func (m *MockTxCommonInternal) GasPrice() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GasPrice")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// GasPrice indicates an expected call of GasPrice.
func (mr *MockTxCommonInternalMockRecorder) GasPrice() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GasPrice", reflect.TypeOf((*MockTxCommonInternal)(nil).GasPrice))
}

// GasTipCap mocks base method.
func (m *MockTxCommonInternal) GasTipCap() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GasTipCap")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// GasTipCap indicates an expected call of GasTipCap.
func (mr *MockTxCommonInternalMockRecorder) GasTipCap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GasTipCap", reflect.TypeOf((*MockTxCommonInternal)(nil).GasTipCap))
}

// Nonce mocks base method.
func (m *MockTxCommonInternal) Nonce() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Nonce")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// Nonce indicates an expected call of Nonce.
func (mr *MockTxCommonInternalMockRecorder) Nonce() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Nonce", reflect.TypeOf((*MockTxCommonInternal)(nil).Nonce))
}

// SanityCheck mocks base method.
func (m *MockTxCommonInternal) SanityCheck() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SanityCheck")
	ret0, _ := ret[0].(error)
	return ret0
}

// SanityCheck indicates an expected call of SanityCheck.
func (mr *MockTxCommonInternalMockRecorder) SanityCheck() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SanityCheck", reflect.TypeOf((*MockTxCommonInternal)(nil).SanityCheck))
}

// Version mocks base method.
func (m *MockTxCommonInternal) Version() uint32 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Version")
	ret0, _ := ret[0].(uint32)
	return ret0
}

// Version indicates an expected call of Version.
func (mr *MockTxCommonInternalMockRecorder) Version() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Version", reflect.TypeOf((*MockTxCommonInternal)(nil).Version))
}

// setChainID mocks base method.
func (m *MockTxCommonInternal) setChainID(arg0 uint32) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "setChainID", arg0)
}

// setChainID indicates an expected call of setChainID.
func (mr *MockTxCommonInternalMockRecorder) setChainID(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "setChainID", reflect.TypeOf((*MockTxCommonInternal)(nil).setChainID), arg0)
}

// setGas mocks base method.
func (m *MockTxCommonInternal) setGas(arg0 uint64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "setGas", arg0)
}

// setGas indicates an expected call of setGas.
func (mr *MockTxCommonInternalMockRecorder) setGas(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "setGas", reflect.TypeOf((*MockTxCommonInternal)(nil).setGas), arg0)
}

// setNonce mocks base method.
func (m *MockTxCommonInternal) setNonce(arg0 uint64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "setNonce", arg0)
}

// setNonce indicates an expected call of setNonce.
func (mr *MockTxCommonInternalMockRecorder) setNonce(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "setNonce", reflect.TypeOf((*MockTxCommonInternal)(nil).setNonce), arg0)
}

// toEthTx mocks base method.
func (m *MockTxCommonInternal) toEthTx(arg0 *common.Address, arg1 *big.Int, arg2 []byte) *types.Transaction {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "toEthTx", arg0, arg1, arg2)
	ret0, _ := ret[0].(*types.Transaction)
	return ret0
}

// toEthTx indicates an expected call of toEthTx.
func (mr *MockTxCommonInternalMockRecorder) toEthTx(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "toEthTx", reflect.TypeOf((*MockTxCommonInternal)(nil).toEthTx), arg0, arg1, arg2)
}

// toProto mocks base method.
func (m *MockTxCommonInternal) toProto() *iotextypes.ActionCore {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "toProto")
	ret0, _ := ret[0].(*iotextypes.ActionCore)
	return ret0
}

// toProto indicates an expected call of toProto.
func (mr *MockTxCommonInternalMockRecorder) toProto() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "toProto", reflect.TypeOf((*MockTxCommonInternal)(nil).toProto))
}

// MockTxDynamicGas is a mock of TxDynamicGas interface.
type MockTxDynamicGas struct {
	ctrl     *gomock.Controller
	recorder *MockTxDynamicGasMockRecorder
}

// MockTxDynamicGasMockRecorder is the mock recorder for MockTxDynamicGas.
type MockTxDynamicGasMockRecorder struct {
	mock *MockTxDynamicGas
}

// NewMockTxDynamicGas creates a new mock instance.
func NewMockTxDynamicGas(ctrl *gomock.Controller) *MockTxDynamicGas {
	mock := &MockTxDynamicGas{ctrl: ctrl}
	mock.recorder = &MockTxDynamicGasMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTxDynamicGas) EXPECT() *MockTxDynamicGasMockRecorder {
	return m.recorder
}

// GasFeeCap mocks base method.
func (m *MockTxDynamicGas) GasFeeCap() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GasFeeCap")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// GasFeeCap indicates an expected call of GasFeeCap.
func (mr *MockTxDynamicGasMockRecorder) GasFeeCap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GasFeeCap", reflect.TypeOf((*MockTxDynamicGas)(nil).GasFeeCap))
}

// GasTipCap mocks base method.
func (m *MockTxDynamicGas) GasTipCap() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GasTipCap")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// GasTipCap indicates an expected call of GasTipCap.
func (mr *MockTxDynamicGasMockRecorder) GasTipCap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GasTipCap", reflect.TypeOf((*MockTxDynamicGas)(nil).GasTipCap))
}

// MockTxBlob is a mock of TxBlob interface.
type MockTxBlob struct {
	ctrl     *gomock.Controller
	recorder *MockTxBlobMockRecorder
}

// MockTxBlobMockRecorder is the mock recorder for MockTxBlob.
type MockTxBlobMockRecorder struct {
	mock *MockTxBlob
}

// NewMockTxBlob creates a new mock instance.
func NewMockTxBlob(ctrl *gomock.Controller) *MockTxBlob {
	mock := &MockTxBlob{ctrl: ctrl}
	mock.recorder = &MockTxBlobMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTxBlob) EXPECT() *MockTxBlobMockRecorder {
	return m.recorder
}

// BlobGas mocks base method.
func (m *MockTxBlob) BlobGas() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobGas")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// BlobGas indicates an expected call of BlobGas.
func (mr *MockTxBlobMockRecorder) BlobGas() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobGas", reflect.TypeOf((*MockTxBlob)(nil).BlobGas))
}

// BlobGasFeeCap mocks base method.
func (m *MockTxBlob) BlobGasFeeCap() *big.Int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobGasFeeCap")
	ret0, _ := ret[0].(*big.Int)
	return ret0
}

// BlobGasFeeCap indicates an expected call of BlobGasFeeCap.
func (mr *MockTxBlobMockRecorder) BlobGasFeeCap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobGasFeeCap", reflect.TypeOf((*MockTxBlob)(nil).BlobGasFeeCap))
}

// BlobHashes mocks base method.
func (m *MockTxBlob) BlobHashes() []common.Hash {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobHashes")
	ret0, _ := ret[0].([]common.Hash)
	return ret0
}

// BlobHashes indicates an expected call of BlobHashes.
func (mr *MockTxBlobMockRecorder) BlobHashes() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobHashes", reflect.TypeOf((*MockTxBlob)(nil).BlobHashes))
}

// BlobTxSidecar mocks base method.
func (m *MockTxBlob) BlobTxSidecar() *types.BlobTxSidecar {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BlobTxSidecar")
	ret0, _ := ret[0].(*types.BlobTxSidecar)
	return ret0
}

// BlobTxSidecar indicates an expected call of BlobTxSidecar.
func (mr *MockTxBlobMockRecorder) BlobTxSidecar() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlobTxSidecar", reflect.TypeOf((*MockTxBlob)(nil).BlobTxSidecar))
}
