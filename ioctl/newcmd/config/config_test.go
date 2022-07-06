// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"github.com/iotexproject/iotex-core/ioctl/config"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/testutil"
)

func TestInitConfig(t *testing.T) {
	require := require.New(t)
	testPath, err := os.MkdirTemp(os.TempDir(), "testCfg")
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(testPath)
	}()
	_configDir = testPath
	cfg, cfgFilePath, err := InitConfig()
	require.NoError(err)
	require.Equal(testPath, cfg.Wallet)
	require.Equal(_validExpl[0], cfg.Explorer)
	require.Equal(_supportedLanguage[0], cfg.Language)
	require.Equal(filepath.Join(testPath, _defaultConfigFileName), cfgFilePath)
}

func TestConfigReset(t *testing.T) {
	require := require.New(t)

	info := newInfo(config.Config{
		Wallet:           "wallet",
		Endpoint:         "testEndpoint",
		SecureConnect:    false,
		DefaultAccount:   config.Context{AddressOrAlias: ""},
		Explorer:         "explorer",
		Language:         "RandomLanguage",
		AnalyserEndpoint: "testAnalyser",
	}, _defaultConfigFileName)

	require.NoError(info.reset())

	config.DefaultConfigFile = _defaultConfigFileName
	cfg, err := config.LoadConfig()
	require.NoError(err)

	// ensure config has been reset
	require.Equal(".", cfg.Wallet)
	require.Equal("", cfg.Endpoint)
	require.Equal(true, cfg.SecureConnect)
	require.Equal("English", cfg.Language)
	require.Equal(_defaultAnalyserEndpoint, cfg.AnalyserEndpoint)
	require.Equal("iotexscan", cfg.Explorer)
	require.Equal(*new(config.Context), cfg.DefaultAccount)

	defer testutil.CleanupPath("config.default")
}
