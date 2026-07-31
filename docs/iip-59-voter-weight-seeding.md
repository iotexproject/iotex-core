# IIP-59 voter weight seeding (A4)

**Status:** implemented. Sections 4 and 6 describe a *rejected* earlier design;
see §4.0 for why, and §4.3 for what was built.
**Depends on:** A1–A3 (per-`(candidate, voter)` weights as first-class committed
state; `loadVoterWeightView` replacing the startup rebuild-and-compare).
**Blocks:** setting the IIP-59 activation height.

---

## 1. Why this exists

A1–A3 made the per-`(candidate, voter)` weights committed state. Startup now
*loads* them instead of recomputing and comparing against a digest, which is
what removed the non-recoverable startup halt: there is only one derivation, so
there is nothing for a restarting node to disagree with.

That leaves exactly one gap. `loadVoterWeightView` recognises two states:

| state | what it means | what it does |
|---|---|---|
| no entries in state | pre-activation — nothing has been written yet | seed the view from buckets |
| entries in state | post-seeding — state is authoritative | load them |

There is a third state it cannot recognise and must never encounter: **activated,
but only the pairs touched since activation written.** In that state the view is
silently missing every voter who has not staked since the fork, and the first era
freeze pays out against it.

Closing that gap is this document. Until it lands, **the activation height must
not be set.** A1–A3 are inert before activation, so they can ship first; A4 is
the hard prerequisite for turning the fork on.

## 2. Why it cannot be a single block

The natural implementation — enumerate every bucket at the activation height and
write all the entries — costs about **7s at 30k buckets**, roughly 3× the 2.5s
Dardanelles block budget. Every node would stall on the same block.

The same constraint already shaped the era drain, which is chunked across blocks
behind `epochDrainCursor`. Seeding reuses that shape.

## 3. Constraints

1. **Deterministic.** Every node must write the same entries in the same blocks.
   No wall-clock, no goroutines, no iteration over Go maps.
2. **Bounded per block.** A configurable number of buckets per block, in the
   same spirit as `VoterBudgetPerBlock`.
3. **Correct under concurrent mutation.** Buckets keep changing during the
   seeding window. A bucket must be counted exactly once, whether it is mutated
   before, during, or after the block that seeds it.
4. **No new halt.** A problem during seeding must degrade — delay a payout —
   never stop block production. This is the same rule the delegate-profile
   bridge already follows at `FreezePollSnapshot`.
5. **Inert before activation.** Nothing written, no control flow changed, so the
   deployment window stays byte-identical to the previous release.

## 4.0 What was actually built, and why not the frontier

The frontier design in §4.1–4.2 below was written first and then abandoned
during implementation, because a simpler formulation turned out to be available.
It is kept here because the reasoning is what justifies the replacement.

The frontier exists to answer "has this bucket's weight been written yet?" at
every hook. That question only matters if seeding writes *deltas*. It does not:
A2's `voterWeightBase.persist` writes each touched pair's **absolute** current
weight (`weightOf(ref)`), never an increment.

Two consequences follow immediately:

1. A pair mutated during the seeding window is written correctly by the ordinary
   per-block path, whether or not the cursor has reached it.
2. The cursor writing that pair again later is idempotent — same value.

So no hook needs to know about the cursor, no bucket needs an index, and nothing
has to reason about which side of a boundary a mutation fell on.

That also removes the second reason seeding was going to read buckets. At the
activation height the in-memory view **already holds the complete table**: the
staking hooks maintain it on both sides of the fork, and a restart rebuilds it
from buckets. Seeding is therefore a *flush of what is already in memory*, not a
recomputation from buckets.

What disappeared with the frontier:

- the per-source cursor, the source enum, and pinning `End` per source
- threading bucket identity through `addCandidateVotes`/`subCandidateVotes` to
  ~20 call sites
- the `ContractBucketObserver` interface change to carry `(addr, id)`
- open question 11.1 entirely — no bucket index is consulted anywhere
- open question 11.2 — no indexer is read during seeding, so a lagging indexer
  cannot stall it

