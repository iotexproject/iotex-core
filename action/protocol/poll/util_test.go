// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package poll

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// TestSnapshotCommissionRates_TolerantToMissingIdentity verifies the
// IIP-59 PutPollResult hook gracefully skips entries where Identity is
// empty or unparseable. Legacy candidates pre-dating the identity field
// have Identity="", and the snapshot helper must not crash on them — it
// simply leaves their CommissionRate at the default zero (legacy
// behavior). The staking-side happy path is exercised end-to-end by the
// integration test in PR 5.
func TestSnapshotCommissionRates_TolerantToMissingIdentity(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManagerWithoutHeightFunc(ctrl)
	sm.EXPECT().Height().Return(uint64(0), nil).AnyTimes()

	candidates := state.CandidateList{
		// Empty Identity — legacy candidate; helper must skip silently.
		{Identity: "", Address: identityset.Address(7).String(), Votes: big.NewInt(100)},
		// Bogus Identity — helper must skip, not error.
		{Identity: "not-an-address", Address: identityset.Address(8).String(), Votes: big.NewInt(100)},
		// Identity that doesn't map to any staking candidate — helper
		// must leave the rate at 0 (no panic, no error).
		{Identity: identityset.Address(9).String(), Address: identityset.Address(8).String(), Votes: big.NewInt(100)},
	}

	r.NoError(snapshotCommissionRates(sm, candidates))
	for i, c := range candidates {
		r.Equal(uint64(0), c.CommissionRate, "candidate %d should be left at the default rate", i)
	}
}
