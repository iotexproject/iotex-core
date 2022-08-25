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
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

const (
	_bucketlistMethodByVoter     = "voter"
	_bucketlistMethodByCandidate = "cand"
)

var (
	_validMethods = []string{
		_bucketlistMethodByVoter,
		_bucketlistMethodByCandidate,
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

// _bcBucketListCmd represents the bc bucketlist command
var _bcBucketListCmd = &cobra.Command{
	Use:   config.TranslateInLang(_bcBucketListCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_bcBucketListCmdShorts, config.UILanguage),
	Long:  config.TranslateInLang(_bcBucketListCmdLongs, config.UILanguage),
	Args:  cobra.MinimumNArgs(2),
	Example: `ioctl bc bucketlist voter [VOTER_ADDRESS] [OFFSET] [LIMIT]
ioctl bc bucketlist cand [CANDIDATE_NAME] [OFFSET] [LIMIT]`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := getBucketList(args[0], args[1], args[2:]...)
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
func getBucketList(method, addr string, args ...string) (err error) {
	offset, limit := uint32(0), uint32(1000)
	if len(args) > 0 {
		val, err := strconv.ParseUint(args[0], 10, 32)
		if err != nil {
			return output.NewError(output.ValidationError, "invalid offset", err)
		}
		offset = uint32(val)
	}
	if len(args) > 1 {
		val, err := strconv.ParseUint(args[1], 10, 32)
		if err != nil {
			return output.NewError(output.ValidationError, "invalid limit", err)
		}
		limit = uint32(val)
	}
	switch method {
	case _bucketlistMethodByVoter:
		return getBucketListByVoter(addr, offset, limit)
	case _bucketlistMethodByCandidate:
		return getBucketListByCand(addr, offset, limit)
	}
	return output.NewError(output.InputError, "unknown <method>", nil)
}

// getBucketList get bucket list from chain by voter address
func getBucketListByVoter(addr string, offset, limit uint32) error {
	address, err := util.GetAddress(addr)
	if err != nil {
		return output.NewError(output.AddressError, "", err)
	}
	bl, err := getBucketListByVoterAddress(address, offset, limit)
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

func getBucketListByVoterAddress(addr string, offset, limit uint32) (*iotextypes.VoteBucketList, error) {
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
	return GetBucketList(iotexapi.ReadStakingDataMethod_BUCKETS_BY_VOTER, readStakingdataRequest)
}

// getBucketListByCand get bucket list from chain by candidate name
func getBucketListByCand(candName string, offset, limit uint32) error {
	bl, err := getBucketListByCandidateName(candName, offset, limit)
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

func getBucketListByCandidateName(candName string, offset, limit uint32) (*iotextypes.VoteBucketList, error) {
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
	return GetBucketList(iotexapi.ReadStakingDataMethod_BUCKETS_BY_CANDIDATE, readStakingDataRequest)
}