### 4.3 The implemented design

`voterWeightSeedCursor` (`voter_weight_seed.go`) holds a **key position**, not an
offset: the last `(candidate, voter)` pair written. Each block, in
`CreatePreStates`, `seedVoterWeights` takes the next `VoterWeightSeedBatchSize`
pairs in key order strictly after that position, calls `MarkForRewrite` on each,
and advances the cursor. `viewData.Commit` then writes them through the same
path any other change takes. A short batch means the walk reached the end, and
the cursor is marked `Done`.

A key position rather than an offset is the one subtlety worth keeping: voters
are added and removed while the flush runs, and a numeric offset would skip or
repeat entries as the set shifts underneath it.

Ordering is `(candidate, voter)` ascending — `refLess` — matching the order
`persist` already writes in.

---

## 4. The core idea: a seeded frontier *(rejected — see §4.0)*

Seeding walks buckets in a fixed order under a monotonic cursor. At any moment
the bucket space is split in three, and **which side of the frontier a bucket is
on decides whether the staking hooks touch the view for it**:

```
   already seeded          not yet seeded           created after the boundary
 ┌────────────────────┬────────────────────────┬──────────────────────────────┐
 │  index < Cursor    │ Cursor ≤ index < End   │      index ≥ End             │
 │  hooks APPLY       │ hooks SKIP             │      hooks APPLY             │
 └────────────────────┴────────────────────────┴──────────────────────────────┘
```

- **`index < Cursor` — seeded.** The entry exists with the weight the bucket had
  when it was seeded. Later mutations must be applied normally.
- **`Cursor ≤ index < End` — pending.** No entry exists yet. Deltas are skipped,
  because seeding will read the bucket's *current* state when it reaches it and
  write the final weight in one shot. Applying a delta here would either create a
  partial entry that seeding then double-counts, or hit the negative-delta swallow
  path.
- **`index ≥ End` — new.** Created after the seeding boundary was pinned, so
  seeding will never reach it. The normal `+weight` from the create hook is the
  only thing that ever writes it, which is exactly right.

`End` is pinned once, when seeding starts, from the bucket count at that height.
Pinning it is what makes the third region well-defined and finite work.

### 4.1 Why this is correct

The argument rests on one ordering property: **`CreatePreStates` runs before any
action handler in the same block.** So within a block the frontier advances
first, then the block's actions run against the new frontier. There is no
interleaving to reason about.

Given that, for each bucket exactly one of these holds:

| bucket is | seeded value | hook deltas | result |
|---|---|---|---|
| seeded in an earlier block | weight at seed time | all later deltas applied | correct |
| seeded in this block | weight after the previous block | this block's deltas applied (frontier already advanced) | correct |
| pending, mutated | — (skipped) | none applied | seeding later reads final state → correct |
| pending, withdrawn | — (key absent, skipped) | none applied | no entry, and none should exist → correct |
| created after `End` | never seeded | create/mutate deltas applied | correct |

The dangerous middle case — a bucket mutated while pending — resolves because
seeding reads *state*, not a delta: whatever the bucket looks like when the
cursor arrives is what gets written.

### 4.2 What this costs

The frontier test has to be reachable from every hook. `applyVoterWeightDelta`
currently takes `(csm, candIdentifier, voter, delta)` and knows nothing about
which bucket produced the delta. It will need the bucket identity, which means
threading a bucket reference (or `(source, index)`) through
`addCandidateVotes` / `subCandidateVotes` to every call site.

That is the bulk of the implementation work, and it is mechanical: the C1 choke
point already funnels every site through two functions, and
`TestCandidateVoteMutationsUseChokePoint` fails the build if a site is added that
bypasses them.

## 5. State

A new 1-byte tag alongside `_voterWeights` in `protocol.go`:

```go
// _voterWeightSeed is the tag for the single-value seeding cursor.
// Full key: {_voterWeightSeed}.
_voterWeightSeed
```

