// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package update

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	shorts = map[config.Language]string{
		config.English: "Update password for IoTeX account",
		config.Chinese: "为IoTeX账户更新密码",
	}
	uses = map[config.Language]string{
		config.English: "update [ALIAS|ADDRESS]",
		config.Chinese: "update [别名|地址]",
	}
)

func NewUpdateCmd(c ioctl.Client) *cobra.Command {
	var versionType string
	use, _ := c.SelectTranslation(uses)
	short, _ := c.SelectTranslation(shorts)
	uc := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			var cmdString string
			switch versionType {
			default:
				output.NewError(output.FlagError, "invalid version-type flag:"+versionType, nil)
			case "stable":
				cmdString = "curl --silent https://raw.githubusercontent.com/iotexproject/" + "iotex-core/master/install-cli.sh | sh"
			case "unstable":
				cmdString = "curl --silent https://raw.githubusercontent.com/iotexproject/" + "iotex-core/master/install-cli.sh | sh -s \"unstable\""

			}

			output.PrintResult(fmt.Sprintf("Downloading the latest %s version ...\n", versionType))
			err := exec.Command("bash", "-c", cmdString).Run()
			if err != nil {
				return output.NewError(output.UpdateError, "failed to update ioctl", nil)
			}
			output.PrintResult("ioctl is up-to-date now.")
			return nil

		},
	}

	uc.Flags().StringVarP(&versionType, "version-type", "t", "stable",
		`set version type, "stable" or "unstable"`)

	return uc

}
