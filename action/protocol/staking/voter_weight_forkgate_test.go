// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

// forkGateCtx builds a context whose FeatureCtx has IIP-59 either off or on.
func forkGateCtx(height uint64, activated bool) context.Context {
	g := genesis.TestDefault()
	if activated {
		g.ToBeEnabledBlockHeight = height
	}
	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
	return protocol.WithFeatureCtx(ctx)
}

// TestVoterWeightPersistenceGatedOnFork pins the rule that keeps this change
// inert before IIP-59 activates.
//
// Nodes take a new release over days, well ahead of the activation height. If
// voter weight entries were written during that window, an upgraded node would
// put state into the staking namespace that the previous release does not
// write, and its state root would diverge from every node still on the old
// binary — a split at deployment time rather than at activation. The in-memory
// view is still maintained on both sides of the fork, so it is already correct
// at the block the flag flips; only the state write waits.
func TestVoterWeightPersistenceGatedOnFork(t *testing.T) {
	r := require.New(t)
	cand := candID(1)
	voter := identityset.Address(2)

	t.Run("pre-fork writes nothing", func(t *testing.T) {
		sm := mock_chainmanager.NewMockStateManager(gomock.NewController(t))
		// No PutState expectation: any call fails the test.
		v := NewVoterWeightView()
		v.Apply(cand, voter, big.NewInt(100))
		r.True(v.IsDirty(), "the view still tracks weights before the fork")

		out, err := v.Commit(forkGateCtx(1, false), sm)
		r.NoError(err)
		r.False(out.IsDirty(), "commit still clears the dirty flag")
	})

	t.Run("post-fork writes the touched entry", func(t *testing.T) {
		sm := mock_chainmanager.NewMockStateManager(gomock.NewController(t))
		v := NewVoterWeightView()
		v.Apply(cand, voter, big.NewInt(100))

		var gotKey []byte
		var gotWeight *big.Int
		sm.EXPECT().PutState(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(src interface{}, opts ...protocol.StateOption) (uint64, error) {
				key, err := testStateConfig(opts...)
				if err != nil {
					return 0, err
				}
				gotKey = key
				gotWeight = src.(*voterWeightEntry).Weight
				return 1, nil
			},
		).Times(1)

		_, err := v.Commit(forkGateCtx(1, true), sm)
		r.NoError(err)
		r.Equal(voterWeightKey(cand, hash.BytesToHash160(voter.Bytes())), gotKey)
		r.Equal(int64(100), gotWeight.Int64())
	})

	t.Run("post-fork deletes an entry driven to zero", func(t *testing.T) {
		sm := mock_chainmanager.NewMockStateManager(gomock.NewController(t))
		v := NewVoterWeightView()
		v.Apply(cand, voter, big.NewInt(100))
		_, err := v.Commit(forkGateCtx(1, false), sm) // pre-fork: nothing written
		r.NoError(err)

		// The voter's whole weight is withdrawn: the pair must leave state
		// rather than be stored as a zero.
		v.Apply(cand, voter, big.NewInt(-100))
		var delKey []byte
		sm.EXPECT().DelState(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(opts ...protocol.StateOption) (uint64, error) {
				key, err := testStateConfig(opts...)
				if err != nil {
					return 0, err
				}
				delKey = key
				return 1, nil
			},
		).Times(1)

		_, err = v.Commit(forkGateCtx(1, true), sm)
		r.NoError(err)
		r.Equal(voterWeightKey(cand, hash.BytesToHash160(voter.Bytes())), delKey)
	})

	t.Run("no feature ctx writes nothing", func(t *testing.T) {
		// A commit path that lost its FeatureCtx must fall back to the
		// pre-fork behaviour rather than guessing, so a missing context can
		// never introduce a state write the rest of the network does not make.
		sm := mock_chainmanager.NewMockStateManager(gomock.NewController(t))
		v := NewVoterWeightView()
		v.Apply(cand, voter, big.NewInt(100))

		_, err := v.Commit(context.Background(), sm)
		r.NoError(err)
	})
}
