// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	// ConfigDir is the directory to store config file
	ConfigDir      string
	configFileName string
)

// ConfigCmd represents the config command
var ConfigCmd = &cobra.Command{
	Use:       "config",
	Short:     "Set or get configuration for ioctl",
	ValidArgs: []string{"set", "get"},
	Args:      cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Print: " + strings.Join(args, " "))
	},
}

func init() {
	ConfigDir = os.Getenv("HOME") + "/.config/ioctl"
	if err := os.MkdirAll(ConfigDir, 0700); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	configFileName = ConfigDir + "/config.default"
}
