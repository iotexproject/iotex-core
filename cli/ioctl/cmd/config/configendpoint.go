// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"

	"github.com/spf13/cobra"
)

const endpointPrefix = "endpoint:"

// ErrEmptyEndpoint indicates error for empty endpoint
var ErrEmptyEndpoint = "no endpoint has been set"

// configGetEndpointCmd represents the config get endpoint command
var configGetEndpointCmd = &cobra.Command{
	Use:       "get",
	Short:     "Get endpoint for ioctl",
	ValidArgs: []string{"get", "endpoint"},
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
		fmt.Println(GetEndpoint())
	},
}

// configSetEndpointCmd represents the config set endpoint command
var configSetEndpointCmd = &cobra.Command{
	Use:   "set",
	Short: "Set endpoint for ioctl",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(setEndpoint(args))
	},
}

// GetEndpoint gets the endpoint
func GetEndpoint() string {
	cfg, err := LoadConfig()
	if err != nil {
		return err.Error()
	}
	if cfg.Endpoint == "" {
		return ErrEmptyEndpoint
	}
	return cfg.Endpoint
}

func setEndpoint(args []string) string {
	cfg, err := LoadConfig()
	if err != nil {
		return err.Error()
	}
	cfg.Endpoint = args[1]
	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return err.Error()
	}
	if err := ioutil.WriteFile(DefaultConfigFile, out, 0600); err != nil {
		return fmt.Sprintf("Failed to write to config file %s.", DefaultConfigFile)
	}
	return "Endpoint is set to " + args[1]
}
