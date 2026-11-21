// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_actpool"
)

// bundleActPool serves a single bundle of acts at height 1 and an empty
// pending pool, so a mint draft is built only from that bundle.
func bundleActPool(t *testing.T, acts ...*action.SealedEnvelope) *mock_actpool.MockActPool {
	bp := actpool.NewBundlePool(genesis.TestDefault())
	b := action.NewBundle()
	require.NoError(t, b.Add(acts...))
	b.SetTargetBlockHeight(1)
	require.NoError(t, bp.AddBundle(context.Background(), identityset.Address(28), "bundle", b))
	ap := mock_actpool.NewMockActPool(gomock.NewController(t))
	ap.EXPECT().BundlePool().Return(bp).AnyTimes()
	ap.EXPECT().PendingActionMap().Return(map[string][]*action.SealedEnvelope{}).AnyTimes()
	ap.EXPECT().DeleteAction(gomock.Any()).AnyTimes()
	return ap
}

// TestMint_SkipsBundleWithNonContinuousNonces checks that the mint path leaves
// out a bundle whose sender nonces are not continuous with the pending nonce.
// Block validation enforces nonce continuity over the whole block, so the
// draft must not include such a bundle. The check is backed by account state
// and is backend-independent, so statedb is sufficient here.
func TestMint_SkipsBundleWithNonContinuousNonces(t *testing.T) {
	for _, tc := range []struct {
		name string
		acts func(t *testing.T) []*action.SealedEnvelope
	}{
		{
			// pending nonce is 1, the execution asks for nonce 2: a forward gap.
			name: "forward gap",
			acts: func(t *testing.T) []*action.SealedEnvelope {
				tx := signFrom(t, 26, 2, 1, 100000, func(b *action.EnvelopeBuilder) *action.EnvelopeBuilder {
					return b.SetAction(action.NewExecution(identityset.Address(29).String(), big.NewInt(0), nil))
				})
				return []*action.SealedEnvelope{tx}
			},
		},
		{
			// the same nonce twice is not continuous either.
			name: "duplicate nonce",
			acts: func(t *testing.T) []*action.SealedEnvelope {
				tx := makeTransferFrom(t, 26, 1, 1)
				return []*action.SealedEnvelope{tx, tx}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require := require.New(t)
			proposer, validator, ctx := newPoisonMintEnv(t, &poisonProtocol{bad: map[string]bool{}}, false, context.Background(), withExecution)
			ap := bundleActPool(t, tc.acts(t)...)
			blk, err := proposer.Mint(ctx, ap, identityset.PrivateKey(27))
			require.NoError(err)
			require.Empty(blk.Actions, "a nonce-discontinuous bundle must be left out")
			require.NoError(validator.Validate(ctx, blk), "mint must never emit a nonce-discontinuous block")
		})
	}
}
