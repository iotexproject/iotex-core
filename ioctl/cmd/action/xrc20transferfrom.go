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
	_xrc20TransferFromCmdUses = map[config.Language]string{
		config.English: "transferFrom (ALIAS|OWNER_ADDRESS) (ALIAS|RECIPIENT_ADDRESS) AMOUNT -c (ALIAS|CONTRACT_ADDRESS)" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "transferFrom (别名|所有人地址) (别名|接收人地址) 数量 -c (" +
			"别名|合约地址)" +
			" [-s 签署] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}
	_xrc20TransferFromCmdShorts = map[config.Language]string{
		config.English: "Send amount of tokens from owner address to target address",
		config.Chinese: "将通证数量从所有者地址发送到目标地址",
	}
)

// _xrc20TransferFromCmd could transfer from owner address to target address
var _xrc20TransferFromCmd = &cobra.Command{
	Use:   config.TranslateInLang(_xrc20TransferFromCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_xrc20TransferFromCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := transferFrom(args)
		return output.PrintError(err)
	},
}

func init() {
	RegisterWriteCommand(_xrc20TransferFromCmd)
}

func transferFrom(args []string) error {
	owner, err := alias.EtherAddress(args[0])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get owner address", err)
	}
	recipient, err := alias.EtherAddress(args[1])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get recipient address", err)
	}
	contract, err := xrc20Contract()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}
	amount, err := parseAmount(contract, args[2])
	if err != nil {
		return output.NewError(0, "failed to parse amount", err)
	}
	bytecode, err := _xrc20ABI.Pack("transferFrom", owner, recipient, amount)
	if err != nil {
		return output.NewError(output.ConvertError, "cannot generate bytecode from given command", err)
	}
	return Execute(contract.String(), big.NewInt(0), bytecode)
}
