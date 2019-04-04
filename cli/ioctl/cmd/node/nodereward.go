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

	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
)

// nodeRewardCmd represents the node reward command
var nodeRewardCmd = &cobra.Command{
	Use:   "reward (ALIAS|DELEGATE_ADDRESS)",
	Short: "Query unclaimed rewards",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := reward(args)
		if err == nil {
			println(output)
		}
		return err
	},
}

func reward(args []string) (string, error) {
	address, err := alias.Address(args[0])
	if err != nil {
		return "", err
	}
	conn, err := util.ConnectToEndpoint()
	if err != nil {
		return "", err
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()
	request := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(rewarding.ProtocolID),
		MethodName: []byte("UnclaimedBalance"),
		Arguments:  [][]byte{[]byte(address)},
	}
	response, err := cli.ReadState(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return "", fmt.Errorf(sta.Message())
		}
		return "", err
	}
	rewardRau, ok := big.NewInt(0).SetString(string(response.Data), 10)
	if !ok {
		return "", fmt.Errorf("failed to convert string into big int")
	}
	return fmt.Sprintf("%s: %s IOTX", address,
		util.RauToString(rewardRau, util.IotxDecimalNum)), nil
}
