package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
	"io"
)

// Binary format constants
var (
	magicHeader = [8]byte{'I', 'O', 'S', 'W', 'S', 'N', 'A', 'P'}
	magicEnd    = [8]byte{'S', 'N', 'A', 'P', 'E', 'N', 'D', 0}
	version1    = uint32(1)
)

// SnapshotWriter writes state entries in the IOSwarm snapshot binary format.
// Output is gzip-compressed.
type SnapshotWriter struct {
	gw         *gzip.Writer
	hasher     hash.Hash
	w          io.Writer // tee of gw and hasher
	count      uint64
	height     uint64
	finalized  bool
}

// NewSnapshotWriter creates a new snapshot writer that writes to w.
// The height is written into the header.
func NewSnapshotWriter(w io.Writer, height uint64) (*SnapshotWriter, error) {
	gw := gzip.NewWriter(w)
	h := sha256.New()
	tee := io.MultiWriter(gw, h)

	sw := &SnapshotWriter{
		gw:     gw,
		hasher: h,
		w:      tee,
		height: height,
	}

	// Write header: magic + version + height
	if _, err := tee.Write(magicHeader[:]); err != nil {
		return nil, fmt.Errorf("write header magic: %w", err)
	}
	if err := binary.Write(tee, binary.BigEndian, version1); err != nil {
		return nil, fmt.Errorf("write header version: %w", err)
	}
	if err := binary.Write(tee, binary.BigEndian, height); err != nil {
		return nil, fmt.Errorf("write header height: %w", err)
	}

	return sw, nil
}

// WriteEntry writes a single namespace/key/value entry.
func (sw *SnapshotWriter) WriteEntry(ns string, key, value []byte) error {
	if sw.finalized {
		return fmt.Errorf("snapshot already finalized")
	}

	nsBytes := []byte(ns)
	if len(nsBytes) > 255 {
		return fmt.Errorf("namespace too long: %d bytes (max 255)", len(nsBytes))
	}

	// entry marker (1 byte: 0x01 = entry follows)
	if err := binary.Write(sw.w, binary.BigEndian, uint8(1)); err != nil {
		return err
	}

	// namespace_len (1 byte) + namespace
	if err := binary.Write(sw.w, binary.BigEndian, uint8(len(nsBytes))); err != nil {
		return err
	}
	if _, err := sw.w.Write(nsBytes); err != nil {
		return err
	}

	// key_len (4 bytes) + key
	if err := binary.Write(sw.w, binary.BigEndian, uint32(len(key))); err != nil {
		return err
	}
	if _, err := sw.w.Write(key); err != nil {
		return err
	}

	// value_len (4 bytes) + value
	if err := binary.Write(sw.w, binary.BigEndian, uint32(len(value))); err != nil {
		return err
	}
	if _, err := sw.w.Write(value); err != nil {
		return err
	}

	sw.count++
	return nil
}

// Finalize writes the trailer and closes the gzip stream.
// Must be called exactly once after all entries are written.
func (sw *SnapshotWriter) Finalize() error {
	if sw.finalized {
		return fmt.Errorf("snapshot already finalized")
	}
	sw.finalized = true

	// Write end-of-entries marker (0x00) through the hashed writer
	if err := binary.Write(sw.w, binary.BigEndian, uint8(0)); err != nil {
		return fmt.Errorf("write end marker: %w", err)
	}

	// Compute digest of everything written so far (header + entries + end marker)
	digest := sw.hasher.Sum(nil)

	// Write trailer directly to gzip writer (not through hasher)
	// entry_count (8 bytes)
	if err := binary.Write(sw.gw, binary.BigEndian, sw.count); err != nil {
		return fmt.Errorf("write trailer count: %w", err)
	}
	// sha256 digest (32 bytes)
	if _, err := sw.gw.Write(digest); err != nil {
		return fmt.Errorf("write trailer digest: %w", err)
	}
	// end magic (8 bytes)
	if _, err := sw.gw.Write(magicEnd[:]); err != nil {
		return fmt.Errorf("write trailer magic: %w", err)
	}

	return sw.gw.Close()
}

// Count returns the number of entries written so far.
func (sw *SnapshotWriter) Count() uint64 {
	return sw.count
}

// SnapshotReader reads entries from an IOSwarm snapshot binary file.
type SnapshotReader struct {
	gr         *gzip.Reader
	height     uint64
	entryCount uint64 // populated after Verify()
	readCount  uint64
	hasher     hash.Hash
	r          io.Reader // tee of gr and hasher
	done       bool
}

