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
1. mkdir ./tools/timemachine/etc
   curl https://raw.githubusercontent.com/iotexproject/iotex-bootstrap/v1.8.4/config_mainnet.yaml > ./tools/timemachine/etc/config.yaml
   curl https://raw.githubusercontent.com/iotexproject/iotex-bootstrap/v1.8.4/genesis_mainnet.yaml > ./tools/timemachine/etc/genesis.yaml
2. make build-timemachine
3. ./bin/timemachine download 13000188
4、after download successful, edit ./tools/timemachine/etc/config.yaml and set chainDBPath trieDBPath gravityChainDB:dbPath, eg: ./tools/timemachine/data/13m/chain-00000012.db
5. ./bin/timemachine try 13000188
6. ./bin/timemachine commit 13000188
7. ./bin/timemachine get`,
}

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
	rootCmd.AddCommand(commit)
}
