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

	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/bc"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

// nodeOrderCmd represents the node delegate command
var nodeOrderCmd = &cobra.Command{
	Use:   "order",
	Short: "Print active consensus delegates in order",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(order())
	},
}

func init() {
	nodeOrderCmd.Flags().Uint64VarP(&epochNum, "epoch-num", "e", 0, "specify specific epoch")
	nodeOrderCmd.Flags().BoolVarP(&nextEpoch, "next-epoch", "n", false,
		"query delegates of upcoming epoch")
}

func order() string {
	if epochNum == 0 {
		chainMeta, err := bc.GetChainMeta()
		if err != nil {
			return err.Error()
		}
		epochNum = chainMeta.Epoch.Num
		if nextEpoch {
			epochNum++
		}
	}
	conn, err := util.ConnectToEndpoint()
	if err != nil {
		return err.Error()
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	request := &iotexapi.ReadStateRequest{
		ProtocolID: []byte("poll"),
		MethodName: []byte("ActiveBlockProducersByEpoch"),
		Arguments:  [][]byte{[]byte(strconv.Itoa(int(epochNum)))},
	}
	ctx := context.Background()
	response, err := cli.ReadState(ctx, request)
	if err != nil {
		if nextEpoch {
			status, ok := status.FromError(err)
			if ok && status.Code() == codes.NotFound {
				return fmt.Sprintf("delegates in upcoming epoch#%d are not determined", epochNum)
			}
		}
		return err.Error()
	}

	var ABPs iotextypes.CandidateList
	if err := proto.Unmarshal(response.Data, &ABPs); err != nil {
		return err.Error()
	}
	aliases := alias.GetAliasMap()
	formataliasLen := 5
	for _, abp := range ABPs.Candidates {
		if len(aliases[abp.Address]) > formataliasLen {
			formataliasLen = len(aliases[abp.Address])
		}
	}
	lines := make([]string, 0)
	lines = append(lines, fmt.Sprintf("Epoch: %d\n", epochNum))
	formatTitleString := "%-41s   %-" + strconv.Itoa(formataliasLen) + "s   %-5s   %s"
	formatDataString := "%-41s   %-" + strconv.Itoa(formataliasLen) + "s   %-5d   %s"
	lines = append(lines, fmt.Sprintf(formatTitleString,
		"Address", "Alias", "Order", "Votes"))
	for index, abp := range ABPs.Candidates {
		votes := big.NewInt(0).SetBytes(abp.Votes)
		lines = append(lines, fmt.Sprintf(formatDataString, abp.Address,
			aliases[abp.Address], index+1, votes))
	}
	return strings.Join(lines, "\n")
}
