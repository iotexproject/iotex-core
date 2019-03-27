// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
)

// accountBalanceCmd represents the account balance command
var accountBalanceCmd = &cobra.Command{
	Use:   "balance (ALIAS|ADDRESS)",
	Short: "Get balance of an account",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(balance(args))
	},
}

// balance gets balance of an IoTeX blockchain address
func balance(args []string) string {
	address, err := alias.Address(args[0])
	if err != nil {
		return err.Error()
	}
	accountMeta, err := GetAccountMeta(address)
	if err != nil {
		return err.Error()
	}
	balance, ok := big.NewInt(0).SetString(accountMeta.Balance, 10)
	if !ok {
		return "failed to convert balance into big int"
	}
	return fmt.Sprintf("%s: %s IOTX", address,
		util.RauToString(balance, util.IotxDecimalNum))
}
