// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
)

var _configResetCmdShorts = map[config.Language]string{
	config.English: "Reset config to default",
	config.Chinese: "将配置重置为默认值",
}

// NewConfigReset resets the config to the default values
func NewConfigReset(client ioctl.Client) *cobra.Command {
	short, _ := client.SelectTranslation(_configResetCmdShorts)

	return &cobra.Command{
		Use:   "reset",
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			err := newInfo(client.Config(), client.ConfigFilePath()).reset()
			if err != nil {
				return errors.Wrap(err, "failed to reset config")
			}
			cmd.Println("successfully reset config")
			return nil
		},
	}
}
