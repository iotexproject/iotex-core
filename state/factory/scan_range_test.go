// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
)

// collect drains an iterator into an ordered list of (key, value) pairs.
func collectStates(t *testing.T, store workingSetStore, ns string, scan *db.RangeScan) ([]string, []string) {
	r := require.New(t)
	iter, err := store.States(ns, nil, nil, scan)
	r.NoError(err)
	var keys, values []string
	for {
		var v valueBytes
		k, err := iter.Next(&v)
		if err != nil {
			break
		}
		keys = append(keys, string(k))
		values = append(values, string(v))
	}
	return keys, values
}

func TestWorkingSetStoreScanRange(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	base := db.NewMemKVStore()
	flusher, err := db.NewKVStoreFlusher(base, batch.NewCachedBatch())
	r.NoError(err)
	store := newStateDBWorkingSetStore(flusher, true)
	r.NoError(store.Start(ctx))

	ns := "scanns"
	// half of the data committed to the base store, half pending in the buffer
	for _, k := range []string{"a", "c", "e"} {
		v := valueBytes("base-" + k)
		r.NoError(base.Put(ns, []byte(k), []byte(v)))
	}
	for _, k := range []string{"b", "d", "f"} {
		v := valueBytes("buf-" + k)
		r.NoError(store.PutObject(ns, []byte(k), &v))
	}
	// a buffered overwrite and a buffered delete on top of the base
	overwrite := valueBytes("buf-a")
	r.NoError(store.PutObject(ns, []byte("a"), &overwrite))
	r.NoError(store.DeleteObject(ns, []byte("c"), nil))

	t.Run("full range", func(t *testing.T) {
		keys, values := collectStates(t, store, ns, &db.RangeScan{})
		require.Equal(t, []string{"a", "b", "d", "e", "f"}, keys)
		require.Equal(t, []string{"buf-a", "buf-b", "buf-d", "base-e", "buf-f"}, values)
	})
	t.Run("half-open interval excludes max", func(t *testing.T) {
		keys, _ := collectStates(t, store, ns, &db.RangeScan{Min: []byte("b"), Max: []byte("e")})
		require.Equal(t, []string{"b", "d"}, keys)
	})
	t.Run("limit is applied after the merge", func(t *testing.T) {
		// "b" only exists in the buffer and sorts second; a limit of 2 must return
		// a,b -- if the limit were pushed into the base scan we would get a,c or a,e
		keys, _ := collectStates(t, store, ns, &db.RangeScan{Limit: 2})
		require.Equal(t, []string{"a", "b"}, keys)
	})
	t.Run("empty range is not an error", func(t *testing.T) {
		iter, err := store.States(ns, nil, nil, &db.RangeScan{Min: []byte("x"), Max: []byte("z")})
		require.NoError(t, err)
		require.Equal(t, 0, iter.Size())
	})
	t.Run("missing namespace is not an error", func(t *testing.T) {
		iter, err := store.States("nosuchns", nil, nil, &db.RangeScan{})
		require.NoError(t, err)
		require.Equal(t, 0, iter.Size())
	})
	t.Run("nil scan keeps the legacy path", func(t *testing.T) {
		// the legacy path over memKVStore still fails the way it always did,
		// proving the old code path is untouched
		_, err := store.States(ns, nil, nil, nil)
		require.Error(t, err)
	})
}

// testWorkingSetStoreFactory mirrors what (*stateDB).createWorkingSetStore does on
// a non-erigon node: put a fresh flusher on top of whatever KVStore it is handed.
// That is the piece that makes a derived working set stack one store on another.
type testWorkingSetStoreFactory struct{}

func (testWorkingSetStoreFactory) CreateWorkingSetStore(
	_ context.Context, _ uint64, kvStore db.KVStore,
) (workingSetStore, error) {
	flusher, err := db.NewKVStoreFlusher(kvStore, batch.NewCachedBatch())
	if err != nil {
		return nil, err
	}
	// readBuffer = true is what genesis.IsNewfoundland reports at every height a
	// range scan can be requested at
	return newStateDBWorkingSetStore(flusher, true), nil
}

func newTestWorkingSet(t *testing.T, height uint64, kvStore db.KVStore) *workingSet {
	t.Helper()
	f := testWorkingSetStoreFactory{}
	store, err := f.CreateWorkingSetStore(context.Background(), height, kvStore)
	require.NoError(t, err)
	require.NoError(t, store.Start(context.Background()))
	return newWorkingSet(height, protocol.NewViews(), store, f)
}

