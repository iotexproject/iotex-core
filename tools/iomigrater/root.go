// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/tools/iomigrater/cmds"
	"github.com/iotexproject/iotex-core/tools/iomigrater/common"
)

// Multi-language support
var (
	rootCmdShorts = map[string]string{
		"english": "Command-line interface for migration of IoTeX blockchain db file",
		"chinese": "IoTeX区块链db文件的迁移命令行工具",
	}
	rootCmdLongs = map[string]string{
		"english": `iomigrater is a command-line interface for interacting with IoTeX blockchain db file.`,
		"chinese": `ioctl 是用于与IoTeX区块链 db 文件进行交互的命令行工具`,
	}
	rootCmdUses = map[string]string{
		"english": "iomigrater",
		"chinese": "iomigrater",
	}
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   common.TranslateInLang(rootCmdUses),
	Short: common.TranslateInLang(rootCmdShorts),
	Long:  common.TranslateInLang(rootCmdLongs),
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	RootCmd.AddCommand(cmd.CheckHeight)
	RootCmd.AddCommand(cmd.MigrateDb)

	RootCmd.HelpFunc()
}
