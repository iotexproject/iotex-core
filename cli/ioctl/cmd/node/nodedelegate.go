// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package node

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/bc"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/state"
)

var (
	epochNum  uint64
	nextEpoch bool
)

// nodeDelegateCmd represents the node delegate command
var nodeDelegateCmd = &cobra.Command{
	Use:   "delegate [-e epoch-num|-n]",
	Short: "print consensus delegates information in certain epoch",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		if nextEpoch {
			fmt.Println(nextDelegates())
		} else {
			fmt.Println(delegates())
		}
	},
}

func init() {
	nodeDelegateCmd.Flags().Uint64VarP(&epochNum, "epoch-num", "e", 0, "specify specific epoch")
	nodeDelegateCmd.Flags().BoolVarP(&nextEpoch, "next-epoch", "n", false,
		"query delegate of upcoming epoch")
}

func delegates() string {
	nodeStatus := map[bool]string{true: "active", false: ""}
	if epochNum == 0 {
		chainMeta, err := bc.GetChainMeta()
		if err != nil {
			return err.Error()
		}
		epochNum = chainMeta.Epoch.Num
	}
	conn, err := util.ConnectToEndpoint()
	if err != nil {
		return err.Error()
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	request := &iotexapi.GetEpochMetaRequest{EpochNumber: epochNum}
	ctx := context.Background()
	response, err := cli.GetEpochMeta(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return sta.Message()
		}
		return err.Error()
	}

	epochData := response.EpochData
	aliases := alias.GetAliasMap()
	formataliasLen := 5
	for _, bp := range response.BlockProducersInfo {
		if len(aliases[bp.Address]) > formataliasLen {
			formataliasLen = len(aliases[bp.Address])
		}
	}
	lines := make([]string, 0)
	lines = append(lines, fmt.Sprintf("Epoch: %d,  Start block height: %d,"+
		"  Total blocks in epoch: %d\n", epochData.Num, epochData.Height, response.TotalBlocks))
	formatTitleString := "%-41s   %-4s   %-" + strconv.Itoa(formataliasLen) +
		"s   %-6s   %-6s   %s"
	formatDataString := "%-41s   %4d   %-" + strconv.Itoa(formataliasLen) +
		"s   %-6s   %-6s   %s"
	lines = append(lines, fmt.Sprintf(formatTitleString,
		"Address", "Rank", "Alias", "Status", "Blocks", "Votes"))
	for rank, bp := range response.BlockProducersInfo {
		votes, ok := big.NewInt(0).SetString(bp.Votes, 10)
		if !ok {
			return "failed to convert votes into big int"
		}
		production := ""
		if bp.Active {
			production = strconv.Itoa(int(bp.Production))
		}
		lines = append(lines, fmt.Sprintf(formatDataString, bp.Address, rank+1,
			aliases[bp.Address], nodeStatus[bp.Active], production,
			util.RauToString(votes, util.IotxDecimalNum)))
	}
	return strings.Join(lines, "\n")
}

func nextDelegates() string {
	nodeStatus := map[bool]string{true: "active", false: ""}
	chainMeta, err := bc.GetChainMeta()
	if err != nil {
		return err.Error()
	}
	epochNum = chainMeta.Epoch.Num + 1
	conn, err := util.ConnectToEndpoint()
	if err != nil {
		return err.Error()
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()
	request := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(poll.ProtocolID),
		MethodName: []byte("ActiveBlockProducersByEpoch"),
		Arguments:  [][]byte{byteutil.Uint64ToBytes(epochNum)},
	}
	abpResponse, err := cli.ReadState(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok && sta.Code() == codes.NotFound {
			return fmt.Sprintf("delegates of upcoming epoch #%d are not determined", epochNum)
		} else if ok {
			return sta.Message()
		}
		return err.Error()
	}
	var ABPs state.CandidateList
	if err := ABPs.Deserialize(abpResponse.Data); err != nil {
		return err.Error()
	}
	request = &iotexapi.ReadStateRequest{
		ProtocolID: []byte(poll.ProtocolID),
		MethodName: []byte("BlockProducersByEpoch"),
		Arguments:  [][]byte{byteutil.Uint64ToBytes(epochNum)},
	}
	bpResponse, err := cli.ReadState(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return sta.Message()
		}
		return err.Error()
	}
	var BPs state.CandidateList
	if err := BPs.Deserialize(bpResponse.Data); err != nil {
		return err.Error()
	}
	isActive := make(map[string]bool)
	for _, abp := range ABPs {
		isActive[abp.Address] = true
	}

	aliases := alias.GetAliasMap()
	formataliasLen := 5
	for _, bp := range BPs {
		if len(aliases[bp.Address]) > formataliasLen {
			formataliasLen = len(aliases[bp.Address])
		}
	}
	lines := make([]string, 0)
	lines = append(lines, fmt.Sprintf("Epoch: %d\n", epochNum))
	formatTitleString := "%-41s   %-4s   %-" + strconv.Itoa(formataliasLen) +
		"s   %-6s   %s"
	formatDataString := "%-41s   %4d   %-" + strconv.Itoa(formataliasLen) +
		"s   %-6s   %s"
	lines = append(lines, fmt.Sprintf(formatTitleString,
		"Address", "Rank", "Alias", "Status", "Votes"))

	for rank, bp := range BPs {
		votes := big.NewInt(0).SetBytes(bp.Votes.Bytes())
		lines = append(lines, fmt.Sprintf(formatDataString, bp.Address, rank+1,
			aliases[bp.Address], nodeStatus[isActive[bp.Address]],
			util.RauToString(votes, util.IotxDecimalNum)))
	}
	return strings.Join(lines, "\n")
}
