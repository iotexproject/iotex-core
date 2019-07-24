// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// accountBalanceCmd represents the account balance command
var accountBalanceCmd = &cobra.Command{
	Use:   "balance [ALIAS|ADDRESS]",
	Short: "Get balance of an account",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := balance(args[0])
		return output.PrintError(err)
	},
}

type balanceMessage struct {
	Address string `json:"address"`
	Balance string `json:"balance"`
}

// balance gets balance of an IoTeX blockchain address
func balance(arg string) error {
	address, err := util.GetAddress(arg)
	if err != nil {
		return output.NewError(output.AddressError, "", err)
	}
	accountMeta, err := GetAccountMeta(address)
	if err != nil {
		return output.NewError(0, "", err) // TODO: undefined error
	}
	balance, ok := big.NewInt(0).SetString(accountMeta.Balance, 10)
	if !ok {
		return output.NewError(output.ConvertError, "", err)
	}
	message := balanceMessage{
		Address: address,
		Balance: util.RauToString(balance, util.IotxDecimalNum),
	}
	fmt.Println((message.String()))
	return nil
}

func (m *balanceMessage) String() string {
	if output.Format == "" {
		return fmt.Sprintf("%s: %s IOTX", m.Address, m.Balance)
	}
	return output.FormatString(output.Result, m)
}
