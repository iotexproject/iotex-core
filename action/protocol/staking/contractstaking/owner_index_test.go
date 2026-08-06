// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"bytes"
	"context"
	"math/big"
	"sort"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// forkCtx builds a context whose FeatureCtx has IIP-59 either off or on, the
// same way the staking package's fork-gate tests do.
func forkCtx(activated bool) context.Context {
	const height = uint64(1)
	g := genesis.TestDefault()
	if activated {
		g.ToBeEnabledBlockHeight = height
	}
	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
	return protocol.WithFeatureCtx(ctx)
}

func testBucket(owner address.Address) *Bucket {
	return &Bucket{
		Candidate:      identityset.Address(30),
		Owner:          owner,
		StakedAmount:   big.NewInt(100),
		StakedDuration: 10,
		CreatedAt:      1,
	}
}

func newTestSM(t *testing.T) *mock_chainmanager.MockStateManager {
	return testdb.NewMockStateManager(gomock.NewController(t))
}

// rawIndex reads the owner index key straight out of the staking namespace,
// bypassing the reader, so a test can tell "empty list" from "no key".
func rawIndex(t *testing.T, sm protocol.StateManager, owner address.Address) (ContractBucketRefs, bool) {
	t.Helper()
	var refs ContractBucketRefs
	_, err := sm.State(&refs,
		protocol.NamespaceOption(state.StakingNamespace),
		protocol.KeyOption(lsdVoterIndexKey(owner)))
	if err != nil {
		require.ErrorIs(t, errors.Cause(err), state.ErrStateNotExist)
		return nil, false
	}
	return refs, true
}

func refIDs(refs ContractBucketRefs) []uint64 {
	out := make([]uint64, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.BucketID)
	}
	return out
}

// TestLSDVoterIndexKeyShape pins the on-disk key layout. The staking namespace
// is shared with native bucket indices, endorsements, poll snapshots and voter
// weights, so both the tag byte and the length are consensus surface.
func TestLSDVoterIndexKeyShape(t *testing.T) {
	r := require.New(t)
	owner := identityset.Address(1)
	key := lsdVoterIndexKey(owner)

	r.Equal(byte(8), LSDVoterIndexPrefix)
	r.Len(key, 21)
	r.Equal(LSDVoterIndexPrefix, key[0])
	r.Equal(owner.Bytes(), key[1:])

	got, ok := ParseLSDVoterIndexKey(key)
	r.True(ok)
	r.Equal(owner.String(), got.String())

	// Anything that is not exactly {tag}||addr(20) must be rejected, so a scan
	// of the shared namespace cannot mistake another record for this one.
	for _, bad := range [][]byte{
		nil,
		{},
		{LSDVoterIndexPrefix},
		append([]byte{2}, owner.Bytes()...), // native _voterIndex
		append([]byte{LSDVoterIndexPrefix}, make([]byte, 40)...), // voter-weight length
	} {
		_, ok := ParseLSDVoterIndexKey(bad)
		r.False(ok, "key %x must not parse as an owner index key", bad)
	}
}

// TestOwnerIndexGatedOnFork is the reason this change is safe to ship ahead of
// activation. Nodes take a new release over days; one that wrote these keys
// early would put state into the staking namespace that the previous release
// does not write, and its state root would diverge from every node still on
// the old binary — a split at deployment time rather than at activation.
func TestOwnerIndexGatedOnFork(t *testing.T) {
	r := require.New(t)
	sm := newTestSM(t)
	cs := NewContractStakingStateManager(sm)
	contract, owner := identityset.Address(20), identityset.Address(1)

	r.NoError(cs.UpsertBucket(forkCtx(false), contract, 7, testBucket(owner)))
	_, exists := rawIndex(t, sm, owner)
	r.False(exists, "pre-fork upsert must not write the owner index")

	// The bucket itself is still written, byte for byte as before.
	bkt, err := cs.Bucket(contract, 7)
	r.NoError(err)
	r.Equal(owner.String(), bkt.Owner.String())

	r.NoError(cs.DeleteBucket(forkCtx(false), contract, 7))
	_, exists = rawIndex(t, sm, owner)
	r.False(exists)

	// A context with no feature context at all (indexer bootstrap, tests)
	// reads as pre-activation.
	r.NoError(cs.UpsertBucket(context.Background(), contract, 8, testBucket(owner)))
	_, exists = rawIndex(t, sm, owner)
	r.False(exists)
}

