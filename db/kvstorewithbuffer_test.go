package db

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_batch"
)

func TestFlusher(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Run("create failed with nil kvStore", func(t *testing.T) {
		f, err := NewKVStoreFlusher(nil, nil)
		require.Nil(t, f)
		require.Error(t, err)
		require.Contains(t, err.Error(), "store cannot be nil")
	})
	t.Run("fail to create with nil buffer", func(t *testing.T) {
		store := NewMockKVStore(ctrl)
		f, err := NewKVStoreFlusher(store, nil)
		require.Nil(t, f)
		require.Error(t, err)
		require.Contains(t, err.Error(), "buffer cannot be nil")
	})
	t.Run("create flusher successfully", func(t *testing.T) {
		store := NewMockKVStore(ctrl)
		buffer := mock_batch.NewMockCachedBatch(ctrl)
		f, err := NewKVStoreFlusher(store, buffer)
		require.NoError(t, err)
		kvb := f.KVStoreWithBuffer()
		expectedError := errors.New("failed to start")
		ns := "namespace"
		key := []byte("key")
		value := []byte("value")
		t.Run("fail to start kvStore with buffer", func(t *testing.T) {
			store.EXPECT().Start(gomock.Any()).Return(err).Times(1)
			require.Equal(t, err, kvb.Start(context.Background()))
		})
		t.Run("fail to stop kvStore with buffer", func(t *testing.T) {
			err = errors.New("failed to stop")
			store.EXPECT().Stop(gomock.Any()).Return(err).Times(1)
			require.Equal(t, err, kvb.Stop(context.Background()))
		})
		t.Run("start kv store successfully", func(t *testing.T) {
			store.EXPECT().Start(gomock.Any()).Return(nil).Times(1)
			store.EXPECT().Stop(gomock.Any()).Return(nil).Times(1)
			require.NoError(t, kvb.Start(context.Background()))
			require.NoError(t, kvb.Stop(context.Background()))
		})
		t.Run("fail to flush", func(t *testing.T) {
			buffer.EXPECT().Translate(gomock.Any()).Return(buffer).Times(1)
			store.EXPECT().WriteBatch(gomock.Any()).Return(expectedError).Times(1)
			require.Equal(t, expectedError, f.Flush())
		})
		t.Run("flush successfully", func(t *testing.T) {
			buffer.EXPECT().Translate(gomock.Any()).Return(buffer).Times(1)
			store.EXPECT().WriteBatch(gomock.Any()).Return(nil).Times(1)
			buffer.EXPECT().Lock().Times(1)
			buffer.EXPECT().ClearAndUnlock().Times(1)
			require.NoError(t, f.Flush())
		})
		t.Run("Get", func(t *testing.T) {
			buffer.EXPECT().Get(ns, key).Return(value, nil).Times(1)
			v, err := kvb.Get(ns, key)
			require.True(t, bytes.Equal(value, v))
			require.NoError(t, err)
			buffer.EXPECT().Get(ns, key).Return(nil, batch.ErrNotExist).Times(1)
			store.EXPECT().Get(ns, key).Return(value, nil)
			v, err = kvb.Get(ns, key)
			require.True(t, bytes.Equal(value, v))
			require.NoError(t, err)
			buffer.EXPECT().Get(ns, key).Return(nil, batch.ErrAlreadyDeleted).Times(1)
			v, err = kvb.Get(ns, key)
			require.Nil(t, v)
			require.Equal(t, errors.Cause(err), ErrNotExist)
		})
		t.Run("Snapshot", func(t *testing.T) {
			buffer.EXPECT().Snapshot().Return(1).Times(1)
			require.Equal(t, 1, kvb.Snapshot())
		})
		t.Run("Revert", func(t *testing.T) {
			buffer.EXPECT().RevertSnapshot(gomock.Any()).Return(expectedError).Times(1)
			require.Equal(t, expectedError, kvb.RevertSnapshot(1))
			buffer.EXPECT().RevertSnapshot(gomock.Any()).Return(nil).Times(1)
			require.NoError(t, kvb.RevertSnapshot(1))
		})
		t.Run("Size", func(t *testing.T) {
			buffer.EXPECT().Size().Return(5).Times(1)
			require.Equal(t, 5, kvb.Size())
		})
		t.Run("SerializeQueue", func(t *testing.T) {
			buffer.EXPECT().SerializeQueue(gomock.Any(), gomock.Any()).Return(value).Times(1)
			require.Equal(t, value, f.SerializeQueue())
		})
		t.Run("MustPut", func(t *testing.T) {
			buffer.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
			kvb.MustPut(ns, key, value)
		})
		t.Run("MustDelete", func(t *testing.T) {
			buffer.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
			kvb.MustDelete(ns, key)
		})
	})
}

