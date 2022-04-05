// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

// Multi-language support
var (
	_createAddCmdShorts = map[config.Language]string{
		config.English: "Create new account for ioctl",
		config.Chinese: "为ioctl创建新账户",
	}
	_createAddCmdUses = map[config.Language]string{
		config.English: "createadd ALIAS",
		config.Chinese: "createadd 别名",
	}
)

// _accountCreateAddCmd represents the account createadd command
var _accountCreateAddCmd = &cobra.Command{
	Use:   config.TranslateInLang(_createAddCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_createAddCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := accountCreateAdd(args)
		return output.PrintError(err)
	},
}

func accountCreateAdd(args []string) error {
	// Validate inputs
	if err := validator.ValidateAlias(args[0]); err != nil {
		return output.NewError(output.ValidationError, "invalid alias", err)
	}
	alias := args[0]
	if addr, ok := config.ReadConfig.Aliases[alias]; ok {
		var confirm string
		info := fmt.Sprintf("** Alias \"%s\" has already used for %s\n"+
			"Overwriting the account will keep the previous keystore file stay, "+
			"but bind the alias to the new one.\nWould you like to continue?\n", alias, addr)
		message := output.ConfirmationMessage{Info: info, Options: []string{"yes"}}
		fmt.Println(message.String())
		fmt.Scanf("%s", &confirm)
		if !strings.EqualFold(confirm, "yes") {
			output.PrintResult("quit")
			return nil
		}
	}

	var addr string
	var err error
	if CryptoSm2 {
		addr, err = newAccountSm2(alias)
		if err != nil {
			return output.NewError(0, "", err)
		}
	} else {
		addr, err = newAccount(alias)
		if err != nil {
			return output.NewError(0, "", err)
		}
	}
	config.ReadConfig.Aliases[alias] = addr
	out, err := yaml.Marshal(&config.ReadConfig)
	if err != nil {
		return output.NewError(output.SerializationError, "failed to marshal config", err)
	}
	if err := os.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return output.NewError(output.WriteFileError,
			fmt.Sprintf("failed to write to config file %s", config.DefaultConfigFile), err)
	}
	output.PrintResult(fmt.Sprintf("New account \"%s\" is created.\n"+
		"Please Keep your password, or you will lose your private key.", alias))
	return nil
}
