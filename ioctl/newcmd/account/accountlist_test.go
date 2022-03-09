// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"errors"
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

func TestNewAccountList(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).AnyTimes()

	t.Run("When NewAccountList returns no error", func(t *testing.T) {
		client.EXPECT().IsCryptoSm2().Return(false)
		testAccountFolder := filepath.Join(os.TempDir(), "testAccount")
		require.NoError(os.MkdirAll(testAccountFolder, os.ModePerm))
		defer func() {
			require.NoError(os.RemoveAll(testAccountFolder))
		}()

		ks := keystore.NewKeyStore(testAccountFolder, keystore.StandardScryptN, keystore.StandardScryptP)
		genAccount := func(passwd string) string {
			account, err := ks.NewAccount(passwd)
			require.NoError(err)
			addr, err := address.FromBytes(account.Address.Bytes())
			require.NoError(err)
			return addr.String()
		}
		addra := genAccount("test1")
		addrb := genAccount("test2")
		client.EXPECT().NewKeyStore().Return(ks)
		client.EXPECT().AliasMap().Return(map[string]string{
			addra: "a",
			addrb: "b",
		}).Times(2)

		cmd := NewAccountList(client)
		result, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		require.Contains(result, addra+" - a")
		require.Contains(result, addrb+" - b")
	})

	t.Run("When NewAccountList returns error", func(t *testing.T) {
		client.EXPECT().IsCryptoSm2().Return(true)
		client.EXPECT().Config().Return(config.Config{}).Times(1)
		client.EXPECT().AliasMap().Return(map[string]string{
			"io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx": "a",
			"io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1": "b",
		}).Times(2)
		expectedErr := errors.New("failed to get sm2 accounts")

		cmd := NewAccountList(client)
		_, err := util.ExecuteCmd(cmd)
		require.Error(err)
		require.Contains(err.Error(), expectedErr.Error())
	})
}
