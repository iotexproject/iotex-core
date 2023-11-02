// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
)

type (
	// compositeStakingStateReader is the compositive staking state reader, which combine native and contract staking
	compositeStakingStateReader struct {
		contractIndexer ContractStakingIndexer
		nativeIndexer   *CandidatesBucketsIndexer
		nativeSR        CandidateStateReader
	}
)

// newCompositeStakingStateReader creates a new compositive staking state reader
func newCompositeStakingStateReader(contractIndexer ContractStakingIndexer, nativeIndexer *CandidatesBucketsIndexer, sr protocol.StateReader) (*compositeStakingStateReader, error) {
	nativeSR, err := ConstructBaseView(sr)
	if err != nil {
		return nil, err
	}
	return &compositeStakingStateReader{
		contractIndexer: contractIndexer,
		nativeIndexer:   nativeIndexer,
		nativeSR:        nativeSR,
	}, nil
}

func (c *compositeStakingStateReader) readStateBuckets(ctx context.Context, req *iotexapi.ReadStakingDataRequest_VoteBuckets) (*iotextypes.VoteBucketList, uint64, error) {
	// get height arg
	inputHeight, err := c.nativeSR.SR().Height()
	if err != nil {
		return nil, 0, err
	}
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	epochStartHeight := rp.GetEpochHeight(rp.GetEpochNum(inputHeight))

	var (
		buckets *iotextypes.VoteBucketList
		height  uint64
	)
	if epochStartHeight != 0 && c.nativeIndexer != nil {
		// read native buckets from indexer
		buckets, height, err = c.nativeIndexer.GetBuckets(epochStartHeight, req.GetPagination().GetOffset(), req.GetPagination().GetLimit())
		if err != nil {
			return nil, 0, err
		}
	} else {
		// read native buckets from state
		buckets, height, err = c.nativeSR.readStateBuckets(ctx, req)
		if err != nil {
			return nil, 0, err
		}
	}

	if !c.isContractStakingEnabled() {
		buckets.Buckets = getPageOfArray(buckets.Buckets, int(req.GetPagination().GetOffset()), int(req.GetPagination().GetLimit()))
		return buckets, height, nil
	}

	// read LSD buckets
	lsdBuckets, err := c.contractIndexer.Buckets(inputHeight)
	if err != nil {
		return nil, 0, err
	}
	lsdIoTeXBuckets, err := toIoTeXTypesVoteBucketList(lsdBuckets)
	if err != nil {
		return nil, 0, err
	}
	// merge native and LSD buckets
	buckets.Buckets = append(buckets.Buckets, lsdIoTeXBuckets.Buckets...)
	buckets.Buckets = getPageOfArray(buckets.Buckets, int(req.GetPagination().GetOffset()), int(req.GetPagination().GetLimit()))
	return buckets, height, err
}

