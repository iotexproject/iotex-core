// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/bc"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/flag"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// --auth and --evm-chain-id together turn an otherwise-legacy write command
// (transfer, contract invoke) into an EIP-7702 SetCodeTx send via TX_CONTAINER
// encoding. The command is only legal on targets that have a `to` address —
// contract deploy paths do NOT register these flags.
var (
	_authFlagUsages = map[config.Language]string{
		config.English: "signed authorization as 0x-hex RLP (output of 'ioctl action sign-auth'); repeatable; presence switches the action to a SetCodeTx (EIP-7702)",
		config.Chinese: "已签名的授权, 0x-hex RLP 格式 (即 'ioctl action sign-auth' 的输出); 可重复; 一旦传入则该操作改用 EIP-7702 SetCodeTx",
	}
	_evmChainIDUsages = map[config.Language]string{
		config.English: "override EVM chain id when --auth is used (default: derive from iotex chain id, 1→4689 / 2→4690 / 3→4691)",
		config.Chinese: "使用 --auth 时指定 EVM chain id (默认根据 iotex chain id 推导: 1→4689 / 2→4690 / 3→4691)",
	}

	_evmChainIDFlag = flag.NewUint64VarP("evm-chain-id", "", 0, config.TranslateInLang(_evmChainIDUsages, config.UILanguage))

	// Bound by RegisterAuthFlags via cmd.Flags().StringArrayVar.
	_authValues []string
)

// RegisterAuthFlags registers --auth (repeatable) and --evm-chain-id on a
// write command that should accept EIP-7702 authorizations. Call after
// RegisterWriteCommand.
func RegisterAuthFlags(cmd *cobra.Command) {
	cmd.Flags().StringArrayVar(&_authValues, "auth", nil, config.TranslateInLang(_authFlagUsages, config.UILanguage))
	_evmChainIDFlag.RegisterCommand(cmd)
}

// ParsedAuthList decodes the current --auth flag values into typed authorizations.
// Returns nil when the flag was not given — caller treats that as "stay on the legacy path".
func ParsedAuthList() ([]types.SetCodeAuthorization, error) {
	if len(_authValues) == 0 {
		return nil, nil
	}
	out := make([]types.SetCodeAuthorization, 0, len(_authValues))
	for i, raw := range _authValues {
		blob, err := hex.DecodeString(util.TrimHexPrefix(raw))
		if err != nil {
			return nil, output.NewError(output.InputError,
				fmt.Sprintf("--auth #%d: invalid hex", i+1), err)
		}
		var auth types.SetCodeAuthorization
		if err := rlp.DecodeBytes(blob, &auth); err != nil {
			return nil, output.NewError(output.SerializationError,
				fmt.Sprintf("--auth #%d: failed to RLP-decode SetCodeAuthorization", i+1), err)
		}
		out = append(out, auth)
	}
	return out, nil
}

// resolveEVMChainID returns the EVM chain id to use for signing.
// If --evm-chain-id was set, that wins. Otherwise we derive from the iotex
// chain id using the canonical iotex-bootstrap mapping (1→4689 / 2→4690 / 3→4691).
// Returns an error for any other iotex chain id without an explicit override —
// silently guessing would produce signatures that don't verify on chain.
func resolveEVMChainID(iotexChainID uint32) (uint64, error) {
	if v := _evmChainIDFlag.Value().(uint64); v != 0 {
		return v, nil
	}
	switch iotexChainID {
	case 1:
		return 4689, nil
	case 2:
		return 4690, nil
	case 3:
		return 4691, nil
	default:
		return 0, output.NewError(output.InputError,
			fmt.Sprintf("unknown iotex chain id %d, set --evm-chain-id explicitly", iotexChainID), nil)
	}
}

// EthTxBuilder constructs and signs a go-ethereum *types.Transaction.
type EthTxBuilder func(senderECKey *ecdsa.PrivateKey, nonce uint64, evmChainID *big.Int) (*types.Transaction, error)

// SetCodeTxBuilder returns an EthTxBuilder that produces a signed EIP-7702 SetCodeTx.
// gasPrice is used for BOTH GasTipCap and GasFeeCap (the simplification keeps the
// existing --gas-price UX working for both legacy and SetCodeTx paths).
func SetCodeTxBuilder(to common.Address, value, gasPrice *big.Int, gasLimit uint64, data []byte, auths []types.SetCodeAuthorization) EthTxBuilder {
	return func(key *ecdsa.PrivateKey, nonce uint64, evmChainID *big.Int) (*types.Transaction, error) {
		chainID256, _ := uint256.FromBig(evmChainID)
		value256, overflow := uint256.FromBig(value)
		if overflow {
			return nil, fmt.Errorf("value overflows uint256")
		}
		gasPrice256, overflow := uint256.FromBig(gasPrice)
		if overflow {
			return nil, fmt.Errorf("gas-price overflows uint256")
		}
		return types.MustSignNewTx(key, types.LatestSignerForChainID(evmChainID), &types.SetCodeTx{
			ChainID:   chainID256,
			Nonce:     nonce,
			GasTipCap: gasPrice256,
			GasFeeCap: gasPrice256,
			Gas:       gasLimit,
			To:        to,
			Value:     value256,
			Data:      data,
			AuthList:  auths,
		}), nil
	}
}

