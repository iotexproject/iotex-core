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

	"github.com/golang/protobuf/ptypes/timestamp"
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

type actionsInfoMessage struct {
	ActionInfo []actionInfo `json:"actions"`
}

type actionInfo struct {
	ActHash   string               `protobuf:"bytes,2,opt,name=actHash,proto3" json:"actHash,omitempty"`
	Sender    string               `protobuf:"bytes,6,opt,name=sender,proto3" json:"sender,omitempty"`
	GasFee    string               `protobuf:"bytes,7,opt,name=gasFee,proto3" json:"gasFee,omitempty"`
	Timestamp *timestamp.Timestamp `protobuf:"bytes,4,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
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
	var actionInfos []*iotexapi.ActionInfo
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
		actionInfos, err = getActionInfoByBlock(blockMeta.Hash, blockMeta.Height, uint64(blockMeta.NumActions))
		if err != nil {
			return output.NewError(0, "failed to get actions info", err)
		}
		for _, ele := range actionInfos {
			actionInfo := actionInfo{
				ActHash:   ele.ActHash,
				Sender:    ele.Sender,
				GasFee:    ele.GasFee,
				Timestamp: ele.Timestamp,
			}
			blockInfoMessage.Actions = append(blockInfoMessage.Actions, actionInfo)
		}
	}
	fmt.Println(blockInfoMessage.String())
	return nil
}

// getActionInfoByBlock gets action info by block hash with start index and action count
func getActionInfoByBlock(hash string, height uint64, count uint64) ([]*iotexapi.ActionInfo, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	//request := iotexapi.GetActionsRequest{Lookup: &iotexapi.GetActionsRequest_ByBlk{ByBlk: &iotexapi.GetActionsByBlockRequest{BlkHash: hash, Start: height, Count: count}}}
	request := iotexapi.GetActionsRequest{Lookup: &iotexapi.GetActionsRequest_ByIndex{ByIndex: &iotexapi.GetActionsByIndexRequest{Start: height, Count: count}}}
	ctx := context.Background()

	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	response, err := cli.GetActions(ctx, &request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return nil, output.NewError(output.APIError, sta.Message(), nil)
		}
		return nil, output.NewError(output.NetworkError, "failed to invoke GetActions api", err)
	}
	if len(response.ActionInfo) == 0 {
		return nil, output.NewError(output.APIError, "no actions returned", err)
	}
	return response.ActionInfo, nil

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
