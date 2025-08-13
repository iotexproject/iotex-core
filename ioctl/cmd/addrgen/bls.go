// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package addrgen

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

// Multi-language support
var (
	_blsCmdShorts = map[config.Language]string{
		config.English: "Generate BLS private and public key pair",
		config.Chinese: "生成BLS私钥和公钥对",
	}
	_blsCmdLongs = map[config.Language]string{
		config.English: `Generate a new BLS (Boneh-Lynn-Shacham) private key and its corresponding public key.
BLS keys are used for cryptographic signatures and verification in consensus mechanisms.`,
		config.Chinese: `生成新的BLS（Boneh-Lynn-Shacham）私钥及其对应的公钥。
BLS密钥用于共识机制中的加密签名和验证。`,
	}
	_blsCmdUse = map[config.Language]string{
		config.English: "bls",
		config.Chinese: "bls",
	}
	_flagECDSAPrivateKey = map[config.Language]string{
		config.English: "ECDSA private key (hex string) to use as input for BLS key generation",
		config.Chinese: "用作BLS密钥生成输入的ECDSA私钥（十六进制字符串）",
	}
)

// _blsCmd represents the bls command
var _blsCmd = &cobra.Command{
	Use:   config.TranslateInLang(_blsCmdUse, config.UILanguage),
	Short: config.TranslateInLang(_blsCmdShorts, config.UILanguage),
	Long:  config.TranslateInLang(_blsCmdLongs, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(_ *cobra.Command, _ []string) error {
		err := generateBLS()
		return output.PrintError(err)
	},
}

var (
	_ecdsaPrivateKey string
)

func init() {
	_blsCmd.Flags().StringVarP(&_ecdsaPrivateKey, "ecdsa-key", "k", "",
		config.TranslateInLang(_flagECDSAPrivateKey, config.UILanguage))
	_blsCmd.MarkFlagRequired("ecdsa-key")
}

// blsKeyPair represents a BLS key pair
type blsKeyPair struct {
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
}

// generateBLS generates a BLS private and public key pair
func generateBLS() error {
	// Remove "0x" prefix if present
	ecdsaKeyHex := _ecdsaPrivateKey
	if strings.HasPrefix(ecdsaKeyHex, "0x") {
		ecdsaKeyHex = ecdsaKeyHex[2:]
	}

	// Validate hex format
	ecdsaKeyBytes, err := hex.DecodeString(ecdsaKeyHex)
	if err != nil {
		return errors.Wrap(err, "invalid ECDSA private key hex format")
	}
	ecdsaPrivKey, err := crypto.BytesToPrivateKey(ecdsaKeyBytes)
	if err != nil {
		return errors.Wrap(err, "failed to convert ECDSA private key bytes to key")
	}
	privateKey, publicKey, err := generateBLSKeyPair(ecdsaPrivKey)
	if err != nil {
		return errors.Wrap(err, "failed to generate BLS key pair")
	}

	keyPair := &blsKeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}

	// Print to console
	return printBLSKeys(keyPair)
}

// generateBLSKeyPair generates a BLS key pair using ECDSA private key as input
func generateBLSKeyPair(ecdsaPrivKey crypto.PrivateKey) (string, string, error) {
	// Use ECDSA private key as Input Key Material (IKM) for BLS key generation
	privKey, err := crypto.GenerateBLS12381PrivateKey(ecdsaPrivKey.Bytes())
	if err != nil {
		return "", "", errors.Wrap(err, "failed to generate BLS private key")
	}

	privateKeyHex := hex.EncodeToString(privKey.Bytes())
	publicKeyHex := hex.EncodeToString(privKey.PublicKey().Bytes())

	return privateKeyHex, publicKeyHex, nil
}

// printBLSKeys prints the BLS keys to console
func printBLSKeys(keyPair *blsKeyPair) error {
	message := fmt.Sprintf("BLS Key Pair Generated:\nPrivate Key: %s\nPublic Key: %s",
		keyPair.PrivateKey, keyPair.PublicKey)

	fmt.Println(message)
	return nil
}
