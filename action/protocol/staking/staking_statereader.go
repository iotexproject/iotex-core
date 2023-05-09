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
	// TODO (iip-13): combine native and liquid staking buckets
	return c.nativeSR.readStateBuckets(ctx, req)
}

func (c *compositeStakingStateReader) readStateBucketsByVoter(ctx context.Context, req *iotexapi.ReadStakingDataRequest_VoteBucketsByVoter) (*iotextypes.VoteBucketList, uint64, error) {
	// TODO (iip-13): combine native and liquid staking buckets
	return c.nativeSR.readStateBucketsByVoter(ctx, req)
}

func (c *compositeStakingStateReader) readStateBucketsByCandidate(ctx context.Context, req *iotexapi.ReadStakingDataRequest_VoteBucketsByCandidate) (*iotextypes.VoteBucketList, uint64, error) {
	// TODO (iip-13): combine native and liquid staking buckets
	return c.nativeSR.readStateBucketsByCandidate(ctx, req)
}

func (c *compositeStakingStateReader) readStateBucketByIndices(ctx context.Context, req *iotexapi.ReadStakingDataRequest_VoteBucketsByIndexes) (*iotextypes.VoteBucketList, uint64, error) {
	// TODO (iip-13): combine native and liquid staking buckets
	return c.nativeSR.readStateBucketByIndices(ctx, req)
}

func (c *compositeStakingStateReader) readStateBucketCount(ctx context.Context, _ *iotexapi.ReadStakingDataRequest_BucketsCount) (*iotextypes.BucketsCount, uint64, error) {
	// TODO (iip-13): combine native and liquid staking buckets
	return c.nativeSR.readStateBucketCount(ctx, nil)
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
	// TODO (iip-13): combine native and liquid staking buckets
	return c.nativeSR.readStateTotalStakingAmount(ctx, nil)
}

func addLSDVotes(candidate *iotextypes.CandidateV2, liquidSR LiquidStakingIndexer) error {
	votes, ok := big.NewInt(0).SetString(candidate.TotalWeightedVotes, 10)
	if !ok {
		return errors.Errorf("invalid total weighted votes %s", candidate.TotalWeightedVotes)
	}
	votes.Add(votes, liquidSR.CandidateVotes(candidate.OwnerAddress))
	candidate.TotalWeightedVotes = votes.String()
	return nil
}
