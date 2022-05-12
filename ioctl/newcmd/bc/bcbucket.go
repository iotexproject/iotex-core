// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

const (
	_bcBucketOptMax   = "max"
	_bcBucketOptCount = "count"
)

// Multi-language support
var (
	_bcBucketCmdShorts = map[config.Language]string{
		config.English: "Get bucket for given index on IoTeX blockchain",
		config.Chinese: "在IoTeX区块链上根据索引读取投票",
	}
	_bcBucketUses = map[config.Language]string{
		config.English: "bucket [OPTION|BUCKET_INDEX]",
		config.Chinese: "bucket [选项|票索引]",
	}
)

type bucket struct {
	Index            uint64 `json:"index"`
	Owner            string `json:"owner"`
	Candidate        string `json:"candidate"`
	StakedAmount     string `json:"stakedAmount"`
	StakedDuration   uint32 `json:"stakedDuration"`
	AutoStake        bool   `json:"autoStake"`
	CreateTime       string `json:"createTime"`
	StakeStartTime   string `json:"stakeStartTime"`
	UnstakeStartTime string `json:"unstakeStartTime"`
}

type bucketMessage struct {
	Node   string  `json:"node"`
	Bucket *bucket `json:"bucket"`
}

// NewBCBucketCmd represents the bc Bucket command
func NewBCBucketCmd(c ioctl.Client) *cobra.Command {
	bcBucketUses, _ := c.SelectTranslation(_bcBlockCmdUses)
	bcBucketCmdShorts, _ := c.SelectTranslation(_bcBlockCmdShorts)

	cmd := &cobra.Command{
		Use:   bcBucketUses,
		Short: bcBucketCmdShorts,
		Args:  cobra.ExactArgs(1),
		Example: `ioctl bc bucket [BUCKET_INDEX], to read bucket information by bucket index
	ioctl bc bucket max, to query the max bucket index
	ioctl bc bucket count, to query total number of active buckets
	`,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			cmd.SilenceUsage = true
			switch args[0] {
			case _bcBucketOptMax:
				count, err := getBucketsCount(c)
				if err != nil {
					return err
				}
				cmd.Println(count.GetTotal())
				return nil
			case _bcBucketOptCount:
				count, err := getBucketsCount(c)
				if err != nil {
					return err
				}
				cmd.Println(count.GetActive())
			default:
				bucketindex, err := strconv.ParseUint(args[0], 10, 64)
				if err != nil {
					return err
				}
				bucketpb, err := getBucketByIndex(c, bucketindex)
				if err != nil {
					return err
				}
				if bucketpb == nil {
					return errors.New("The bucket has been withdrawn")
				}
				bucket, err := newBucket(bucketpb)
				if err != nil {
					return err
				}
				message := bucketMessage{
					Node:   c.Config().Endpoint,
					Bucket: bucket,
				}
				cmd.Println(message.Bucket.String())
			}
			cmd.Println(err)
			return nil
		},
	}
	return cmd
}

func newBucket(bucketpb *iotextypes.VoteBucket) (*bucket, error) {
	amount, ok := new(big.Int).SetString(bucketpb.StakedAmount, 10)
	if !ok {
		return nil, errors.New("failed to convert amount into big int")
	}
	unstakeStartTimeFormat := "none"
	if err := bucketpb.UnstakeStartTime.CheckValid(); err != nil {
		return nil, err
	}
	unstakeTime := bucketpb.UnstakeStartTime.AsTime()
	if unstakeTime != time.Unix(0, 0).UTC() {
		unstakeStartTimeFormat = unstakeTime.Format(time.RFC3339Nano)
	}
	return &bucket{
		Index:            bucketpb.Index,
		Owner:            bucketpb.Owner,
		Candidate:        bucketpb.CandidateAddress,
		StakedAmount:     util.RauToString(amount, util.IotxDecimalNum),
		StakedDuration:   bucketpb.StakedDuration,
		AutoStake:        bucketpb.AutoStake,
		CreateTime:       bucketpb.CreateTime.AsTime().Format(time.RFC3339Nano),
		StakeStartTime:   bucketpb.StakeStartTime.AsTime().Format(time.RFC3339Nano),
		UnstakeStartTime: unstakeStartTimeFormat,
	}, nil
}

func (b *bucket) String() string {
	var lines []string
	lines = append(lines, "{")
	lines = append(lines, fmt.Sprintf("	index: %d", b.Index))
	lines = append(lines, fmt.Sprintf("	owner: %s", b.Owner))
	lines = append(lines, fmt.Sprintf("	candidate: %s", b.Candidate))
	lines = append(lines, fmt.Sprintf("	stakedAmount: %s IOTX", b.StakedAmount))
	lines = append(lines, fmt.Sprintf("	stakedDuration: %d days", b.StakedDuration))
	lines = append(lines, fmt.Sprintf("	autoStake: %v", b.AutoStake))
	lines = append(lines, fmt.Sprintf("	createTime: %s", b.CreateTime))
	lines = append(lines, fmt.Sprintf("	stakeStartTime: %s", b.StakeStartTime))
	lines = append(lines, fmt.Sprintf("	unstakeStartTime: %s", b.UnstakeStartTime))
	lines = append(lines, "}")
	return strings.Join(lines, "\n")
}

func getBucketByIndex(c ioctl.Client, index uint64) (*iotextypes.VoteBucket, error) {
	conn, err := util.ConnectToEndpoint(c.Config().SecureConnect && !config.Insecure)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to endpoint")
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_BUCKETS_BY_INDEXES,
	}
	methodData, err := proto.Marshal(method)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal read staking data method")
	}
	readStakingdataRequest := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsByIndexes{
			BucketsByIndexes: &iotexapi.ReadStakingDataRequest_VoteBucketsByIndexes{
				Index: []uint64{index},
			},
		},
	}
	requestData, err := proto.Marshal(readStakingdataRequest)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal read staking data request")
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
			return nil, errors.New(sta.Message())
		}
		return nil, errors.Wrap(err, "failed to invoke ReadState api")
	}
	buckets := iotextypes.VoteBucketList{}
	if err := proto.Unmarshal(response.Data, &buckets); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}
	if len(buckets.GetBuckets()) == 0 {
		return nil, errors.New("zero len response")
	}
	return buckets.GetBuckets()[0], nil
}

func getBucketsCount(c ioctl.Client) (count *iotextypes.BucketsCount, err error) {
	conn, err := util.ConnectToEndpoint(c.Config().SecureConnect && !config.Insecure)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to endpoint")
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_BUCKETS_COUNT,
	}
	methodData, err := proto.Marshal(method)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal read staking data method")
	}
	readStakingdataRequest := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsCount_{
			BucketsCount: &iotexapi.ReadStakingDataRequest_BucketsCount{},
		},
	}
	requestData, err := proto.Marshal(readStakingdataRequest)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal read staking data request")
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
			return nil, errors.New(sta.Message())
		}
		return nil, errors.Wrap(err, "failed to invoke ReadState api")
	}
	count = &iotextypes.BucketsCount{}
	if err := proto.Unmarshal(response.Data, count); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}
	return count, nil
}
