// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	stake2TransferCmdUses = map[config.Language]string{
		config.English: "transfer (ALIAS|VOTE_ADDRESS) BUCKET_INDEX [DATA]" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "transfer (别名|投票地址) 桶索引 [数据]" +
			" [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}

	stake2TransferCmdShorts = map[config.Language]string{
		config.English: "transfer transfers stake ownership on IoTeX blockchain",
		config.Chinese: "在区块链上转移质押",
	}
)

// stake2TransferCmd represents the stake2 transfer command
var stake2TransferCmd = &cobra.Command{
	Use:   config.TranslateInLang(stake2TransferCmdUses, config.UILanguage),
	Short: config.TranslateInLang(stake2TransferCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(1, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := stake2Transfer(args)
		return output.PrintError(err)

	},
}

func init() {
	registerWriteCommand(stake2TransferCmd)
}

func stake2Transfer(args []string) error {
	voterAddrStr, err := util.Address(args[0])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get voter address", err)
	}

	bucketIndex, ok := new(big.Int).SetString(args[1], 10)
	if !ok {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
	}
	index := bucketIndex.Uint64()

	var payload []byte
	if len(args) == 3 {
		payload = make([]byte, 2*len([]byte(args[2])))
		hex.Encode(payload, []byte(args[2]))
	}

	sender, err := signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signed address", err)
	}

	gasLimit := gasLimitFlag.Value().(uint64)
	if gasLimit == 0 {
		gasLimit = action.MoveStakeBaseIntrinsicGas +
			action.MoveStakePayloadGas*uint64(len(payload))
	}

	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce ", err)
	}
	s2t, err := action.NewTransferStake(nonce, voterAddrStr, index, payload, gasLimit, gasPriceRau)

	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(s2t).Build(),
		sender)

}