// ---------------------------------------------------------------------------
// ScanRange: the base scan is bounded by limit + (buffered deletes in range).
//
// The reference for every case below is the SAME query run with limit = 0 and
// then truncated by hand: that is the known-correct, pre-optimization behavior,
// so any divergence is a regression regardless of which side looks nicer.
// ---------------------------------------------------------------------------

const (
	_bufScanNS    = "BufScanMain"
	_bufScanOther = "BufScanOther"
)

// recordingScanStore is a base store that remembers the limit argument every
// ScanRange it served was given. It is what distinguishes the optimization from
// a no-op: a base scan that still receives 0 is the old, unbounded behavior.
type recordingScanStore struct {
	KVStoreWithRangeScan
	limits []int
}

func (r *recordingScanStore) ScanRange(ns string, min, max []byte, limit int) ([][]byte, [][]byte, error) {
	r.limits = append(r.limits, limit)
	return r.KVStoreWithRangeScan.ScanRange(ns, min, max, limit)
}

func (r *recordingScanStore) lastLimit(t *testing.T) int {
	t.Helper()
	require.NotEmpty(t, r.limits, "base store was never scanned")
	return r.limits[len(r.limits)-1]
}

// bufScanOp is one pending buffer write.
type bufScanOp struct {
	del bool
	ns  string
	k   string
	v   string
}

// newBufScanFixture builds a kvStoreWithBuffer whose base store holds baseKeys
// (in _bufScanNS, value "base-"+key) and whose buffer holds ops, still pending.
func newBufScanFixture(t *testing.T, baseKeys []string, ops []bufScanOp) (*recordingScanStore, batch.CachedBatch, KVStoreWithRangeScan) {
	t.Helper()
	r := require.New(t)
	base := &recordingScanStore{KVStoreWithRangeScan: NewMemKVStore().(KVStoreWithRangeScan)}
	for _, k := range baseKeys {
		r.NoError(base.Put(_bufScanNS, []byte(k), []byte("base-"+k)))
	}
	buffer := batch.NewCachedBatch()
	flusher, err := NewKVStoreFlusher(base, buffer)
	r.NoError(err)
	kvb := flusher.KVStoreWithBuffer()
	for _, op := range ops {
		ns := op.ns
		if ns == "" {
			ns = _bufScanNS
		}
		if op.del {
			r.NoError(kvb.Delete(ns, []byte(op.k)))
			continue
		}
		r.NoError(kvb.Put(ns, []byte(op.k), []byte(op.v)))
	}
	scanner, ok := kvb.(KVStoreWithRangeScan)
	r.True(ok, "kvStoreWithBuffer must implement KVStoreWithRangeScan")
	// the fixture writes must not count as scans
	base.limits = nil
	return base, buffer, scanner
}

// truncateScanResult applies a limit to an already-sorted result the way
// sortAndTruncateScan does, including its nil-for-empty convention.
func truncateScanResult(keys, values [][]byte, limit int) ([][]byte, [][]byte) {
	if limit > 0 && limit < len(keys) {
		keys, values = keys[:limit], values[:limit]
	}
	if len(keys) == 0 {
		return nil, nil
	}
	return keys, values
}