```go
// voterWeightSeedCursor tracks the one-time population of voter weight entries
// after IIP-59 activates. Consensus state: every node advances it identically.
type voterWeightSeedCursor struct {
    // Source selects which bucket space is being walked. Sources are processed
    // in a fixed order: native, then each contract indexer by contract address.
    Source     uint32
    // Index is the next bucket index to seed within Source.
    Index      uint64
    // End is the exclusive upper bound for Source, pinned when that source
    // started. Buckets at or beyond it are handled by the normal hooks.
    End        uint64
    // Done is set when every source has been walked. Once true the cursor is
    // never read again on the hot path.
    Done       bool
    DoneHeight uint64
}
```

`Source` is an enum, not a contract address, so the encoded cursor stays small
and the ordering is explicit:

```go
const (
    seedSourceNative  uint32 = iota // bucketKey(0) .. bucketKey(NumOfNativeBucket)
    seedSourceContractV1
    seedSourceContractV2
    seedSourceContractV3
    seedSourceComplete
)
```

Serialization follows the existing rewarding cursor: a protobuf message, with
`Encode`/`Decode` for Erigon dual-storage.

## 6. Per-block execution

In `staking.Protocol.CreatePreStates`, which already runs every block with a
`StateManager`, a `FeatureCtx` and a `BlockCtx`:

```
if featureCtx.NoVoterRewardDistribution        -> return       (pre-activation: inert)
cursor := readSeedCursor(sm)
if cursor == nil                               -> cursor = startNativeSeed(sm)   (activation block)
if cursor.Done                                 -> return       (one state read, then out)
seedNextBatch(ctx, sm, cursor, g.VoterWeightSeedBatchSize)
writeSeedCursor(sm, cursor)
```

`seedNextBatch` for `seedSourceNative`:

1. Take up to `BatchSize` indices from `cursor.Index`, capped at `cursor.End`.
2. Read those buckets by key. A missing key is a withdrawn bucket — skip it.
3. For each bucket: skip if `isUnstaked()`; resolve its candidate (skip if
   absent); compute `CalculateVoteWeight` with the same self-stake gate
   `buildVoterWeightView` uses; apply `+weight` to the view.
4. Advance `cursor.Index`. When it reaches `cursor.End`, move to the next
   source and pin its `End` from `TotalBucketCount(height)`.

For contract sources the same loop uses `BucketsByIndices(indices, height)`.

The writes themselves need no new machinery: applying to the view marks the
pairs touched, and A2's `Commit` persists exactly those at the end of the block.

### 6.1 Ordering within a batch

Buckets are walked in ascending index order and the view aggregates per
`(candidate, voter)`, so the resulting entries do not depend on batch
boundaries — a batch size of 500 and one of 5000 produce identical state. That
makes `VoterWeightSeedBatchSize` a tunable rather than a consensus-relevant
constant *within a single chain*, though it must of course be identical across
nodes, so it belongs in genesis.

## 7. Coordination with the era freeze

The first era boundary can fall inside the seeding window. `FreezePollSnapshot`
must not snapshot a partial view.

It already has the right behaviour for this, and it should be reused rather than
replaced: when the view is unavailable it leaves `Entries` nil, and rewarding
keeps that delegate's voter pool pending until a later era has an eligible
snapshot. The change is one condition:

```go
// Seeding still in progress: the view holds only part of the voter set, so no
// snapshot taken now would be payable. Leave Entries nil and let the pool roll
// into the next era, the same way an unavailable view already does.
if !seedCursorDone(sm) { vw = nil }
```

Consequence: if seeding spans an era boundary, that era's voter rewards are paid
one era later. Nothing is lost — the pool accrues in `PendingBlockRewardPool` —
and no delegate is treated differently from any other.

**Sizing rule:** seeding must comfortably finish inside one era. At 30k buckets
and 2000/block that is ~15 blocks, against ~34,560 blocks per era. Even a
pathological batch size finishes with four orders of magnitude of headroom, so
in practice the first era after activation pays normally.

