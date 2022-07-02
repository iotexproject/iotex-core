// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import (
	"strconv"
	"strings"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
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
			MethodVoter + ", " + MethodCandidate + "]",
		config.Chinese: "根据方法和参数在IoTeX区块链上读取投票列表\n可用方法有：" +
			MethodVoter + "，" + MethodCandidate,
	}
)

// constants
const (
	MethodVoter     = "voter"
	MethodCandidate = "cand"
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
			cmd.SilenceUsage = true

			var (
				bl      *iotextypes.VoteBucketList
				address string
				err     error
			)

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
			case MethodVoter:
				address, err = client.AddressWithDefaultIfNotExist(addr)
				if err != nil {
					return err
				}
				bl, err = getBucketListByVoterAddress(client, address, uint32(offset), uint32(limit))
			case MethodCandidate:
				bl, err = getBucketListByCandidateName(client, addr, uint32(offset), uint32(limit))
			default:
				return errors.New("unknown <method>")
			}
			if err != nil {
				return err
			}
			var lines []string
			if len(bl.Buckets) == 0 {
				lines = append(lines, "Empty bucketlist with given address")
			} else {
				for _, b := range bl.Buckets {
					bucket, err := newBucket(b)
					if err != nil {
						return err
					}
					lines = append(lines, bucket.String())
				}
			}
			cmd.Println(strings.Join(lines, "\n"))
			return nil
		},
	}
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
