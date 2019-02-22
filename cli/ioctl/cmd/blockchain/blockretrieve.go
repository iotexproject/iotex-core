// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// blockRetrieveCmd creates a new `ioctl blockchain blockheader` command
var blockRetrieveCmd = &cobra.Command{
	Use:   "blockheader",
	Short: "Retrieve a block from blockchain",
	Args:  cobra.ExactArgs(1), // TODO - add help for
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(blockRetrieve(args))
	},
}

func init() {
	BlockchainCmd.AddCommand(blockRetrieveCmd)
}

// blockRetrieve is the actual implementation
func blockRetrieve(args []string) string {
	fmt.Println("TODO: Implement it")

	// default method to getBlock
	method := "hash"

	// check for user input to decide method at runtime
	v := args[0]
	if _, err := strconv.Atoi(v); err == nil {
		method = "height"
	}

	if method == "hash" {
		// call GetblockByHash
		return "call GetByHash method here"
	}
	return "call GetByHeight method here"
}
