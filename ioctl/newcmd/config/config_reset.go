// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	_configResetCmdUses = map[config.Language]string{
		config.English: "",
		config.Chinese: "",
	}
	_configResetCmdShorts = map[config.Language]string{
		config.English: "",
		config.Chinese: "",
	}
)

// NewConfigReset represents the config reset command
func NewConfigReset(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_configResetCmdUses)
	short, _ := client.SelectTranslation(_configResetCmdShorts)

	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			return nil
		},
	}
}
