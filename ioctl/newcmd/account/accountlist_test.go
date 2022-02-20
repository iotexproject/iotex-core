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
	client.EXPECT().AliasMap().Return(map[string]string{
		"a": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
		"b": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
		"c": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1",
		"io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
	}).Times(4)

	t.Run("When NewAccountList returns no error", func(t *testing.T) {
		client.EXPECT().IsCryptoSm2().Return(false)
		testAccountFolder := filepath.Join(os.TempDir(), "testAccount")
		require.NoError(t, os.MkdirAll(testAccountFolder, os.ModePerm))
		defer func() {
			require.NoError(t, os.RemoveAll(testAccountFolder))
		}()
		ks := keystore.NewKeyStore(testAccountFolder, keystore.StandardScryptN, keystore.StandardScryptP)
		_, _ = ks.NewAccount("test")
		_, _ = ks.NewAccount("test2")

		client.EXPECT().NewKeyStore(gomock.Any(), gomock.Any(), gomock.Any()).Return(ks)
		cmd := NewAccountList(client)
		_, err := util.ExecuteCmd(cmd)
		require.NoError(t, err)
	})

	t.Run("When NewAccountList returns error", func(t *testing.T) {
		client.EXPECT().IsCryptoSm2().Return(true)
		config.ReadConfig.Wallet = ""

		cmd := NewAccountList(client)
		_, err := util.ExecuteCmd(cmd)
		require.Error(t, err)
	})
}
