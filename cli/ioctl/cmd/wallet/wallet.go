// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package wallet

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// WalletCmd represents the wallet command
var WalletCmd = &cobra.Command{
	Use:   "wallet",
	Short: "Manage accounts",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Print: " + strings.Join(args, " "))
	},
}

func init() {
	WalletCmd.AddCommand(walletCreateCmd)
	WalletCmd.AddCommand(walletListCmd)
}
