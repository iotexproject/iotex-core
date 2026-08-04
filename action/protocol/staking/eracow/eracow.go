// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// Package eracow implements the IIP-59 era copy-on-write layer.
//
// # Why this exists
//
// IIP-59 pays voter rewards in consensus. At an era boundary (height H) the
// protocol freezes a per-delegate work list and then drains it over several
// later blocks. The drain recomputes each voter's weight from buckets rather
// than from a materialized entry list, which means it reads mutable bucket
// state *after* H — and the drain mutates that state itself (compound deposits
// grow the very buckets whose weights later chunks are computed against). Left
// alone, a voter paid in drain block 1 changes the weights voters in blocks
// 2..N are measured with, and the era's split stops being a function of the
// state at H.
//
// This package is the fix: every covered key, on its *first* mutation after H,
// has its as-of-H value copied aside. Reads that must see H go through
// Resolve, which returns the copy when one exists and the live value when one
// does not — correct because "no copy" means "not mutated since H".
//
// # Design decisions a future reader would otherwise have to re-derive
//
//  1. **First write wins, and only the first.** The copy is written only when
//     no copy exists yet. Later mutations in the same era find the entry
//     present and return without touching it. That is what makes the copy the
//     as-of-H value rather than the as-of-last-mutation value.
//
//  2. **Absence is meaningful, so absence is recorded.** A key that did not
//     exist at H but is created afterwards still gets an entry — a tombstone
//     with Exists=false. Without it, Resolve would fall through to the live
//     value and hand the drain a bucket that did not exist at H.
//
//  3. **The era tag is the freeze height H, not an era number.** H is unique,
//     monotonic, and is exactly what the weight recompute needs anyway (see
//     the note on evaluation height below), so tagging by H means the drain
//     never has to translate between two identifiers. Stale eras are
//     identifiable by their tag being != the live window's.
//
//  4. **A journal, not a scan.** GC must be bounded per block and must not
//     depend on range scans over a shared namespace. Every copy also appends
//     one journal record keyed by a monotone per-era sequence number, so GC is
//     a bounded walk of dense integer keys: read journal[i], delete the entry
//     it names, delete the journal record, advance the cursor.
//
//  5. **The gate is checked before any state access.** Pre-activation
//     NewSession returns an inert session that performs zero reads and zero
//     writes, so pre-fork execution is byte-identical to today. This is
//     consensus state; a stray pre-activation write is a hard fork.
//
//  6. **No window, no work.** Between drain completion and the next era
//     boundary there is nothing to protect, and the per-mutation cost is one
//     read of a single small control key — no bucket read, no copy, no write.
//     Pre-activation even that read does not happen.
//
// The values copied aside are the *serialized* forms produced by the caller's
// own state.Serializer, so this package needs no knowledge of staking types
// and cannot drift from them.
package eracow

