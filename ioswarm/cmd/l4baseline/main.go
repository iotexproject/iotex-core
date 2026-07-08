// l4baseline converts a mainnet trie.db (BoltDB) into an IOSWSNAP baseline
// snapshot for L4 agent state sync.
//
// The trie.db contains all IoTeX blockchain state in BoltDB buckets:
//   - "Account": account states (protobuf-serialized), key = hash160(addr)
//   - "Code":    contract bytecode, key = codeHash
//   - "Contract": Merkle Patricia Trie nodes for contract storage
//
// The IOSWSNAP format is a gzip-compressed binary with SHA-256 integrity:
//
//	header: magic(8) + version(4) + height(8)
//	entries: [marker(1) + ns_len(1) + ns + key_len(4) + key + val_len(4) + val]*
//	end: marker(1, 0x00)
//	trailer: count(8) + sha256(32) + end_magic(8)
//
// Usage:
//
//	# Inspect trie.db (stats only, fast)
//	go run ./ioswarm/cmd/l4baseline --source trie.db --stats
//
//	# Export full baseline snapshot
//	go run ./ioswarm/cmd/l4baseline --source trie.db --output baseline.snap.gz
//
//	# Export only Account bucket (skip Contract trie nodes to save space)
//	go run ./ioswarm/cmd/l4baseline --source trie.db --output baseline.snap.gz --namespaces Account,Code
package main

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"flag"
	"fmt"
	"hash"
	"io"
	"log"
	"os"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

// ---------------------------------------------------------------------------
// IOSWSNAP binary format (matches iotex-core snapshotexporter)
// ---------------------------------------------------------------------------

var (
	magicHeader = [8]byte{'I', 'O', 'S', 'W', 'S', 'N', 'A', 'P'}
	magicEnd    = [8]byte{'S', 'N', 'A', 'P', 'E', 'N', 'D', 0}
	version1    = uint32(1)
)

type snapshotWriter struct {
	gw        *gzip.Writer
	hasher    hash.Hash
	w         io.Writer
	count     uint64
	height    uint64
	finalized bool
}

func newSnapshotWriter(w io.Writer, height uint64) (*snapshotWriter, error) {
	gw := gzip.NewWriter(w)
	h := sha256.New()
	tee := io.MultiWriter(gw, h)

	sw := &snapshotWriter{gw: gw, hasher: h, w: tee, height: height}

	if _, err := tee.Write(magicHeader[:]); err != nil {
		return nil, err
	}
	if err := binary.Write(tee, binary.BigEndian, version1); err != nil {
		return nil, err
	}
	if err := binary.Write(tee, binary.BigEndian, height); err != nil {
		return nil, err
	}
	return sw, nil
}

func (sw *snapshotWriter) writeEntry(ns string, key, value []byte) error {
	nsBytes := []byte(ns)
	if err := binary.Write(sw.w, binary.BigEndian, uint8(1)); err != nil {
		return err
	}
	if err := binary.Write(sw.w, binary.BigEndian, uint8(len(nsBytes))); err != nil {
		return err
	}
	if _, err := sw.w.Write(nsBytes); err != nil {
		return err
	}
	if err := binary.Write(sw.w, binary.BigEndian, uint32(len(key))); err != nil {
		return err
	}
	if _, err := sw.w.Write(key); err != nil {
		return err
	}
	if err := binary.Write(sw.w, binary.BigEndian, uint32(len(value))); err != nil {
		return err
	}
	if _, err := sw.w.Write(value); err != nil {
		return err
	}
	sw.count++
	return nil
}

func (sw *snapshotWriter) finalize() error {
	if sw.finalized {
		return nil
	}
	sw.finalized = true

	if err := binary.Write(sw.w, binary.BigEndian, uint8(0)); err != nil {
		return err
	}
	digest := sw.hasher.Sum(nil)
	if err := binary.Write(sw.gw, binary.BigEndian, sw.count); err != nil {
		return err
	}
	if _, err := sw.gw.Write(digest); err != nil {
		return err
	}
	if _, err := sw.gw.Write(magicEnd[:]); err != nil {
		return err
	}
	return sw.gw.Close()
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	source := flag.String("source", "", "path to trie.db (BoltDB)")
	output := flag.String("output", "", "output snapshot file (e.g., baseline.snap.gz)")
	stats := flag.Bool("stats", false, "print stats only, don't export")
	namespaces := flag.String("namespaces", "Account,Code,Contract", "comma-separated namespaces to export")
	timeout := flag.Duration("timeout", 30*time.Minute, "maximum runtime (kills process to avoid holding DB lock)")
	flag.Parse()

	// Safety: kill process after timeout to avoid holding BoltDB lock indefinitely.
	// BoltDB ReadOnly mode still acquires a shared flock, which blocks iotex-server
	// from acquiring its exclusive lock on the same trie.db file.
	go func() {
		<-time.After(*timeout)
		log.Fatalf("timeout after %v — exiting to release DB lock", *timeout)
	}()

	if *source == "" {
		fmt.Fprintln(os.Stderr, "usage: l4baseline --source <trie.db> [--output <file>] [--stats] [--namespaces Account,Code,Contract]")
		os.Exit(1)
	}
	if !*stats && *output == "" {
		fmt.Fprintln(os.Stderr, "error: specify --output or --stats")
		os.Exit(1)
	}

	var nsList []string
	for _, ns := range strings.Split(*namespaces, ",") {
		ns = strings.TrimSpace(ns)
		if ns != "" {
			nsList = append(nsList, ns)
		}
	}

	db, err := bolt.Open(*source, 0400, &bolt.Options{ReadOnly: true, Timeout: 10 * time.Second})
	if err != nil {
		log.Fatalf("open trie.db: %v", err)
	}
	defer db.Close()

	// Read height
	height, err := readHeight(db)
	if err != nil {
		log.Fatalf("read height: %v", err)
	}
	log.Printf("trie.db height: %d", height)

	if *stats {
		printStats(db)
		return
	}

	// Export
	if err := export(db, *output, height, nsList); err != nil {
		log.Fatalf("export: %v", err)
	}
}

