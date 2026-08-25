// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// TestHandleCandidateUpdate_DuplicateBLSKeyIsDeterministic is the receipt-level
// regression guard for the fork that duplicate BLS keys used to cause.
//
// Two candidates holding one pubkey is reachable: nothing forbids it before the
// uniqueness rule activates, and it can be arranged deliberately ahead of the
// fork. When one of those two then touches the key, the handler has to decide
// between Success and ErrCandidateConflict. That decision used to be made by
// naming "the" holder — the first match in a Go map walk — so the caller
// sometimes found itself and sometimes found the other holder. Nodes wrote
// different receipt statuses for the same block, and therefore different
// receipt roots.
//
// The status must now be the same on every node and in every round.
func TestHandleCandidateUpdate_DuplicateBLSKeyIsDeterministic(t *testing.T) {
	require := require.New(t)

	sk := blsKeyFromSeed(t, "duplicate-holder")
	pk := sk.PublicKey().Bytes()

	// Enough rounds that a map-order dependency would show up: the pre-fix
	// implementation split 176/24 over 200 draws.
	const rounds = 60
	statuses := map[uint64]int{}

	for i := 0; i < rounds; i++ {
		func() {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			sm, p, candA, candB := initAll(t, ctrl)

			// Put both candidates on the same pubkey, the pre-fork state the
			// uniqueness rule was never able to prevent.
			csm, err := NewCandidateStateManager(sm)
			require.NoError(err)
			a, b := candA.Clone(), candB.Clone()
			a.BLSPubKey, b.BLSPubKey = pk, pk
			require.NoError(csm.Upsert(a))
			require.NoError(csm.Upsert(b))
			require.NoError(csm.Commit(buildHandlerCtx(a.Owner, true, 1)))

			// candA re-asserts the key it already holds. Whether that is
			// "carrying my own key forward" or "claiming someone else's"
			// is exactly the question that used to depend on map order.
			pop, err := SignBLSPop(sk, a.GetIdentifier())
			require.NoError(err)
			cu, err := action.NewCandidateUpdateWithBLS(
				a.Name,
				identityset.Address(28).String(),
				identityset.Address(29).String(),
				pk, pop,
			)
			require.NoError(err)

			require.NoError(setupAccount(sm, a.Owner, 100_000_000))
			elp := builder.SetNonce(1).SetGasLimit(1_000_000).
				SetGasPrice(testGasPrice).SetAction(cu).Build()
			ctx := buildHandlerCtx(a.Owner, true, 1)
			require.NoError(p.Validate(ctx, elp, sm))
			r, err := p.Handle(ctx, elp, sm)
			require.NoError(err)
			require.NotNil(r)
			statuses[r.Status]++
		}()
	}

	require.Len(statuses, 1,
		"receipt status must not depend on map iteration order; got %v", statuses)
	require.Equal(rounds, statuses[uint64(iotextypes.ReceiptStatus_ErrCandidateConflict)],
		"a key held by another candidate must always be a conflict, got %v", statuses)
}
