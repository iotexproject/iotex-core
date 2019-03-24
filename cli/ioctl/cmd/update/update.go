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
)

var (
	versionType string
)

// UpdateCmd represents the update command
var UpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update ioctl with latest version",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(update())
	},
}

func init() {
	UpdateCmd.Flags().StringVarP(&versionType, "versionType", "t", "stable",
		"set version type, \"stable\" or \"unstable\"")
}

func update() string {
	var cmdString string
	switch versionType {
	case "stable":
		cmdString = "curl --silent https://raw.githubusercontent.com/iotexproject/" +
			"iotex-core/master/install-cli.sh | sh"
	case "unstable":
		cmdString = "curl --silent https://raw.githubusercontent.com/iotexproject/" +
			"iotex-core/master/install-cli.sh | sh -s \"unstable\""
	default:
		return fmt.Sprintf("invalid flag %s", versionType)
	}
	cmd := exec.Command("bash", "-c", cmdString)
	fmt.Printf("Downloading the latest %s version ...\n", versionType)
	err := cmd.Run()
	if err != nil {
		return fmt.Sprintf("Failed to update ioctl.")
	}
	return "ioctl is up-to-date now."
}
