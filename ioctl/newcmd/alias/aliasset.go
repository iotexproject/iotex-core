// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package alias

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

// Multi-language support
var (
	setCmdShorts = map[config.Language]string{
		config.English: "Set alias for address",
		config.Chinese: "设定地址的别名",
	}
	setCmdUses = map[config.Language]string{
		config.English: "set ALIAS ADDRESS",
		config.Chinese: "set 别名 地址",
	}
)

// NewAliasSetCmd represents the alias set command
func NewAliasSetCmd(c ioctl.Client) *cobra.Command {

	use, _ := c.SelectTranslation(setCmdUses)
	short, _ := c.SelectTranslation(setCmdShorts)

	ec := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if err := validator.ValidateAlias(args[0]); err != nil {
				return output.NewError(output.ValidationError, "invalid alias", err)
			}
			alias := args[0]
			if err := validator.ValidateAddress(args[1]); err != nil {
				return output.NewError(output.ValidationError, "invalid address", err)
			}
			addr := args[1]
			aliases := c.AliasMap()
			for aliases[addr] != "" {
				delete(c.Config().Aliases, aliases[addr])
				aliases = c.AliasMap()
			}
			c.Config().Aliases[alias] = addr
			if err := c.WriteAlias(c.Config().Aliases); err != nil {
				return output.NewError(output.WriteFileError,
					fmt.Sprintf("failed to write to config file %s", config.DefaultConfigFile), err)
			}
			output.PrintResult(args[0] + " has been set!")
			return nil
		},
	}
	return ec
}
