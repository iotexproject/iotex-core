// Copyright (c) 2022 IoTeX
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package bc

import (
	"context"
	"fmt"
	"strconv"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

// Multi-language support
var (
	_bcBlockCmdShorts = map[config.Language]string{
		config.English: "Get block from block chain",
		config.Chinese: "获取IoTeX区块链中的区块",
	}
	_bcBlockCmdUses = map[config.Language]string{
		config.English: "block [HEIGHT|HASH] [--verbose]",
		config.Chinese: "block [高度|哈希] [--verbose]",
	}
	_flagVerboseUsages = map[config.Language]string{
		config.English: "returns block info and all actions within this block.",
		config.Chinese: "返回区块信息和区块内的所有事务",
	}
)

// NewBCBlockCmd represents the bc block command
func NewBCBlockCmd(c ioctl.Client) *cobra.Command {
	bcBlockCmdUse, _ := c.SelectTranslation(_bcBlockCmdUses)
	bcBlockCmdShort, _ := c.SelectTranslation(_bcBlockCmdShorts)
	flagVerboseUsage, _ := c.SelectTranslation(_flagVerboseUsages)
	var verbose bool

	cmd := &cobra.Command{
		Use:   bcBlockCmdUse,
		Short: bcBlockCmdShort,
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			var (
				err        error
				request    *iotexapi.GetBlockMetasRequest
				blockMeta  *iotextypes.BlockMeta
				blocksInfo []*iotexapi.BlockInfo
			)

			apiServiceClient, err := c.APIServiceClient()
			if err != nil {
				return err
			}
			if len(args) != 0 {
				request, err = parseArg(c, args[0])
			} else {
				request, err = parseArg(c, "")
			}
			if err != nil {
				return err
			}

			blockMeta, err = getBlockMeta(&apiServiceClient, request)
			if err != nil {
				return errors.Wrap(err, "failed to get block meta")
			}
			message := blockMessage{Node: c.Config().Endpoint, Block: blockMeta, Actions: nil}
			if verbose {
				blocksInfo, err = getActionInfoWithinBlock(&apiServiceClient, blockMeta.Height, uint64(blockMeta.NumActions))
				if err != nil {
					return errors.Wrap(err, "failed to get actions info")
				}
				for _, ele := range blocksInfo {
					for i, item := range ele.Block.Body.Actions {
						actionInfo := actionInfo{
							Version:         item.Core.Version,
							Nonce:           item.Core.Nonce,
							GasLimit:        item.Core.GasLimit,
							GasPrice:        item.Core.GasPrice,
							SenderPubKey:    item.SenderPubKey,
							Signature:       item.Signature,
							Status:          ele.Receipts[i].Status,
							BlkHeight:       ele.Receipts[i].BlkHeight,
							ActHash:         hash.Hash256b(ele.Receipts[i].ActHash),
							GasConsumed:     ele.Receipts[i].GasConsumed,
							ContractAddress: ele.Receipts[i].ContractAddress,
							Logs:            ele.Receipts[i].Logs,
						}
						message.Actions = append(message.Actions, actionInfo)
					}
				}
			}
			cmd.Println(fmt.Sprintf("Blockchain Node: %s\n%s\n%s", message.Node, JSONString(message.Block), JSONString(message.Actions)))
			return nil
		},
	}

	cmd.PersistentFlags().BoolVar(&verbose, "verbose", false, flagVerboseUsage)

	return cmd
}

type blockMessage struct {
	Node    string                `json:"node"`
	Block   *iotextypes.BlockMeta `json:"block"`
	Actions []actionInfo          `json:"actions"`
}

