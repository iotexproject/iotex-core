// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action/protocol"
)

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
