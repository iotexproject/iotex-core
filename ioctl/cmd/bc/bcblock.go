// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import (
	"context"
	"fmt"
	"strconv"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

// Multi-language support
var (
	bcBlockCmdShorts = map[config.Language]string{
		config.English: "Get block from block chain",
		config.Chinese: "获取IoTeX区块链中的区块",
	}
	bcBlockCmdUses = map[config.Language]string{
		config.English: "block [HEIGHT|HASH]",
		config.Chinese: "block [高度|哈希]",
	}
)

// bcBlockCmd represents the bc Block command
var bcBlockCmd = &cobra.Command{
	Use:   config.TranslateInLang(bcBlockCmdUses, config.UILanguage),
	Short: config.TranslateInLang(bcBlockCmdShorts, config.UILanguage),
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := getBlock(args)
		return output.PrintError(err)
	},
}

type blockMessage struct {
	Node  string                `json:"node"`
	Block *iotextypes.BlockMeta `json:"block"`
}

func (m *blockMessage) String() string {
	if output.Format == "" {
		message := fmt.Sprintf("Blockchain Node: %s\n%s", m.Node, output.JSONString(m.Block))
		return message
	}
	return output.FormatString(output.Result, m)
}

// getBlock get block from block chain
func getBlock(args []string) error {
	var height uint64
	var err error
	isHeight := true
	if len(args) != 0 {
		height, err = strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			isHeight = false
		} else if err = validator.ValidatePositiveNumber(int64(height)); err != nil {
			return output.NewError(output.ValidationError, "invalid height", err)
		}
	} else {
		chainMeta, err := GetChainMeta()
		if err != nil {
			return output.NewError(0, "failed to get chain meta", err)
		}
		height = chainMeta.Height
	}
	var blockMeta *iotextypes.BlockMeta
	if isHeight {
		blockMeta, err = getBlockMetaByHeight(height)
	} else {
		blockMeta, err = getBlockMetaByHash(args[0])
	}
	if err != nil {
		return output.NewError(0, "failed to get block meta", err)
	}
	message := blockMessage{Node: config.ReadConfig.Endpoint, Block: blockMeta}
	fmt.Println(message.String())
	return nil
}

// getBlockMetaByHeight gets block metadata by height
func getBlockMetaByHeight(height uint64) (*iotextypes.BlockMeta, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
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

	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	response, err := cli.GetBlockMetas(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return nil, output.NewError(output.APIError, sta.Message(), nil)
		}
		return nil, output.NewError(output.NetworkError, "failed to invoke GetBlockMetas api", err)
	}
	if len(response.BlkMetas) == 0 {
		return nil, output.NewError(output.APIError, "no block returned", err)
	}
	return response.BlkMetas[0], nil
}

// getBlockMetaByHash gets block metadata by hash
func getBlockMetaByHash(hash string) (*iotextypes.BlockMeta, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	request := &iotexapi.GetBlockMetasRequest{
		Lookup: &iotexapi.GetBlockMetasRequest_ByHash{
			ByHash: &iotexapi.GetBlockMetaByHashRequest{BlkHash: hash},
		},
	}
	ctx := context.Background()

	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	response, err := cli.GetBlockMetas(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return nil, output.NewError(output.APIError, sta.Message(), nil)
		}
		return nil, output.NewError(output.NetworkError, "failed to invoke GetBlockMetas api", err)
	}
	if len(response.BlkMetas) == 0 {
		return nil, output.NewError(output.APIError, "no block returned", err)
	}
	return response.BlkMetas[0], nil
}
