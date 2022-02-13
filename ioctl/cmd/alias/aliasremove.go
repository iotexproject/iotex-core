// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package alias

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

// Multi-language support
var (
	removeCmdShorts = map[config.Language]string{
		config.English: "Remove alias",
		config.Chinese: "移除别名",
	}
	removeCmdUses = map[config.Language]string{
		config.English: "remove ALIAS",
		config.Chinese: "remove 别名",
	}
)

// aliasRemoveCmd represents the alias remove command
var aliasRemoveCmd = &cobra.Command{
	Use:   config.TranslateInLang(removeCmdUses, config.UILanguage),
	Short: config.TranslateInLang(removeCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := remove(args[0])
		return output.PrintError(err)
	},
}

// remove removes alias
func remove(arg string) error {
	if err := validator.ValidateAlias(arg); err != nil {
		return output.NewError(output.ValidationError, "invalid alias", err)
	}
	alias := arg
	delete(config.ReadConfig.Aliases, alias)
	out, err := yaml.Marshal(&config.ReadConfig)
	if err != nil {
		return output.NewError(output.SerializationError, "failed to marshal config", err)
	}
	if err := os.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return output.NewError(output.WriteFileError,
			fmt.Sprintf("failed to write to config file %s", config.DefaultConfigFile), err)
	}
	output.PrintResult(alias + " is removed")
	return nil
}
