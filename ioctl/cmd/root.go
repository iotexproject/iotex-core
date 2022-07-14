// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/cmd/bc"
	"github.com/iotexproject/iotex-core/ioctl/cmd/contract"
	"github.com/iotexproject/iotex-core/ioctl/cmd/did"
	"github.com/iotexproject/iotex-core/ioctl/cmd/hdwallet"
	"github.com/iotexproject/iotex-core/ioctl/cmd/jwt"
	"github.com/iotexproject/iotex-core/ioctl/cmd/node"
	"github.com/iotexproject/iotex-core/ioctl/cmd/update"
	"github.com/iotexproject/iotex-core/ioctl/cmd/version"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	_ioctlRootCmdShorts = map[config.Language]string{
		config.English: "Command-line interface for IoTeX blockchain",
		config.Chinese: "IoTeX区块链命令行工具",
	}
	_ioctlRootCmdLongs = map[config.Language]string{
		config.English: `ioctl is a command-line interface for interacting with IoTeX blockchain.`,
		config.Chinese: `ioctl 是用于与IoTeX区块链进行交互的命令行工具`,
	}
	_xctlRootCmdShorts = map[config.Language]string{
		config.English: "Command-line interface for consortium blockchain",
		config.Chinese: "联盟链命令行工具",
	}
	_xctlRootCmdLongs = map[config.Language]string{
		config.English: `xctl is a command-line interface for interacting with consortium blockchain.`,
		config.Chinese: `xctl 是用于与联盟链进行交互的命令行工具`,
	}
	_flagOutputFormatUsages = map[config.Language]string{
		config.English: "output format",
		config.Chinese: "指定输出格式",
	}
)

// NewIoctl returns ioctl root cmd
func NewIoctl() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "ioctl",
		Short: config.TranslateInLang(_ioctlRootCmdShorts, config.UILanguage),
		Long:  config.TranslateInLang(_ioctlRootCmdLongs, config.UILanguage),
	}

	rootCmd.AddCommand(config.ConfigCmd)
	rootCmd.AddCommand(account.AccountCmd)
	rootCmd.AddCommand(alias.AliasCmd)
	rootCmd.AddCommand(action.ActionCmd)
	rootCmd.AddCommand(action.Xrc20Cmd)
	rootCmd.AddCommand(action.Stake2Cmd)
	rootCmd.AddCommand(bc.BCCmd)
	rootCmd.AddCommand(node.NodeCmd)
	rootCmd.AddCommand(version.VersionCmd)
	rootCmd.AddCommand(update.UpdateCmd)
	rootCmd.AddCommand(contract.ContractCmd)
	rootCmd.AddCommand(did.DIDCmd)
	rootCmd.AddCommand(hdwallet.HdwalletCmd)
	rootCmd.AddCommand(jwt.JwtCmd)
	rootCmd.PersistentFlags().StringVarP(&output.Format, "output-format", "o", "",
		config.TranslateInLang(_flagOutputFormatUsages, config.UILanguage))

	return rootCmd
}

// NewXctl returns xctl root cmd
func NewXctl() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "xctl",
		Short: config.TranslateInLang(_xctlRootCmdShorts, config.UILanguage),
		Long:  config.TranslateInLang(_xctlRootCmdLongs, config.UILanguage),
	}

	rootCmd.AddCommand(config.ConfigCmd)
	rootCmd.AddCommand(account.AccountCmd)
	rootCmd.AddCommand(alias.AliasCmd)
	rootCmd.AddCommand(action.ActionCmd)
	rootCmd.AddCommand(action.Xrc20Cmd)
	rootCmd.AddCommand(bc.BCCmd)
	rootCmd.AddCommand(version.VersionCmd)
	rootCmd.AddCommand(contract.ContractCmd)
	// TODO: add xctl's UpdateCmd

	rootCmd.PersistentFlags().StringVarP(&output.Format, "output-format", "o", "",
		config.TranslateInLang(_flagOutputFormatUsages, config.UILanguage))

	return rootCmd
}
