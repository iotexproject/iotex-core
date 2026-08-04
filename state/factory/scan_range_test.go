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
