// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"testing"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/flag"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/ioctl/util"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_ioctlclient"
)

func TestNewActionClaimCmd(t *testing.T) {
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
	client.EXPECT().Alias(gomock.Any()).Return("producer", nil).Times(2)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).AnyTimes()
	client.EXPECT().IsCryptoSm2().Return(false).Times(9)
	client.EXPECT().ReadSecret().Return("", nil).Times(3)
	client.EXPECT().Address(gomock.Any()).Return(accAddr.String(), nil).Times(3)
	client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(accAddr.String(), nil).Times(4)
	client.EXPECT().NewKeyStore().Return(ks).Times(6)
	client.EXPECT().AskToConfirm(gomock.Any()).Return(true, nil).Times(2)
	client.EXPECT().Config().Return(config.Config{
		Explorer: "iotexscan",
		Endpoint: "testnet1",
	}).AnyTimes()

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
	apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(accountResp, nil).Times(5)
	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResp, nil).Times(3)
	apiServiceClient.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return(sendActionResp, nil).Times(2)

	t.Run("action claim", func(t *testing.T) {
		cmd := NewActionClaimCmd(client)
		_, err := util.ExecuteCmd(cmd, "--amount", "0")
		require.Error(err)
		require.Contains(err.Error(), "expect amount greater than 0")
	})

	t.Run("action claim with address", func(t *testing.T) {
		address := "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he"
		cmd := NewActionClaimCmd(client)
		result, err := util.ExecuteCmd(cmd, "--amount", "1", "--address", address, "--gas-limit", "0")
		require.NoError(err)
		require.Contains(result, "Action has been sent to blockchain")
	})

	t.Run("action claim with invalid address", func(t *testing.T) {
		addr := "0x123"
		cmd := NewActionClaimCmd(client)
		_, err := util.ExecuteCmd(cmd, "--amount", "1", "--address", addr, "--gas-limit", "0")
		require.Equal(address.ErrInvalidAddr, errors.Cause(err))
	})

	t.Run("action claim with payload", func(t *testing.T) {
		payload := "0a10080118a08d062202313062040a023130124104dc4c548c3a478278a6a09ffa8b5c4b384368e49654b35a6961ee8288fc889cdc39e9f8194e41abdbfac248ef9dc3f37b131a36ee2c052d974c21c1d2cd56730b1a4161e219c2c5d5987f8a9efa33e8df0cde9d5541689fff05784cdc24f12e9d9ee8283a5aa720f494b949535b7969c07633dfb68c4ef9359eb16edb9abc6ebfadc801"
		cmd := NewActionClaimCmd(client)
		result, err := util.ExecuteCmd(cmd,
			"--amount", "1",
			"--address", "",
			"--payload", payload,
			"--gas-limit", "0",
		)
		require.NoError(err)
		require.Contains(result, "Action has been sent to blockchain")
	})

	t.Run("failed to get nonce", func(t *testing.T) {
		expectedErr := errors.New("failed to get nonce")
		apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil, expectedErr)
		cmd := NewActionClaimCmd(client)
		_, err := util.ExecuteCmd(cmd, "--amount", "1")
		require.Error(err)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to get signer address", func(t *testing.T) {
		expectedErr := errors.New("failed to get signer address")
		client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return("", expectedErr)
		cmd := NewActionClaimCmd(client)
		_, err := util.ExecuteCmd(cmd, "--amount", "1")
		require.Error(err)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("invalid amount", func(t *testing.T) {
		expectedErr := errors.New(`required flag(s) "amount" not set`)
		claimAmount = flag.NewStringVarP("amount", "", "invalid", "")
		cmd := NewActionClaimCmd(client)
		_, err := util.ExecuteCmd(cmd, "")
		require.Error(err)
		require.Contains(err.Error(), expectedErr.Error())
	})
}
