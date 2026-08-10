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

// This file lets the IIP-59 drain enumerate voters without a frozen entry list.
//
// The drain is voter-major: it walks the voter key space and, for each voter it
// lands on, recomputes what that voter is owed. "The voter key space" is the
// union of two indexes -- {_voterIndex}||addr for native buckets and
// {_lsdVoterIndex}||addr for contract-staking ones -- so walking it means
// merging two ordered streams and visiting each address once.
//
// The walk has to be resumable at a bounded cost per block, which rules out
// scanning either index end to end. Both are keyed by a 20-byte address
// immediately after a 1-byte tag, so the first address byte partitions the
// space into 256 shards of roughly equal size that are each a single contiguous
// key range. A block drains whole shards, and can stop part-way through one by
// remembering the last address it visited: shard population is attacker-
// controllable (addresses are cheap and their first byte is grindable), so
// shard-granular resume alone would leave the per-block work unbounded.

// AddressShards is the number of key-space shards the voter walk is split into,
// one per possible value of an address's first byte.
const AddressShards = 256

// _addrLen is the byte length of an IoTeX address body.
const _addrLen = 20

// ShardOf returns the shard an address belongs to.
func ShardOf(addr address.Address) byte {
	if addr == nil {
		return 0
	}
	b := addr.Bytes()
	if len(b) == 0 {
		return 0
	}
	return b[0]
}

// liveVoterIndexPrefixes are the two live index tags the walk merges. They are
// named here rather than at the use site so the _voterIndex tag, which is
// package-private, does not have to be exported for the rewarding protocol.
var liveVoterIndexPrefixes = [2]byte{_voterIndex, contractstaking.LSDVoterIndexPrefix}

// cowVoterIndexKinds are the copy-on-write counterparts of the two live tags.
var cowVoterIndexKinds = [2]eracow.Kind{eracow.KindNativeVoterIndex, eracow.KindLSDVoterIndex}

// rawState is a state value we do not want to decode. The scans below care
// about keys only; decoding every index list to throw it away would double the
// cost of the walk.
type rawState struct{ data []byte }

func (r *rawState) Deserialize(b []byte) error {
	r.data = append(r.data[:0], b...)
	return nil
}

// FrozenShardVoters returns every voter address in one shard that had a native
// or contract-staking bucket index at the era freeze height, ascending and
// deduplicated, skipping everything at or before `after`.
//
// Four ranges are merged, not two. The obvious pair is the two live indexes,
// but a voter who withdraws their last bucket during the drain window has their
// live index key deleted while the copy-on-write layer keeps the value they had
// at the freeze height. Scanning only the live keys would silently drop such a
// voter -- they are owed a share of an era they were part of, and the money
// would fall through to the residual sweep. So the two copy-on-write entry
// ranges are scanned as well; their keys are
// {EntryPrefix}||u64BE(H)||kind||addr, which puts the shard byte at a fixed
// offset and makes them shard-bounded in exactly the same way.
//
// The reverse case -- a voter who acquires their first bucket after the freeze
// -- costs nothing to admit: the copy-on-write layer wrote a tombstone for
// them, and the tombstone is skipped here; even if it were not, the weight
// recompute would resolve them to zero.
//
// Every scan is bounded to this shard's key range. None of them may be issued
// unbounded: the state layer materializes whatever range it is handed before
// any limit is applied, so an unbounded scan is an unbounded amount of work
// inside one block regardless of what the caller intends to consume.
func FrozenShardVoters(
	sr protocol.StateReader,
	window eracow.Window,
	shard byte,
	after []byte,
) ([]address.Address, error) {
	if !window.Open() {
		return nil, errors.New("staking: no era window open")
	}
	var found [][]byte
	for _, prefix := range liveVoterIndexPrefixes {
		addrs, err := scanLiveVoterShard(sr, prefix, shard, after)
		if err != nil {
			return nil, err
		}
		found = append(found, addrs...)
	}
	for _, kind := range cowVoterIndexKinds {
		addrs, err := scanCOWVoterShard(sr, window.FreezeHeight, kind, shard, after)
		if err != nil {
			return nil, err
		}
		found = append(found, addrs...)
	}
	sort.Slice(found, func(i, j int) bool { return bytes.Compare(found[i], found[j]) < 0 })
	out := make([]address.Address, 0, len(found))
	var prev []byte
	for _, raw := range found {
		if prev != nil && bytes.Equal(prev, raw) {
			continue
		}
		addr, err := address.FromBytes(raw)
		if err != nil {
			return nil, errors.Wrap(err, "staking: decode voter address from index key")
		}
		out = append(out, addr)
		prev = raw
	}
	return out, nil
}