import (
	"bytes"
	"context"
	"encoding/binary"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

// Namespace is where every record this package writes lives. The COW copies of
// contract-staking buckets do NOT live in their source namespace
// (cs_bucket_<contract>): keeping every record of this layer in one namespace
// is what lets GC, the tag reservations and the key-shape discrimination stay
// in one place. The source namespace is instead encoded in the entry key via
// the Kind byte plus the contract address in the subkey.
const Namespace = state.StakingNamespace

// State key tags, continuing the iota block in
// action/protocol/staking/protocol.go (which reserves 0..8). They are defined
// here rather than there because `staking` imports this package, not the other
// way round — the same arrangement as contractstaking.LSDVoterIndexPrefix. The
// staking package carries compile-time assertions that the two agree.
const (
	// ControlPrefix keys the single control record: {9}.
	ControlPrefix = byte(9)
	// EntryPrefix keys one copied value:
	// {10} || u64BE(freezeHeight) || kind || subkey.
	EntryPrefix = byte(10)
	// JournalPrefix keys one GC journal record:
	// {11} || u64BE(freezeHeight) || u64BE(seq), value = kind || subkey.
	JournalPrefix = byte(11)
)

// Kind names a covered key family. It is part of the entry key, so these
// values are consensus-visible and must never be renumbered.
type Kind byte

const (
	// KindNativeBucket mirrors _stakingNameSpace {1}||u64BE(index).
	// Subkey: u64BE(index).
	KindNativeBucket Kind = 1
	// KindNativeVoterIndex mirrors _stakingNameSpace {2}||voter(20).
	// Subkey: voter(20). Volatile: the whole record is deleted once a voter's
	// last bucket is withdrawn, so a voter can vanish mid-era.
	KindNativeVoterIndex Kind = 2
	// KindLSDBucket mirrors cs_bucket_<contract> keyed by the bucket id.
	// Subkey: contract(20) || u64BE(bucketID).
	//
	// Note the subkey uses big-endian while the live key is little-endian
	// (byteutil.Uint64ToBytes). That is deliberate: the live encoding is fixed
	// by existing state, this one is ours, and big-endian keeps the journal and
	// entry key spaces sorted in id order for anyone reading a dump.
	KindLSDBucket Kind = 3
	// KindLSDVoterIndex mirrors _stakingNameSpace {8}||owner(20), the
	// contract-staking owner index. Subkey: owner(20).
	KindLSDVoterIndex Kind = 4
)

// _controlKey is the single-value key holding the window and GC state.
var _controlKey = []byte{ControlPrefix}

// EntryKey returns the state key of one copied value.
func EntryKey(freezeHeight uint64, kind Kind, subkey []byte) []byte {
	key := make([]byte, 0, 1+8+1+len(subkey))
	key = append(key, EntryPrefix)
	key = appendU64(key, freezeHeight)
	key = append(key, byte(kind))
	return append(key, subkey...)
}

// JournalKey returns the state key of one GC journal record.
func JournalKey(freezeHeight, seq uint64) []byte {
	key := make([]byte, 0, 1+8+8)
	key = append(key, JournalPrefix)
	key = appendU64(key, freezeHeight)
	return appendU64(key, seq)
}

// NativeBucketSubkey builds the subkey for KindNativeBucket.
func NativeBucketSubkey(index uint64) []byte { return appendU64(make([]byte, 0, 8), index) }

// AddrSubkey builds the subkey for the address-keyed kinds.
func AddrSubkey(addr []byte) []byte { return append(make([]byte, 0, len(addr)), addr...) }

// LSDBucketSubkey builds the subkey for KindLSDBucket.
func LSDBucketSubkey(contract []byte, bucketID uint64) []byte {
	out := make([]byte, 0, len(contract)+8)
	out = append(out, contract...)
	return appendU64(out, bucketID)
}

func appendU64(b []byte, v uint64) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], v)
	return append(b, buf[:]...)
}

func appendU32(b []byte, v uint32) []byte {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], v)
	return append(b, buf[:]...)
}

// ---------------------------------------------------------------- records --

// Entry is one copied value. Exists=false is a tombstone: the covered key had
// no value at the freeze height.
type Entry struct {
	Exists bool
	Data   []byte
}

// Serialize implements state.Serializer.
func (e *Entry) Serialize() ([]byte, error) {
	out := make([]byte, 1, 1+len(e.Data))
	if e.Exists {
		out[0] = 1
	}
	return append(out, e.Data...), nil
}

// Deserialize implements state.Deserializer.
func (e *Entry) Deserialize(buf []byte) error {
	if len(buf) < 1 {
		return errors.New("eracow: entry must be at least 1 byte")
	}
	e.Exists = buf[0] != 0
	e.Data = append([]byte{}, buf[1:]...)
	return nil
}

// Encode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (e *Entry) Encode() (systemcontracts.GenericValue, error) {
	data, err := e.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

// Decode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (e *Entry) Decode(v systemcontracts.GenericValue) error { return e.Deserialize(v.PrimaryData) }

// journalRecord names the covered key one copy belongs to, so GC can delete
// the entry without scanning for it.
type journalRecord struct {
	Kind   Kind
	Subkey []byte
}

