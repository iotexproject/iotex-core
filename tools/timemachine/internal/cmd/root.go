// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use: "timemachine [command] [flags]",
	Short: `timemachine is a command-line interface to go back specific block height.
steps:
1. mkdir -p ./bin/timemachine/etc
2. refer to iotex-bootstrap to download config.yaml and genesis.yaml
3. make build-timemachine
4. cd ./bin/timemachine
5. ./timemachine download mainnet 13000188
6、after download successful, edit ./etc/config.yaml and set path of chainDBPath trieDBPath gravityChainDB:dbPath(path of poll.db) separately.
7. ./timemachine try 13000188 --genesis-path ./etc/genesis.yaml --config-path ./etc/config.yaml
8. ./timemachine get --genesis-path ./etc/genesis.yaml --config-path ./etc/config.yaml`,
}

var (
	// _genesisPath is the path to the genesis file
	_genesisPath string
	// _configPath is the path to the config file
	_configPath string
)

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(download)
	rootCmd.AddCommand(get)
	rootCmd.AddCommand(try)

	get.Flags().StringVar(&_genesisPath, "genesis-path", "", "Genesis path")
	get.Flags().StringVar(&_configPath, "config-path", "", "Config path")
	try.Flags().StringVar(&_genesisPath, "genesis-path", "", "Genesis path")
	try.Flags().StringVar(&_configPath, "config-path", "", "Config path")
}