// naiveBufScanRange reproduces what ScanRange would compute if it pushed the
// caller's RAW limit into the base scan. It exists only so a test can show that
// a case actually discriminates -- that the bound has to be limit + d and not
// limit.
func naiveBufScanRange(base KVStoreWithRangeScan, buffer batch.CachedBatch, ns string, min, max []byte, limit int) ([][]byte, [][]byte, error) {
	if emptyScanRange(min, max) {
		return nil, nil, nil
	}
	baseKeys, baseValues, err := base.ScanRange(ns, min, max, limit)
	if err != nil {
		return nil, nil, err
	}
	merged := make(map[string][]byte, len(baseKeys))
	for i, k := range baseKeys {
		merged[string(k)] = baseValues[i]
	}
	for i := 0; i < buffer.Size(); i++ {
		entry, err := buffer.Entry(i)
		if err != nil {
			return nil, nil, err
		}
		if !scanEntryInRange(entry, ns, min, max) {
			continue
		}
		switch entry.WriteType() {
		case batch.Put:
			merged[string(entry.Key())] = copyBytes(entry.Value())
		case batch.Delete:
			delete(merged, string(entry.Key()))
		}
	}
	keys := make([][]byte, 0, len(merged))
	values := make([][]byte, 0, len(merged))
	for ks, v := range merged {
		keys = append(keys, []byte(ks))
		values = append(values, v)
	}
	k, v := sortAndTruncateScan(keys, values, limit)
	return k, v, nil
}

func bufScanKeys(prefix string, from, to int) []string {
	out := make([]string, 0, to-from)
	for i := from; i < to; i++ {
		out = append(out, fmt.Sprintf("%s%02d", prefix, i))
	}
	return out
}