// Serialize implements state.Serializer.
func (j *journalRecord) Serialize() ([]byte, error) {
	return append([]byte{byte(j.Kind)}, j.Subkey...), nil
}

// Deserialize implements state.Deserializer.
func (j *journalRecord) Deserialize(buf []byte) error {
	if len(buf) < 1 {
		return errors.New("eracow: journal record must be at least 1 byte")
	}
	j.Kind = Kind(buf[0])
	j.Subkey = append([]byte{}, buf[1:]...)
	return nil
}

// Encode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (j *journalRecord) Encode() (systemcontracts.GenericValue, error) {
	data, err := j.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

// Decode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (j *journalRecord) Decode(v systemcontracts.GenericValue) error {
	return j.Deserialize(v.PrimaryData)
}

// GCState is one sealed era awaiting collection. Cursor is the next journal
// sequence to delete; End is the exclusive upper bound recorded at Seal.
type GCState struct {
	FreezeHeight uint64
	Cursor       uint64
	End          uint64
}

// ContractBucketCount is one contract-staking contract's bucket high-water
// mark, frozen at H. Contract is the 20-byte address; NumOfBuckets is the
// value NumOfBuckets(contract) held at H.
type ContractBucketCount struct {
	Contract     []byte
	NumOfBuckets uint64
}

const _contractCountLen = 20 + 8

// Control is the single record describing the live window and the GC backlog.
//
// FreezeHeight == 0 means no window is open. TotalBucketCount is the native
// bucket high-water mark captured at Begin — see the comment on
// Window.TotalBucketCount.
//
// Pending is a FIFO of sealed-but-not-yet-collected eras. In practice it never
// holds more than one entry: an era is thousands of blocks long and GC runs in
// bounded chunks every block, so a sealed era is collected long before the
// next boundary. It is a slice rather than a single value so that a slow GC
// can never silently lose an era's backlog.
type Control struct {
	FreezeHeight     uint64
	TotalBucketCount uint64
	NextSeq          uint64
	Pending          []GCState
	// ContractCounts holds the per-contract LSD bucket high-water marks at H.
	// Bounded by the number of registered contract-staking contracts (single
	// digits), so it lives inline in the control record rather than in its own
	// keyspace.
	ContractCounts []ContractBucketCount
}

const _controlHeaderLen = 8 + 8 + 8 + 4 + 4

// Serialize implements state.Serializer.
func (c *Control) Serialize() ([]byte, error) {
	out := make([]byte, 0, _controlHeaderLen+len(c.Pending)*24+len(c.ContractCounts)*_contractCountLen)
	out = appendU64(out, c.FreezeHeight)
	out = appendU64(out, c.TotalBucketCount)
	out = appendU64(out, c.NextSeq)
	out = appendU32(out, uint32(len(c.Pending)))
	out = appendU32(out, uint32(len(c.ContractCounts)))
	for _, p := range c.Pending {
		out = appendU64(out, p.FreezeHeight)
		out = appendU64(out, p.Cursor)
		out = appendU64(out, p.End)
	}
	for _, cc := range c.ContractCounts {
		if len(cc.Contract) != 20 {
			return nil, errors.Errorf("eracow: contract address must be 20 bytes, got %d", len(cc.Contract))
		}
		out = append(out, cc.Contract...)
		out = appendU64(out, cc.NumOfBuckets)
	}
	return out, nil
}

