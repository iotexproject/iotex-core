// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package ioctl

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/ioctl/config"
)

func TestStop(t *testing.T) {
	r := require.New(t)
	c, err := NewClient(config.Config{}, "", EnableCryptoSm2())
	r.NoError(err)
	c.SetEndpointWithFlag(func(p *string, _ string, _ string, _ string) {
		*p = "127.0.0.1:14014"
	})
	c.SetInsecureWithFlag(func(p *bool, _ string, _ bool, _ string) {
		*p = true
	})
	c.SetXrc20ContractAddrWithFlag(func(p *string, _ string, _ string, _ string, _ string) {
		*p = "0xsjfdjfk"
	}, func(_ string) error {
		return nil
	})
	_, err = c.APIServiceClient()
	r.NoError(err)
	err = c.Stop(context.Background())
	r.NoError(err)
}

func TestAskToConfirm(t *testing.T) {
	r := require.New(t)
	c, err := NewClient(config.Config{}, "")
	r.NoError(err)
	defer c.Stop(context.Background())
	confirmed, err := c.AskToConfirm("test")
	// no input
	r.Equal("EOF", err.Error())
	r.False(confirmed)
}

func TestAPIServiceClient(t *testing.T) {
	r := require.New(t)
	c, err := NewClient(config.Config{}, "")
	r.NoError(err)
	defer c.Stop(context.Background())

	apiServiceClient, err := c.APIServiceClient()
	r.Contains(err.Error(), `use "ioctl config set endpoint" to config endpoint first`)
	r.Nil(apiServiceClient)

	c.SetEndpointWithFlag(func(p *string, _ string, _ string, _ string) {
		*p = "127.0.0.1:14011"
	})
	c.SetInsecureWithFlag(func(p *bool, _ string, _ bool, _ string) {
		*p = true
	})
	apiServiceClient, err = c.APIServiceClient()
	r.NoError(err)
	r.NotNil(apiServiceClient)

	c.SetEndpointWithFlag(func(p *string, _ string, _ string, _ string) {
		*p = "127.0.0.1:14014"
	})
	c.SetInsecureWithFlag(func(p *bool, _ string, _ bool, _ string) {
		*p = false
	})
	apiServiceClient, err = c.APIServiceClient()
	r.NoError(err)
	r.NotNil(apiServiceClient)
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
		configFilePath := writeTempConfig(t, &test.cfg)
		cfgload := loadTempConfig(t, configFilePath)
		r.Equal(test.cfg, cfgload)

		c, err := NewClient(cfgload, configFilePath)
		r.NoError(err)
		out, err := c.AddressWithDefaultIfNotExist(test.in)
		if err != nil {
			r.Contains(err.Error(), test.errMsg)
		}
		r.Equal(test.out, out)
	}
}

func TestNewKeyStore(t *testing.T) {
	r := require.New(t)
	testWallet := t.TempDir()

	c, err := NewClient(config.Config{
		Wallet: testWallet,
	}, testWallet+"/config.default")
	r.NoError(err)
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

	configFilePath := writeTempConfig(t, &cfg)
	cfgload := loadTempConfig(t, configFilePath)
	r.Equal(cfg, cfgload)

	exprAliases := map[string]string{
		"io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95haaa": "aaa",
		"io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hbbb": "bbb",
		"io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hccc": "ccc",
	}
	c, err := NewClient(cfgload, configFilePath)
	r.NoError(err)
	defer c.Stop(context.Background())
	result := c.AliasMap()
	r.Equal(exprAliases, result)
}

func TestAlias(t *testing.T) {
	r := require.New(t)
	cfg := config.Config{
		Aliases: map[string]string{
			"aaa": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hc5r",
			"bbb": "io187evpmjdankjh0g5dfz83w2z3p23ljhn4s9jw7",
		},
	}
	configFilePath := writeTempConfig(t, &cfg)
	cfgload := loadTempConfig(t, configFilePath)
	r.Equal(cfg, cfgload)

	c, err := NewClient(cfgload, configFilePath)
	r.NoError(err)
	defer c.Stop(context.Background())
	for alias, addr := range cfg.Aliases {
		result, err := c.Alias(addr)
		r.NoError(err)
		r.Equal(alias, result)
	}
}

