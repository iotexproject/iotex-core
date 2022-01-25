// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewAccountSign(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).AnyTimes()

	testAccountFolder := filepath.Join(os.TempDir(), "testNewAccountSign")
	require.NoError(os.MkdirAll(testAccountFolder, os.ModePerm))
	defer func() {
		require.NoError(os.RemoveAll(testAccountFolder))
	}()
	ks := keystore.NewKeyStore(testAccountFolder, keystore.StandardScryptN, keystore.StandardScryptP)
	client.EXPECT().NewKeyStore(gomock.Any(), gomock.Any(), gomock.Any()).Return(ks).AnyTimes()

	t.Run("invalid_account", func(t *testing.T) {
		cmd := NewAccountSign(client)
		require.NoError(cmd.Flag("signer").Value.Set("io1rc2d2de7rtuucalsqv4d9ng0h297t63w7wvlph"))
		_, err := util.ExecuteCmd(cmd, "1234")
		require.Equal("failed to sign message: invalid address #io1rc2d2de7rtuucalsqv4d9ng0h297t63w7wvlph", err.Error())
	})
	t.Run("valid_account", func(t *testing.T) {
		const pwd = "test"
		acc, _ := ks.NewAccount(pwd)
		accAddr, _ := address.FromBytes(acc.Address.Bytes())
		client.EXPECT().ReadSecret().Return(pwd, nil).Times(1)
		cmd := NewAccountSign(client)
		require.NoError(cmd.Flag("signer").Value.Set(accAddr.String()))
		_, err := util.ExecuteCmd(cmd, "1234")
		require.NoError(err)
	})
}
