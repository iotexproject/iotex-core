// Copyright (c) 2019 IoTeX Foundation
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

	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// nodeRewardCmd represents the node reward command
var nodeRewardCmd = &cobra.Command{
	Use:   "reward [ALIAS|DELEGATE_ADDRESS]",
	Short: "Query rewards",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		var err error
		if len(args) == 0 {
			err = rewardPool()
		} else {
			err = reward(args[0])
		}
		return output.PrintError(err)
	},
}

type rewardPoolMessage struct {
	AvailableReward string `json:"availableReward"`
	TotalReward     string `json:"totalReward"`
}

func (m *rewardPoolMessage) String() string {
	if output.Format == "" {
		message := fmt.Sprintf("Available Reward: %s IOTX   Total Reward: %s IOTX",
			m.AvailableReward, m.TotalReward)
		return message
	}
	return output.FormatString(output.Result, m)
}

type rewardMessage struct {
	Address string `json:"address"`
	Reward  string `json:"reward"`
}

func (m *rewardMessage) String() string {
	if output.Format == "" {
		message := fmt.Sprintf("%s: %s IOTX", m.Address, m.Reward)
		return message
	}
	return output.FormatString(output.Result, m)
}

func rewardPool() error {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()
	request := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(rewarding.ProtocolID),
		MethodName: []byte("AvailableBalance"),
	}
	response, err := cli.ReadState(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return output.NewError(output.APIError, sta.Message(), nil)
		}
		return output.NewError(output.NetworkError, "failed to invoke ReadState api", err)
	}
	availableRewardRau, ok := big.NewInt(0).SetString(string(response.Data), 10)
	if !ok {
		return output.NewError(output.ConvertError, "failed to convert string into big int", err)
	}
	request = &iotexapi.ReadStateRequest{
		ProtocolID: []byte(rewarding.ProtocolID),
		MethodName: []byte("TotalBalance"),
	}
	response, err = cli.ReadState(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return output.NewError(output.APIError, sta.Message(), nil)
		}
		return output.NewError(output.NetworkError, "failed to invoke ReadState api", err)
	}
	totalRewardRau, ok := big.NewInt(0).SetString(string(response.Data), 10)
	if !ok {
		return output.NewError(output.ConvertError, "failed to convert string into big int", err)
	}
	message := rewardPoolMessage{
		AvailableReward: util.RauToString(availableRewardRau, util.IotxDecimalNum),
		TotalReward:     util.RauToString(totalRewardRau, util.IotxDecimalNum),
	}
	fmt.Println(message.String())
	return nil
}

func reward(arg string) error {
	address, err := util.Address(arg)
	if err != nil {
		return output.NewError(output.AddressError, "failed to get address", err)
	}
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return output.NewError(output.NetworkError, "failed to connect to endpoint", err)
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
			return output.NewError(output.APIError, sta.Message(), nil)
		}
		return output.NewError(output.NetworkError, "failed to invoke ReadState api", err)
	}
	rewardRau, ok := big.NewInt(0).SetString(string(response.Data), 10)
	if !ok {
		return output.NewError(output.ConvertError, "failed to convert string into big int", err)
	}
	message := rewardMessage{Address: address, Reward: util.RauToString(rewardRau, util.IotxDecimalNum)}
	fmt.Println(message.String())
	return nil
}
