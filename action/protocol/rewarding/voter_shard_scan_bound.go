// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"bytes"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/state"
)

// This file bounds the *scan* half of the per-block voter budget.
//
// The budget in runVoterDistributionChunk caps how many voters a block pays.
// On its own that caps only the work the block commits, not the work it does:
// staking.FrozenShardVoters reads one whole shard before the loop sees its
// first voter, and shard population is attacker-controlled. Addresses are cheap
// and their first byte is grindable, so an attacker can concentrate voters into
// a single shard and make one block materialize that entire shard -- four range
// scans, a merge, a sort, and one extra state read per copy-on-write key --
// before a single voter is paid. Under an even distribution a shard holds a few
// hundred voters and this never shows, which is exactly why it has to be
// bounded rather than measured.
//
// The bound is a limit pushed into the scans plus a coverage bound read back
// out of them.
//
// # Why a limit alone is not enough
//
// FrozenShardVoters merges four strictly ascending key streams. Truncating each
// stream to N and merging yields the correct first N distinct addresses only if
// every scanned key becomes a candidate address. That does not hold here:
// scanCOWVoterShard drops copy-on-write tombstones *after* scanning, so a
// truncated stream can hand back fewer addresses than it scanned keys. Take the
// first N addresses of the merged result and you can walk past an address that
// a *different*, also-truncated stream would have produced below that point --
// and because ResumeVoter then advances past it, that voter is never revisited
// and never paid.
//
// # The coverage bound
//
// Instead of counting results, the scans report how far they are known to be
// complete. A scan that came back short of its limit saw its whole range, so it
// is complete to the end of the shard. A scan that came back at exactly its
// limit is complete only up to its last key. The shard's coverage is the
// minimum of those, and only voters at or below the coverage may be paid: for
// such a voter every stream was scanned past its key, so if the voter has an
// entry anywhere it was seen.
//
// The coverage doubles as the resume point. It is a raw 20-byte address body
// and need not be a real voter -- ResumeVoter is used only as an exclusive
// lower bound, and staking.FrozenShardVoters requires nothing of it beyond
// length and shard byte.
//
// # Why the limit is pushed down at all
//
// The limit reaches all the way down. db.kvStoreWithBuffer.ScanRange cannot
// pass the caller's raw limit to its base scan -- the write buffer can promote
// keys that sort before a truncated base scan's cutoff -- but it does scan the
// base with limit plus the number of buffered deletes in range, which is the
// smallest bound derivable from the buffer alone -- a tighter one would have to
// know which of those deletes actually hit a base key, which costs the very
// scan it is trying to bound (see the comment there). So a limit of N reads
// O(N + buffered deletes) keys off the bottom-most kv store rather than the
// whole shard, which is what takes the grinding attack in the first paragraph
// off the table.
//
// Everything above that read is bounded by the same limit, and that is where
// most of the cost was anyway: iterator materialization, per-key copies, the
// merge, the sort, the address decode, and -- dominant by a wide margin -- the
// one extra sr.State() per copy-on-write key.

// _voterScanKeyBudgetPerVoter is how many scanned index keys a block may spend
// per unit of its voter budget. Four, because FrozenShardVoters merges four
// streams and a voter may legitimately appear in all four (native index, LSD
// index, and a copy-on-write entry for each), times two for headroom so that a
// shard dense in tombstones or in already-visited addresses still makes forward
// progress within a single block instead of stalling on a round that pays
// nobody.
const _voterScanKeyBudgetPerVoter = 8

// _completeCoverage is the coverage value meaning "this shard was scanned to
// the end": the maximum 20-byte address body, which no real address exceeds.
var _completeCoverage = bytes.Repeat([]byte{0xFF}, 20)

// boundedShardReader decorates a StateReader so that every ranged States() call
// it forwards carries a Limit, and records what each of those scans returned.
//
// Only ranged scans are touched. Point reads (State) and keyed States() calls
// pass straight through: the copy-on-write tombstone check is a point read per
// key, and limiting it would be meaningless.
type boundedShardReader struct {
	protocol.StateReader
	limit int
	scans []*recordingIterator
}

