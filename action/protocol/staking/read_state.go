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

func readStateBuckets(ctx context.Context, sr protocol.StateReader,
	req *iotexapi.ReadStakingDataRequest_VoteBuckets) (*iotextypes.VoteBucketList, error) {
	all, err := getAllBuckets(sr)
	if err != nil {
		return nil, err
	}

	offset := int(req.GetPagination().GetOffset())
	limit := int(req.GetPagination().GetLimit())
	buckets := getPageOfBuckets(all, offset, limit)
	return toIoTeXTypesVoteBucketList(buckets)
}

func readStateBucketsByVoter(ctx context.Context, sr protocol.StateReader,
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
	return toIoTeXTypesVoteBucketList(buckets)
}

func readStateBucketsByCandidate(ctx context.Context, sr protocol.StateReader, cv CandidateView,
	req *iotexapi.ReadStakingDataRequest_VoteBucketsByCandidate) (*iotextypes.VoteBucketList, error) {
	c, err := getCandidateByName(cv, req.GetCandName())
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
	return toIoTeXTypesVoteBucketList(buckets)
}

func readStateCandidates(ctx context.Context, cv CandidateView,
	req *iotexapi.ReadStakingDataRequest_Candidates) (*iotextypes.CandidateListV2, error) {
	offset := int(req.GetPagination().GetOffset())
	limit := int(req.GetPagination().GetLimit())
	candidates := getPageOfCandidates(cv.All(), offset, limit)

	return toIoTeXTypesCandidateListV2(candidates), nil
}

func readStateCandidateByName(ctx context.Context, cv CandidateView,
	req *iotexapi.ReadStakingDataRequest_CandidateByName) (*iotextypes.CandidateV2, error) {
	c, err := getCandidateByName(cv, req.GetCandName())
	if err != nil {
		return nil, err
	}
	if c == nil {
		return &iotextypes.CandidateV2{}, nil
	}
	return c.toIoTeXTypes(), nil
}

func toIoTeXTypesVoteBucketList(buckets []*VoteBucket) (*iotextypes.VoteBucketList, error) {
	res := iotextypes.VoteBucketList{
		Buckets: make([]*iotextypes.VoteBucket, 0, len(buckets)),
	}
	for _, b := range buckets {
		typBucket, err := b.toIoTeXTypes()
		if err != nil {
			return nil, err
		}
		res.Buckets = append(res.Buckets, typBucket)
	}
	return &res, nil
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

func toIoTeXTypesCandidateListV2(candidates CandidateList) *iotextypes.CandidateListV2 {
	res := iotextypes.CandidateListV2{
		Candidates: make([]*iotextypes.CandidateV2, 0, len(candidates)),
	}
	for _, c := range candidates {
		res.Candidates = append(res.Candidates, c.toIoTeXTypes())
	}
	return &res
}

func getPageOfCandidates(candidates CandidateList, offset, limit int) CandidateList {
	var res CandidateList
	if offset >= len(candidates) {
		return res
	}
	candidates = candidates[offset:]
	if limit > len(candidates) {
		limit = len(candidates)
	}
	res = make([]*Candidate, 0, limit)
	for i := 0; i < limit; i++ {
		res = append(res, candidates[i])
	}
	return res
}
