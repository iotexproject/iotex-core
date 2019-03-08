// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/bc"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
)

// nodeProductivityCmd represents the node productivity command
var nodeProductivityCmd = &cobra.Command{
	Use:   "productivity [DELEGATE]",
	Short: "get productivity of delegates",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(productivity(args))
	},
}

func productivity(args []string) string {
	delegate := ""
	var err error
	if len(args) != 0 {
		delegate, err = account.Address(args[0])
		if err != nil {
			return err.Error()
		}
	}
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
	request := &iotexapi.GetProductivityRequest{EpochNumber: epochNum}
	ctx := context.Background()
	response, err := cli.GetProductivity(ctx, request)
	if err != nil {
		return err.Error()
	}
	if len(delegate) != 0 {
		return fmt.Sprintf("%s: %d (produced) / %d (total of epoch %d)", delegate,
			response.BlksPerDelegate[delegate], response.TotalBlks, epochNum)
	}
	lines := make([]string, 0)
	for delegate, productivity := range response.BlksPerDelegate {
		lines = append(lines, fmt.Sprintf("%s: %d (produced) / %d (total of epoch %d)",
			delegate, productivity, response.TotalBlks, epochNum))
	}
	return strings.Join(lines, "\n")
}
