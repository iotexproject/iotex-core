// Copyright (c) 2022 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package update

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	_shorts = map[config.Language]string{
		config.English: "Update password for IoTeX account",
		config.Chinese: "为IoTeX账户更新密码",
	}
	_uses = map[config.Language]string{
		config.English: "update [ALIAS|ADDRESS]",
		config.Chinese: "update [别名|地址]",
	}
	_flagUsages = map[config.Language]string{
		config.English: `set version type, "stable" or "unstable"`,
		config.Chinese: `设置版本类型, "稳定版" 或 "非稳定版"`,
	}
	_invalidVersionType = map[config.Language]string{
		config.English: "invalid version-type flag:%s",
		config.Chinese: "无效版本状态:%s",
	}
	_resultSuccess = map[config.Language]string{
		config.English: "ioctl is up-to-date now.",
		config.Chinese: "ioctl 现已更新完毕。",
	}
	_resultFail = map[config.Language]string{
		config.English: "failed to update ioctl",
		config.Chinese: "ioctl 更新失败",
	}
	_resultInfo = map[config.Language]string{
		config.English: "Downloading the latest %s version ...\n",
		config.Chinese: "正在下载最新的 %s 版本 ...\n",
	}
	_infoWarn = map[config.Language]string{
		config.English: "Type 'YES' to continue, quit for anything else.",
		config.Chinese: "输入 'YES' 以继续, 否则退出",
	}
	_infoQuit = map[config.Language]string{
		config.English: "quit",
		config.Chinese: "退出",
	}
)

// NewUpdateCmd represents the update command
func NewUpdateCmd(c ioctl.Client) *cobra.Command {
	var versionType string

	use, _ := c.SelectTranslation(_uses)
	short, _ := c.SelectTranslation(_shorts)
	flagUsage, _ := c.SelectTranslation(_flagUsages)
	success, _ := c.SelectTranslation(_resultSuccess)
	fail, _ := c.SelectTranslation(_resultFail)
	info, _ := c.SelectTranslation(_resultInfo)
	invalidVersionType, _ := c.SelectTranslation(_invalidVersionType)
	_infoWarn, _ := c.SelectTranslation(_infoWarn)
	_infoQuit, _ := c.SelectTranslation(_infoQuit)

	uc := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			var cmdString string
			switch versionType {
			case "stable":
				cmdString = "curl --silent https://raw.githubusercontent.com/iotexproject/" + "iotex-core/master/install-cli.sh | sh"
			case "unstable":
				cmdString = "curl --silent https://raw.githubusercontent.com/iotexproject/" + "iotex-core/master/install-cli.sh | sh -s \"unstable\""
			default:
				return errors.New(fmt.Sprintf(invalidVersionType, versionType))
			}

			confirmed, err := c.AskToConfirm(_infoWarn)
			if err != nil {
				return errors.Wrap(err, "failed to ask confirm")
			}
			if !confirmed {
				cmd.Println(_infoQuit)
				return nil
			}
			cmd.Printf(info, versionType)

			if err = c.Execute(cmdString); err != nil {
				return errors.Wrap(err, fail)
			}
			cmd.Println(success)
			return nil
		},
	}

	uc.Flags().StringVarP(&versionType, "version-type", "t", "stable", flagUsage)
	return uc
}