// Deserialize implements state.Deserializer.
func (c *Control) Deserialize(buf []byte) error {
	if len(buf) < _controlHeaderLen {
		return errors.Errorf("eracow: control record must be at least %d bytes, got %d", _controlHeaderLen, len(buf))
	}
	c.FreezeHeight = binary.BigEndian.Uint64(buf[0:])
	c.TotalBucketCount = binary.BigEndian.Uint64(buf[8:])
	c.NextSeq = binary.BigEndian.Uint64(buf[16:])
	nPending := int(binary.BigEndian.Uint32(buf[24:]))
	nContracts := int(binary.BigEndian.Uint32(buf[28:]))
	rest := buf[_controlHeaderLen:]
	if want := nPending*24 + nContracts*_contractCountLen; len(rest) != want {
		return errors.Errorf("eracow: control record declares %d bytes of body but carries %d", want, len(rest))
	}
	c.Pending = nil
	if nPending > 0 {
		c.Pending = make([]GCState, 0, nPending)
		for i := 0; i < nPending; i++ {
			off := i * 24
			c.Pending = append(c.Pending, GCState{
				FreezeHeight: binary.BigEndian.Uint64(rest[off:]),
				Cursor:       binary.BigEndian.Uint64(rest[off+8:]),
				End:          binary.BigEndian.Uint64(rest[off+16:]),
			})
		}
	}
	c.ContractCounts = nil
	if nContracts > 0 {
		base := nPending * 24
		c.ContractCounts = make([]ContractBucketCount, 0, nContracts)
		for i := 0; i < nContracts; i++ {
			off := base + i*_contractCountLen
			c.ContractCounts = append(c.ContractCounts, ContractBucketCount{
				Contract:     append([]byte{}, rest[off:off+20]...),
				NumOfBuckets: binary.BigEndian.Uint64(rest[off+20:]),
			})
		}
	}
	return nil
}

// Encode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (c *Control) Encode() (systemcontracts.GenericValue, error) {
	data, err := c.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

// Decode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (c *Control) Decode(v systemcontracts.GenericValue) error { return c.Deserialize(v.PrimaryData) }

// ------------------------------------------------------------ persistence --

func readControl(sr protocol.StateReader) (*Control, error) {
	c := &Control{}
	if _, err := sr.State(c,
		protocol.NamespaceOption(Namespace),
		protocol.KeyOption(_controlKey),
	); err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return nil, nil
		}
		return nil, errors.Wrap(err, "eracow: read control record")
	}
	return c, nil
}

func writeControl(sm protocol.StateManager, c *Control) error {
	// Nothing open and nothing pending: drop the key rather than leave a
	// zero-valued record alive forever. The absence of the key is the same
	// signal as a zeroed one and costs no trie node.
	if c.FreezeHeight == 0 && len(c.Pending) == 0 {
		_, err := sm.DelState(
			protocol.NamespaceOption(Namespace),
			protocol.KeyOption(_controlKey),
			protocol.ObjectOption(&Control{}),
		)
		if err != nil && errors.Cause(err) == state.ErrStateNotExist {
			return nil
		}
		return errors.Wrap(err, "eracow: delete control record")
	}
	_, err := sm.PutState(c,
		protocol.NamespaceOption(Namespace),
		protocol.KeyOption(_controlKey),
	)
	return errors.Wrap(err, "eracow: write control record")
}

// --------------------------------------------------------------- lifecycle --

// Enabled reports whether the COW layer may touch the state trie at all.
//
// Bound to the IIP-59 fork gate, exactly like
// staking.voterWeightPersistenceEnabled and
// contractstaking.OwnerIndexEnabled. A context with no feature context (test
// setups, indexer bootstraps) reads as pre-activation so nothing is written by
// accident.
func Enabled(ctx context.Context) bool {
	fCtx, ok := protocol.GetFeatureCtx(ctx)
	return ok && !fCtx.NoVoterRewardDistribution
}

