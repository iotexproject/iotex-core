// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	_stake2ReclaimCmdUses = map[config.Language]string{
		config.English: "reclaim BUCKET_INDEX TYPE" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "reclaim 票索引 类型" +
			" [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}

	_stake2ReclaimCmdShorts = map[config.Language]string{
		config.English: "Reclaim bucket on IoTeX blockchain",
		config.Chinese: "认领IoTeX区块链上的投票",
	}
)

// _stake2ReclaimCmd represents the stake2 reclaim command
var _stake2ReclaimCmd = &cobra.Command{
	Use:   config.TranslateInLang(_stake2ReclaimCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_stake2ReclaimCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := stake2Reclaim(args)
		return output.PrintError(err)
	},
}

func init() {
	RegisterWriteCommand(_stake2ReclaimCmd)
}

func stake2Reclaim(args []string) error {
	bucketIndex, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
	}

	sender, err := Signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signed address", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce ", err)
	}

	// construct consignment message
	msg, err := action.NewConsignMsg(args[1], sender, bucketIndex, nonce)
	if err != nil {
		return output.NewError(output.InputError, "failed to create reclaim message", err)
	}
	fmt.Printf("Here's the reclaim message:\n\n")
	fmt.Println(string(msg))

	// ask user to input signature
	sig, err := readSigFromStdin(args)
	if err != nil {
		return output.NewError(output.InputError, "failed to generate signature", err)
	}
	fmt.Println()

	// construct consignment JSON
	payload, err := action.NewConsignJSON(args[1], sender, sig, bucketIndex, nonce)
	if err != nil {
		return output.NewError(output.InputError, "failed to create reclaim JSON", err)
	}

	gasLimit := _gasLimitFlag.Value().(uint64)
	if gasLimit == 0 {
		gasLimit = action.MoveStakeBaseIntrinsicGas +
			action.MoveStakePayloadGas*uint64(len(payload))
	}

	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}

	s2t, err := action.NewTransferStake(nonce, sender, bucketIndex, payload, gasLimit, gasPriceRau)
	if err != nil {
		return output.NewError(output.InstantiationError, "failed to make a transferStake instance", err)
	}
	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(s2t).Build(),
		sender)
}

func readSigFromStdin(args []string) (string, error) {
	sig := "\n1. Use HDWallet or off-line sign this message on " + args[1] + " using the private key of bucket " + args[0] + "'s owner"
	fmt.Printf("%s", sig)
	fmt.Printf("\n2. Paste the signature, and hit Enter:\n")

	reader := bufio.NewReader(os.Stdin)
	sig, err := reader.ReadString('\n')
	if err != nil {
		return "", output.NewError(output.InputError, "failed to read signature", err)
	}
	if len(sig) <= 2 {
		return "", output.NewError(output.InputError, "invalid signature", err)
	}

	// remove possible 0x at beginning and the trailing \n
	if sig[0] == '0' && sig[1] == 'x' {
		sig = sig[2:]
	}
	sig = sig[:len(sig)-1]
	if _, err = hex.DecodeString(sig); err != nil {
		return "", output.NewError(output.ConvertError, "failed to decode signature", err)
	}
	return sig, nil
}
