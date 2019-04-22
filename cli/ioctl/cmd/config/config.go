// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/pkg/log"
)

// Directories
var (
	// ConfigDir is the directory to store config file
	ConfigDir string
	// DefaultConfigFile is the default config file name
	DefaultConfigFile string
)

// Error strings
var (
	// ErrConfigNotMatch indicates error for no config matchs
	ErrConfigNotMatch = fmt.Errorf("no config matchs")
	// ErrEmptyEndpoint indicates error for empty endpoint
	ErrEmptyEndpoint = fmt.Errorf("no endpoint has been set")
)

// ConfigCmd represents the config command
var ConfigCmd = &cobra.Command{
	Use:       "config",
	Short:     "Set or get configuration for ioctl",
	ValidArgs: []string{"set", "get"},
	Args:      cobra.MinimumNArgs(1),
}

// Config defines the config schema
type Config struct {
	Wallet        string            `yaml:"wallet"`
	Endpoint      string            `yaml:"endpoint"`
	SecureConnect bool              `yaml:"secureConnect"`
	Aliases       map[string]string `yaml:"aliases"`
}

var (
	// ReadConfig represents the current config read from local
	ReadConfig Config
	// Insecure represents the insecure connect option of grpc dial, default is false
	Insecure = false
)

func init() {
	ConfigDir = os.Getenv("HOME") + "/.config/ioctl/default"
	if err := os.MkdirAll(ConfigDir, 0700); err != nil {
		log.L().Panic(err.Error())
	}
	DefaultConfigFile = ConfigDir + "/config.default"
	var err error
	ReadConfig, err = LoadConfig()
	if err != nil || len(ReadConfig.Wallet) == 0 {
		if err != nil && !os.IsNotExist(err) {
			log.L().Panic(err.Error()) // Config file exists but error occurs
		}
		ReadConfig.Wallet = ConfigDir
		if os.IsNotExist(err) {
			ReadConfig.SecureConnect = true
		}
		out, err := yaml.Marshal(&ReadConfig)
		if err != nil {
			log.L().Panic(err.Error())
		}
		if err := ioutil.WriteFile(DefaultConfigFile, out, 0600); err != nil {
			log.L().Panic(fmt.Sprintf("Failed to write to config file %s.", DefaultConfigFile))
		}

	}
	ConfigCmd.AddCommand(configGetCmd)
	ConfigCmd.AddCommand(configSetCmd)
}

// LoadConfig loads config file in yaml format
func LoadConfig() (Config, error) {
	ReadConfig := Config{
		Aliases: make(map[string]string),
	}
	in, err := ioutil.ReadFile(DefaultConfigFile)
	if err == nil {
		if err := yaml.Unmarshal(in, &ReadConfig); err != nil {
			return ReadConfig, err
		}
	}
	return ReadConfig, err
}