// newBoundedShardReader returns a reader that caps each ranged scan so it
// yields at most limit *new* keys. A limit <= 0 disables the decoration
// entirely, which is the unbounded-budget configuration.
//
// The injected limit is limit+1, not limit. staking.resumeMin narrows a
// resumed shard scan to start at the resume address itself and lets the
// post-scan filter drop it, so the resume key occupies the first slot of every
// resumed stream. Injecting limit would hand back a range whose only key is the
// resume point: no new key, a coverage bound equal to the resume point, and a
// cursor that never advances. The extra slot is what guarantees forward
// progress on every round.
func newBoundedShardReader(sr protocol.StateReader, limit int) *boundedShardReader {
	if limit > 0 {
		limit++
	}
	return &boundedShardReader{StateReader: sr, limit: limit}
}

// States forwards the call with a Limit appended when the call is a ranged
// scan, and wraps the iterator so the last key it yields is observable.
func (r *boundedShardReader) States(opts ...protocol.StateOption) (uint64, state.Iterator, error) {
	if r.limit <= 0 {
		return r.StateReader.States(opts...)
	}
	cfg, err := protocol.CreateStateConfig(opts...)
	if err != nil {
		return 0, nil, err
	}
	if cfg.Keys != nil || (cfg.RangeMin == nil && cfg.RangeMax == nil) {
		return r.StateReader.States(opts...)
	}
	// A fresh slice: append must not write into the caller's backing array.
	bounded := make([]protocol.StateOption, 0, len(opts)+1)
	bounded = append(bounded, opts...)
	bounded = append(bounded, protocol.LimitOption(r.limit))

	height, iter, err := r.StateReader.States(bounded...)
	if err != nil {
		// Includes state.ErrStateNotExist, which scanShardKeys reads as an
		// empty range. An unreadable range contributes no coverage constraint
		// because it contributed no keys either.
		return height, iter, err
	}
	rec := &recordingIterator{Iterator: iter, limit: r.limit}
	r.scans = append(r.scans, rec)
	return height, rec, nil
}

// keysScanned is the total number of keys the ranged scans materialized. The
// drain debits its per-block key budget by this so that a round which pays
// nobody -- every key a tombstone, or every address already visited -- still
// costs what it actually read.
func (r *boundedShardReader) keysScanned() int {
	n := 0
	for _, s := range r.scans {
		n += s.Size()
	}
	return n
}

// coverage returns the address body up to which this shard is known to have
// been scanned in full, and whether that is the whole shard.
//
// A scan that returned fewer results than its limit saw its entire range. One
// that returned exactly its limit may have been cut short, so it constrains
// coverage to its last key's address body. The shard's coverage is the smallest
// such constraint, because a voter is only safe to pay once *every* stream has
// been scanned past it.
func (r *boundedShardReader) coverage() ([]byte, bool, error) {
	out := _completeCoverage
	complete := true
	for _, s := range r.scans {
		if s.Size() < s.limit {
			continue
		}
		last := s.lastKey
		if len(last) < _voterAddrLen {
			// A truncated scan whose keys were never consumed, or whose keys
			// are too short to carry an address. Either is a contract
			// violation by the scan layer, not a state condition; there is no
			// safe coverage to report, so refuse rather than guess one.
			return nil, false, errors.Errorf(
				"rewarding: bounded shard scan returned %d keys at its limit but no usable last key",
				s.Size())
		}
		tail := last[len(last)-_voterAddrLen:]
		if bytes.Compare(tail, out) < 0 {
			out = tail
			complete = false
		} else if bytes.Equal(tail, out) {
			complete = bytes.Equal(out, _completeCoverage)
		}
	}
	if complete {
		return _completeCoverage, true, nil
	}
	return append([]byte(nil), out...), false, nil
}

// _voterAddrLen is the byte length of an address body, the trailing component
// of every voter-index key the shard walk scans.
const _voterAddrLen = 20

// recordingIterator forwards a state.Iterator and remembers the last key it
// handed out, which is what the coverage bound is derived from.
type recordingIterator struct {
	state.Iterator
	limit   int
	lastKey []byte
}

func (it *recordingIterator) Next(s interface{}) ([]byte, error) {
	key, err := it.Iterator.Next(s)
	if len(key) > 0 {
		it.lastKey = append(it.lastKey[:0], key...)
	}
	return key, err
}

// voterScanLimit is the per-stream result cap for one shard read, given how
// many voters the block may still pay and how many index keys it may still
// read. Zero means unbounded, which is the pre-fork / unconfigured-budget path.
func voterScanLimit(remainingVoters uint32, remainingKeys int) int {
	if remainingVoters == 0 {
		return 0
	}
	limit := int(remainingVoters)
	if remainingKeys > 0 && remainingKeys < limit {
		limit = remainingKeys
	}
	if limit < 1 {
		limit = 1
	}
	return limit
}
