// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"github.com/iotexproject/iotex-core/testutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewAccountImportCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).AnyTimes()
	client.EXPECT().Config().Return(config.Config{}).AnyTimes()

	cmd := NewAccountImportCmd(client)
	result, err := util.ExecuteCmd(cmd, "hh")

	require.NotNil(result)
	require.Error(err)
}

func TestNewAccountImportKeyCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).AnyTimes()
	client.EXPECT().Config().Return(config.Config{}).AnyTimes()

	cmd := NewAccountImportKeyCmd(client)
	result, err := util.ExecuteCmd(cmd, "hh")

	require.NotNil(result)
	require.Error(err)
}

func TestNewAccountImportKeyStoreCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).AnyTimes()
	client.EXPECT().Config().Return(config.Config{}).AnyTimes()

	testAccountFolder := filepath.Join(os.TempDir(), "testAccountImportKeyStore")
	require.NoError(os.MkdirAll(testAccountFolder, os.ModePerm))
	defer func() {
		require.NoError(os.RemoveAll(testAccountFolder))
	}()

	cmd := NewAccountImportKeyStoreCmd(client)
	result, err := util.ExecuteCmd(cmd, []string{"hh", "./testAccountImportKeyStoreCmd"}...)

	require.NotNil(result)
	require.Error(err)
}

func TestNewAccountImportPemCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).AnyTimes()
	client.EXPECT().Config().Return(config.Config{}).AnyTimes()

	cmd := NewAccountImportPemCmd(client)
	result, err := util.ExecuteCmd(cmd, []string{"hh", "./testAccountImportPemCmd"}...)

	require.NotNil(result)
	require.Error(err)
}

func TestValidateAlias(t *testing.T) {
	require := require.New(t)
	testFilePath := filepath.Join(os.TempDir(), testPath)
	defer testutil.CleanupPath(testFilePath)
	alias := "aaa"

	config.ReadConfig.Aliases = map[string]string{}
	err := validateAlias(alias)
	require.NoError(err)

	config.ReadConfig.Aliases[alias] = "a"
	err = validateAlias(alias)
	require.Error(err)

	alias = strings.Repeat("a", 50)
	err = validateAlias(alias)
	require.Error(err)
}

func TestWriteToFile(t *testing.T) {
	require := require.New(t)
	testFilePath := filepath.Join(os.TempDir(), testPath)
	defer testutil.CleanupPath(testFilePath)
	alias := "aaa"
	addr := "bbb"

	err := writeToFile(alias, addr)
	require.NoError(err)
}

func TestReadPasswordFromStdin(t *testing.T) {
	require := require.New(t)
	_, err := readPasswordFromStdin()
	require.Error(err)
}
