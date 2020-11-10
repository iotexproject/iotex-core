// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	balanceCmdUses = map[config.Language]string{
		config.English: "balanceOf (ALIAS|OWNER_ADDRESS) -c ALIAS|CONTRACT_ADDRESS",
		config.Chinese: "balanceOf (别名|所有人地址) -c 别名地址|合约地址",
	}
	balanceCmdShorts = map[config.Language]string{
		config.English: "Get account balance",
		config.Chinese: "获取账户余额",
	}
)

// xrc20BalanceOfCmd represents balanceOf function
var xrc20BalanceOfCmd = &cobra.Command{
	Use:   config.TranslateInLang(balanceCmdUses, config.UILanguage),
	Short: config.TranslateInLang(balanceCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := balanceOf(args[0])
		return output.PrintError(err)
	},
}

func balanceOf(arg string) error {
	owner, err := alias.EtherAddress(arg)
	if err != nil {
		return output.NewError(output.AddressError, "failed to get owner address", err)
	}
	bytecode, err := xrc20ABI.Pack("balanceOf", owner)
	if err != nil {
		return output.NewError(output.ConvertError, "cannot generate bytecode from given command", err)
	}
	contract, err := xrc20Contract()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}
	result, err := Read(contract, "0", bytecode)
	if err != nil {
		return output.NewError(0, "failed to read contract", err)
	}
	decimal, _ := new(big.Int).SetString(result, 16)
	if result == "" {
		result = "0"
	}
	if decimal == nil {
		decimal = big.NewInt(0)
	}
	message := amountMessage{RawData: result, Decimal: decimal.String()}
	fmt.Println(message.String())
	return err
}
