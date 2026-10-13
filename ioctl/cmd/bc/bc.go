// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package bc

import (
	"context"
	"strconv"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// Multi-language support
var (
	_bcCmdShorts = map[config.Language]string{
		config.English: "Deal with block chain of IoTeX blockchain",
		config.Chinese: "处理IoTeX区块链上的区块",
	}
	_flagEndpointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点",
	}
	_flagInsecureUsages = map[config.Language]string{
		config.English: "insecure connection for once",
		config.Chinese: "一次不安全的连接",
	}
)

// BCCmd represents the bc(block chain) command
var BCCmd = &cobra.Command{
	Use:   "bc",
	Short: config.TranslateInLang(_bcCmdShorts, config.UILanguage),
}

func init() {
	BCCmd.AddCommand(_bcBlockCmd)
	BCCmd.AddCommand(_bcInfoCmd)
	BCCmd.AddCommand(_bcBucketListCmd)
	BCCmd.AddCommand(_bcBucketCmd)
	BCCmd.AddCommand(_bcDelegateCmd)
	BCCmd.AddCommand(_bcVersionCmd)
	BCCmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, config.TranslateInLang(_flagEndpointUsages, config.UILanguage))
	BCCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		config.TranslateInLang(_flagInsecureUsages, config.UILanguage))
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

	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

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

	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

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

// EpochReadHeight returns the height to read the poll protocol state of an epoch:
// empty (i.e. latest state) for the current epoch, which a non-archive node can
// serve, and the epoch start height for a past epoch, which needs an archive node
func EpochReadHeight(epochNum uint64, epochStartHeight uint64) (string, error) {
	chainMeta, err := GetChainMeta()
	if err != nil {
		return "", err
	}
	if chainMeta.GetEpoch().GetNum() == epochNum {
		return "", nil
	}
	return strconv.FormatUint(epochStartHeight, 10), nil
}

// GetProbationList gets probation list
func GetProbationList(epochNum uint64, epochStartHeight uint64) (*iotexapi.ReadStateResponse, error) {
	height, err := EpochReadHeight(epochNum, epochStartHeight)
	if err != nil {
		return nil, err
	}
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)

	request := &iotexapi.ReadStateRequest{
		ProtocolID: []byte("poll"),
		MethodName: []byte("ProbationListByEpoch"),
		Arguments:  [][]byte{[]byte(strconv.FormatUint(epochNum, 10))},
		Height:     height,
	}
	ctx := context.Background()

	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	response, err := cli.ReadState(ctx, request)
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

// GetBucketList get bucket list
func GetBucketList(
	methodName iotexapi.ReadStakingDataMethod_Name,
	readStakingDataRequest *iotexapi.ReadStakingDataRequest,
) (*iotextypes.VoteBucketList, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	method := &iotexapi.ReadStakingDataMethod{Method: methodName}
	methodData, err := proto.Marshal(method)
	if err != nil {
		return nil, output.NewError(output.SerializationError, "failed to marshal read staking data method", err)
	}
	requestData, err := proto.Marshal(readStakingDataRequest)
	if err != nil {
		return nil, output.NewError(output.SerializationError, "failed to marshal read staking data request", err)
	}

	request := &iotexapi.ReadStateRequest{
		ProtocolID: []byte("staking"),
		MethodName: methodData,
		Arguments:  [][]byte{requestData},
	}

	ctx := context.Background()
	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	response, err := cli.ReadState(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return nil, output.NewError(output.APIError, sta.Message(), nil)
		}
		return nil, output.NewError(output.NetworkError, "failed to invoke ReadState api", err)
	}
	bucketlist := iotextypes.VoteBucketList{}
	if err := proto.Unmarshal(response.Data, &bucketlist); err != nil {
		return nil, output.NewError(output.SerializationError, "failed to unmarshal response", err)
	}
	return &bucketlist, nil
}
