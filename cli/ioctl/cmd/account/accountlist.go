// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
)

// accountListCmd represents the account list command
var accountListCmd = &cobra.Command{
	Use:   "list",
	Short: "List existing account for ioctl",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(accountList())
	},
}

func accountList() string {
	w, err := config.LoadConfig()
	if err != nil {
		return err.Error()
	}
	lines := make([]string, 0)
	if len(w.AccountList) != 0 {
		lines = append(lines, "Account:")
		for name, addr := range w.AccountList {
			lines = append(lines, fmt.Sprintf("  %s - %s", addr, name))
		}
	}
	if len(w.NameList) != 0 {
		lines = append(lines, "Name:")
		for name, addr := range w.NameList {
			lines = append(lines, fmt.Sprintf("  %s - %s", addr, name))
		}
	}
	return strings.Join(lines, "\n")
}
