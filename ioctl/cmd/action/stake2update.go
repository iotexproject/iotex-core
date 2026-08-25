// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// Multi-language support
var (
	_stake2UpdateCmdUses = map[config.Language]string{
		config.English: "update NAME (ALIAS|OPERATOR_ADDRESS) (ALIAS|REWARD_ADDRESS) [BLS-FLAGS]" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "update 名字 (别名|操作者地址) (别名|奖励地址) [BLS标志]" +
			" [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}
	_stake2UpdateCmdShorts = map[config.Language]string{
		config.English: "Update candidate (BLS rotation requires --candidate-id; without BLS flags BLS is untouched)",
		config.Chinese: "更新候选人（BLS 旋转需 --candidate-id；不指定 BLS 标志时不动 BLS）",
	}

	_updateBLSFlags blsPoPFlags
)

// _stake2UpdateCmd updates a candidate. Positional args (BREAKING CHANGE —
// was 4 with BLS_PUBKEY at args[3]):
//
//	NAME OPERATOR REWARD
//
// BLS rotation is now opt-in via flags. Run without any BLS flag to
// touch only name / operator / reward. See blspop_helper.go for the
// three-option matrix governing BLS rotation, plus --bls-from-signer
// for the explicit "rotate to the signer-derived BLS key" opt-in.
var _stake2UpdateCmd = &cobra.Command{
	Use:   config.TranslateInLang(_stake2UpdateCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_stake2UpdateCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := stake2Update(args)
		return output.PrintError(err)
	},
}

func init() {
	RegisterWriteCommand(_stake2UpdateCmd)

	f := _stake2UpdateCmd.Flags()
	f.StringVar(&_updateBLSFlags.pubKeyHex, "bls-pubkey", "",
		"BLS public key (hex). Use with --bls-pop for Option 3 (explicit PoP).")
	f.StringVar(&_updateBLSFlags.popHex, "bls-pop", "",
		"BLS proof-of-possession (96 B hex). Pairs with --bls-pubkey for Option 3.")
	f.StringVar(&_updateBLSFlags.privKeyHex, "bls-priv-key", "",
		"BLS private key (32 B hex) — Option 2. Tool derives the pubkey + signs the PoP.")
	f.StringVar(&_updateBLSFlags.keystorePath, "bls-keystore", "",
		"BLS keystore path — placeholder; not yet implemented.")
	f.BoolVar(&_updateBLSFlags.fromSigner, "bls-from-signer", false,
		"Opt-in: rotate to the BLS key derived from the signer's ECDSA private key (Option 1 for update). Requires --candidate-id.")
	f.BoolVar(&_updateBLSFlags.autoConfirm, "yes", false,
		"Skip the --bls-from-signer confirmation prompt. Use for CI / scripted flows.")
	f.StringVar(&_updateBLSFlags.candidateIDStr, "candidate-id", "",
		"Candidate identifier address (c.GetIdentifier()) — required when rotating BLS. Future versions will resolve this via RPC from the signer.")
}

func stake2Update(args []string) error {
	name := args[0]
	if !action.IsValidCandidateName(name) {
		return output.NewError(output.ValidationError, "", action.ErrInvalidCanName)
	}

	operatorAddrStr, err := util.Address(args[1])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get operator address", err)
	}
	rewardAddrStr, err := util.Address(args[2])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get reward address", err)
	}

	sender, err := Signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signed address", err)
	}

	// Resolve BLS pubkey + PoP. Returns (nil, nil, nil) for Option 0
	// (no BLS flags) — the resulting action leaves c.BLSPubKey
	// unchanged on the handler side.
	blsPubKey, blsPop, err := resolveBLSForUpdate(&_updateBLSFlags, sender, account.PasswordByFlag())
	if err != nil {
		return err
	}

	gasLimit := _gasLimitFlag.Value().(uint64)
	if gasLimit == 0 {
		gasLimit = action.CandidateUpdateBaseIntrinsicGas
	}

	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce ", err)
	}

	var s2u *action.CandidateUpdate
	if len(blsPubKey) > 0 {
		s2u, err = action.NewCandidateUpdateWithBLS(name, operatorAddrStr, rewardAddrStr, blsPubKey, blsPop)
	} else {
		s2u, err = action.NewCandidateUpdate(name, operatorAddrStr, rewardAddrStr)
	}
	if err != nil {
		return output.NewError(output.InstantiationError, "failed to make a candidateUpdate instance", err)
	}
	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(s2u).Build(),
		sender)
}
