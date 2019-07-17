// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

// accountCreateAddCmd represents the account createadd command
var accountCreateAddCmd = &cobra.Command{
	Use:   "createadd ALIAS",
	Short: "Create new account for ioctl",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := accountCreateAdd(args)
		return err
	},
}

func accountCreateAdd(args []string) error {
	// Validate inputs
	if err := validator.ValidateAlias(args[0]); err != nil {
		return output.PrintError(output.ValidationError, err.Error())
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
	addr, err := newAccount(alias, config.ReadConfig.Wallet)
	if err != nil {
		return output.PrintError(0, err.Error()) // TODO: undefined error
	}
	config.ReadConfig.Aliases[alias] = addr
	out, err := yaml.Marshal(&config.ReadConfig)
	if err != nil {
		return output.PrintError(output.SerializationError, err.Error())
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return output.PrintError(output.WriteFileError,
			fmt.Sprintf("failed to write to config file %s", config.DefaultConfigFile))
	}
	output.PrintResult(fmt.Sprintf("New account \"%s\" is created.\n"+
		"Please Keep your password, or your will lose your private key.", alias))
	return nil
}
