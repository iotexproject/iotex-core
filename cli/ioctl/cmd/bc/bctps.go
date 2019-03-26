// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import (
	"fmt"

	"github.com/spf13/cobra"
)

// bcTpsCmd represents the bc tps command
var bcTpsCmd = &cobra.Command{
	Use:   "tps",
	Short: "Get current tps of block chain",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(tps())
	},
}

// tps get current tps of block chain from server
func tps() string {
	chainMeta, err := GetChainMeta()
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("%d", chainMeta.Tps)
}
