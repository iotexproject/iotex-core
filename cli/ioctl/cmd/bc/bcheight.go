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

// bcHeightCmd represents the bc height command
var bcHeightCmd = &cobra.Command{
	Use:   "height",
	Short: "Get current block height",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(currentBlockHeigh())
	},
}

// currentBlockHeigh get current height of block chain from server
func currentBlockHeigh() string {
	chainMeta, err := GetChainMeta()
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("%d", chainMeta.Height)
}