// Window describes the open COW window.
type Window struct {
	// FreezeHeight is H, the height the era's state is frozen at. Zero means
	// no window is open.
	FreezeHeight uint64
	// TotalBucketCount is the native totalBucketCount read at H.
	//
	// putBucket assigns index = count and then increments, and delBucket never
	// decrements, so native bucket indices are strictly monotonic and never
	// reused. Any native bucket whose index is >= this number therefore did not
	// exist at H, and the drain must ignore it. This is a scalar freeze rather
	// than a COW copy on purpose: it is strictly stronger, because it still
	// rejects a post-H bucket even if that bucket's COW copy were somehow
	// missed, whereas a COW copy of the counter would not.
	TotalBucketCount uint64
	// ContractCounts are the per-contract LSD bucket high-water marks at H.
	//
	// The same "never reused, so the id bounds existence" argument holds for
	// contract-staking buckets. All three deployed IIP-13 contracts mint ids
	// from a strictly monotonic private counter with a single write site
	// (`__currTokenId = unsafeInc(__currTokenId)` in V1, `bucketId =
	// __currBucketId = unsafeInc(__currBucketId)` in V2/V3); `_burn` never
	// touches the counter, and `merge` reuses an already-live id rather than
	// minting, so a burned id is never resurrected. The node side tracks the
	// max id it has ever seen — blockindex/contractstaking/cache.go calls it
	// "total number of buckets including burned buckets" and only ever
	// raises it — and persists it through UpdateNumOfBuckets, so it is
	// readable at H as NumOfBuckets(contract).
	ContractCounts []ContractBucketCount
}

// Open reports whether a window is open.
func (w Window) Open() bool { return w.FreezeHeight != 0 }

// NativeBucketExisted reports whether a native bucket index could have existed
// at the freeze height.
//
// totalBucketCount is the *next* index to be assigned, so the last valid index
// at H is TotalBucketCount-1: the bound is exclusive.
func (w Window) NativeBucketExisted(index uint64) bool {
	return index < w.TotalBucketCount
}

// ContractBucketExisted reports whether a contract-staking bucket id could have
// existed at the freeze height.
//
// NumOfBuckets is a max-*seen* id rather than a next-id counter, so the last
// valid id at H is NumOfBuckets itself: the bound is inclusive. Note the
// deliberate off-by-one against NativeBucketExisted.
//
// A contract with no frozen entry had no NumOfBuckets record at H, which means
// no bucket of it existed at H, so everything is rejected.
func (w Window) ContractBucketExisted(contract []byte, id uint64) bool {
	for i := range w.ContractCounts {
		if bytes.Equal(w.ContractCounts[i].Contract, contract) {
			return id <= w.ContractCounts[i].NumOfBuckets
		}
	}
	return false
}

// Begin opens the COW window for the era frozen at the current block height.
//
// Called from the era-boundary freeze, after every state mutation that belongs
// to block H has already been applied — everything written from this point on
// is "after H" and must be copied aside before it changes.
//
// No-op pre-activation. Re-opening a window at the same height is idempotent
// so a replayed boundary does not reset the sequence counter and orphan the
// entries already written under it.
func Begin(
	ctx context.Context,
	sm protocol.StateManager,
	freezeHeight, totalBucketCount uint64,
	contractCounts []ContractBucketCount,
) error {
	if !Enabled(ctx) {
		return nil
	}
	if freezeHeight == 0 {
		return errors.New("eracow: cannot freeze at height 0")
	}
	c, err := readControl(sm)
	if err != nil {
		return err
	}
	if c == nil {
		c = &Control{}
	}
	if c.FreezeHeight == freezeHeight {
		return nil
	}
	if c.FreezeHeight != 0 {
		// A boundary arrived while the previous era's drain was still
		// outstanding. Seal the old window so its copies are still collected;
		// the outgoing drain reads a window that is no longer maintained, which
		// is a correctness problem for that drain, but silently discarding the
		// entries would leak them forever. Callers are expected to prevent
		// this by not starting a new era before the previous drain completes.
		c.Pending = append(c.Pending, GCState{
			FreezeHeight: c.FreezeHeight,
			Cursor:       0,
			End:          c.NextSeq,
		})
	}
	c.FreezeHeight = freezeHeight
	c.TotalBucketCount = totalBucketCount
	c.ContractCounts = contractCounts
	c.NextSeq = 0
	return writeControl(sm, c)
}

