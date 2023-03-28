// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.
package action

import (
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewActionTransferCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)

	ks := keystore.NewKeyStore(t.TempDir(), 2, 1)
	acc, err := ks.NewAccount("")
	require.NoError(err)
	accAddr, err := address.FromBytes(acc.Address.Bytes())
	require.NoError(err)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).AnyTimes()
	client.EXPECT().Alias(gomock.Any()).Return("producer", nil).Times(10)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).AnyTimes()
	client.EXPECT().IsCryptoSm2().Return(false).AnyTimes()
	client.EXPECT().ReadSecret().Return("", nil).AnyTimes()
	client.EXPECT().Address(gomock.Any()).Return(accAddr.String(), nil).Times(17)
	client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(accAddr.String(), nil).Times(7)
	client.EXPECT().NewKeyStore().Return(ks).AnyTimes()
	client.EXPECT().AskToConfirm(gomock.Any()).Return(true, nil).Times(5)
	client.EXPECT().Config().Return(config.Config{
		Explorer: "iotexscan",
		Endpoint: "testnet1",
	}).Times(10)

	accountResp := &iotexapi.GetAccountResponse{
		AccountMeta: &iotextypes.AccountMeta{
			IsContract:   false,
			PendingNonce: 10,
			Balance:      "100000000000000000000",
		},
	}
	chainMetaResp := &iotexapi.GetChainMetaResponse{
		ChainMeta: &iotextypes.ChainMeta{
			ChainID: 0,
		},
	}
	sendActionResp := &iotexapi.SendActionResponse{}
	apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(accountResp, nil).Times(18)
	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResp, nil).AnyTimes()
	apiServiceClient.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return(sendActionResp, nil).Times(5)

	t.Run("action transfer", func(t *testing.T) {
		cmd := NewActionTransferCmd(client)
		_, err = util.ExecuteCmd(cmd, accAddr.String(), "10", "--signer", accAddr.String())
		require.NoError(err)
	})

	t.Run("invalid amount", func(t *testing.T) {
		expectedErr := errors.New("invalid amount")
		cmd := NewActionTransferCmd(client)
		_, err = util.ExecuteCmd(cmd, accAddr.String(), "10.00.00", "--signer", accAddr.String())
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("action transfer with payload", func(t *testing.T) {
		payload := "0a10080118a08d062202313062040a023130124104dc4c548c3a478278a6a09ffa8b5c4b384368e49654b35a6961ee8288fc889cdc39e9f8194e41abdbfac248ef9dc3f37b131a36ee2c052d974c21c1d2cd56730b1a4161e219c2c5d5987f8a9efa33e8df0cde9d5541689fff05784cdc24f12e9d9ee8283a5aa720f494b949535b7969c07633dfb68c4ef9359eb16edb9abc6ebfadc801"
		cmd := NewActionTransferCmd(client)
		_, err = util.ExecuteCmd(cmd, accAddr.String(), "10", payload, "--signer", accAddr.String())
		require.NoError(err)
	})

	t.Run("failed to decode data", func(t *testing.T) {
		expectedErr := errors.New("failed to decode data")

		cmd := NewActionTransferCmd(client)
		_, err = util.ExecuteCmd(cmd, accAddr.String(), "10", "test", "--signer", accAddr.String())
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("zero gas limit", func(t *testing.T) {
		payload := "0a10080118a08d062202313062040a023130124104dc4c548c3a478278a6a09ffa8b5c4b384368e49654b35a6961ee8288fc889cdc39e9f8194e41abdbfac248ef9dc3f37b131a36ee2c052d974c21c1d2cd56730b1a4161e219c2c5d5987f8a9efa33e8df0cde9d5541689fff05784cdc24f12e9d9ee8283a5aa720f494b949535b7969c07633dfb68c4ef9359eb16edb9abc6ebfadc801"

		cmd := NewActionTransferCmd(client)
		_, err = util.ExecuteCmd(cmd, accAddr.String(), "10", payload, "--signer", accAddr.String(), "--gas-limit", "0")
		require.NoError(err)
	})

	t.Run("action transfer with zero gas price", func(t *testing.T) {
		apiServiceClient.EXPECT().SuggestGasPrice(gomock.Any(), gomock.Any()).Return(&iotexapi.SuggestGasPriceResponse{
			GasPrice: 10,
		}, nil)

		cmd := NewActionTransferCmd(client)
		result, err := util.ExecuteCmd(cmd, accAddr.String(), "10", "--signer", accAddr.String(), "--gas-price", "")
		require.NoError(err)
		require.Contains(result, "Action has been sent to blockchain.\nWait for several seconds and query this action by hash")
	})

	t.Run("failed to get gas price", func(t *testing.T) {
		expectedErr := errors.New("failed to get gas price")
		apiServiceClient.EXPECT().SuggestGasPrice(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		cmd := NewActionTransferCmd(client)
		_, err = util.ExecuteCmd(cmd, accAddr.String(), "10", "--signer", accAddr.String(), "--gas-price", "")
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("action transfer with nonce", func(t *testing.T) {
		cmd := NewActionTransferCmd(client)
		result, err := util.ExecuteCmd(cmd, accAddr.String(), "10", "--signer", accAddr.String(), "--nonce", "10")
		require.NoError(err)
		require.Contains(result, "Action has been sent to blockchain.\nWait for several seconds and query this action by hash")
	})

	t.Run("failed to get account meta", func(t *testing.T) {
		expectedErr := errors.New("failed to get account meta")
		apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		cmd := NewActionTransferCmd(client)
		_, err = util.ExecuteCmd(cmd, accAddr.String(), "10", "--signer", accAddr.String())
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("use 'ioctl contract' command instead", func(t *testing.T) {
		expectedErr := errors.New("use 'ioctl contract' command instead")
		accountResp := &iotexapi.GetAccountResponse{
			AccountMeta: &iotextypes.AccountMeta{
				IsContract: true,
			},
		}
		apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(accountResp, nil)

		cmd := NewActionTransferCmd(client)
		_, err = util.ExecuteCmd(cmd, accAddr.String(), "10", "--signer", accAddr.String())
		require.Equal(expectedErr.Error(), err.Error())
	})

	t.Run("failed to get nonce", func(t *testing.T) {
		expectedErr := errors.New("failed to get nonce")
		apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		cmd := NewActionTransferCmd(client)
		_, err = util.ExecuteCmd(cmd, accAddr.String(), "10", "--signer", accAddr.String())
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to get signed address", func(t *testing.T) {
		expectedErr := errors.New("failed to get signed address")
		client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return("", expectedErr)
		apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(accountResp, nil)

		cmd := NewActionTransferCmd(client)
		_, err = util.ExecuteCmd(cmd, accAddr.String(), "10", "--signer", accAddr.String())
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to get recipient address", func(t *testing.T) {
		expectedErr := errors.New("failed to get recipient address")
		client.EXPECT().Address(gomock.Any()).Return("", expectedErr)

		cmd := NewActionTransferCmd(client)
		_, err = util.ExecuteCmd(cmd, "0", "10", "--signer", accAddr.String())
		require.Contains(err.Error(), expectedErr.Error())
	})
}
