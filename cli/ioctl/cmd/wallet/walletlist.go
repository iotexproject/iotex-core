// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package wallet

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
)

// walletListCmd represents the wallet list command
var walletListCmd = &cobra.Command{
	Use:   "list",
	Short: "List existing wallet for ioctl",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(walletList())
	},
}

func walletList() string {
	w, err := config.LoadConfig()
	if err != nil {
		return err.Error()
	}
	lines := make([]string, 0)
	for n, addr := range w.WalletList {
		lines = append(lines, fmt.Sprintf("name: %s, address:%s", n, addr))
	}
	return strings.Join(lines, "\n")
}
