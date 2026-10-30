// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// TestCoveredKeyAddresses pins the state address of every key the IIP-59 era
// copy-on-write layer covers.
//
// These four addresses are consensus surface twice over: they name where the
// live value is written, and where eracow.Resolve looks when there is no copy.
// Changing one silently re-homes existing state — and because a frozen read
// that misses is skipped rather than failed (FrozenVoterWeight's `continue`),
// the symptom of getting it wrong is an under-payment on a chain that keeps
// producing blocks. The expected values below are therefore written out by hand
// rather than derived, so this test disagrees with the code rather than
// agreeing with it by construction.
func TestCoveredKeyAddresses(t *testing.T) {
	addr := identityset.Address(1)
	contract := identityset.Address(2)

	// The live namespace string, spelled out. state.StakingNamespace and
	// _stakingNameSpace are the same namespace under two names, which is
	// precisely why neither is used here.
	const stakingNS = "Staking"

	addrOf := func(t *testing.T, opts []protocol.StateOption) (string, []byte) {
		t.Helper()
		cfg, err := protocol.CreateStateConfig(opts...)
		require.NoError(t, err)
		return cfg.Namespace, cfg.Key
	}

	t.Run("native bucket", func(t *testing.T) {
		r := require.New(t)
		ns, key := addrOf(t, nativeBucketStateOpts(7))
		r.Equal(stakingNS, ns)
		// {_bucket} || big-endian index(8)
		r.Equal([]byte{1, 0, 0, 0, 0, 0, 0, 0, 7}, key)
	})

	t.Run("native voter index", func(t *testing.T) {
		r := require.New(t)
		ns, key := addrOf(t, nativeBucketIndexStateOpts(addr, _voterIndex))
		r.Equal(stakingNS, ns)
		r.Len(key, 21)
		r.Equal(byte(2), key[0])
		r.Equal(addr.Bytes(), key[1:])
	})

	t.Run("native candidate index shares the shape", func(t *testing.T) {
		r := require.New(t)
		_, key := addrOf(t, nativeBucketIndexStateOpts(addr, _candIndex))
		r.Len(key, 21)
		r.Equal(byte(3), key[0])
		r.Equal(addr.Bytes(), key[1:])
	})

	t.Run("contract bucket", func(t *testing.T) {
		r := require.New(t)
		// nil state reader: these constructors compute an address and touch no
		// state, which is what lets the frozen reads borrow them.
		ns, key := addrOf(t, contractstaking.NewStateReader(nil).BucketStateOpts(contract, 7))
		r.Equal("cs_bucket_"+hexOf(contract.Bytes()), ns)
		// little-endian, unlike the native bucket key above. The asymmetry is
		// existing state, not a mistake to fix.
		r.Equal([]byte{7, 0, 0, 0, 0, 0, 0, 0}, key)
	})

	t.Run("contract owner index", func(t *testing.T) {
		r := require.New(t)
		ns, key := addrOf(t, contractstaking.NewStateReader(nil).OwnerIndexStateOpts(addr))
		r.Equal(stakingNS, ns)
		r.Len(key, 21)
		r.Equal(byte(6), key[0])
		r.Equal(addr.Bytes(), key[1:])
	})

	// That the frozen reads use these same constructors is not asserted here —
	// it is a compile-time fact (frozen_voter_weight.go calls them) reinforced by
	// the live-write/frozen-read round trips in era_cow_window_test.go.
}

