// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package db

import (
	"bytes"
	"sort"
)

// helpers shared by all KVStoreWithRangeScan implementations. They exist so that
// every engine makes the same decision for every edge case -- a divergence here
// between bolt and pebble would be a chain fork.

// RangeScan is an ordered, bounded range-scan request. A nil *RangeScan means
// "no range scan requested", which every caller must treat as "keep doing exactly
// what you did before".
//
// The fields carry the same semantics as the ScanRange arguments they end up in:
// Min == nil is the start of the namespace, Max == nil is the end, the interval is
// half-open [Min, Max), and Limit <= 0 is unlimited.
type RangeScan struct {
	Min   []byte
	Max   []byte
	Limit int
}

// emptyScanRange reports whether [min, max) is provably empty, i.e. max is
// bounded and min sorts at or after it. Callers must short-circuit on this
// before handing the bounds to a storage engine, because some engines reject or
// misbehave on inverted bounds.
func emptyScanRange(min, max []byte) bool {
	return max != nil && bytes.Compare(min, max) >= 0
}

// inScanRange reports whether k belongs to the half-open interval [min, max).
func inScanRange(k, min, max []byte) bool {
	if min != nil && bytes.Compare(k, min) < 0 {
		return false
	}
	if max != nil && bytes.Compare(k, max) >= 0 {
		return false
	}
	return true
}

// copyBytes returns a caller-owned copy of b. A nil input yields an empty
// non-nil slice so that all engines agree on the representation of an empty
// value (bolt hands back nil for an empty value, pebble hands back an empty
// slice).
func copyBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

// sortAndTruncateScan sorts the <k, v> pairs ascending by bytes.Compare(k) and
// then applies limit (limit <= 0 means unlimited). Sorting must happen BEFORE
// truncation, never the other way around.
func sortAndTruncateScan(keys, values [][]byte, limit int) ([][]byte, [][]byte) {
	if len(keys) == 0 {
		return nil, nil
	}
	idx := make([]int, len(keys))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(i, j int) bool {
		return bytes.Compare(keys[idx[i]], keys[idx[j]]) < 0
	})
	n := len(idx)
	if limit > 0 && limit < n {
		n = limit
	}
	sortedKeys := make([][]byte, n)
	sortedValues := make([][]byte, n)
	for i := 0; i < n; i++ {
		sortedKeys[i] = keys[idx[i]]
		sortedValues[i] = values[idx[i]]
	}
	return sortedKeys, sortedValues
}
