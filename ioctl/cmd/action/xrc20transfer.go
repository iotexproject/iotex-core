// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	_xrc20TransferCmdUses = map[config.Language]string{
		config.English: "transfer (ALIAS|TARGET_ADDRESS) AMOUNT -c ALIAS|CONTRACT_ADDRESS [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "transfer (别名|目标地址) 数量 -c 别名|合约地址 [-s 签署人" +
			"] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}
	_xrc20TransferCmdShorts = map[config.Language]string{
		config.English: "Transfer token to the target address",
		config.Chinese: "将通证转移至目标地址",
	}
)

// _xrc20TransferCmd could do transfer action
var _xrc20TransferCmd = &cobra.Command{
	Use:   config.TranslateInLang(_xrc20TransferCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_xrc20TransferCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := xrc20Transfer(args)
		return output.PrintError(err)
	},
}

func init() {
	RegisterWriteCommand(_xrc20TransferCmd)
}

func xrc20Transfer(args []string) error {
	recipient, err := alias.EtherAddress(args[0])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get recipient address", err)
	}
	contract, err := xrc20Contract()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}
	amount, err := parseAmount(contract, args[1])
	if err != nil {
		return output.NewError(0, "failed to parse amount", err)
	}
	bytecode, err := _xrc20ABI.Pack("transfer", recipient, amount)
	if err != nil {
		return output.NewError(output.ConvertError, "cannot generate bytecode from given command", err)
	}
	return Execute(contract.String(), big.NewInt(0), bytecode)
}
