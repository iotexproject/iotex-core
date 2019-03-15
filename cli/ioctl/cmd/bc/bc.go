// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

// BCCmd represents the bc(block chain) command
var BCCmd = &cobra.Command{
	Use:   "bc",
	Short: "Deal with block chain of IoTeX blockchain",
	Args:  cobra.ExactArgs(1),
}

func init() {
	BCCmd.AddCommand(bcBlockCmd)
	BCCmd.AddCommand(bcHeightCmd)
}

// GetChainMeta gets block chain metadata
func GetChainMeta() (*iotextypes.ChainMeta, error) {
	conn, err := util.ConnectToEndpoint()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	request := iotexapi.GetChainMetaRequest{}
	ctx := context.Background()
	response, err := cli.GetChainMeta(ctx, &request)
	if err != nil {
		return nil, err
	}
	return response.ChainMeta, err
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
				Start: height - 1,
				Count: 1,
			},
		},
	}
	ctx := context.Background()
	response, err := cli.GetBlockMetas(ctx, request)
	if err != nil {
		return nil, err
	}
	return response.BlkMetas[0], err
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
	return response.BlkMetas[0], err
}
