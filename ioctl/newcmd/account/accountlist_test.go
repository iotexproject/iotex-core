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
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewAccountList(t *testing.T) {

	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).AnyTimes()

	t.Run("When NewAccountList returns no error", func(t *testing.T) {
		testAccountFolder := filepath.Join(os.TempDir(), "testAccount")
		require.NoError(t, os.MkdirAll(testAccountFolder, os.ModePerm))
		defer func() {
			require.NoError(t, os.RemoveAll(testAccountFolder))
		}()
		ks := keystore.NewKeyStore(testAccountFolder, keystore.StandardScryptN, keystore.StandardScryptP)
		_, _ = ks.NewAccount("test")
		_, _ = ks.NewAccount("test2")
		
		client.EXPECT().GetAliasMap().DoAndReturn(
			func() map[string]string {
				aliases := make(map[string]string)
				for name, addr := range config.ReadConfig.Aliases {
					aliases[addr] = name
				}
				return aliases
			}).Times(2)

		client.EXPECT().NewKeyStore(gomock.Any(), gomock.Any(), gomock.Any()).Return(ks)
		cmd := NewAccountList(client)
		_, err := util.ExecuteCmd(cmd)
		require.NoError(t, err)
	})

	t.Run("When NewAccountList returns error", func(t *testing.T) {
		CryptoSm2 = true
		config.ReadConfig.Wallet = ""

		client.EXPECT().GetAliasMap().DoAndReturn(
			func() map[string]string {
				aliases := make(map[string]string)
				for name, addr := range config.ReadConfig.Aliases {
					aliases[addr] = name
				}
				return aliases
			}).Times(2)

		cmd := NewAccountList(client)
		_, err := util.ExecuteCmd(cmd)
		require.Error(t, err)
	})

}
