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

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

// Multi-language support
var (
	importCmdShorts = map[config.Language]string{
		config.English: "Import IoTeX private key or keystore into wallet",
		config.Chinese: "将IoTeX的私钥或私钥库导入钱包",
	}
	importCmdUses = map[config.Language]string{
		config.English: "import",
		config.Chinese: "导入",
	}
	importKeyCmdShorts = map[config.Language]string{
		config.English: "Import IoTeX private key into wallet",
		config.Chinese: "将IoTeX的私钥导入钱包",
	}
	importKeyCmdUses = map[config.Language]string{
		config.English: "key ALIAS",
		config.Chinese: "key 别名",
	}
	importKeyStoreCmdShorts = map[config.Language]string{
		config.English: "Import IoTeX keystore into wallet",
		config.Chinese: "将IoTeX的私钥库导入钱包",
	}
	importKeyStoreCmdUses = map[config.Language]string{
		config.English: "keystore ALIAS FILEPATH",
		config.Chinese: "keystore 别名 文件路径",
	}
)
var (
	// accountImportCmd represents the account import command
	accountImportCmd = &cobra.Command{
		Use:   config.TranslateInLang(importCmdUses, config.UILanguage),
		Short: config.TranslateInLang(importCmdShorts, config.UILanguage),
	}
	// accountImportKeyCmd represents the account import key command
	accountImportKeyCmd = &cobra.Command{
		Use:   config.TranslateInLang(importKeyCmdUses, config.UILanguage),
		Short: config.TranslateInLang(importKeyCmdShorts, config.UILanguage),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			err := accountImportKey(args)
			return output.PrintError(err)
		},
	}
	// accountImportKeyCmd represents the account import keystore command
	accountImportKeyStoreCmd = &cobra.Command{
		Use:   config.TranslateInLang(importKeyStoreCmdUses, config.UILanguage),
		Short: config.TranslateInLang(importKeyStoreCmdShorts, config.UILanguage),
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			err := accountImportKeyStore(args)
			return output.PrintError(err)
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
		return output.NewError(output.SerializationError, "failed to marshal config", err)
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return output.NewError(output.WriteFileError,
			fmt.Sprintf("failed to write to config file %s", config.DefaultConfigFile), err)
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
		return output.NewError(output.ValidationError, "invalid alias", err)
	}
	output.PrintQuery(fmt.Sprintf("#%s: Enter your private key, "+
		"which will not be exposed on the screen.", alias))
	privateKey, err := readPasswordFromStdin()
	if err != nil {
		return output.NewError(output.InputError, "failed to get password", err)
	}
	// TODO: final arg depends on private key type
	// use config.ReadConfig.Wallet for p256k1 key (the code below)
	// use PEM file location for p256sm2 key
	addr, err := newAccountByKey(alias, privateKey, config.ReadConfig.Wallet)
	if err != nil {
		return output.NewError(0, "", err)
	}
	return writeToFile(alias, addr)
}
func accountImportKeyStore(args []string) error {
	// Validate inputs
	alias := args[0]
	err := validataAlias(alias)
	if err != nil {
		return output.NewError(output.ValidationError, "invalid alias", err)
	}
	output.PrintQuery(fmt.Sprintf("#%s: Enter your password of keystore, "+
		"which will not be exposed on the screen.", alias))
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.NewError(output.InputError, "failed to get password", err)
	}
	// TODO: final arg depends on key type
	// use config.ReadConfig.Wallet for p256k1 key (the code below)
	// use PEM file location for p256sm2 key
	addr, err := newAccountByKeyStore(alias, password, args[1], config.ReadConfig.Wallet)
	if err != nil {
		return output.NewError(0, "", err) // TODO: undefined error
	}
	return writeToFile(alias, addr)
}
