// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package ioctl

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestStop(t *testing.T) {
	r := require.New(t)
	c := NewClient(false)
	_, err := c.APIServiceClient(APIServiceConfig{Endpoint: "127.0.0.1:14014", Insecure: true})
	r.NoError(err)
	err = c.Stop(context.Background())
	r.NoError(err)
}

func TestAskToConfirm(t *testing.T) {
	r := require.New(t)
	c := NewClient(false)
	defer c.Stop(context.Background())
	blang := c.AskToConfirm("test")
	// no input
	r.False(blang)
}

func TestAPIServiceClient(t *testing.T) {
	r := require.New(t)
	c := NewClient(false)
	defer c.Stop(context.Background())
	apiServiceClient, err := c.APIServiceClient(APIServiceConfig{Endpoint: "127.0.0.1:14014", Insecure: true})
	r.NoError(err)
	r.NotNil(apiServiceClient)

	apiServiceClient, err = c.APIServiceClient(APIServiceConfig{Endpoint: "127.0.0.199:14014", Insecure: false})
	r.NoError(err)
	r.NotNil(apiServiceClient)

	apiServiceClient, err = c.APIServiceClient(APIServiceConfig{Endpoint: "", Insecure: false})
	r.Contains(err.Error(), `use "ioctl config set endpoint" to config endpoint first`)
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
		c := NewClient(false)
		out, err := c.GetAddress(test.in)
		if err != nil {
			r.Contains(err.Error(), test.errMsg)
		}
		r.Equal(test.out, out)
	}
}

func TestNewKeyStore(t *testing.T) {
	r := require.New(t)
	testWallet, err := os.MkdirTemp(os.TempDir(), "ksTest")
	r.NoError(err)
	defer testutil.CleanupPath(t, testWallet)

	c := NewClient(false)
	defer c.Stop(context.Background())

	tests := [][]int{
		{keystore.StandardScryptN, keystore.StandardScryptP},
		{keystore.LightScryptN, keystore.LightScryptP},
	}
	for _, test := range tests {
		ks := c.NewKeyStore(testWallet, test[0], test[1])
		acc, err := ks.NewAccount("test")
		r.NoError(err)
		_, err = os.Stat(acc.URL.Path)
		r.NoError(err)
		r.True(strings.HasPrefix(acc.URL.Path, testWallet))
		r.True(ks.HasAddress(acc.Address))
	}
}

func TestAliasMap(t *testing.T) {
	r := require.New(t)
	cfg := config.Config{
		Aliases: map[string]string{
			"aaa": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95haaa",
			"bbb": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hbbb",
			"ccc": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hccc",
		},
	}
	r.NoError(writeTempConfig(t, &cfg))
	cfgload, err := config.LoadConfig()
	r.NoError(err)
	r.Equal(cfg, cfgload)
	defer testutil.CleanupPath(t, config.ConfigDir)
	config.ReadConfig = cfgload

	exprAliases := map[string]string{
		cfg.Aliases["aaa"]: "aaa",
		cfg.Aliases["bbb"]: "bbb",
		cfg.Aliases["ccc"]: "ccc",
	}
	c := NewClient(false)
	defer c.Stop(context.Background())
	result := c.AliasMap()
	r.Equal(exprAliases, result)
}

func TestWriteConfig(t *testing.T) {
	r := require.New(t)
	testPathd, err := os.MkdirTemp(os.TempDir(), "cfgtest")
	r.NoError(err)
	defer testutil.CleanupPath(t, testPathd)
	config.ConfigDir = testPathd
	config.DefaultConfigFile = config.ConfigDir + "/config.default"

	tests := []config.Config{
		{
			Endpoint:      "127.1.1.1:1234",
			SecureConnect: true,
			Aliases: map[string]string{
				"aaa": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95haaa",
				"bbb": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hbbb",
				"ccc": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hccc",
			},
			DefaultAccount: config.Context{AddressOrAlias: "ddd"},
		},

		{
			Aliases: map[string]string{
				"": "",
			},
			DefaultAccount: config.Context{AddressOrAlias: ""},
		},
	}

	for _, test := range tests {
		c := NewClient(false)
		err = c.WriteConfig(test)
		cfgload, err := config.LoadConfig()
		r.NoError(err)
		r.Equal(test, cfgload)
	}
}

func writeTempConfig(t *testing.T, cfg *config.Config) error {
	r := require.New(t)
	testPathd, err := os.MkdirTemp(os.TempDir(), "kstest")
	r.NoError(err)
	config.ConfigDir = testPathd
	config.DefaultConfigFile = config.ConfigDir + "/config.default"
	config.ReadConfig.Wallet = config.ConfigDir
	out, err := yaml.Marshal(cfg)
	r.NoError(err)
	r.NoError(os.WriteFile(config.DefaultConfigFile, out, 0600))
	return nil
}
