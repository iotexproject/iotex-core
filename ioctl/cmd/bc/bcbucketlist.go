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
	bcBucketListCmdShorts = map[config.Language]string{
		config.English: "Get bucket list for given address on IoTeX blockchain",
		config.Chinese: "在IoTeX区块链上读取账户地址的投票列表",
	}
	bcBucketListUses = map[config.Language]string{
		config.English: "bucketlist [ALIAS|ADDRESS]",
		config.Chinese: "bucketlist [别名|地址]",
	}
)

// bcBucketListCmd represents the bc bucketlist command
var bcBucketListCmd = &cobra.Command{
	Use:   config.TranslateInLang(bcBucketListUses, config.UILanguage),
	Short: config.TranslateInLang(bcBucketListCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		arg := ""
		if len(args) == 1 {
			arg = args[0]
		}
		err := getBucketList(arg)
		return output.PrintError(err)
	},
}

type bucketlistMessage struct {
	Node       string    `json:"node"`
	Bucketlist []*bucket `json:"bucketlist"`
}

func (m *bucketlistMessage) String() string {
	if output.Format == "" {
		var lines []string
		if len(m.Bucketlist) == 0 {
			lines = append(lines, "Empty bucketlist with given address")
		} else {
			for _, bucket := range m.Bucketlist {
				lines = append(lines, bucket.String())
			}
		}
		return strings.Join(lines, "\n")
	}
	return output.FormatString(output.Result, m)
}

// getBucketList get bucket list from chain
func getBucketList(arg string) error {
	address, err := util.GetAddress(arg)
	if err != nil {
		return output.NewError(output.AddressError, "", err)
	}
	bl, err := getBucketListByAddress(address)
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

func getBucketListByAddress(addr string) (*iotextypes.VoteBucketList, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_BUCKETS_BY_VOTER,
	}
	methodData, err := proto.Marshal(method)
	if err != nil {
		return nil, output.NewError(output.SerializationError, "failed to marshal read staking data method", err)
	}
	readStakingdataRequest := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsByVoter{
			BucketsByVoter: &iotexapi.ReadStakingDataRequest_VoteBucketsByVoter{
				VoterAddress: addr,
				Pagination: &iotexapi.PaginationParam{
					Offset: uint32(0),
					Limit:  uint32(1000),
				},
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
	bucketlist := iotextypes.VoteBucketList{}
	if err := proto.Unmarshal(response.Data, &bucketlist); err != nil {
		return nil, output.NewError(output.SerializationError, "failed to unmarshal response", err)
	}
	return &bucketlist, nil
}
