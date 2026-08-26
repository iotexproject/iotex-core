// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"context"
	"sort"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/eracow"
	"github.com/iotexproject/iotex-core/v2/state"
)

// _addressLength is the byte length of an IoTeX address body.
const _addressLength = 20

// FrozenVoterPage is one bounded page of the voter address space at an era's
// freeze height. Voters are ascending and deduplicated. ResumeAfter is an
// exclusive cursor; it may name a scanned address at which no voter exists.
// Complete means the requested address range was covered completely.
type FrozenVoterPage struct {
	Voters           []address.Address
	ResumeAfter      []byte
	Complete         bool
	IndexKeysScanned int
}

type voterIndexFamily uint8

const (
	nativeVoterIndex voterIndexFamily = iota
	contractVoterIndex
	voterIndexFamilyCount
)

// voterIndexSource identifies one of the four ordered key streams merged by a
// frozen voter scan. family tells the merge which live index a COW record
// overrides; cowKind distinguishes a COW stream from a live prefix.
type voterIndexSource struct {
	family     voterIndexFamily
	livePrefix byte
	cowKind    eracow.Kind
}

func (s voterIndexSource) isCOW() bool { return s.cowKind != 0 }

var frozenVoterIndexSources = [...]voterIndexSource{
	{family: nativeVoterIndex, livePrefix: _voterIndex},
	{family: contractVoterIndex, livePrefix: contractstaking.LSDVoterIndexPrefix},
	{family: nativeVoterIndex, cowKind: eracow.KindNativeVoterIndex},
	{family: contractVoterIndex, cowKind: eracow.KindLSDVoterIndex},
}

// ignoredStateValue satisfies the state iterator's decoder. Voter enumeration
// uses index keys only, so retaining each value would be wasted work.
type ignoredStateValue struct{}

func (*ignoredStateValue) Deserialize([]byte) error { return nil }

// ScanFrozenVoters returns at most voterLimit voters in [rangeStart, rangeEnd),
// resuming strictly after resumeAfter. A nil rangeEnd means the top of the
// address space. A zero voterLimit or indexKeyLimit disables that bound.
//
// The four streams are voter indexes, not bucket lists: native live,
// contract-staking live, and their two copy-on-write counterparts. The COW
// streams retain voters whose final bucket index was deleted after the freeze.
// This function owns their merge so rewarding can paginate voters without
// depending on the staking storage layout.
//
// indexKeyLimit bounds index enumeration independently of voter processing. COW
// tombstones and duplicate addresses can consume keys without producing a
// voter, so a raw `Limit(voterLimit)` cannot be resumed safely. The merge only
// returns addresses at or below the minimum point covered by every truncated
// stream and uses that point as ResumeAfter when necessary.
func ScanFrozenVoters(
	sr protocol.StateReader,
	window eracow.Window,
	rangeStart []byte,
	rangeEnd []byte,
	resumeAfter []byte,
	voterLimit int,
	indexKeyLimit int,
) (FrozenVoterPage, error) {
	if !window.Open() {
		return FrozenVoterPage{}, errors.New("staking: no era window open")
	}
	if err := validateVoterScanRange(rangeStart, rangeEnd, resumeAfter); err != nil {
		return FrozenVoterPage{}, err
	}
	page := FrozenVoterPage{ResumeAfter: append([]byte(nil), resumeAfter...)}
	for voterLimit <= 0 || len(page.Voters) < voterLimit {
		if indexKeyLimit > 0 && page.IndexKeysScanned >= indexKeyLimit {
			return page, nil
		}
		remainingVoters := voterLimit - len(page.Voters)
		perSourceKeyLimit := remainingVoters
		if voterLimit <= 0 {
			perSourceKeyLimit = 0
		}
		if indexKeyLimit > 0 {
			streamCount := len(frozenVoterIndexSources)
			remainingKeys := indexKeyLimit - page.IndexKeysScanned
			if remainingKeys < streamCount {
				return page, nil
			}
			perStream := remainingKeys / streamCount
			if perSourceKeyLimit <= 0 || perStream < perSourceKeyLimit {
				perSourceKeyLimit = perStream
			}
		}
		resumeBefore := append([]byte(nil), page.ResumeAfter...)
		batch, err := scanFrozenVoterIndexes(
			sr, window, rangeStart, rangeEnd, page.ResumeAfter, perSourceKeyLimit,
		)
		if err != nil {
			return FrozenVoterPage{}, err
		}
		page.IndexKeysScanned += batch.indexKeysScanned
		for _, raw := range batch.voters {
			if voterLimit > 0 && len(page.Voters) >= voterLimit {
				return page, nil
			}
			addr, err := address.FromBytes(raw)
			if err != nil {
				return FrozenVoterPage{}, errors.Wrap(err, "staking: decode voter address from index key")
			}
			page.Voters = append(page.Voters, addr)
			page.ResumeAfter = append(page.ResumeAfter[:0], raw...)
		}
		if batch.complete {
			page.Complete = true
			return page, nil
		}
		if len(page.ResumeAfter) == 0 || bytes.Compare(batch.scannedThrough, page.ResumeAfter) > 0 {
			page.ResumeAfter = append(page.ResumeAfter[:0], batch.scannedThrough...)
		} else if bytes.Equal(page.ResumeAfter, resumeBefore) {
			return FrozenVoterPage{}, errors.Errorf(
				"staking: bounded voter scan made no progress (scanned through %x <= resume %x)",
				batch.scannedThrough, page.ResumeAfter,
			)
		}
	}
	return page, nil
}

