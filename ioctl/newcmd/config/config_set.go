// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

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
	_configSetUse = map[config.Language]string{
		config.English: "set VARIABLE VALUE",
		config.Chinese: "set 变量 值",
	}
	_configSetUseCmdShorts = map[config.Language]string{
		config.English: "Set config fields for ioctl",
		config.Chinese: "为 ioctl 设置配置字段",
	}
	_configSetUseCmdLong = map[config.Language]string{
		config.English: "Set config fields for ioctl\nValid Variables: [" + strings.Join(_validArgs, ", ") + "]",
		config.Chinese: "为 ioctl 设置配置字段\n有效变量: [" + strings.Join(_validArgs, ", ") + "]",
	}
)

// NewConfigSetCmd is a command to set config fields from iotcl.
func NewConfigSetCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_configSetUse)
	short, _ := client.SelectTranslation(_configSetUseCmdShorts)
	long, _ := client.SelectTranslation(_configSetUseCmdLong)

	cmd := &cobra.Command{
		Use:       use,
		Short:     short,
		Long:      long,
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
			result, err := newInfo(client.Config(), client.ConfigFilePath()).set(args)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("problem setting config fields %+v", args))
			}
			cmd.Println(result)
			return nil
		},
	}

	client.SetInsecureWithFlag(cmd.PersistentFlags().BoolVar)
	return cmd
}
