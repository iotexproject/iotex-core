// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

func TestNewAccountBalance(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).AnyTimes()

	accAddr := identityset.Address(28)
	client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(accAddr.String(), nil).AnyTimes()
	client.EXPECT().Config().Return(config.Config{}).AnyTimes()

	apiServiceClient := mock_apiserviceclient.NewMockServiceClient(ctrl)
	accountResponse := &iotexapi.GetAccountResponse{AccountMeta: &iotextypes.AccountMeta{
		Address:          accAddr.String(),
		Balance:          "20000000132432000",
		Nonce:            uint64(0),
		PendingNonce:     uint64(1),
		NumActions:       uint64(2),
		IsContract:       true,
		ContractByteCode: []byte("60806040526101f4600055603260015534801561001b57600080fd5b506102558061002b6000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806358931c461461003b5780637f353d5514610045575b600080fd5b61004361004f565b005b61004d610097565b005b60006001905060005b6000548110156100935760028261006f9190610114565b915060028261007e91906100e3565b9150808061008b90610178565b915050610058565b5050565b60005b6001548110156100e057600281908060018154018082558091505060019003906000526020600020016000909190919091505580806100d890610178565b91505061009a565b50565b60006100ee8261016e565b91506100f98361016e565b925082610109576101086101f0565b5b828204905092915050565b600061011f8261016e565b915061012a8361016e565b9250817fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0483118215151615610163576101626101c1565b5b828202905092915050565b6000819050919050565b60006101838261016e565b91507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8214156101b6576101b56101c1565b5b600182019050919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601260045260246000fdfea2646970667358221220cb9cada3f1d447c978af17aa3529d6fe4f25f9c5a174085443e371b6940ae99b64736f6c63430008070033"),
	}}
	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil).AnyTimes()
	apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(accountResponse, nil).AnyTimes()

	t.Run("get account balance", func(t *testing.T) {
		cmd := NewAccountBalance(client)
		result, err := util.ExecuteCmd(cmd, accAddr.String())
		require.NoError(err)
		require.Contains(result, "0.020000000132432")
	})

	t.Run("get balance with empty address", func(t *testing.T) {
		accAddr := ""

		cmd := NewAccountBalance(client)
		result, err := util.ExecuteCmd(cmd, accAddr)
		require.NoError(err)
		require.Contains(result, "0.020000000132432")
	})

	t.Run("get balance with StakingBucketPoolAddr", func(t *testing.T) {
		accAddr := "io000000000000000000000000stakingprotocol"

		cmd := NewAccountBalance(client)
		result, err := util.ExecuteCmd(cmd, accAddr)
		require.NoError(err)
		require.Contains(result, "0.020000000132432")
	})

	t.Run("get balance with RewardingPoolAddr", func(t *testing.T) {
		accAddr := "io0000000000000000000000rewardingprotocol"

		cmd := NewAccountBalance(client)
		result, err := util.ExecuteCmd(cmd, accAddr)
		require.NoError(err)
		require.Contains(result, "0.020000000132432")
	})
}
