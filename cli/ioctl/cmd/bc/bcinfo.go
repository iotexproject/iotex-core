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

// bcInfoCmd represents the bc info command
var bcInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get current block chain information",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(bcInfo())
	},
}

// bcInfo get current information of block chain from server
func bcInfo() string {
	chainMeta, err := GetChainMeta()
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("height:%d  numActions:%d  tps:%d\nepochNum:%d  epochStartHeight:%d"+
		"  gravityChainStartHeight:%d", chainMeta.Height, chainMeta.NumActions, chainMeta.Tps,
		chainMeta.Epoch.Num, chainMeta.Epoch.Height, chainMeta.Epoch.GravityChainStartHeight)
}
