// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"os"
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
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	veryLightScryptN = 2
	veryLightScryptP = 1
)

func TestSigner(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)

	t.Run("returns signer's address", func(t *testing.T) {
		client.EXPECT().Config().Return(config.Config{
			DefaultAccount: config.Context{
				AddressOrAlias: "test",
			},
		})
		client.EXPECT().Address(gomock.Any()).Return("test", nil)

		result, err := Signer(client)
		require.NoError(err)
		require.Equal(result, "test")
	})

	t.Run("use 'ioctl config set defaultacc ADDRESS|ALIAS' to config default account first", func(t *testing.T) {
		expectedErr := errors.New("use 'ioctl config set defaultacc ADDRESS|ALIAS' to config default account first")

		client.EXPECT().Config().Return(config.Config{})

		_, err := Signer(client)
		require.Equal(err.Error(), expectedErr.Error())
	})
}

func TestSendRaw(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	selp := &iotextypes.Action{}
	response := &iotexapi.SendActionResponse{}

	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(4)
	apiServiceClient.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return(response, nil).Times(3)

	t.Run("sends raw action to blockchain", func(t *testing.T) {
		t.Run("endpoint iotexscan", func(t *testing.T) {
			client.EXPECT().Config().Return(config.Config{
				Explorer: "iotexscan",
				Endpoint: "testnet1",
			}).Times(2)

			err := SendRaw(client, selp)
			require.NoError(err)
		})

		t.Run("endpoint iotxplorer", func(t *testing.T) {
			client.EXPECT().Config().Return(config.Config{
				Explorer: "iotxplorer",
			})

			err := SendRaw(client, selp)
			require.NoError(err)
		})

		t.Run("endpoint default", func(t *testing.T) {
			client.EXPECT().Config().Return(config.Config{
				Explorer: "test",
			}).Times(2)

			err := SendRaw(client, selp)
			require.NoError(err)
		})
	})

	t.Run("failed to invoke SendAction api", func(t *testing.T) {
		expectedErr := errors.New("failed to invoke SendAction api")
		apiServiceClient.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		err := SendRaw(client, selp)
		require.Equal(err.Error(), expectedErr.Error())
	})
}

func TestSendAction(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).Times(2)
	client.EXPECT().SetEndpointWithFlag(gomock.Any()).Do(func(_ func(*string, string, string, string)) {})
	client.EXPECT().SetInsecureWithFlag(gomock.Any()).Do(func(_ func(*bool, string, bool, string)) {})
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(3)
	apiServiceClient.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return(&iotexapi.SendActionResponse{}, nil)

	client.EXPECT().IsCryptoSm2().Return(false).Times(2)
	testWallet, err := os.MkdirTemp(os.TempDir(), "testWallet")
	require.NoError(err)
	defer testutil.CleanupPath(testWallet)
	ks := keystore.NewKeyStore(testWallet, veryLightScryptN, veryLightScryptP)
	client.EXPECT().NewKeyStore().Return(ks).Times(2)

	passwd := "123456"
	client.EXPECT().ReadSecret().Return(passwd, nil).Times(2)
	account, err := ks.NewAccount(passwd)
	require.NoError(err)
	addr, err := address.FromBytes(account.Address.Bytes())
	require.NoError(err)
	client.EXPECT().Address(gomock.Any()).Return(addr.String(), nil)

	chainMetaResponse := &iotexapi.GetChainMetaResponse{ChainMeta: &iotextypes.ChainMeta{}}
	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil)

	elp := createEnvelope(0)
	cost, err := elp.Cost()
	require.NoError(err)
	accountResponse := &iotexapi.GetAccountResponse{AccountMeta: &iotextypes.AccountMeta{
		Address:      addr.String(),
		Nonce:        1,
		PendingNonce: 1,
		Balance:      cost.String(),
	}}
	apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(accountResponse, nil)

	client.EXPECT().Alias(gomock.Any()).Return("producer", nil).Times(2)
	client.EXPECT().ReadInput().Return("confirm", nil)
	client.EXPECT().AskToConfirm(gomock.Any()).Return(false)

	t.Run("sends signed action to blockchain", func(t *testing.T) {
		cmd := NewActionCmd(client)
		err := SendAction(client, cmd, elp, addr.String())
		require.NoError(err)
	})
}

func TestRead(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	contractAddr := identityset.Address(28)

	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(2)
	client.EXPECT().Config().Return(config.Config{
		DefaultAccount: config.Context{
			AddressOrAlias: "test",
		},
	}).Times(2)
	client.EXPECT().Address(gomock.Any()).Return("test", nil).Times(2)

	t.Run("reads smart contract on IoTeX blockchain", func(t *testing.T) {
		response := &iotexapi.ReadContractResponse{
			Data: "test",
		}
		apiServiceClient.EXPECT().ReadContract(gomock.Any(), gomock.Any()).Return(response, nil)

		result, err := Read(client, contractAddr, "0", []byte("0x1bfc56c600000000000000000000000000000000000000000000000000000000000000"))
		require.NoError(err)
		require.Equal(result, "test")
	})

	t.Run("failed to invoke ReadContract api", func(t *testing.T) {
		expectedErr := errors.New("failed to invoke ReadContract api")

		apiServiceClient.EXPECT().ReadContract(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		_, err := Read(client, contractAddr, "0", []byte("0x1bfc56c600000000000000000000000000000000000000000000000000000000000000"))
		require.Equal(err.Error(), expectedErr.Error())
	})
}
