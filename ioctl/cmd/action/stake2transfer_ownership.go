// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// Multi-language support
var (
	_stake2TransferOwnershipCmdUses = map[config.Language]string{
		config.English: "transferownership (ALIAS|NEWOWNER_ADDRESS) [DATA]" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "transferownership (别名|新所有人地址) [DATA]" +
			" [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}
	_stake2TransferOwnershipCmdShorts = map[config.Language]string{
		config.English: "Transfer Candidate Ownership on IoTeX blockchain",
		config.Chinese: "在IoTeX区块链上转让节点所有权",
	}
)

var _stake2TransferOwnershipCmd = &cobra.Command{
	Use:   config.TranslateInLang(_stake2TransferOwnershipCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_stake2TransferOwnershipCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := stake2TransferOwnership(args)
		return output.PrintError(err)
	},
}

func init() {
	RegisterWriteCommand(_stake2TransferOwnershipCmd)
}

func stake2TransferOwnership(args []string) error {
	ownerAddrStr, err := util.Address(args[0])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get operator address", err)
	}

	var payload []byte
	if len(args) == 2 {
		payload, err = hex.DecodeString(args[1])
		if err != nil {
			return output.NewError(output.ConvertError, "failed to decode data", err)
		}
	}

	sender, err := Signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signed address", err)
	}

	gasLimit := _gasLimitFlag.Value().(uint64)
	if gasLimit == 0 {
		gasLimit = action.CandidateTransferOwnershipBaseIntrinsicGas
	}

	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce ", err)
	}

	cto, err := action.NewCandidateTransferOwnership(ownerAddrStr, payload)
	if err != nil {
		return output.NewError(output.InstantiationError, "failed to make a candidateTransferOwnership instance", err)
	}
	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(cto).Build(),
		sender)
}
