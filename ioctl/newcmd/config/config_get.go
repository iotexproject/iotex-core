// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

var (
	_configGetUse = map[config.Language]string{
		config.English: "get VARIABLE",
		config.Chinese: "get 变量",
	}
	_configGetUseCmdShorts = map[config.Language]string{
		config.English: "Get config fields from ioctl",
		config.Chinese: "从 ioctl 获取配置字段",
	}
	_configGetUseCmdLong = map[config.Language]string{
		config.English: "Get config fields from ioctl\nValid Variables: [" + strings.Join(_validGetArgs, ", ") + "]",
		config.Chinese: "从 ioctl 获取配置字段\n有效变量: [" + strings.Join(_validGetArgs, ", ") + "]",
	}
)

// NewConfigGetCmd is a command to get config fields from iotcl.
func NewConfigGetCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_configGetUse)
	short, _ := client.SelectTranslation(_configGetUseCmdShorts)
	long, _ := client.SelectTranslation(_configGetUseCmdLong)

	return &cobra.Command{
		Use:       use,
		Short:     short,
		Long:      long,
		ValidArgs: _validGetArgs,
		Args:      cobra.ExactValidArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			result, err := newInfo(client.Config(), client.ConfigFilePath()).get(args[0])
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("issue fetching config value %s", args[0]))
			}
			cmd.Println(result)
			return nil
		},
	}
}
