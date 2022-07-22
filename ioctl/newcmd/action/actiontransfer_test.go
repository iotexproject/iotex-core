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
	accountResp := &iotexapi.GetAccountResponse{
		AccountMeta: &iotextypes.AccountMeta{
			IsContract:   false,
			PendingNonce: 10,
			Balance:      "100000000000000000000",
		},
	}
	gasPriceResp := &iotexapi.SuggestGasPriceResponse{
		GasPrice: 10,
	}
	chainMetaResp := &iotexapi.GetChainMetaResponse{
		ChainMeta: &iotextypes.ChainMeta{
			ChainID: 0,
		},
	}
	sendActionResp := &iotexapi.SendActionResponse{}

	ks := keystore.NewKeyStore(t.TempDir(), 2, 1)
	acc, err := ks.NewAccount("")
	require.NoError(err)
	accAddr, err := address.FromBytes(acc.Address.Bytes())
	require.NoError(err)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).AnyTimes()
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).AnyTimes()
	client.EXPECT().IsCryptoSm2().Return(false).Times(3)
	client.EXPECT().ReadSecret().Return("", nil).Times(2)
	client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(accAddr.String(), nil)
	client.EXPECT().Address(gomock.Any()).Return(accAddr.String(), nil).Times(2)
	client.EXPECT().NewKeyStore().Return(ks).Times(2)
	client.EXPECT().AskToConfirm(gomock.Any()).Return(true, nil)
	client.EXPECT().Config().Return(config.Config{
		Explorer: "iotexscan",
		Endpoint: "testnet1",
	}).Times(2)

	apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(accountResp, nil).AnyTimes()
	apiServiceClient.EXPECT().SuggestGasPrice(gomock.Any(), gomock.Any()).Return(gasPriceResp, nil)
	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResp, nil)
	apiServiceClient.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return(sendActionResp, nil)

	cmd := NewActionTransferCmd(client)
	RegisterWriteCommand(client, cmd)
	_, err = util.ExecuteCmd(cmd, accAddr.String(), "10", "--signer", accAddr.String())
	require.NoError(err)
}
