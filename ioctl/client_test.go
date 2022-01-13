// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package ioctl

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestStop(t *testing.T) {
	c := NewClient()
	_, err := c.APIServiceClient(APIServiceConfig{Endpoint: "127.0.0.1:14014", Insecure: true})
	require.NoError(t, err)
	err = c.Stop(context.Background())
	require.NoError(t, err)
}

func TestAskToConfirm(t *testing.T) {
	c := NewClient()
	defer c.Stop(context.Background())
	blang := c.AskToConfirm()
	// no input
	require.False(t, blang)

	c = &client{
		cfg:  config.ReadConfig,
		lang: config.Chinese,
	}
	blang = c.AskToConfirm()
	require.False(t, blang)

	c = &client{
		cfg:  config.ReadConfig,
		lang: 2, // other language is english as default
	}
	blang = c.AskToConfirm()
	require.False(t, blang)
}

func TestAPIServiceClient(t *testing.T) {
	r := require.New(t)
	c := NewClient()
	defer c.Stop(context.Background())
	apiServiceClient, err := c.APIServiceClient(APIServiceConfig{Endpoint: "127.0.0.1:14014", Insecure: true})
	r.NoError(err)
	r.NotNil(apiServiceClient)

	apiServiceClient, err = c.APIServiceClient(APIServiceConfig{Endpoint: "127.0.0.199:14014", Insecure: false})
	r.NoError(err)
	r.NotNil(apiServiceClient)

	apiServiceClient, err = c.APIServiceClient(APIServiceConfig{Endpoint: "", Insecure: false})
	r.Error(err)
	r.Contains(`use "ioctl config set endpoint" to config endpoint first`, err.Error())
	r.Nil(apiServiceClient)
}

func TestGetAddress(t *testing.T) {
	type Data struct {
		cfg    config.Config
		in     string
		out    string
		errMsg string
	}

	tests := []Data{
		{
			config.Config{
				Aliases:        map[string]string{"": ""},
				DefaultAccount: config.Context{AddressOrAlias: "abcdef"},
			}, "abcdef", "", "cannot find address from",
		},

		{
			config.Config{
				Aliases: map[string]string{
					"000io187evpmjdankjh0g5dfz83w2z3p23ljhn4s9jw7": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hc5r",
					"bbb": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hbbb",
				},
				DefaultAccount: config.Context{AddressOrAlias: "000io187evpmjdankjh0g5dfz83w2z3p23ljhn4s9jw7"},
			}, "000io187evpmjdankjh0g5dfz83w2z3p23ljhn4s9jw7", "", "invalid IoTeX address",
		},

		{
			config.Config{
				Aliases: map[string]string{
					"bbb": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hbbb",
				},
				DefaultAccount: config.Context{AddressOrAlias: ""},
			}, "", "", `use "ioctl config set defaultacc ADDRESS|ALIAS" to config default account first`,
		},

		{
			config.Config{
				Aliases: map[string]string{
					"abcdef": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hc5r",
					"bbb":    "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hbbb",
				},
				DefaultAccount: config.Context{AddressOrAlias: ""},
			}, "abcdef", "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hc5r", "",
		},

		{
			config.Config{
				Aliases: map[string]string{
					"ccc": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hccc",
					"abc": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95aabc",
					"bbb": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hbbb",
				},
				DefaultAccount: config.Context{AddressOrAlias: "abc"},
			}, "abc", "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95aabc", "",
		},
	}

	for _, test := range tests {
		r := require.New(t)
		r.NoError(writeTempConfig(t, &test.cfg))
		cfg, err := config.LoadConfig()
		r.NoError(err)
		defer testutil.CleanupPath(t, config.ConfigDir)
		config.ReadConfig = cfg
		c := NewClient()
		out, err := c.GetAddress(test.in)
		if err != nil {
			r.Error(err)
			r.Contains(err.Error(), test.errMsg)
		}
		r.Equal(test.out, out)
	}
}

func TestNewKeyStore(t *testing.T) {
	testWallet := filepath.Join(os.TempDir(), "ksTest")
	defer testutil.CleanupPath(t, testWallet)
	config.ReadConfig.Wallet = testWallet
	c := NewClient()
	defer c.Stop(context.Background())
	keyStore := c.NewKeyStore(config.ReadConfig.Wallet, keystore.StandardScryptN, keystore.StandardScryptP)
	require.NotNil(t, keyStore)
	keyStore = c.NewKeyStore(config.ReadConfig.Wallet, keystore.LightScryptN, keystore.LightScryptP)
	require.NotNil(t, keyStore)
}

func TestGetAliasMap(t *testing.T) {
	cfg := config.Config{
		Aliases: map[string]string{
			"aaa": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95haaa",
			"bbb": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hbbb",
			"ccc": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hccc",
		},
	}
	require.NoError(t, writeTempConfig(t, &cfg))
	cfg, err := config.LoadConfig()
	defer testutil.CleanupPath(t, config.ConfigDir)
	require.NoError(t, err)
	config.ReadConfig = cfg

	exprAliases := map[string]string{
		cfg.Aliases["aaa"]: "aaa",
		cfg.Aliases["bbb"]: "bbb",
		cfg.Aliases["ccc"]: "ccc",
	}
	c := NewClient()
	defer c.Stop(context.Background())
	result := c.GetAliasMap()
	require.Equal(t, exprAliases, result)
}

func TestWriteConfig(t *testing.T) {
	config.ConfigDir = os.Getenv("HOME") + "/.config/ioctl/default"
	if !fileutil.FileExists(config.ConfigDir) {
		err := os.MkdirAll(config.ConfigDir, 0700)
		require.NoError(t, err)
	}
	config.DefaultConfigFile = config.ConfigDir + "/config.default"

	cfg := config.Config{
		Aliases: map[string]string{
			"aaa": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95haaa",
			"bbb": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hbbb",
			"ccc": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hccc",
		},
		DefaultAccount: config.Context{AddressOrAlias: "ddd"},
	}
	c := NewClient()
	defer c.Stop(context.Background())
	err := c.WriteConfig(cfg)
	require.NoError(t, err)

	cfg = config.Config{
		Aliases: map[string]string{
			"": "",
		},
		DefaultAccount: config.Context{AddressOrAlias: ""},
	}
	err = c.WriteConfig(cfg)
	require.NoError(t, err)
}

func writeTempConfig(t *testing.T, cfg *config.Config) error {
	testPathd, _ := ioutil.TempDir(os.TempDir(), "kstest")
	config.ConfigDir = testPathd
	config.DefaultConfigFile = config.ConfigDir + "/config.default"
	out, err := yaml.Marshal(cfg)
	if err != nil {
		t.Error(err)
		return err
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		t.Error(err)
		return err
	}
	return nil
}
