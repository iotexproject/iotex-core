// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package node

import (
	"context"
	"fmt"
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	_rewardUses = map[config.Language]string{
		config.English: "reward [ALIAS|DELEGATE_ADDRESS]",
		config.Chinese: "reward [别名|委托地址]",
	}
	_rewardShorts = map[config.Language]string{
		config.English: "Query rewards",
		config.Chinese: "查询奖励",
	}
	_rewardPoolMessageTranslations = map[config.Language]string{
		config.English: "Available Reward: %s IOTX   Total Reward: %s IOTX",
		config.Chinese: "可用奖金: %s IOTX   总奖金: %s IOTX",
	}
)

// NewNodeRewardCmd represents the node reward command
func NewNodeRewardCmd(client ioctl.Client) *cobra.Command {
	var endpoint string
	var insecure bool

	use, _ := client.SelectTranslation(_rewardUses)
	short, _ := client.SelectTranslation(_rewardShorts)
	rewardPoolMessageTranslation, _ := client.SelectTranslation(_rewardPoolMessageTranslations)
	nc := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			var err error
			if len(args) == 0 {
				apiClient, err := client.APIServiceClient(ioctl.APIServiceConfig{Endpoint: endpoint, Insecure: insecure})
				if err != nil {
					return err
				}

				response, err := apiClient.ReadState(
					context.Background(),
					&iotexapi.ReadStateRequest{
						ProtocolID: []byte("rewarding"),
						MethodName: []byte("AvailableBalance"),
					},
				)

				if err != nil {
					sta, ok := status.FromError(err)
					if ok {
						return errors.New(sta.Message())
					}
					return errors.New("failed to invoke ReadState api")
				}

				availableRewardRau, ok := new(big.Int).SetString(string(response.Data), 10)
				if !ok {
					return errors.New("failed to convert string into big int")
				}

				response, err = apiClient.ReadState(
					context.Background(),
					&iotexapi.ReadStateRequest{
						ProtocolID: []byte("rewarding"),
						MethodName: []byte("TotalBalance"),
					},
				)
				if err != nil {
					sta, ok := status.FromError(err)
					if ok {
						return errors.New(sta.Message())
					}
					return errors.New("failed to invoke ReadState api")
				}
				totalRewardRau, ok := new(big.Int).SetString(string(response.Data), 10)
				if !ok {
					return errors.New("failed to convert string into big int")
				}

				message := rewardPoolMessage{
					AvailableReward: util.RauToString(availableRewardRau, util.IotxDecimalNum),
					TotalReward:     util.RauToString(totalRewardRau, util.IotxDecimalNum),
				}
				cmd.Println(message.String(rewardPoolMessageTranslation))

			} else {
				arg := args[0]
				address, err := client.Address(arg)
				if err != nil {
					return errors.New("failed to get address")
				}
				apiClient, err := client.APIServiceClient(ioctl.APIServiceConfig{
					Endpoint: endpoint,
					Insecure: insecure,
				})
				if err != nil {
					return err
				}

				response, err := apiClient.ReadState(
					context.Background(),
					&iotexapi.ReadStateRequest{
						ProtocolID: []byte("rewarding"),
						MethodName: []byte("UnclaimedBalance"),
						Arguments:  [][]byte{[]byte(address)},
					},
				)
				if err != nil {
					sta, ok := status.FromError(err)
					if ok {
						return errors.New(sta.Message())
					}
					return errors.New("failed to get version from server")
				}
				rewardRau, ok := new(big.Int).SetString(string(response.Data), 10)
				if !ok {
					return errors.New("failed to convert string into big int")
				}
				cmd.Println(fmt.Sprintf("%s: %s IOTX", address, util.RauToString(rewardRau, util.IotxDecimalNum)))
			}
			cmd.Println(err)
			return nil
		},
	}
	return nc
}

type rewardPoolMessage struct {
	AvailableReward string `json:"availableReward"`
	TotalReward     string `json:"totalReward"`
}

func (m *rewardPoolMessage) String(trans ...string) string {
	return fmt.Sprintf(trans[0], m.AvailableReward, m.TotalReward)
}
