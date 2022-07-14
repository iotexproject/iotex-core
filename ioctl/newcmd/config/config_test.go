// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
)

func TestInitConfig(t *testing.T) {
	require := require.New(t)
	testPath := t.TempDir()
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
	cfgDir := t.TempDir()
	cfgFile := fmt.Sprintf("%s/%s", cfgDir, "config.test")

	info := newInfo(config.Config{
		Wallet:           "wallet",
		Endpoint:         "testEndpoint",
		SecureConnect:    false,
		DefaultAccount:   config.Context{AddressOrAlias: ""},
		Explorer:         "explorer",
		Language:         "Croatian",
		AnalyserEndpoint: "testAnalyser",
	}, cfgFile)

	// write the config to the temp dir and then reset
	require.NoError(info.writeConfig())
	require.NoError(info.loadConfig())
	cfg := info.readConfig

	require.Equal("wallet", cfg.Wallet)
	require.Equal("testEndpoint", cfg.Endpoint)
	require.Equal(false, cfg.SecureConnect)
	require.Equal("Croatian", cfg.Language)
	require.Equal("testAnalyser", cfg.AnalyserEndpoint)
	require.Equal("explorer", cfg.Explorer)
	require.Equal(config.Context{AddressOrAlias: ""}, cfg.DefaultAccount)

	require.NoError(info.reset())
	require.NoError(info.loadConfig())
	resetCfg := info.readConfig

	// ensure config has been reset
	require.Equal(cfgDir, resetCfg.Wallet)
	require.Equal("", resetCfg.Endpoint)
	require.Equal(true, resetCfg.SecureConnect)
	require.Equal("English", resetCfg.Language)
	require.Equal(_defaultAnalyserEndpoint, resetCfg.AnalyserEndpoint)
	require.Equal("iotexscan", resetCfg.Explorer)
	require.Equal(*new(config.Context), resetCfg.DefaultAccount)
}
