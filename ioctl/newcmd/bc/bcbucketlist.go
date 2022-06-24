// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

var (
	_validMethods = []string{
		"voter",
		"cand",
	}
)

// Multi-language support
var (
	_bcBucketListCmdShorts = map[config.Language]string{
		config.English: "Get bucket list with method and arg(s) on IoTeX blockchain",
		config.Chinese: "根据方法和参数在IoTeX区块链上读取投票列表",
	}
	_bcBucketListCmdUses = map[config.Language]string{
		config.English: "bucketlist <method> [arguments]",
		config.Chinese: "bucketlist <方法> [参数]",
	}
	_bcBucketListCmdLongs = map[config.Language]string{
		config.English: "Read bucket list\nValid methods: [" +
			strings.Join(_validMethods, ", ") + "]",
		config.Chinese: "根据方法和参数在IoTeX区块链上读取投票列表\n可用方法有：" +
			strings.Join(_validMethods, "，"),
	}
)

// NewBCBucketListCmd represents the bc bucketlist command
func NewBCBucketListCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_bcBucketListCmdUses)
	short, _ := client.SelectTranslation(_bcBucketListCmdShorts)
	long, _ := client.SelectTranslation(_bcBucketListCmdLongs)

	return &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		Args:  cobra.MinimumNArgs(2),
		Example: `ioctl bc bucketlist voter [VOTER_ADDRESS] [OFFSET] [LIMIT]
	ioctl bc bucketlist cand [CANDIDATE_NAME] [OFFSET] [LIMIT]`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error

			cmd.SilenceUsage = true
			offset, limit := uint64(0), uint64(1000)
			method, addr := args[0], args[1]
			s := args[2:]

			if len(s) > 0 {
				offset, err = strconv.ParseUint(s[0], 10, 64)
				if err != nil {
					return errors.Wrap(err, "invalid offset")
				}
			}
			if len(s) > 1 {
				limit, err = strconv.ParseUint(s[1], 10, 64)
				if err != nil {
					return errors.Wrap(err, "invalid limit")
				}
			}
			switch method {
			case "voter":
				return getBucketListByVoter(client, addr, uint32(offset), uint32(limit))
			case "cand":
				return getBucketListByCand(client, addr, uint32(offset), uint32(limit))
			default:
				return errors.New("unknown <method>")
			}
		},
	}
}

type bucketlistMessage struct {
	Node       string    `json:"node"`
	Bucketlist []*bucket `json:"bucketlist"`
}

func (m *bucketlistMessage) String() string {
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

// getBucketList get bucket list from chain by voter address
func getBucketListByVoter(client ioctl.Client, addr string, offset, limit uint32) error {
	address, err := util.GetAddress(addr)
	if err != nil {
		return errors.Wrap(err, "")
	}
	bl, err := getBucketListByVoterAddress(client, address, offset, limit)
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
		Node:       client.Config().Endpoint,
		Bucketlist: bucketlist,
	}
	fmt.Println(message.String())
	return nil
}

func getBucketListByVoterAddress(client ioctl.Client, addr string, offset, limit uint32) (*iotextypes.VoteBucketList, error) {
	readStakingdataRequest := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsByVoter{
			BucketsByVoter: &iotexapi.ReadStakingDataRequest_VoteBucketsByVoter{
				VoterAddress: addr,
				Pagination: &iotexapi.PaginationParam{
					Offset: offset,
					Limit:  limit,
				},
			},
		},
	}
	return GetBucketList(client, iotexapi.ReadStakingDataMethod_BUCKETS_BY_VOTER, readStakingdataRequest)
}

// getBucketListByCand get bucket list from chain by candidate name
func getBucketListByCand(client ioctl.Client, candName string, offset, limit uint32) error {
	bl, err := getBucketListByCandidateName(client, candName, offset, limit)
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
		Node:       client.Config().Endpoint,
		Bucketlist: bucketlist,
	}
	fmt.Println(message.String())
	return nil
}

func getBucketListByCandidateName(client ioctl.Client, candName string, offset, limit uint32) (*iotextypes.VoteBucketList, error) {
	readStakingDataRequest := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsByCandidate{
			BucketsByCandidate: &iotexapi.ReadStakingDataRequest_VoteBucketsByCandidate{
				CandName: candName,
				Pagination: &iotexapi.PaginationParam{
					Offset: offset,
					Limit:  limit,
				},
			},
		},
	}
	return GetBucketList(client, iotexapi.ReadStakingDataMethod_BUCKETS_BY_CANDIDATE, readStakingDataRequest)
}
