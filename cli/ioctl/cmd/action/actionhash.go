// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
)

// actionHashCmd represents the account balance command
var actionHashCmd = &cobra.Command{
	Use:   "hash actionhash",
	Short: "Get action by hash",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(getActionByHash(args))
	},
}

// getActionByHash gets balance of an IoTeX Blockchain address
func getActionByHash(args []string) string {
	hash := args[0]
	endpoint := config.GetEndpoint()
	if endpoint == config.ErrEmptyEndpoint {
		log.L().Error(config.ErrEmptyEndpoint)
		return "use \"ioctl config set endpoint\" to config endpoint first."
	}
	conn, err := grpc.Dial(endpoint, grpc.WithInsecure())
	if err != nil {
		log.L().Error("failed to connect to server", zap.Error(err))
		return err.Error()
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()
	requestByHash := iotexapi.GetActionByHashRequest{}
	request := iotexapi.GetActionsRequest{
		Lookup: &iotexapi.GetActionsRequest_ByHash{
			ByHash: &iotexapi.GetActionByHashRequest{
				ActionHash: hash,
			},
		},
	}
	response, err := cli.GetActions(ctx, &request)
	if err != nil {
		log.L().Error("cannot get action from "+requestByHash.ActionHash, zap.Error(err))
		return err.Error()
	}
	return proto.MarshalTextString(response.Actions[0])
}
