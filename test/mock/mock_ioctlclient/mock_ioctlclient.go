// Code generated by MockGen. DO NOT EDIT.
// Source: ./ioctl/client.go

// Package mock_ioctlclient is a generated GoMock package.
package mock_ioctlclient

import (
	context "context"
	ecdsa "crypto/ecdsa"
	http "net/http"
	reflect "reflect"

	keystore "github.com/ethereum/go-ethereum/accounts/keystore"
	gomock "github.com/golang/mock/gomock"
	ioctl "github.com/iotexproject/iotex-core/ioctl"
	config "github.com/iotexproject/iotex-core/ioctl/config"
	iotexapi "github.com/iotexproject/iotex-proto/golang/iotexapi"
)

// MockClient is a mock of Client interface.
type MockClient struct {
	ctrl     *gomock.Controller
	recorder *MockClientMockRecorder
}

// MockClientMockRecorder is the mock recorder for MockClient.
type MockClientMockRecorder struct {
	mock *MockClient
}

// NewMockClient creates a new mock instance.
func NewMockClient(ctrl *gomock.Controller) *MockClient {
	mock := &MockClient{ctrl: ctrl}
	mock.recorder = &MockClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockClient) EXPECT() *MockClientMockRecorder {
	return m.recorder
}

// APIServiceClient mocks base method.
func (m *MockClient) APIServiceClient(arg0 ioctl.APIServiceConfig) (iotexapi.APIServiceClient, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "APIServiceClient", arg0)
	ret0, _ := ret[0].(iotexapi.APIServiceClient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// APIServiceClient indicates an expected call of APIServiceClient.
func (mr *MockClientMockRecorder) APIServiceClient(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "APIServiceClient", reflect.TypeOf((*MockClient)(nil).APIServiceClient), arg0)
}

// Address mocks base method.
func (m *MockClient) Address(in string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Address", in)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Address indicates an expected call of Address.
func (mr *MockClientMockRecorder) Address(in interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Address", reflect.TypeOf((*MockClient)(nil).Address), in)
}

// AddressWithDefaultIfNotExist mocks base method.
func (m *MockClient) AddressWithDefaultIfNotExist(in string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddressWithDefaultIfNotExist", in)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddressWithDefaultIfNotExist indicates an expected call of AddressWithDefaultIfNotExist.
func (mr *MockClientMockRecorder) AddressWithDefaultIfNotExist(in interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddressWithDefaultIfNotExist", reflect.TypeOf((*MockClient)(nil).AddressWithDefaultIfNotExist), in)
}

// AliasMap mocks base method.
func (m *MockClient) AliasMap() map[string]string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AliasMap")
	ret0, _ := ret[0].(map[string]string)
	return ret0
}

// AliasMap indicates an expected call of AliasMap.
func (mr *MockClientMockRecorder) AliasMap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AliasMap", reflect.TypeOf((*MockClient)(nil).AliasMap))
}

// AskToConfirm mocks base method.
func (m *MockClient) AskToConfirm(arg0 string) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AskToConfirm", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// AskToConfirm indicates an expected call of AskToConfirm.
func (mr *MockClientMockRecorder) AskToConfirm(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AskToConfirm", reflect.TypeOf((*MockClient)(nil).AskToConfirm), arg0)
}

// Config mocks base method.
func (m *MockClient) Config() config.Config {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Config")
	ret0, _ := ret[0].(config.Config)
	return ret0
}

// Config indicates an expected call of Config.
func (mr *MockClientMockRecorder) Config() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Config", reflect.TypeOf((*MockClient)(nil).Config))
}

