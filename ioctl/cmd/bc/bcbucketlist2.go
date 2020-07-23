// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

const (
	MethodByAddress   = "voter"
	MethodByCandidate = "candidate"
)

var (
	validMethods = []string{MethodByAddress, MethodByCandidate}
)

// Multi-language support
var (
	bcBLCmdShorts = map[config.Language]string{
		config.English: "Get bucket list with method and arg(s) on IoTeX blockchain",
		config.Chinese: "根据方法和参数在IoTeX区块链上读取投票列表",
	}
	bcBLUses = map[config.Language]string{
		config.English: "bl <method> [arguments]",
		config.Chinese: "bl <方法> [参数]",
	}
	bcBLCmdLongs = map[config.Language]string{
		config.English: "Read bucket list\nValid methods: [" +
			strings.Join(validMethods, ", ") + "]",
		config.Chinese: "根据方法和参数在IoTeX区块链上读取投票列表\n可用方法有：" +
			strings.Join(validMethods, "，"),
	}
)

// bcBLCmd represents the bc bl command
var bcBLCmd = &cobra.Command{
	Use:   config.TranslateInLang(bcBLUses, config.UILanguage),
	Short: config.TranslateInLang(bcBLCmdShorts, config.UILanguage),
	Long:  config.TranslateInLang(bcBLCmdLongs, config.UILanguage),
	Example: `  ioctl bc bl voter [ALIAS|ADDRESS]
  ioctl bc bl candidate [CANDIDATE_NAME]`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		cmd.SilenceUsage = true
		err = getBucketListWithMethod(args[0], args[1:]...)
		return output.PrintError(err)
	},
}

func getBucketListWithMethod(method string, args ...string) error {
	switch method {
	case MethodByAddress:
		arg := ""
		if len(args) > 0 {
			arg = args[0]
		}
		return getBucketList(arg)
	case MethodByCandidate:
		arg := ""
		if len(args) > 0 {
			arg = args[0]
		}
		return getBucketListByCand(arg)
	}
	return output.NewError(output.InputError, "unknown method", nil)
}

// getBucketListByCand get bucket list from chain
func getBucketListByCand(arg string) error {
	bl, err := getBucketListByCandidate(arg)
	if err != nil {
		return err
	}
	var bucketlist []*bucket
	for _, b := range bl.Buckets {
		bucket, err := newBucket(b)
		if err != nil {
			return err
		}
		bucketlist = append(bucketlist, bucket)
	}
	message := bucketlistMessage{
		Node:       config.ReadConfig.Endpoint,
		Bucketlist: bucketlist,
	}
	fmt.Println(message.String())
	return nil
}

func getBucketListByCandidate(candName string) (*iotextypes.VoteBucketList, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_BUCKETS_BY_CANDIDATE,
	}
	methodData, err := proto.Marshal(method)
	if err != nil {
		return nil, output.NewError(output.SerializationError, "failed to marshal read staking data method", err)
	}
	readStakingDataRequest := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsByCandidate{
			BucketsByCandidate: &iotexapi.ReadStakingDataRequest_VoteBucketsByCandidate{
				CandName: candName,
				Pagination: &iotexapi.PaginationParam{
					Offset: uint32(0),
					Limit:  uint32(1000),
				},
			},
		},
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