// TestCoveredKeyAccessorsAddNoReadBehavior pins the other half of what
// TestCoveredKeyAddresses starts: that reading a covered key through its live
// accessor and reading it the way eracow.Resolve does — a bare State() at the
// shared address — yield the same bytes.
//
// # Why this is a test and not a call
//
// The obvious alternative is to have Resolve call csm.NativeBucket /
// csr.Bucket outright. It cannot, for two reasons, and the second is the
// load-bearing one:
//
//   - eracow cannot import staking, because staking imports eracow. The native
//     accessor is out of reach by construction.
//   - Resolve has three branches, and the copy branch physically cannot go
//     through an accessor. The as-of-H value lives in eracow's own namespace
//     under an era-tagged key and comes back through state.Deserialize; it is
//     not at the bucket's address at all. Routing only the live branch through
//     the accessor would make a bucket decode one way if someone happened to
//     touch it during the era and another way if nobody did — a
//     consensus-visible asymmetry, worse than the divergence it would fix.
//
// So the invariant is not "Resolve calls the accessor". It is:
//
//	Read-time behavior on a covered key belongs in Deserialize, which both
//	branches share — never in the accessor, which only the live branch has.
//
// The accessors honor that today: each is an address, a State(), and an error
// mapping the frozen readers deliberately do not want (they switch on the raw
// state.ErrStateNotExist). This test goes red the moment one of them grows
// anything more — a format migration, a cache, a field backfill. Without it the
// symptom is a frozen read returning a stale-format value, a wrong voter
// weight, and a block that still validates.
func TestCoveredKeyAccessorsAddNoReadBehavior(t *testing.T) {
	// The fork gate is open so the contract-staking owner index is maintained
	// (contractstaking.OwnerIndexEnabled), but no era window is opened: with no
	// copies in play both reads take the live leg, which is precisely the pair
	// Resolve's fallback has to agree with.
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	csm := eraTestCSM(ctx, sm)
	voter := identityset.Address(2)
	contract := identityset.Address(20)

	index, err := csm.putBucketAndIndex(eraTestBucket(t, 2, 1, 100))
	require.NoError(t, err)

	cs := contractstaking.NewContractStakingStateManager(sm)
	require.NoError(t, cs.UpsertBucket(ctx, contract, 3, eraTestContractBucket(2, 1, 100)))

	// The contract-side reader is built with no options, matching
	// contractstaking.ContractStakingStateReader — that is the reader Resolve's
	// address actually comes from.
	csr := contractstaking.NewStateReader(sm)

	t.Run("native bucket", func(t *testing.T) {
		r := require.New(t)
		viaAccessor, err := csm.NativeBucket(index)
		r.NoError(err)

		var viaResolve VoteBucket
		_, err = sm.State(&viaResolve, nativeBucketStateOpts(index)...)
		r.NoError(err)

		r.Equal(rawOf(t, viaAccessor), rawOf(t, &viaResolve))
	})

	t.Run("native voter index", func(t *testing.T) {
		r := require.New(t)
		viaAccessor, _, err := newCandidateStateReader(sm).NativeBucketIndices(voter, _voterIndex)
		r.NoError(err)

		var viaResolve BucketIndices
		_, err = sm.State(&viaResolve, nativeBucketIndexStateOpts(voter, _voterIndex)...)
		r.NoError(err)

		r.Equal(rawOf(t, viaAccessor), rawOf(t, &viaResolve))
	})

	t.Run("contract bucket", func(t *testing.T) {
		r := require.New(t)
		viaAccessor, err := cs.Bucket(contract, 3)
		r.NoError(err)

		var viaResolve contractstaking.Bucket
		_, err = sm.State(&viaResolve, csr.BucketStateOpts(contract, 3)...)
		r.NoError(err)

		r.Equal(rawOf(t, viaAccessor), rawOf(t, &viaResolve))
	})

	t.Run("contract owner index", func(t *testing.T) {
		r := require.New(t)
		viaAccessor, _, err := cs.BucketRefsByOwner(voter)
		r.NoError(err)
		r.NotEmpty(viaAccessor, "the upsert above must have indexed the owner, or this compares two empty lists")

		var viaResolve contractstaking.ContractBucketRefs
		_, err = sm.State(&viaResolve, csr.OwnerIndexStateOpts(voter)...)
		r.NoError(err)

		r.Equal(rawOf(t, &viaAccessor), rawOf(t, &viaResolve))
	})
}

// rawOf is the comparison the covered keys are actually judged on: the stored
// form. Comparing decoded structs would let a difference in an unexported or
// derived field slip past reflect.DeepEqual's idea of equality.
func rawOf(t *testing.T, s state.Serializer) []byte {
	t.Helper()
	b, err := s.Serialize()
	require.NoError(t, err)
	return b
}

// hexOf renders bytes the way the contract bucket namespace does (%x).
func hexOf(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, len(b)*2)
	for _, c := range b {
		out = append(out, digits[c>>4], digits[c&0xf])
	}
	return string(out)
}
