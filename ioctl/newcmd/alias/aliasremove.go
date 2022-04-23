// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package alias

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

var (
	_removeShorts = map[config.Language]string{
		config.English: "Remove alias",
		config.Chinese: "移除别名",
	}
	_removeUses = map[config.Language]string{
		config.English: "remove",
		config.Chinese: "remove",
	}
	_removeInvalidAlias = map[config.Language]string{
		config.English: "invalid alias %s",
		config.Chinese: "不可用别名 %s",
	}
	_removeWriteError = map[config.Language]string{
		config.English: "failed to write to config file %s",
		config.Chinese: "无法写入配置文件 %s",
	}
	_removeResult = map[config.Language]string{
		config.English: "%s is removed",
		config.Chinese: "%s 已移除",
	}
)

// NewAliasRemove represents the removes alias command
func NewAliasRemove(c ioctl.Client) *cobra.Command {
	use, _ := c.SelectTranslation(_removeUses)
	short, _ := c.SelectTranslation(_removeShorts)
	invalidAlias, _ := c.SelectTranslation(_removeInvalidAlias)
	writeError, _ := c.SelectTranslation(_removeWriteError)
	result, _ := c.SelectTranslation(_removeResult)

	ec := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			alias := args[0]
			if err := validator.ValidateAlias(alias); err != nil {
				return errors.Errorf(invalidAlias, alias)
			}
			if err := c.DeleteAlias(alias); err != nil {
				return errors.Wrap(err, writeError)
			}
			cmd.Println(fmt.Sprintf(result, alias))
			return nil
		},
	}
	return ec
}
