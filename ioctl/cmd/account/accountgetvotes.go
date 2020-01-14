// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"strconv"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	getVotesCmdShorts = map[config.Language]string{
		config.English: "Get votes of this votee",
		config.Chinese: "从投票人处获得投票",
	}
	getVotesCmdUses = map[config.Language]string{
		config.English: "getVotes VOTEE HEIGHT OFFSET LIMIT",
		config.Chinese: "getVotes 投票人 高度 偏移 限制",
	}
)

// accountGetVotesCmd represents the account get votes command
var accountGetVotesCmd = &cobra.Command{
	Use:   config.TranslateInLang(getVotesCmdUses, config.UILanguage),
	Short: config.TranslateInLang(getVotesCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := getVotes(args)
		return output.PrintError(err)
	},
}

func getVotes(args []string) error {
	offset, err := strconv.ParseUint(args[2], 10, 32)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to get offset", err)
	}
	limit, err := strconv.ParseUint(args[3], 10, 32)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to get limit", err)
	}
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return output.NewError(output.NetworkError, "failed to connect endpoint", err)
	}
	defer conn.Close()

	res, err := iotexapi.NewAPIServiceClient(conn).GetVotes(
		context.Background(),
		&iotexapi.GetVotesRequest{
			Votee:  args[0],
			Height: args[1],
			Offset: uint32(offset),
			Limit:  uint32(limit),
		},
	)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return output.NewError(output.APIError, sta.Message(), nil)
		}
		return output.NewError(output.NetworkError, "failed to invoke GetVotes api", err)
	}
	output.PrintResult(res.String())
	return nil
}
