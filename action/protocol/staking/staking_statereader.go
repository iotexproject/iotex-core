// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

type (

	// compositiveStakingStateReader is the compositive staking state reader, which combine native and liquid staking
	compositiveStakingStateReader struct {
		liquidSR      LiquidStakingStateReader
		nativeIndexer *CandidatesBucketsIndexer
		nativeSR      CandidateStateReader
	}
)

var (
	_ ReadState = (*compositiveStakingStateReader)(nil)
)

// newCompositiveStakingStateReader creates a new compositive staking state reader
func newCompositiveStakingStateReader(liquidSR LiquidStakingStateReader, nativeIndexer *CandidatesBucketsIndexer, nativeSR CandidateStateReader) *compositiveStakingStateReader {
	return &compositiveStakingStateReader{
		liquidSR:      liquidSR,
		nativeIndexer: nativeIndexer,
		nativeSR:      nativeSR,
	}
}

func (c *compositiveStakingStateReader) readStateBuckets(ctx context.Context, req *iotexapi.ReadStakingDataRequest_VoteBuckets) (*iotextypes.VoteBucketList, uint64, error) {
	return c.nativeSR.readStateBuckets(ctx, req)
}

func (c *compositiveStakingStateReader) readStateBucketsByVoter(ctx context.Context, req *iotexapi.ReadStakingDataRequest_VoteBucketsByVoter) (*iotextypes.VoteBucketList, uint64, error) {
	return c.nativeSR.readStateBucketsByVoter(ctx, req)
}

func (c *compositiveStakingStateReader) readStateBucketsByCandidate(ctx context.Context, req *iotexapi.ReadStakingDataRequest_VoteBucketsByCandidate) (*iotextypes.VoteBucketList, uint64, error) {
	return c.nativeSR.readStateBucketsByCandidate(ctx, req)
}

func (c *compositiveStakingStateReader) readStateBucketByIndices(ctx context.Context, req *iotexapi.ReadStakingDataRequest_VoteBucketsByIndexes) (*iotextypes.VoteBucketList, uint64, error) {
	return c.nativeSR.readStateBucketByIndices(ctx, req)
}

func (c *compositiveStakingStateReader) readStateBucketCount(ctx context.Context, _ *iotexapi.ReadStakingDataRequest_BucketsCount) (*iotextypes.BucketsCount, uint64, error) {
	return c.nativeSR.readStateBucketCount(ctx, nil)
}

func (c *compositiveStakingStateReader) readStateCandidates(ctx context.Context, req *iotexapi.ReadStakingDataRequest_Candidates) (*iotextypes.CandidateListV2, uint64, error) {
	candidates, height, err := c.nativeSR.readStateCandidates(ctx, req)
	if err != nil {
		return candidates, height, err
	}
	for _, candidate := range candidates.Candidates {
		if err = addLiquidVotes(candidate, c.liquidSR); err != nil {
			return candidates, height, err
		}
	}
	return candidates, height, nil
}

func (c *compositiveStakingStateReader) readStateCandidatesByIndexer(ctx context.Context, req *iotexapi.ReadStakingDataRequest_Candidates, height uint64) (*iotextypes.CandidateListV2, uint64, error) {
	candidatesBytes, height, err := c.nativeIndexer.GetCandidates(height, req.GetPagination().GetOffset(), req.GetPagination().GetLimit())
	if err != nil {
		return nil, height, err
	}
	candidates := &iotextypes.CandidateListV2{}
	if err := proto.Unmarshal(candidatesBytes, candidates); err != nil {
		return nil, height, errors.Wrap(err, "failed to unmarshal candidates")
	}
	for _, candidate := range candidates.Candidates {
		if err = addLiquidVotes(candidate, c.liquidSR); err != nil {
			return candidates, height, err
		}
	}
	return candidates, height, nil
}

func (c *compositiveStakingStateReader) readStateCandidateByName(ctx context.Context, req *iotexapi.ReadStakingDataRequest_CandidateByName) (*iotextypes.CandidateV2, uint64, error) {
	candidate, height, err := c.nativeSR.readStateCandidateByName(ctx, req)
	if err != nil {
		return candidate, height, err
	}
	if err := addLiquidVotes(candidate, c.liquidSR); err != nil {
		return candidate, height, err
	}
	return candidate, height, nil
}

func (c *compositiveStakingStateReader) readStateCandidateByAddress(ctx context.Context, req *iotexapi.ReadStakingDataRequest_CandidateByAddress) (*iotextypes.CandidateV2, uint64, error) {
	candidate, height, err := c.nativeSR.readStateCandidateByAddress(ctx, req)
	if err != nil {
		return candidate, height, err
	}
	if err := addLiquidVotes(candidate, c.liquidSR); err != nil {
		return candidate, height, err
	}
	return candidate, height, nil
}

func (c *compositiveStakingStateReader) readStateTotalStakingAmount(ctx context.Context, _ *iotexapi.ReadStakingDataRequest_TotalStakingAmount) (*iotextypes.AccountMeta, uint64, error) {
	return c.nativeSR.readStateTotalStakingAmount(ctx, nil)
}

func addLiquidVotes(candidate *iotextypes.CandidateV2, liquidSR LiquidStakingStateReader) error {
	votes, ok := big.NewInt(0).SetString(candidate.TotalWeightedVotes, 10)
	if !ok {
		return errors.Errorf("invalid total weighted votes %s", candidate.TotalWeightedVotes)
	}
	votes.Add(votes, liquidSR.CandidateVotes(candidate.Name))
	candidate.TotalWeightedVotes = votes.String()
	return nil
}
