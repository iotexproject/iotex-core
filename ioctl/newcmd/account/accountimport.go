// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"github.com/iotexproject/iotex-core/ioctl"
	"os"
	"sigs.k8s.io/yaml"

	"github.com/spf13/cobra"

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
	importPemCmdShorts = map[config.Language]string{
		config.English: "Import IoTeX key from pem file into wallet",
		config.Chinese: "将IoTeX私钥从pem文件导入钱包",
	}
	importPemCmdUses = map[config.Language]string{
		config.English: "pem ALIAS FILEPATH",
		config.Chinese: "pem 别名 文件路径",
	}
)

func NewAccountImportKeyCmd(client ioctl.Client) *cobra.Command {
	importKeyUses, _ := client.SelectTranslation(importKeyCmdUses)
	importKeyShorts, _ := client.SelectTranslation(importKeyCmdShorts)
	accountImportKeyCmd := &cobra.Command{
		Use:   importKeyUses,
		Short: importKeyShorts,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			err := accountImportKey(client, cmd, args)
			return output.PrintError(err)
		},
	}
	return accountImportKeyCmd
}

func NewAccountImportKeyStoreCmd(client ioctl.Client) *cobra.Command {
	// accountImportKeyStoreCmd represents the account import keystore command
	importKeyStoreUses, _ := client.SelectTranslation(importKeyStoreCmdUses)
	importKeyStoreShorts, _ := client.SelectTranslation(importKeyStoreCmdShorts)
	accountImportKeyStoreCmd := &cobra.Command{
		Use:   importKeyStoreUses,
		Short: importKeyStoreShorts,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			err := accountImportKeyStore(client, cmd, args)
			return output.PrintError(err)
		},
	}
	return accountImportKeyStoreCmd
}

func NewAccountImportPemCmd(client ioctl.Client) *cobra.Command {
	// accountImportPemCmd represents the account import pem command
	importPemUses, _ := client.SelectTranslation(importPemCmdUses)
	importPemShorts, _ := client.SelectTranslation(importPemCmdShorts)
	accountImportPemCmd := &cobra.Command{
		Use:   importPemUses,
		Short: importPemShorts,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			err := accountImportPem(client, cmd, args)
			return output.PrintError(err)
		},
	}
	return accountImportPemCmd
}

func NewAccountImportCmd(client ioctl.Client) *cobra.Command {
	importUses, _ := client.SelectTranslation(importCmdUses)
	importShorts, _ := client.SelectTranslation(importCmdShorts)
	accountImportCmd := &cobra.Command{
		Use:   importUses,
		Short: importShorts,
	}

	accountImportCmd.AddCommand(NewAccountImportKeyCmd(client))
	accountImportCmd.AddCommand(NewAccountImportKeyStoreCmd(client))
	accountImportCmd.AddCommand(NewAccountImportPemCmd(client))

	return accountImportCmd
}

func validateAlias(alias string) error {
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
	if err := os.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return output.NewError(output.WriteFileError,
			fmt.Sprintf("failed to write to config file %s", config.DefaultConfigFile), err)
	}
	output.PrintResult(fmt.Sprintf("New account #%s is created. Keep your password, "+
		"or you will lose your private key.", alias))
	return nil
}

func readPasswordFromStdin() (string, error) {
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return "", fmt.Errorf("failed to get password")
	}
	return password, nil
}
func accountImportKey(client ioctl.Client, cmd *cobra.Command, args []string) error {
	// Validate inputs
	alias := args[0]
	err := validateAlias(alias)
	if err != nil {
		return output.NewError(output.ValidationError, "invalid alias", err)
	}
	output.PrintQuery(fmt.Sprintf("#%s: Enter your private key, "+
		"which will not be exposed on the screen.", alias))
	privateKey, err := readPasswordFromStdin()
	privateKey = util.TrimHexPrefix(privateKey)
	if err != nil {
		return output.NewError(output.InputError, "failed to get password", err)
	}
	addr, err := newAccountByKey(client, cmd, alias, privateKey)
	if err != nil {
		return output.NewError(0, "", err)
	}
	return writeToFile(alias, addr)
}

func accountImportKeyStore(client ioctl.Client, cmd *cobra.Command, args []string) error {
	// Validate inputs
	alias := args[0]
	err := validateAlias(alias)
	if err != nil {
		return output.NewError(output.ValidationError, "invalid alias", err)
	}
	_, err = os.Stat(args[1])
	if err != nil {
		return output.NewError(output.ReadFileError, "", err)
	}

	output.PrintQuery(fmt.Sprintf("#%s: Enter your password of keystore, "+
		"which will not be exposed on the screen.", alias))
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.NewError(output.InputError, "failed to get password", err)
	}
	addr, err := newAccountByKeyStore(client, cmd, alias, password, args[1])
	if err != nil {
		return output.NewError(0, "", err)
	}
	return writeToFile(alias, addr)
}

func accountImportPem(client ioctl.Client, cmd *cobra.Command, args []string) error {
	// Validate inputs
	alias := args[0]
	err := validateAlias(alias)
	if err != nil {
		return output.NewError(output.ValidationError, "invalid alias", err)
	}
	_, err = os.Stat(args[1])
	if err != nil {
		return output.NewError(output.ReadFileError, "", err)
	}

	output.PrintQuery(fmt.Sprintf("#%s: Enter your password of pem file, "+
		"which will not be exposed on the screen.", alias))
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.NewError(output.InputError, "failed to get password", err)
	}
	addr, err := newAccountByPem(client, cmd, alias, password, args[1])
	if err != nil {
		return output.NewError(0, "", err)
	}
	return writeToFile(alias, addr)
}
