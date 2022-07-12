// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"github.com/pkg/errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

var (
	_configUseCmdShorts = map[config.Language]string{
		config.English: "Get config fields from ioctl",
		config.Chinese: "从 ioctl 获取配置字段",
	}
	_configUseCmdLong = map[config.Language]string{
		config.English: "Get config fields from ioctl\nValid Variables: [" + strings.Join(_validGetArgs, ", ") + "]",
		config.Chinese: "从 ioctl 获取配置字段\n有效变量: [" + strings.Join(_validGetArgs, ", ") + "]",
	}
)

// NewConfigGetCmd is a command to get config fields from iotcl.
func NewConfigGetCmd(client ioctl.Client) *cobra.Command {
	short, _ := client.SelectTranslation(_configUseCmdShorts)
	long, _ := client.SelectTranslation(_configUseCmdLong)

	return &cobra.Command{
		Use:       "get VARIABLE",
		Short:     short,
		Long:      long,
		ValidArgs: _validGetArgs,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("accepts 1 arg(s), received %d\n"+
					"Valid arg(s): %s", len(args), _validGetArgs)
			}
			return cobra.OnlyValidArgs(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			result, err := newInfo(client.Config(), "").get(args[0])
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("issue fetching config value %s", args[0]))
			}
			cmd.Println(result)
			return nil
		},
	}
}
