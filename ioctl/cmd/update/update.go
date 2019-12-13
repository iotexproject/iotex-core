// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package update

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	updateCmdUses = map[config.Language]string{
		config.English: "update [-t version-type]",
		config.Chinese: "update [-t 版本类型]",
	}
	updateCmdShorts = map[config.Language]string{
		config.English: "Update ioctl with latest version",
		config.Chinese: "使用最新版本更新ioctl",
	}
	flagVersionTypeUsages = map[config.Language]string{
		config.English: `set version type, "stable" or "unstable"`,
		config.Chinese: `设置版本类型, "稳定版" 或 "非稳定版"`,
	}
)

var (
	versionType string
)

// UpdateCmd represents the update command
var UpdateCmd = &cobra.Command{
	Use:   config.TranslateInLang(updateCmdUses, config.UILanguage),
	Short: config.TranslateInLang(updateCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := update()
		return err
	},
}

func init() {
	UpdateCmd.Flags().StringVarP(&versionType, "version-type", "t", "stable",
		config.TranslateInLang(flagVersionTypeUsages, config.UILanguage))
}

func update() error {
	var cmdString string
	switch versionType {
	default:
		return output.NewError(output.FlagError, "invalid version-type flag: "+versionType, nil)
	case "stable":
		cmdString = "curl --silent https://raw.githubusercontent.com/iotexproject/" +
			"iotex-core/master/install-cli.sh | sh"
	case "unstable":
		cmdString = "curl --silent https://raw.githubusercontent.com/iotexproject/" +
			"iotex-core/master/install-cli.sh | sh -s \"unstable\""

	}
	cmd := exec.Command("bash", "-c", cmdString)
	output.PrintResult(fmt.Sprintf("Downloading the latest %s version ...\n", versionType))
	err := cmd.Run()
	if err != nil {
		return output.NewError(output.UpdateError, "failed to update ioctl", nil)
	}
	output.PrintResult("ioctl is up-to-date now.")
	return nil
}
