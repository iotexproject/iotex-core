// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"fmt"
	"strconv"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/explorer"
	"github.com/spf13/cobra"
)

var address string

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
	blockRetrieveCmd.Flags().StringVarP(&address, "host", "s", "127.0.0.1:14014", "host of api server")
	BlockchainCmd.AddCommand(blockRetrieveCmd)
}

// blockRetrieve is the actual implementation
func blockRetrieve(args []string) (*block.Block, error) {
	// deduce height or hash method from user input
	method := "hash"
	var userHeight uint64

	v := args[0]
	if h, err := strconv.Atoi(v); err == nil {
		method = "height"
		userHeight = uint64(h)
	}

	if method == "hash" {
		fmt.Println("complete Hash method")
	}
	fmt.Println("user height is ", userHeight)

	client := explorer.NewExplorerProxy(address)
	fmt.Println(client)

	// TODO - How to call GetBlockByHeight method with this server
	//var bc blockchain.Blockchain
	//resp, err := bc.GetBlockByHeight(userHeight)
	//if err != nil {
	//	fmt.Println(err)
	//}
	//fmt.Println(resp)
	//return resp, err
	return new(block.Block), nil
}
