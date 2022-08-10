// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestSigner(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).AnyTimes()
	client.EXPECT().SetEndpointWithFlag(gomock.Any()).Do(func(_ func(*string, string, string, string)) {})
	client.EXPECT().SetInsecureWithFlag(gomock.Any()).Do(func(_ func(*bool, string, bool, string)) {})

	t.Run("returns signer's address", func(t *testing.T) {
		client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return("test", nil).AnyTimes()

		cmd := NewActionCmd(client)
		RegisterWriteCommand(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--signer", "test")
		require.NoError(err)
		result, err := Signer(client, cmd)
		require.NoError(err)
		require.Equal(result, "test")
	})
}

func TestSendRaw(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	selp := &iotextypes.Action{}

	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).AnyTimes()
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(7)

	for _, test := range []struct {
		endpoint string
		insecure bool
	}{
		{
			endpoint: "111:222:333:444:5678",
			insecure: false,
		},
		{
			endpoint: "",
			insecure: true,
		},
	} {
		callbackEndpoint := func(cb func(*string, string, string, string)) {
			cb(&test.endpoint, "endpoint", test.endpoint, "endpoint usage")
		}
		callbackInsecure := func(cb func(*bool, string, bool, string)) {
			cb(&test.insecure, "insecure", !test.insecure, "insecure usage")
		}
		client.EXPECT().SetEndpointWithFlag(gomock.Any()).Do(callbackEndpoint).Times(3)
		client.EXPECT().SetInsecureWithFlag(gomock.Any()).Do(callbackInsecure).Times(3)

		t.Run("sends raw action to blockchain", func(t *testing.T) {
			response := &iotexapi.SendActionResponse{}

			apiServiceClient.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return(response, nil).Times(3)

			cmd := NewActionCmd(client)
			_, err := util.ExecuteCmd(cmd)
			require.NoError(err)

			t.Run("endpoint iotexscan", func(t *testing.T) {
				client.EXPECT().Config().Return(config.Config{
					Explorer: "iotexscan",
					Endpoint: "testnet1",
				}).Times(2)

				err = SendRaw(client, cmd, selp)
				require.NoError(err)
			})

			t.Run("endpoint iotxplorer", func(t *testing.T) {
				client.EXPECT().Config().Return(config.Config{
					Explorer: "iotxplorer",
				}).Times(2)

				err := SendRaw(client, cmd, selp)
				require.NoError(err)
			})

			t.Run("endpoint default", func(t *testing.T) {
				client.EXPECT().Config().Return(config.Config{
					Explorer: "test",
				}).Times(2)

				err := SendRaw(client, cmd, selp)
				require.NoError(err)
			})
		})
	}

	t.Run("failed to invoke SendAction api", func(t *testing.T) {
		expectedErr := errors.New("failed to invoke SendAction api")

		apiServiceClient.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		cmd := NewActionCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		err = SendRaw(client, cmd, selp)
		require.Contains(err.Error(), expectedErr.Error())
	})
}

