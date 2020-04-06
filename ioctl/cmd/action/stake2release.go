package action

import (
	"encoding/hex"
	"math/big"

	"github.com/iotexproject/iotex-core/action"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/spf13/cobra"
)

// Multi-language support
var (
	stake2ReleaseCmdUses = map[config.Language]string{
		config.English: "release BUCKET_INDEX [DATA] " +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "release 桶索引 [数据] " +
			" [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}
	stake2ReleaseCmdShorts = map[config.Language]string{
		config.English: "Release bucket on IoTeX blockchain",
		config.Chinese: "发布IoTeX区块链上的存储桶",
	}
)

// stake2ReleaseCmd represents the stake2 release command
var stake2ReleaseCmd = &cobra.Command{
	Use:   config.TranslateInLang(releaseCmdUses, config.UILanguage),
	Short: config.TranslateInLang(releaseCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := stake2Release(args)
		return output.PrintError(err)
	},
}

func init() {
	registerWriteCommand(stake2ReleaseCmd)
}

func stake2Release(args []string) error {
	bucketIndex, ok := new(big.Int).SetString(args[0], 10)
	if !ok {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
	}
	index := bucketIndex.Uint64()

	data := []byte{}
	if len(args) == 2 {
		data = make([]byte, 2*len([]byte(args[1])))
		hex.Encode(data, []byte(args[1]))
	}
	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	sender, err := signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signer address", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce", err)
	}
	gasLimit := gasLimitFlag.Value().(uint64)
	s2r, err := action.NewUnstake(nonce, index, data, gasLimit, gasPriceRau)
	if err != nil || s2r == nil {
		return output.NewError(output.InstantiationError, "failed to make a Unstake  instance", err)
	}
	if gasLimit == 0 {
		gasLimit = action.ReclaimStakeBaseIntrinsicGas + action.ReclaimStakePayloadGas
	}

	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(s2r).Build(),
		sender)
}
