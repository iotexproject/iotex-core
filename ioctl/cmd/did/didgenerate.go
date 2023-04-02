// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package did

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	_generateCmdShorts = map[config.Language]string{
		config.English: "Generate DID document using private key from wallet",
		config.Chinese: "用钱包中的私钥产生DID document",
	}
	_generateCmdUses = map[config.Language]string{
		config.English: "generate [-s SIGNER]",
		config.Chinese: "generate [-s 签署人]",
	}
)

// _didGenerateCmd represents the generate command
var _didGenerateCmd = &cobra.Command{
	Use:   config.TranslateInLang(_generateCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_generateCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := generate()
		return output.PrintError(err)
	},
}

func init() {
	action.RegisterWriteCommand(_didGenerateCmd)
}

func generate() error {
	addr, err := action.Signer()
	if err != nil {
		return output.NewError(output.InputError, "failed to get signer addr", err)
	}
	fmt.Printf("Enter password #%s:\n", addr)
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.NewError(output.InputError, "failed to get password", err)
	}
	generatedMessage, err := generateFromSigner(addr, password)
	if err != nil {
		return output.NewError(output.KeystoreError, "failed to sign message", err)
	}
	output.PrintResult(generatedMessage)
	return nil
}

func generateFromSigner(signer, password string) (generatedMessage string, err error) {
	pri, err := account.PrivateKeyFromSigner(signer, password)
	if err != nil {
		return
	}

	publicKey := pri.EcdsaPrivateKey().(*ecdsa.PrivateKey).Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", output.NewError(output.ConvertError, "generate public key error", nil)
	}
	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	doc, err := NewDIDDoc(publicKeyBytes)
	if err != nil {
		return "", output.NewError(output.ConvertError, "", err)
	}
	msg, err := doc.Json()
	if err != nil {
		return "", output.NewError(output.ConvertError, "", err)
	}
	hash, err := doc.Hash()
	if err != nil {
		return "", output.NewError(output.ConvertError, "", err)
	}

	generatedMessage = msg + "\n\nThe hex encoded SHA256 hash of the DID doc is:" + hex.EncodeToString(hash[:])
	return
}
