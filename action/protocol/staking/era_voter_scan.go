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

// _addrLen is the byte length of an IoTeX address body.
const _addrLen = 20

// FrozenVoterPage is one bounded page of the voter address space at an era's
// freeze height. Voters are ascending and deduplicated. Next is an exclusive
// resume bound; it may name a scanned address at which no voter exists. Done
// means the requested address range was covered completely.
type FrozenVoterPage struct {
	Voters      []address.Address
	Next        []byte
	Done        bool
	KeysScanned int
}

// liveVoterIndexPrefixes are the two live index tags the walk merges. They are
// named here rather than at the use site so the _voterIndex tag, which is
// package-private, does not have to be exported for the rewarding protocol.
var liveVoterIndexPrefixes = [2]byte{_voterIndex, contractstaking.LSDVoterIndexPrefix}

// cowVoterIndexKinds are the copy-on-write counterparts of the two live tags.
var cowVoterIndexKinds = [2]eracow.Kind{eracow.KindNativeVoterIndex, eracow.KindLSDVoterIndex}

// rawState is a state value we do not want to decode. Voter enumeration uses
// index keys only.
type rawState struct{ data []byte }

func (r *rawState) Deserialize(b []byte) error {
	r.data = append(r.data[:0], b...)
	return nil
}

// FrozenVotersPage returns at most voterLimit voters in [start, end), resuming
// strictly after `after`. A nil end means the top of the address space. A zero
// voterLimit or keyLimit disables that bound.
//
// The four streams are voter indexes, not bucket lists: native live,
// contract-staking live, and their two copy-on-write counterparts. The COW
// streams retain voters whose final bucket index was deleted after the freeze.
// This function owns their merge so rewarding can paginate voters without
// depending on the staking storage layout.
//
// keyLimit bounds index enumeration independently of voter processing. COW
// tombstones and duplicate addresses can consume keys without producing a
// voter, so a raw `Limit(voterLimit)` cannot be resumed safely. The merge only
// returns addresses at or below the minimum point covered by every truncated
// stream and uses that coverage point as Next when necessary.
func FrozenVotersPage(
	sr protocol.StateReader,
	window eracow.Window,
	start []byte,
	end []byte,
	after []byte,
	voterLimit int,
	keyLimit int,
) (FrozenVoterPage, error) {
	if !window.Open() {
		return FrozenVoterPage{}, errors.New("staking: no era window open")
	}
	if err := validateVoterRange(start, end, after); err != nil {
		return FrozenVoterPage{}, err
	}
	page := FrozenVoterPage{Next: append([]byte(nil), after...)}
	for voterLimit <= 0 || len(page.Voters) < voterLimit {
		if keyLimit > 0 && page.KeysScanned >= keyLimit {
			return page, nil
		}
		remainingVoters := voterLimit - len(page.Voters)
		roundLimit := remainingVoters
		if voterLimit <= 0 {
			roundLimit = 0
		}
		if keyLimit > 0 {
			streamCount := len(liveVoterIndexPrefixes) + len(cowVoterIndexKinds)
			remainingKeys := keyLimit - page.KeysScanned
			if remainingKeys < streamCount {
				return page, nil
			}
			perStream := remainingKeys / streamCount
			if roundLimit <= 0 || perStream < roundLimit {
				roundLimit = perStream
			}
		}
		resumeBefore := append([]byte(nil), page.Next...)
		round, err := scanFrozenVoterRange(sr, window, start, end, page.Next, roundLimit)
		if err != nil {
			return FrozenVoterPage{}, err
		}
		page.KeysScanned += round.keysScanned
		for _, raw := range round.voters {
			if voterLimit > 0 && len(page.Voters) >= voterLimit {
				return page, nil
			}
			addr, err := address.FromBytes(raw)
			if err != nil {
				return FrozenVoterPage{}, errors.Wrap(err, "staking: decode voter address from index key")
			}
			page.Voters = append(page.Voters, addr)
			page.Next = append(page.Next[:0], raw...)
		}
		if round.done {
			page.Done = true
			return page, nil
		}
		if len(page.Next) == 0 || bytes.Compare(round.coverage, page.Next) > 0 {
			page.Next = append(page.Next[:0], round.coverage...)
		} else if bytes.Equal(page.Next, resumeBefore) {
			return FrozenVoterPage{}, errors.Errorf(
				"staking: bounded voter scan made no progress (coverage %x <= resume %x)",
				round.coverage, page.Next,
			)
		}
	}
	return page, nil
}

