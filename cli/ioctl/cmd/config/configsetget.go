// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// configGetCmd represents the config get endpoint command
var configGetCmd = &cobra.Command{
	Use:       "get name",
	Short:     "Get config from ioctl",
	ValidArgs: []string{"endpoint", "wallet"},
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("accepts 1 arg(s), received %d", len(args))
		}
		if err := cobra.OnlyValidArgs(cmd, args); err != nil {
			return err
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(Get(args[0]))
	},
}

// configSetCmd represents the config set endpoint command
var configSetCmd = &cobra.Command{
	Use:       "set name value",
	Short:     "Set config for ioctl",
	ValidArgs: []string{"endpoint", "wallet"},
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return fmt.Errorf("accepts 2 arg(s), received %d", len(args))
		}
		if err := cobra.OnlyValidArgs(cmd, args[:1]); err != nil {
			return err
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(set(args))
	},
}

// Get gets the endpoint
func Get(arg string) string {
	cfg, err := LoadConfig()
	if err != nil {
		return err.Error()
	}
	switch arg {
	case "endpoint":
		if cfg.Endpoint == "" {
			return ErrEmptyEndpoint
		}
		return cfg.Endpoint
	case "wallet":
		return cfg.Wallet
	default:
		return ErrConfigNotMatch
	}
}

func set(args []string) string {
	cfg, err := LoadConfig()
	if err != nil {
		return err.Error()
	}
	switch args[0] {
	case "endpoint":
		cfg.Endpoint = args[1]
	case "wallet":
		cfg.Wallet = args[1]
	default:
		return ErrConfigNotMatch
	}
	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return err.Error()
	}
	if err := ioutil.WriteFile(DefaultConfigFile, out, 0600); err != nil {
		return fmt.Sprintf("Failed to write to config file %s.", DefaultConfigFile)
	}
	return args[0] + " is set to " + args[1]
}
