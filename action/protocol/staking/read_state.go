// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
)

func readStateBuckets(ctx context.Context, sr protocol.StateReader,
	req *iotexapi.ReadStakingDataRequest_VoteBuckets) (*iotextypes.VoteBucketList, uint64, error) {
	all, height, err := getAllBuckets(sr)
	if err != nil {
		return nil, height, err
	}

	offset := int(req.GetPagination().GetOffset())
	limit := int(req.GetPagination().GetLimit())
	buckets := getPageOfBuckets(all, offset, limit)
	pbBuckets, err := toIoTeXTypesVoteBucketList(buckets)
	return pbBuckets, height, err
}

func readStateBucketsByVoter(ctx context.Context, sr protocol.StateReader,
	req *iotexapi.ReadStakingDataRequest_VoteBucketsByVoter) (*iotextypes.VoteBucketList, uint64, error) {
	voter, err := address.FromString(req.GetVoterAddress())
	if err != nil {
		return nil, 0, err
	}

	indices, height, err := getVoterBucketIndices(sr, voter)
	if errors.Cause(err) == state.ErrStateNotExist {
		return &iotextypes.VoteBucketList{}, height, nil
	}
	if indices == nil || err != nil {
		return nil, height, err
	}
	buckets, err := getBucketsWithIndices(sr, *indices)
	if err != nil {
		return nil, height, err
	}

	offset := int(req.GetPagination().GetOffset())
	limit := int(req.GetPagination().GetLimit())
	buckets = getPageOfBuckets(buckets, offset, limit)
	pbBuckets, err := toIoTeXTypesVoteBucketList(buckets)
	return pbBuckets, height, err
}

func readStateBucketsByCandidate(ctx context.Context, csr CandidateStateReader,
	req *iotexapi.ReadStakingDataRequest_VoteBucketsByCandidate) (*iotextypes.VoteBucketList, uint64, error) {
	c := csr.GetCandidateByName(req.GetCandName())
	if c == nil {
		return &iotextypes.VoteBucketList{}, 0, nil
	}

	indices, height, err := getCandBucketIndices(csr.SR(), c.Owner)
	if errors.Cause(err) == state.ErrStateNotExist {
		return &iotextypes.VoteBucketList{}, height, nil
	}
	if indices == nil || err != nil {
		return nil, height, err
	}
	buckets, err := getBucketsWithIndices(csr.SR(), *indices)
	if err != nil {
		return nil, height, err
	}

	offset := int(req.GetPagination().GetOffset())
	limit := int(req.GetPagination().GetLimit())
	buckets = getPageOfBuckets(buckets, offset, limit)
	pbBuckets, err := toIoTeXTypesVoteBucketList(buckets)
	return pbBuckets, height, err
}

func readStateBucketByIndices(ctx context.Context, sr protocol.StateReader,
	req *iotexapi.ReadStakingDataRequest_VoteBucketsByIndexes) (*iotextypes.VoteBucketList, uint64, error) {
	height, err := sr.Height()
	if err != nil {
		return &iotextypes.VoteBucketList{}, height, err
	}
	buckets, err := getBucketsWithIndices(sr, BucketIndices(req.GetIndex()))
	if err != nil {
		return nil, height, err
	}
	pbBuckets, err := toIoTeXTypesVoteBucketList(buckets)
	return pbBuckets, height, err
}

func readStateBucketCount(ctx context.Context, csr CandidateStateReader,
	_ *iotexapi.ReadStakingDataRequest_BucketsCount) (*iotextypes.BucketsCount, uint64, error) {
	total, err := getTotalBucketCount(csr.SR())
	if errors.Cause(err) == state.ErrStateNotExist {
		return &iotextypes.BucketsCount{}, csr.Height(), nil
	}
	if err != nil {
		return nil, 0, err
	}
	active, h, err := getActiveBucketsCount(ctx, csr)
	if err != nil {
		return nil, h, err
	}
	return &iotextypes.BucketsCount{
		Total:  total,
		Active: active,
	}, h, nil
}

func readStateCandidates(ctx context.Context, csr CandidateStateReader,
	req *iotexapi.ReadStakingDataRequest_Candidates) (*iotextypes.CandidateListV2, uint64, error) {
	offset := int(req.GetPagination().GetOffset())
	limit := int(req.GetPagination().GetLimit())
	candidates := getPageOfCandidates(csr.AllCandidates(), offset, limit)

	return toIoTeXTypesCandidateListV2(candidates), csr.Height(), nil
}

func readStateCandidateByName(ctx context.Context, csr CandidateStateReader,
	req *iotexapi.ReadStakingDataRequest_CandidateByName) (*iotextypes.CandidateV2, uint64, error) {
	c := csr.GetCandidateByName(req.GetCandName())
	if c == nil {
		return &iotextypes.CandidateV2{}, csr.Height(), nil
	}
	return c.toIoTeXTypes(), csr.Height(), nil
}

func readStateCandidateByAddress(ctx context.Context, csr CandidateStateReader,
	req *iotexapi.ReadStakingDataRequest_CandidateByAddress) (*iotextypes.CandidateV2, uint64, error) {
	owner, err := address.FromString(req.GetOwnerAddr())
	if err != nil {
		return nil, 0, err
	}
	c := csr.GetCandidateByOwner(owner)
	if c == nil {
		return &iotextypes.CandidateV2{}, csr.Height(), nil
	}
	return c.toIoTeXTypes(), csr.Height(), nil
}

func readStateTotalStakingAmount(ctx context.Context, csr CandidateStateReader,
	_ *iotexapi.ReadStakingDataRequest_TotalStakingAmount) (*iotextypes.AccountMeta, uint64, error) {
	meta := iotextypes.AccountMeta{}
	meta.Address = address.StakingBucketPoolAddr
	total, h, err := getTotalStakedAmount(ctx, csr)
	if err != nil {
		return nil, h, err
	}
	meta.Balance = total.String()
	return &meta, h, nil
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

func getTotalStakedAmount(ctx context.Context, csr CandidateStateReader) (*big.Int, uint64, error) {
	featureCtx := protocol.MustGetFeatureWithHeightCtx(ctx)
	if featureCtx.ReadStateFromDB(csr.Height()) {
		// after Greenland, read state from db
		var total totalAmount
		h, err := csr.SR().State(&total, protocol.NamespaceOption(StakingNameSpace), protocol.KeyOption(_bucketPoolAddrKey))
		if err != nil {
			return nil, h, err
		}
		return total.amount, h, nil
	}

	// otherwise read from bucket pool
	return csr.TotalStakedAmount(), csr.Height(), nil
}

func getActiveBucketsCount(ctx context.Context, csr CandidateStateReader) (uint64, uint64, error) {
	featureCtx := protocol.MustGetFeatureWithHeightCtx(ctx)
	if featureCtx.ReadStateFromDB(csr.Height()) {
		// after Greenland, read state from db
		var total totalAmount
		h, err := csr.SR().State(&total, protocol.NamespaceOption(StakingNameSpace), protocol.KeyOption(_bucketPoolAddrKey))
		if err != nil {
			return 0, h, err
		}
		return total.count, h, nil
	}
	// otherwise read from bucket pool
	return csr.ActiveBucketsCount(), csr.Height(), nil
}
