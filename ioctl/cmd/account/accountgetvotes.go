// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"strconv"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// accountGetVotesCmd represents the account get votes command
var accountGetVotesCmd = &cobra.Command{
	Use:   "getVotes VOTEE HEIGHT OFFSET LIMIT",
	Short: "Get votes of this votee",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := getVotes(args)
		return err
	},
}

func getVotes(args []string) error {
	offset, err := strconv.ParseUint(args[2], 10, 32)
	if err != nil {
		return output.PrintError(output.InputError, err.Error())
	}
	limit, err := strconv.ParseUint(args[3], 10, 32)
	if err != nil {
		return output.PrintError(output.InputError, err.Error())
	}
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return output.PrintError(output.NetworkError, err.Error())
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
		return err
	}
	output.PrintResult(res.String())
	return nil
}
