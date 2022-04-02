// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
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

// NewAccountImportCmd combines three account import command
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

// NewAccountImportKeyCmd represents the account import key command
func NewAccountImportKeyCmd(client ioctl.Client) *cobra.Command {
	importKeyUses, _ := client.SelectTranslation(importKeyCmdUses)
	importKeyShorts, _ := client.SelectTranslation(importKeyCmdShorts)
	return &cobra.Command{
		Use:   importKeyUses,
		Short: importKeyShorts,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			alias := args[0]

			if err := validateAlias(client, alias); err != nil {
				return errors.Wrap(err, "invalid alias")
			}
			cmd.Println(fmt.Sprintf("#%s: Enter your private key, "+
				"which will not be exposed on the screen.", alias))
			privateKey, err := client.ReadSecret()
			privateKey = util.TrimHexPrefix(privateKey)
			if err != nil {
				return errors.Wrap(err, "failed to get password")
			}
			addr, err := newAccountByKey(client, cmd, alias, privateKey)
			if err != nil {
				return err
			}
			return client.SetAlias(alias, addr)
		},
	}
}

// NewAccountImportKeyStoreCmd represents the account import keystore command
func NewAccountImportKeyStoreCmd(client ioctl.Client) *cobra.Command {
	importKeyStoreUses, _ := client.SelectTranslation(importKeyStoreCmdUses)
	importKeyStoreShorts, _ := client.SelectTranslation(importKeyStoreCmdShorts)
	return &cobra.Command{
		Use:   importKeyStoreUses,
		Short: importKeyStoreShorts,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			alias := args[0]

			if err := validateAlias(client, alias); err != nil {
				return errors.Wrap(err, "invalid alias")
			}
			if _, err := os.Stat(args[1]); err != nil {
				return err
			}

			cmd.Println(fmt.Sprintf("#%s: Enter your password of keystore, "+
				"which will not be exposed on the screen.", alias))
			password, err := client.ReadSecret()
			if err != nil {
				return errors.Wrap(err, "failed to get password")
			}
			addr, err := newAccountByKeyStore(client, cmd, alias, password, args[1])
			if err != nil {
				return err
			}
			return client.SetAlias(alias, addr)
		},
	}
}

// NewAccountImportPemCmd represents the account import pem command
func NewAccountImportPemCmd(client ioctl.Client) *cobra.Command {
	importPemUses, _ := client.SelectTranslation(importPemCmdUses)
	importPemShorts, _ := client.SelectTranslation(importPemCmdShorts)
	return &cobra.Command{
		Use:   importPemUses,
		Short: importPemShorts,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			alias := args[0]

			if err := validateAlias(client, alias); err != nil {
				return errors.Wrap(err, "invalid alias")
			}
			if _, err := os.Stat(args[1]); err != nil {
				return err
			}

			cmd.Println(fmt.Sprintf("#%s: Enter your password of pem file, "+
				"which will not be exposed on the screen.", alias))
			password, err := client.ReadSecret()
			if err != nil {
				return errors.Wrap(err, "failed to get password")
			}
			addr, err := newAccountByPem(client, cmd, alias, password, args[1])
			if err != nil {
				return err
			}
			return client.SetAlias(alias, addr)
		},
	}
}

func validateAlias(client ioctl.Client, alias string) error {
	if err := validator.ValidateAlias(alias); err != nil {
		return err
	}
	if addr, ok := client.Config().Aliases[alias]; ok {
		return fmt.Errorf("alias \"%s\" has already used for %s", alias, addr)
	}
	return nil
}
