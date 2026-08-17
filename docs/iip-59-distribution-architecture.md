# IIP-59 reward distribution architecture — as implemented

**Status:** describes the current implementation in PR
iotexproject/iotex-core#4953. IIP-59 is **not activated**: every mechanism
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
3. **Do not turn node-local faults into consensus results.** Failures derived
   entirely from committed state may settle as a failed system action. Storage,
   scan, and other infrastructure failures propagate and fail the block, because
   validators may not observe them uniformly; §7 describes the distinction.

## 2. Shape of the settlement

A **settlement** distributes one era's accrued voter pools. It has two phases
and they run in different blocks.

```
freeze block H                  boundary-epoch     continuation blocks C+1, C+2, ...
(mid-epoch, see below)          last block C
──────────────────────────────  ─────────────────  ─────────────────────────────────
PutPollResult                   GrantEpochReward        continuation blocks
  └── FreezeCandidateRewardSnapshots  ├── split epoch reward  └── GrantVoterRewardChunk
        ├── freeze delegate inputs     ├── credit voter pools        ├── scan voter addresses
        └── BeginEraCOWWindow(H)       └── initialize distribution   ├── recompute weights at H
                                           (plan + progress)         ├── pay each voter once
                                                                     └── checkpoint progress
                                                               final chunk:
                                                                 ├── SealEraCOWWindow
                                                                 └── mark distribution completed
```

The freeze and initialization are fixed-size per delegate. The voter scan is
proportional to the voter population, and it is the part that is chunked.

### 2.1 H is not the era boundary block

The freeze rides on `PutPollResult`, and that action is not created at an era
boundary — it is created around the **midpoint of the preceding epoch**
(`createPostSystemActions` in `action/protocol/poll/util.go` returns nil until
`blockHeight >= epochHeight + epochLen/2`). The action carries `nextEpochHeight`,
so `setCandidates` derives the epoch number of the *target* epoch and gates the
freeze on that epoch being an era boundary. The gate is on the right epoch; the
execution is a half-epoch early.

`GrantEpochReward`, which initializes distribution, sits at the other end: it asserts it
is running on the **last block of the boundary epoch** (`assertLastBlockInEpoch`).

So for an era boundary epoch E:

| event | height |
|---|---|
| `FreezeCandidateRewardSnapshots`, `BeginEraCOWWindow(H)` | ≈ midpoint of epoch E−1 |
| `GrantEpochReward`, `initializeVoterRewardDistribution` | last block of epoch E |
| first continuation chunk | first block of epoch E+1 |

On mainnet an epoch is `NumDelegates × WakeNumSubEpochs` = 24 × 60 = 1,440
blocks, so H precedes distribution by roughly **1.5 epochs ≈ 2,160 blocks ≈ 90
minutes** at a 2.5s block interval. In small test fixtures the same 1.5-epoch
gap is only a handful of blocks, which is why it is easy to miss locally: a
4-delegate 1s-interval nightly observed `freeze_height=117` for boundary 128 and
`freeze_height=181` for boundary 192 — 11 blocks, the same ratio.

This is a deliberate accepted position, not an oversight, but it has a real
consequence worth stating plainly: **stake activity in the last half of epoch
E−1 and all of epoch E does not affect the weights that settle that era.**
`TotalWeight` and every bucket high-water mark are as of H, and the COW window
is open for that whole span, so buckets mutated in it pay the copy cost.

Nothing here is a divergence risk: H travels with the snapshot as `FreezeHeight`
and every recompute evaluates at it, so all nodes agree on the same numbers. The
mismatch is between the code and the phrase "era boundary freeze", not between
nodes. Wherever this document says "the boundary block H", read "the freeze
block H, which is the `PutPollResult` block whose target epoch opens the era".

Moving the freeze onto the boundary block itself would require a separate
boundary-block hook and a re-measurement of the COW window's open/close timing;
it was considered and deferred.

## 3. The freeze

### 3.1 What is frozen, and as what

`staking.FreezeCandidateRewardSnapshots` writes one `CandidateRewardSnapshot` per **opted-in
candidate** at the freeze block H (`action/protocol/staking/poll_snapshot.go`)
— which is the `PutPollResult` block a half-epoch ahead of the era boundary
epoch, not the boundary block itself; see §2.1.

The frozen set is enumerated from the candidate center and filtered by the
opt-in bit alone. It is deliberately *not* the poll list that rides the same
block: the poll list is filtered by `isActiveCandidate` and by a vote-score
threshold, and it is frozen once per era while the paid set is recomputed
every epoch inside the era, so the two drift. A candidate that has not opted in
at H gets no record. Because opt-in is a one-way transition, snapshot presence
fully captures the era's routing decision: an opt-in submitted after H takes
effect at the next freeze, while absence keeps the current era on the legacy
route.

