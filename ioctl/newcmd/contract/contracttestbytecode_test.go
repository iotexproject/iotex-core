// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contract

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_ioctlclient"
)

func TestNewContractTestBytecodeCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	addr := identityset.Address(0).String()

	ks := keystore.NewKeyStore(t.TempDir(), 2, 1)
	acc, err := ks.NewAccount("")
	require.NoError(err)
	accAddr, err := address.FromBytes(acc.Address.Bytes())
	require.NoError(err)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("contract", config.English).Times(40)
	client.EXPECT().SetEndpointWithFlag(gomock.Any()).AnyTimes()
	client.EXPECT().SetInsecureWithFlag(gomock.Any()).AnyTimes()
	client.EXPECT().Alias(gomock.Any()).Return("producer", nil).AnyTimes()
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).AnyTimes()
	client.EXPECT().IsCryptoSm2().Return(false).AnyTimes()
	client.EXPECT().ReadSecret().Return("", nil).AnyTimes()
	client.EXPECT().Address(gomock.Any()).Return(accAddr.String(), nil).Times(4)
	client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(accAddr.String(), nil).AnyTimes()
	client.EXPECT().NewKeyStore().Return(ks).AnyTimes()
	client.EXPECT().AskToConfirm(gomock.Any()).Return(true, nil).AnyTimes()
	client.EXPECT().Config().Return(config.Config{
		Explorer: "iotexscan",
		Endpoint: "testnet1",
	}).AnyTimes()

	apiServiceClient.EXPECT().ReadContract(gomock.Any(), gomock.Any()).Return(&iotexapi.ReadContractResponse{
		Data: hex.EncodeToString([]byte("60fe47b100000000000000000000000000000000000000000000000000000000")),
	}, nil)

	t.Run("compile contract", func(t *testing.T) {
		cmd := NewContractTestBytecodeCmd(client)
		result, err := util.ExecuteCmd(cmd, addr, "a9059cbb0000000000000000000000004867c4bada9553216bf296c4c64e9ff0749206490000000000000000000000000000000000000000000000000000000000000001")
		require.NoError(err)
		require.Contains(result, "return")
	})

	t.Run("failed to read contract", func(t *testing.T) {
		expectedErr := errors.New("failed to read contract")
		apiServiceClient.EXPECT().ReadContract(gomock.Any(), gomock.Any()).Return(nil, expectedErr)
		cmd := NewContractTestBytecodeCmd(client)
		_, err := util.ExecuteCmd(cmd, addr, "a9059cbb0000000000000000000000004867c4bada9553216bf296c4c64e9ff0749206490000000000000000000000000000000000000000000000000000000000000001")
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("invalid amount", func(t *testing.T) {
		expectedErr := errors.New("invalid amount")
		cmd := NewContractTestBytecodeCmd(client)
		_, err := util.ExecuteCmd(cmd, addr, "a9059cbb0000000000000000000000004867c4bada9553216bf296c4c64e9ff0749206490000000000000000000000000000000000000000000000000000000000000001", "test")
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("invalid bytecode", func(t *testing.T) {
		expectedErr := errors.New("invalid bytecode")
		cmd := NewContractTestBytecodeCmd(client)
		_, err := util.ExecuteCmd(cmd, addr, "test")
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to get contract address", func(t *testing.T) {
		expectedErr := errors.New("failed to get contract address")
		client.EXPECT().Address(gomock.Any()).Return("", expectedErr)
		cmd := NewContractTestBytecodeCmd(client)
		_, err := util.ExecuteCmd(cmd, "test", "")
		require.Contains(err.Error(), expectedErr.Error())
	})
}