// shardRange builds the half-open key range covering one shard, given the fixed
// prefix that precedes the shard byte. The upper bound is the prefix with the
// shard byte incremented; the top shard has no such successor, so it borrows
// the successor of the last prefix byte instead. Both callers pass a prefix
// whose last byte is a tag well below 0xFF, and the assertion below keeps that
// from becoming a silent wrap.
func shardRange(prefix []byte, shard byte) ([]byte, []byte, error) {
	if len(prefix) == 0 {
		return nil, nil, errors.New("staking: empty shard range prefix")
	}
	min := append(append([]byte{}, prefix...), shard)
	var max []byte
	if shard != 0xFF {
		max = append(append([]byte{}, prefix...), shard+1)
		return min, max, nil
	}
	last := prefix[len(prefix)-1]
	if last == 0xFF {
		return nil, nil, errors.New("staking: shard range prefix has no successor")
	}
	max = append([]byte{}, prefix...)
	max[len(max)-1] = last + 1
	return min, max, nil
}

// scanShardKeys runs one bounded, ordered scan and hands back the keys.
//
// It asserts that the keys arrive strictly ascending. That is the contract the
// range scan is documented to provide, and every caller here depends on it: the
// merge dedupes by comparing against the previous key, and the resume point is
// the last key seen. A backend that returned keys out of order would turn both
// into silent underpayment, so this is an error and not a comment.
func scanShardKeys(sr protocol.StateReader, min, max []byte) ([][]byte, error) {
	_, iter, err := sr.States(
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.RangeOption(min, max),
	)
	switch {
	case err == nil:
	case errors.Is(err, state.ErrStateNotExist):
		return nil, nil
	default:
		return nil, err
	}
	keys := make([][]byte, 0, iter.Size())
	var prev []byte
	for i := 0; i < iter.Size(); i++ {
		var v rawState
		key, err := iter.Next(&v)
		if err != nil && !errors.Is(err, state.ErrNilValue) {
			return nil, err
		}
		if prev != nil && bytes.Compare(key, prev) <= 0 {
			return nil, errors.Errorf(
				"staking: range scan returned non-ascending keys (%x after %x)", key, prev,
			)
		}
		prev = key
		keys = append(keys, key)
	}
	return keys, nil
}

// scanLiveVoterShard reads one shard of a live {tag}||addr index.
func scanLiveVoterShard(sr protocol.StateReader, prefix byte, shard byte, after []byte) ([][]byte, error) {
	min, max, err := shardRange([]byte{prefix}, shard)
	if err != nil {
		return nil, err
	}
	if resumed := resumeMin(min, after, shard); resumed != nil {
		min = resumed
	}
	keys, err := scanShardKeys(sr, min, max)
	if err != nil {
		return nil, err
	}
	out := make([][]byte, 0, len(keys))
	for _, key := range keys {
		if len(key) != 1+_addrLen || key[0] != prefix {
			// A key of another shape sorted into the range. The staking
			// namespace is shared, so this is a tag collision, not a stray
			// value: refuse rather than decode 20 bytes of something else as
			// an address.
			return nil, errors.Errorf("staking: unexpected key %x in voter index shard scan", key)
		}
		addr := key[1:]
		if after != nil && bytes.Compare(addr, after) <= 0 {
			continue
		}
		out = append(out, append([]byte{}, addr...))
	}
	return out, nil
}

// scanCOWVoterShard reads one shard of a copy-on-write voter-index entry range.
// Tombstones are dropped: they record that the voter had no index at the freeze
// height, which is precisely the case this walk must not pay.
func scanCOWVoterShard(
	sr protocol.StateReader,
	freezeHeight uint64,
	kind eracow.Kind,
	shard byte,
	after []byte,
) ([][]byte, error) {
	prefix := eracow.EntryKey(freezeHeight, kind, nil)
	min, max, err := shardRange(prefix, shard)
	if err != nil {
		return nil, err
	}
	if resumed := resumeMin(min, after, shard); resumed != nil {
		min = resumed
	}
	keys, err := scanShardKeys(sr, min, max)
	if err != nil {
		return nil, err
	}
	out := make([][]byte, 0, len(keys))
	for _, key := range keys {
		if len(key) != len(prefix)+_addrLen || !bytes.Equal(key[:len(prefix)], prefix) {
			return nil, errors.Errorf("staking: unexpected key %x in era copy shard scan", key)
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
			return nil, err
		}
		if !entry.Exists {
			continue
		}
		out = append(out, append([]byte{}, addr...))
	}
	return out, nil
}

// resumeMin narrows a shard's lower bound to the resume point, or returns nil
// when the resume point does not fall in this shard. Addresses are fixed width,
// so starting at exactly `after` and dropping the equal key is enough; no
// byte-increment is needed.
func resumeMin(min []byte, after []byte, shard byte) []byte {
	if len(after) != _addrLen || after[0] != shard {
		return nil
	}
	// min already ends with the shard byte, which is after[0].
	return append(append([]byte{}, min...), after[1:]...)
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
