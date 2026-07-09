// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// TestReadStateCandidateByAddress exercises the owner/id lookup branches and the
// invalid-address and not-found error paths of readStateCandidateByAddress.
func TestReadStateCandidateByAddress(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)

	owner := identityset.Address(1)
	// Identifier deliberately differs from Owner (post-ownership-transfer case)
	// so the id-lookup branch is distinct from the owner-lookup branch.
	identifier := identityset.Address(3)
	cand := &Candidate{
		Owner:              owner,
		Identifier:         identifier,
		Operator:           identityset.Address(7),
		Reward:             owner,
		Name:               "cand1",
		Votes:              big.NewInt(10),
		SelfStakeBucketIdx: candidateNoSelfStakeBucketIndex,
		SelfStake:          big.NewInt(0),
	}
	// a second, unrelated candidate to exercise the owner/id conflict branch
	owner2 := identityset.Address(2)
	cand2 := &Candidate{
		Owner:              owner2,
		Identifier:         identityset.Address(4),
		Operator:           identityset.Address(8),
		Reward:             owner2,
		Name:               "cand2",
		Votes:              big.NewInt(20),
		SelfStakeBucketIdx: candidateNoSelfStakeBucketIndex,
		SelfStake:          big.NewInt(0),
	}
	candCenter, err := NewCandidateCenter(CandidateList{cand, cand2})
	r.NoError(err)
	csr := &candSR{
		StateReader: sm,
		height:      1,
		view:        &viewData{candCenter: candCenter},
	}

	ctx := protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{BlockHeight: 1})
	ctx = protocol.WithFeatureCtx(genesis.WithGenesisContext(ctx, genesis.TestDefault()))

	t.Run("lookup by owner address", func(t *testing.T) {
		r := require.New(t)
		res, _, err := csr.readStateCandidateByAddress(ctx,
			&iotexapi.ReadStakingDataRequest_CandidateByAddress{OwnerAddr: owner.String()})
		r.NoError(err)
		r.Equal(owner.String(), res.OwnerAddress)
		r.Equal(identifier.String(), res.Id)
		r.Equal("cand1", res.Name)
	})

	t.Run("lookup by identifier", func(t *testing.T) {
		r := require.New(t)
		res, _, err := csr.readStateCandidateByAddress(ctx,
			&iotexapi.ReadStakingDataRequest_CandidateByAddress{Id: identifier.String()})
		r.NoError(err)
		r.Equal("cand1", res.Name)
		r.Equal(identifier.String(), res.Id)
		// an owner address is NOT a valid identifier, so id-lookup by owner misses
		res, _, err = csr.readStateCandidateByAddress(ctx,
			&iotexapi.ReadStakingDataRequest_CandidateByAddress{Id: owner.String()})
		r.NoError(err)
		r.Empty(res.Name)
	})

	t.Run("owner and identifier both supplied and consistent", func(t *testing.T) {
		r := require.New(t)
		// both point at the same candidate -> the id-resolved candidate is returned
		res, _, err := csr.readStateCandidateByAddress(ctx,
			&iotexapi.ReadStakingDataRequest_CandidateByAddress{OwnerAddr: owner.String(), Id: identifier.String()})
		r.NoError(err)
		r.Equal("cand1", res.Name)
		r.Equal(identifier.String(), res.Id)
	})

	t.Run("owner and identifier resolve to different candidates: id wins", func(t *testing.T) {
		r := require.New(t)
		// OwnerAddr resolves to cand2, Id resolves to cand1; per the reader's
		// precedence the Id-resolved candidate is returned and the owner match
		// is ignored.
		res, _, err := csr.readStateCandidateByAddress(ctx,
			&iotexapi.ReadStakingDataRequest_CandidateByAddress{OwnerAddr: owner2.String(), Id: identifier.String()})
		r.NoError(err)
		r.Equal("cand1", res.Name)
		r.Equal(identifier.String(), res.Id)
	})

	t.Run("invalid owner address", func(t *testing.T) {
		r := require.New(t)
		_, _, err := csr.readStateCandidateByAddress(ctx,
			&iotexapi.ReadStakingDataRequest_CandidateByAddress{OwnerAddr: "not-an-address"})
		r.Error(err)
	})

	t.Run("invalid identifier", func(t *testing.T) {
		r := require.New(t)
		_, _, err := csr.readStateCandidateByAddress(ctx,
			&iotexapi.ReadStakingDataRequest_CandidateByAddress{Id: "not-an-address"})
		r.Error(err)
	})

	t.Run("candidate not found returns empty", func(t *testing.T) {
		r := require.New(t)
		res, height, err := csr.readStateCandidateByAddress(ctx,
			&iotexapi.ReadStakingDataRequest_CandidateByAddress{OwnerAddr: identityset.Address(5).String()})
		r.NoError(err)
		r.EqualValues(1, height)
		r.Empty(res.OwnerAddress)
	})
}