// TestOwnerIndexCreateTransferDelete walks the full lifecycle through the two
// choke points.
func TestOwnerIndexCreateTransferDelete(t *testing.T) {
	r := require.New(t)
	ctx := forkCtx(true)
	sm := newTestSM(t)
	cs := NewContractStakingStateManager(sm)
	contract := identityset.Address(20)
	alice, bob := identityset.Address(1), identityset.Address(2)

	// create
	r.NoError(cs.UpsertBucket(ctx, contract, 3, testBucket(alice)))
	refs, _, err := cs.BucketRefsByOwner(alice)
	r.NoError(err)
	r.Equal(ContractBucketRefs{{Contract: contract, BucketID: 3}}, refs)

	// re-upsert with the same owner is idempotent
	r.NoError(cs.UpsertBucket(ctx, contract, 3, testBucket(alice)))
	refs, _, err = cs.BucketRefsByOwner(alice)
	r.NoError(err)
	r.Len(refs, 1)

	// transfer owner: the ref moves, it is not duplicated
	r.NoError(cs.UpsertBucket(ctx, contract, 3, testBucket(bob)))
	_, _, err = cs.BucketRefsByOwner(alice)
	r.ErrorIs(err, ErrOwnerIndexNotExist)
	refs, _, err = cs.BucketRefsByOwner(bob)
	r.NoError(err)
	r.Equal(ContractBucketRefs{{Contract: contract, BucketID: 3}}, refs)

	// delete
	r.NoError(cs.DeleteBucket(ctx, contract, 3))
	_, _, err = cs.BucketRefsByOwner(bob)
	r.ErrorIs(err, ErrOwnerIndexNotExist)
}

// TestOwnerIndexLastRefDeletesKey pins that the key disappears rather than
// holding an empty list, which would keep a trie node alive forever and make a
// later namespace scan see a record that means nothing.
func TestOwnerIndexLastRefDeletesKey(t *testing.T) {
	r := require.New(t)
	ctx := forkCtx(true)
	sm := newTestSM(t)
	cs := NewContractStakingStateManager(sm)
	contract, owner := identityset.Address(20), identityset.Address(1)

	r.NoError(cs.UpsertBucket(ctx, contract, 1, testBucket(owner)))
	r.NoError(cs.UpsertBucket(ctx, contract, 2, testBucket(owner)))

	r.NoError(cs.DeleteBucket(ctx, contract, 1))
	refs, exists := rawIndex(t, sm, owner)
	r.True(exists)
	r.Equal([]uint64{2}, refIDs(refs))

	r.NoError(cs.DeleteBucket(ctx, contract, 2))
	_, exists = rawIndex(t, sm, owner)
	r.False(exists, "the last ref must delete the key, not store an empty list")
}

// TestOwnerIndexOrderingDeterminism is the consensus-critical one: the stored
// bytes must be a function of the ref set alone, never of the order the writes
// happened to arrive in or of Go map iteration.
func TestOwnerIndexOrderingDeterminism(t *testing.T) {
	r := require.New(t)
	ctx := forkCtx(true)
	owner := identityset.Address(1)
	c1, c2 := identityset.Address(20), identityset.Address(21)

	type write struct {
		contract address.Address
		id       uint64
	}
	writes := []write{{c1, 5}, {c2, 1}, {c1, 2}, {c2, 9}, {c1, 100}}

	build := func(order []int) []byte {
		sm := newTestSM(t)
		cs := NewContractStakingStateManager(sm)
		for _, i := range order {
			r.NoError(cs.UpsertBucket(ctx, writes[i].contract, writes[i].id, testBucket(owner)))
		}
		refs, exists := rawIndex(t, sm, owner)
		r.True(exists)
		b, err := refs.Serialize()
		r.NoError(err)
		return b
	}

	forward := build([]int{0, 1, 2, 3, 4})
	reverse := build([]int{4, 3, 2, 1, 0})
	shuffled := build([]int{2, 0, 4, 1, 3})
	r.Equal(forward, reverse)
	r.Equal(forward, shuffled)

	// And the order itself is (contract bytes, bucket id) ascending.
	var refs ContractBucketRefs
	r.NoError(refs.Deserialize(forward))
	r.True(sort.SliceIsSorted(refs, func(i, j int) bool {
		return compareRef(refs[i], refs[j]) < 0
	}))
	// c1 vs c2 ordering follows the raw address bytes, not the write order.
	if bytes.Compare(c1.Bytes(), c2.Bytes()) < 0 {
		r.Equal(c1.String(), refs[0].Contract.String())
	} else {
		r.Equal(c2.String(), refs[0].Contract.String())
	}
}