func TestScanRangeBoundedBaseScan(t *testing.T) {
	base10 := bufScanKeys("k", 0, 10)
	cases := []struct {
		name     string
		baseKeys []string
		ops      []bufScanOp
		ns       string
		min, max []byte
		limit    int
		// wantBaseLimit is the limit the base store must have been given
		wantBaseLimit int
	}{
		{
			// (1) the delete lands exactly at the naive cutoff: without the +d
			// the base scan returns k00,k01,k02 and the delete leaves only two.
			name: "delete at the naive cutoff", baseKeys: base10,
			ops:   []bufScanOp{{del: true, k: "k02"}},
			limit: 3, wantBaseLimit: 4,
		},
		{
			name: "delete just below the naive cutoff", baseKeys: base10,
			ops:   []bufScanOp{{del: true, k: "k01"}},
			limit: 3, wantBaseLimit: 4,
		},
		{
			// (2) multiple deletes straddling the cutoff
			name: "multiple deletes straddling the cutoff", baseKeys: base10,
			ops:   []bufScanOp{{del: true, k: "k01"}, {del: true, k: "k03"}, {del: true, k: "k05"}},
			limit: 4, wantBaseLimit: 7,
		},
		{
			// (3) puts that sort BELOW the cutoff and displace base keys
			name: "puts displace base keys from below", baseKeys: bufScanKeys("k", 10, 20),
			ops:   []bufScanOp{{k: "k00", v: "buf-k00"}, {k: "k01", v: "buf-k01"}},
			limit: 3, wantBaseLimit: 3,
		},
		{
			name: "puts below the cutoff mixed with a delete", baseKeys: bufScanKeys("k", 10, 20),
			ops:   []bufScanOp{{k: "k00", v: "buf-k00"}, {del: true, k: "k11"}},
			limit: 3, wantBaseLimit: 4,
		},
		{
			// (4) put overriding an existing base key
			name: "put overrides a base key", baseKeys: base10,
			ops:   []bufScanOp{{k: "k01", v: "override"}},
			limit: 3, wantBaseLimit: 3,
		},
		{
			name: "put overrides a base key and a delete removes another", baseKeys: base10,
			ops:   []bufScanOp{{k: "k01", v: "override"}, {del: true, k: "k02"}},
			limit: 3, wantBaseLimit: 4,
		},
		{
			// (5) over-counting is harmless: the base does not hold k99
			name: "delete of a key absent from the base", baseKeys: base10,
			ops:   []bufScanOp{{del: true, k: "k99"}},
			limit: 3, wantBaseLimit: 4,
		},
		{
			// (6) duplicate deletes are counted twice, on purpose
			name: "duplicate delete of the same key", baseKeys: base10,
			ops:   []bufScanOp{{del: true, k: "k02"}, {del: true, k: "k02"}},
			limit: 3, wantBaseLimit: 5,
		},
		{
			name: "delete then put of the same key is not collapsed", baseKeys: base10,
			ops:   []bufScanOp{{del: true, k: "k02"}, {k: "k02", v: "back"}},
			limit: 3, wantBaseLimit: 4,
		},
		{
			// (7) deletes that this scan does not replay must not be counted
			name: "delete in another namespace is not counted", baseKeys: base10,
			ops:   []bufScanOp{{del: true, ns: _bufScanOther, k: "k02"}},
			limit: 3, wantBaseLimit: 3,
		},
		{
			name: "delete below min is not counted", baseKeys: base10,
			ops:   []bufScanOp{{del: true, k: "k00"}},
			min:   []byte("k03"),
			limit: 3, wantBaseLimit: 3,
		},
		{
			name: "delete at max is not counted (half-open)", baseKeys: base10,
			ops:   []bufScanOp{{del: true, k: "k05"}},
			max:   []byte("k05"),
			limit: 3, wantBaseLimit: 3,
		},
		{
			name: "in-range and out-of-range deletes mixed", baseKeys: base10,
			ops: []bufScanOp{
				{del: true, k: "k00"},                    // below min, not counted
				{del: true, k: "k04"},                    // in range, counted
				{del: true, k: "k09"},                    // at/above max, not counted
				{del: true, ns: _bufScanOther, k: "k04"}, // other ns, not counted
			},
			min: []byte("k03"), max: []byte("k08"),
			limit: 3, wantBaseLimit: 4,
		},
		{
			// (8) limit <= 0 must be bit-identical to before: base scan unbounded
			name: "limit zero leaves the base scan unbounded", baseKeys: base10,
			ops:   []bufScanOp{{del: true, k: "k02"}, {k: "k99", v: "added"}},
			limit: 0, wantBaseLimit: 0,
		},
		{
			name: "negative limit leaves the base scan unbounded", baseKeys: base10,
			ops:   []bufScanOp{{del: true, k: "k02"}},
			limit: -5, wantBaseLimit: 0,
		},
		{
			// (9) range exhausted before the limit
			name: "range exhausted before the limit", baseKeys: base10,
			ops:   []bufScanOp{{del: true, k: "k02"}},
			limit: 100, wantBaseLimit: 101,
		},
		{
			name: "narrow range exhausted before the limit", baseKeys: base10,
			ops: []bufScanOp{{del: true, k: "k04"}},
			min: []byte("k03"), max: []byte("k06"),
			limit: 50, wantBaseLimit: 51,
		},
		{
			name: "empty result with a limit", baseKeys: base10,
			ops: []bufScanOp{{del: true, k: "z00"}},
			min: []byte("z00"), max: []byte("z99"),
			limit: 3, wantBaseLimit: 4,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := require.New(t)
			ns := c.ns
			if ns == "" {
				ns = _bufScanNS
			}
			base, _, kvb := newBufScanFixture(t, c.baseKeys, c.ops)

			// reference: the unlimited query, truncated by hand
			refK, refV, err := kvb.ScanRange(ns, c.min, c.max, 0)
			r.NoError(err)
			r.Equal(0, base.lastLimit(t), "the reference query must scan the base unbounded")
			wantK, wantV := truncateScanResult(refK, refV, c.limit)

			base.limits = nil
			gotK, gotV, err := kvb.ScanRange(ns, c.min, c.max, c.limit)
			r.NoError(err)
			r.Equal(wantK, gotK, "keys")
			r.Equal(wantV, gotV, "values")
			r.Equal(c.wantBaseLimit, base.lastLimit(t), "limit pushed into the base scan")
		})
	}
}

