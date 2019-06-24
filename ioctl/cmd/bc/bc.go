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
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// BCCmd represents the bc(block chain) command
var BCCmd = &cobra.Command{
	Use:   "bc",
	Short: "Deal with block chain of IoTeX blockchain",
}

func init() {
	BCCmd.AddCommand(bcBlockCmd)
	BCCmd.AddCommand(bcInfoCmd)
	BCCmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, "set endpoint for once")
	BCCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		"insecure connection for once")
}

// GetChainMeta gets block chain metadata
func GetChainMeta() (*iotextypes.ChainMeta, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	request := iotexapi.GetChainMetaRequest{}
	ctx := context.Background()
	response, err := cli.GetChainMeta(ctx, &request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return nil, fmt.Errorf(sta.Message())
		}
		return nil, err
	}
	return response.ChainMeta, nil
}