// DecryptPrivateKey mocks base method.
func (m *MockClient) DecryptPrivateKey(arg0, arg1 string) (*ecdsa.PrivateKey, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DecryptPrivateKey", arg0, arg1)
	ret0, _ := ret[0].(*ecdsa.PrivateKey)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DecryptPrivateKey indicates an expected call of DecryptPrivateKey.
func (mr *MockClientMockRecorder) DecryptPrivateKey(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DecryptPrivateKey", reflect.TypeOf((*MockClient)(nil).DecryptPrivateKey), arg0, arg1)
}

// DeleteAlias mocks base method.
func (m *MockClient) DeleteAlias(arg0 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteAlias", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteAlias indicates an expected call of DeleteAlias.
func (mr *MockClientMockRecorder) DeleteAlias(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteAlias", reflect.TypeOf((*MockClient)(nil).DeleteAlias), arg0)
}

// Execute mocks base method.
func (m *MockClient) Execute(arg0 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Execute", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Execute indicates an expected call of Execute.
func (mr *MockClientMockRecorder) Execute(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Execute", reflect.TypeOf((*MockClient)(nil).Execute), arg0)
}

// ExportHdwallet mocks base method.
func (m *MockClient) ExportHdwallet(arg0 string) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ExportHdwallet", arg0)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ExportHdwallet indicates an expected call of ExportHdwallet.
func (mr *MockClientMockRecorder) ExportHdwallet(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExportHdwallet", reflect.TypeOf((*MockClient)(nil).ExportHdwallet), arg0)
}

// IsCryptoSm2 mocks base method.
func (m *MockClient) IsCryptoSm2() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsCryptoSm2")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsCryptoSm2 indicates an expected call of IsCryptoSm2.
func (mr *MockClientMockRecorder) IsCryptoSm2() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsCryptoSm2", reflect.TypeOf((*MockClient)(nil).IsCryptoSm2))
}

// NewKeyStore mocks base method.
func (m *MockClient) NewKeyStore() *keystore.KeyStore {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewKeyStore")
	ret0, _ := ret[0].(*keystore.KeyStore)
	return ret0
}

// NewKeyStore indicates an expected call of NewKeyStore.
func (mr *MockClientMockRecorder) NewKeyStore() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewKeyStore", reflect.TypeOf((*MockClient)(nil).NewKeyStore))
}

// QueryAnalyser mocks base method.
func (m *MockClient) QueryAnalyser(arg0 interface{}) (*http.Response, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryAnalyser", arg0)
	ret0, _ := ret[0].(*http.Response)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryAnalyser indicates an expected call of QueryAnalyser.
func (mr *MockClientMockRecorder) QueryAnalyser(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryAnalyser", reflect.TypeOf((*MockClient)(nil).QueryAnalyser), arg0)
}

// ReadSecret mocks base method.
func (m *MockClient) ReadSecret() (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadSecret")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadSecret indicates an expected call of ReadSecret.
func (mr *MockClientMockRecorder) ReadSecret() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadSecret", reflect.TypeOf((*MockClient)(nil).ReadSecret))
}

// SelectTranslation mocks base method.
func (m *MockClient) SelectTranslation(arg0 map[config.Language]string) (string, config.Language) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SelectTranslation", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(config.Language)
	return ret0, ret1
}

// SelectTranslation indicates an expected call of SelectTranslation.
func (mr *MockClientMockRecorder) SelectTranslation(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SelectTranslation", reflect.TypeOf((*MockClient)(nil).SelectTranslation), arg0)
}

// SetAlias mocks base method.
func (m *MockClient) SetAlias(arg0, arg1 string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetAlias", arg0, arg1)
}

// SetAlias indicates an expected call of SetAlias.
func (mr *MockClientMockRecorder) SetAlias(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetAlias", reflect.TypeOf((*MockClient)(nil).SetAlias), arg0, arg1)
}

// SetAliasAndSave mocks base method.
func (m *MockClient) SetAliasAndSave(arg0, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetAliasAndSave", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetAliasAndSave indicates an expected call of SetAliasAndSave.
func (mr *MockClientMockRecorder) SetAliasAndSave(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetAliasAndSave", reflect.TypeOf((*MockClient)(nil).SetAliasAndSave), arg0, arg1)
}

// Start mocks base method.
func (m *MockClient) Start(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start.
func (mr *MockClientMockRecorder) Start(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockClient)(nil).Start), arg0)
}

// Stop mocks base method.
func (m *MockClient) Stop(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop.
func (mr *MockClientMockRecorder) Stop(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockClient)(nil).Stop), arg0)
}

// WriteConfig mocks base method.
func (m *MockClient) WriteConfig() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteConfig")
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteConfig indicates an expected call of WriteConfig.
func (mr *MockClientMockRecorder) WriteConfig() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteConfig", reflect.TypeOf((*MockClient)(nil).WriteConfig))
}
