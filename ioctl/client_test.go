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

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestStop(t *testing.T) {
	r := require.New(t)
	c := NewClient(config.Config{}, EnableCryptoSm2())
	_, err := c.APIServiceClient(APIServiceConfig{Endpoint: "127.0.0.1:14014", Insecure: true})
	r.NoError(err)
	err = c.Stop(context.Background())
	r.NoError(err)
}

func TestAskToConfirm(t *testing.T) {
	r := require.New(t)
	c := NewClient(config.Config{})
	defer c.Stop(context.Background())
	blang := c.AskToConfirm("test")
	// no input
	r.False(blang)
}

func TestAPIServiceClient(t *testing.T) {
	r := require.New(t)
	c := NewClient(config.Config{})
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
		cleanupPath := writeTempConfig(t, &test.cfg)
		defer cleanupPath()
		cfg, err := config.LoadConfig()
		r.NoError(err)
		c := NewClient(cfg)
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

	cfg := config.Config{
		Wallet: testWallet,
	}
	c := NewClient(cfg)
	defer c.Stop(context.Background())

	ks := c.NewKeyStore()
	acc, err := ks.NewAccount("test")
	r.NoError(err)
	_, err = os.Stat(acc.URL.Path)
	r.NoError(err)
	r.True(strings.HasPrefix(acc.URL.Path, testWallet))
	r.True(ks.HasAddress(acc.Address))
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
	cleanupPath := writeTempConfig(t, &cfg)
	defer cleanupPath()
	cfgload, err := config.LoadConfig()
	r.NoError(err)
	r.Equal(cfg, cfgload)

	exprAliases := map[string]string{
		"io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95haaa": "aaa",
		"io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hbbb": "bbb",
		"io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hccc": "ccc",
	}
	c := NewClient(cfgload)
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
		c := NewClient(config.Config{})
		err = c.WriteConfig(test)
		cfgload, err := config.LoadConfig()
		r.NoError(err)
		r.Equal(test, cfgload)
	}
}

func writeTempConfig(t *testing.T, cfg *config.Config) func() {
	r := require.New(t)
	testPathd, err := os.MkdirTemp(os.TempDir(), "kstest")
	r.NoError(err)
	config.ConfigDir = testPathd
	config.DefaultConfigFile = config.ConfigDir + "/config.default"
	cfg.Wallet = config.ConfigDir
	out, err := yaml.Marshal(cfg)
	r.NoError(err)
	r.NoError(os.WriteFile(config.DefaultConfigFile, out, 0600))
	return func() {
		defer testutil.CleanupPath(t, testPathd)
	}
}
