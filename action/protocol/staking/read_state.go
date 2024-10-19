// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/state"
)

func toIoTeXTypesVoteBucketList(sr protocol.StateReader, buckets []*VoteBucket) (*iotextypes.VoteBucketList, error) {
	esr := NewEndorsementStateReader(sr)
	res := iotextypes.VoteBucketList{
		Buckets: make([]*iotextypes.VoteBucket, 0, len(buckets)),
	}
	for _, b := range buckets {
		typBucket, err := b.toIoTeXTypes()
		if err != nil {
			return nil, err
		}
		// fill in the endorsement
		if b.isNative() {
			endorsement, err := esr.Get(b.Index)
			switch errors.Cause(err) {
			case nil:
				typBucket.EndorsementExpireBlockHeight = endorsement.ExpireHeight
			case state.ErrStateNotExist:
			default:
				return nil, err
			}
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

func toIoTeXTypesCandidateV2(csr CandidateStateReader, cand *Candidate, featureCtx protocol.FeatureCtx) (*iotextypes.CandidateV2, error) {
	esr := NewEndorsementStateReader(csr.SR())
	height, _ := csr.SR().Height()
	needClear := func(c *Candidate) (bool, error) {
		if !c.isSelfStakeBucketSettled() {
			return false, nil
		}
		vb, err := csr.getBucket(c.SelfStakeBucketIdx)
		if err != nil {
			if errors.Is(err, state.ErrStateNotExist) {
				return true, nil
			}
			return false, err
		}
		if isSelfOwnedBucket(csr, vb) {
			return vb.isUnstaked(), nil
		}
		status, err := esr.Status(featureCtx, c.SelfStakeBucketIdx, height)
		if err != nil {
			return false, err
		}
		return status == EndorseExpired, nil
	}
	c := cand.toIoTeXTypes()
	// clear self-stake bucket if endorsement is expired but not updated yet
	clear, err := needClear(cand)
	if err != nil {
		return nil, err
	}
	if clear {
		c.SelfStakeBucketIdx = candidateNoSelfStakeBucketIndex
		c.SelfStakingTokens = "0"
	}
	return c, nil
}

func toIoTeXTypesCandidateListV2(csr CandidateStateReader, candidates CandidateList, featureCtx protocol.FeatureCtx) (*iotextypes.CandidateListV2, error) {
	res := iotextypes.CandidateListV2{
		Candidates: make([]*iotextypes.CandidateV2, 0, len(candidates)),
	}
	for _, c := range candidates {
		cand, err := toIoTeXTypesCandidateV2(csr, c, featureCtx)
		if err != nil {
			return nil, err
		}
		res.Candidates = append(res.Candidates, cand)
	}
	return &res, nil
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