### 7.1 A wrinkle: system-action validation runs *before* `CreatePreStates`

In `workingSet.process`, when `FeatureCtx.PreStateSystemAction` is set,
`validatePostSystemActions` runs **before** the `CreatePreStates` loop, while the
handlers run after it. A validator and its handler therefore observe different
cursor values within the same block: the validator sees the cursor as of the
previous block, the handler sees it after this block's batch.

IIP-59 already has paired validate/handle gates on
`NoVoterRewardDistribution` (`validations.go`, `handlers.go`), so this pattern is
established — but the seeding gate must not be duplicated into a validator that
would then disagree with its handler at the block seeding completes. **The
`Done` check belongs in the handler path only** (`FreezePollSnapshot`), never in
an action validator.

## 8. Genesis parameters

```go
// VoterWeightSeedBatchSize is the number of buckets whose voter weight is
// written per block during the one-time seeding that follows IIP-59
// activation. 0 means seed everything in a single block, which is only
// appropriate for tests and small chains.
VoterWeightSeedBatchSize uint64 `yaml:"voterWeightSeedBatchSize"`
```

Default **2000**, measured by `BenchmarkVoterWeightSeedFlush`.

Per-block cost, in-memory state manager (lower bound — see the fidelity caveat
below), one op = one block of list + mark + write:

| batch | mainnet (7.5k pairs) | ceiling (30k pairs) |
|---|---|---|
| 500 | 3.4 ms/block, 16 blocks | 3.8 ms/block, 60 blocks |
| 1000 | 6.5 ms/block, 8 blocks | 7.7 ms/block, 30 blocks |
| **2000** | **13 ms/block, 4 blocks** | **15 ms/block, 15 blocks** |
| 5000 | 19 ms/block, 2 blocks | 36 ms/block, 6 blocks |

Cost is linear in batch size at roughly **7.5 µs/pair**, so the whole flush is
~225 ms at ceiling scale no matter how it is divided; the batch size only trades
per-block cost against block count.

At 2000 the ceiling case costs **15 ms against a 2500 ms budget — 0.6%** — and
completes in 15 blocks (~37 s at a 2.5 s interval), against an era of ~34,560
blocks. Even allowing a 10× penalty for real trie writes over the in-memory
stand-in, that is 150 ms, or 6% of the budget.

Going to 5000 would save 9 blocks and cost 2.4× the per-block time. There is no
reason to trade headroom for finishing half a minute sooner, so 2000 stands.

**Fidelity caveat**, matching `BenchmarkFreezeSnapshotNativeEnumeration`: writes
go to `testdb.NewMockStateManager`, an in-memory map. Production writes go
through the state trie and will be slower by an implementation-dependent factor.
These are a lower bound on per-block cost, and therefore an upper bound on the
batch size they justify.

## 9. Edge cases

| case | handling |
|---|---|
| withdrawn bucket inside a pending range | key absent → skipped; no entry written, none should exist |
| bucket unstaked while pending | seeding sees `isUnstaked()` → skipped, matching `buildVoterWeightView` |
| bucket changes candidate while pending | seeded against its final candidate; no stale entry, because none was written |
| candidate not found for a bucket | skipped, exactly as `buildVoterWeightView` does |
| new bucket during seeding | index ≥ `End` → normal hooks, never seeded |
| contract indexer lags the chain height | seeding for that source cannot proceed; see open question 11.2 |
| chain restarts mid-seeding | cursor is committed state; the view loads the partial entries and resumes from `cursor.Index` |
| seeding never starts (activation block missed) | cannot happen: `CreatePreStates` runs on every block, and the cursor is created on the first post-activation block, whichever that is |

## 10. Test plan

1. **Seed-equals-rebuild.** For random bucket sets, seed in batches of 1, 7 and
   all-at-once; assert all three produce the same view hash as
   `buildVoterWeightView` over the same buckets.