// TestOwnerIndexMultiContract covers the reason a ref needs both fields: the
// same bucket id in two contracts is two different buckets.
func TestOwnerIndexMultiContract(t *testing.T) {
	r := require.New(t)
	ctx := forkCtx(true)
	sm := newTestSM(t)
	cs := NewContractStakingStateManager(sm)
	owner := identityset.Address(1)
	c1, c2 := identityset.Address(20), identityset.Address(21)

	r.NoError(cs.UpsertBucket(ctx, c1, 4, testBucket(owner)))
	r.NoError(cs.UpsertBucket(ctx, c2, 4, testBucket(owner)))

	refs, _, err := cs.BucketRefsByOwner(owner)
	r.NoError(err)
	r.Len(refs, 2)

	// Deleting bucket 4 of c1 must leave bucket 4 of c2 alone.
	r.NoError(cs.DeleteBucket(ctx, c1, 4))
	refs, _, err = cs.BucketRefsByOwner(owner)
	r.NoError(err)
	r.Equal(ContractBucketRefs{{Contract: c2, BucketID: 4}}, refs)
}

// TestContractBucketRefsSerializeRoundTrip covers the value encoding on its own.
func TestContractBucketRefsSerializeRoundTrip(t *testing.T) {
	r := require.New(t)
	refs := ContractBucketRefs{
		{Contract: identityset.Address(21), BucketID: 9},
		{Contract: identityset.Address(20), BucketID: 1},
	}
	b, err := refs.Serialize()
	r.NoError(err)

	var out ContractBucketRefs
	r.NoError(out.Deserialize(b))
	// Serialize sorts, so the round trip normalises a hand-built list.
	r.True(sort.SliceIsSorted(out, func(i, j int) bool { return compareRef(out[i], out[j]) < 0 }))
	r.Len(out, 2)

	gv, err := refs.Encode()
	r.NoError(err)
	var out2 ContractBucketRefs
	r.NoError(out2.Decode(gv))
	r.Equal(out, out2)

	var empty ContractBucketRefs
	b, err = empty.Serialize()
	r.NoError(err)
	var outEmpty ContractBucketRefs
	r.NoError(outEmpty.Deserialize(b))
	r.Empty(outEmpty)

	r.Error(new(ContractBucketRefs).Deserialize([]byte{0xff, 0xff, 0xff}))
}

// TestOwnerIndexUpsertRejectsOwnerlessBucket: a bucket with no owner cannot be
// indexed, and silently skipping it would leave the index permanently short.
func TestOwnerIndexUpsertRejectsOwnerlessBucket(t *testing.T) {
	r := require.New(t)
	cs := NewContractStakingStateManager(newTestSM(t))
	bkt := testBucket(identityset.Address(1))
	bkt.Owner = nil
	r.ErrorContains(cs.UpsertBucket(forkCtx(true), identityset.Address(20), 1, bkt), "no owner")
}

// TestOwnerIndexDeleteMissingBucket: deleting a bucket that was never in state
// touches nothing and is not an error, matching the pre-existing DeleteBucket
// behaviour.
func TestOwnerIndexDeleteMissingBucket(t *testing.T) {
	r := require.New(t)
	sm := newTestSM(t)
	cs := NewContractStakingStateManager(sm)
	owner := identityset.Address(1)
	contract := identityset.Address(20)

	r.NoError(cs.UpsertBucket(forkCtx(true), contract, 1, testBucket(owner)))
	r.NoError(cs.DeleteBucket(forkCtx(true), contract, 99))
	refs, exists := rawIndex(t, sm, owner)
	r.True(exists)
	r.Equal([]uint64{1}, refIDs(refs))
}

// TestBucketRefsByOwnerUnknown pins the "no key" error shape.
func TestBucketRefsByOwnerUnknown(t *testing.T) {
	r := require.New(t)
	cs := NewContractStakingStateManager(newTestSM(t))
	refs, _, err := cs.BucketRefsByOwner(identityset.Address(1))
	r.ErrorIs(err, ErrOwnerIndexNotExist)
	r.Nil(refs)
}
