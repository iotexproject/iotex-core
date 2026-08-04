# IIP-59 reward distribution architecture — as implemented

**Status:** describes the code on `iip-59/consolidated-pr-5-through-5.5b`
(PR iotexproject/iotex-core#4953). IIP-59 is **not activated**: every mechanism
below is behind `!NoVoterRewardDistribution` and no activation height is set.

**Supersedes** the era-based *proposal* that previously occupied this file. That
document predicted a `VoterRewardEraCursor`, a per-era `VoterWeightSnapshot`, and
a Phase 3 background compound sweep. None of the three were built under those
names; what shipped is described here. Where a difference is worth knowing it is
called out as such rather than quietly rewritten.

---

## 1. What the problem was

Legacy voter reward distribution runs off-chain (Hermes). IIP-59 moves it into
the protocol. The naive on-chain shape — split every delegate's voter pool
across every voter inside the epoch-last block — concentrates ~27,020 per-voter
operations into one block, measured at **~1.15s in-memory / ~3.4s trie-backed**
against a **2.5s** Dardanelles block budget. The distribution therefore has to
be spread across blocks, and spreading it across blocks is what creates every
other problem this document is about: once payouts span blocks, the inputs they
divide by keep moving underneath them.

Three constraints shape everything below.

1. **Determinism.** Every node must pay the same voter the same amount in the
   same block. No map iteration order, no wall clock, no unbounded state scans
   whose cost depends on data layout.
2. **Bounded per-block work.** A configured voter budget, not a delegate count:
   voter counts per delegate are attacker-influenced, delegate counts are not.
3. **Degrade, never halt.** A failure in the distribution must cost a delayed
   payout, not a stopped chain. This is a locked project decision and it is why
   §7 looks the way it does.

## 2. Shape of the settlement

A **settlement** distributes one era's accrued voter pools. It has two phases
and they run in different blocks.

```
era boundary block H                    continuation blocks H+1, H+2, ...
──────────────────────────────────────  ───────────────────────────────────────
PutPollResult                            CreatePostSystemActions emits one
  └── FreezePollSnapshot                 VoterRewardChunk per block while an
        ├── per-delegate scalars         incomplete cursor exists
        └── beginEraCOWWindow(H)           └── GrantVoterRewardChunk
GrantEpochReward                                ├── walk the voter key space
  ├── per-delegate commission/voter split       ├── recompute weights at H
  ├── credit voter share to pending pool        ├── pay each voter once
  └── persistDrainCursor (plan + progress)      └── checkpoint the cursor
                                         final chunk:
                                           ├── residual/orphan sweep
                                           ├── SealEraCOWWindow
                                           └── mark cursor Completed
```

Phase A is cheap and fixed-size (per-delegate scalars). Phase B is the
proportional-to-voters work, and it is the part that is chunked.

## 3. The freeze

### 3.1 What is frozen, and as what

`staking.FreezePollSnapshot` writes one `CandidatePollSnapshot` per delegate at
the boundary block H (`action/protocol/staking/poll_snapshot.go`). The snapshot
is **scalars only**:

| field | why it is frozen |
|---|---|
| `BlockCommissionBasisPoints`, `EpochCommissionBasisPoints`, `Registered` | the DelegateProfile contract can change between H and the last chunk |
| `OnchainRewardEnabled` | opt-in must not flip mid-settlement |
| `TotalWeight` | the denominator of every share; it is `candidate.Votes` at H, and `Votes` keeps moving during the drain window |
| `SnapshotHash` | join key stamped into every partial `DelegateDistributed` log so off-chain consumers can reassemble one settlement's per-block logs |
| `FreezeHeight` (H) | the height every weight recompute is evaluated at |
| `SelfStakeBucketIdx` | the only candidate field the weight recompute reads |

There is **no materialized per-voter entry list**. An earlier design froze
`(voter, weight)` pairs per delegate; at mainnet scale that is ~27k entries of
state churn per era, and it needed a whole second mechanism (`VoterWeightView`
plus an activation-height seeding cursor — see
`docs/iip-59-voter-weight-seeding.md`, withdrawn) to produce the pairs cheaply.
Both are gone. Per-voter weights are recomputed on demand.

`TotalWeight == 0` is exactly "this delegate has no payable voter set this era";
its pending pool is left intact and rolls into a later era. There is no separate
`HasWeightedEntries` flag any more, because with the entry list gone that flag
*is* this field being positive.

### 3.2 Why `candidate.Votes` is an acceptable denominator

`candidate.Votes` is the same number the removed entry list summed to.
`TestVoterWeightInvariant` (`action/protocol/staking/voter_weight_invariant_test.go`)
asserts `candidate.Votes == Σ_voters weight(cand, voter)` after every staking
handler. Freezing `Votes` is therefore freezing the sum of a list that is no
longer stored — read from the one place that still holds it at H.

"Acceptable" is not "provably equal at every historical height": see §8, and see
the clamp in §5.3 which is what bounds the consequences when it is not.

### 3.3 Copy-on-write, not a copy

The recompute in §4 has to read bucket state *as it was at H*, from a block
several heights later. `beginEraCOWWindow` (`action/protocol/staking/era_window.go`)
opens a window at the end of the boundary block; from then on, the first write
to a covered key copies the pre-write value aside under
`{EntryPrefix}||u64BE(H)||kind||addr`. Reads go through `eracow.Resolve`, which
returns the copy if there is one, the live value otherwise, and `ErrNotFrozen`
for a tombstone (the key did not exist at H).

Copy-on-write rather than a snapshot because the *typical* case is that almost
nothing moves during a drain window: eras are long, the drain occupies its first
few blocks, and the fraction of buckets mutated in that span is small. Cost is
proportional to churn, not to population.

Two things are frozen as plain scalars rather than copied on write, because a
scalar is strictly stronger than a copy that might be missed:

- native `totalBucketCount` — the next index `putBucket` will hand out. Indices
  are strictly monotonic (`delBucket` does not decrement), so a native bucket
  with index ≥ this number cannot have existed at H.
- each staking contract's `NumOfBuckets` — highest id seen, burnt included.

This is why `putBucket` has no copy-on-write hook while `updateBucket` and
`delBucket` do: a post-H bucket is rejected by the high-water mark, so there is
nothing to copy.

The window is sealed by the final chunk (`SealEraCOWWindow`), after which the
hooks are branch-only no-ops. Sealed-era copies are deleted a bounded number per
block by `CollectEraCOWGarbage`, called from `CreatePreStates`
(`_eraCOWGarbagePerBlock`) — deleting tens of thousands of copies in one block
would blow the same budget the drain is chunked to respect.

## 4. On-demand weight recompute

`staking.FrozenVoterWeight` and `staking.FrozenVoterCandidates`
(`action/protocol/staking/era_voter_scan.go`) answer, for a voter, "which
delegates did you have buckets with at H, and what was each bucket worth" —
reading buckets through the era window and evaluating
`CalculateVoteWeight(consts, bucket, isSelfStake)` at H.

The evaluation height is `work.FreezeHeight`, never the height of the block the
chunk runs in. A contract-staking bucket that is not timestamp-based has its
remaining duration measured in blocks; evaluating the same frozen bucket at
chunk 1 and chunk 5 of one settlement would otherwise produce two different
weights for the same era. Copy-on-write cannot paper over this, because the
drifting input is the evaluation height itself, not a stored value — which is
why H travels with each work item instead of being derived.

## 5. The drain

### 5.1 Voter-major, sharded by address byte

The drain walks the **voter key space**, not the delegate list. The space is the
union of `{_voterIndex}||addr` (native) and `{_lsdVoterIndex}||addr`
(contract-staking); both put a 20-byte address immediately after a 1-byte tag,
so the **first address byte partitions it into 256 contiguous shards**
(`staking.AddressShards`).

Voter-major matters for money, not just for iteration: a voter with buckets on
twenty delegates is paid **once**, after their per-delegate shares are summed —
one destination lookup and one balance write instead of twenty.

Resume is *shard plus last-address*, not shard alone. Shard population is
attacker-controllable — addresses are cheap and their first byte is grindable —
so shard-granular resume would leave per-block work unbounded. A chunk may stop
part-way through a shard and record `ResumeVoter`.

`FrozenShardVoters` merges **four** ranges per shard, not two: the two live
indexes plus the two copy-on-write entry ranges. A voter who withdraws their
last bucket during the drain window has their live index key deleted while the
copy still holds what they had at H; scanning only live keys would drop a voter
who is owed a share of an era they were part of. The reverse case — a voter who
acquires their first bucket after H — costs nothing to admit: the tombstone is
skipped, and the recompute would resolve them to zero anyway.

Every scan is bounded to the shard's key range. None may be unbounded: the state
layer materializes whatever range it is handed *before* any limit is applied, so
an unbounded scan is unbounded work inside one block no matter what the caller
intends to consume.

### 5.2 Cursor state: plan, progress, and a rotated start

Split across two records (`action/protocol/rewarding/epoch_drain_cursor.go`):

- **`epochDrainPlan`** — immutable for the settlement: `TargetEra`, epoch range,
  the settlement seed, and the per-delegate `epochDrainDelegateWork` list
  (`CandidateIdentifier`, `VoterAmountFrozen`, `RewardAddress`,
  `EpochCommission`, `TotalWeight`, `SnapshotHash`, `FreezeHeight`,
  `SelfStakeBucketIdx`). Split out so a continuation block does not re-version
  the whole delegate list in archive storage every time it checkpoints.
- **`epochDrainProgress`** — mutable: `StartShard`, `ShardsDone`, `ResumeVoter`,
  the per-delegate `Distributed` running totals, the skipped-delegate bitmap,
  `Completed` / `CompletedHeight`.

`StartShard` is `settlementStartShard(seed)` where the seed is derived from the
parent block hash and the domain string `"iip59.settlement-start.v1"` plus the
target era. The walk is a rotation of the 256 shards from that offset, so a
settlement that repeatedly runs long does not always serve the same corner of
the address space first. It is seeded from block data, not entropy: every node
computes the same rotation.

`FreezeHeight == 0` on a work item means "no frozen era" and makes the delegate
unpayable — that combination can only be a pre-activation artifact, and the
recompute refuses to run against it rather than defaulting to the current
height.

### 5.3 The share rule and the payout clamp

`computeVoterShares` (`action/protocol/rewarding/voter_allocation.go`) is the
single implementation, behind both the drain (which pays) and the read-only
`voterRewardStatus` query (which reports what is owed). For each delegate the
voter had a frozen bucket with:

```
share_i = floor(pool_i * weight_i / totalWeight_i)
share_i = min(share_i, pool_i - distributed_i)      // the clamp
```

then the shares are summed and paid as one transfer.

The clamp is not defensive boilerplate. `totalWeight` is a frozen `candidate.Votes`
and the per-voter weights are recomputed from buckets; if the two disagree in
the direction of the recompute being larger (§8), the unclamped sum of shares
can exceed the frozen pool, and the drain would pay out money the delegate's
pool does not contain. The clamp bounds Σ payouts per delegate by
`VoterAmountFrozen` exactly, and whatever is left over is swept on the orphan
path at completion rather than silently retained.

Note that `completeEpochDrain` folds each delegate's residual into
`Distributed`, so after completion `Distributed == VoterAmountFrozen` by
construction. Any assertion about how much was actually *paid* has to sample
while the cursor is still live — which is what `iip59DrainWatch` in
`e2etest/iip59_payout_test.go` exists for.

### 5.4 Routing: compound or credit

`payVoterCombined` (`action/protocol/rewarding/voter_reward.go`) decides once
per voter for the combined amount. With an eligible auto-deposit bucket the
whole sum is compounded via `AddDepositForCompound`; otherwise it is credited to
the voter's reward destination with `creditPrimaryAccount` — a **native account
balance**, not a rewarding `unclaimedBalance`, so there is no claim step.

Every fallback branch (no bridge, bridge error, unreadable or ineligible bucket,
self-stake role changed since the freeze) degrades to a direct credit. The share
is still owed; only its destination changed.

## 6. Read-state surface

Eth-ABI views dispatched by 4-byte selector in
`action/protocol/rewarding/ethabi/` (`iip59.go`, `base.go`):

| view | returns |
|---|---|
| `pendingBlockRewardPool(address)` | accrued voter pool for a delegate |
| `pendingBlockRewardPoolIndex()` | delegates with a non-zero pool |
| `eraDrainCursor()` | settlement plan + progress |
| `voterRewardDelegateSnapshot(address)` | the frozen per-delegate scalars of §3.1 |
| `voterRewardAddress(address)` | a delegate's configured reward address |
| `voterRewardDestination(address)` | a voter's configured destination |
| `voterRewardStatus(address)` | a voter's settlement status and amount |

**`voterRewardSnapshot` was renamed to `voterRewardDelegateSnapshot` and the old
selector is not registered.** The old name returned
`(…, address[] voters, uint256[] weights)` off the materialized entry list. That
list no longer exists and the tuple now ends in
`(uint64 freezeHeight, uint64 selfStakeBucketIdx)`. Keeping the name would keep
the 4-byte selector, and any caller that had not rebuilt would decode the new
tuple as the old one — silently, since ABI decoding of a same-arity prefix does
not fail. A hard selector change turns that into an `errInvalidCallSig`.
No deprecated alias is registered: IIP-59 is unactivated, so there is no
deployed caller to preserve compatibility with.
`TestIIP59RetiredVoterRewardSnapshotSelector` rebuilds the old ABI and asserts
the retired selector is rejected.

**`epochDrainCursor` was renamed to `eraDrainCursor` for the same reason**, and
the hazard there is sharper: the view takes no arguments, so its retired
selector is reachable from calldata that is nothing but the 4-byte id. The
candidate-major cursor exposed
`(delegateIndex, voterIndex, delegateStartIndex, voterStartIndices)`; the
voter-major one exposes `(startShard, shardsDone, resumeVoter)` instead.
Decoded against the old ABI a shard counter reads as a delegate index — a
small, plausible, wrong number, which is worse than a decode failure.
`TestIIP59RetiredEpochDrainCursorSelector` pins the removal.

Both renames stop at the eth ABI. The native `ReadState` method names
(`EpochDrainCursor`, `VoterRewardSnapshot`) are unchanged: an unrecognised
`ReadState` name errors out loudly instead of mis-decoding, so it carries none
of the silent-reinterpretation risk that motivated the selector change, and the
repo already pairs a renamed selector with an unchanged native method
(`candidateByAddressV4` → `CANDIDATE_BY_ADDRESS`).

The name is descriptive rather than `V2`. The `V`-suffix convention in
`staking/ethabi/{v2,v3,v4}` marks versions that **coexist** in the dispatch
table; a `V2` with no `V1` registered would suggest an older view is still
callable. `voterRewardDelegateSnapshot` also disambiguates the per-delegate view
from the per-voter `voterRewardStatus`.

The native ReadState method name (`"VoterRewardSnapshot"`) is unchanged — it is a
separate namespace from eth selectors, and nothing about it is ambiguous.

## 7. Failure handling and observability

A `VoterRewardChunk` that returns an error settles with a `Failure` receipt and
the block still commits. This is deliberate and is not up for revision: degrade
the item, never abort the block. The cursor is left exactly where it was, so the
next block retries the same chunk.

The cost of that choice is that a persistently failing chunk is **invisible from
chain data alone**: the next era boundary's `writeEpochDrainCursor` replaces both
plan and progress, so a chunk that keeps failing quietly discards an era of
voter payouts when the boundary arrives. It used to be logged at `Debug`.

`reportVoterRewardChunkFailure` (`epoch_drain_cursor.go`) now logs at **`Error`**
with the block height, target era, epoch range, `ShardsDone`, current shard,
`ResumeVoter` (one address, hex), delegate count, and the completed flag — a
fixed-size record, never a voter list. It is explicitly best-effort and
diagnostic-only: it runs before `Handle` reverts to its entry snapshot, so it
must not write state.

Two metrics, registered the same way as the rest of the package
(`iip59_metrics.go`):

- `iotex_rewarding_iip59_drain_chunk_failures_total` (counter)
- `iotex_rewarding_iip59_drain_stalled_shards_done` (gauge) — `ShardsDone` as of
  the most recent failure.

Read together: rising counter with a flat gauge is a stuck drain; rising counter
with a rising gauge is a drain making progress through intermittent failures.

Only the `action.VoterRewardChunk` case was changed. Error handling for the other
system actions in that switch is untouched.

## 8. The self-stake predicate divergence

Recorded here because it is the reason §5.3's clamp exists.

**Two predicates for "is this bucket the candidate's self-stake bucket" coexist.**

- Stateless: `bkt.Index == cand.SelfStakeBucketIdx` (with `ContractAddress == ""`).
  This is what `FrozenVoterWeight` uses, and what the frozen
  `SelfStakeBucketIdx` scalar serves.
- Refined: `isSelfStakeBucket(...)`, which additionally consults the endorsement
  record. This is what every `candidate.Votes` mutator uses, and what
  `isActiveCandidate` uses.

They can disagree only when an endorsement is in state `EndorseExpired` while
`SelfStakeBucketIdx` still names the bucket.

**That divergence cannot be created at any height IIP-59 runs at.**

- `EnforceLegacyEndorsement = !g.IsUpernavik(height)`
  (`action/protocol/context.go`). `LegacyStatus` is the only producer of
  `EndorseExpired` from a live record; the new-mode `Status` returns only
  `Endorsed` or `UnEndorsing`.
- Upernavik (31174201) < Xingu (41648761), and IIP-59 activates no earlier than
  the current fork frontier, so new-mode dispatch applies everywhere IIP-59 runs.
- The other source of `EndorseExpired` — a *missing* endorsement record — is not
  a second path. The sole `esm.Delete` site is always preceded by
  `SelfStakeBucketIdx` being cleared (via `clearCandidateSelfStake` or
  `csm.deactivate`), and the `requestDeactivation` branch returns early and never
  deletes.

**What remains reachable is inherited skew.** An endorsement that lapsed under
*legacy* rules in [Tsunami 29275561, Upernavik 31174201) can leave
`candidate.Votes` carrying a self-stake bonus the refined predicate no longer
agrees with. A later bucket mutation then does `sub(nonBonusOld)` /
`add(nonBonusNew)` against a `Votes` that still includes that bonus, so the
difference `bonusOld − nonBonusOld` is stranded in `Votes` permanently.

From then on the frozen `TotalWeight` is **not** the exact sum of what the
recompute produces for the same buckets: the denominator comes from the mutators'
predicate, the numerators from the stateless one. The sign of the gap depends on
how the post-lapse mutation moved the bucket's stake, and when it falls the wrong
way the unclamped shares sum to more than the frozen pool. That is the
over-payment condition, and §5.3's clamp is what bounds it; the residual is
swept on the orphan path.

Regression coverage already exists; do not duplicate it:

- `action/protocol/staking/selfstake_predicate_divergence_test.go`
  → `TestLapsedEndorsementDivergesSelfStakePredicates`
- `action/protocol/rewarding/voter_allocation_test.go`
  → `TestLapsedSelfStakeBonusCannotOverpayDelegatePool`

## 9. Genesis parameters

| field | meaning |
|---|---|
| `Rewarding.EpochsPerRewardEra` | era length in epochs (mainnet target 24) |
| `Rewarding.VoterBudgetPerBlock` | voters processed per continuation chunk; 0 means unbounded |
| `Rewarding.VoterWeightSeedBatchSize` | **deprecated and unused** — sized the withdrawn seeding flush; parsed so existing genesis files still load |

`EpochsPerRewardEra` must be a genesis constant and the boundary condition must
be `epochNum % EpochsPerRewardEra == 0`. Anything derived from wall clock or
node-local state is a consensus fault.

## 10. Verification

Correctness of the settlement is asserted end-to-end in `e2etest`:

- `iip59_payout_test.go` — per-voter payout equality against a model built from
  the fixture's stake parameters and `CalculateVoteWeight` alone (deliberately
  *not* `computeVoterShares` or `FrozenVoterWeight`, which would make the check
  circular); the per-delegate `Σ payouts ≤ VoterAmountFrozen` bound sampled
  while the cursor is live; `TestIIP59DrainResumeEquivalence` (identical payouts
  whether the era drains in one chunk or ten); and
  `TestIIP59DrainPaysTheFrozenEraNotTheLiveOne` (a bucket created after H earns
  nothing, a bucket deleted after H is still paid its frozen share — the
  copy-on-write layer's raison d'être).
- `iip59_stress_test.go`, `iip59_perf_test.go` — the same per-voter assertions on
  the chunked-drain stress and perf harnesses, plus the fund-conservation
  invariant at every block boundary.

Measurements: `docs/iip-59-perf-report.md`.
