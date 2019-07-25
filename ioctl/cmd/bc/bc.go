// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import (
	"context"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
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
		return nil, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	request := iotexapi.GetChainMetaRequest{}
	ctx := context.Background()
	response, err := cli.GetChainMeta(ctx, &request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return nil, output.NewError(output.APIError, sta.Message(), nil)
		}
		return nil, output.NewError(output.NetworkError, "failed to invoke GetChainMeta api", err)
	}
	return response.ChainMeta, nil
}

// GetEpochMeta gets blockchain epoch meta
func GetEpochMeta(epochNum uint64) (*iotexapi.GetEpochMetaResponse, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	request := &iotexapi.GetEpochMetaRequest{EpochNumber: epochNum}
	ctx := context.Background()
	response, err := cli.GetEpochMeta(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return nil, output.NewError(output.APIError, sta.Message(), nil)
		}
		return nil, output.NewError(output.NetworkError, "failed to invoke GetEpochMeta api", err)
	}
	return response, nil
}
