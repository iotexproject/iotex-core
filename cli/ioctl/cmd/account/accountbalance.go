// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	grpc "google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/pkg/log"
	pb "github.com/iotexproject/iotex-core/protogen/iotexapi"
)

// TODO: use wallet config later
var configAddress = "ioaddress"

// balanceCmd represents the wallet balance command
var accountBalanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Get balance of an account",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(balance(args))
	},
}

func init() {
	AccountCmd.AddCommand(accountBalanceCmd)
}

// getCurrentBlockHeigh get current height of block chain from server
func balance(args []string) string {
	conn, err := grpc.Dial("127.0.0.1:8080", grpc.WithInsecure())
	if err != nil {
		log.L().Error("failed to connect to server", zap.Error(err))
		return err.Error()
	}
	defer conn.Close()
	cli := pb.NewAPIServiceClient(conn)
	ctx := context.Background()

	request := pb.GetAccountRequest{}
	res := ""
	if len(args) == 0 {
		request.Address = configAddress
		response, err := cli.GetAccount(ctx, &request)
		if err != nil {
			log.L().Error("cannot get account from "+request.Address, zap.Error(err))
			return err.Error()
		}
		accountMeta := response.AccountMeta
		res = res + fmt.Sprintf("%s: %s\n", request.Address, accountMeta.Balance)

	} else {
		for _, addr := range args {
			request.Address = addr
			response, err := cli.GetAccount(ctx, &request)
			if err != nil {
				log.L().Error("cannot get account"+request.Address, zap.Error(err))
				return err.Error()
			}
			accountMeta := response.AccountMeta
			res = res + fmt.Sprintf("%s: %s\n", request.Address, accountMeta.Balance)
		}
	}
	return res
}
