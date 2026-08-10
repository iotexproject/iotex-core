// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package account

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// Multi-language support
var (
	_accountBlsSignPoPCmdShorts = map[config.Language]string{
		config.English: "Sign a BLS proof-of-possession offline (air-gap friendly)",
		config.Chinese: "离线签名 BLS 持有证明（适合冷机器场景）",
	}
	_accountBlsSignPoPCmdLongs = map[config.Language]string{
		config.English: `Generate a BLS proof-of-possession for the given candidate identity,
without sending any transaction. Intended for air-gapped workflows
where the BLS private key never touches a network-connected machine —
sign the PoP here, transfer the resulting hex back to a hot machine,
and pass it to "ioctl stake2 register --bls-pubkey ... --bls-pop ...".`,
		config.Chinese: `为指定的 candidate 身份生成 BLS 持有证明，但不发送任何交易。
适合 air-gap 场景——BLS 私钥永不接触联网机器，在冷机器上签好 PoP，
把 hex 转回热机器，喂给 ioctl stake2 register --bls-pubkey ... --bls-pop ...`,
	}
	_accountBlsSignPoPCmdUse = map[config.Language]string{
		config.English: "bls-sign-pop --candidate-id ADDRESS (--bls-priv-key HEX | --signer ADDRESS)",
		config.Chinese: "bls-sign-pop --candidate-id 地址 (--bls-priv-key HEX | --signer 地址)",
	}

	_accountBlsSignPoPCandidateID string
	_accountBlsSignPoPBLSPrivKey  string
	_accountBlsSignPoPSigner      string
)

// _accountBlsSignPoPCmd produces a BLS PoP hex for a given candidateID.
//
// Two key sources:
//
//   --bls-priv-key HEX
//       Use a standalone BLS private key. Bytes are read directly; nothing
//       is decrypted, so this flow is safe to use on an offline machine.
//
//   --signer ADDRESS  (paired with -P PASSWORD)
//       Use the BLS key derived from an existing iotex account's ECDSA
//       private key — same scheme as `ioctl account blskey`. Requires
//       the keystore + password to be present.
//
// Exactly one of the two must be supplied. Output is the 96-byte PoP
// hex written to stdout (so it composes with shell redirection).
var _accountBlsSignPoPCmd = &cobra.Command{
	Use:   config.TranslateInLang(_accountBlsSignPoPCmdUse, config.UILanguage),
	Short: config.TranslateInLang(_accountBlsSignPoPCmdShorts, config.UILanguage),
	Long:  config.TranslateInLang(_accountBlsSignPoPCmdLongs, config.UILanguage),
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		return output.PrintError(runBlsSignPoP())
	},
}

func init() {
	RegisterPasswordFlag(_accountBlsSignPoPCmd)
	f := _accountBlsSignPoPCmd.Flags()
	f.StringVar(&_accountBlsSignPoPCandidateID, "candidate-id", "",
		"Candidate identifier address (the value c.GetIdentifier() returns on chain). At register time this is the owner address declared in the action.")
	f.StringVar(&_accountBlsSignPoPBLSPrivKey, "bls-priv-key", "",
		"BLS private key (32 B hex). Mutually exclusive with --signer.")
	f.StringVar(&_accountBlsSignPoPSigner, "signer", "",
		"iotex address whose ECDSA keystore is used to derive a BLS key (same algorithm as `ioctl account blskey`). Mutually exclusive with --bls-priv-key.")
}

func runBlsSignPoP() error {
	if _accountBlsSignPoPCandidateID == "" {
		return output.NewError(output.FlagError, "--candidate-id is required", nil)
	}
	candAddrStr, err := util.Address(_accountBlsSignPoPCandidateID)
	if err != nil {
		return output.NewError(output.AddressError, "invalid --candidate-id", err)
	}
	candAddr, err := address.FromString(candAddrStr)
	if err != nil {
		return output.NewError(output.AddressError, "invalid --candidate-id", err)
	}

	if (_accountBlsSignPoPBLSPrivKey == "") == (_accountBlsSignPoPSigner == "") {
		return output.NewError(output.FlagError,
			"exactly one of --bls-priv-key or --signer must be supplied", nil)
	}

	var blsSk *crypto.BLS12381PrivateKey
	switch {
	case _accountBlsSignPoPBLSPrivKey != "":
		b, err := hex.DecodeString(strings.TrimPrefix(_accountBlsSignPoPBLSPrivKey, "0x"))
		if err != nil {
			return output.NewError(output.ConvertError, "invalid --bls-priv-key hex", err)
		}
		blsSk, err = crypto.BLS12381PrivateKeyFromBytes(b)
		if err != nil {
			return output.NewError(output.ValidationError, "invalid BLS private key bytes", err)
		}
	case _accountBlsSignPoPSigner != "":
		signerStr, err := util.Address(_accountBlsSignPoPSigner)
		if err != nil {
			return output.NewError(output.AddressError, "invalid --signer", err)
		}
		ecdsaSk, err := PrivateKeyFromSigner(signerStr, PasswordByFlag())
		if err != nil {
			return output.NewError(output.KeystoreError, "failed to decrypt signer keystore", err)
		}
		blsSk, err = crypto.GenerateBLS12381PrivateKey(ecdsaSk.Bytes())
		ecdsaSk.Zero()
		if err != nil {
			return errors.Wrap(err, "failed to derive BLS key")
		}
	}
	defer blsSk.Zero()

	pop, err := staking.SignBLSPop(blsSk, candAddr)
	if err != nil {
		return errors.Wrap(err, "failed to sign BLS PoP")
	}

	// Print pubkey + PoP. Pubkey on stderr (informational) so stdout
	// stays exactly the PoP hex for clean shell redirection.
	fmt.Fprintf(os.Stderr, "BLS pubkey:    0x%s\n", hex.EncodeToString(blsSk.PublicKey().Bytes()))
	fmt.Fprintf(os.Stderr, "Candidate ID:  %s\n", candAddr.String())
	fmt.Fprintf(os.Stderr, "PoP bytes:     %d\n", len(pop))
	fmt.Println(hex.EncodeToString(pop))
	return nil
}
