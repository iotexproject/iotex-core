// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"

	"github.com/spf13/cobra"
)

// TODO: use wallet config later
var configAddress = "ioaddress"

// accountBalanceCmd represents the account balance command
var accountBalanceCmd = &cobra.Command{
	Use:   "balance address",
	Short: "Get balance of an account",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(balance(args))
	},
}

// Balance gets balance of an IoTeX blockchain address
func balance(args []string) string {
	address := args[0]
	accountMeta, err := GetAccountMeta(address)
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("%s: %s", address, accountMeta.Balance)
}
