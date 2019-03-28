// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/cli/ioctl/validator"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

// bcBlockCmd represents the bc Block command
var bcBlockCmd = &cobra.Command{
	Use:   "block [HEIGHT|HASH]",
	Short: "Get block from block chain",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(getBlock(args))
	},
}

// getBlock get block from block chain
func getBlock(args []string) string {
	var height uint64
	var err error
	isHeight := true
	if len(args) != 0 {
		height, err = strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			isHeight = false
		} else if err = validator.ValidatePositiveNumber(int64(height)); err != nil {
			return err.Error()
		}
	} else {
		chainMeta, err := GetChainMeta()
		if err != nil {
			return err.Error()
		}
		height = chainMeta.Height
	}
	var blockMeta *iotextypes.BlockMeta
	if isHeight {
		blockMeta, err = GetBlockMetaByHeight(height)
	} else {
		blockMeta, err = GetBlockMetaByHash(args[0])
	}
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("Transactions: %d\n", blockMeta.NumActions) +
		fmt.Sprintf("Height: %d\n", blockMeta.Height) +
		fmt.Sprintf("Total Amount: %s\n", blockMeta.TransferAmount) +
		fmt.Sprintf("Timestamp: %d\n", blockMeta.Timestamp) +
		fmt.Sprintf("Producer Public Key: %s\n", blockMeta.ProducerAddress) +
		fmt.Sprintf("Transactions Root: %s\n", blockMeta.TxRoot) +
		fmt.Sprintf("Receipt Root: %s\n", blockMeta.ReceiptRoot) +
		fmt.Sprintf("Delta State Digest: %s\n", blockMeta.DeltaStateDigest) +
		fmt.Sprintf("Hash: %s", blockMeta.Hash)
}

// GetBlockMetaByHeight gets block metadata by height
func GetBlockMetaByHeight(height uint64) (*iotextypes.BlockMeta, error) {
	conn, err := util.ConnectToEndpoint()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	request := &iotexapi.GetBlockMetasRequest{
		Lookup: &iotexapi.GetBlockMetasRequest_ByIndex{
			ByIndex: &iotexapi.GetBlockMetasByIndexRequest{
				Start: height,
				Count: 1,
			},
		},
	}
	ctx := context.Background()
	response, err := cli.GetBlockMetas(ctx, request)
	if err != nil {
		return nil, err
	}
	if len(response.BlkMetas) == 0 {
		return nil, fmt.Errorf("no block returned")
	}
	return response.BlkMetas[0], nil
}

// GetBlockMetaByHash gets block metadata by hash
func GetBlockMetaByHash(hash string) (*iotextypes.BlockMeta, error) {
	conn, err := util.ConnectToEndpoint()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	request := &iotexapi.GetBlockMetasRequest{
		Lookup: &iotexapi.GetBlockMetasRequest_ByHash{
			ByHash: &iotexapi.GetBlockMetaByHashRequest{BlkHash: hash},
		},
	}
	ctx := context.Background()
	response, err := cli.GetBlockMetas(ctx, request)
	if err != nil {
		return nil, err
	}
	if len(response.BlkMetas) == 0 {
		return nil, fmt.Errorf("no block returned")
	}
	return response.BlkMetas[0], nil
}
