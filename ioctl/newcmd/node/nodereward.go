// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package node

import (
	"context"
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
	_rewardCmdUses = map[config.Language]string{
		config.English: "reward unclaimed|pool [ALIAS|DELEGATE_ADDRESS]",
		config.Chinese: "reward 未支取|奖金池 [别名|委托地址]",
	}
	_rewardCmdShorts = map[config.Language]string{
		config.English: "Query rewards",
		config.Chinese: "查询奖励",
	}
	_rewardPoolLong = map[config.Language]string{
		config.English: "ioctl node reward pool returns unclaimed and available Rewards in fund pool.\nTotalUnclaimed is the amount of all delegates that have been issued but are not claimed;\nTotalAvailable is the amount of balance that has not been issued to anyone.\n\nioctl node reward unclaimed [ALIAS|DELEGATE_ADDRESS] returns unclaimed rewards of a specific delegate.",
		config.Chinese: "ioctl node reward 返回奖金池中的未支取奖励和可获取的奖励. TotalUnclaimed是所有代表已被发放但未支取的奖励的总和; TotalAvailable 是奖金池中未被发放的奖励的总和.\n\nioctl node [ALIAS|DELEGATE_ADDRESS] 返回特定代表的已被发放但未支取的奖励.",
	}
)

// NewNodeRewardCmd represents the node reward command
func NewNodeRewardCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_rewardCmdUses)
	short, _ := client.SelectTranslation(_rewardCmdShorts)
	long, _ := client.SelectTranslation(_rewardPoolLong)

	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.RangeArgs(1, 2),
		Long:  long,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			switch args[0] {
			case "pool":
				if len(args) != 1 {
					return errors.New("wrong number of arg(s) for ioctl node reward pool command. \nRun 'ioctl node reward --help' for usage")
				}
				message, err := rewardPool(client)
				if err != nil {
					return err
				}
				cmd.Printf("Total Unclaimed:\t %s IOTX\nTotal Available:\t %s IOTX\nTotal Balance:\t\t %s IOTX",
					message.TotalUnclaimed, message.TotalAvailable, message.TotalBalance)
			case "unclaimed":
				if len(args) != 2 {
					return errors.New("wrong number of arg(s) for ioctl node reward unclaimed [ALIAS|DELEGATE_ADDRESS] command. \nRun 'ioctl node reward --help' for usage")
				}
				message, err := reward(client, args[1])
				if err != nil {
					return err
				}
				cmd.Printf("%s: %s IOTX", message.Address, message.Reward)
			default:
				return errors.New("unknown command. \nRun 'ioctl node reward --help' for usage")
			}
			return nil
		},
	}
}

type rewardPoolMessage struct {
	TotalBalance   string `json:"TotalBalance"`
	TotalUnclaimed string `json:"TotalUnclaimed"`
	TotalAvailable string `json:"TotalAvailable"`
}

type rewardMessage struct {
	Address string `json:"address"`
	Reward  string `json:"reward"`
}

func rewardPool(client ioctl.Client) (rewardPoolMessage, error) {
	var endpoint string
	var insecure bool

	apiClient, err := client.APIServiceClient(ioctl.APIServiceConfig{Endpoint: endpoint, Insecure: insecure})
	if err != nil {
		return rewardPoolMessage{}, err
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
			return rewardPoolMessage{}, errors.New(sta.Message())
		}
		return rewardPoolMessage{}, errors.New("failed to invoke ReadState api")
	}

	availableRewardRau, ok := new(big.Int).SetString(string(response.Data), 10)
	if !ok {
		return rewardPoolMessage{}, errors.New("failed to convert string into big int")
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
			return rewardPoolMessage{}, errors.New(sta.Message())
		}
		return rewardPoolMessage{}, errors.New("failed to invoke ReadState api")
	}
	totalRewardRau, ok := new(big.Int).SetString(string(response.Data), 10)
	if !ok {
		return rewardPoolMessage{}, errors.New("failed to convert string into big int")
	}

	totalUnclaimedRewardRau := big.NewInt(0)
	totalUnclaimedRewardRau.Sub(totalRewardRau, availableRewardRau)
	message := rewardPoolMessage{
		TotalBalance:   util.RauToString(totalRewardRau, util.IotxDecimalNum),
		TotalUnclaimed: util.RauToString(totalUnclaimedRewardRau, util.IotxDecimalNum),
		TotalAvailable: util.RauToString(availableRewardRau, util.IotxDecimalNum),
	}

	return message, err
}

func reward(client ioctl.Client, arg string) (rewardMessage, error) {
	var endpoint string
	var insecure bool

	address, err := client.Address(arg)
	if err != nil {
		return rewardMessage{}, errors.New("failed to get address")
	}
	apiClient, err := client.APIServiceClient(ioctl.APIServiceConfig{
		Endpoint: endpoint,
		Insecure: insecure,
	})
	if err != nil {
		return rewardMessage{}, err
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
			return rewardMessage{}, errors.New(sta.Message())
		}
		return rewardMessage{}, errors.New("failed to get version from server")
	}
	rewardRau, ok := new(big.Int).SetString(string(response.Data), 10)
	if !ok {
		return rewardMessage{}, errors.New("failed to convert string into big int")
	}
	message := rewardMessage{Address: address, Reward: util.RauToString(rewardRau, util.IotxDecimalNum)}
	return message, err
}
