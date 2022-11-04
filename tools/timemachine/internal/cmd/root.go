// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/log"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "timemachine [command] [flags]",
	Short:  "timemachine is a command-line interface to go back specific block height.",
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.L().Fatal("failed to add cmd", zap.Error(err))
	}
}

func init() {
	rootCmd.AddCommand(getHeight)
	rootCmd.AddCommand(syncHeight)
	rootCmd.AddCommand(playNext)
	rootCmd.AddCommand(commitNext)
}