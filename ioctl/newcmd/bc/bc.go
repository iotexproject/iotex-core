// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import (
	"context"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

// Multi-language support
var (
	bcCmdShorts = map[config.Language]string{
		config.English: "Deal with block chain of IoTeX blockchain",
		config.Chinese: "处理IoTeX区块链上的区块",
	}
	bcCmdUses = map[config.Language]string{
		config.English: "bc",
		config.Chinese: "bc",
	}
	flagEndpointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点",
	}
	flagInsecureUsages = map[config.Language]string{
		config.English: "insecure connection for once",
		config.Chinese: "一次不安全的连接",
	}
)

// NewBCCmd represents the bc(block chain) command
func NewBCCmd(client ioctl.Client) *cobra.Command {
	bcShorts, _ := client.SelectTranslation(bcCmdShorts)
	bcUses, _ := client.SelectTranslation(bcCmdUses)

	var endpoint string
	var insecure bool

	bc := &cobra.Command{
		Use:   bcUses,
		Short: bcShorts,
	}

	bc.AddCommand(NewBCInfoCmd(client))

	bc.PersistentFlags().StringVar(
		&endpoint,
		"endpoint",
		client.Config().Endpoint,
		"set endpoint for once")
	bc.PersistentFlags().BoolVar(&insecure,
		"insecure",
		!client.Config().SecureConnect,
		"insecure connection for once")

	return bc
}

// GetChainMeta gets block chain metadata
func GetChainMeta(client ioctl.Client) (*iotextypes.ChainMeta, error) {
	var endpoint string
	var insecure bool
	apiServiceClient, err := client.APIServiceClient(ioctl.APIServiceConfig{
		Endpoint: endpoint,
		Insecure: insecure,
	})
	if err != nil {
		return nil, err
	}

	chainMetaResponse, err := apiServiceClient.GetChainMeta(context.Background(), &iotexapi.GetChainMetaRequest{})

	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return nil, output.NewError(output.APIError, sta.Message(), nil)
		}
		return nil, output.NewError(output.NetworkError, "failed to invoke GetChainMeta api", err)
	}
	return chainMetaResponse.ChainMeta, nil
}

// GetEpochMeta gets blockchain epoch meta
func GetEpochMeta(epochNum uint64, client ioctl.Client) (*iotexapi.GetEpochMetaResponse, error) {

	var endpoint string
	var insecure bool
	apiServiceClient, err := client.APIServiceClient(ioctl.APIServiceConfig{
		Endpoint: endpoint,
		Insecure: insecure,
	})
	if err != nil {
		return nil, err
	}

	epochMetaresponse, err := apiServiceClient.GetEpochMeta(context.Background(), &iotexapi.GetEpochMetaRequest{})
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return nil, output.NewError(output.APIError, sta.Message(), nil)
		}
		return nil, output.NewError(output.NetworkError, "failed to invoke GetEpochMeta api", err)
	}
	return epochMetaresponse, nil
}

// GetKickoutList gets kickout list
func GetKickoutList(epochNum uint64, client ioctl.Client) (*iotexapi.ReadStateResponse, error) {
	var endpoint string
	var insecure bool
	apiServiceClient, err := client.APIServiceClient(ioctl.APIServiceConfig{
		Endpoint: endpoint,
		Insecure: insecure,
	})
	if err != nil {
		return nil, err
	}

	request := &iotexapi.ReadStateRequest{
		ProtocolID: []byte("poll"),
		MethodName: []byte("KickoutListByEpoch"),
		Arguments:  [][]byte{byteutil.Uint64ToBytes(epochNum)},
	}

	response, err := apiServiceClient.ReadState(context.Background(), request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok && sta.Code() == codes.NotFound {
			return nil, nil
		} else if ok {
			return nil, output.NewError(output.APIError, sta.Message(), nil)
		}
		return nil, output.NewError(output.NetworkError, "failed to invoke ReadState api", err)
	}
	return response, nil
}
