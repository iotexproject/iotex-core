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
	ErrConfigNotMatch = "no config matchs"
	// ErrEmptyEndpoint indicates error for empty endpoint
	ErrEmptyEndpoint = "no endpoint has been set"
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
	Endpoint string            `yaml:"endpoint"`
	Wallet   string            `yaml:"wallet"`
	Aliases  map[string]string `yaml:"aliases"`
}

// ReadConfig represents the current config read from local
var ReadConfig Config

func init() {
	ConfigDir = os.Getenv("HOME") + "/.config/ioctl/default"
	if err := os.MkdirAll(ConfigDir, 0700); err != nil {
		log.L().Panic(err.Error())
	}
	DefaultConfigFile = ConfigDir + "/config.default"
	var err error
	ReadConfig, err = LoadConfig()
	if err != nil || ReadConfig.Wallet == "" {
		if !os.IsNotExist(err) || ReadConfig.Wallet == "" {
			ReadConfig.Wallet = ConfigDir
			out, err := yaml.Marshal(&ReadConfig)
			if err != nil {
				log.L().Panic(err.Error())
			}
			if err := ioutil.WriteFile(DefaultConfigFile, out, 0600); err != nil {
				log.L().Panic(fmt.Sprintf("Failed to write to config file %s.", DefaultConfigFile))
			}
		} else {
			log.L().Panic(err.Error())
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
