package erigonstore

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
	"github.com/iotexproject/iotex-core/v2/state"
)

// TestVaultBucketErigon exercises the actual system-contract storage, commit
// and historical reads. GenericValue Encode/Decode alone cannot detect a
// missing registration or a value lost during FinalizeTx.
func TestVaultBucketErigon(t *testing.T) {
	r := require.New(t)
	ctx := genesis.WithGenesisContext(context.Background(), genesis.TestDefault())
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{})
	at := func(h uint64) context.Context {
		return protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: h}))
	}
	edb := NewErigonDB(t.TempDir())
	r.NoError(edb.Start(ctx))
	t.Cleanup(func() { edb.Stop(ctx) })
	s, err := edb.NewErigonStore(at(0), 0)
	r.NoError(err)
	r.NoError(s.CreateGenesisStates(at(0)))
	r.NoError(s.FinalizeTx(at(0)))
	r.NoError(s.Commit(at(0), 0))

	vault := &staking.VaultBucketRecord{VaultBucket: bucketguard.VaultBucket{VaultIndex: 0, Owner: bytes.Repeat([]byte{1}, 20), Amount: []byte{100}}}
	guard := &staking.BucketGuardV2Record{BucketGuardV2: bucketguard.BucketGuardV2{AlgID: pqcrypto.AlgSLHDSA192s, PublicKey: bytes.Repeat([]byte{2}, 48), BucketClass: bucketguard.ClassAVault, Generation: 1, NextRequestID: 1, BoundHeight: 1, UnbindDelay: bucketguard.UnbindDelayBlocks}}
	meta := &staking.VaultMetaRecord{TotalVaultCount: 1}
	type entry struct {
		key   []byte
		value any
		fresh func() any
	}
	entries := []entry{
		{make([]byte, 9), vault, func() any { return &staking.VaultBucketRecord{} }},
		{make([]byte, 9), guard, func() any { return &staking.BucketGuardV2Record{} }},
		{[]byte{12}, meta, func() any { return &staking.VaultMetaRecord{} }},
	}
	entries[0].key[0], entries[1].key[0] = 10, 11
	for _, e := range entries {
		index, ok := GetObjectStorageRegistry().matchContractIndex(state.StakingNamespace, e.value)
		r.True(ok)
		r.Equal(BucketPoolContractIndex, index)
		_, explicit := GetObjectStorageRegistry().contracts[state.StakingNamespace][reflect.TypeOf(e.value)]
		r.True(explicit, "must register explicitly")
	}

	s, err = edb.NewErigonStore(at(1), 1)
	r.NoError(err)
	for _, e := range entries {
		r.NoError(s.PutObject(state.StakingNamespace, e.key, e.value))
	}
	r.NoError(s.FinalizeTx(at(1)))
	r.NoError(s.Commit(at(1), 0))

	// Close the vault at height 2. Its permanent tombstone is an actual V2
	// record with no public key; the count must survive the deletion.
	s, err = edb.NewErigonStore(at(2), 2)
	r.NoError(err)
	r.NoError(s.DeleteObject(state.StakingNamespace, entries[0].key, &staking.VaultBucketRecord{}))
	closed := &staking.BucketGuardV2Record{BucketGuardV2: bucketguard.BucketGuardV2{BucketClass: bucketguard.ClassAVault, Generation: 2, GuardNonce: 1, NextRequestID: 1, BoundHeight: 2, UnbindDelay: bucketguard.UnbindDelayBlocks}}
	r.NoError(s.PutObject(state.StakingNamespace, entries[1].key, closed))
	r.NoError(s.FinalizeTx(at(2)))
	r.NoError(s.Commit(at(2), 0))
	for _, h := range []uint64{1, 2} {
		read, err := edb.NewErigonStoreDryrun(at(h+1), h+1)
		r.NoError(err)
		for i, e := range entries {
			got := e.fresh()
			err := read.GetObject(state.StakingNamespace, e.key, got)
			if h == 2 && i == 0 {
				r.ErrorIs(err, state.ErrStateNotExist)
				continue
			}
			r.NoError(err)
			want := e.value
			if h == 2 && i == 1 {
				want = closed
			}
			r.Equal(want, got)
		}
		read.Close()
	}
}
