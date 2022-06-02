// Copyright (c) 2022 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestNewNodeDelegateCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)

	mnemonic := "lake stove quarter shove dry matrix hire split wide attract argue core"
	password := "123"

	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).Times(6)
	client.EXPECT().Config().Return(config.Config{
		Wallet: config.ReadConfig.Wallet,
	}).Times(2)

	t.Run("import hdwallet", func(t *testing.T) {
		client.EXPECT().ReadInput().Return(mnemonic, nil)
		client.EXPECT().ReadSecret().Return(password, nil)
		client.EXPECT().ReadSecret().Return(password, nil)
		client.EXPECT().WriteFile(gomock.Any(), gomock.Any()).Return(nil)
		defer os.RemoveAll(_hdWalletConfigFile)

		cmd := NewHdwalletImportCmd(client)
		result, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		require.Contains(result, mnemonic)
	})

	t.Run("failed to write to config file", func(t *testing.T) {
		expectErr := errors.New("failed to write to config file")
		client.EXPECT().ReadInput().Return(mnemonic, nil)
		client.EXPECT().ReadSecret().Return(password, nil)
		client.EXPECT().ReadSecret().Return(password, nil)
		client.EXPECT().WriteFile(gomock.Any(), gomock.Any()).Return(expectErr)
		defer os.RemoveAll(_hdWalletConfigFile)

		cmd := NewHdwalletImportCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.Contains(err.Error(), expectErr.Error())
	})

	t.Run("hdwalletConfigFile exist", func(t *testing.T) {
		expectedValue := "Please run 'ioctl hdwallet delete' before import"
		testAccountFolder, err := os.MkdirTemp(os.TempDir(), "default")
		require.NoError(err)
		defer testutil.CleanupPath(testAccountFolder)
		file := filepath.Join(testAccountFolder, "hdwallet")
		if err := os.WriteFile(file, []byte("content"), 0666); err != nil {
			log.Fatal(err)
		}

		client.EXPECT().Config().Return(config.Config{
			Wallet: testAccountFolder,
		})

		cmd := NewHdwalletImportCmd(client)
		result, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		require.Contains(result, expectedValue)
	})
}