type frozenVoterScanBatch struct {
	voters           [][]byte
	scannedThrough   []byte
	complete         bool
	indexKeysScanned int
}

type voterIndexScanResult struct {
	source           voterIndexSource
	entries          []frozenVoterIndexEntry
	scannedThrough   []byte
	complete         bool
	indexKeysScanned int
}

// frozenVoterIndexEntry is one live or copied voter-index key. For COW keys,
// exists is the value the index had at the freeze height; false is a tombstone
// that must override a matching live key created after the freeze.
type frozenVoterIndexEntry struct {
	voter  []byte
	exists bool
}

func validateVoterScanRange(start, end, resumeAfter []byte) error {
	if len(start) != _addressLength {
		return errors.Errorf("staking: voter range start has length %d, want %d", len(start), _addressLength)
	}
	if end != nil && len(end) != _addressLength {
		return errors.Errorf("staking: voter range end has length %d, want %d", len(end), _addressLength)
	}
	if end != nil && bytes.Compare(start, end) > 0 {
		return errors.New("staking: voter range start exceeds end")
	}
	if resumeAfter != nil {
		if len(resumeAfter) != _addressLength {
			return errors.Errorf("staking: voter resume has length %d, want %d", len(resumeAfter), _addressLength)
		}
		if bytes.Compare(resumeAfter, start) < 0 || (end != nil && bytes.Compare(resumeAfter, end) >= 0) {
			return errors.New("staking: voter resume lies outside requested range")
		}
	}
	return nil
}

func scanFrozenVoterIndexes(
	sr protocol.StateReader,
	window eracow.Window,
	start, end, after []byte,
	limit int,
) (frozenVoterScanBatch, error) {
	scans := make([]voterIndexScanResult, 0, len(frozenVoterIndexSources))
	for _, source := range frozenVoterIndexSources {
		scan, err := scanVoterIndexSource(sr, window, source, start, end, after, limit)
		if err != nil {
			return frozenVoterScanBatch{}, err
		}
		scans = append(scans, scan)
	}

	batch := frozenVoterScanBatch{complete: true}
	for _, scan := range scans {
		batch.indexKeysScanned += scan.indexKeysScanned
		if !scan.complete && (batch.complete || bytes.Compare(scan.scannedThrough, batch.scannedThrough) < 0) {
			batch.scannedThrough = append(batch.scannedThrough[:0], scan.scannedThrough...)
			batch.complete = false
		}
	}
	type voterPresence struct {
		voter       []byte
		present     [voterIndexFamilyCount]bool
		cowRecorded [voterIndexFamilyCount]bool
	}
	presenceByVoter := make(map[string]*voterPresence)
	for _, scan := range scans {
		for _, entry := range scan.entries {
			if !batch.complete && bytes.Compare(entry.voter, batch.scannedThrough) > 0 {
				continue
			}
			key := string(entry.voter)
			presence := presenceByVoter[key]
			if presence == nil {
				presence = &voterPresence{voter: entry.voter}
				presenceByVoter[key] = presence
			}
			if scan.source.isCOW() {
				// First-write-wins COW records are authoritative for this
				// family. In particular, a tombstone suppresses a live index
				// that was first created after the freeze.
				presence.present[scan.source.family] = entry.exists
				presence.cowRecorded[scan.source.family] = true
			} else if !presence.cowRecorded[scan.source.family] {
				presence.present[scan.source.family] = true
			}
		}
	}
	for _, presence := range presenceByVoter {
		if presence.present[0] || presence.present[1] {
			batch.voters = append(batch.voters, presence.voter)
		}
	}
	sort.Slice(batch.voters, func(i, j int) bool {
		return bytes.Compare(batch.voters[i], batch.voters[j]) < 0
	})
	return batch, nil
}

