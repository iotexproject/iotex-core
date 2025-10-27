// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package account

import (
	"encoding/hex"
	"fmt"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// Multi-language support
var (
	_accountBlsCmdShorts = map[config.Language]string{
		config.English: "Generate BLS key pair from account",
		config.Chinese: "从账户生成BLS密钥对",
	}
	_accountBlsCmdLongs = map[config.Language]string{
		config.English: `Generate a BLS (Boneh-Lynn-Shacham) key pair from an existing IoTeX account.
The account's ECDSA private key is used as input for BLS key generation.
BLS keys are used for cryptographic signatures and verification in consensus mechanisms.`,
		config.Chinese: `从现有IoTeX账户生成BLS（Boneh-Lynn-Shacham）密钥对。
账户的ECDSA私钥用作BLS密钥生成的输入。
BLS密钥用于共识机制中的加密签名和验证。`,
	}
	_accountBlsCmdUse = map[config.Language]string{
		config.English: "blskey (ALIAS|ADDRESS)",
		config.Chinese: "blskey (别名|地址)",
	}
)

// _accountBlsCmd represents the account bls command
var _accountBlsCmd = &cobra.Command{
	Use:   config.TranslateInLang(_accountBlsCmdUse, config.UILanguage),
	Short: config.TranslateInLang(_accountBlsCmdShorts, config.UILanguage),
	Long:  config.TranslateInLang(_accountBlsCmdLongs, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		err := generateAccountBLS(args[0])
		return output.PrintError(err)
	},
}

func init() {
	RegisterPasswordFlag(_accountBlsCmd)
}

// accountBlsKeyPair represents a BLS key pair
type accountBlsKeyPair struct {
	SourceAccount string `json:"sourceAccount"`
	PrivateKey    string `json:"privateKey"`
	PublicKey     string `json:"publicKey"`
}

// String returns the string representation of BLS key pair
func (m *accountBlsKeyPair) String() string {
	if output.Format == "" {
		return fmt.Sprintf(`BLS Key Pair Generated Successfully:
Source Account: %s
BLS Private Key: %s
BLS Public Key: %s`,
			m.SourceAccount, m.PrivateKey, m.PublicKey)
	}
	return output.FormatString(output.Result, m)
}

// generateAccountBLS generates a BLS private and public key pair from account
func generateAccountBLS(arg string) error {
	var (
		addr string
		err  error
	)

	// Get account address from argument (alias or address)
	if util.AliasIsHdwalletKey(arg) {
		addr = arg
	} else {
		addr, err = util.Address(arg)
		if err != nil {
			return output.NewError(output.AddressError, "failed to get address", err)
		}
	}

	// Get ECDSA private key from account
	ecdsaPrivKey, err := PrivateKeyFromSigner(addr, PasswordByFlag())
	if err != nil {
		return output.NewError(output.KeystoreError, "failed to get private key from keystore", err)
	}

	// Generate BLS key pair from ECDSA private key
	privateKey, publicKey, err := generateAccountBLSKeyPair(ecdsaPrivKey)
	if err != nil {
		return errors.Wrap(err, "failed to generate BLS key pair")
	}

	// Get ethereum address for display
	iotexAddr, err := util.Address(addr)
	if err != nil {
		iotexAddr = addr
	}
	ethAddr, err := address.FromString(iotexAddr)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert to ethereum address", err)
	}

	// Find alias for the address if exists
	alias := findAliasForAddress(iotexAddr)
	var addressDisplay string
	if alias != "" {
		addressDisplay = fmt.Sprintf("%s (%s)", ethAddr.Hex(), alias)
	} else {
		addressDisplay = ethAddr.Hex()
	}

	keyPair := &accountBlsKeyPair{
		SourceAccount: addressDisplay,
		PrivateKey:    privateKey,
		PublicKey:     publicKey,
	}

	// Print to console
	fmt.Println(keyPair.String())
	return nil
}

// generateAccountBLSKeyPair generates a BLS key pair using ECDSA private key as input
func generateAccountBLSKeyPair(ecdsaPrivKey crypto.PrivateKey) (string, string, error) {
	// Use ECDSA private key as Input Key Material (IKM) for BLS key generation
	privKey, err := crypto.GenerateBLS12381PrivateKey(ecdsaPrivKey.Bytes())
	if err != nil {
		return "", "", errors.Wrap(err, "failed to generate BLS private key")
	}

	privateKeyHex := hex.EncodeToString(privKey.Bytes())
	publicKeyHex := hex.EncodeToString(privKey.PublicKey().Bytes())

	return privateKeyHex, publicKeyHex, nil
}

// findAliasForAddress returns the alias for the given address if exists
func findAliasForAddress(addr string) string {
	for alias, aliasAddr := range config.ReadConfig.Aliases {
		if aliasAddr == addr {
			return alias
		}
	}
	return ""
}