func readHeight(db *bolt.DB) (uint64, error) {
	var height uint64
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Account"))
		if b == nil {
			return fmt.Errorf("Account bucket not found")
		}
		v := b.Get([]byte("currentHeight"))
		if v == nil {
			return fmt.Errorf("currentHeight key not found")
		}
		if len(v) != 8 {
			return fmt.Errorf("unexpected currentHeight length: %d", len(v))
		}
		// iotex-core uses byteutil.Uint64ToBytes which is little-endian on most platforms,
		// but the snapshotexporter reads big-endian. Try both and use the reasonable one.
		heightLE := binary.LittleEndian.Uint64(v)
		heightBE := binary.BigEndian.Uint64(v)
		// Mainnet is at ~45M blocks. The correct one should be in a reasonable range.
		if heightBE > 0 && heightBE < 100_000_000 {
			height = heightBE
		} else if heightLE > 0 && heightLE < 100_000_000 {
			height = heightLE
		} else {
			// Default to big-endian
			height = heightBE
		}
		log.Printf("  raw bytes: %x (BE=%d, LE=%d, using=%d)", v, heightBE, heightLE, height)
		return nil
	})
	return height, err
}

func printStats(db *bolt.DB) {
	err := db.View(func(tx *bolt.Tx) error {
		fmt.Println()
		fmt.Println("╔══════════════════════════════════════════════════════════════╗")
		fmt.Println("║              trie.db Statistics                             ║")
		fmt.Println("╠══════════════════════════════════════════════════════════════╣")

		return tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			stats := b.Stats()
			leafSize := int64(stats.LeafInuse)
			branchSize := int64(stats.BranchInuse)
			totalSize := leafSize + branchSize

			fmt.Printf("║  %-20s  keys=%-10d  size=%-10s  ║\n",
				string(name), stats.KeyN, formatSize(totalSize))
			return nil
		})
	})
	if err != nil {
		log.Fatalf("stats: %v", err)
	}
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
}

func export(boltDB *bolt.DB, outputPath string, height uint64, namespaces []string) error {
	start := time.Now()
	success := false

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer func() {
		f.Close()
		if !success {
			os.Remove(outputPath) // clean up incomplete file
		}
	}()

	bw := bufio.NewWriterSize(f, 4*1024*1024) // 4 MB write buffer
	sw, err := newSnapshotWriter(bw, height)
	if err != nil {
		return fmt.Errorf("create writer: %w", err)
	}

	var totalEntries uint64

	for _, ns := range namespaces {
		log.Printf("exporting %s...", ns)
		nsCount := uint64(0)
		nsStart := time.Now()

		err := boltDB.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(ns))
			if b == nil {
				log.Printf("  %s: bucket not found, skipping", ns)
				return nil
			}
			c := b.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				if v == nil {
					continue // skip nested buckets
				}
				if err := sw.writeEntry(ns, k, v); err != nil {
					return err
				}
				nsCount++
				if nsCount%1_000_000 == 0 {
					log.Printf("  %s: %d entries... (%.1fs)", ns, nsCount, time.Since(nsStart).Seconds())
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("export %s: %w", ns, err)
		}
		log.Printf("  %s: %d entries in %.1fs", ns, nsCount, time.Since(nsStart).Seconds())
		totalEntries += nsCount
	}

	if err := sw.finalize(); err != nil {
		return fmt.Errorf("finalize: %w", err)
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}

	stat, _ := f.Stat()
	elapsed := time.Since(start)
	success = true

	log.Printf("export complete:")
	log.Printf("  entries:  %d", totalEntries)
	log.Printf("  size:     %s", formatSize(stat.Size()))
	log.Printf("  height:   %d", height)
	log.Printf("  elapsed:  %v", elapsed.Round(time.Millisecond))
	log.Printf("  output:   %s", outputPath)
	return nil
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(bytes)/(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(bytes)/(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
