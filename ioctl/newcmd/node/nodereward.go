// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package node

import (
	"context"
	"math/big"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

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
				totalUnclaimed, totalAvailable, totalBalance, err := rewardPool(client)
				if err != nil {
					return err
				}
				cmd.Printf("Total Unclaimed:\t %s IOTX\nTotal Available:\t %s IOTX\nTotal Balance:\t\t %s IOTX\n",
					totalUnclaimed, totalAvailable, totalBalance)
			case "unclaimed":
				if len(args) != 2 {
					return errors.New("wrong number of arg(s) for ioctl node reward unclaimed [ALIAS|DELEGATE_ADDRESS] command. \nRun 'ioctl node reward --help' for usage")
				}
				address, reward, err := reward(client, args[1])
				if err != nil {
					return err
				}
				cmd.Printf("%s: %s IOTX\n", address, reward)
			default:
				return errors.New("unknown command. \nRun 'ioctl node reward --help' for usage")
			}
			return nil
		},
	}
}

func rewardPool(client ioctl.Client) (string, string, string, error) {
	apiClient, err := client.APIServiceClient()
	if err != nil {
		return "", "", "", err
	}
	ctx := context.Background()
	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}
	response, err := apiClient.ReadState(
		ctx,
		&iotexapi.ReadStateRequest{
			ProtocolID: []byte("rewarding"),
			MethodName: []byte("AvailableBalance"),
		},
	)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return "", "", "", errors.New(sta.Message())
		}
		return "", "", "", errors.Wrap(err, "failed to invoke ReadState api")
	}

	availableRewardRau, ok := new(big.Int).SetString(string(response.Data), 10)
	if !ok {
		return "", "", "", errors.New("failed to convert string into big int")
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
			return "", "", "", errors.New(sta.Message())
		}
		return "", "", "", errors.Wrap(err, "failed to invoke ReadState api")
	}
	totalRewardRau, ok := new(big.Int).SetString(string(response.Data), 10)
	if !ok {
		return "", "", "", errors.New("failed to convert string into big int")
	}

	totalUnclaimedRewardRau := big.NewInt(0)
	totalUnclaimedRewardRau.Sub(totalRewardRau, availableRewardRau)

	return util.RauToString(totalUnclaimedRewardRau, util.IotxDecimalNum),
		util.RauToString(availableRewardRau, util.IotxDecimalNum),
		util.RauToString(totalRewardRau, util.IotxDecimalNum),
		err
}

func reward(client ioctl.Client, arg string) (string, string, error) {
	address, err := client.Address(arg)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get address")
	}
	apiClient, err := client.APIServiceClient()
	if err != nil {
		return "", "", errors.Wrap(err, "failed to connect to endpoint")
	}
	ctx := context.Background()
	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}
	response, err := apiClient.ReadState(
		ctx,
		&iotexapi.ReadStateRequest{
			ProtocolID: []byte("rewarding"),
			MethodName: []byte("UnclaimedBalance"),
			Arguments:  [][]byte{[]byte(address)},
		},
	)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return "", "", errors.New(sta.Message())
		}
		return "", "", errors.Wrap(err, "failed to get version from server")
	}
	rewardRau, ok := new(big.Int).SetString(string(response.Data), 10)
	if !ok {
		return "", "", errors.New("failed to convert string into big int")
	}
	return address, util.RauToString(rewardRau, util.IotxDecimalNum), err
}