The snapshot is **scalars only**:

| field | why it is frozen |
|---|---|
| `BlockCommissionBasisPoints`, `EpochCommissionBasisPoints`, `CommissionConfigured` | the DelegateProfile contract can change between H and the last chunk |
| `TotalWeight` | the denominator of every share; it is `candidate.Votes` at H, and `Votes` keeps moving during the drain window |
| `FreezeHeight` (H) | the height every weight recompute is evaluated at |
| `SelfStakeBucketIdx` | the only candidate field the weight recompute reads |

There is **no materialized per-voter entry list**. An earlier design froze
`(voter, weight)` pairs per delegate; at mainnet scale that is ~27k entries of
state churn per era, plus a persistent `VoterWeightView` and activation-time
seeding to produce the pairs. All three were removed before activation.
Per-voter weights are recomputed on demand.

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
several heights later. `BeginEraCOWWindow` (`action/protocol/staking/era_cow_window.go`)
opens a window at the end of the freeze block H (§2.1); from then on, the first write
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
- each staking contract's exclusive bucket-index upper bound, derived as
  `NumOfBuckets + 1` from the highest id seen, burnt included.

This is why `putBucket` has no copy-on-write hook while `updateBucket` and
`delBucket` do: a post-H bucket is rejected by the high-water mark, so there is
nothing to copy.

The window is sealed by the final chunk (`SealEraCOWWindow`), after which the
hooks are branch-only no-ops. Stale copies are found directly by their ordered
`EntryPrefix || freezeHeight` key range and deleted a bounded number per block
by `CollectEraCOWGarbage`, called from `CreatePreStates`
(`_eraCOWGarbagePerBlock`) — deleting tens of thousands of copies in one block
would blow the same budget the drain is chunked to respect.

## 4. On-demand weight recompute

`staking.FrozenCandidatesForVoter` and `staking.FrozenVoterWeight`
(`action/protocol/staking/frozen_voter_scan.go` and
`frozen_voter_weight.go`) answer, for a voter, "which delegates did you have
buckets with at H, and what was each bucket worth" — reading buckets through
the era window and evaluating `CalculateVoteWeight(consts, bucket,
isSelfStake)` at H.

The evaluation height is the plan's `FreezeHeight`, never the height of the
block the chunk runs in. A contract-staking bucket that is not timestamp-based has its
remaining duration measured in blocks; evaluating the same frozen bucket at
chunk 1 and chunk 5 of one settlement would otherwise produce two different
weights for the same era. Copy-on-write cannot paper over this, because the
drifting input is the evaluation height itself, not a stored value — which is
why H is persisted once in the immutable distribution plan instead of being
derived from a continuation block.

## 5. The drain

### 5.1 Voter-major circular address scan

The drain walks the **voter key space**, not the delegate list. The space is the
union of `{_voterIndex}||addr` (native) and
`{contractstaking.LSDVoterIndexPrefix}||addr` (contract staking)
(contract-staking); both put a 20-byte address immediately after a 1-byte tag.
The settlement seed maps directly to a 20-byte `StartVoter`. The walk scans
`[StartVoter, max]`, wraps once, then scans `[min, StartVoter)`.

Voter-major matters for money, not just for iteration: a voter with buckets on
twenty delegates is paid **once**, after their per-delegate shares are summed —
one destination lookup and one balance write instead of twenty.

`ResumeVoter` is an exclusive lower bound in the current range, so a chunk can
stop at any address without tying progress to a coarse partition. `ScanPhase`
records whether the tail, wrapped head, or the whole circular walk is complete.

`ScanFrozenVoters` merges **four** ordered streams for the current bounded
range: the two live indexes plus the two copy-on-write index streams. A voter
who withdraws their last bucket during the drain window has their live index
key deleted while the copy still holds what they had at H; scanning only live
keys would drop a voter who is owed a share of the frozen era. A voter first
added after H is suppressed by the COW tombstone for that index family.

Two independent limits keep block work bounded. `VoterBudgetPerBlock` limits
the voters paid, while the scan key budget limits index keys consumed by the
four-way merge, including duplicates and tombstones that produce no voter.
When a source reaches its key limit, staking resumes only through the minimum
address covered by every source, so storage iteration order cannot skip a
voter.

### 5.2 Distribution state: immutable plan and mutable progress

