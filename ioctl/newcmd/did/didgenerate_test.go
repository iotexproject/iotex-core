// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

import (
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewDidGenerateCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	passwd := "123456"

	ks := keystore.NewKeyStore(t.TempDir(), 2, 1)
	acc, err := ks.NewAccount(passwd)
	require.NoError(err)
	accAddr, err := address.FromBytes(acc.Address.Bytes())
	require.NoError(err)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("did", config.English).AnyTimes()
	client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(accAddr.String(), nil).Times(3)

	t.Run("failed to get password", func(t *testing.T) {
		expectedErr := errors.New("failed to get password")
		client.EXPECT().ReadSecret().Return("", expectedErr)
		cmd := NewDidGenerateCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.Contains(err.Error(), expectedErr.Error())
	})

	client.EXPECT().ReadSecret().Return(passwd, nil).Times(2)
	client.EXPECT().IsCryptoSm2().Return(false).Times(3)
	client.EXPECT().NewKeyStore().Return(ks).Times(3)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).AnyTimes()

	t.Run("generate did", func(t *testing.T) {
		client.EXPECT().Address(gomock.Any()).Return(accAddr.String(), nil)
		cmd := NewDidGenerateCmd(client)
		result, err := util.ExecuteCmd(cmd, "--signer", accAddr.String())
		require.NoError(err)
		require.Contains(result, acc.Address.Hex())
	})

	t.Run("failed to get private key from signer", func(t *testing.T) {
		expectedErr := errors.New("failed to get private key from signer")
		client.EXPECT().Address(gomock.Any()).Return("", expectedErr)
		cmd := NewDidGenerateCmd(client)
		_, err := util.ExecuteCmd(cmd, "--signer", accAddr.String())
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to get signer addr", func(t *testing.T) {
		expectedErr := errors.New("failed to get signer addr")
		client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return("", expectedErr)
		cmd := NewDidGenerateCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.Contains(err.Error(), expectedErr.Error())
	})
}
