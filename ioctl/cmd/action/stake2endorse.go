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
	_stake2EndorseCmdUses = map[config.Language]string{
		config.English: "endorse BUCKET_INDEX" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "endorse 票索引" +
			" [-s 签署人] [-n NONCE] [-l GAS 限制] [-p GAS 价格] [-P 密码] [-y]",
	}
	_stake2IntentToRevokeCmdUses = map[config.Language]string{
		config.English: "intentToRevokeEndorsement BUCKET_INDEX" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "intentToRevokeEndorsement 票索引" +
			" [-s 签署人] [-n NONCE] [-l GAS 限制] [-p GAS 价格] [-P 密码] [-y]",
	}
	_stake2RevokeCmdUses = map[config.Language]string{
		config.English: "revokeEndorsement BUCKET_INDEX" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "revokeEndorsement 票索引" +
			" [-s 签署人] [-n NONCE] [-l GAS 限制] [-p GAS 价格] [-P 密码] [-y]",
	}

	_stake2EndorseCmdShorts = map[config.Language]string{
		config.English: "Endorse bucket's candidate on IoTeX blockchain",
		config.Chinese: "在 IoTeX 区块链上背书候选人",
	}
	_stake2IntentToRevokeCmdShorts = map[config.Language]string{
		config.English: "IntentToRevoke Endorsement on IoTeX blockchain",
		config.Chinese: "在 IoTeX 区块链上提交撤销背书意向",
	}
	_stake2RevokeCmdShorts = map[config.Language]string{
		config.English: "Revoke Endorsement on IoTeX blockchain",
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
	_stake2IntentToRevokeCmd = &cobra.Command{
		Use:   config.TranslateInLang(_stake2IntentToRevokeCmdUses, config.UILanguage),
		Short: config.TranslateInLang(_stake2IntentToRevokeCmdShorts, config.UILanguage),
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			err := stake2IntentToRevoke(args)
			return output.PrintError(err)
		},
	}
	_stake2RevokeCmd = &cobra.Command{
		Use:   config.TranslateInLang(_stake2RevokeCmdUses, config.UILanguage),
		Short: config.TranslateInLang(_stake2RevokeCmdShorts, config.UILanguage),
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			err := stake2Revoke(args)
			return output.PrintError(err)
		},
	}
)

func init() {
	RegisterWriteCommand(_stake2EndorseCmd)
	RegisterWriteCommand(_stake2IntentToRevokeCmd)
	RegisterWriteCommand(_stake2RevokeCmd)
}

func stake2Revoke(args []string) error {
	bucketIndex, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
	}

	return doEndorsement(bucketIndex, action.CandidateEndorsementOpRevoke)
}

func stake2IntentToRevoke(args []string) error {
	bucketIndex, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
	}

	return doEndorsement(bucketIndex, action.CandidateEndorsementOpIntentToRevoke)
}

func stake2Endorse(args []string) error {
	bucketIndex, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
	}

	return doEndorsement(bucketIndex, action.CandidateEndorsementOpEndorse)
}

func doEndorsement(bucketIndex uint64, op action.CandidateEndorsementOp) error {
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
	s2t, err := action.NewCandidateEndorsement(bucketIndex, op)
	if err != nil {
		return output.NewError(output.InstantiationError, "failed to make a candidate endorsement", err)
	}
	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(s2t).Build(),
		sender)
}
