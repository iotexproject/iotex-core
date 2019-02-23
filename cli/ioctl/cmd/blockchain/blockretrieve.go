// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/pkg/hash"
	pb "github.com/iotexproject/iotex-core/protogen/iotexapi"
	pb1 "github.com/iotexproject/iotex-core/protogen/iotextypes"
)

var serverAddr string

// blockHeaderRetrieveCmd creates a new `ioctl blockchain blockheader` command
var blockHeaderRetrieveCmd = &cobra.Command{
	Use:   "blockheader",
	Short: "Retrieve a block from blockchain",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(blockHeaderRetrieve(args))
	},
}

func init() {
	blockHeaderRetrieveCmd.Flags().StringVarP(&serverAddr, "host", "s", "127.0.0.1:14014", "host of api server")
	BlockchainCmd.AddCommand(blockHeaderRetrieveCmd)
}

// blockHeaderRetrieve is the actual implementation
func blockHeaderRetrieve(args []string) (*pb1.BlockHeader, error) {
	// deduce height or hash method from user input
	method := "hash"
	var userHeight uint64
	var userHash hash.Hash256

	v := args[0]
	// check if user provided height
	if h, err := strconv.Atoi(v); err == nil {
		method = "height"
		userHeight = uint64(h)
	}

	// otherwise it's hash string
	if method == "hash" {
		userHash, _ = toHash256(v)
	}

	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	client := pb.NewAPIServiceClient(conn)
	req := &pb.GetBlockMetasRequest{}
	_, err = client.GetBlockMetas(context.Background(), req)

	if err != nil {
		return nil, err
	}

	// TODO - implement code to get blockheader
	return new(pb1.BlockHeader), nil
}

func toHash256(hashString string) (hash.Hash256, error) {
	bytes, err := hex.DecodeString(hashString)
	if err != nil {
		return hash.ZeroHash256, err
	}
	var _hash hash.Hash256
	copy(_hash[:], bytes)
	return _hash, nil
}
