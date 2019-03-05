// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
)

// heightCmd represents the account height command
var heightCmd = &cobra.Command{
	Use:   "height",
	Short: "Get current block height",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(getCurrentBlockHeigh(args))
	},
}

// getCurrentBlockHeigh get current height of block chain from server
func getCurrentBlockHeigh(args []string) string {
	endpoint := config.Get("endpoint")
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
	request := iotexapi.GetChainMetaRequest{}
	ctx := context.Background()
	response, err := cli.GetChainMeta(ctx, &request)
	if err != nil {
		log.L().Error("server error", zap.Error(err))
		return err.Error()
	}
	return fmt.Sprintf("%d", response.ChainMeta.Height)
}
