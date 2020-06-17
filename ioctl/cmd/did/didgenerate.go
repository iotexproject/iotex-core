// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

var signer string

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
	flagSignerUsages = map[config.Language]string{
		config.English: "choose a signing account",
		config.Chinese: "选择一个签名账户",
	}
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
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
	generateCmd.Flags().StringVarP(&signer, "signer", "s", "", config.TranslateInLang(flagSignerUsages, config.UILanguage))
}

func generate() error {
	addr, err := util.GetAddress(signer)
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
	pri, err := account.LocalAccountToPrivateKey(signer, password)
	if err != nil {
		return
	}
	doc := newDIDDoc()
	ethAddress, err := util.IoAddrToEvmAddr(signer)
	if err != nil {
		return "", output.NewError(output.AddressError, "", err)
	}
	doc.ID = DIDPrefix + ethAddress.String()
	uncompressed := pri.PublicKey().HexString()
	x := uncompressed[2:66]
	last := uncompressed[129:]
	lastNum, err := strconv.ParseInt(last, 16, 64)
	if err != nil {
		return "", output.NewError(output.ConvertError, "", err)
	}
	var compressed string
	if lastNum%2 == 0 {
		compressed = "02" + x
	} else {
		compressed = "03" + x
	}
	authentication := authenticationStruct{
		ID:           doc.ID + DIDOwner,
		Type:         DIDAuthType,
		Controller:   doc.ID,
		PublicKeyHex: compressed,
	}
	doc.Authentication = append(doc.Authentication, authentication)
	msg, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", output.NewError(output.ConvertError, "", err)
	}
	generatedMessage = string(msg)
	return
}
