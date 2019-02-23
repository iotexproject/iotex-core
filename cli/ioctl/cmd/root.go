// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/bc"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/version"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/wallet"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "ioctl",
	Short: "Command-line interface for IoTeX blockchain",
	Long:  `ioctl is a command-line interface for interacting with IoTeX blockchain.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
func init() {
	RootCmd.AddCommand(account.AccountCmd)
	RootCmd.AddCommand(version.VersionCmd)
	RootCmd.AddCommand(bc.BCCmd)
	RootCmd.AddCommand(wallet.WalletCmd)
}
