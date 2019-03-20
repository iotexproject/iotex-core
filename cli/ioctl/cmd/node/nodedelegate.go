// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package node

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/bc"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
)

// nodeDelegateCmd represents the node delegate command
var nodeDelegateCmd = &cobra.Command{
	Use:   "delegate [DELEGATE]",
	Short: "list delegates and number of blocks produced",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(delegate(args))
	},
}

func delegate(args []string) string {
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
		name, err := account.Name(delegate)
		if err != nil && err != account.ErrNoNameFound {
			return err.Error()
		}
		return fmt.Sprintf("Epoch: %d, Total blocks: %d\n", epochNum, response.TotalBlks) +
			fmt.Sprintf("%s  %s  %d", delegate, name, response.BlksPerDelegate[delegate])
	}

	names, err := account.GetNameMap()
	if err != nil {
		return err.Error()
	}
	formatNameLen := 0
	for delegate := range response.BlksPerDelegate {
		if len(names[delegate]) > formatNameLen {
			formatNameLen = len(names[delegate])
		}
	}
	formatTitleString := "%-41s  %-" + strconv.Itoa(formatNameLen) + "s  %-s"
	formatDataString := "%-41s  %-" + strconv.Itoa(formatNameLen) + "s  %-d"
	lines := make([]string, 0)
	lines = append(lines, fmt.Sprintf("Epoch: %d, Total blocks: %d\n",
		epochNum, response.TotalBlks))
	lines = append(lines, fmt.Sprintf(formatTitleString, "Address", "Name", "Blocks"))
	for delegate, productivity := range response.BlksPerDelegate {
		lines = append(lines, fmt.Sprintf(formatDataString, delegate, names[delegate], productivity))
	}
	return strings.Join(lines, "\n")
}
