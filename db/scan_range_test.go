// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package db

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/db/batch"
)

// The whole point of this file: bolt and pebble are both in production (see
// CreateKVStore / DBAuto), so a ScanRange that answers differently on the two
// engines is a chain fork. Every implementation of KVStoreWithRangeScan is fed the
// identical op sequence and then hammered with identical queries, and all of them
// must agree byte for byte -- with each other AND with an independent in-test
// model, so that "all engines wrong the same way" is also caught.

const (
	_scanSeed = 20240917 // fixed so a failure reproduces exactly

	_scanNsMain    = "ScanMain"    // populated
	_scanNsEmptied = "ScanEmptied" // written then fully deleted
	_scanNsMissing = "ScanMissing" // never touched
)

type scanOp struct {
	del   bool
	ns    string
	k, v  []byte
	index int
}

// scanTarget is one store under test.
type scanTarget struct {
	name string
	kv   KVStoreWithRangeScan
	// apply performs the op against this target
	apply func(op scanOp) error
	stop  func()
}

// buildScanKeys produces keys of differing lengths over a tiny alphabet so that
// prefix relationships ("a" vs "ab" vs "abc") occur densely.
func buildScanKeys() [][]byte {
	alphabet := []byte{0x00, 0x01, 0x41, 0x7f, 0x80, 0xfe, 0xff}
	var keys [][]byte
	for _, a := range alphabet {
		keys = append(keys, []byte{a})
		for _, b := range alphabet {
			keys = append(keys, []byte{a, b})
			keys = append(keys, []byte{a, b, 0x2e}) // '.' -- the memKVStore delimiter
		}
	}
	// explicit prefix chain
	keys = append(keys, []byte("a"), []byte("ab"), []byte("abc"), []byte("abcd"))
	return keys
}

func genScanOps(rnd *rand.Rand, keys [][]byte, n int) []scanOp {
	namespaces := []string{_scanNsMain, _scanNsMain, _scanNsMain, _scanNsEmptied}
	ops := make([]scanOp, 0, n)
	for i := 0; i < n; i++ {
		ns := namespaces[rnd.Intn(len(namespaces))]
		k := keys[rnd.Intn(len(keys))]
		op := scanOp{ns: ns, k: k, index: i}
		switch {
		case rnd.Intn(4) == 0:
			op.del = true
		case rnd.Intn(20) == 0:
			op.v = []byte{} // empty value: engines must agree on its representation
		default:
			op.v = []byte(fmt.Sprintf("v-%d-%x", i, k))
		}
		ops = append(ops, op)
	}
	// make sure _scanNsEmptied really ends up empty. these must keep increasing
	// indices, otherwise the buffered targets would replay them into the base store
	// underneath pending buffer writes for the same keys.
	for i, k := range keys {
		ops = append(ops, scanOp{del: true, ns: _scanNsEmptied, k: k, index: n + i})
	}
	return ops
}