Split across two records (`action/protocol/rewarding/voter_reward_distribution.go`):

- **`voterRewardDistributionPlan`**, stored under
  `state.VoterRewardDistributionPlanKey` — immutable `TargetEra`,
  `FreezeHeight`, `SettlementSeed`, and the per-delegate
  `DelegateAllocations`. Each allocation contains only
  `CandidateIdentifier`, `VoterAmountFrozen`, `TotalWeight`, and
  `SelfStakeBucketIdx`. Keeping this record immutable avoids re-versioning the
  full delegate list in archive storage on every continuation block.
- **`voterRewardDistributionProgress`**, stored under
  `state.VoterRewardDistributionProgressKey` — mutable `ScanPhase`,
  `ResumeVoter`, the positionally aligned `DistributedByDelegate` totals, and
  `CompletedHeight`. Completion is derived from `ScanPhase == voterScanDone`.

The seed is a domain-separated hash (`"iip59.settlement-start.v2"`) of the
parent block hash and target era. `settlementStartVoter` uses its first 20 bytes,
so successive settlements rotate the start of the global address walk without
introducing non-consensus entropy.

`FreezeHeight == 0` on the plan means there is no valid frozen era. The
recompute refuses to substitute the current height.

### 5.3 The share rule and the payout clamp

`computeVoterShares` (`action/protocol/rewarding/voter_allocation.go`) is the
single implementation used by the drain. For each delegate the voter had a
frozen bucket with:

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
`VoterAmountFrozen` exactly. Integer-division dust or an under-payment caused by
a denominator mismatch remains in the delegate's pending pool and is eligible
for a later era; completion does not sweep or relabel it as distributed.
`DistributedByDelegate` therefore continues to mean the amount actually paid
from this frozen allocation.

### 5.4 Routing: compound or credit

`payVoterCombined` (`action/protocol/rewarding/voter_reward.go`) decides once
per voter for the combined amount. With an eligible auto-deposit bucket the
whole sum is compounded via `AddDepositForCompound`; otherwise it is credited to
the voter's reward destination with `creditPrimaryAccount` — a **native account
balance**, not a rewarding `unclaimedBalance`, so there is no claim step.

Every fallback branch (no bridge, bridge error, unreadable or ineligible bucket,
self-stake role changed since the freeze) degrades to a direct credit. The share
is still owed; only its destination changed.

Each chunk also emits one `DelegateVoterRewardsDistributed` receipt event per
delegate that contributed a positive share in that chunk. The event remains
delegate-scoped for off-chain accounting, but its `voters`, `recipients`,
`amounts`, `compoundBucketIds`, and `compounded` arrays contain the rows produced
by the voter-major scan. `distributedlog.ABI`, `Topic0`, and `Unpack` expose the
canonical ABI and decoder; `compounded[i]`, not bucket ID zero, distinguishes a
compound payout because native bucket index 0 is valid.

## 6. Read-state surface

Eth-ABI views dispatched by 4-byte selector in
`action/protocol/rewarding/ethabi/` (`iip59.go`, `base.go`):

| view | returns |
|---|---|
| `pendingVoterReward(address)` | accrued voter pool for a delegate |
| `pendingVoterRewardDelegates()` | delegates with a non-zero pool |
| `voterRewardDistribution()` | distribution plan + progress |
| `delegateRewardSnapshot(address)` | the frozen per-delegate scalars of §3.1; errors with `ErrStateNotExist` for a delegate that was not opted in at H |
| `delegatePayoutAddress(address)` | a delegate's effective payout address |
| `voterRewardDestination(address)` | a voter's configured destination |

**`voterRewardSnapshot` was renamed to `delegateRewardSnapshot` and the old
selector is not registered.** The old name returned
`(…, address[] voters, uint256[] weights)` off the materialized entry list. That
list no longer exists and the tuple now ends in
`(uint64 freezeHeight, uint64 selfStakeBucketIdx)`. Keeping the name would keep
the 4-byte selector, and any caller that had not rebuilt would decode the new
tuple as the old one — silently, since ABI decoding of a same-arity prefix does
not fail. A hard selector change turns that into an `errInvalidCallSig`.
No deprecated alias is registered: IIP-59 is unactivated, so there is no
deployed caller to preserve compatibility with.

The distribution view is named `voterRewardDistribution`, matching the
business operation represented by `VoterRewardDistributionState`. It exposes
the immutable delegate allocation plan and the mutable voter-scan progress
without leaking storage-oriented names such as “cursor” or “drain”. Its scan
fields are `startVoter`, `scanPhase`, and `resumeVoter`; its delegate arrays are
`delegateIds`, `voterAmounts`, `distributedAmounts`, `totalWeights`, and
`selfStakeBucketIdxs`.

