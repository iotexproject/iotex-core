package ioswarm

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func testLogger() *zap.Logger {
	l, _ := zap.NewDevelopment()
	return l
}

func newTestDiffStore(t *testing.T) (*DiffStore, func()) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "statediffs.db")
	ds, err := OpenDiffStore(path, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	return ds, func() { ds.Close() }
}

func makeDiff(height uint64, n int) *StateDiff {
	entries := make([]StateDiffEntry, n)
	for i := 0; i < n; i++ {
		entries[i] = StateDiffEntry{
			Type:      0,
			Namespace: "Account",
			Key:       []byte{byte(i)},
			Value:     []byte{byte(height), byte(i)},
		}
	}
	return &StateDiff{
		Height:      height,
		Entries:     entries,
		DigestBytes: []byte{byte(height)},
	}
}

func TestDiffStore_AppendAndGet(t *testing.T) {
	ds, cleanup := newTestDiffStore(t)
	defer cleanup()

	diff := makeDiff(100, 3)
	if err := ds.Append(diff); err != nil {
		t.Fatal(err)
	}

	got, err := ds.Get(100)
	if err != nil {
		t.Fatal(err)
	}
	if got.Height != 100 {
		t.Fatalf("expected height 100, got %d", got.Height)
	}
	if len(got.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got.Entries))
	}
	if got.Entries[0].Namespace != "Account" {
		t.Fatalf("expected namespace Account, got %s", got.Entries[0].Namespace)
	}
}

func TestDiffStore_GetNotFound(t *testing.T) {
	ds, cleanup := newTestDiffStore(t)
	defer cleanup()

	_, err := ds.Get(999)
	if err == nil {
		t.Fatal("expected error for missing height")
	}
}

func TestDiffStore_GetRange(t *testing.T) {
	ds, cleanup := newTestDiffStore(t)
	defer cleanup()

	for h := uint64(10); h <= 20; h++ {
		if err := ds.Append(makeDiff(h, 1)); err != nil {
			t.Fatal(err)
		}
	}

	// Full range
	diffs, err := ds.GetRange(10, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 11 {
		t.Fatalf("expected 11 diffs, got %d", len(diffs))
	}

	// Partial range
	diffs, err = ds.GetRange(15, 18)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 4 {
		t.Fatalf("expected 4 diffs, got %d", len(diffs))
	}
	if diffs[0].Height != 15 || diffs[3].Height != 18 {
		t.Fatalf("unexpected range bounds: %d..%d", diffs[0].Height, diffs[3].Height)
	}

	// Empty range
	diffs, err = ds.GetRange(100, 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 0 {
		t.Fatalf("expected 0 diffs, got %d", len(diffs))
	}
}

func TestDiffStore_LatestAndOldestHeight(t *testing.T) {
	ds, cleanup := newTestDiffStore(t)
	defer cleanup()

	if ds.LatestHeight() != 0 || ds.OldestHeight() != 0 {
		t.Fatal("expected 0 for empty store")
	}

	ds.Append(makeDiff(50, 1))
	ds.Append(makeDiff(51, 1))
	ds.Append(makeDiff(52, 1))

	if ds.OldestHeight() != 50 {
		t.Fatalf("expected oldest 50, got %d", ds.OldestHeight())
	}
	if ds.LatestHeight() != 52 {
		t.Fatalf("expected latest 52, got %d", ds.LatestHeight())
	}
}

func TestDiffStore_Prune(t *testing.T) {
	ds, cleanup := newTestDiffStore(t)
	defer cleanup()

	for h := uint64(1); h <= 10; h++ {
		ds.Append(makeDiff(h, 1))
	}

	// Prune heights < 6
	deleted, err := ds.Prune(6)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 5 {
		t.Fatalf("expected 5 deleted, got %d", deleted)
	}
	if ds.OldestHeight() != 6 {
		t.Fatalf("expected oldest 6 after prune, got %d", ds.OldestHeight())
	}
	if ds.LatestHeight() != 10 {
		t.Fatalf("expected latest 10 after prune, got %d", ds.LatestHeight())
	}

	// Verify pruned entries are gone
	_, err = ds.Get(5)
	if err == nil {
		t.Fatal("expected error for pruned height 5")
	}

	// Verify remaining entries exist
	got, err := ds.Get(6)
	if err != nil {
		t.Fatal(err)
	}
	if got.Height != 6 {
		t.Fatalf("expected height 6, got %d", got.Height)
	}
}

func TestDiffStore_PruneAll(t *testing.T) {
	ds, cleanup := newTestDiffStore(t)
	defer cleanup()

	ds.Append(makeDiff(1, 1))
	ds.Append(makeDiff(2, 1))

	deleted, err := ds.Prune(100)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 deleted, got %d", deleted)
	}
	if ds.OldestHeight() != 0 || ds.LatestHeight() != 0 {
		t.Fatal("expected 0/0 after pruning all")
	}
}

func TestDiffStore_IdempotentAppend(t *testing.T) {
	ds, cleanup := newTestDiffStore(t)
	defer cleanup()

	diff1 := makeDiff(100, 2)
	diff2 := makeDiff(100, 5) // same height, different data

	if err := ds.Append(diff1); err != nil {
		t.Fatal(err)
	}
	if err := ds.Append(diff2); err != nil {
		t.Fatal(err)
	}

	// Should have the second version
	got, err := ds.Get(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Entries) != 5 {
		t.Fatalf("expected 5 entries (second write), got %d", len(got.Entries))
	}
}

func TestDiffStore_Reopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "statediffs.db")

	// Write some diffs
	ds, err := OpenDiffStore(path, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	for h := uint64(10); h <= 20; h++ {
		ds.Append(makeDiff(h, 1))
	}
	ds.Close()

	// Reopen and verify
	ds2, err := OpenDiffStore(path, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer ds2.Close()

	if ds2.OldestHeight() != 10 {
		t.Fatalf("expected oldest 10, got %d", ds2.OldestHeight())
	}
	if ds2.LatestHeight() != 20 {
		t.Fatalf("expected latest 20, got %d", ds2.LatestHeight())
	}

	got, err := ds2.Get(15)
	if err != nil {
		t.Fatal(err)
	}
	if got.Height != 15 {
		t.Fatalf("expected height 15, got %d", got.Height)
	}
}

func TestDiffStore_PruneZero(t *testing.T) {
	ds, cleanup := newTestDiffStore(t)
	defer cleanup()

	ds.Append(makeDiff(1, 1))

	// keepAfter=0 should be a no-op
	deleted, err := ds.Prune(0)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 0 {
		t.Fatalf("expected 0 deleted with keepAfter=0, got %d", deleted)
	}
}

// Suppress unused import warning
var _ = os.TempDir
