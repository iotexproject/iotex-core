// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package newcmd

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/account"
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
	_ioctlRootCmdUses = map[config.Language]string{
		config.English: "ioctl",
		config.Chinese: "ioctl",
	}
	_xctlRootCmdShorts = map[config.Language]string{
		config.English: "Command-line interface for consortium blockchain",
		config.Chinese: "联盟链命令行工具",
	}
	_xctlRootCmdLongs = map[config.Language]string{
		config.English: `xctl is a command-line interface for interacting with consortium blockchain.`,
		config.Chinese: `xctl 是用于与联盟链进行交互的命令行工具`,
	}
	_xctlRootCmdUses = map[config.Language]string{
		config.English: "xctl",
		config.Chinese: "xctl",
	}
)

// NewIoctl returns ioctl root cmd
func NewIoctl(client ioctl.Client) *cobra.Command {
	rootUses, _ := client.SelectTranslation(_ioctlRootCmdUses)
	rootShorts, _ := client.SelectTranslation(_ioctlRootCmdShorts)
	rootLongs, _ := client.SelectTranslation(_ioctlRootCmdLongs)

	rootCmd := &cobra.Command{
		Use:   rootUses,
		Short: rootShorts,
		Long:  rootLongs,
	}

	rootCmd.AddCommand(config.ConfigCmd)
	rootCmd.AddCommand(account.NewAccountCmd(client))

	return rootCmd
}

// NewXctl returns xctl root cmd
func NewXctl(client ioctl.Client) *cobra.Command {
	rootUses, _ := client.SelectTranslation(_xctlRootCmdUses)
	rootShorts, _ := client.SelectTranslation(_xctlRootCmdShorts)
	rootLongs, _ := client.SelectTranslation(_xctlRootCmdLongs)

	var rootCmd = &cobra.Command{
		Use:   rootUses,
		Short: rootShorts,
		Long:  rootLongs,
	}

	rootCmd.AddCommand(config.ConfigCmd)
	rootCmd.AddCommand(account.NewAccountCmd(client))

	return rootCmd
}
