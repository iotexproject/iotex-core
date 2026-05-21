// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/bc"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/flag"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

// Multi-language support
var (
	_signAuthCmdShorts = map[config.Language]string{
		config.English: "Sign an EIP-7702 SetCode authorization with the signer's key (offline, no broadcast)",
		config.Chinese: "用签署人的密钥签名一个 EIP-7702 SetCode 授权 (离线, 不广播)",
	}
	_signAuthCmdUses = map[config.Language]string{
		config.English: "sign-auth DELEGATE_ADDR_HEX -n NONCE [-s SIGNER] [-P PASSWORD] [--evm-chain-id N]",
		config.Chinese: "sign-auth 委托目标地址 -n NONCE [-s 签署人] [-P 密码] [--evm-chain-id N]",
	}
	_signAuthNonceUsages = map[config.Language]string{
		config.English: "authority's expected nonce (REQUIRED, no default)",
		config.Chinese: "授权人的预期 nonce (必填, 没有默认值)",
	}
)

// sign-auth uses a local nonce flag because its semantics differ from the shared
// _nonceFlag ("set nonce (default using pending nonce)") — here the user MUST
// supply the authority's expected nonce explicitly.
var _signAuthNonceFlag = flag.NewUint64VarP("nonce", "n", 0,
	config.TranslateInLang(_signAuthNonceUsages, config.UILanguage))

var _actionSignAuthCmd = &cobra.Command{
	Use:   config.TranslateInLang(_signAuthCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_signAuthCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		if !cmd.Flags().Changed("nonce") {
			return output.PrintError(output.NewError(output.InputError, "--nonce is required", nil))
		}
		return output.PrintError(signAuth(args))
	},
}

func init() {
	_signAuthNonceFlag.RegisterCommand(_actionSignAuthCmd)
	_signerFlag.RegisterCommand(_actionSignAuthCmd)
	account.RegisterPasswordFlag(_actionSignAuthCmd)
	_evmChainIDFlag.RegisterCommand(_actionSignAuthCmd)
}

func signAuth(args []string) error {
	delegateHex := args[0]
	if !common.IsHexAddress(delegateHex) {
		return output.NewError(output.AddressError,
			fmt.Sprintf("invalid delegate target address %q (expected 0x + 40 hex chars)", delegateHex), nil)
	}

	signer, err := Signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signer address", err)
	}
	prvKey, err := account.PrivateKeyFromSigner(signer, account.PasswordByFlag())
	if err != nil {
		return err
	}
	defer prvKey.Zero()
	ecKey, ok := prvKey.EcdsaPrivateKey().(*ecdsa.PrivateKey)
	if !ok {
		return output.NewError(output.CryptoError, "signer key is not an ECDSA key", nil)
	}

	evmChainID := _evmChainIDFlag.Value().(uint64)
	if evmChainID == 0 {
		chainMeta, err := bc.GetChainMeta()
		if err != nil {
			return output.NewError(0, "failed to get chain meta (pass --evm-chain-id to skip)", err)
		}
		evmChainID, err = resolveEVMChainID(chainMeta.GetChainID())
		if err != nil {
			return err
		}
	}

	auth, err := types.SignSetCode(ecKey, types.SetCodeAuthorization{
		ChainID: *uint256.NewInt(evmChainID),
		Address: common.HexToAddress(delegateHex),
		Nonce:   _signAuthNonceFlag.Value().(uint64),
	})
	if err != nil {
		return output.NewError(output.CryptoError, "failed to sign authorization", err)
	}

	encoded, err := rlp.EncodeToBytes(&auth)
	if err != nil {
		return output.NewError(output.SerializationError, "failed to RLP-encode authorization", err)
	}
	fmt.Println("0x" + hex.EncodeToString(encoded))
	return nil
}
