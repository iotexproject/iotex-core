// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import (
	"context"
	"fmt"
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	bcBucketCmdShorts = map[config.Language]string{
		config.English: "Get bucket from block chain with given bucket index",
		config.Chinese: "",
	}
	bcBucketUses = map[config.Language]string{
		config.English: "bucket [BUCKET_INDEX]",
		config.Chinese: "",
	}
)

// bcBucketCmd represents the bc Bucket command
var bcBucketCmd = &cobra.Command{
	Use:   config.TranslateInLang(bcBucketUses, config.UILanguage),
	Short: config.TranslateInLang(bcBucketCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := getBucket(args[0])
		return output.PrintError(err)
	},
}

type bucketMessage struct {
	Node   string                 `json:"node"`
	Bucket *iotextypes.VoteBucket `json:"bucket"`
}

func (m *bucketMessage) String() string {
	if output.Format == "" {
		message := fmt.Sprintf("Blockchain Node: %s\n%s", m.Node, output.JSONString(m.Bucket))
		return message
	}
	return output.FormatString(output.Result, m)
}

// getBucket get bucket from chain
func getBucket(arg string) error {
	bucketindex, err := strconv.ParseUint(arg, 10, 64)
	bucket, err := getBucketByIndex(bucketindex)
	if err != nil {
		return err
	}
	message := bucketMessage{Node: config.ReadConfig.Endpoint, Bucket: bucket}
	fmt.Println(message.String())
	return nil
}

func getBucketByIndex(index uint64) (*iotextypes.VoteBucket, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_BUCKET_BY_INDEX,
	}
	methodData, err := proto.Marshal(method)
	if err != nil {
		return nil, output.NewError(output.SerializationError, "failed to marshal read staking data method", err)
	}
	readStakingdataRequest := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketByIndex{
			BucketByIndex: &iotexapi.ReadStakingDataRequest_VoteBucketByIndex{
				Index: index,
			},
		},
	}
	requestData, err := proto.Marshal(readStakingdataRequest)
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
	Bucket := iotextypes.VoteBucket{}
	if err := proto.Unmarshal(response.Data, &Bucket); err != nil {
		return nil, output.NewError(output.SerializationError, "failed to unmarshal response", err)
	}
	return &Bucket, nil
}