// TestScanRangeBoundedBaseScanRejectsNaiveLimit is the test that says WHY the
// bound is limit+d: for these inputs, pushing the caller's raw limit down gives
// a different -- wrong -- answer.
func TestScanRangeBoundedBaseScanRejectsNaiveLimit(t *testing.T) {
	cases := []struct {
		name  string
		ops   []bufScanOp
		limit int
	}{
		{"delete at the cutoff", []bufScanOp{{del: true, k: "k02"}}, 3},
		{"delete below the cutoff", []bufScanOp{{del: true, k: "k00"}}, 3},
		{"deletes straddling the cutoff", []bufScanOp{{del: true, k: "k01"}, {del: true, k: "k03"}}, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := require.New(t)
			base, buffer, kvb := newBufScanFixture(t, bufScanKeys("k", 0, 10), c.ops)

			refK, refV, err := kvb.ScanRange(_bufScanNS, nil, nil, 0)
			r.NoError(err)
			wantK, wantV := truncateScanResult(refK, refV, c.limit)

			gotK, gotV, err := kvb.ScanRange(_bufScanNS, nil, nil, c.limit)
			r.NoError(err)
			r.Equal(wantK, gotK)
			r.Equal(wantV, gotV)

			naiveK, _, err := naiveBufScanRange(base, buffer, _bufScanNS, nil, nil, c.limit)
			r.NoError(err)
			r.NotEqual(wantK, naiveK, "case does not discriminate: the naive push-down happens to be right here")
		})
	}
}

// TestScanRangeBoundedBaseScanDifferential hammers the merge with random base
// contents, random buffer ops and random bounds, always comparing against the
// unlimited-then-truncated reference, and re-checks the pushed-down limit each
// time.
func TestScanRangeBoundedBaseScanDifferential(t *testing.T) {
	r := require.New(t)
	rnd := rand.New(rand.NewSource(20260803))
	pool := bufScanKeys("k", 0, 40)
	bounds := func() []byte {
		switch rnd.Intn(4) {
		case 0:
			return nil
		case 1:
			return []byte(pool[rnd.Intn(len(pool))])
		default:
			return []byte(fmt.Sprintf("k%02d", rnd.Intn(50)))
		}
	}
	for iter := 0; iter < 300; iter++ {
		var baseKeys []string
		for _, k := range pool {
			if rnd.Intn(2) == 0 {
				baseKeys = append(baseKeys, k)
			}
		}
		var ops []bufScanOp
		for n := rnd.Intn(12); n > 0; n-- {
			op := bufScanOp{k: pool[rnd.Intn(len(pool))]}
			if rnd.Intn(3) == 0 {
				op.ns = _bufScanOther
			}
			if rnd.Intn(2) == 0 {
				op.del = true
			} else {
				op.v = fmt.Sprintf("buf-%d-%s", iter, op.k)
			}
			ops = append(ops, op)
		}
		base, _, kvb := newBufScanFixture(t, baseKeys, ops)

		min, max := bounds(), bounds()
		limit := 0
		switch rnd.Intn(3) {
		case 0:
			limit = 1 + rnd.Intn(5)
		case 1:
			limit = 1 + rnd.Intn(50)
		}

		refK, refV, err := kvb.ScanRange(_bufScanNS, min, max, 0)
		r.NoError(err, "iter %d", iter)
		wantK, wantV := truncateScanResult(refK, refV, limit)

		base.limits = nil
		gotK, gotV, err := kvb.ScanRange(_bufScanNS, min, max, limit)
		r.NoError(err, "iter %d", iter)
		r.Equal(wantK, gotK, "iter %d keys (min=%s max=%s limit=%d)", iter, min, max, limit)
		r.Equal(wantV, gotV, "iter %d values (min=%s max=%s limit=%d)", iter, min, max, limit)

		if emptyScanRange(min, max) {
			r.Empty(base.limits, "iter %d: provably empty range must not touch the base", iter)
			continue
		}
		d := 0
		for _, op := range ops {
			if op.del && op.ns == "" && inScanRange([]byte(op.k), min, max) {
				d++
			}
		}
		want := 0
		if limit > 0 {
			want = limit + d
		}
		r.Equal(want, base.lastLimit(t), "iter %d: pushed-down limit (limit=%d d=%d)", iter, limit, d)
	}
}
