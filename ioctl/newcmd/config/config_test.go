// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_ioctlclient"
)

func TestNewConfigCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("config usage", config.English).Times(8)
	client.EXPECT().SetInsecureWithFlag(gomock.Any())
	cmd := NewConfigCmd(client)
	result, err := util.ExecuteCmd(cmd)
	require.NoError(err)
	require.Contains(result, "Available Commands")
}

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
	require.Equal(_defaultWsEndpoint, cfg.WsEndpoint)
	require.Equal(_defaultIPFSEndpoint, cfg.IPFSEndpoint)
	require.Equal(_defaultIPFSGateway, cfg.IPFSGateway)
}

func TestConfigGet(t *testing.T) {
	require := require.New(t)
	testPath := t.TempDir()
	info := newInfo(config.Config{
		Wallet:                    testPath,
		SecureConnect:             true,
		Aliases:                   make(map[string]string),
		DefaultAccount:            config.Context{AddressOrAlias: "test"},
		Explorer:                  "iotexscan",
		Language:                  "English",
		AnalyserEndpoint:          "testAnalyser",
		WsEndpoint:                "testWsEndpoint",
		IPFSEndpoint:              "testIPFSEndpoint",
		IPFSGateway:               "testIPFSGateway",
		WsProjectRegisterContract: "testWsProjectRegisterContract",
		WsProjectStoreContract:    "testWsProjectStoreContract",
		WsFleetManagementContract: "testWsFleetManagementContract",
		WsProverStoreContract:     "testWsProverStoreContract",
		WsProjectDevicesContract:  "testWsProjectDevicesContract",
		WsRouterContract:          "testWsRouterContract",
		WsVmTypeContract:          "testWsVmTypeContract",
	}, testPath)

	tcs := []struct {
		arg      string
		expected string
	}{
		{
			"endpoint",
			"no endpoint has been set",
		},
		{
			"wallet",
			testPath,
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
			"0",
		},
		{
			"analyserEndpoint",
			"testAnalyser",
		},
		{
			"wsEndpoint",
			"testWsEndpoint",
		},
		{
			"ipfsEndpoint",
			"testIPFSEndpoint",
		},
		{
			"ipfsGateway",
			"testIPFSGateway",
		},
		{
			"wsProjectRegisterContract",
			"testWsProjectRegisterContract",
		},
		{
			"wsProjectStoreContract",
			"testWsProjectStoreContract",
		},
		{
			"wsFleetManagementContract",
			"testWsFleetManagementContract",
		},
		{
			"wsProverStoreContract",
			"testWsProverStoreContract",
		},
		{
			"wsRouterContract",
			"testWsRouterContract",
		},
		{
			"wsVmTypeContract",
			"testWsVmTypeContract",
		},
		{
			"all",
			"  \"endpoint\": \"\",\n  \"secureConnect\": true,\n  \"aliases\": {},\n  \"defaultAccount\": {\n    \"addressOrAlias\": \"test\"\n  },\n  \"explorer\": \"iotexscan\",\n  \"language\": \"English\",\n  \"nsv2height\": 0,\n  \"analyserEndpoint\": \"testAnalyser\",\n  \"wsEndpoint\": \"testWsEndpoint\",\n  \"ipfsEndpoint\": \"testIPFSEndpoint\",\n  \"ipfsGateway\": \"testIPFSGateway\",\n  \"wsProjectRegisterContract\": \"testWsProjectRegisterContract\",\n  \"wsProjectStoreContract\": \"testWsProjectStoreContract\",\n  \"wsFleetManagementContract\": \"testWsFleetManagementContract\",\n  \"wsProverStoreContract\": \"testWsProverStoreContract\",\n  \"wsProjectDevicesContract\": \"testWsProjectDevicesContract\",\n  \"wsRouterContract\": \"testWsRouterContract\",\n  \"wsVmTypeContract\": \"testWsVmTypeContract\",\n  \"ioidProjectRegisterContract\": \"\",\n  \"ioidProjectStoreContract\": \"\"\n}",
		},
	}

	for _, tc := range tcs {
		cfgItem, err := info.get(tc.arg)
		if err != nil {
			require.Contains(err.Error(), tc.expected)
		} else {
			require.Contains(cfgItem, tc.expected)
		}
	}
}

