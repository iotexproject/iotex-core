// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
)

var (
	alias    string
	bytecode []byte
	gasLimit uint64
	gasPrice int64
)

// ActionCmd represents the account command
var ActionCmd = &cobra.Command{
	Use:   "action",
	Short: "Deal with actions of IoTeX blockchain",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Print: " + strings.Join(args, " "))
	},
}

func init() {
	ActionCmd.AddCommand(actionHashCmd)
	ActionCmd.AddCommand(actionTransferCmd)
	ActionCmd.AddCommand(actionDeployCmd)
	ActionCmd.AddCommand(actionInvokeCmd)
}

func sendAction(request *iotexapi.SendActionRequest) string {
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
	_, err = cli.SendAction(ctx, request)
	if err != nil {
		log.L().Error("server error", zap.Error(err))
		return err.Error()
	}
	return "Action has been sent."
}
