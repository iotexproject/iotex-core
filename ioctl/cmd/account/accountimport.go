// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

var (
	// accountImportCmd represents the account import command
	accountImportCmd = &cobra.Command{
		Use:   "import",
		Short: "Import IoTeX private key or keystore into wallet",
	}
	// accountImportKeyCmd represents the account import key command
	accountImportKeyCmd = &cobra.Command{
		Use:   "key ALIAS",
		Short: "Import IoTeX private key into wallet",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			err := accountImportKey(args)
			return err
		},
	}
	// accountImportKeyCmd represents the account import keystore command
	accountImportKeyStoreCmd = &cobra.Command{
		Use:   "keystore ALIAS FILEPATH",
		Short: "Import IoTeX keystore into wallet",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			err := accountImportKeyStore(args)
			return err
		},
	}
)

func init() {
	accountImportCmd.AddCommand(accountImportKeyCmd)
	accountImportCmd.AddCommand(accountImportKeyStoreCmd)
}
func validataAlias(alias string) error {
	if err := validator.ValidateAlias(alias); err != nil {
		return err
	}
	if addr, ok := config.ReadConfig.Aliases[alias]; ok {
		return fmt.Errorf("alias \"%s\" has already used for %s", alias, addr)
	}
	return nil
}
func writeToFile(alias, addr string) error {
	config.ReadConfig.Aliases[alias] = addr
	out, err := yaml.Marshal(&config.ReadConfig)
	if err != nil {
		return output.PrintError(output.SerializationError, err.Error())
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return output.PrintError(output.WriteFileError,
			fmt.Sprintf("failed to write to config file %s", config.DefaultConfigFile))
	}
	output.PrintResult(fmt.Sprintf("New account #%s is created. Keep your password, "+
		"or your will lose your private key.", alias))
	return nil
}
func readPasswordFromStdin() (string, error) {
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return "", fmt.Errorf("failed to get password")
	}
	return password, nil
}
func accountImportKey(args []string) error {
	// Validate inputs
	alias := args[0]
	err := validataAlias(alias)
	if err != nil {
		return output.PrintError(output.ValidationError, err.Error())
	}
	output.PrintQuery(fmt.Sprintf("#%s: Enter your private key, "+
		"which will not be exposed on the screen.", alias))
	privateKey, err := readPasswordFromStdin()
	if err != nil {
		return output.PrintError(output.InputError, err.Error())
	}
	addr, err := newAccountByKey(alias, privateKey, config.ReadConfig.Wallet)
	if err != nil {
		return output.PrintError(0, err.Error()) // TODO: undefined error
	}
	return writeToFile(alias, addr)
}
func accountImportKeyStore(args []string) error {
	// Validate inputs
	alias := args[0]
	err := validataAlias(alias)
	if err != nil {
		return output.PrintError(output.ValidationError, err.Error())
	}
	output.PrintQuery(fmt.Sprintf("#%s: Enter your password of keystore, "+
		"which will not be exposed on the screen.", alias))
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.PrintError(output.InputError, err.Error())
	}
	addr, err := newAccountByKeyStore(alias, password, args[1], config.ReadConfig.Wallet)
	if err != nil {
		return output.PrintError(0, err.Error()) // TODO: undefined error
	}
	return writeToFile(alias, addr)
}
