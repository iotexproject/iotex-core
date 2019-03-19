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

// UpdateCmd represents the update command
var UpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update ioctl with latest release",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(update())
	},
}

func update() string {
	cmdString := "curl --silent " +
		"https://raw.githubusercontent.com/iotexproject/iotex-core/master/install-cli.sh | sh"
	cmd := exec.Command("bash", "-c", cmdString)
	fmt.Println("Downloading the latest release ...")
	err := cmd.Run()
	if err != nil {
		return fmt.Sprintf("Failed to update ioctl.")
	}
	return "Ioctl is up-to-date now."
}
