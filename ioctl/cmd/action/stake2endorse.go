package action

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	_stake2EndorseCmdUses = map[config.Language]string{
		config.English: "endorse BUCKET_INDEX" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "endorse 票索引" +
			" [-s 签署人] [-n NONCE] [-l GAS 限制] [-p GAS 价格] [-P 密码] [-y]",
	}
	_stake2UnEndorseCmdUses = map[config.Language]string{
		config.English: "unendorse BUCKET_INDEX" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "unendorse 票索引" +
			" [-s 签署人] [-n NONCE] [-l GAS 限制] [-p GAS 价格] [-P 密码] [-y]",
	}

	_stake2EndorseCmdShorts = map[config.Language]string{
		config.English: "Endorse bucket's candidate on IoTeX blockchain",
		config.Chinese: "在 IoTeX 区块链上背书候选人",
	}
	_stake2UnEndorseCmdShorts = map[config.Language]string{
		config.English: "UnEndorse bucket's candidate on IoTeX blockchain",
		config.Chinese: "在 IoTeX 区块链上撤销背书",
	}
)

var (
	// _stake2EndorseCmd represents the stake2 transfer command
	_stake2EndorseCmd = &cobra.Command{
		Use:   config.TranslateInLang(_stake2EndorseCmdUses, config.UILanguage),
		Short: config.TranslateInLang(_stake2EndorseCmdShorts, config.UILanguage),
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			err := stake2Endorse(args)
			return output.PrintError(err)
		},
	}
	_stake2UnEndorseCmd = &cobra.Command{
		Use:   config.TranslateInLang(_stake2UnEndorseCmdUses, config.UILanguage),
		Short: config.TranslateInLang(_stake2UnEndorseCmdShorts, config.UILanguage),
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			err := stake2UnEndorse(args)
			return output.PrintError(err)
		},
	}
)

func init() {
	RegisterWriteCommand(_stake2EndorseCmd)
	RegisterWriteCommand(_stake2UnEndorseCmd)
}

func stake2UnEndorse(args []string) error {
	bucketIndex, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
	}

	return doEndorsement(bucketIndex, false)
}

func stake2Endorse(args []string) error {
	bucketIndex, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
	}

	return doEndorsement(bucketIndex, true)
}

func doEndorsement(bucketIndex uint64, isEndorse bool) error {
	sender, err := Signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signed address", err)
	}

	gasLimit := _gasLimitFlag.Value().(uint64)
	if gasLimit == 0 {
		gasLimit = action.CandidateEndorsementBaseIntrinsicGas
	}

	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce ", err)
	}
	s2t := action.NewCandidateEndorsementLegacy(nonce, gasLimit, gasPriceRau, bucketIndex, isEndorse)
	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(s2t).Build(),
		sender)
}
