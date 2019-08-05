// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"flag"
	"os"

	"github.com/pkg/errors"
	uconfig "go.uber.org/config"

	"github.com/iotexproject/iotex-core/tools/bot/pkg/log"
)

func init() {
	flag.StringVar(&_overwritePath, "config-path", "config.yaml", "Config path")
}

var (
	// overwritePath is the path to the config file which overwrite default values
	_overwritePath string
)

var (
	// Default is the default config
	Default = Config{
		API: API{
			Url: "api.testnet.iotex.one:80",
		},
	}

	// ErrInvalidCfg indicates the invalid config value
	ErrInvalidCfg = errors.New("invalid config value")
)

// Network is the config struct for network package
type (
	// API is the api service config
	API struct {
		Url string `yaml:"url"`
	}
	// Config is the root config struct, each package's config should be put as its sub struct
	Config struct {
		API         API              `yaml:"api"`
		Log         log.GlobalConfig `yaml:"log"`
		RunInterval uint64           `yaml:"runInterval"`
		Transfer    transfer         `yaml:"transfer"`
		Wallet      string           `yaml:"wallet"`
		Xrc20       xrc20            `yaml:"xrc20"`
		Execution   execution        `yaml:"execution"`
	}
	transfer struct {
		From           []string `yaml:"from"`
		To             []string `yaml:"to"`
		AmountInRau    string   `yaml:"amountInRau"`
		GasLimit       uint64   `yaml:"gaslimit"`
		GasPrice       uint64   `yaml:"gasprice"`
		AlertThreshold uint64   `yaml:"alertThreshold"`
	}
	xrc20 struct {
		Contract       string   `yaml:"contract"`
		From           []string `yaml:"from"`
		To             []string `yaml:"to"`
		Amount         string   `yaml:"amount"` // amount in smallest unit
		GasLimit       uint64   `yaml:"gaslimit"`
		GasPrice       uint64   `yaml:"gasprice"`
		AlertThreshold uint64   `yaml:"alertThreshold"`
	}
	execution struct {
		Contract       string   `yaml:"contract"`
		From           []string `yaml:"from"`
		Amount         string   `yaml:"amount"` // amount in smallest unit
		GasLimit       uint64   `yaml:"gaslimit"`
		GasPrice       uint64   `yaml:"gasprice"`
		AlertThreshold uint64   `yaml:"alertThreshold"`
		Data           string   `yaml:"data"`
	}
)

func New() (Config, error) {
	opts := make([]uconfig.YAMLOption, 0)
	opts = append(opts, uconfig.Static(Default))
	opts = append(opts, uconfig.Expand(os.LookupEnv))
	if _overwritePath != "" {
		opts = append(opts, uconfig.File(_overwritePath))
	}

	yaml, err := uconfig.NewYAML(opts...)
	if err != nil {
		return Config{}, errors.Wrap(err, "failed to init config")
	}

	var cfg Config
	if err := yaml.Get(uconfig.Root).Populate(&cfg); err != nil {
		return Config{}, errors.Wrap(err, "failed to unmarshal YAML config to struct")
	}

	return cfg, nil
}
