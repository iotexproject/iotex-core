// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"

	"github.com/iotexproject/iotex-address/address"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// Multi-language support
var (
	_registerCmdUses = map[config.Language]string{
		config.English: "register NAME (ALIAS|OPERATOR_ADDRESS) (ALIAS|REWARD_ADDRESS) (ALIAS|OWNER_ADDRESS) AMOUNT_IOTX STAKE_DURATION [DATA] [--auto-stake] [BLS-FLAGS] [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "register 名字 (别名|操作者地址）（别名|奖励地址）（别名|所有者地址）IOTX数量 质押持续时间 [数据] [--auto-stake] [BLS标志] [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}

	_registerCmdShorts = map[config.Language]string{
		config.English: "Register a candidate (with BLS proof-of-possession)",
		config.Chinese: "在IoTeX区块链上注册候选人（含 BLS 持有证明）",
	}

	// BLS PoP flags used by stake2 register.
	_registerBLSFlags blsPoPFlags
)

// _stake2RegisterCmd represents the stake2 register a candidate command.
//
// Positional args (BREAKING CHANGE — was 7/8 with BLS_PUBKEY at args[4]):
//
//	NAME OPERATOR REWARD OWNER AMOUNT DURATION [DATA]
//
// BLS_PUBKEY is now supplied through a flag along with the new PoP
// material. See blspop_helper.go for the three-option matrix.
var _stake2RegisterCmd = &cobra.Command{
	Use:   config.TranslateInLang(_registerCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_registerCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(6, 7),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := register(args)
		return output.PrintError(err)
	},
}

func init() {
	RegisterWriteCommand(_stake2RegisterCmd)
	_stake2RegisterCmd.Flags().BoolVar(&_stake2AutoStake, "auto-stake", false, config.TranslateInLang(_stake2FlagAutoStakeUsages, config.UILanguage))

	f := _stake2RegisterCmd.Flags()
	f.StringVar(&_registerBLSFlags.pubKeyHex, "bls-pubkey", "",
		"BLS public key (hex). Use with --bls-pop for Option 3 (explicit PoP). Pre-decoded validation runs locally.")
	f.StringVar(&_registerBLSFlags.popHex, "bls-pop", "",
		"BLS proof-of-possession (96 B hex). Pairs with --bls-pubkey for Option 3.")
	f.StringVar(&_registerBLSFlags.privKeyHex, "bls-priv-key", "",
		"BLS private key (32 B hex) — Option 2. Tool derives the pubkey + signs the PoP. WARNING: appears in shell history.")
	f.StringVar(&_registerBLSFlags.keystorePath, "bls-keystore", "",
		"BLS keystore path — placeholder; not yet implemented.")
	f.BoolVar(&_registerBLSFlags.autoConfirm, "yes", false,
		"Skip the auto-derive confirmation prompt (Option 1). Use for CI / scripted flows.")
}

func register(args []string) error {
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
	ownerAddrStr, err := util.Address(args[3])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get owner address", err)
	}
	ownerAddr, err := address.FromString(ownerAddrStr)
	if err != nil {
		return output.NewError(output.AddressError, "invalid owner address", err)
	}

	amountInRau, err := util.StringToRau(args[4], util.IotxDecimalNum)
	if err != nil {
		return output.NewError(output.ConvertError, "invalid amount", err)
	}

	stakeDuration, err := parseStakeDuration(args[5])
	if err != nil {
		return output.NewError(0, "", err)
	}
	duration := uint32(stakeDuration.Uint64())

	var payload []byte
	if len(args) == 7 {
		payload, err = hex.DecodeString(args[6])
		if err != nil {
			return output.NewError(output.ConvertError, "failed to decode data", err)
		}
	}

	sender, err := Signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signed address", err)
	}

	// Resolve the BLS pubkey + PoP via the three-option matrix.
	// candidateID at register time is the owner address; in the
	// non-collision case this becomes c.Identifier verbatim via
	// generateCandidateID's owner-first fast path.
	blsPubKey, blsPop, err := resolveBLSForRegister(&_registerBLSFlags, sender, account.PasswordByFlag(), ownerAddr)
	if err != nil {
		return err
	}

	gasLimit := _gasLimitFlag.Value().(uint64)
	if gasLimit == 0 {
		gasLimit = action.CandidateRegisterBaseIntrinsicGas +
			action.CandidateRegisterPayloadGas*uint64(len(payload))
	}

	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce ", err)
	}

	cr, err := action.NewCandidateRegisterWithBLS(name, operatorAddrStr, rewardAddrStr, ownerAddrStr,
		amountInRau.String(), duration, _stake2AutoStake, blsPubKey, blsPop, payload)
	if err != nil {
		return output.NewError(output.InstantiationError, "failed to make a candidateRegister instance", err)
	}

	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(cr).Build(),
		sender)
}