func TestConfigReset(t *testing.T) {
	require := require.New(t)
	cfgDir := t.TempDir()
	cfgFile := fmt.Sprintf("%s/%s", cfgDir, "config.test")

	info := newInfo(config.Config{
		Wallet:                    "wallet",
		Endpoint:                  "testEndpoint",
		SecureConnect:             false,
		DefaultAccount:            config.Context{AddressOrAlias: ""},
		Explorer:                  "explorer",
		Language:                  "Croatian",
		AnalyserEndpoint:          "testAnalyser",
		WsEndpoint:                "testWsEndpoint",
		IPFSEndpoint:              "testIPFSEndpoint",
		IPFSGateway:               "testIPFSGateway",
		WsProjectRegisterContract: "testWsProjectRegisterContract",
		WsProjectStoreContract:    "testWsProjectStoreContract",
		WsFleetManagementContract: "testWsFleetManagementContract",
		WsProverStoreContract:     "testWsProverStoreContract",
		WsProjectDevicesContract:  "testWsProjectDevicesContract",
		WsRouterContract:          "testWsRouterContract",
		WsVmTypeContract:          "testWsVmTypeContract",
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
	require.Equal("testWsEndpoint", cfg.WsEndpoint)
	require.Equal("testIPFSEndpoint", cfg.IPFSEndpoint)
	require.Equal("testIPFSGateway", cfg.IPFSGateway)
	require.Equal("testWsProjectRegisterContract", cfg.WsProjectRegisterContract)
	require.Equal("testWsProjectStoreContract", cfg.WsProjectStoreContract)
	require.Equal("testWsFleetManagementContract", cfg.WsFleetManagementContract)
	require.Equal("testWsProverStoreContract", cfg.WsProverStoreContract)
	require.Equal("testWsProjectDevicesContract", cfg.WsProjectDevicesContract)
	require.Equal("testWsRouterContract", cfg.WsRouterContract)
	require.Equal("testWsVmTypeContract", cfg.WsVmTypeContract)

	require.NoError(info.reset())
	require.NoError(info.loadConfig())
	resetCfg := info.readConfig

	// ensure config has been reset
	require.Equal(cfgDir, resetCfg.Wallet)
	require.Equal("", resetCfg.Endpoint)
	require.Equal(true, resetCfg.SecureConnect)
	require.Equal("English", resetCfg.Language)
	require.Equal(_defaultAnalyserEndpoint, resetCfg.AnalyserEndpoint)
	require.Equal(_defaultWsEndpoint, resetCfg.WsEndpoint)
	require.Equal(_defaultIPFSEndpoint, resetCfg.IPFSEndpoint)
	require.Equal(_defaultIPFSGateway, resetCfg.IPFSGateway)
	require.Equal(_defaultWsProjectRegisterContract, resetCfg.WsProjectRegisterContract)
	require.Equal(_defaultWsProjectStoreContract, resetCfg.WsProjectStoreContract)
	require.Equal(_defaultWsFleetManagementContract, resetCfg.WsFleetManagementContract)
	require.Equal(_defaultWsProverStoreContract, resetCfg.WsProverStoreContract)
	require.Equal(_defaultWsProjectDevicesContract, resetCfg.WsProjectDevicesContract)
	require.Equal(_defaultWsRouterContract, resetCfg.WsRouterContract)
	require.Equal(_defaultWsVmTypeContract, resetCfg.WsVmTypeContract)
	require.Equal("iotexscan", resetCfg.Explorer)
	require.Equal(*new(config.Context), resetCfg.DefaultAccount)
}

func TestConfigSet(t *testing.T) {
	require := require.New(t)
	testPath := t.TempDir()
	cfgFile := fmt.Sprintf("%s/%s", testPath, "config.test")

	info := newInfo(config.Config{
		Wallet:           testPath,
		SecureConnect:    true,
		Aliases:          make(map[string]string),
		DefaultAccount:   config.Context{AddressOrAlias: "test"},
		Explorer:         "iotexscan",
		Language:         "English",
		AnalyserEndpoint: "testAnalyser",
		WsEndpoint:       "testWsEndpoint",
		IPFSEndpoint:     "testIPFSEndpoint",
		IPFSGateway:      "testIPFSGateway",
	}, cfgFile)

	tcs := []struct {
		args     []string
		expected string
	}{
		{
			[]string{"endpoint", "invalid endpoint"},
			"endpoint invalid endpoint is not valid",
		},
		{
			[]string{"wallet", testPath},
			testPath,
		},
		{
			[]string{"defaultacc", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he"},
			"Defaultacc is set to io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he",
		},
		{
			[]string{"defaultacc", "suzxctxgbidciovisbrecerurkbjkmyqrftxtnjyp"},
			"failed to validate alias or address suzxctxgbidciovisbrecerurkbjkmyqrftxtnjyp",
		},
		{
			[]string{"explorer", "iotxplorer"},
			"Explorer is set to iotxplorer",
		},
		{
			[]string{"explorer", "invalid"},
			"explorer invalid is not valid\nValid explorers: [iotexscan iotxplorer custom]",
		},
		{
			[]string{"language", "中文"},
			"Language is set to 中文",
		},
		{
			[]string{"language", "unknown language"},
			"language unknown language is not supported\nSupported languages: [English 中文]",
		},
		{
			[]string{"nsv2height", "20"},
			"Nsv2height is set to 20",
		},
		{
			[]string{"nsv2height", "invalid height"},
			"invalid height",
		},
		{
			[]string{"unknownField", ""},
			"no matching config",
		},
		{
			[]string{"wsEndpoint", "testWsEndpoint"},
			"testWsEndpoint",
		},
		{
			[]string{"ipfsEndpoint", "testIPFSEndpoint"},
			"testIPFSEndpoint",
		},
		{
			[]string{"ipfsGateway", "testIPFSGateway"},
			"testIPFSGateway",
		},
		{
			[]string{"wsProjectRegisterContract", "testWsProjectRegisterContract"},
			"testWsProjectRegisterContract",
		},
		{
			[]string{"wsProjectStoreContract", "testWsProjectStoreContract"},
			"testWsProjectStoreContract",
		},
		{
			[]string{"wsFleetManagementContract", "testWsFleetManagementContract"},
			"testWsFleetManagementContract",
		},
		{
			[]string{"wsProverStoreContract", "testWsProverStoreContract"},
			"testWsProverStoreContract",
		},
		{
			[]string{"wsRouterContract", "testWsRouterContract"},
			"testWsRouterContract",
		},
		{
			[]string{"wsVmTypeContract", "testWsVmTypeContract"},
			"testWsVmTypeContract",
		},
	}

	for _, tc := range tcs {
		setResult, err := info.set(tc.args, false, nil)
		if err != nil {
			require.Contains(err.Error(), tc.expected)
		} else {
			require.Contains(setResult, tc.expected)
		}
	}
}
