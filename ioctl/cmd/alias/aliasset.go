// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package alias

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

// Multi-language support
var (
	_setCmdShorts = map[config.Language]string{
		config.English: "Set alias for address",
		config.Chinese: "设定地址的别名",
	}
	_setCmdUses = map[config.Language]string{
		config.English: "set ALIAS ADDRESS",
		config.Chinese: "set 别名 地址",
	}
)

// _aliasSetCmd represents the alias set command
var _aliasSetCmd = &cobra.Command{
	Use:   config.TranslateInLang(_setCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_setCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := set(args)
		return output.PrintError(err)
	},
}

// set sets alias
func set(args []string) error {
	if err := validator.ValidateAlias(args[0]); err != nil {
		return output.NewError(output.ValidationError, "invalid alias", err)
	}
	alias := args[0]
	if err := validator.ValidateAddress(args[1]); err != nil {
		return output.NewError(output.ValidationError, "invalid address", err)
	}
	addr := args[1]
	aliases := GetAliasMap()
	for aliases[addr] != "" {
		delete(config.ReadConfig.Aliases, aliases[addr])
		aliases = GetAliasMap()
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
	output.PrintResult(args[0] + " has been set!")
	return nil
}