// NewSnapshotReader creates a reader from a gzip-compressed snapshot stream.
// It reads and validates the header, returning the snapshot height.
func NewSnapshotReader(r io.Reader) (*SnapshotReader, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("open gzip: %w", err)
	}

	h := sha256.New()
	tee := io.TeeReader(gr, h)

	sr := &SnapshotReader{
		gr:     gr,
		hasher: h,
		r:      tee,
	}

	// Read header
	var magic [8]byte
	if _, err := io.ReadFull(tee, magic[:]); err != nil {
		return nil, fmt.Errorf("read header magic: %w", err)
	}
	if magic != magicHeader {
		return nil, fmt.Errorf("invalid snapshot magic: %x", magic)
	}

	var ver uint32
	if err := binary.Read(tee, binary.BigEndian, &ver); err != nil {
		return nil, fmt.Errorf("read header version: %w", err)
	}
	if ver != version1 {
		return nil, fmt.Errorf("unsupported snapshot version: %d", ver)
	}

	if err := binary.Read(tee, binary.BigEndian, &sr.height); err != nil {
		return nil, fmt.Errorf("read header height: %w", err)
	}

	return sr, nil
}

// Height returns the snapshot height from the header.
func (sr *SnapshotReader) Height() uint64 {
	return sr.height
}

// Next reads the next entry. Returns io.EOF when the trailer is reached.
// The caller should call Verify() after Next returns io.EOF to validate the digest.
func (sr *SnapshotReader) Next() (ns string, key, value []byte, err error) {
	if sr.done {
		return "", nil, nil, io.EOF
	}

	// Read entry marker: 0x01 = entry follows, 0x00 = end of entries
	var marker uint8
	if err := binary.Read(sr.r, binary.BigEndian, &marker); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			sr.done = true
			return "", nil, nil, io.EOF
		}
		return "", nil, nil, err
	}
	if marker == 0 {
		sr.done = true
		return "", nil, nil, io.EOF
	}

	// Read namespace_len
	var nsLen uint8
	if err := binary.Read(sr.r, binary.BigEndian, &nsLen); err != nil {
		return "", nil, nil, fmt.Errorf("read ns_len: %w", err)
	}

	// Read namespace
	nsBytes := make([]byte, nsLen)
	if _, err := io.ReadFull(sr.r, nsBytes); err != nil {
		return "", nil, nil, fmt.Errorf("read namespace: %w", err)
	}

	// Read key
	var keyLen uint32
	if err := binary.Read(sr.r, binary.BigEndian, &keyLen); err != nil {
		return "", nil, nil, fmt.Errorf("read key_len: %w", err)
	}
	key = make([]byte, keyLen)
	if _, err := io.ReadFull(sr.r, key); err != nil {
		return "", nil, nil, fmt.Errorf("read key: %w", err)
	}

	// Read value
	var valLen uint32
	if err := binary.Read(sr.r, binary.BigEndian, &valLen); err != nil {
		return "", nil, nil, fmt.Errorf("read value_len: %w", err)
	}
	value = make([]byte, valLen)
	if _, err := io.ReadFull(sr.r, value); err != nil {
		return "", nil, nil, fmt.Errorf("read value: %w", err)
	}

	sr.readCount++
	return string(nsBytes), key, value, nil
}

// ReadCount returns the number of entries read so far.
func (sr *SnapshotReader) ReadCount() uint64 {
	return sr.readCount
}

// Verify reads and validates the trailer (entry count + sha256 digest + end magic).
// Should be called after Next() returns io.EOF.
func (sr *SnapshotReader) Verify() error {
	// Get digest of everything hashed so far (header + entries)
	expectedDigest := sr.hasher.Sum(nil)

	// Read trailer directly from gzip reader (not through hasher)
	var count uint64
	if err := binary.Read(sr.gr, binary.BigEndian, &count); err != nil {
		return fmt.Errorf("read trailer count: %w", err)
	}
	sr.entryCount = count

	if count != sr.readCount {
		return fmt.Errorf("entry count mismatch: trailer says %d, read %d", count, sr.readCount)
	}

	digest := make([]byte, 32)
	if _, err := io.ReadFull(sr.gr, digest); err != nil {
		return fmt.Errorf("read trailer digest: %w", err)
	}

	for i := range digest {
		if digest[i] != expectedDigest[i] {
			return fmt.Errorf("digest mismatch at byte %d: expected %x, got %x", i, expectedDigest, digest)
		}
	}

	var endMagic [8]byte
	if _, err := io.ReadFull(sr.gr, endMagic[:]); err != nil {
		return fmt.Errorf("read end magic: %w", err)
	}
	if endMagic != magicEnd {
		return fmt.Errorf("invalid end magic: %x", endMagic)
	}

	return sr.gr.Close()
}
