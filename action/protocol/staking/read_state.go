// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

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

func getPageOfArray[T any](array []T, offset, limit int) []T {
	var res []T
	if offset >= len(array) {
		return res
	}
	array = array[offset:]
	if limit > len(array) {
		limit = len(array)
	}
	res = make([]T, 0, limit)
	for i := 0; i < limit; i++ {
		res = append(res, array[i])
	}
	return res
}

func getPageOfBuckets(buckets []*VoteBucket, offset, limit int) []*VoteBucket {
	return getPageOfArray(buckets, offset, limit)
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
	return getPageOfArray(candidates, offset, limit)
}

func getTotalStakedAmount(ctx context.Context, csr CandidateStateReader) (*big.Int, uint64, error) {
	featureCtx := protocol.MustGetFeatureWithHeightCtx(ctx)
	if featureCtx.ReadStateFromDB(csr.Height()) {
		// after Greenland, read state from db
		var total totalAmount
		h, err := csr.SR().State(&total, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(_bucketPoolAddrKey))
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
		h, err := csr.SR().State(&total, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(_bucketPoolAddrKey))
		if err != nil {
			return 0, h, err
		}
		return total.count, h, nil
	}
	// otherwise read from bucket pool
	return csr.ActiveBucketsCount(), csr.Height(), nil
}
