// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestNewAccountSign(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).AnyTimes()

	testAccountFolder, err := os.MkdirTemp(os.TempDir(), "testNewAccountSign")
	require.NoError(err)
	defer testutil.CleanupPath(testAccountFolder)
	ks := keystore.NewKeyStore(testAccountFolder, veryLightScryptN, veryLightScryptP)
	client.EXPECT().NewKeyStore().Return(ks).AnyTimes()

	t.Run("invalid_account", func(t *testing.T) {
		client.EXPECT().IsCryptoSm2().Return(false).Times(2)
		client.EXPECT().Address(gomock.Any()).Return("io1rc2d2de7rtuucalsqv4d9ng0h297t63w7wvlph", nil).Times(1)
		cmd := NewAccountSign(client)
		require.NoError(cmd.Flag("signer").Value.Set("io1rc2d2de7rtuucalsqv4d9ng0h297t63w7wvlph"))
		_, err := util.ExecuteCmd(cmd, "1234")
		require.Equal("failed to sign message: invalid address #io1rc2d2de7rtuucalsqv4d9ng0h297t63w7wvlph", err.Error())
	})

	t.Run("valid_account", func(t *testing.T) {
		client.EXPECT().IsCryptoSm2().Return(false).Times(2)
		const pwd = "test"
		acc, _ := ks.NewAccount(pwd)
		accAddr, _ := address.FromBytes(acc.Address.Bytes())
		client.EXPECT().ReadSecret().Return(pwd, nil).Times(1)
		client.EXPECT().Address(gomock.Any()).Return(accAddr.String(), nil).Times(2)
		cmd := NewAccountSign(client)
		require.NoError(cmd.Flag("signer").Value.Set(accAddr.String()))
		result, err := util.ExecuteCmd(cmd, "1234")
		require.NoError(err)
		require.Contains(result, accAddr.String())
	})
}
