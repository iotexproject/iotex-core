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
	grpc "google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/pkg/log"
	pb "github.com/iotexproject/iotex-core/protogen/iotexapi"
)

var (
	address string
)

// heightCmd represents the account height command
var heightCmd = &cobra.Command{
	Use:   "height",
	Short: "Get current block height",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(getCurrentBlockHeigh(args))
	},
}

func init() {
	heightCmd.Flags().StringVarP(&address, "host", "s", "127.0.0.1:8080", "host of api server")
	BCCmd.AddCommand(heightCmd)
}

// getCurrentBlockHeigh get current height of block chain from server
func getCurrentBlockHeigh(args []string) string {
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.L().Error("failed to connect to server", zap.Error(err))
		return err.Error()
	}
	defer conn.Close()

	cli := pb.NewAPIServiceClient(conn)
	request := pb.GetChainMetaRequest{}
	ctx := context.Background()
	response, err := cli.GetChainMeta(ctx, &request)
	if err != nil {
		log.L().Error("server error", zap.Error(err))
		return err.Error()
	}
	return fmt.Sprintf("%d", response.ChainMeta.Height)
}