// scanWorkingSet runs one range query through the public StateReader API.
func scanWorkingSet(t *testing.T, ws *workingSet, ns string, opts ...protocol.StateOption) ([]string, []string) {
	t.Helper()
	_, iter, err := ws.States(append([]protocol.StateOption{protocol.NamespaceOption(ns)}, opts...)...)
	require.NoError(t, err)
	keys := []string{}
	values := []string{}
	for i := 0; i < iter.Size(); i++ {
		var v valueBytes
		k, err := iter.Next(&v)
		require.NoError(t, err)
		keys = append(keys, string(k))
		values = append(values, string(v))
	}
	return keys, values
}

// TestDerivedWorkingSetScanRange pins the proposer/validator equivalence that a
// missing ScanRange on *stateDBWorkingSetStore breaks.
//
// When the state DB lags the chain tip, (*stateDB).Mint derives the next working
// set from the cached parent working set instead of from the DAO, and the parent
// store ends up in the child flusher's base-store slot. A validator replaying the
// same height builds its working set on a committed DAO. Both must answer an
// ordered range scan identically -- if the derived one errors or returns less,
// the proposer and the validators of that block write different state.
func TestDerivedWorkingSetScanRange(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	ns := "scanns"

	// what the DAO holds before block 1
	committed := map[string]string{"a": "h0-a", "c": "h0-c", "e": "h0-e"}
	// what block 1 writes: one new key, one overwrite, one delete
	blk1Puts := map[string]string{"b": "h1-b", "a": "h1-a"}
	blk1Dels := []string{"c"}
	// what block 2 writes, used by the two-level case
	blk2Puts := map[string]string{"d": "h2-d", "e": "h2-e"}

	seed := func(kv db.KVStore, m map[string]string) {
		for k, v := range m {
			r.NoError(kv.Put(ns, []byte(k), []byte(v)))
		}
	}
	apply := func(ws *workingSet, puts map[string]string, dels []string) {
		for k, v := range puts {
			val := valueBytes(v)
			_, err := ws.PutState(&val, protocol.NamespaceOption(ns), protocol.KeyOption([]byte(k)))
			r.NoError(err)
		}
		for _, k := range dels {
			_, err := ws.DelState(protocol.NamespaceOption(ns), protocol.KeyOption([]byte(k)))
			r.NoError(err)
		}
	}

	// ---- proposer side: block 1's working set is finalized but NOT committed,
	// and block 2's working set is derived from it
	parentBase := db.NewMemKVStore()
	seed(parentBase, committed)
	parent := newTestWorkingSet(t, 1, parentBase)
	apply(parent, blk1Puts, blk1Dels)
	// set directly rather than going through finalize(), which only adds the
	// height key in a different namespace and needs a full block context
	parent.finalized = true
	child, err := parent.NewWorkingSet(ctx)
	r.NoError(err)
	r.Equal(uint64(2), child.height)

	// ---- validator side: block 1 is committed, block 2 starts from the DAO
	freshBase := db.NewMemKVStore()
	seed(freshBase, committed)
	seed(freshBase, blk1Puts)
	for _, k := range blk1Dels {
		r.NoError(freshBase.Delete(ns, []byte(k)))
	}
	fresh := newTestWorkingSet(t, 2, freshBase)

	queries := []struct {
		name string
		opts []protocol.StateOption
	}{
		{"whole range", []protocol.StateOption{protocol.RangeOption([]byte("a"), []byte("z"))}},
		{"half-open sub range", []protocol.StateOption{protocol.RangeOption([]byte("b"), []byte("e"))}},
		{"unbounded with limit", []protocol.StateOption{protocol.LimitOption(2)}},
		{"bounded with limit", []protocol.StateOption{
			protocol.RangeOption([]byte("a"), []byte("z")), protocol.LimitOption(3)}},
		{"empty range", []protocol.StateOption{protocol.RangeOption([]byte("x"), []byte("z"))}},
	}
	for _, q := range queries {
		t.Run("derived/"+q.name, func(t *testing.T) {
			wantK, wantV := scanWorkingSet(t, fresh, ns, q.opts...)
			gotK, gotV := scanWorkingSet(t, child, ns, q.opts...)
			require.Equal(t, wantK, gotK)
			require.Equal(t, wantV, gotV)
		})
	}
	// the expectations above are only meaningful if the query actually sees the
	// block-1 writes
	gotK, gotV := scanWorkingSet(t, child, ns, protocol.RangeOption([]byte("a"), []byte("z")))
	r.Equal([]string{"a", "b", "e"}, gotK)
	r.Equal([]string{"h1-a", "h1-b", "h0-e"}, gotV)

	// ---- two-level: Mint can lag by more than one block, so a child can itself
	// be the parent of the next working set
	apply(child, blk2Puts, nil)
	child.finalized = true
	grandchild, err := child.NewWorkingSet(ctx)
	r.NoError(err)
	r.Equal(uint64(3), grandchild.height)

	fresh3Base := db.NewMemKVStore()
	seed(fresh3Base, committed)
	seed(fresh3Base, blk1Puts)
	for _, k := range blk1Dels {
		r.NoError(fresh3Base.Delete(ns, []byte(k)))
	}
	seed(fresh3Base, blk2Puts)
	fresh3 := newTestWorkingSet(t, 3, fresh3Base)

	for _, q := range queries {
		t.Run("grandchild/"+q.name, func(t *testing.T) {
			wantK, wantV := scanWorkingSet(t, fresh3, ns, q.opts...)
			gotK, gotV := scanWorkingSet(t, grandchild, ns, q.opts...)
			require.Equal(t, wantK, gotK)
			require.Equal(t, wantV, gotV)
		})
	}
	gotK, gotV = scanWorkingSet(t, grandchild, ns, protocol.RangeOption([]byte("a"), []byte("z")))
	r.Equal([]string{"a", "b", "d", "e"}, gotK)
	r.Equal([]string{"h1-a", "h1-b", "h2-d", "h2-e"}, gotV)
}