2. **Mutation during seeding.** Extend the `vwModel` harness in
   `voter_weight_equivalence_test.go` with a seeding phase: interleave random
   mutations with batch advances and assert the final view equals a rebuild.
   This is the test that actually exercises the frontier rule, and it is the
   one that would catch a double count.
3. **Frontier boundary.** Targeted cases for a bucket mutated in the same block
   it is seeded, one mutated the block before, and one the block after.
4. **Restart mid-seed.** Persist, drop the in-memory view, reload, resume;
   assert the result matches an uninterrupted run.
5. **Era boundary inside the window.** Assert `Entries` is nil, the pool stays
   pending, and the next era pays the full amount.
6. **Idempotence.** Re-running a batch (same cursor) must not double count.
7. **Pre-activation inertness.** `CreatePreStates` writes nothing and reads at
   most nothing while `NoVoterRewardDistribution` is true.

## 11. Open questions *(11.1 and 11.2 are void — see §4.0)*

**11.1 Do contract `VoteBucket`s carry a usable index? (blocking)**
The evidence points both ways and this decides the cursor's shape:

- *For:* `ContractStakingIndexer` exposes `BucketsByIndices([]uint64, height)`
  and `TotalBucketCount(height)` — exactly the shape chunking needs — and
  `blockindex/contractstaking/bucket.go:18` sets `Index` from the NFT token id.
- *Against:* `staking.Protocol.convertToVoteBucket` (`protocol.go:1197`) builds
  contract `VoteBucket`s with a hardcoded `Index: 0`, and the comment in
  `buildVoterWeightView` relies on that ("contract buckets, which always have
  Index = 0"). Today that is harmless, because the self-stake gate keys off
  `ContractAddress == ""` rather than the index — but the frontier rule cannot
  use an index that is always zero.

So the indexers evidently *have* per-bucket ids while at least one conversion
path discards them. **Verify per indexer (V1/V2/V3) which `VoteBucket`s reach
the hooks and whether they carry the id.** If the id is not reliably present at
the hook, the frontier for contract sources needs a different discriminator —
`(contract address, token id)` — and `applyVoterWeightDelta` must receive it.

**11.2 What if a contract indexer lags at the activation height?**
`Protocol.Start` already waits for indexers via `delayTolerantIndexer`, but
seeding runs inside block processing where waiting is not an option. Options:
(a) hold the cursor on that source until the indexer catches up, delaying
completion; (b) treat a lagging indexer as an empty source and rely on the
normal hooks. (a) is safe but can stall seeding indefinitely; (b) silently
under-counts. Neither is obviously right; needs a decision before implementation.

**11.3 Should seeding also emit a receipt log?**
The drain emits `DelegateDistributed`. A `VoterWeightSeeded(source, from, to)`
log would let off-chain consumers observe progress without polling state. Cheap,
but it is new log surface — worth deciding deliberately rather than by default.

## 12. What landed

| piece | where |
|---|---|
| cursor state, batch flush, completion check | `voter_weight_seed.go` |
| `MarkForRewrite` / `SeedPairsAfter` on the view | `voter_weight_view.go` |
| per-block invocation | `staking.Protocol.CreatePreStates` |
| freeze guard | `poll_snapshot.go`, handler-side only (see §7.1) |
| `_voterWeightSeed` tag | `protocol.go` |
| `VoterWeightSeedBatchSize`, default 2000 | `blockchain/genesis/genesis.go` |
| tests | `voter_weight_seed_test.go` |

Tests cover batch-size invariance (1/2/5/7/36/1000/all — all must produce an
identical table), concurrent mutation across 20 randomized runs, cursor
serialization, and walk ordering under resumption.

All of it is inert before activation: `seedVoterWeights` returns immediately
while `NoVoterRewardDistribution` is true, and `loadVoterWeightView` builds from
buckets until the cursor reports `Done`.

### Remaining before the activation height can be set

- **A5**, the non-consensus audit. A1–A3 removed the halt by removing the
  detector; the audit is what puts a detector back without putting it on the
  consensus path.

The batch size is settled: see §8 for the measurements behind 2000.
