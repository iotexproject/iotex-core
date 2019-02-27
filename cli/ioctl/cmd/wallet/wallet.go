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
)

const (
	walletPrefix = "wallet:"
	walletEnd    = "endWallet"
)

// WalletCmd represents the wallet command
var WalletCmd = &cobra.Command{
	Use:   "wallet",
	Short: "Manage accounts",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Print: " + strings.Join(args, " "))
	},
}

func init() {
	WalletCmd.AddCommand(walletCreateCmd)
	WalletCmd.AddCommand(walletListCmd)
}

func parseConfig(file []byte, start, end, name string) (int, int, bool, bool) {
	var startLine, endLine int
	find := false
	exist := false
	lines := strings.Split(string(file), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, end) {
			endLine = i
			break
		}
		if !find && strings.HasPrefix(line, start) {
			find = true
			startLine = i
			continue
		}
		// detect name collision
		if find && name != "" && strings.HasPrefix(line, name) {
			exist = true
		}
	}
	return startLine, endLine, find, exist
}
