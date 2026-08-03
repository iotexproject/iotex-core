# IIP-59 reward distribution architecture — v2 (era-based)

**Status:** draft.
**Supersedes:** the distribution mechanism in PRs 3′/4′/mitigation-3.
**Requires:** IIP-59 spec amendment (§3.5, §3.7, §4).

## 1. Executive summary

The current on-chain reward distribution (as landed in PR 3′/4′ and extended
by PR #4936 for chunking) tightly couples three separate concerns:

1. Accruing block/epoch rewards to a per-delegate voter pool
2. Splitting each delegate's voter pool across their voters
3. Compounding voters' credited balance into their staking bucket

All three run inside a single system action at the epoch-last block, which
concentrates ~27,020 voter operations into one block and exceeds the 2.5s
mainnet block budget by ~40% (in-mem) to ~140% (trie-backed).

This document proposes decoupling the three concerns and moving voter-facing
distribution from **per-epoch** to **per-day** (24 epochs). The voter weight
snapshot used for reward splitting is also lengthened to a daily cadence.
The result:

- **~24× reduction** in per-day compute for the distribution path
- **Predictable, small single-block cost** (~200-600ms trie-backed peak)
- **No voter-list freeze** — the daily snapshot is naturally stable through
  the distribution window
- **Log semantic freed** from the "one log per delegate per epoch" constraint

The architecture mirrors the era-based reward model used by Polkadot.

## 2. Background

### 2.1 Current on-chain data flow

```
per-block:
    GrantBlockReward
      ├── delegate.reward_address += block_reward × (1 - voter_take)
      └── delegate.voter_pool     += block_reward × voter_take

per-epoch (last block):
    GrantEpochReward
      ├── slashUqd(...)
      ├── splitEpochReward → per-delegate epoch_amount
      └── FOR EACH delegate:
            distributeCombinedReward(delegate.voter_pool + epoch_amount)
              └── FOR EACH voter of delegate:
                    IF opt-in compound AND bucket valid:
                        AddDepositForCompound(bucket, alloc)
                    ELSE:
                        voter.unclaimedBalance += alloc
                    emit DelegateDistributed batched log
```

### 2.2 Measured cost (mainnet-scale, mitigation 2 already applied)

- 27,020 distinct bucket owners (per gRPC enumeration, 2026-07)
- Per-voter processing: ~2.5μs read + ~40μs compound-or-credit ≈ 42.5μs in-mem
- Trie-backed: ~130μs per voter
- Single-block drain: **~1.15s in-mem / ~3.4s trie** — exceeds the 2.5s
  Dardanelles block budget

### 2.3 PR #4936 mitigation and why it is insufficient

PR #4936 (mitigation 3) chunks the drain at delegate boundaries with a
persisted cursor. This spreads the total work across N blocks but:

- Introduces a delegate-atomicity constraint that keeps `DelegateDistributed`
  as one log per delegate; head-heavy delegates (~5-10k voters each) still
  produce **~640ms-1.3s single-block trie work**
- Requires either a "voter freeze" (~1.6MB per epoch of state churn) or a
  hidden `chunkSize ≥ max_delegates` constraint to avoid poll-snapshot drift
- Does not reduce total per-day compute — only smears it

The architecture below removes the concentration entirely rather than
chunking around it.

## 3. Proposed architecture

### 3.1 Three-phase decomposition

```
Phase 1  every block             │ Phase 2   day boundary                │ Phase 3  any time
────────────────────────────────┼──────────────────────────────────────┼───────────────────
GrantBlockReward,               │ For each delegate d with              │ For any voter v
GrantEpochReward:                │ voter_pool > 0:                       │ with unclaimedBalance
  delegate.reward_address += ..  │   For each voter v in d.voters:       │ > 0 AND compound
  delegate.voter_pool     += ..  │     v.unclaimedBalance += share       │ opt-in AND bucket valid:
                                 │   delegate.voter_pool = 0             │   AddDepositForCompound(...)
                                 │                                       │   v.unclaimedBalance -= amount
```

### 3.2 Phase decoupling: what each phase depends on

| Phase | Reads | Writes | Time-sensitive? |
|---|---|---|---|
| 1 | delegate committee (from poll), current `voter_take` rate | `delegate.reward_address`, `delegate.voter_pool` | No |
| 2 | `delegate.voter_pool` (accumulated), daily VoterWeightSnapshot | `voter.unclaimedBalance`, `delegate.voter_pool = 0` | **Yes** — must consume before next snapshot |
| 3 | `voter.unclaimedBalance`, opt-in flag, bucket state | `voter.unclaimedBalance`, bucket state via `AddDepositForCompound` | No — deferrable arbitrarily |

The critical observation is that **Phase 3's data has no expiration**:
`unclaimedBalance` is monotonic between Phase 2 credits and either Phase 3
compound or user `Claim`. Phase 3 can run at any cadence — including "never"
— without breaking correctness.

### 3.3 Distribution cadence

Change from per-epoch to per-day (24 epochs):

- Phase 1 continues every block (no change)
- Phase 2 fires at day boundaries (every 24th epoch end)
- Phase 3 runs as a background system action (or per §3.6 alternatives)

Number of iotex mainnet epochs per day: **24** (each epoch ≈ 1 hour at 2.5s
block interval, 1440 blocks/epoch). The value SHOULD be a genesis parameter
`EpochsPerRewardEra` (default 24) to allow adjustment.

### 3.4 Voter weight snapshot cadence

Currently `PutPollResult` refreshes both the delegate committee and the
voter weight view (`SnapshotForEpochReward`) at every epoch boundary. This
document proposes decoupling them:

- **Delegate committee selection**: unchanged, per-epoch
- **VoterWeightSnapshot**: refreshed only at day boundaries (every 24 epochs)

This means voters' proportional share of the day's rewards is computed
against a single stable snapshot for the whole day. There is no
"which epoch's snapshot governs which reward" ambiguity, and no need to
retain multiple snapshot versions.

Voter weight changes (stake, unstake, restake, endorsement events) still
mutate the live `VoterWeightView` every block for accurate query — only the
**snapshot** used for reward math freezes for 24 epochs.

Trade-off: a voter who stakes mid-day earns rewards from that day only if
they held stake at the previous day boundary. This is materially the same
as Polkadot's per-era model and is acceptable given daily granularity.

### 3.5 State model

**Added:**

- `voter_reward_era_cursor` (rewarding namespace, singleton) — Phase 2
  progress cursor:
  ```proto
  message VoterRewardEraCursor {
    uint64 era_start_epoch = 1;
    uint32 delegate_idx    = 2;
    uint32 voter_offset    = 3;
  }
  ```
  Size: ~20-30 bytes. Written at day boundary, updated per Phase 2 chunk,
  deleted at Phase 2 completion.

- `voter_reward_era_sentinel` (rewarding namespace, per-era) — replay guard:
  key `"vre" || era_start_epoch`, empty payload. Written at Phase 2
  completion, permanent.

- `compound_pending_index` (rewarding namespace) — Phase 3 sweep cursor
  (see §3.6). Optional depending on Phase 3 strategy.

**Modified:**

- `SnapshotForEpochReward` write path: conditional on
  `epochNum % EpochsPerRewardEra == 0`. Non-era-boundary epochs skip the
  snapshot write.

**Kept:**

- `PendingBlockRewardPool` per delegate (from PR 4′) — accumulator target
  for Phase 1

**Removed (from PR #4936 disposition):**

- `EpochDrainCursor`, `EpochDrainDelegateWork`,
  `EpochDrainFoundationBonusWork`, `EpochDrainOrphanWork` protos and
  associated code — no longer needed under era-based distribution

### 3.6 Phase 3 execution strategy

Four viable options, in order of increasing simplicity:

**A. Continuous background system action**
- `CreatePostSystemActions` emits `CompoundBatch` on every block
- Handler consumes up to `CompoundBatchSize` opt-in voters with
  `unclaimedBalance > 0` per invocation
- State: `compound_pending_index` (sequential scan pointer) or
  `compound_pending_set` (secondary index maintained by Phase 2)
- Peak block cost: `CompoundBatchSize × 130μs trie` (e.g., 500 voters →
  ~65ms trie)

**B. On-Claim lazy compound**
- No system action at all
- User `Claim` handler detects opt-in and routes to `AddDepositForCompound`
- Trade-off: violates "automatic compound" UX (user must send tx to trigger)

**C. External-triggered `CompoundBatch` action**
- Any account can send `CompoundBatch(voters[])` action
- Handler validates each voter's opt-in + balance and does the work
- Off-chain automation (hermes-patch, bots) picks up
- On-chain code minimal; execution guaranteed by economic incentive
  (delegate operator maintains their voters' auto-deposit UX)

**D. Hybrid**
- Continuous background scan (A) at low `CompoundBatchSize`
- Plus (B) as a safety net for voters whose scan hasn't reached them yet

**Recommendation:** ship with **D** for full parity with the current
pre-PR-#4936 experience. If off-chain infra is unreliable or scan cost
proves lower than expected, degrade to **A** only.

### 3.7 Log semantics

Per-epoch `DelegateDistributed{epoch, delegate, voters[], amounts[]}` is
replaced:

- **Phase 2**: emit `EraVoterCredited{era_start_epoch, delegate, voters[], amounts[], chunk_seq, is_final}`
  per delegate per chunk. Off-chain aggregators sum by
  `(era_start_epoch, delegate)`.
- **Phase 3**: no new rewarding-side log needed — `AddDepositForCompound`
  already emits its `AddDeposit` event on the staking side.

The IIP-59 §3.7 "one log per delegate per epoch" constraint is dropped in
favor of "logs are aggregable by `(era, delegate)`".

## 4. Data flow diagram (era boundary)

```
epoch:  N-2         N-1         N       N+1  ...  N+23    N+24
        ─┬──────────┬───────────┬─────  ...  ────┬───────┬──
         │          │           │                │       │
         │ Phase 1  │ Phase 1   │ Phase 1        │       │ Phase 1
         │ (block   │  ...      │                │       │
         │  reward  │           │                │       │
         │  credits │           │                │       │
         │  to      │           │                │       │
         │  voter_  │           │                │       │
         │  pool    │           │                │       │
         │  each    │           │                │       │
         │  block)  │           │                │       │
                                                 │       │
                              era N boundary:    │       │
                              ┌──────────────────┘       │
                              │  Phase 2 chunks:         │
                              │  drain voter_pool → ...  │
                              │  27,020 credits over     │
                              │  ~6 blocks               │
                              │                          │
                              │  VoterWeightSnapshot     │
                              │  refreshed here          │
                                                         │
                                                    era N+1 boundary
                                                    (same cycle repeats)

Phase 3 runs continuously in the background across all epochs, orthogonal
to era boundaries.
```

## 5. Cost analysis

### 5.1 Per-day compute

| Phase | Frequency | Per-voter | Total voters | Aggregate/day (trie) |
|---|---|---|---|---|
| 1 (accrual) | per block | ms-level total | n/a | ~ms/block × 34,560 blocks ≈ negligible |
| 2 (credit) | 1× per day | ~40μs | 27,020 | **~1.08s per day** |
| 3 (compound, 60% opt-in) | continuous | ~130μs | ~16,200 | **~2.1s per day**, spread across ~34,560 blocks → **~60μs/block** |

Compare to current per-epoch model: **24 × (Phase 2 + Phase 3) ≈ 76s trie
work per day**. **~24× reduction**.

### 5.2 Single-block peak cost (Phase 2)

With `VoterBudgetPerBlock = 5000`:

- Blocks to complete Phase 2: `⌈27,020 / 5000⌉ = 6 blocks`
- Peak block cost: `5000 × 130μs ≈ 650ms trie`
- Fits within 2.5s block budget with 74% headroom

With `VoterBudgetPerBlock = 2000`:

- Blocks to complete Phase 2: `⌈27,020 / 2000⌉ = 14 blocks`
- Peak block cost: `2000 × 130μs ≈ 260ms trie` (10% of budget)

### 5.3 State footprint

| Item | Size | Lifecycle |
|---|---|---|
| `voter_reward_era_cursor` | ~30B | Era boundary write → 6-14 chunk rewrites → deleted at Phase 2 completion |
| `voter_reward_era_sentinel` | 1B key, empty value | Written at Phase 2 completion, permanent |
| `compound_pending_set` (if used, option A/D) | ~20B × up-to-27,020 = ~540KB peak | Maintained by Phase 2 add / Phase 3 delete |
| Modified `SnapshotForEpochReward` cadence | unchanged size, 24× less write frequency | Trie state churn reduces ~24× |

**No voter-list freeze required.** The daily snapshot is a live-view
mechanism that the Phase 2 chunks read from directly. Phase 2's window
(blocks 1 to ~14 after era boundary) is bounded by the next era boundary,
so snapshot stability is guaranteed.

## 6. Compatibility & migration

### 6.1 Fork gate

Reuse the existing `ToBeEnabledBlockHeight` gate (already scoped to IIP-59
in feature context as `!NoVoterRewardDistribution`). Behavior before the
gate: legacy hermes-patch-based off-chain distribution. Behavior after:
this document's era-based on-chain distribution.

### 6.2 First era after fork

The first era begins at `firstEra = ceil(forkEpoch / EpochsPerRewardEra) * EpochsPerRewardEra`.
Phase 1 accrues from `forkEpoch` onward; Phase 2 first runs at `firstEra`.

Rewards earned between `forkEpoch` and `firstEra` are distributed at
`firstEra` using the snapshot taken at `firstEra`.

### 6.3 PR #4936 disposition

**Recommendation: close without merge.** PR #4936's cursor infrastructure
was designed for the mid-epoch drain use case which era-based distribution
eliminates. The branch is preserved so §8.4's salvage list can extract
reusable fragments into PR A / C / D.

### 6.4 IIP-59 spec amendment

Sections needing amendment (versus the current draft in `iotexproject/iips`
PR #73):

- **§3.5 Distribution cadence:** change "per epoch" to "per reward era
  (24 epochs)"
- **§3.6 Voter weight snapshot:** change "at each `PutPollResult`" to
  "at each era boundary `PutPollResult`"
- **§3.7 `DelegateDistributed` log:** rename to `EraVoterCredited` and
  drop the per-delegate-per-epoch atomicity constraint
- **§4 Migration:** document the first-era boundary and hermes-patch
  wind-down

### 6.5 Off-chain migration

- **Hermes-patch service (#44):** must stop distributing to opted-in
  delegates before the fork height. The service already filters by
  `Candidate.VoterRewardOnchainOptIn` (per PR 1′). Post-fork, the service
  should retain a compatibility mode for un-opted delegates and shut down
  after the last delegate opts in.
- **Verifier tool (#45):** update aggregation key from
  `(epoch, delegate)` to `(era_start_epoch, delegate)`.
- **Explorer / dashboard:** adapt to era-based reward events, expose
  "next reward at" as `next_era_boundary`.

## 7. Trade-offs & risks

### 7.1 Voter UX

- Reward accrual visibility drops from hourly to daily
- No material APY impact (daily vs hourly compound at 5% APY differs by
  <0.001%)
- Voter can still see accrued-but-not-distributed rewards via
  `pending_block_reward_pool[delegate]` if we expose a read API

### 7.2 Voter fairness across day boundaries

- Voter staking mid-day: earns from that day only if held stake at previous
  era boundary — otherwise waits for next era boundary
- Voter unstaking mid-day: still earns full share for the day (was in
  snapshot)
- Delegate switching mid-day: earns from *destination* delegate for the
  full day, forgoes *source* delegate rewards
- Precedent: identical semantics to Polkadot per-era rewards, considered
  acceptable in that ecosystem

### 7.3 Phase 3 backlog

If Phase 3 is stopped (bug, upgrade, deliberate pause), unclaimedBalance
accumulates but stays claimable. No correctness impact. Backlog processing
resumes on Phase 3 restart. Documenting an upper bound on backlog and
resume speed is required for operations.

### 7.4 Era boundary block load

The era boundary block (`epoch % 24 == 0`) still does:
- Committee selection (`PutPollResult`, unchanged)
- Voter weight snapshot refresh (was per-epoch, now per-day — same cost,
  lower frequency)
- Phase 2 first chunk

Peak load: PutPollResult + snapshot (~50-100ms) + 5000 voter credits
(~650ms) ≈ ~750ms trie. Still 30% of block budget.

### 7.5 Consensus divergence risk in `PutPollResult` change

Changing `SnapshotForEpochReward` cadence is a subtle protocol change. All
validators must agree on the boundary condition. Recommended:

- Boundary condition MUST be `epochNum % EpochsPerRewardEra == 0`, not
  wall-clock or block-count based
- `EpochsPerRewardEra` MUST be a genesis-time constant, not runtime-mutable
- Coverage test: run poll suite with `EpochsPerRewardEra = 3` (short era)
  and verify snapshot is written only at eras {3, 6, 9, ...}

## 8. Implementation plan

### 8.1 Disposition of already-landed / open IIP-59 PRs

The v2 amendment preserves every mechanism piece of v1 — only the firing
cadence and the batched-log format change. Concrete disposition:

**Preserved as-is (no follow-up PR required):**

| Existing PR | What it delivered | Why v2 keeps it |
|---|---|---|
| PR 1' | `VoterRewardOnchainOptIn` field + `SetVoterRewardOptIn` action | Opt-in gate; §3.7 unchanged |
| PR 2 redo | `VoterWeightView` + 9 handler hooks + view lifecycle | Live view still needed every block for query; only the snapshot-*write*-trigger changes (PR B) |
| PR 4.5 | `DelegateProfile` bridge (`SnapshotCommissionRates`) | Rate source; §3.5 unchanged |
| PR 4.6 | `AutoDeposit` `compoundOrCredit` bridge | Routing preconditions unchanged; §3.6. Call site moves from Phase 2 to Phase 3 (PR D) |
| PR 4a–4c | `PendingBlockRewardPool` proto + credit / drain / orphan | Phase 1 accumulator; §3.1 unchanged. Drain call site moves from epoch-close to era-close (PR C) |
| PR #4928 (mitigation 2) | `SlotBucketReader` direct-slot AutoDeposit reader | Phase 3 still walks the same per-voter path; the 26× per-call speedup carries over verbatim |

**Modified in-place by PRs below** (the code stays, its call site or
trigger condition shifts):

| Existing PR | What changes | New PR |
|---|---|---|
| PR 2' | `SnapshotForEpochReward` writer — add `IsEraBoundary` gate at the write site | PR B |
| PR 3' | `distributeVoterReward` — refactor from one-shot to cursor-driven chunked; move compound call out to Phase 3 | PR C |
| PR 4.7 | `DelegateDistributed` batched log — rename + add `chunk_seq` / `is_final` | PR C |
| PR 4' | Block-reward pool drain — moves from epoch-close to era-close | PR C |

**Reverted (closed without merge):**

| PR | Reason |
|---|---|
| PR #4936 (mitigation 3, `EpochDrainCursor` + delegate-atomic chunking) | Superseded by voter-atomic `VoterRewardEraCursor` in §8.6. Reusable fragments extracted into PR A / C / D per §8.4 below. |

### 8.2 New PR sequence

Targeting `iotexproject/iotex-core` upstream, in order:

**Preflight — close PR #4936 without merge.** No code churn; the branch is
preserved for salvage (§8.4).

**PR A — genesis params + context helper (~small)**
- `blockchain/genesis/genesis.go`: add `EpochsPerRewardEra` (uint64,
  default 24), `VoterBudgetPerBlock` (default 2000), `CompoundBatchSize`
  (default 500) to the `Rewarding` struct. Rename existing
  `EpochDrainChunkSize` → `VoterBudgetPerBlock` (semantic shift from
  "delegates per block" to "voters per block"; parser stays,
  never-observed field so no fixture rewrite needed).
- `action/protocol/context.go`: no new fork gate — reuse
  `!NoVoterRewardDistribution`. Add `IsEraBoundary(epochNum,
  epochsPerEra) bool` helper.
- Tests: genesis parse fixtures, boundary helper edge cases
  (`epochNum=0`, `epochsPerEra=1`).
- Verification: build/vet, all existing tests unchanged behavior.

**PR B — poll snapshot cadence gate (~small)**
- `action/protocol/poll/util.go` (or wherever `setCandidates` invokes
  `SnapshotForEpochReward` via the staking bridge): wrap the snapshot
  write call in `if IsEraBoundary(epochNum, cfg.EpochsPerRewardEra)
  { ... }`.
- Tests: snapshot present only at era-boundary epochs; intra-era
  snapshot bytes byte-identical across successive `PutPollResult`
  calls; a stake mutation between era boundaries does NOT alter the
  snapshot until the next boundary.
- No changes to snapshot payload format — off-chain consumers untouched.

**PR C — Phase 2 chunked credit (~large; flag for high review)**
Refactor of PR 3' + PR 4.7 + PR 4' drain-site. This is the largest and
most intrusive PR in the plan.
- `action/protocol/rewarding/rewardingpb/rewarding.proto`: add
  `VoterRewardEraCursor` message per §8.5.
- `action/protocol/rewarding/voter_reward.go`: refactor
  `distributeVoterReward` from one-shot to cursor-driven. Phase 2
  credits `unclaimedBalance` only (no compound call); appends voters
  whose compound preference is active to `compound_pending_set`
  (structure defined in PR D).
- `action/protocol/rewarding/reward.go`:
  - `GrantEpochReward` no longer drains voter pool per epoch. The
    delegate-side epoch-share calculation still fires every epoch; the
    per-delegate share is folded into the existing pending pool (§3.1)
    alongside block-reward contributions.
  - New handler `GrantEraVoterReward` runs Phase 2 chunks.
  - Orphan drain (§3.3) moves from epoch-close to era-close.
- `action/protocol/rewarding/protocol.go`: `CreatePostSystemActions`
  emits `GrantEraVoterReward` at era boundary or while cursor is live.
  Salvage the cursor-continuation pattern from PR #4936.
- Log format: rename `DelegateDistributed` → `EraVoterCredited`, add
  `chunk_seq` (uint32) + `is_final` (bool) per §8.7.
- Tests: cross-cadence equivalence (24 legacy per-epoch grants vs. 1
  v2 per-era grant under identical accrual), cursor lifecycle,
  chunked-mid-block revert safety, era-boundary overrun guard
  (§8.12 test 21), orphan drain at era-close.
- Splittable if reviewer prefers into **C1** (proto + cursor state
  helpers, additive), **C2** (`distributeVoterReward` refactor +
  `GrantEraVoterReward` handler + `CreatePostSystemActions` wiring),
  **C3** (log rename + payload change).

**PR D — Phase 3 hybrid compound (~medium)**
- New file `action/protocol/rewarding/compound_sweep.go`:
  - `compound_pending_set` state structure per §8.5 (secondary index).
  - `runCompoundSweep` handler consuming up to `CompoundBatchSize`
    voters per block via `AutoDeposit` bridge (PR 4.6, unchanged).
- `action/protocol/rewarding/protocol.go`: `CreatePostSystemActions`
  also emits `CompoundSweep` every block when `compound_pending_set`
  is non-empty.
- `action/protocol/rewarding/reward.go`: augment `Claim` handler with
  lazy compound — if opt-in preference is active, route through
  `AddDepositForCompound` and decrement `unclaimedBalance`.
- `action/protocol/rewarding/voter_reward.go`: Phase 2 add-to-set
  hook per §8.11.
- Tests: sweep drains at `CompoundBatchSize × block_rate`; lazy
  `Claim` compound routing; Phase 3 pause → resume backlog
  convergence (§8.12 test 20).

**PR F — off-chain migration guide (~small, docs only)**
- Hermes-patch filter change (observe `VoterRewardOnchainOptIn` at
  era boundaries, not epoch boundaries; safe superset if the filter
  runs every epoch).
- Verifier tool aggregation key `(epoch, delegate)` →
  `(era_start_epoch, delegate)`.
- Explorer / dashboard event-schema update to `EraVoterCredited`.

### 8.3 Sequencing rationale

- **PR A first** — pure additive, unblocks PR B and PR C which depend
  on the new genesis fields.
- **PR B second** — small, unlocks the per-era snapshot semantics
  that PR C's tests need to gate against.
- **PR C third** — largest and most intrusive; PR A + PR B in place
  before this to have concrete config and snapshot behavior to gate on.
- **PR D fourth** — depends on PR C's `compound_pending_set` writer
  being present. Ships Phase 3 alongside Phase 2 for launch parity
  with the pre-v2 automatic-compound UX.
- **PR F last** — docs; no code dependency.

Fork-height activation (task #17) references PR A–D as a single
bundle. PR F is post-fork operational and can ship at its own
cadence.

### 8.4 Salvage from PR #4936

Extract into subsequent PRs when closing #4936:

1. `genesis.Rewarding.EpochDrainChunkSize` field + parser → rename to
   `VoterBudgetPerBlock` in **PR A**. Existing field is unused
   post-close, so the rename is safe.
2. `CreatePostSystemActions` cursor-continuation pattern (emit a
   system action while a cursor is live in state) → structural reuse
   in **PR C** (for `GrantEraVoterReward`) and **PR D** (for
   `CompoundSweep`).
3. Cursor state-manager helpers (load / store / delete over a proto
   blob keyed by a small prefix) → pattern reusable; write new
   helpers targeting `VoterRewardEraCursor` in **PR C**.

Non-reusable, delete when #4936 is closed: `EpochDrainCursor` proto,
`epoch_drain.go` (freeze / chunk / coda decomposition doesn't match
the v2 phase model), the delegate-atomic loop in `epoch_drain.go`,
`epoch_drain_cursor.go`, and their tests.

## 9. Verification

- **Cross-era equivalence:** run 24 epochs of rewards through both the
  legacy per-epoch path (fork gate off) and the era-based path (fork gate
  on, `EpochsPerRewardEra = 24`); assert per-voter cumulative
  `unclaimedBalance + bucket.stakedAmount` matches within rounding
  tolerance.
- **Phase 2 peak block wall-clock:** E2E bench (task #68 reforged) —
  trie-backed factory, 27,020 voters, `VoterBudgetPerBlock = 5000`,
  measure the era-boundary block wall-clock. Target: < 800ms.
- **Snapshot stability across an era:** run 24 epochs with mixed staking
  actions between them; assert `SnapshotForEpochReward` returns the same
  bytes for every intra-era read.
- **Phase 3 backlog convergence:** simulate a 24-hour Phase 3 pause,
  restart, verify backlog drains at `CompoundBatchSize × block_rate`.
- **Determinism harness (task #16):** era-based path executed under
  block-replay with intra-era reverts.

## 10. Open questions

1. Should `EpochsPerRewardEra` be a hardcoded constant or a genesis
   parameter? (Recommendation: genesis parameter with default 24, for
   testnet flexibility.)
2. Do we expose a query API for `pending_block_reward_pool[delegate]` so
   voters can see accrued-but-not-credited rewards? (Recommendation: yes,
   as an EVM view function on a system contract.)
3. Should Phase 3 handle contract-staking (V1/V2/V3 CSC) voters or only
   native? (Recommendation: identical to current behavior — bucket
   ownership check applies uniformly.)
4. Do we need per-voter fork migration — i.e., a voter's earned rewards
   at fork height distributed pro-rata across their pre-fork stake vs
   post-fork era boundary? (Recommendation: no, use first-era-boundary
   snapshot for all pre-fork accrued rewards; document as known trade-off.)
5. How to handle the era boundary conflict with the existing
   `EpochRewardHistoryKeyPrefix` sentinel semantics? (Sentinels become
   per-era instead of per-epoch; migration requires no history rewrite.)
