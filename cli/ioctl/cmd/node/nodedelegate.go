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

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/bc"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
)

var (
	epochNum  uint64
	nextEpoch bool
)

// nodeDelegateCmd represents the node delegate command
var nodeDelegateCmd = &cobra.Command{
	Use:   "delegate",
	Short: "print consensus delegates information in certain epoch",
	Args:  cobra.MaximumNArgs(1),
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
		return err.Error()
	}

	epockData := response.EpochData
	aliases := alias.GetAliasMap()
	formataliasLen := 0
	for _, delegateInfo := range response.BlockProducersInfo {
		if len(aliases[delegateInfo.Address]) > formataliasLen {
			formataliasLen = len(aliases[delegateInfo.Address])
		}
	}
	lines := make([]string, 0)
	lines = append(lines, fmt.Sprintf("Epoch: %d,  Start block height: %d,"+
		"  Total blocks in epoch: %d\n", epockData.Num, epockData.Height, response.TotalBlocks))
	formatTitleString := "%-41s   %-5s   %-" + strconv.Itoa(formataliasLen) +
		"s   %-6s   %-6s   %s"
	formatDataString := "%-41s   %5d   %-" + strconv.Itoa(formataliasLen) +
		"s   %-6s   %-6s   %s"
	lines = append(lines, fmt.Sprintf(formatTitleString,
		"Address", "Index", "Alias", "Status", "Blocks", "Votes"))
	for index, bp := range response.BlockProducersInfo {
		votes, ok := big.NewInt(0).SetString(bp.Votes, 10)
		if !ok {
			return "failed to convert votes into big int"
		}
		production := ""
		if bp.Active {
			production = strconv.Itoa(int(bp.Production))
		}
		lines = append(lines, fmt.Sprintf(formatDataString, bp.Address, index+1,
			aliases[bp.Address], nodeStatus[bp.Active], production,
			util.RauToString(votes, util.IotxDecimalNum)))
	}
	return strings.Join(lines, "\n")
}

func nextDelegates() {
	
}