func TestSendAction(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	passwd := "123456"

	ks := keystore.NewKeyStore(t.TempDir(), 2, 1)
	acc, err := ks.NewAccount(passwd)
	require.NoError(err)
	accAddr, err := address.FromBytes(acc.Address.Bytes())
	require.NoError(err)

	chainMetaResponse := &iotexapi.GetChainMetaResponse{ChainMeta: &iotextypes.ChainMeta{}}
	elp := createEnvelope(0)
	cost, err := elp.Cost()
	require.NoError(err)
	accountResponse := &iotexapi.GetAccountResponse{AccountMeta: &iotextypes.AccountMeta{
		Address:      accAddr.String(),
		Nonce:        1,
		PendingNonce: 1,
		Balance:      cost.String(),
	}}

	client.EXPECT().SelectTranslation(gomock.Any()).Return("action", config.English).AnyTimes()
	client.EXPECT().SetEndpointWithFlag(gomock.Any()).Do(func(_ func(*string, string, string, string)) {}).AnyTimes()
	client.EXPECT().SetInsecureWithFlag(gomock.Any()).Do(func(_ func(*bool, string, bool, string)) {}).AnyTimes()
	client.EXPECT().IsCryptoSm2().Return(false).Times(15)
	client.EXPECT().NewKeyStore().Return(ks).Times(15)

	t.Run("failed to get privateKey", func(t *testing.T) {
		expectedErr := errors.New("failed to get privateKey")
		client.EXPECT().ReadSecret().Return("", expectedErr).Times(1)

		cmd := NewActionCmd(client)
		RegisterWriteCommand(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--password", "")
		require.NoError(err)
		err = SendAction(client, cmd, elp, accAddr.String())
		require.Contains(err.Error(), expectedErr.Error())
	})

	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).AnyTimes()
	client.EXPECT().ReadSecret().Return(passwd, nil).Times(1)
	client.EXPECT().Address(gomock.Any()).Return(accAddr.String(), nil).Times(7)
	client.EXPECT().Alias(gomock.Any()).Return("producer", nil).Times(8)
	client.EXPECT().ReadInput().Return("confirm", nil)
	client.EXPECT().AskToConfirm(gomock.Any()).Return(true, nil).Times(2)
	client.EXPECT().Config().Return(config.Config{
		Explorer: "iotexscan",
		Endpoint: "testnet1",
	}).Times(11)

	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil).Times(6)
	apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(accountResponse, nil).Times(5)
	apiServiceClient.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return(&iotexapi.SendActionResponse{}, nil).Times(3)

	t.Run("sends signed action to blockchain", func(t *testing.T) {
		cmd := NewActionCmd(client)
		RegisterWriteCommand(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--password", passwd)
		require.NoError(err)
		err = SendAction(client, cmd, elp, accAddr.String())
		require.NoError(err)
	})

	t.Run("send action with nonce", func(t *testing.T) {
		mnemonic := "lake stove quarter shove dry matrix hire split wide attract argue core"
		client.EXPECT().HdwalletMnemonic(gomock.Any()).Return(mnemonic, nil)

		cmd := NewActionCmd(client)
		RegisterWriteCommand(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--password", passwd)
		require.NoError(err)
		err = SendAction(client, cmd, elp, "hdw::1/2")
		require.NoError(err)
	})

	t.Run("quit action command", func(t *testing.T) {
		client.EXPECT().AskToConfirm(gomock.Any()).Return(false, nil)

		cmd := NewActionCmd(client)
		RegisterWriteCommand(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--password", passwd)
		require.NoError(err)
		err = SendAction(client, cmd, elp, accAddr.String())
		require.NoError(err)
	})

	t.Run("failed to ask confirm", func(t *testing.T) {
		expectedErr := errors.New("failed to ask confirm")
		client.EXPECT().AskToConfirm(gomock.Any()).Return(false, expectedErr)

		cmd := NewActionCmd(client)
		RegisterWriteCommand(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--password", passwd)
		require.NoError(err)
		err = SendAction(client, cmd, elp, accAddr.String())
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to pass balance check", func(t *testing.T) {
		expectedErr := errors.New("failed to pass balance check")
		apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		cmd := NewActionCmd(client)
		RegisterWriteCommand(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--password", passwd)
		require.NoError(err)
		err = SendAction(client, cmd, elp, accAddr.String())
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to get nonce", func(t *testing.T) {
		mnemonic := "lake stove quarter shove dry matrix hire split wide attract argue core"
		expectedErr := errors.New("failed to get nonce")
		client.EXPECT().HdwalletMnemonic(gomock.Any()).Return(mnemonic, nil)
		apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		cmd := NewActionCmd(client)
		RegisterWriteCommand(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--password", passwd)
		require.NoError(err)
		err = SendAction(client, cmd, elp, "hdw::1/2")
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to get chain meta", func(t *testing.T) {
		expectedErr := errors.New("failed to get chain meta")
		apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		cmd := NewActionCmd(client)
		RegisterWriteCommand(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--password", passwd)
		require.NoError(err)
		err = SendAction(client, cmd, elp, accAddr.String())
		require.Contains(err.Error(), expectedErr.Error())
	})
}
