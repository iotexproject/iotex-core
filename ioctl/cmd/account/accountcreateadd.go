// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/validator"
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
	// Validate alias
	if err := validator.ValidateAlias(args[0]); err != nil {
		return output.NewError(output.ValidationError, "invalid alias", err)
	}
	alias := args[0]

	// Check for existing alias and confirm overwrite if necessary
	if io1Addr, ok := config.ReadConfig.Aliases[alias]; ok {
		// Convert io1 to 0x format for display
		iotexAddr, err := address.FromString(io1Addr)
		if err != nil {
			return output.NewError(output.AddressError, "failed to parse existing alias address", err)
		}
		ethAddr := iotexAddr.Hex()

		// Show confirmation prompt in "0x (io1)" format
		proceed, err := confirmOverwrite(alias, fmt.Sprintf("%s (%s)", ethAddr, io1Addr))
		if err != nil {
			return err
		}
		if !proceed {
			output.PrintResult("quit")
			return nil
		}
	}

	// Create new account based on crypto type
	var io1Addr string
	var err error
	if CryptoSm2 {
		io1Addr, err = newAccountSm2(alias)
		if err != nil {
			return output.NewError(output.CryptoError, "failed to create SM2 account", err)
		}
	} else {
		io1Addr, err = newAccount(alias)
		if err != nil {
			return output.NewError(output.CryptoError, "failed to create account", err)
		}
	}

	// Convert io1 address to 0x format for display
	iotexAddr, err := address.FromString(io1Addr)
	if err != nil {
		return output.NewError(output.AddressError, "failed to convert to address struct", err)
	}
	ethAddr := iotexAddr.Hex()

	// Update alias configuration and write changes to the config file
	config.ReadConfig.Aliases[alias] = io1Addr
	out, err := yaml.Marshal(&config.ReadConfig)
	if err != nil {
		return output.NewError(output.SerializationError, "failed to marshal config", err)
	}
	if err := os.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return output.NewError(output.WriteFileError, fmt.Sprintf("failed to write to config file %s", config.DefaultConfigFile), err)
	}

	// Display both formats in "0x (io1)" format on a single line
	output.PrintResult(fmt.Sprintf("New account \"%s\" is created.\n"+
		"Please keep your password safe, or you will lose access to your private key.\n\n"+
		"Account address:\n%s (%s)", alias, ethAddr, io1Addr))

	return nil
}

func confirmOverwrite(alias, formattedAddress string) (bool, error) {
	info := fmt.Sprintf("** Alias \"%s\" is already used for:\n\n%s\n\n"+
		"Overwriting the account will keep the previous keystore file, "+
		"but bind the alias to the new one.\nWould you like to continue? [yes/no]", alias, formattedAddress)
	fmt.Println(info)

	var confirm string
	if _, err := fmt.Scanf("%s", &confirm); err != nil {
		return false, output.NewError(output.InputError, "failed to read confirmation input", err)
	}
	return strings.EqualFold(confirm, "yes"), nil
}
