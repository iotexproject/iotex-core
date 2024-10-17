// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package did

import (
	"crypto/ecdsa"
	"encoding/hex"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
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
	key, _, err := loadPrivateKey()
	if err != nil {
		return err
	}
	generatedMessage, err := generateFromSigner(key)
	if err != nil {
		return err
	}
	output.PrintResult(generatedMessage)
	return nil
}

func generateFromSigner(key *ecdsa.PrivateKey) (generatedMessage string, err error) {
	publicKey, err := loadPublicKey(key)
	if err != nil {
		return "", err
	}
	doc, err := NewDIDDoc(publicKey)
	if err != nil {
		return "", output.NewError(output.ConvertError, "", err)
	}
	msg, err := doc.JSON()
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
