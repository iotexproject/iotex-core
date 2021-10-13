// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	generateCmdShorts = map[config.Language]string{
		config.English: "Generate DID document using private key from wallet",
		config.Chinese: "用钱包中的私钥产生DID document",
	}
	generateCmdUses = map[config.Language]string{
		config.English: "generate [-s SIGNER]",
		config.Chinese: "generate [-s 签署人]",
	}
)

// didGenerateCmd represents the generate command
var didGenerateCmd = &cobra.Command{
	Use:   config.TranslateInLang(generateCmdUses, config.UILanguage),
	Short: config.TranslateInLang(generateCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := generate()
		return output.PrintError(err)
	},
}

func init() {
	action.RegisterWriteCommand(didGenerateCmd)
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
	doc := newDIDDoc()
	ethAddress, err := util.IoAddrToEvmAddr(signer)
	if err != nil {
		return "", output.NewError(output.AddressError, "", err)
	}
	doc.ID = DIDPrefix + ethAddress.String()
	authentication := authenticationStruct{
		ID:         doc.ID + DIDOwner,
		Type:       DIDAuthType,
		Controller: doc.ID,
	}
	uncompressed := pri.PublicKey().Bytes()
	if len(uncompressed) == 33 && (uncompressed[0] == 2 || uncompressed[0] == 3) {
		authentication.PublicKeyHex = hex.EncodeToString(uncompressed)
	} else if len(uncompressed) == 65 && uncompressed[0] == 4 {
		lastNum := uncompressed[64]
		authentication.PublicKeyHex = hex.EncodeToString(uncompressed[1:33])
		if lastNum%2 == 0 {
			authentication.PublicKeyHex = "02" + authentication.PublicKeyHex
		} else {
			authentication.PublicKeyHex = "03" + authentication.PublicKeyHex
		}
	} else {
		return "", output.NewError(output.CryptoError, "invalid public key", nil)
	}

	doc.Authentication = append(doc.Authentication, authentication)
	msg, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", output.NewError(output.ConvertError, "", err)
	}

	sum := sha256.Sum256(msg)
	generatedMessage = string(msg) + "\n\nThe hex encoded SHA256 hash of the DID doc is:" + hex.EncodeToString(sum[:])
	return
}