type frozenVoterScanRound struct {
	voters      [][]byte
	coverage    []byte
	done        bool
	keysScanned int
}

type voterIndexScan struct {
	entries  []frozenVoterIndexEntry
	coverage []byte
	done     bool
	keys     int
}

// frozenVoterIndexEntry is one live or copied voter-index key. For COW keys,
// exists is the value the index had at the freeze height; false is a tombstone
// that must override a matching live key created after the freeze.
type frozenVoterIndexEntry struct {
	voter  []byte
	exists bool
}

func validateVoterRange(start, end, after []byte) error {
	if len(start) != _addrLen {
		return errors.Errorf("staking: voter range start has length %d, want %d", len(start), _addrLen)
	}
	if end != nil && len(end) != _addrLen {
		return errors.Errorf("staking: voter range end has length %d, want %d", len(end), _addrLen)
	}
	if end != nil && bytes.Compare(start, end) > 0 {
		return errors.New("staking: voter range start exceeds end")
	}
	if after != nil {
		if len(after) != _addrLen {
			return errors.Errorf("staking: voter resume has length %d, want %d", len(after), _addrLen)
		}
		if bytes.Compare(after, start) < 0 || (end != nil && bytes.Compare(after, end) >= 0) {
			return errors.New("staking: voter resume lies outside requested range")
		}
	}
	return nil
}