func TestStatesConfigValidation(t *testing.T) {
	r := require.New(t)

	cfg, err := protocol.CreateStateConfig(protocol.RangeOption([]byte("a"), []byte("z")))
	r.NoError(err)
	r.NoError(validateStatesConfig(cfg))
	scan := rangeScanFromConfig(cfg)
	r.NotNil(scan)
	r.Equal([]byte("a"), scan.Min)
	r.Equal([]byte("z"), scan.Max)
	r.Zero(scan.Limit)

	// nil bounds must stay nil (nil = unbounded, empty slice = a real bound)
	cfg, err = protocol.CreateStateConfig(protocol.RangeOption(nil, nil), protocol.LimitOption(7))
	r.NoError(err)
	scan = rangeScanFromConfig(cfg)
	r.NotNil(scan)
	r.Nil(scan.Min)
	r.Nil(scan.Max)
	r.Equal(7, scan.Limit)

	// an empty (non-nil) max is preserved as a real bound
	cfg, err = protocol.CreateStateConfig(protocol.RangeOption(nil, []byte{}))
	r.NoError(err)
	scan = rangeScanFromConfig(cfg)
	r.NotNil(scan)
	r.NotNil(scan.Max)
	r.Empty(scan.Max)

	// no options at all => nil scan => legacy path
	cfg, err = protocol.CreateStateConfig(protocol.NamespaceOption("ns"))
	r.NoError(err)
	r.Nil(rangeScanFromConfig(cfg))
	r.NoError(validateStatesConfig(cfg))

	// Keys alone is fine
	cfg, err = protocol.CreateStateConfig(protocol.KeysOption(func() ([][]byte, error) {
		return [][]byte{[]byte("k")}, nil
	}))
	r.NoError(err)
	r.NoError(validateStatesConfig(cfg))
	r.Nil(rangeScanFromConfig(cfg))

	// Keys combined with Range or Limit is rejected
	for _, opt := range []protocol.StateOption{
		protocol.RangeOption([]byte("a"), nil),
		protocol.RangeOption(nil, []byte("z")),
		protocol.LimitOption(3),
	} {
		cfg, err := protocol.CreateStateConfig(protocol.KeysOption(func() ([][]byte, error) {
			return [][]byte{[]byte("k")}, nil
		}), opt)
		r.NoError(err)
		err = validateStatesConfig(cfg)
		r.Error(err)
		r.Equal(ErrNotSupported, errors.Cause(err))
	}
}