func (c *compositeStakingStateReader) readStateBucketsByVoter(ctx context.Context, req *iotexapi.ReadStakingDataRequest_VoteBucketsByVoter) (*iotextypes.VoteBucketList, uint64, error) {
	// read native buckets
	buckets, height, err := c.nativeSR.readStateBucketsByVoter(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	if !c.isContractStakingEnabled() {
		buckets.Buckets = getPageOfArray(buckets.Buckets, int(req.GetPagination().GetOffset()), int(req.GetPagination().GetLimit()))
		return buckets, height, err
	}

	// read LSD buckets
	lsdBuckets, err := c.contractIndexer.Buckets(height)
	if err != nil {
		return nil, 0, err
	}
	lsdBuckets = filterBucketsByVoter(lsdBuckets, req.GetVoterAddress())
	lsdIoTeXBuckets, err := toIoTeXTypesVoteBucketList(lsdBuckets)
	if err != nil {
		return nil, 0, err
	}
	// merge native and LSD buckets
	buckets.Buckets = append(buckets.Buckets, lsdIoTeXBuckets.Buckets...)
	buckets.Buckets = getPageOfArray(buckets.Buckets, int(req.GetPagination().GetOffset()), int(req.GetPagination().GetLimit()))
	return buckets, height, err
}

func (c *compositeStakingStateReader) readStateBucketsByCandidate(ctx context.Context, req *iotexapi.ReadStakingDataRequest_VoteBucketsByCandidate) (*iotextypes.VoteBucketList, uint64, error) {
	// read native buckets
	buckets, height, err := c.nativeSR.readStateBucketsByCandidate(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	if !c.isContractStakingEnabled() {
		buckets.Buckets = getPageOfArray(buckets.Buckets, int(req.GetPagination().GetOffset()), int(req.GetPagination().GetLimit()))
		return buckets, height, err
	}

	// read LSD buckets
	candidate := c.nativeSR.GetCandidateByName(req.GetCandName())
	if candidate == nil {
		return &iotextypes.VoteBucketList{}, height, nil
	}
	lsdBuckets, err := c.contractIndexer.BucketsByCandidate(candidate.Owner, height)
	if err != nil {
		return nil, 0, err
	}
	lsdIoTeXBuckets, err := toIoTeXTypesVoteBucketList(lsdBuckets)
	if err != nil {
		return nil, 0, err
	}
	// merge native and LSD buckets
	buckets.Buckets = append(buckets.Buckets, lsdIoTeXBuckets.Buckets...)
	buckets.Buckets = getPageOfArray(buckets.Buckets, int(req.GetPagination().GetOffset()), int(req.GetPagination().GetLimit()))
	return buckets, height, err
}

func (c *compositeStakingStateReader) readStateBucketByIndices(ctx context.Context, req *iotexapi.ReadStakingDataRequest_VoteBucketsByIndexes) (*iotextypes.VoteBucketList, uint64, error) {
	// read native buckets
	buckets, height, err := c.nativeSR.readStateBucketByIndices(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	if !c.isContractStakingEnabled() {
		return buckets, height, nil
	}

	// read LSD buckets
	lsdBuckets, err := c.contractIndexer.BucketsByIndices(req.GetIndex(), height)
	if err != nil {
		return nil, 0, err
	}
	lsbIoTeXBuckets, err := toIoTeXTypesVoteBucketList(lsdBuckets)
	if err != nil {
		return nil, 0, err
	}
	// merge native and LSD buckets
	buckets.Buckets = append(buckets.Buckets, lsbIoTeXBuckets.Buckets...)
	return buckets, height, nil
}

func (c *compositeStakingStateReader) readStateBucketCount(ctx context.Context, req *iotexapi.ReadStakingDataRequest_BucketsCount) (*iotextypes.BucketsCount, uint64, error) {
	bucketCnt, height, err := c.nativeSR.readStateBucketCount(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	if !c.isContractStakingEnabled() {
		return bucketCnt, height, nil
	}
	buckets, err := c.contractIndexer.Buckets(height)
	if err != nil {
		return nil, 0, err
	}
	bucketCnt.Active += uint64(len(buckets))
	tbc, err := c.contractIndexer.TotalBucketCount(height)
	if err != nil {
		return nil, 0, err
	}
	bucketCnt.Total += tbc
	return bucketCnt, height, nil
}

func (c *compositeStakingStateReader) readStateCandidates(ctx context.Context, req *iotexapi.ReadStakingDataRequest_Candidates) (*iotextypes.CandidateListV2, uint64, error) {
	// get height arg
	inputHeight, err := c.nativeSR.SR().Height()
	if err != nil {
		return nil, 0, err
	}
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	epochStartHeight := rp.GetEpochHeight(rp.GetEpochNum(inputHeight))

	// read native candidates
	var (
		candidates *iotextypes.CandidateListV2
		height     uint64
	)
	if epochStartHeight != 0 && c.nativeIndexer != nil {
		// read candidates from indexer
		candidates, height, err = c.nativeIndexer.GetCandidates(epochStartHeight, req.GetPagination().GetOffset(), req.GetPagination().GetLimit())
		if err != nil {
			return nil, 0, err
		}
	} else {
		// read candidates from native state
		candidates, height, err = c.nativeSR.readStateCandidates(ctx, req)
		if err != nil {
			return nil, 0, err
		}
	}
	if !protocol.MustGetFeatureCtx(ctx).AddContractStakingVotes {
		return candidates, height, nil
	}
	if !c.isContractStakingEnabled() {
		return candidates, height, nil
	}
	for _, candidate := range candidates.Candidates {
		if err = addContractStakingVotes(ctx, candidate, c.contractIndexer, height); err != nil {
			return nil, 0, err
		}
	}
	return candidates, height, nil
}

func (c *compositeStakingStateReader) readStateCandidateByName(ctx context.Context, req *iotexapi.ReadStakingDataRequest_CandidateByName) (*iotextypes.CandidateV2, uint64, error) {
	candidate, height, err := c.nativeSR.readStateCandidateByName(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	if !c.isContractStakingEnabled() {
		return candidate, height, nil
	}
	if !protocol.MustGetFeatureCtx(ctx).AddContractStakingVotes {
		return candidate, height, nil
	}
	if err := addContractStakingVotes(ctx, candidate, c.contractIndexer, height); err != nil {
		return nil, 0, err
	}
	return candidate, height, nil
}

func (c *compositeStakingStateReader) readStateCandidateByAddress(ctx context.Context, req *iotexapi.ReadStakingDataRequest_CandidateByAddress) (*iotextypes.CandidateV2, uint64, error) {
	candidate, height, err := c.nativeSR.readStateCandidateByAddress(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	if !c.isContractStakingEnabled() {
		return candidate, height, nil
	}
	if !protocol.MustGetFeatureCtx(ctx).AddContractStakingVotes {
		return candidate, height, nil
	}
	if err := addContractStakingVotes(ctx, candidate, c.contractIndexer, height); err != nil {
		return nil, 0, err
	}
	return candidate, height, nil
}

func (c *compositeStakingStateReader) readStateTotalStakingAmount(ctx context.Context, _ *iotexapi.ReadStakingDataRequest_TotalStakingAmount) (*iotextypes.AccountMeta, uint64, error) {
	// read native total staking amount
	accountMeta, height, err := c.nativeSR.readStateTotalStakingAmount(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	amount, ok := big.NewInt(0).SetString(accountMeta.Balance, 10)
	if !ok {
		return nil, 0, errors.Errorf("invalid balance %s", accountMeta.Balance)
	}
	if !c.isContractStakingEnabled() {
		return accountMeta, height, nil
	}
	// add contract staking amount
	buckets, err := c.contractIndexer.Buckets(height)
	if err != nil {
		return nil, 0, err
	}
	for _, bucket := range buckets {
		amount.Add(amount, bucket.StakedAmount)
	}

	accountMeta.Balance = amount.String()
	return accountMeta, height, nil
}

func (c *compositeStakingStateReader) readStateContractStakingBucketTypes(ctx context.Context, _ *iotexapi.ReadStakingDataRequest_ContractStakingBucketTypes) (*iotextypes.ContractStakingBucketTypeList, uint64, error) {
	if !c.isContractStakingEnabled() {
		return &iotextypes.ContractStakingBucketTypeList{}, c.nativeSR.Height(), nil
	}
	height := c.nativeSR.Height()
	bts, err := c.contractIndexer.BucketTypes(height)
	if err != nil {
		return nil, 0, err
	}
	pbBts := make([]*iotextypes.ContractStakingBucketType, 0, len(bts))
	for _, bt := range bts {
		pbBts = append(pbBts, &iotextypes.ContractStakingBucketType{
			StakedAmount:   bt.Amount.String(),
			StakedDuration: uint32(bt.Duration),
		})
	}
	return &iotextypes.ContractStakingBucketTypeList{BucketTypes: pbBts}, height, nil
}

func (c *compositeStakingStateReader) isContractStakingEnabled() bool {
	return c.contractIndexer != nil
}

// TODO: move into compositeStakingStateReader
func addContractStakingVotes(ctx context.Context, candidate *iotextypes.CandidateV2, contractStakingSR ContractStakingIndexer, height uint64) error {
	votes, ok := big.NewInt(0).SetString(candidate.TotalWeightedVotes, 10)
	if !ok {
		return errors.Errorf("invalid total weighted votes %s", candidate.TotalWeightedVotes)
	}
	addr, err := address.FromString(candidate.OwnerAddress)
	if err != nil {
		return err
	}
	contractVotes, err := contractStakingSR.CandidateVotes(ctx, addr, height)
	if err != nil {
		return err
	}
	votes.Add(votes, contractVotes)
	candidate.TotalWeightedVotes = votes.String()
	return nil
}

func filterBucketsByVoter(buckets []*VoteBucket, voterAddress string) []*VoteBucket {
	var filtered []*VoteBucket
	for _, bucket := range buckets {
		if bucket.Owner.String() == voterAddress {
			filtered = append(filtered, bucket)
		}
	}
	return filtered
}