// model is the independent reference implementation.
func modelScan(state map[string]map[string][]byte, ns string, min, max []byte, limit int) ([][]byte, [][]byte) {
	if max != nil && bytes.Compare(min, max) >= 0 {
		return nil, nil
	}
	m, ok := state[ns]
	if !ok {
		return nil, nil
	}
	var keys []string
	for k := range m {
		kb := []byte(k)
		if min != nil && bytes.Compare(kb, min) < 0 {
			continue
		}
		if max != nil && bytes.Compare(kb, max) >= 0 {
			continue
		}
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	if limit > 0 && limit < len(keys) {
		keys = keys[:limit]
	}
	if len(keys) == 0 {
		return nil, nil
	}
	rk := make([][]byte, 0, len(keys))
	rv := make([][]byte, 0, len(keys))
	for _, k := range keys {
		rk = append(rk, []byte(k))
		rv = append(rv, copyBytes(m[k]))
	}
	return rk, rv
}

func applyToModel(state map[string]map[string][]byte, op scanOp) {
	m, ok := state[op.ns]
	if !ok {
		m = map[string][]byte{}
		state[op.ns] = m
	}
	if op.del {
		delete(m, string(op.k))
		return
	}
	m[string(op.k)] = copyBytes(op.v)
}

func kvApply(kv KVStore) func(scanOp) error {
	return func(op scanOp) error {
		if op.del {
			return kv.Delete(op.ns, op.k)
		}
		return kv.Put(op.ns, op.k, op.v)
	}
}

func newScanTargets(t *testing.T, ctx context.Context, splitAt int) []*scanTarget {
	r := require.New(t)
	dir := t.TempDir()

	// bolt
	boltCfg := DefaultConfig
	boltCfg.DbPath = filepath.Join(dir, "scan.bolt")
	bolt := NewBoltDB(boltCfg)
	r.NoError(bolt.Start(ctx))

	// pebble
	pebbleCfg := DefaultConfig
	pebbleCfg.DbPath = filepath.Join(dir, "scan.pebble")
	pebble := NewPebbleDB(pebbleCfg)
	r.NoError(pebble.Start(ctx))

	// in-memory
	mem := NewMemKVStore().(KVStoreWithRangeScan)

	targets := []*scanTarget{
		{name: "bolt", kv: bolt, apply: kvApply(bolt), stop: func() { r.NoError(bolt.Stop(ctx)) }},
		{name: "pebble", kv: pebble, apply: kvApply(pebble), stop: func() { r.NoError(pebble.Stop(ctx)) }},
		{name: "mem", kv: mem, apply: kvApply(mem), stop: func() {}},
	}

	// kvStoreWithBuffer over each engine: the first splitAt ops are committed to the
	// base store, the rest stay pending in the buffer, so every query exercises the
	// merge of base + buffer.
	bufferBolt := func() KVStore {
		cfg := DefaultConfig
		cfg.DbPath = filepath.Join(dir, "buf.bolt")
		b := NewBoltDB(cfg)
		r.NoError(b.Start(ctx))
		return b
	}()
	bufferPebble := func() KVStore {
		cfg := DefaultConfig
		cfg.DbPath = filepath.Join(dir, "buf.pebble")
		p := NewPebbleDB(cfg)
		r.NoError(p.Start(ctx))
		return p
	}()
	bases := []struct {
		name    string
		kv      KVStore
		started bool
	}{
		{"buffer-over-mem", NewMemKVStore(), false},
		{"buffer-over-bolt", bufferBolt, true},
		{"buffer-over-pebble", bufferPebble, true},
	}
	for i := range bases {
		base := bases[i]
		flusher, err := NewKVStoreFlusher(base.kv, batch.NewCachedBatch())
		r.NoError(err)
		buffered := flusher.KVStoreWithBuffer()
		scanner, ok := buffered.(KVStoreWithRangeScan)
		r.True(ok, "kvStoreWithBuffer must implement KVStoreWithRangeScan")
		applyBase, applyBuffer := kvApply(base.kv), kvApply(buffered)
		stop := func() {}
		if base.started {
			kv := base.kv
			stop = func() { r.NoError(kv.Stop(ctx)) }
		}
		targets = append(targets, &scanTarget{
			name: base.name,
			kv:   scanner,
			apply: func(op scanOp) error {
				if op.index < splitAt {
					return applyBase(op)
				}
				return applyBuffer(op)
			},
			stop: stop,
		})
	}
	return targets
}

func TestScanRangeDifferential(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	rnd := rand.New(rand.NewSource(_scanSeed))

	keys := buildScanKeys()
	ops := genScanOps(rnd, keys, 400)
	// split point for the buffered targets: base gets the first half, buffer the rest
	splitAt := len(ops) / 2

	targets := newScanTargets(t, ctx, splitAt)
	defer func() {
		for _, tgt := range targets {
			tgt.stop()
		}
	}()

	model := map[string]map[string][]byte{}
	for _, op := range ops {
		applyToModel(model, op)
		for _, tgt := range targets {
			r.NoError(tgt.apply(op), "target %s failed to apply op %d", tgt.name, op.index)
		}
	}

	// queries: explicit edge cases first, then randomized
	type query struct {
		desc     string
		ns       string
		min, max []byte
		limit    int
	}
	sortedKeys := make([][]byte, len(keys))
	copy(sortedKeys, keys)
	sort.Slice(sortedKeys, func(i, j int) bool { return bytes.Compare(sortedKeys[i], sortedKeys[j]) < 0 })

	queries := []query{
		{"full scan", _scanNsMain, nil, nil, 0},
		{"min==nil", _scanNsMain, nil, []byte{0x80}, 0},
		{"max==nil", _scanNsMain, []byte{0x41}, nil, 0},
		{"min==max", _scanNsMain, []byte{0x41}, []byte{0x41}, 0},
		{"min>max", _scanNsMain, []byte{0xfe}, []byte{0x01}, 0},
		{"empty range between keys", _scanNsMain, []byte{0x41, 0x00, 0x01}, []byte{0x41, 0x00, 0x02}, 0},
		{"limit==1", _scanNsMain, nil, nil, 1},
		{"limit huge", _scanNsMain, nil, nil, 1 << 20},
		{"limit negative means unlimited", _scanNsMain, nil, nil, -5},
		{"prefix chain", _scanNsMain, []byte("a"), []byte("abcd"), 0},
		{"prefix chain inclusive of max-1", _scanNsMain, []byte("ab"), nil, 3},
		{"exclusive upper bound", _scanNsMain, []byte{0xff}, nil, 0},
		{"max is empty slice", _scanNsMain, nil, []byte{}, 0},
		{"namespace with no keys", _scanNsEmptied, nil, nil, 0},
		{"namespace with no keys, bounded", _scanNsEmptied, []byte{0x00}, []byte{0xff}, 3},
		{"namespace that does not exist", _scanNsMissing, nil, nil, 0},
		{"namespace that does not exist, bounded", _scanNsMissing, []byte{0x00}, []byte{0xff}, 2},
	}
	// tile the whole keyspace with adjacent half-open ranges and make sure the tiles
	// reassemble into exactly the full scan (this is what half-open buys us)
	for i := 0; i+1 < len(sortedKeys); i += 7 {
		j := i + 7
		if j >= len(sortedKeys) {
			j = len(sortedKeys) - 1
		}
		queries = append(queries, query{
			desc: fmt.Sprintf("tile [%x,%x)", sortedKeys[i], sortedKeys[j]),
			ns:   _scanNsMain, min: sortedKeys[i], max: sortedKeys[j],
		})
	}
	// randomized
	pickBound := func() []byte {
		switch rnd.Intn(5) {
		case 0:
			return nil
		case 1:
			return sortedKeys[rnd.Intn(len(sortedKeys))]
		default:
			n := 1 + rnd.Intn(3)
			b := make([]byte, n)
			for i := range b {
				b[i] = byte(rnd.Intn(256))
			}
			return b
		}
	}
	for i := 0; i < 500; i++ {
		limit := 0
		switch rnd.Intn(3) {
		case 0:
			limit = 1 + rnd.Intn(5)
		case 1:
			limit = 1 + rnd.Intn(200)
		}
		queries = append(queries, query{
			desc: fmt.Sprintf("random #%d", i),
			ns:   []string{_scanNsMain, _scanNsEmptied, _scanNsMissing}[rnd.Intn(3)],
			min:  pickBound(), max: pickBound(), limit: limit,
		})
	}

	for _, q := range queries {
		wantK, wantV := modelScan(model, q.ns, q.min, q.max, q.limit)
		for _, tgt := range targets {
			gotK, gotV, err := tgt.kv.ScanRange(q.ns, q.min, q.max, q.limit)
			r.NoError(err, "%s: %s", tgt.name, q.desc)
			r.Equal(wantK, gotK, "%s: %s keys (min=%x max=%x limit=%d)", tgt.name, q.desc, q.min, q.max, q.limit)
			r.Equal(wantV, gotV, "%s: %s values (min=%x max=%x limit=%d)", tgt.name, q.desc, q.min, q.max, q.limit)
			// ordering invariant
			for i := 1; i < len(gotK); i++ {
				r.Negative(bytes.Compare(gotK[i-1], gotK[i]), "%s: %s not ascending", tgt.name, q.desc)
			}
			// an empty result is (nil, nil, nil), never ErrNotExist
			if len(wantK) == 0 {
				r.Nil(gotK, "%s: %s must return nil keys", tgt.name, q.desc)
				r.Nil(gotV, "%s: %s must return nil values", tgt.name, q.desc)
			}
		}
	}

	// adjacent tiles must reassemble into the full scan, with no overlap and no gap
	fullK, fullV, err := targets[0].kv.ScanRange(_scanNsMain, nil, nil, 0)
	r.NoError(err)
	// guard against a vacuous test: the corpus must actually contain data
	r.Greater(len(fullK), 20)
	var tiledK, tiledV [][]byte
	bounds := [][]byte{nil, {0x01}, {0x41, 0x41}, {0x80}, {0xfe, 0xfe}, nil}
	for i := 0; i+1 < len(bounds); i++ {
		for _, tgt := range targets {
			k, v, err := tgt.kv.ScanRange(_scanNsMain, bounds[i], bounds[i+1], 0)
			r.NoError(err)
			if tgt == targets[0] {
				tiledK = append(tiledK, k...)
				tiledV = append(tiledV, v...)
			}
		}
	}
	r.Equal(fullK, tiledK)
	r.Equal(fullV, tiledV)
}

func TestNextPrefix(t *testing.T) {
	r := require.New(t)
	r.Equal([]byte{0x01}, nextPrefix([]byte{0x00}))
	r.Equal([]byte{0x13, 0x00}, nextPrefix([]byte{0x12, 0xff}))
	r.Equal([]byte{0x01, 0x00, 0x00}, nextPrefix([]byte{0x00, 0xff, 0xff}))
	// all-0xff carries out: no upper bound exists
	r.Nil(nextPrefix([]byte{0xff, 0xff}))
	r.Nil(nextPrefix([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}))
	// the returned bound must sort strictly after every key carrying the prefix
	for _, p := range [][]byte{{0x12, 0xff}, {0x00, 0x00}, {0x7f, 0xfe}} {
		ub := nextPrefix(p)
		r.NotNil(ub)
		r.Positive(bytes.Compare(ub, append(append([]byte{}, p...), 0xff, 0xff)))
	}
}

func TestScanRangeUnsupportedBase(t *testing.T) {
	r := require.New(t)
	// a base store that does not implement KVStoreWithRangeScan must produce an
	// explicit error rather than a silently truncated answer
	flusher, err := NewKVStoreFlusher(&noScanKVStore{}, batch.NewCachedBatch())
	r.NoError(err)
	scanner := flusher.KVStoreWithBuffer().(KVStoreWithRangeScan)
	_, _, err = scanner.ScanRange("ns", nil, nil, 0)
	r.ErrorIs(err, ErrNotSupported)
}

type noScanKVStore struct{}

func (n *noScanKVStore) Start(context.Context) error         { return nil }
func (n *noScanKVStore) Stop(context.Context) error          { return nil }
func (n *noScanKVStore) Put(string, []byte, []byte) error    { return nil }
func (n *noScanKVStore) Get(string, []byte) ([]byte, error)  { return nil, ErrNotExist }
func (n *noScanKVStore) Delete(string, []byte) error         { return nil }
func (n *noScanKVStore) WriteBatch(batch.KVStoreBatch) error { return nil }
func (n *noScanKVStore) Filter(string, Condition, []byte, []byte) ([][]byte, [][]byte, error) {
	return nil, nil, ErrNotSupported
}
