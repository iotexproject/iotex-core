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
	xrc20AllowanceCmdUses = map[config.Language]string{
		config.English: "allowance [-s SIGNER] (ALIAS|SPENDER_ADDRESS) -c ALIAS|CONTRACT_ADDRESS ",
		config.Chinese: "allowance [-s 签署人] (ALIAS|支出者地址) -c 别名|合约地址 ",
	}
	xrc20AllowanceCmdShorts = map[config.Language]string{
		config.English: "the amount which spender is still allowed to withdraw from owner",
		config.Chinese: "仍然允许支出者从所有者中提取的金额",
	}
)

// xrc20AllowanceCmd represents your signer limited amount on target address
var xrc20AllowanceCmd = &cobra.Command{
	Use:   config.TranslateInLang(xrc20AllowanceCmdUses, config.UILanguage),
	Short: config.TranslateInLang(xrc20AllowanceCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := allowance(args[0])
		return output.PrintError(err)
	},
}

func init() {
	RegisterWriteCommand(xrc20AllowanceCmd)
}

func allowance(arg string) error {
	caller, err := Signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signer address", err)
	}
	owner, err := alias.EtherAddress(caller)
	if err != nil {
		return output.NewError(output.AddressError, "failed to get owner address", err)
	}
	spender, err := alias.EtherAddress(arg)
	if err != nil {
		return output.NewError(output.AddressError, "failed to get spender address", err)
	}
	contract, err := xrc20Contract()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}

	bytecode, err := xrc20ABI.Pack("allowance", owner, spender)
	if err != nil {
		return output.NewError(output.ConvertError, "cannot generate bytecode from given command", err)
	}
	result, err := Read(contract, "0", bytecode)
	if err != nil {
		return output.NewError(0, "failed to read contract", err)
	}
	decimal, _ := new(big.Int).SetString(result, 16)
	message := amountMessage{RawData: result, Decimal: decimal.String()}
	fmt.Println(message.String())
	return err
}