func scanVoterRangeKeys(sr protocol.StateReader, prefix, start, end, after []byte, limit int) ([][]byte, bool, error) {
	minAddr := start
	if after != nil {
		var ok bool
		minAddr, ok = nextVoterAddress(after)
		if !ok || (end != nil && bytes.Compare(minAddr, end) >= 0) {
			return nil, true, nil
		}
	}
	min := append(append([]byte{}, prefix...), minAddr...)
	var max []byte
	if end != nil {
		max = append(append([]byte{}, prefix...), end...)
	} else {
		if len(prefix) == 0 || prefix[len(prefix)-1] == 0xFF {
			return nil, false, errors.New("staking: voter index prefix has no successor")
		}
		max = append([]byte{}, prefix...)
		max[len(max)-1]++
	}
	opts := []protocol.StateOption{
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.RangeOption(min, max),
	}
	if limit > 0 {
		opts = append(opts, protocol.LimitOption(limit))
	}
	_, iter, err := sr.States(
		opts...,
	)
	switch {
	case err == nil:
	case errors.Is(err, state.ErrStateNotExist):
		return nil, true, nil
	default:
		return nil, false, err
	}
	keys := make([][]byte, 0, iter.Size())
	var prev []byte
	for i := 0; i < iter.Size(); i++ {
		var v ignoredStateValue
		key, err := iter.Next(&v)
		if err != nil && !errors.Is(err, state.ErrNilValue) {
			return nil, false, err
		}
		if prev != nil && bytes.Compare(key, prev) <= 0 {
			return nil, false, errors.Errorf(
				"staking: range scan returned non-ascending keys (%x after %x)", key, prev,
			)
		}
		owned := append([]byte(nil), key...)
		prev = owned
		keys = append(keys, owned)
	}
	return keys, limit <= 0 || len(keys) < limit, nil
}

func scanVoterIndexSource(
	sr protocol.StateReader,
	window eracow.Window,
	source voterIndexSource,
	start, end, resumeAfter []byte,
	limit int,
) (voterIndexScanResult, error) {
	if source.isCOW() {
		return scanCOWVoterIndex(
			sr, window.FreezeHeight, source, start, end, resumeAfter, limit,
		)
	}
	return scanLiveVoterIndex(sr, source, start, end, resumeAfter, limit)
}

func scanLiveVoterIndex(
	sr protocol.StateReader,
	source voterIndexSource,
	start, end, after []byte,
	limit int,
) (voterIndexScanResult, error) {
	keys, complete, err := scanVoterRangeKeys(
		sr, []byte{source.livePrefix}, start, end, after, limit,
	)
	if err != nil {
		return voterIndexScanResult{}, err
	}
	out := voterIndexScanResult{
		source: source, complete: complete, indexKeysScanned: len(keys),
	}
	for _, key := range keys {
		if len(key) != 1+_addressLength || key[0] != source.livePrefix {
			return voterIndexScanResult{}, errors.Errorf("staking: unexpected key %x in voter index range scan", key)
		}
		out.entries = append(out.entries, frozenVoterIndexEntry{
			voter: append([]byte{}, key[1:]...), exists: true,
		})
	}
	if !complete && len(keys) > 0 {
		out.scannedThrough = append([]byte(nil), keys[len(keys)-1][1:]...)
	}
	return out, nil
}

