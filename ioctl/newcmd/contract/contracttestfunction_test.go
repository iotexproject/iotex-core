// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contract

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/stretchr/testify/require"
)

func TestNewContractTestFunctionCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("test", config.English).AnyTimes()
	client.EXPECT().Address(gomock.Any()).DoAndReturn(func(addr string) (string, error) {
		_, err := address.FromString(addr)
		return addr, err
	}).AnyTimes()
	client.EXPECT().APIServiceClient().Return(apiClient, nil).AnyTimes()
	client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return("", nil).AnyTimes()
	apiClient.EXPECT().ReadContract(gomock.Any(), gomock.Any(), gomock.Any()).Return(&iotexapi.ReadContractResponse{}, nil).AnyTimes()

	t.Run("test_pass", func(t *testing.T) {
		cmd := NewContractTestFunctionCmd(client)
		result, err := util.ExecuteCmd(cmd, "--with-arguments", `{"recipients":["io1h8zxmdacge966wp6t90a02ncghaa6eptnftfqr","io14fmlh7zedcx7tn3k9k744v54nxnv8zky86tjhj"],"amounts":["3123132","123"],"payload":"PLEASE!!!"}`, "io1h8zxmdacge966wp6t90a02ncghaa6eptnftfqr", "test.abi", "multiSend", "3.4")
		require.NoError(err)
		require.Contains(result, "return")
	})

	t.Run("invalid_address", func(t *testing.T) {
		cmd := NewContractTestFunctionCmd(client)
		_, err := util.ExecuteCmd(cmd, "--with-arguments", `{"recipients":["io1h8zxmdacge966wp6t90a02ncghaa6eptnftfqr","io14fmlh7zedcx7tn3k9k744v54nxnv8zky86tjhj"],"amounts":["3123132","123"],"payload":"PLEASE!!!"}`, "123", "test.abi", "multiSend")
		require.Contains(err.Error(), "invalid address")
	})

	t.Run("abifile_not_exist", func(t *testing.T) {
		cmd := NewContractTestFunctionCmd(client)
		_, err := util.ExecuteCmd(cmd, "--with-arguments", `{"recipients":["io1h8zxmdacge966wp6t90a02ncghaa6eptnftfqr","io14fmlh7zedcx7tn3k9k744v54nxnv8zky86tjhj"],"amounts":["3123132","123"],"payload":"PLEASE!!!"}`, "io1h8zxmdacge966wp6t90a02ncghaa6eptnftfqr", "not_exist.abi", "multiSend")
		require.Contains(err.Error(), "no such file")
	})

	t.Run("invalid_method", func(t *testing.T) {
		cmd := NewContractTestFunctionCmd(client)
		_, err := util.ExecuteCmd(cmd, "--with-arguments", `{"recipients":["io1h8zxmdacge966wp6t90a02ncghaa6eptnftfqr","io14fmlh7zedcx7tn3k9k744v54nxnv8zky86tjhj"],"amounts":["3123132","123"],"payload":"PLEASE!!!"}`, "io1h8zxmdacge966wp6t90a02ncghaa6eptnftfqr", "test.abi", "multiSend1")
		require.Contains(err.Error(), "invalid method name")
	})

	t.Run("invalid_amount", func(t *testing.T) {
		cmd := NewContractTestFunctionCmd(client)
		_, err := util.ExecuteCmd(cmd, "--with-arguments", `{"recipients":["io1h8zxmdacge966wp6t90a02ncghaa6eptnftfqr","io14fmlh7zedcx7tn3k9k744v54nxnv8zky86tjhj"],"amounts":["3123132","123"],"payload":"PLEASE!!!"}`, "io1h8zxmdacge966wp6t90a02ncghaa6eptnftfqr", "test.abi", "multiSend", "amount")
		require.Contains(err.Error(), "invalid amount")
	})

	t.Run("invalid_argument", func(t *testing.T) {
		cmd := NewContractTestFunctionCmd(client)
		_, err := util.ExecuteCmd(cmd, "--with-arguments", `{"recipients":"asd"}`, "io1h8zxmdacge966wp6t90a02ncghaa6eptnftfqr", "test.abi", "multiSend", "3.4")
		require.Contains(err.Error(), "invalid argument")
	})
}
