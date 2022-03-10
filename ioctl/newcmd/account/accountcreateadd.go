// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

// Multi-language support
var (
	createAddCmdShorts = map[config.Language]string{
		config.English: "Create new account for ioctl",
		config.Chinese: "为ioctl创建新账户",
	}
	createAddCmdUses = map[config.Language]string{
		config.English: "createadd ALIAS",
		config.Chinese: "createadd 别名",
	}
	invalidAlias = map[config.Language]string{
		config.English: "invalid alias",
		config.Chinese: "无效别名",
	}
	aliasHasAlreadyUsed = map[config.Language]string{
		config.English: "** Alias \"%s\" has already used for %s\n" +
			"Overwriting the account will keep the previous keystore file stay, " +
			"but bind the alias to the new one.\nWould you like to continue?\n",
		config.Chinese: "** 这个别名 \"%s\" 已被 %s 使用!\n" +
			"复写帐户后先前的 keystore 文件将会留存!\n" +
			"但底下的别名将绑定为新的。您是否要继续？",
	}
	outputMessage = map[config.Language]string{
		config.English: "New account \"%s\" is created.\n" +
			"Please Keep your password, or you will lose your private key.",
		config.Chinese: "新帐户 \"%s\" 已建立。\n" +
			"请保护好您的密码，否则您会失去您的私钥。",
	}
)

// NewAccountCreateAdd represents the account createadd command
func NewAccountCreateAdd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(createAddCmdUses)
	short, _ := client.SelectTranslation(createAddCmdShorts)
	invalidAlias, _ := client.SelectTranslation(invalidAlias)
	aliasHasAlreadyUsed, _ := client.SelectTranslation(aliasHasAlreadyUsed)
	infoQuit, _ := client.SelectTranslation(infoQuit)
	failToWriteToConfigFile, _ := client.SelectTranslation(failToWriteToConfigFile)
	failToGenerateNewPrivateKey, _ := client.SelectTranslation(failToGenerateNewPrivateKey)
	failToGenerateNewPrivateKeySm2, _ := client.SelectTranslation(failToGenerateNewPrivateKeySm2)
	outputMessage, _ := client.SelectTranslation(outputMessage)

	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			if err := validator.ValidateAlias(args[0]); err != nil {
				return errors.Wrap(err, invalidAlias)
			}

			if addr, ok := client.Config().Aliases[args[0]]; ok {
				if !client.AskToConfirm(fmt.Sprintf(aliasHasAlreadyUsed, args[0], addr)) {
					client.PrintInfo(infoQuit)
					return nil
				}
			}

			var addr string
			var err error
			if client.IsCryptoSm2() {
				addr, err = newAccountSm2(client, args[0])
				if err != nil {
					return errors.Wrap(err, failToGenerateNewPrivateKey)
				}
			} else {
				addr, err = newAccount(client, args[0])
				if err != nil {
					return errors.Wrap(err, failToGenerateNewPrivateKeySm2)
				}
			}
			if err := client.SetAlias(args[0], addr); err != nil {
				return errors.Wrapf(err, failToWriteToConfigFile)
			}
			client.PrintInfo(fmt.Sprintf(outputMessage, args[0]))
			return nil
		},
	}
}
