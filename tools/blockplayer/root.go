// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package main

import (
	"os"

	cmd "github.com/iotexproject/iotex-core/tools/blockplayer/cmds"
	"github.com/spf13/cobra"
	// cmd "github.com/iotexproject/iotex-core/tools/blockplayer/cmds"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "blockplayer",
	Short: "blockplayer is a command-line interface for replay all txs in the block",
	// Long:  common.TranslateInLang(rootCmdLongs),
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	RootCmd.AddCommand(cmd.SyncHeight)
	RootCmd.AddCommand(cmd.GetHeight)
	RootCmd.AddCommand(cmd.ReplayNext)

	RootCmd.HelpFunc()
}
