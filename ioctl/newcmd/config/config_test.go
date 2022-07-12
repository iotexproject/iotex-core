// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
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

func TestConfigGet(t *testing.T) {
	require := require.New(t)
	tempDir := os.TempDir()
	testPath, err := os.MkdirTemp(tempDir, "testCfg")
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(testPath)
	}()
	_configDir = testPath
	cfg, cfgFilePath, err := InitConfig()
	// the endpoint & default account are blank strings within the initial config so I'm setting it here
	cfg.Endpoint = "http://google.com"
	cfg.DefaultAccount = config.Context{AddressOrAlias: "test"}
	require.NoError(err)

	info := newInfo(cfg, cfgFilePath)

	tcs := []struct {
		arg      string
		expected string
	}{
		{
			"endpoint",
			"http://google.com    secure connect(TLS):true",
		},
		{
			"wallet",
			fmt.Sprintf("%s%s", tempDir, "testCfg"),
		},
		{
			"defaultacc",
			"{\n  \"addressOrAlias\": \"test\"\n}",
		},
		{
			"explorer",
			"iotexscan",
		},
		{
			"language",
			"English",
		},
		{
			"nsv2height",
			"5165641",
		},
		{
			"analyserEndpoint",
			"https://iotex-analyser-api-mainnet.chainanalytics.org",
		},
		{
			"all",
			"\"endpoint\": \"http://google.com\",\n  \"secureConnect\": true,\n  \"aliases\": {},\n  \"defaultAccount\": {\n    \"addressOrAlias\": \"test\"\n  },\n  \"explorer\": \"iotexscan\",\n  \"language\": \"English\",\n  \"nsv2height\": 5165641,\n  \"analyserEndpoint\": \"https://iotex-analyser-api-mainnet.chainanalytics.org\"\n}",
		},
	}

	for _, tc := range tcs {
		cfgItem, err := info.get(tc.arg)
		require.NoError(err)
		require.Contains(cfgItem, tc.expected)
	}
}

func TestConfigGetError(t *testing.T) {
	require := require.New(t)
	tempDir := os.TempDir()
	testPath, err := os.MkdirTemp(tempDir, "testCfg")
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(testPath)
	}()
	_configDir = testPath
	cfg, cfgFilePath, err := InitConfig()
	info := newInfo(cfg, cfgFilePath)

	tcs := []struct {
		arg      string
		expected string
	}{
		{
			"endpoint",
			"no endpoint has been set",
		},
		{
			"defaultacc",
			"default account not set",
		},
	}
	for _, tc := range tcs {
		_, err := info.get(tc.arg)
		require.Error(err)
		require.Contains(err.Error(), tc.expected)
	}
}
