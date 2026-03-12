// snapshotexporter exports an iotex-core trie.db into a portable snapshot file.
//
// Usage:
//
//	snapshotexporter --source <trie.db> --output <snapshot.bin.gz>
package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/iotexproject/iotex-core/v2/db"
)

// namespaces to export from trie.db
var exportNamespaces = []string{"Account", "Code", "Contract"}

// forEacher is implemented by both BoltDB and PebbleDB
type forEacher interface {
	ForEach(string, func([]byte, []byte) error) error
}

func main() {
	source := flag.String("source", "", "path to trie.db (BoltDB)")
	output := flag.String("output", "snapshot.bin.gz", "output snapshot file path")
	flag.Parse()

	if *source == "" {
		fmt.Fprintln(os.Stderr, "usage: snapshotexporter --source <trie.db> [--output <snapshot.bin.gz>]")
		os.Exit(1)
	}

	if err := run(*source, *output); err != nil {
		log.Fatal(err)
	}
}

func run(source, output string) error {
	start := time.Now()

	// Open trie.db read-only (auto-detects BoltDB vs PebbleDB)
	cfg := db.DefaultConfig
	cfg.ReadOnly = true
	kvStore, err := db.CreateKVStore(cfg, source)
	if err != nil {
		return fmt.Errorf("create kvstore: %w", err)
	}
	if err := kvStore.Start(context.Background()); err != nil {
		return fmt.Errorf("open trie.db: %w", err)
	}
	defer kvStore.Stop(context.Background())

	// Read current height from Account/"currentHeight"
	heightBytes, err := kvStore.Get("Account", []byte("currentHeight"))
	if err != nil {
		return fmt.Errorf("read currentHeight: %w", err)
	}
	height := binary.BigEndian.Uint64(heightBytes)
	log.Printf("source height: %d", height)

	// Create output file
	f, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer f.Close()

	// Create snapshot writer
	sw, err := NewSnapshotWriter(f, height)
	if err != nil {
		return fmt.Errorf("create snapshot writer: %w", err)
	}

	// Type-assert to get ForEach (available on both BoltDB and PebbleDB)
	fe, ok := kvStore.(forEacher)
	if !ok {
		return fmt.Errorf("kvstore does not support ForEach iteration")
	}

	// Export each namespace
	for _, ns := range exportNamespaces {
		log.Printf("exporting namespace: %s", ns)
		nsCount := uint64(0)

		err := fe.ForEach(ns, func(k, v []byte) error {
			if err := sw.WriteEntry(ns, k, v); err != nil {
				return err
			}
			nsCount++
			if nsCount%1_000_000 == 0 {
				log.Printf("  %s: %d entries...", ns, nsCount)
			}
			return nil
		})
		if err != nil {
			// Skip missing buckets (not all nodes have all namespaces)
			log.Printf("  %s: skipped (%v)", ns, err)
			continue
		}
		log.Printf("  %s: %d entries", ns, nsCount)
	}

	// Finalize
	if err := sw.Finalize(); err != nil {
		return fmt.Errorf("finalize snapshot: %w", err)
	}

	elapsed := time.Since(start)
	stat, _ := f.Stat()
	sizeGB := float64(stat.Size()) / (1024 * 1024 * 1024)

	log.Printf("snapshot complete: %d entries, %.2f GB, %v", sw.Count(), sizeGB, elapsed)
	log.Printf("output: %s", output)
	return nil
}
