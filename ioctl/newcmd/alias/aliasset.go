// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package alias

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
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

// NewAliasSetCmd represents the alias set command
func NewAliasSetCmd(c ioctl.Client) *cobra.Command {
	use, _ := c.SelectTranslation(_setCmdUses)
	short, _ := c.SelectTranslation(_setCmdShorts)

	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if err := validator.ValidateAlias(args[0]); err != nil {
				return errors.Wrap(err, "invalid alias")
			}
			if err := validator.ValidateAddress(args[1]); err != nil {
				return errors.Wrap(err, "invalid address")
			}
			if err := c.SetAliasAndSave(args[0], args[1]); err != nil {
				return errors.Wrap(err, "failed to write to config file ")
			}
			cmd.Println(args[0] + " has been set!")
			return nil
		},
	}
}
