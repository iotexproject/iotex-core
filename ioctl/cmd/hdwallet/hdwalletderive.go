// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	hdwalletDeriveCmdShorts = map[config.Language]string{
		config.English: "create hdwallet account using hdwallet derive key",
		config.Chinese: "通过派生key生成钱包账号",
	}
	hdwalletDeriveCmdUses = map[config.Language]string{
		config.English: "derive id1/id2[/id3]",
		config.Chinese: "derive id1/id2[/id3]",
	}
)

// hdwalletDeriveCmd represents the hdwallet derive command
var hdwalletDeriveCmd = &cobra.Command{
	Use:   config.TranslateInLang(hdwalletDeriveCmdUses, config.UILanguage),
	Short: config.TranslateInLang(hdwalletDeriveCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := hdwalletDerive(args[0])
		return output.PrintError(err)
	},
}

func hdwalletDerive(path string) error {
	signer := "hdw::" + path
	account, change, index, err := util.ParseHdwPath(signer)
	if err != nil {
		return output.NewError(output.InputError, "invalid hdwallet key format", err)
	}

	output.PrintQuery("Enter password\n")
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.NewError(output.InputError, "failed to get password", err)
	}

	_, pri, err := DeriveKey(account, change, index, password)
	if err != nil {
		return err
	}

	addr, err := address.FromBytes(hashECDSAPublicKey(pri.PublicKey().EcdsaPublicKey().(*ecdsa.PublicKey)))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert public key into address", err)
	}
	output.PrintResult(fmt.Sprintf("address: %s\n", addr.String()))

	return nil
}
