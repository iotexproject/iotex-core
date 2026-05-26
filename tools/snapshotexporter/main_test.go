package main

import (
	"bytes"
	"io"
	"testing"
)

func TestSnapshotRoundTrip(t *testing.T) {
	// Write a snapshot to a buffer
	var buf bytes.Buffer
	height := uint64(12345)

	sw, err := NewSnapshotWriter(&buf, height)
	if err != nil {
		t.Fatal(err)
	}

	type entry struct {
		ns    string
		key   []byte
		value []byte
	}

	entries := []entry{
		{"Account", []byte("addr1"), []byte("balance1")},
		{"Account", []byte("addr2"), []byte("balance2")},
		{"Code", []byte("contract1"), []byte{0xDE, 0xAD, 0xBE, 0xEF}},
		{"Contract", []byte("storage1"), []byte("slot_value")},
	}

	for _, e := range entries {
		if err := sw.WriteEntry(e.ns, e.key, e.value); err != nil {
			t.Fatal(err)
		}
	}

	if sw.Count() != uint64(len(entries)) {
		t.Fatalf("expected count %d, got %d", len(entries), sw.Count())
	}

	if err := sw.Finalize(); err != nil {
		t.Fatal(err)
	}

	// Read it back
	sr, err := NewSnapshotReader(&buf)
	if err != nil {
		t.Fatal(err)
	}

	if sr.Height() != height {
		t.Fatalf("expected height %d, got %d", height, sr.Height())
	}

	var readEntries []entry
	for {
		ns, key, value, err := sr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		readEntries = append(readEntries, entry{ns, key, value})
	}

	if len(readEntries) != len(entries) {
		t.Fatalf("expected %d entries, got %d", len(entries), len(readEntries))
	}

	for i, e := range entries {
		got := readEntries[i]
		if got.ns != e.ns {
			t.Errorf("entry %d: ns = %q, want %q", i, got.ns, e.ns)
		}
		if !bytes.Equal(got.key, e.key) {
			t.Errorf("entry %d: key mismatch", i)
		}
		if !bytes.Equal(got.value, e.value) {
			t.Errorf("entry %d: value mismatch", i)
		}
	}

	// Verify trailer
	if err := sr.Verify(); err != nil {
		t.Fatal(err)
	}

	if sr.ReadCount() != uint64(len(entries)) {
		t.Fatalf("read count = %d, want %d", sr.ReadCount(), len(entries))
	}
}

func TestSnapshotEmpty(t *testing.T) {
	var buf bytes.Buffer

	sw, err := NewSnapshotWriter(&buf, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := sw.Finalize(); err != nil {
		t.Fatal(err)
	}

	sr, err := NewSnapshotReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if sr.Height() != 0 {
		t.Fatalf("expected height 0, got %d", sr.Height())
	}

	_, _, _, err = sr.Next()
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}

	if err := sr.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotLargeEntries(t *testing.T) {
	var buf bytes.Buffer
	height := uint64(99999)

	sw, err := NewSnapshotWriter(&buf, height)
	if err != nil {
		t.Fatal(err)
	}

	// Write 1000 entries with varying sizes
	n := 1000
	for i := 0; i < n; i++ {
		key := make([]byte, 32)
		for j := range key {
			key[j] = byte(i*7 + j)
		}
		value := make([]byte, 100+i%500)
		for j := range value {
			value[j] = byte(i + j)
		}
		if err := sw.WriteEntry("Account", key, value); err != nil {
			t.Fatal(err)
		}
	}

	if err := sw.Finalize(); err != nil {
		t.Fatal(err)
	}

	// Read back and verify
	sr, err := NewSnapshotReader(&buf)
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	for {
		_, _, _, err := sr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		count++
	}

	if count != n {
		t.Fatalf("expected %d entries, got %d", n, count)
	}

	if err := sr.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotDoubleFinalize(t *testing.T) {
	var buf bytes.Buffer
	sw, err := NewSnapshotWriter(&buf, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := sw.Finalize(); err != nil {
		t.Fatal(err)
	}
	if err := sw.Finalize(); err == nil {
		t.Fatal("expected error on double finalize")
	}
}

func TestSnapshotWriteAfterFinalize(t *testing.T) {
	var buf bytes.Buffer
	sw, err := NewSnapshotWriter(&buf, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := sw.Finalize(); err != nil {
		t.Fatal(err)
	}
	if err := sw.WriteEntry("Account", []byte("k"), []byte("v")); err == nil {
		t.Fatal("expected error writing after finalize")
	}
}
