// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

func (p *Protocol) readStateBuckets(ctx context.Context, sr protocol.StateReader,
	req *iotexapi.ReadStakingDataRequest_VoteBuckets) (*iotextypes.VoteBucketList, error) {
	all, err := getAllBuckets(sr)
	if err != nil {
		return nil, err
	}

	offset := int(req.GetPagination().GetOffset())
	limit := int(req.GetPagination().GetLimit())
	buckets := getPageOfBuckets(all, offset, limit)
	return toIoTexTypesVoteBucketList(buckets)
}

func (p *Protocol) readStateBucketsByVoter(ctx context.Context, sr protocol.StateReader,
	req *iotexapi.ReadStakingDataRequest_VoteBucketsByVoter) (*iotextypes.VoteBucketList, error) {
	voter, err := address.FromString(req.GetVoterAddress())
	if err != nil {
		return nil, err
	}
	indices, err := getVoterBucketIndices(sr, voter)
	if errors.Cause(err) == state.ErrStateNotExist {
		return &iotextypes.VoteBucketList{}, nil
	}
	if indices == nil || err != nil {
		return nil, err
	}
	buckets, err := getBucketsWithIndices(sr, *indices)
	if err != nil {
		return nil, err
	}

	offset := int(req.GetPagination().GetOffset())
	limit := int(req.GetPagination().GetLimit())
	buckets = getPageOfBuckets(buckets, offset, limit)
	return toIoTexTypesVoteBucketList(buckets)
}

func (p *Protocol) readStateBucketsByCandidate(ctx context.Context, sr protocol.StateReader,
	req *iotexapi.ReadStakingDataRequest_VoteBucketsByCandidate) (*iotextypes.VoteBucketList, error) {
	c, err := getCandidateByName(sr, req.GetCandName())
	if err != nil {
		return nil, err
	}
	if c == nil {
		return &iotextypes.VoteBucketList{}, nil
	}

	indices, err := getCandBucketIndices(sr, c.Owner)
	if errors.Cause(err) == state.ErrStateNotExist {
		return &iotextypes.VoteBucketList{}, nil
	}
	if indices == nil || err != nil {
		return nil, err
	}
	buckets, err := getBucketsWithIndices(sr, *indices)
	if err != nil {
		return nil, err
	}

	offset := int(req.GetPagination().GetOffset())
	limit := int(req.GetPagination().GetLimit())
	buckets = getPageOfBuckets(buckets, offset, limit)
	return toIoTexTypesVoteBucketList(buckets)
}

func readStateCandidates(ctx context.Context, sr protocol.StateReader,
	req *iotexapi.ReadStakingDataRequest_Candidates) (*iotextypes.CandidateListV2, error) {
	// TODO
	return nil, nil
}

func readStateCandidateByName(ctx context.Context, sr protocol.StateReader,
	req *iotexapi.ReadStakingDataRequest_CandidateByName) (*iotextypes.CandidateV2, error) {
	// TODO
	return nil, nil
}

func toIoTexTypesVoteBucketList(buckets []*VoteBucket) (*iotextypes.VoteBucketList, error) {
	res := &iotextypes.VoteBucketList{
		Buckets: make([]*iotextypes.VoteBucket, 0, len(buckets)),
	}
	for _, b := range buckets {
		typBucket, err := b.toIoTexTypes()
		if err != nil {
			return nil, err
		}
		res.Buckets = append(res.Buckets, typBucket)
	}
	return res, nil
}

func getPageOfBuckets(buckets []*VoteBucket, offset, limit int) []*VoteBucket {
	var res []*VoteBucket
	if offset >= len(buckets) {
		return res
	}
	buckets = buckets[offset:]
	if limit > len(buckets) {
		limit = len(buckets)
	}
	res = make([]*VoteBucket, 0, limit)
	for i := 0; i < limit; i++ {
		res = append(res, buckets[i])
	}
	return res
}
