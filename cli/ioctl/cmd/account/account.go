// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

// AccountCmd represents the account command
var AccountCmd = &cobra.Command{
	Use:   "account",
	Short: "Deal with accounts of IoTeX blockchain",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Print: " + strings.Join(args, " "))
	},
}

func init() {
	AccountCmd.AddCommand(accountBalanceCmd)
	AccountCmd.AddCommand(accountCreateCmd)
	AccountCmd.AddCommand(accountCreateAddCmd)
	AccountCmd.AddCommand(accountImportCmd)
}

// GetAccountMeta gets account metadata
func GetAccountMeta(addr string) (*iotextypes.AccountMeta, error) {
	endpoint := config.GetEndpoint()
	if endpoint == config.ErrEmptyEndpoint {
		log.L().Error(config.ErrEmptyEndpoint)
		return nil, errors.New("use \"ioctl config set endpoint\" to config endpoint first")
	}
	conn, err := grpc.Dial(endpoint, grpc.WithInsecure())
	if err != nil {
		log.L().Error("failed to connect to server", zap.Error(err))
		return nil, err
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()

	request := iotexapi.GetAccountRequest{}
	request.Address = addr
	response, err := cli.GetAccount(ctx, &request)
	if err != nil {
		return nil, err
	}
	accountMeta := response.AccountMeta
	return accountMeta, nil
}