func scanFrozenVoterRange(
	sr protocol.StateReader,
	window eracow.Window,
	start, end, after []byte,
	limit int,
) (frozenVoterScanRound, error) {
	scans := make([]voterIndexScan, 0, len(liveVoterIndexPrefixes)+len(cowVoterIndexKinds))
	for _, prefix := range liveVoterIndexPrefixes {
		scan, err := scanLiveVoterRange(sr, prefix, start, end, after, limit)
		if err != nil {
			return frozenVoterScanRound{}, err
		}
		scans = append(scans, scan)
	}
	for _, kind := range cowVoterIndexKinds {
		scan, err := scanCOWVoterRange(sr, window.FreezeHeight, kind, start, end, after, limit)
		if err != nil {
			return frozenVoterScanRound{}, err
		}
		scans = append(scans, scan)
	}

	round := frozenVoterScanRound{done: true}
	for _, scan := range scans {
		round.keysScanned += scan.keys
		if !scan.done && (round.done || bytes.Compare(scan.coverage, round.coverage) < 0) {
			round.coverage = append(round.coverage[:0], scan.coverage...)
			round.done = false
		}
	}
	type voterPresence struct {
		voter   []byte
		present [2]bool
	}
	found := make(map[string]*voterPresence)
	for i, scan := range scans {
		family := i % len(liveVoterIndexPrefixes)
		isCOW := i >= len(liveVoterIndexPrefixes)
		for _, entry := range scan.entries {
			if !round.done && bytes.Compare(entry.voter, round.coverage) > 0 {
				continue
			}
			key := string(entry.voter)
			presence := found[key]
			if presence == nil {
				presence = &voterPresence{voter: entry.voter}
				found[key] = presence
			}
			if isCOW {
				// First-write-wins COW records are authoritative for this
				// family. In particular, a tombstone suppresses a live index
				// that was first created after the freeze.
				presence.present[family] = entry.exists
			} else {
				presence.present[family] = true
			}
		}
	}
	for _, presence := range found {
		if presence.present[0] || presence.present[1] {
			round.voters = append(round.voters, presence.voter)
		}
	}
	sort.Slice(round.voters, func(i, j int) bool {
		return bytes.Compare(round.voters[i], round.voters[j]) < 0
	})
	return round, nil
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
		var v rawState
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

func scanLiveVoterRange(
	sr protocol.StateReader,
	prefix byte,
	start, end, after []byte,
	limit int,
) (voterIndexScan, error) {
	keys, done, err := scanVoterRangeKeys(sr, []byte{prefix}, start, end, after, limit)
	if err != nil {
		return voterIndexScan{}, err
	}
	out := voterIndexScan{done: done, keys: len(keys)}
	for _, key := range keys {
		if len(key) != 1+_addrLen || key[0] != prefix {
			return voterIndexScan{}, errors.Errorf("staking: unexpected key %x in voter index range scan", key)
		}
		out.entries = append(out.entries, frozenVoterIndexEntry{
			voter: append([]byte{}, key[1:]...), exists: true,
		})
	}
	if !done && len(keys) > 0 {
		out.coverage = append([]byte(nil), keys[len(keys)-1][1:]...)
	}
	return out, nil
}

func scanCOWVoterRange(
	sr protocol.StateReader,
	freezeHeight uint64,
	kind eracow.Kind,
	start, end, after []byte,
	limit int,
) (voterIndexScan, error) {
	prefix := eracow.EntryKey(freezeHeight, kind, nil)
	keys, done, err := scanVoterRangeKeys(sr, prefix, start, end, after, limit)
	if err != nil {
		return voterIndexScan{}, err
	}
	out := voterIndexScan{done: done, keys: len(keys)}
	for _, key := range keys {
		if len(key) != len(prefix)+_addrLen || !bytes.Equal(key[:len(prefix)], prefix) {
			return voterIndexScan{}, errors.Errorf("staking: unexpected key %x in era copy voter scan", key)
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
			return voterIndexScan{}, err
		}
		out.entries = append(out.entries, frozenVoterIndexEntry{
			voter: append([]byte{}, addr...), exists: entry.Exists,
		})
	}
	if !done && len(keys) > 0 {
		out.coverage = append([]byte(nil), keys[len(keys)-1][len(prefix):]...)
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

// FrozenVoterCandidates returns the distinct candidates one voter's frozen
// buckets point at, ascending by address bytes.
//
// The drain needs this to know which delegates a voter can be owed by. It
// exists so the per-candidate weight recompute is run only for candidates the
// voter actually has a bucket with, instead of once per delegate in the work
// list; the recompute itself stays the single implementation of the weight
// rule.
func FrozenVoterCandidates(
	sr protocol.StateReader,
	window eracow.Window,
	voter address.Address,
) ([]address.Address, error) {
	if !window.Open() {
		return nil, errors.New("staking: no era window open")
	}
	var raw [][]byte
	indices, err := FrozenNativeBucketIndices(sr, window, voter)
	if err != nil {
		return nil, err
	}
	for _, index := range indices {
		bkt, err := FrozenNativeBucket(sr, window, index)
		switch {
		case err == nil:
		case errors.Is(err, ErrBucketPostFreeze), errors.Cause(err) == state.ErrStateNotExist:
			continue
		default:
			return nil, err
		}
		if bkt.Candidate == nil || bkt.isUnstaked() {
			continue
		}
		raw = append(raw, bkt.Candidate.Bytes())
	}
	refs, err := FrozenContractBucketRefs(sr, window, voter)
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		bkt, err := FrozenContractBucket(sr, window, ref.Contract, ref.BucketID)
		switch {
		case err == nil:
		case errors.Is(err, ErrBucketPostFreeze), errors.Cause(err) == state.ErrStateNotExist:
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

// TestOnlyBeginEraCOWWindow opens an era copy-on-write window directly, without
// going through a poll-snapshot freeze. Only tests may call it: production
// opens the window inside FreezePollSnapshot so it cannot be opened without the
// snapshot that names what the era froze.
func TestOnlyBeginEraCOWWindow(ctx context.Context, sm protocol.StateManager, freezeHeight uint64) error {
	return beginEraCOWWindow(ctx, sm, freezeHeight)
}