// Seal closes the open window and queues its copies for collection. Call it
// when the era's drain completes: from here on nothing needs protecting until
// the next boundary, and the hooks become branch-only no-ops.
//
// No-op pre-activation and when no window is open.
func Seal(ctx context.Context, sm protocol.StateManager) error {
	if !Enabled(ctx) {
		return nil
	}
	c, err := readControl(sm)
	if err != nil || c == nil || c.FreezeHeight == 0 {
		return err
	}
	c.Pending = append(c.Pending, GCState{
		FreezeHeight: c.FreezeHeight,
		Cursor:       0,
		End:          c.NextSeq,
	})
	c.FreezeHeight = 0
	c.TotalBucketCount = 0
	c.ContractCounts = nil
	c.NextSeq = 0
	return writeControl(sm, c)
}

// LoadWindow returns the open window, or the zero Window when none is open.
// Readers (the drain) use it to learn H and the bucket high-water mark.
func LoadWindow(sr protocol.StateReader) (Window, error) {
	c, err := readControl(sr)
	if err != nil || c == nil {
		return Window{}, err
	}
	return Window{
		FreezeHeight:     c.FreezeHeight,
		TotalBucketCount: c.TotalBucketCount,
		ContractCounts:   c.ContractCounts,
	}, nil
}

// CollectGarbage deletes at most max copied entries belonging to sealed eras
// and returns how many it deleted.
//
// Bounded on purpose: a busy era can accumulate tens of thousands of copies
// and deleting them in one block would blow the block budget the drain was
// chunked to respect in the first place. max <= 0 collects nothing.
//
// No-op pre-activation and when the backlog is empty.
func CollectGarbage(ctx context.Context, sm protocol.StateManager, max int) (int, error) {
	if !Enabled(ctx) || max <= 0 {
		return 0, nil
	}
	c, err := readControl(sm)
	if err != nil || c == nil || len(c.Pending) == 0 {
		return 0, err
	}
	deleted := 0
	for deleted < max && len(c.Pending) > 0 {
		gc := &c.Pending[0]
		if gc.Cursor >= gc.End {
			c.Pending = c.Pending[1:]
			continue
		}
		var rec journalRecord
		jKey := JournalKey(gc.FreezeHeight, gc.Cursor)
		_, rErr := sm.State(&rec,
			protocol.NamespaceOption(Namespace),
			protocol.KeyOption(jKey),
		)
		switch {
		case rErr == nil:
			if _, dErr := sm.DelState(
				protocol.NamespaceOption(Namespace),
				protocol.KeyOption(EntryKey(gc.FreezeHeight, rec.Kind, rec.Subkey)),
				protocol.ObjectOption(&Entry{}),
			); dErr != nil && errors.Cause(dErr) != state.ErrStateNotExist {
				return deleted, errors.Wrap(dErr, "eracow: delete copied entry")
			}
			if _, dErr := sm.DelState(
				protocol.NamespaceOption(Namespace),
				protocol.KeyOption(jKey),
				protocol.ObjectOption(&journalRecord{}),
			); dErr != nil && errors.Cause(dErr) != state.ErrStateNotExist {
				return deleted, errors.Wrap(dErr, "eracow: delete journal record")
			}
		case errors.Cause(rErr) == state.ErrStateNotExist:
			// Already collected (a replay, or a partially applied chunk).
			// Advancing past it is the only sound move: the sequence is dense
			// by construction, so a hole cannot mean "look further on".
		default:
			return deleted, errors.Wrap(rErr, "eracow: read journal record")
		}
		gc.Cursor++
		deleted++
	}
	// Drop any era whose backlog is exhausted, so the FIFO does not keep a
	// finished era alive until the next call.
	for len(c.Pending) > 0 && c.Pending[0].Cursor >= c.Pending[0].End {
		c.Pending = c.Pending[1:]
	}
	if err := writeControl(sm, c); err != nil {
		return deleted, err
	}
	return deleted, nil
}

