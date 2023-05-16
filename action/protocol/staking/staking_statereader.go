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
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
)

type (
	// compositeStakingStateReader is the compositive staking state reader, which combine native and liquid staking
	compositeStakingStateReader struct {
		liquidIndexer LiquidStakingIndexer
		nativeIndexer *CandidatesBucketsIndexer
		nativeSR      CandidateStateReader
	}
)

// newCompositeStakingStateReader creates a new compositive staking state reader
func newCompositeStakingStateReader(liquidIndexer LiquidStakingIndexer, nativeIndexer *CandidatesBucketsIndexer, sr protocol.StateReader) (ReadState, error) {
	nativeSR, err := ConstructBaseView(sr)
	if err != nil {
		return nil, err
	}
	return &compositeStakingStateReader{
		liquidIndexer: liquidIndexer,
		nativeIndexer: nativeIndexer,
		nativeSR:      nativeSR,
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
		buckets      *iotextypes.VoteBucketList
		height       uint64
		bucketsBytes []byte
	)
	if epochStartHeight != 0 && c.nativeIndexer != nil {
		// read native buckets from indexer
		bucketsBytes, height, err = c.nativeIndexer.GetBuckets(epochStartHeight, req.GetPagination().GetOffset(), req.GetPagination().GetLimit())
		if err != nil {
			return nil, 0, err
		}
		buckets = &iotextypes.VoteBucketList{}
		if err := proto.Unmarshal(bucketsBytes, buckets); err != nil {
			return nil, 0, err
		}
	} else {
		// read native buckets from state
		buckets, height, err = c.nativeSR.readStateBuckets(ctx, req)
		if err != nil {
			return nil, 0, err
		}
	}
	// read LSD buckets
	lsdBuckets, err := c.liquidIndexer.Buckets()
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

	// read LSD buckets
	lsdBuckets, err := c.liquidIndexer.Buckets()
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
	// read LSD buckets
	lsdBuckets, err := c.liquidIndexer.Buckets()
	if err != nil {
		return nil, 0, err
	}
	candidate := c.nativeSR.GetCandidateByName(req.GetCandName())
	if candidate == nil {
		return &iotextypes.VoteBucketList{}, height, nil
	}
	lsdBuckets = filterBucketsByCandidate(lsdBuckets, candidate.Owner)
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
	// read LSD buckets
	lsdBuckets, err := c.liquidIndexer.BucketsByIndices(req.GetIndex())
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
	buckets, err := c.liquidIndexer.Buckets()
	if err != nil {
		return nil, 0, err
	}
	bucketCnt.Active += uint64(len(buckets))
	bucketCnt.Total += c.liquidIndexer.TotalBucketCount()
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
		candidates      *iotextypes.CandidateListV2
		height          uint64
		candidatesBytes []byte
	)
	if epochStartHeight != 0 && c.nativeIndexer != nil {
		// read candidates from indexer
		candidatesBytes, height, err = c.nativeIndexer.GetCandidates(height, req.GetPagination().GetOffset(), req.GetPagination().GetLimit())
		if err != nil {
			return nil, 0, err
		}
		candidates = &iotextypes.CandidateListV2{}
		if err = proto.Unmarshal(candidatesBytes, candidates); err != nil {
			return nil, 0, errors.Wrap(err, "failed to unmarshal candidates")
		}
	} else {
		// read candidates from native state
		candidates, height, err = c.nativeSR.readStateCandidates(ctx, req)
		if err != nil {
			return nil, 0, err
		}
	}

	// add liquid stake votes
	for _, candidate := range candidates.Candidates {
		if err = addLSDVotes(candidate, c.liquidIndexer); err != nil {
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
	if err := addLSDVotes(candidate, c.liquidIndexer); err != nil {
		return nil, 0, err
	}
	return candidate, height, nil
}

func (c *compositeStakingStateReader) readStateCandidateByAddress(ctx context.Context, req *iotexapi.ReadStakingDataRequest_CandidateByAddress) (*iotextypes.CandidateV2, uint64, error) {
	candidate, height, err := c.nativeSR.readStateCandidateByAddress(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	if err := addLSDVotes(candidate, c.liquidIndexer); err != nil {
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

	// add liquid staking amount
	buckets, err := c.liquidIndexer.Buckets()
	if err != nil {
		return nil, 0, err
	}
	for _, bucket := range buckets {
		amount.Add(amount, bucket.StakedAmount)
	}

	accountMeta.Balance = amount.String()
	return accountMeta, height, nil
}

func addLSDVotes(candidate *iotextypes.CandidateV2, liquidSR LiquidStakingIndexer) error {
	votes, ok := big.NewInt(0).SetString(candidate.TotalWeightedVotes, 10)
	if !ok {
		return errors.Errorf("invalid total weighted votes %s", candidate.TotalWeightedVotes)
	}
	addr, err := address.FromString(candidate.OwnerAddress)
	if err != nil {
		return err
	}
	votes.Add(votes, liquidSR.CandidateVotes(addr))
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

func filterBucketsByCandidate(buckets []*VoteBucket, candidate address.Address) []*VoteBucket {
	var filtered []*VoteBucket
	for _, bucket := range buckets {
		if bucket.Candidate.String() == candidate.String() {
			filtered = append(filtered, bucket)
		}
	}
	return filtered
}
