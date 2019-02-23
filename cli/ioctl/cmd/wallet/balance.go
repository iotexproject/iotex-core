// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package wallet

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
	accountAddress string
)

// balanceCmd represents the wallet balance command
var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Get balance of an account",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(balance(args))
	},
}

func init() {
	balanceCmd.Flags().StringVarP(&accountAddress, "address", "a", "", "address of queried account")
	WalletCmd.AddCommand(balanceCmd)
}

// getCurrentBlockHeigh get current height of block chain from server
func balance(args []string) string {
	request := pb.GetAccountRequest{Address: accountAddress}
	if accountAddress == "" {
		request.Address = "ioaddress"
	}
	conn, err := grpc.Dial("127.0.0.1:8080", grpc.WithInsecure())
	if err != nil {
		log.L().Error("failed to connect to server", zap.Error(err))
		return err.Error()
	}
	defer conn.Close()

	cli := pb.NewAPIServiceClient(conn)
	ctx := context.Background()
	response, err := cli.GetAccount(ctx, &request)
	if err != nil {
		log.L().Error("cannot get account", zap.Error(err))
		return err.Error()
	}
	accountMeta := response.AccountMeta
	return fmt.Sprintf("%s", accountMeta.Balance)
}