// PendingGarbage reports how many copied entries are still queued for
// collection across all sealed eras.
func PendingGarbage(sr protocol.StateReader) (uint64, error) {
	c, err := readControl(sr)
	if err != nil || c == nil {
		return 0, err
	}
	var n uint64
	for _, p := range c.Pending {
		if p.End > p.Cursor {
			n += p.End - p.Cursor
		}
	}
	return n, nil
}

// ----------------------------------------------------------------- session --

// Session is the per-state-manager handle the mutation hooks call into.
//
// Construct one alongside the state manager it wraps (per action for the
// native path, per event handler / per commit for the contract-staking path)
// and keep it for that object's lifetime.
//
// Only an *open* window is cached. "No window is open" is re-checked on every
// use, because the window is opened by a consensus action — the era boundary's
// FreezePollSnapshot — that can run after this session was built and before it
// is used again. Caching that negative would silently skip every copy for the
// rest of the object's life, which is the one failure this layer cannot
// tolerate. The re-check is a single small state read, and it happens only
// after the fork gate opens.
//
// The positive direction is cached, and Snapshot re-validates it against the
// control record before it commits to a write, so a session that outlived a
// Seal misses a copy rather than writing one into the wrong era.
//
// Mutations that happen in block H before Begin runs see no window and are
// correctly not copied: they are part of the state being frozen.
type Session struct {
	sm      protocol.StateManager
	enabled bool

	window Window
}

// NewSession returns a session for sm. Pre-activation the session is inert and
// performs no state access whatsoever — not even a read — so pre-fork
// execution is byte-identical to what it was before this package existed.
func NewSession(ctx context.Context, sm protocol.StateManager) *Session {
	return &Session{sm: sm, enabled: Enabled(ctx) && sm != nil}
}

// Active reports whether a window is open, reading the control record until it
// finds one. See the type comment for why the negative answer is not cached.
func (s *Session) Active() (bool, error) {
	if s == nil || !s.enabled {
		return false, nil
	}
	if s.window.Open() {
		return true, nil
	}
	c, err := readControl(s.sm)
	if err != nil {
		return false, err
	}
	if c != nil {
		s.window = Window{
			FreezeHeight:     c.FreezeHeight,
			TotalBucketCount: c.TotalBucketCount,
			ContractCounts:   c.ContractCounts,
		}
	}
	return s.window.Open(), nil
}

// FreezeHeight returns the open window's H, or 0.
func (s *Session) FreezeHeight() (uint64, error) {
	active, err := s.Active()
	if err != nil || !active {
		return 0, err
	}
	return s.window.FreezeHeight, nil
}

// SnapshotNativeBucket records the as-of-H value of one native bucket.
//
// prior is the value the bucket held before the mutation in flight, or nil if
// it is being created. A bucket whose index is beyond the high-water mark did
// not exist at H no matter what it holds now, so it is recorded as a tombstone
// rather than as a copy of its current contents. That makes the COW layer
// self-sufficient: a reader that forgets the high-water-mark check still gets
// "did not exist at H" out of Resolve rather than a post-H bucket.
func (s *Session) SnapshotNativeBucket(index uint64, prior state.Serializer) error {
	active, err := s.Active()
	if err != nil || !active {
		return err
	}
	if !s.window.NativeBucketExisted(index) {
		prior = nil
	}
	return s.Snapshot(KindNativeBucket, NativeBucketSubkey(index), prior)
}

// SnapshotContractBucket records the as-of-H value of one contract-staking
// bucket. See SnapshotNativeBucket for the tombstone rule.
func (s *Session) SnapshotContractBucket(contract []byte, id uint64, prior state.Serializer) error {
	active, err := s.Active()
	if err != nil || !active {
		return err
	}
	if !s.window.ContractBucketExisted(contract, id) {
		prior = nil
	}
	return s.Snapshot(KindLSDBucket, LSDBucketSubkey(contract, id), prior)
}

