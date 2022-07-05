// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
)

// NewConfigReset resets the config to the default values
func NewConfigReset(client ioctl.Client, defaultConfigFileName string) *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Reset config to default",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			info := newInfo(client.Config(), defaultConfigFileName)
			err := info.reset()
			if err != nil {
				return errors.Wrap(err, "failed to reset config")
			}
			cmd.Print("successfully reset config")
			return nil
		},
	}
}
