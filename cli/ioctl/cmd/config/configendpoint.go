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
	"strings"

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

func init() {
	ConfigCmd.AddCommand(configGetEndpointCmd)
	ConfigCmd.AddCommand(configSetEndpointCmd)
}

// GetEndpoint gets the endpoint
func GetEndpoint() string {
	file, err := ioutil.ReadFile(configFileName)
	if err != nil {
		return fmt.Sprintf("failed to open config file %s", configFileName)
	}
	// find the line that specifies endpoint
	var endpoint string
	lines := strings.Split(string(file), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, endpointPrefix) {
			endpoint = strings.TrimPrefix(line, endpointPrefix)
			break
		}
	}
	if endpoint == "" {
		return ErrEmptyEndpoint
	}
	return endpoint
}

func setEndpoint(args []string) string {
	file, err := ioutil.ReadFile(configFileName)
	if err != nil {
		if os.IsNotExist(err) {
			// special case of empty config file just being created
			line := endpointPrefix + args[1]
			if err := ioutil.WriteFile(configFileName, []byte(line), 0644); err != nil {
				return fmt.Sprintf("failed to create config file %s", configFileName)
			}
			return "endpoint set to " + args[1]
		}
		return fmt.Sprintf("failed to open config file %s", configFileName)
	}
	// find the line that specifies endpoint
	findEndpoint := false
	lines := strings.Split(string(file), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, endpointPrefix) {
			lines[i] = endpointPrefix + args[1]
			findEndpoint = true
			break
		}
	}
	if !findEndpoint {
		lines = append(lines, endpointPrefix+args[1])
	}
	output := strings.Join(lines, "\n")
	if err := ioutil.WriteFile(configFileName, []byte(output), 0644); err != nil {
		return fmt.Sprintf("failed to write to config file %s", configFileName)
	}
	return "endpoint set to " + args[1]
}