func TestSetAlias(t *testing.T) {
	type Data struct {
		cfg   config.Config
		alias string
		addr  string
	}
	tests := []Data{
		{
			config.Config{
				Endpoint:      "127.1.1.1:1234",
				SecureConnect: true,
				Aliases: map[string]string{
					"aaa": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95haaa",
					"bbb": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95haaa",
					"ccc": "io1cjh35tq9k8fu0gqcsat4px7yr8trcccccccccc",
				},
				DefaultAccount: config.Context{AddressOrAlias: "ddd"},
			},
			"ddd",
			"io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95haaa",
		},
		{
			config.Config{
				Endpoint:      "127.1.1.1:1234",
				SecureConnect: true,
				Aliases: map[string]string{
					"aaa": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95haaa",
					"bbb": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hbbb",
					"ccc": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hccc",
				},
				DefaultAccount: config.Context{AddressOrAlias: "ddd"},
			},
			"ddd",
			"io1cjh35tq9k8fu0gqcsat4px7yr8trhddddddddd",
		},
		{
			config.Config{
				Aliases: map[string]string{
					"": "",
				},
				DefaultAccount: config.Context{AddressOrAlias: ""},
			},
			"ddd",
			"",
		},
		{
			config.Config{
				Aliases: map[string]string{
					"eee": "",
				},
				DefaultAccount: config.Context{AddressOrAlias: ""},
			},
			"",
			"",
		},
		{
			config.Config{
				Aliases: map[string]string{
					"": "io1cjh35tq9k8fu0gqcsat4px7yr8trhddddddddd",
				},
				DefaultAccount: config.Context{AddressOrAlias: ""},
			},
			"ddd",
			"io1cjh35tq9k8fu0gqcsat4px7yr8trhddddddddd",
		},
	}

	r := require.New(t)
	testPathd := t.TempDir()

	for _, test := range tests {
		configFilePath := testPathd + "/config.default"
		c, err := NewClient(test.cfg, configFilePath)
		r.NoError(err)
		r.NoError(c.SetAliasAndSave(test.alias, test.addr))
		cfgload := loadTempConfig(t, configFilePath)
		count := 0
		for _, v := range cfgload.Aliases {
			if v == test.addr {
				count++
			}
		}
		r.Equal(1, count)
		r.Equal(test.addr, cfgload.Aliases[test.alias])
		r.Equal(test.cfg.Endpoint, cfgload.Endpoint)
		r.Equal(test.cfg.SecureConnect, cfgload.SecureConnect)
		r.Equal(test.cfg.DefaultAccount, cfgload.DefaultAccount)
	}
}

func TestDeleteAlias(t *testing.T) {
	type Data struct {
		cfg   config.Config
		alias string
	}
	tests := []Data{
		{
			config.Config{
				Endpoint:      "127.1.1.1:1234",
				SecureConnect: true,
				Aliases: map[string]string{
					"aaa": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95haaa",
					"bbb": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95haaa",
					"ccc": "io1cjh35tq9k8fu0gqcsat4px7yr8trcccccccccc",
				},
				DefaultAccount: config.Context{AddressOrAlias: "ddd"},
			},
			"aaa",
		},
		{
			config.Config{
				Endpoint:      "127.1.1.1:1234",
				SecureConnect: true,
				Aliases: map[string]string{
					"aaa": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95haaa",
					"bbb": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hbbb",
					"ccc": "io1cjh35tq9k8fu0gqcsat4px7yr8trh75c95hccc",
				},
				DefaultAccount: config.Context{AddressOrAlias: "ddd"},
			},
			"ddd",
		},
		{
			config.Config{
				Aliases: map[string]string{
					"": "",
				},
			},
			"ddd",
		},
	}

	r := require.New(t)
	testPathd := t.TempDir()

	for _, test := range tests {
		configFilePath := testPathd + "/config.default"
		c, err := NewClient(test.cfg, configFilePath)
		r.NoError(err)
		r.NoError(c.DeleteAlias(test.alias))
		cfgload := loadTempConfig(t, configFilePath)
		r.NotContains(cfgload.Aliases, test.alias)
		r.Equal(test.cfg.Endpoint, cfgload.Endpoint)
		r.Equal(test.cfg.SecureConnect, cfgload.SecureConnect)
		r.Equal(test.cfg.DefaultAccount, cfgload.DefaultAccount)
	}
}

func TestHdwalletMnemonic(t *testing.T) {
	r := require.New(t)
	testPathWallet := t.TempDir()
	c, err := NewClient(config.Config{
		Wallet: testPathWallet,
	}, testPathWallet+"/config.default")
	r.NoError(err)
	mnemonic := "lake stove quarter shove dry matrix hire split wide attract argue core"
	password := "123"
	r.NoError(c.WriteHdWalletConfigFile(mnemonic, password))
	result, err := c.HdwalletMnemonic(password)
	r.NoError(err)
	r.Equal(mnemonic, result)
}

func TestWriteHdWalletConfigFile(t *testing.T) {
	r := require.New(t)
	testPathWallet := t.TempDir()

	c, err := NewClient(config.Config{
		Wallet: testPathWallet,
	}, testPathWallet+"/config.default")
	r.NoError(err)
	mnemonic := "lake stove quarter shove dry matrix hire split wide attract argue core"
	password := "123"
	r.NoError(c.WriteHdWalletConfigFile(mnemonic, password))
}

func TestClient_ConfigFilePath(t *testing.T) {
	r := require.New(t)
	testConfigPath := fmt.Sprintf("%s/%s", t.TempDir(), "/config.test")

	c, err := NewClient(config.Config{}, testConfigPath)
	r.NoError(err)
	r.Equal(testConfigPath, c.ConfigFilePath())
}

func writeTempConfig(t *testing.T, cfg *config.Config) string {
	r := require.New(t)
	configFilePath := t.TempDir() + "/config.default"
	out, err := yaml.Marshal(cfg)
	r.NoError(err)
	r.NoError(os.WriteFile(configFilePath, out, 0600))
	return configFilePath
}

func loadTempConfig(t *testing.T, configFilePath string) config.Config {
	r := require.New(t)
	cfg := config.Config{
		Aliases: make(map[string]string),
	}
	in, err := os.ReadFile(configFilePath)
	r.NoError(err)
	r.NoError(yaml.Unmarshal(in, &cfg))
	return cfg
}
