// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	exportCmdShorts = map[config.Language]string{
		config.English: "Export IoTeX private key from wallet",
		config.Chinese: "从钱包导出IoTeX的私钥",
	}
	exportCmdUses = map[config.Language]string{
		config.English: "export (ALIAS|ADDRESS)",
		config.Chinese: "export (别名|地址)",
	}
)

// accountExportCmd represents the account export command
var accountExportCmd = &cobra.Command{
	Use:   config.TranslateInLang(exportCmdUses, config.UILanguage),
	Short: config.TranslateInLang(exportCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := accountExport(args[0])
		return output.PrintError(err)
	},
}

func accountExport(arg string) error {
	addr, err := util.Address(arg)
	if err != nil {
		return output.NewError(output.AddressError, "failed to get address", err)
	}
	output.PrintQuery(fmt.Sprintf("Enter password #%s:\n", addr))
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.NewError(output.InputError, "failed to get password", nil)
	}
	prvKey, err := LocalAccountToPrivateKey(addr, password)
	if err != nil {
		return output.NewError(output.KeystoreError, "failed to get private key from keystore", err)
	}
	defer prvKey.Zero()
	output.PrintResult(prvKey.HexString())
	return nil
}
