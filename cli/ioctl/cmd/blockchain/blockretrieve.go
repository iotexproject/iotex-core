// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"fmt"

	"github.com/spf13/cobra"
)

var blockheaderid int

// blockRetrieveCmd creates a new `ioctl blockchain blockheader` command
var blockRetrieveCmd = &cobra.Command{
	Use:   "blockheader",
	Short: "Retrieve a block from blockchain",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(blockRetrieve(args))
	},
}

func init() {
	blockRetrieveCmd.Flags().IntVarP(&blockheaderid, "blockheaderid", "", 0, "retrieves a full block from the blockchain")
	BlockchainCmd.AddCommand(blockRetrieveCmd)
}

// blockRetrieve is the actual implementation
func blockRetrieve(args []string) string {
	return "TODO: Implement it"
}
