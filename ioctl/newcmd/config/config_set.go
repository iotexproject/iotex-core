// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	_configSetCmdUses = map[config.Language]string{
		config.English: "",
		config.Chinese: "",
	}
	_configSetCmdShorts = map[config.Language]string{
		config.English: "",
		config.Chinese: "",
	}
)

// NewConfigSet represents the config set command
func NewConfigSet(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_configSetCmdUses)
	short, _ := client.SelectTranslation(_configSetCmdShorts)

	return &cobra.Command{
		Use:       use,
		Short:     short,
		ValidArgs: _validArgs,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return fmt.Errorf("accepts 2 arg(s), received %d\n"+
					"Valid arg(s): %s", len(args), _validArgs)
			}
			return cobra.OnlyValidArgs(cmd, args[:1])
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			return nil
		},
	}
}