type actionInfo struct {
	Version         uint32            `protobuf:"varint,1,opt,name=version,proto3" json:"version,omitempty"`
	Nonce           uint64            `protobuf:"varint,2,opt,name=nonce,proto3" json:"nonce,omitempty"`
	GasLimit        uint64            `protobuf:"varint,3,opt,name=gasLimit,proto3" json:"gasLimit,omitempty"`
	GasPrice        string            `protobuf:"bytes,4,opt,name=gasPrice,proto3" json:"gasPrice,omitempty"`
	SenderPubKey    []byte            `protobuf:"bytes,2,opt,name=senderPubKey,proto3" json:"senderPubKey,omitempty"`
	Signature       []byte            `protobuf:"bytes,3,opt,name=signature,proto3" json:"signature,omitempty"`
	Status          uint64            `protobuf:"varint,4,opt,name=status,proto3" json:"status,omitempty"`
	BlkHeight       uint64            `protobuf:"varint,5,opt,name=blkheight,proto3" json:"blkheight,omitempty"`
	ActHash         hash.Hash256      `protobuf:"bytes,4,opt,name=acthash,proto3" json:"acthash,omitempty"`
	GasConsumed     uint64            `protobuf:"varint,6,opt,name=gasconsumed,proto3" json:"gasconsumed,omitempty"`
	ContractAddress string            `protobuf:"bytes,5,opt,name=contractaddress,proto3" json:"contractaddress,omitempty"`
	Logs            []*iotextypes.Log `protobuf:"bytes,6,opt,name=logs,proto3" json:"logs,omitempty"`
}

// getActionInfoByBlock gets action info by block hash with start index and action count
func getActionInfoWithinBlock(cli *iotexapi.APIServiceClient, height uint64, count uint64) ([]*iotexapi.BlockInfo, error) {
	request := iotexapi.GetRawBlocksRequest{StartHeight: height, Count: count, WithReceipts: true}
	ctx := context.Background()

	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	response, err := (*cli).GetRawBlocks(ctx, &request)
	if err != nil {
		if sta, ok := status.FromError(err); ok {
			if sta.Code() == codes.Unavailable {
				return nil, ioctl.ErrInvalidEndpointOrInsecure
			}
			return nil, errors.New(sta.Message())
		}
		return nil, errors.Wrap(err, "failed to invoke GetRawBlocks api")
	}
	if len(response.Blocks) == 0 {
		return nil, errors.New("no actions returned")
	}
	return response.Blocks, nil

}

// getBlockMeta gets block metadata
func getBlockMeta(cli *iotexapi.APIServiceClient, request *iotexapi.GetBlockMetasRequest) (*iotextypes.BlockMeta, error) {
	ctx := context.Background()

	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	response, err := (*cli).GetBlockMetas(ctx, request)
	if err != nil {
		if sta, ok := status.FromError(err); ok {
			if sta.Code() == codes.Unavailable {
				return nil, ioctl.ErrInvalidEndpointOrInsecure
			}
			return nil, errors.New(sta.Message())
		}
		return nil, errors.Wrap(err, "failed to invoke GetBlockMetas api")
	}
	if len(response.BlkMetas) == 0 {
		return nil, errors.New("no block returned")
	}
	return response.BlkMetas[0], nil
}

// parseArg parse argument and returns GetBlockMetasRequest
func parseArg(c ioctl.Client, arg string) (*iotexapi.GetBlockMetasRequest, error) {
	var (
		height uint64
		err    error
	)
	if arg != "" {
		height, err = strconv.ParseUint(arg, 10, 64)
		if err != nil {
			return &iotexapi.GetBlockMetasRequest{
				Lookup: &iotexapi.GetBlockMetasRequest_ByHash{
					ByHash: &iotexapi.GetBlockMetaByHashRequest{BlkHash: arg},
				},
			}, nil
		}
		if err = validator.ValidatePositiveNumber(int64(height)); err != nil {
			return nil, errors.Wrap(err, "invalid height")
		}
		return &iotexapi.GetBlockMetasRequest{
			Lookup: &iotexapi.GetBlockMetasRequest_ByIndex{
				ByIndex: &iotexapi.GetBlockMetasByIndexRequest{
					Start: height,
					Count: 1,
				},
			},
		}, nil
	}
	chainMeta, err := GetChainMeta(c)
	if err != nil {
		return nil, err
	}
	height = chainMeta.Height
	return &iotexapi.GetBlockMetasRequest{
		Lookup: &iotexapi.GetBlockMetasRequest_ByIndex{
			ByIndex: &iotexapi.GetBlockMetasByIndexRequest{
				Start: height,
				Count: 1,
			},
		},
	}, nil
}
