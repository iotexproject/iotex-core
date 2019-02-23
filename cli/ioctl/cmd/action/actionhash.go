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

	"github.com/iotexproject/iotex-core/pkg/log"
	pb "github.com/iotexproject/iotex-core/protogen/iotexapi"
)

// TODO: use config later
var hostAddress string

// actionHashCmd represents the account balance command
var actionHashCmd = &cobra.Command{
	Use:   "hash",
	Short: "Get action by hash",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(getActionByHash(args))
	},
}

func init() {
	actionHashCmd.Flags().StringVarP(&hostAddress, "host", "s", "127.0.0.1:8080", "host address of node")
	ActionCmd.AddCommand(actionHashCmd)
}

// getActionByHash gets balance of an IoTex Blockchian address
func getActionByHash(args []string) string {
	conn, err := grpc.Dial(hostAddress, grpc.WithInsecure())
	if err != nil {
		log.L().Error("failed to connect to server", zap.Error(err))
		return err.Error()
	}
	defer conn.Close()
	cli := pb.NewAPIServiceClient(conn)
	ctx := context.Background()
	requestByHash := pb.GetActionByHashRequest{}
	request := pb.GetActionsRequest{}
	var res string
	for _, hash := range args {
		requestByHash.ActionHash = hash
		request.Lookup = &pb.GetActionsRequest_ByHash{ByHash: &requestByHash}
		response, err := cli.GetActions(ctx, &request)
		if err != nil {
			log.L().Error("cannot get action from "+requestByHash.ActionHash, zap.Error(err))
			return err.Error()
		}
		actions := response.Actions
		res += proto.MarshalTextString(actions[0])
	}
	return res
}
