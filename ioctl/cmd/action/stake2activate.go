package action

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

// Multi-language support
var (
	_stake2ActivateCmdUses = map[config.Language]string{
		config.English: "activate BUCKET_INDEX" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "activate 票索引" +
			" [-s 签署人] [-n NONCE] [-l GAS 限制] [-p GAS 价格] [-P 密码] [-y]",
	}

	_stake2ActivateCmdShorts = map[config.Language]string{
		config.English: "Activate candidate on IoTeX blockchain",
		config.Chinese: "在 IoTeX 区块链上激活质押票的候选人",
	}
)

var (
	// _stake2ActivateCmd represents the stake2 transfer command
	_stake2ActivateCmd = &cobra.Command{
		Use:   config.TranslateInLang(_stake2ActivateCmdUses, config.UILanguage),
		Short: config.TranslateInLang(_stake2ActivateCmdShorts, config.UILanguage),
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			err := stake2Activate(args)
			return output.PrintError(err)
		},
	}
)

func init() {
	RegisterWriteCommand(_stake2ActivateCmd)
}

func stake2Activate(args []string) error {
	bucketIndex, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
	}

	sender, err := Signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signed address", err)
	}

	gasLimit := _gasLimitFlag.Value().(uint64)
	if gasLimit == 0 {
		gasLimit = action.CandidateActivateBaseIntrinsicGas
	}

	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce ", err)
	}
	s2t := action.NewCandidateActivate(bucketIndex)
	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(s2t).Build(),
		sender)
}