// SignAndSendEthTx is the shared write path for SetCodeTx (and any future
// modern-tx-type extension). The caller supplies a builder closure that
// constructs and signs a go-ethereum tx; this helper handles key loading,
// nonce / chain-id discovery, TX_CONTAINER wrapping, confirmation, and gRPC
// SendAction.
func SignAndSendEthTx(signer string, build EthTxBuilder) error {
	prvKey, err := account.PrivateKeyFromSigner(signer, account.PasswordByFlag())
	if err != nil {
		return err
	}
	defer prvKey.Zero()
	ecKey, ok := prvKey.EcdsaPrivateKey().(*ecdsa.PrivateKey)
	if !ok {
		return output.NewError(output.CryptoError, "signer key is not an ECDSA key", nil)
	}

	chainMeta, err := bc.GetChainMeta()
	if err != nil {
		return output.NewError(0, "failed to get chain meta", err)
	}
	iotexChainID := chainMeta.GetChainID()

	evmChainID, err := resolveEVMChainID(iotexChainID)
	if err != nil {
		return err
	}

	signerAddr := signer
	if util.AliasIsHdwalletKey(signer) {
		signerAddr = prvKey.PublicKey().Address().String()
	} else {
		signerAddr, err = Signer()
		if err != nil {
			return output.NewError(output.AddressError, "failed to resolve signer", err)
		}
	}
	txNonce, err := nonce(signerAddr)
	if err != nil {
		return output.NewError(0, "failed to get nonce", err)
	}

	signedTx, err := build(ecKey, txNonce, new(big.Int).SetUint64(evmChainID))
	if err != nil {
		return output.NewError(output.RuntimeError, "failed to build eth tx", err)
	}

	rawBytes, err := signedTx.MarshalBinary()
	if err != nil {
		return output.NewError(output.SerializationError, "failed to marshal eth tx", err)
	}
	_, sig, pubkey, err := action.ExtractTypeSigPubkey(signedTx)
	if err != nil {
		return output.NewError(output.CryptoError, "failed to extract sig/pubkey", err)
	}
	core, err := action.EthRawToContainer(iotexChainID, hex.EncodeToString(rawBytes))
	if err != nil {
		return output.NewError(output.SerializationError, "failed to build tx container", err)
	}
	req := &iotextypes.Action{
		Core:         core,
		SenderPubKey: pubkey.Bytes(),
		Signature:    sig,
		Encoding:     iotextypes.Encoding_TX_CONTAINER,
	}
	selp, err := (&action.Deserializer{}).SetEvmNetworkID(uint32(evmChainID)).ActionToSealedEnvelope(req)
	if err != nil {
		return output.NewError(output.SerializationError, "failed to seal envelope", err)
	}

	if _yesFlag.Value() == false {
		info := printEthTxSummary(signedTx, signerAddr, evmChainID, txNonce)
		confirmed, err := output.Confirm(info + "\nPlease confirm your action.\n")
		if err != nil {
			return output.NewError(output.InputError, "use -y flag to skip confirmation", err)
		}
		if !confirmed {
			return nil
		}
	}

	resp, err := SendRawAndRespond(selp.Proto())
	if err != nil {
		return err
	}
	return outputActionInfo(resp.ActionHash)
}

func printEthTxSummary(tx *types.Transaction, sender string, evmChainID, nonce uint64) string {
	to := "(create)"
	if tx.To() != nil {
		to = tx.To().Hex()
	}
	out := fmt.Sprintf(
		"eth tx (TX_CONTAINER encoding):\n  type: %d\n  from: %s\n  to: %s\n  value: %s\n  nonce: %d\n  gas: %d\n  gasFeeCap: %s\n  gasTipCap: %s\n  evmChainID: %d\n  dataLen: %d",
		tx.Type(), sender, to, tx.Value().String(), nonce, tx.Gas(),
		tx.GasFeeCap().String(), tx.GasTipCap().String(), evmChainID, len(tx.Data()),
	)
	if al := tx.AccessList(); len(al) > 0 {
		out += fmt.Sprintf("\n  accessList: %d entries", len(al))
	}
	if auths := tx.SetCodeAuthorizations(); len(auths) > 0 {
		out += fmt.Sprintf("\n  authList: %d entries", len(auths))
		for i, a := range auths {
			out += fmt.Sprintf("\n    [%d] chainID=%s address=%s nonce=%d",
				i, a.ChainID.String(), a.Address.Hex(), a.Nonce)
		}
	}
	return out
}