// Snapshot records the as-of-H value of one covered key, if it has not been
// recorded already.
//
// prior is the value the key held immediately before the mutation in flight,
// or nil when the key held nothing. Callers pass what they have already read —
// every hook site in this repo reads the previous value anyway — so this costs
// no extra read of the covered key itself.
//
// Returns without writing when: the fork gate is closed, no window is open, or
// a copy already exists (first write wins).
func (s *Session) Snapshot(kind Kind, subkey []byte, prior state.Serializer) error {
	active, err := s.Active()
	if err != nil || !active {
		return err
	}
	entryKey := EntryKey(s.window.FreezeHeight, kind, subkey)
	var existing Entry
	_, err = s.sm.State(&existing,
		protocol.NamespaceOption(Namespace),
		protocol.KeyOption(entryKey),
	)
	switch {
	case err == nil:
		// First write already won.
		return nil
	case errors.Cause(err) == state.ErrStateNotExist:
	default:
		return errors.Wrap(err, "eracow: probe copied entry")
	}

	// Re-read the control record before committing to a write. Active() caches
	// the window for the session's lifetime, and the window can be sealed by an
	// action executed after this session was built. Writing under a sealed
	// window would leak an entry past the journal end recorded at Seal.
	c, err := readControl(s.sm)
	if err != nil {
		return err
	}
	if c == nil || c.FreezeHeight != s.window.FreezeHeight {
		s.window = Window{}
		return nil
	}

	entry := &Entry{}
	if prior != nil {
		data, sErr := prior.Serialize()
		if sErr != nil {
			return errors.Wrap(sErr, "eracow: serialize prior value")
		}
		entry.Exists = true
		entry.Data = data
	}
	if _, pErr := s.sm.PutState(entry,
		protocol.NamespaceOption(Namespace),
		protocol.KeyOption(entryKey),
	); pErr != nil {
		return errors.Wrap(pErr, "eracow: write copied entry")
	}
	if _, pErr := s.sm.PutState(&journalRecord{Kind: kind, Subkey: subkey},
		protocol.NamespaceOption(Namespace),
		protocol.KeyOption(JournalKey(s.window.FreezeHeight, c.NextSeq)),
	); pErr != nil {
		return errors.Wrap(pErr, "eracow: write journal record")
	}
	c.NextSeq++
	return writeControl(s.sm, c)
}

// -------------------------------------------------------------- resolution --

// ErrNotFrozen is returned by Resolve when the covered key had no value at the
// freeze height. It is a normal outcome, not a failure: buckets are created and
// destroyed constantly and the drain must simply skip those.
var ErrNotFrozen = errors.New("eracow: key did not exist at the freeze height")

// Resolve reads a covered key as of freezeHeight into obj.
//
// A copy present under the era tag is authoritative — it is by construction the
// value at H. No copy means the key has not been mutated since H, so the live
// value *is* the value at H; that is the whole invariant this layer maintains,
// and it is why the common case (an untouched bucket) costs one extra read
// rather than a copy.
//
// liveNS/liveKey name the covered key in its own namespace, which for
// contract-staking buckets is not this package's namespace.
//
// Returns ErrNotFrozen when a tombstone says the key did not exist at H, and
// state.ErrStateNotExist when there is no copy and no live value either.
func Resolve(
	sr protocol.StateReader,
	freezeHeight uint64,
	kind Kind,
	subkey []byte,
	liveNS string,
	liveKey []byte,
	obj interface{},
) error {
	if freezeHeight == 0 {
		return errors.New("eracow: resolve requires a non-zero freeze height")
	}
	var entry Entry
	_, err := sr.State(&entry,
		protocol.NamespaceOption(Namespace),
		protocol.KeyOption(EntryKey(freezeHeight, kind, subkey)),
	)
	switch {
	case err == nil:
		if !entry.Exists {
			return ErrNotFrozen
		}
		return state.Deserialize(obj, entry.Data)
	case errors.Cause(err) == state.ErrStateNotExist:
		_, lErr := sr.State(obj,
			protocol.NamespaceOption(liveNS),
			protocol.KeyOption(liveKey),
		)
		return lErr
	default:
		return errors.Wrap(err, "eracow: read copied entry")
	}
}
