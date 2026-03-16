// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// trimstate removes namespaces from a trie.db that a stateless node does not need.
// Specifically it deletes the "Contract" namespace (EVM contract storage trie data),
// which a stateless node verifies via Merkle witness proofs instead.
//
// Usage:
//
//	trimstate -db /var/data/trie.db [-dry-run] [-compact]
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cockroachdb/pebble"
	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/state"
)

const prefixLength = 8

// nsToPrefix mirrors db.nsToPrefix (unexported in the db package).
func nsToPrefix(ns string) []byte {
	h := hash.Hash160b([]byte(ns))
	return h[:prefixLength]
}

// nextPrefix returns the smallest byte slice that is lexicographically greater
// than all keys sharing the given prefix, suitable as an exclusive upper bound.
func nextPrefix(prefix []byte) []byte {
	next := make([]byte, len(prefix))
	copy(next, prefix)
	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] != 0 {
			return next
		}
	}
	// overflow: all bytes wrapped to 0, meaning the prefix covers the entire keyspace
	return nil
}

// countKeys counts keys whose encoded key starts with the namespace prefix.
func countKeys(db *pebble.DB, ns string) (int64, error) {
	lower := nsToPrefix(ns)
	upper := nextPrefix(lower)
	iter, err := db.NewIter(&pebble.IterOptions{
		LowerBound: lower,
		UpperBound: upper,
	})
	if err != nil {
		return 0, err
	}
	defer iter.Close()
	var count int64
	for iter.First(); iter.Valid(); iter.Next() {
		count++
	}
	return count, iter.Error()
}

func main() {
	dbPath := flag.String("db", "", "Path to trie.db (required)")
	dryRun := flag.Bool("dry-run", false, "Count keys without deleting (opens DB read-only)")
	compact := flag.Bool("compact", false, "Run full DB compaction after deletion to reclaim disk space")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: trimstate -db <path> [-dry-run] [-compact]\n\n")
		fmt.Fprintf(os.Stderr, "Removes the EVM contract storage namespace from a trie.db so that\n")
		fmt.Fprintf(os.Stderr, "the database can be used by a stateless node.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *dbPath == "" {
		fmt.Fprintln(os.Stderr, "error: -db is required")
		flag.Usage()
		os.Exit(1)
	}

	comparer := *pebble.DefaultComparer
	comparer.Split = func(a []byte) int { return prefixLength }

	db, err := pebble.Open(*dbPath, &pebble.Options{
		Comparer:           &comparer,
		FormatMajorVersion: pebble.FormatPrePebblev1MarkedCompacted,
		ReadOnly:           *dryRun,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open DB: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	namespacesToTrim := []string{
		state.ContractKVNameSpace,
	}

	for _, ns := range namespacesToTrim {
		count, err := countKeys(db, ns)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to count namespace %q: %v\n", ns, err)
			os.Exit(1)
		}
		fmt.Printf("namespace %q: %d keys\n", ns, count)

		if *dryRun || count == 0 {
			continue
		}

		lower := nsToPrefix(ns)
		upper := nextPrefix(lower)
		if err := db.DeleteRange(lower, upper, pebble.Sync); err != nil {
			fmt.Fprintf(os.Stderr, "failed to delete namespace %q: %v\n", ns, err)
			os.Exit(1)
		}
		fmt.Printf("namespace %q: deleted\n", ns)
	}

	if *dryRun {
		fmt.Println("dry-run: no changes made")
		return
	}

	if *compact {
		fmt.Println("running compaction (this may take a while)...")
		maxKey := make([]byte, prefixLength)
		for i := range maxKey {
			maxKey[i] = 0xFF
		}
		if err := db.Compact(nil, maxKey, false); err != nil {
			fmt.Fprintf(os.Stderr, "compaction failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("compaction done")
	}
}
