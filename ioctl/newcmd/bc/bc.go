// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import (
	"context"
	"strconv"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	_bcCmdShorts = map[config.Language]string{
		config.English: "Deal with blockchain of IoTeX blockchain",
		config.Chinese: "处理IoTeX区块链上的区块",
	}
	_bcCmdUses = map[config.Language]string{
		config.English: "bc",
		config.Chinese: "bc",
	}
)

// NewBCCmd represents the bc(block chain) command
func NewBCCmd(client ioctl.Client) *cobra.Command {
	bcShorts, _ := client.SelectTranslation(_bcCmdShorts)
	bcUses, _ := client.SelectTranslation(_bcCmdUses)

	bc := &cobra.Command{
		Use:   bcUses,
		Short: bcShorts,
	}
	bc.AddCommand(NewBCInfoCmd(client))
	bc.AddCommand(NewBCBlockCmd(client))

	client.SetEndpointWithFlag(bc.PersistentFlags().StringVar)
	client.SetInsecureWithFlag(bc.PersistentFlags().BoolVar)
	return bc
}

// GetChainMeta gets blockchain metadata
func GetChainMeta(client ioctl.Client) (*iotextypes.ChainMeta, error) {
	apiServiceClient, err := client.APIServiceClient()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(context.Background())
	}

	chainMetaResponse, err := apiServiceClient.GetChainMeta(ctx, &iotexapi.GetChainMetaRequest{})

	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return nil, errors.Wrap(nil, sta.Message())
		}
		return nil, errors.Wrap(err, "failed to invoke GetChainMeta api")
	}
	return chainMetaResponse.ChainMeta, nil
}

// GetEpochMeta gets blockchain epoch meta
func GetEpochMeta(epochNum uint64, client ioctl.Client) (*iotexapi.GetEpochMetaResponse, error) {
	apiServiceClient, err := client.APIServiceClient()
	if err != nil {
		return nil, err
	}

	epochMetaresponse, err := apiServiceClient.GetEpochMeta(context.Background(), &iotexapi.GetEpochMetaRequest{})
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return nil, errors.Wrap(nil, sta.Message())
		}
		return nil, errors.Wrap(err, "failed to invoke GetEpochMeta api")
	}
	return epochMetaresponse, nil
}

// GetProbationList gets probation list
func GetProbationList(epochNum uint64, client ioctl.Client) (*iotexapi.ReadStateResponse, error) {
	apiServiceClient, err := client.APIServiceClient()
	if err != nil {
		return nil, err
	}

	request := &iotexapi.ReadStateRequest{
		ProtocolID: []byte("poll"),
		MethodName: []byte("ProbationListByEpoch"),
		Arguments:  [][]byte{[]byte(strconv.FormatUint(epochNum, 10))},
	}

	response, err := apiServiceClient.ReadState(context.Background(), request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok && sta.Code() == codes.NotFound {
			return nil, nil
		} else if ok {
			return nil, errors.Wrap(nil, sta.Message())
		}
		return nil, errors.Wrap(err, "failed to invoke ReadState api")
	}
	return response, nil
}
