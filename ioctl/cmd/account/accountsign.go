// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

var signer string

// Multi-language support
var (
	signCmdShorts = map[config.Language]string{
		config.English: "Sign message with private key from wallet",
		config.Chinese: "用钱包中的私钥对信息签名",
	}
	signCmdUses = map[config.Language]string{
		config.English: "sign MESSAGE [-s SIGNER]",
		config.Chinese: "sign 信息 [-s 签署人]",
	}
	flagSignerUsages = map[config.Language]string{
		config.English: "choose a signing account",
		config.Chinese: "选择一个签名账户",
	}
)

// accountSignCmd represents the account sign command
var accountSignCmd = &cobra.Command{
	Use:   config.TranslateInLang(signCmdUses, config.UILanguage),
	Short: config.TranslateInLang(signCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := accountSign(args[0])
		return output.PrintError(err)
	},
}

func init() {
	accountSignCmd.Flags().StringVarP(&signer, "signer", "s", "", config.TranslateInLang(flagSignerUsages, config.UILanguage))
}

func accountSign(msg string) error {
	var (
		addr string
		err  error
	)
	if util.AliasIsHdwalletKey(signer) {
		addr = signer
	} else {
		addr, err = util.Address(signer)
		if err != nil {
			return output.NewError(output.AddressError, "failed to get address", err)
		}
	}
	signedMessage, err := Sign(addr, "", msg)
	if err != nil {
		return output.NewError(output.KeystoreError, "failed to sign message", err)
	}
	output.PrintResult(signedMessage)
	return nil
}