The native `ReadState` methods use the corresponding business names:
`PendingVoterReward`, `PendingVoterRewardDelegates`,
`VoterRewardDistribution`, `DelegateRewardSnapshot`,
`DelegatePayoutAddress`, and `VoterRewardDestination`.
There are no legacy `VoterRewardDistributionState` or `VoterRewardSnapshot`
method aliases.

## 7. Failure handling and observability

A `VoterRewardChunk` error is settled with a `Failure` receipt only when it is
explicitly marked as a consensus-determined `voterChunkSettleableError`, such as
a missing distribution or a COW window whose freeze height no longer matches.
The distribution state remains at its pre-action position, so the next block
retries the same chunk. Unmarked scan, read, write, and routing errors propagate
and fail block processing rather than allowing different validators to settle
different results from node-local failures.

`reportVoterRewardChunkFailure` (`voter_reward_distribution.go`) logs every
chunk error at **`Error`** with the block height and, when readable, target era,
freeze height, `ScanPhase`, `ResumeVoter` (one address, hex), delegate count,
and completion flag — a fixed-size record, never a voter list. It is explicitly
best-effort and diagnostic-only: it runs before `Handle` reverts to its entry
snapshot, so it must not write state.

Receipt logs provide consensus-visible progress. Every chunk prepends a
`CURSOR_PROGRESS` `RewardLog` containing the pre-chunk target era, scan phase,
resume voter, and number of address ranges remaining. If a distribution is
still incomplete at the next era boundary, `EPOCH_DRAIN_OVERRUN` records the
stale era, delegates with a remaining pool, and total live residue; the pool
balances themselves roll into the next distribution.

Two metrics, registered the same way as the rest of the package
(`iip59_metrics.go`):

- `iotex_rewarding_iip59_drain_chunk_failures_total` (counter)
- `iotex_rewarding_iip59_drain_stalled_scan_phase` (gauge) — the most recent
  readable failed distribution's `ScanPhase` (`tail`, `head`, or `done`).

The error log's `ResumeVoter` identifies progress within a phase; the gauge is
intentionally left unchanged when distribution state itself cannot be read.

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
over-payment condition, and §5.3's clamp is what bounds it; any residual stays
in the pending pool for a later era.

Regression coverage already exists; do not duplicate it:

- `action/protocol/staking/selfstake_predicate_divergence_test.go`
  → `TestLapsedEndorsementDivergesSelfStakePredicates`
- `action/protocol/rewarding/voter_allocation_test.go`
  → `TestLapsedSelfStakeBonusCannotOverpayDelegatePool`

## 9. Genesis parameters

| field | meaning |
|---|---|
| `Rewarding.EpochsPerRewardEra` | era length in epochs (mainnet target 24) |
| `Rewarding.VoterBudgetPerBlock` | voters processed per continuation chunk; values from 1 to 2000 lower the cap, while 0 or values above 2000 use the consensus maximum of 2000 |

`EpochsPerRewardEra` must be a genesis constant and the boundary condition must
be `epochNum % EpochsPerRewardEra == 0`. Anything derived from wall clock or
node-local state is a consensus fault.

## 10. Verification

Correctness of the settlement is asserted end-to-end in `e2etest`:

- `iip59_payout_test.go` — per-voter payout equality against a model built from
  the fixture's stake parameters and `CalculateVoteWeight` alone (deliberately
  *not* `computeVoterShares` or `FrozenVoterWeight`, which would make the check
  circular); the per-delegate `Σ payouts ≤ VoterAmountFrozen` bound;
  `TestIIP59DrainResumeEquivalence` (identical payouts
  whether the era drains in one chunk or ten); and
  `TestIIP59DrainPaysTheFrozenEraNotTheLiveOne` (a bucket created after H earns
  nothing, a bucket deleted after H is still paid its frozen share — the
  copy-on-write layer's raison d'être).
- `voter_scan_budget_test.go` and `era_cow_window_test.go` — the 2000-voter
  block cap, independent key-scan bound, circular range/resume semantics,
  four-stream deduplication, COW tombstones, and post-freeze voter exclusion.
- `iip59_stress_test.go`, `iip59_perf_test.go` — the same per-voter assertions on
  the chunked-drain stress and perf harnesses, plus the fund-conservation
  invariant at every block boundary.

Measurements: `docs/iip-59-perf-report.md`.