func scanCOWVoterIndex(
	sr protocol.StateReader,
	freezeHeight uint64,
	source voterIndexSource,
	start, end, after []byte,
	limit int,
) (voterIndexScanResult, error) {
	prefix := eracow.EntryKey(freezeHeight, source.cowKind, nil)
	keys, complete, err := scanVoterRangeKeys(sr, prefix, start, end, after, limit)
	if err != nil {
		return voterIndexScanResult{}, err
	}
	out := voterIndexScanResult{
		source: source, complete: complete, indexKeysScanned: len(keys),
	}
	for _, key := range keys {
		if len(key) != len(prefix)+_addressLength || !bytes.Equal(key[:len(prefix)], prefix) {
			return voterIndexScanResult{}, errors.Errorf("staking: unexpected key %x in era copy voter scan", key)
		}
		addr := key[len(prefix):]
		if after != nil && bytes.Compare(addr, after) <= 0 {
			continue
		}
		entry := &eracow.Entry{}
		if _, err := sr.State(entry,
			protocol.NamespaceOption(_stakingNameSpace),
			protocol.KeyOption(key),
		); err != nil {
			if errors.Is(err, state.ErrStateNotExist) {
				continue
			}
			return voterIndexScanResult{}, err
		}
		out.entries = append(out.entries, frozenVoterIndexEntry{
			voter: append([]byte{}, addr...), exists: entry.Exists,
		})
	}
	if !complete && len(keys) > 0 {
		out.scannedThrough = append([]byte(nil), keys[len(keys)-1][len(prefix):]...)
	}
	return out, nil
}

func nextVoterAddress(addr []byte) ([]byte, bool) {
	next := append([]byte(nil), addr...)
	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] != 0 {
			return next, true
		}
	}
	return nil, false
}

// FrozenCandidatesForVoter returns the distinct candidates one voter's frozen
// buckets point at, ascending by address bytes.
//
// The drain needs this to know which delegates a voter can be owed by. It
// exists so the per-candidate weight recompute is run only for candidates the
// voter actually has a bucket with, instead of once per delegate in the work
// list; the recompute itself stays the single implementation of the weight
// rule.
func FrozenCandidatesForVoter(
	sr protocol.StateReader,
	window eracow.Window,
	voter address.Address,
) ([]address.Address, error) {
	if !window.Open() {
		return nil, errors.New("staking: no era window open")
	}
	nativeReader := newCandidateStateReader(sr)
	contractReader := contractstaking.NewStateReader(sr)
	var raw [][]byte
	indices, err := nativeReader.FrozenNativeBucketIndices(window, voter)
	if err != nil {
		return nil, err
	}
	for _, index := range indices {
		bkt, err := nativeReader.FrozenNativeBucket(window, index)
		switch {
		case err == nil:
		case errors.Is(err, eracow.ErrBucketPostFreeze), errors.Cause(err) == state.ErrStateNotExist:
			continue
		default:
			return nil, err
		}
		if bkt.Candidate == nil || bkt.isUnstaked() {
			continue
		}
		raw = append(raw, bkt.Candidate.Bytes())
	}
	refs, err := contractReader.FrozenBucketRefs(window, voter)
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		bkt, err := contractReader.FrozenBucket(window, ref.Contract, ref.BucketID)
		switch {
		case err == nil:
		case errors.Is(err, eracow.ErrBucketPostFreeze), errors.Cause(err) == state.ErrStateNotExist:
			continue
		default:
			return nil, err
		}
		if bkt.Candidate == nil {
			continue
		}
		raw = append(raw, bkt.Candidate.Bytes())
	}
	sort.Slice(raw, func(i, j int) bool { return bytes.Compare(raw[i], raw[j]) < 0 })
	out := make([]address.Address, 0, len(raw))
	var prev []byte
	for _, b := range raw {
		if prev != nil && bytes.Equal(prev, b) {
			continue
		}
		addr, err := address.FromBytes(b)
		if err != nil {
			return nil, errors.Wrap(err, "staking: decode candidate address from frozen bucket")
		}
		out = append(out, addr)
		prev = b
	}
	return out, nil
}

// TestOnlyBeginEraCOWWindow opens an era copy-on-write window directly. Only
// tests may call it; production uses the poll protocol's explicit
// FreezeCandidateRewardSnapshots then BeginEraCOWWindow sequence.
func TestOnlyBeginEraCOWWindow(ctx context.Context, sm protocol.StateManager, freezeHeight uint64) error {
	return BeginEraCOWWindow(ctx, sm, freezeHeight)
}
