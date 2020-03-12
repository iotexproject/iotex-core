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

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	rewardCmdUses = map[config.Language]string{
		config.English: "reward [ALIAS|DELEGATE_ADDRESS]",
		config.Chinese: "reward [别名|委托地址]",
	}
	rewardCmdShorts = map[config.Language]string{
		config.English: "Query rewards",
		config.Chinese: "查询奖励",
	}
	rewardPoolLong = map[config.Language]string{
		config.English: "ioctl node reward returns unclaimed and available Rewards in fund pool. TotalUnclaimedRewards is the amount of all delegates that have been issued but are not claimed; TotalAvailableRewards is the amount of balance that has not been issued to anyone.\n\nioctl node [ALIAS|DELEGATE_ADDRESS] returns unclaimed rewards of a specific delegate.",
		config.Chinese: "ioctl node reward 返回奖金池中的未支取奖励和可获取的奖励. TotalUnclaimedRewards是所有代表已被发放但未支取的奖励的总和; TotalAvailableRewards 是奖金池中未被发放的奖励的总和.\n\nioctl node [ALIAS|DELEGATE_ADDRESS] 返回特定代表的已被发放但未支取的奖励.",
	}
)

// nodeRewardCmd represents the node reward command
var nodeRewardCmd = &cobra.Command{
	Use:   config.TranslateInLang(rewardCmdUses, config.UILanguage),
	Short: config.TranslateInLang(rewardCmdShorts, config.UILanguage),
	Args:  cobra.MaximumNArgs(1),
	Long:  config.TranslateInLang(rewardPoolLong, config.UILanguage),
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

// TotalAvailableRewards == Rewards in the pool that has not been issued to anyone
// TotalUnclaimedRewards == Rewards in the pool that has been issued to a delegate but are not claimed yet
type rewardPoolMessage struct {
	//rewardPoolMessageDescription string `json:rewardPoolMessageDescription`
	TotalUnclaimedRewards string `json:"TotalUnclaimedRewards"`
	TotalAvailableRewards string `json:"TotalAvailableRewards"`
}

func (m *rewardPoolMessage) String() string {
	if output.Format == "" {
		message := fmt.Sprintf("TotalUnclaimedRewards: %s IOTX   TotalAvailableRewards: %s IOTX",
			m.TotalUnclaimedRewards, m.TotalAvailableRewards)
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

	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}
	// AvailableBalance == Rewards in the pool that has been issued and unclaimed
	request := &iotexapi.ReadStateRequest{
		ProtocolID: []byte("rewarding"),
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
	// TotalBalance == Rewards in the pool that has not been issued to anyone
	request = &iotexapi.ReadStateRequest{
		ProtocolID: []byte("rewarding"),
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
		TotalUnclaimedRewards: util.RauToString(availableRewardRau, util.IotxDecimalNum),
		TotalAvailableRewards: util.RauToString(totalRewardRau, util.IotxDecimalNum),
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

	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	request := &iotexapi.ReadStateRequest{
		ProtocolID: []byte("rewarding"),
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
