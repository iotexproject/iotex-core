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

var (
	verbose bool
)

// Multi-language support
var (
	bcBlockCmdShorts = map[config.Language]string{
		config.English: "Get block from block chain",
		config.Chinese: "获取IoTeX区块链中的区块",
	}
	bcBlockCmdUses = map[config.Language]string{
		config.English: "block [HEIGHT|HASH] [--verbose]",
		config.Chinese: "block [高度|哈希] [--verbose]",
	}
	flagVerboseUsage = map[config.Language]string{
		config.English: "returns block info and all actions within this block.",
		config.Chinese: "返回区块信息和区块内的所有事务",
	}
)

// bcBlockCmd represents the bc Block command
var bcBlockCmd = &cobra.Command{
	Use:   config.TranslateInLang(bcBlockCmdUses, config.UILanguage),
	Short: config.TranslateInLang(bcBlockCmdShorts, config.UILanguage),
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := getBlock(args)
		return output.PrintError(err)
	},
}

func init() {
	bcBlockCmd.Flags().BoolVar(&verbose, "verbose", false, config.TranslateInLang(flagVerboseUsage, config.UILanguage))
}

type blockMessage struct {
	Node    string                `json:"node"`
	Block   *iotextypes.BlockMeta `json:"block"`
	Actions []actionInfo          `json:actions`
}

type actionInfo struct {
	Version      uint32 `protobuf:"varint,1,opt,name=version,proto3" json:"version,omitempty"`
	Nonce        uint64 `protobuf:"varint,2,opt,name=nonce,proto3" json:"nonce,omitempty"`
	GasLimit     uint64 `protobuf:"varint,3,opt,name=gasLimit,proto3" json:"gasLimit,omitempty"`
	GasPrice     string `protobuf:"bytes,4,opt,name=gasPrice,proto3" json:"gasPrice,omitempty"`
	SenderPubKey []byte `protobuf:"bytes,2,opt,name=senderPubKey,proto3" json:"senderPubKey,omitempty"`
	Signature    []byte `protobuf:"bytes,3,opt,name=signature,proto3" json:"signature,omitempty"`
}

func (m *blockMessage) String() string {
	if output.Format == "" {
		message := fmt.Sprintf("Blockchain Node: %s\n%s\n%s", m.Node, output.JSONString(m.Block), output.JSONString(m.Actions))
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
	var blocksInfo []*iotexapi.BlockInfo
	if isHeight {
		blockMeta, err = getBlockMetaByHeight(height)
	} else {
		blockMeta, err = getBlockMetaByHash(args[0])
	}
	if err != nil {
		return output.NewError(0, "failed to get block meta", err)
	}
	blockInfoMessage := blockMessage{Node: config.ReadConfig.Endpoint, Block: blockMeta, Actions: nil}
	if verbose {
		blocksInfo, err = getActionInfoWithinBlock(blockMeta.Height, uint64(blockMeta.NumActions))
		if err != nil {
			return output.NewError(0, "failed to get actions info", err)
		}
		for _, ele := range blocksInfo {
			for _, item := range ele.Block.Body.Actions {
				actionInfo := actionInfo{
					Version:      item.Core.Version,
					Nonce:        item.Core.Nonce,
					GasLimit:     item.Core.GasLimit,
					GasPrice:     item.Core.GasPrice,
					SenderPubKey: item.SenderPubKey,
					Signature:    item.Signature,
				}
				blockInfoMessage.Actions = append(blockInfoMessage.Actions, actionInfo)
			}
		}
	}
	fmt.Println(blockInfoMessage.String())
	return nil
}

// getActionInfoByBlock gets action info by block hash with start index and action count
func getActionInfoWithinBlock(height uint64, count uint64) ([]*iotexapi.BlockInfo, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	request := iotexapi.GetRawBlocksRequest{StartHeight: height, Count: count, WithReceipts: true}
	ctx := context.Background()

	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	response, err := cli.GetRawBlocks(ctx, &request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return nil, output.NewError(output.APIError, sta.Message(), nil)
		}
		return nil, output.NewError(output.NetworkError, "failed to invoke GetRawBlocks api", err)
	}
	if len(response.Blocks) == 0 {
		return nil, output.NewError(output.APIError, "no actions returned", err)
	}
	return response.Blocks, nil

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
