# CLAUDE.md — action/protocol/staking

Module-specific guidance for the staking protocol. Read [/AGENTS.md](../../../AGENTS.md) first for repo-wide rules.

## What this package does

Implements native staking: candidates, vote buckets, endorsements, and the
bridge to NFT-based contract staking. The protocol runs on every block and
mutates state under three namespaces:

- `state.StakingNamespace` — buckets, voter index, candidate index, endorsements
- `state.CandidateNamespace` — candidate records
- `state.CandsMapNamespace` — legacy name/operator/owner mapping (kept for
  backward compatibility; do not remove)

---

## Module red lines

### Receipt log emission — events path, not legacy

- Post-Fairbank, emit logs via `receiptLog.AddEvent(topics, data)`, not by
  setting `r.topics` / `r.data` directly. `Build()` branches on
  `len(r.events)`; the legacy single-log path **silently discards `data`**
  when `events` is non-empty. This was issue #4829 (CandidateDeactivation
  event data was lost); fixed in `f03a4ac16` / `673d4400e`.
- Non-indexed event arguments **must** land in `Data`, indexed in `Topics`.
  If your event declares `blockNumber` non-indexed, it goes through `AddEvent`.
- See `receipt_log.go`.

### Topic hashing — Fairbank switch

- Pre-Fairbank uses `hash.Hash256b()`; post-Fairbank uses
  `hash.BytesToHash256()`. The constructor `newReceiptLog()` takes a
  `postFairbankMigration bool` from `featureCtx.NewStakingReceiptFormat`.
- Never hardcode the hash function in new code. Always pass the flag and let
  the constructor pick.
- See `receipt_log.go`.

### Candidate `DeactivatedAt` is a sentinel-laden height

- `0` — active, not deactivating
- `math.MaxUint64` — exit requested by user, not yet scheduled
- any other value — scheduled; candidate exits at the named block

The sentinel is internal. Do not expose `MaxUint64` in ABIs or external
views — unpack via the `candidateDeactivation(address)` view (added in
`355e22d3a`) which returns `(requested, scheduledAtBlock)`.

### Self-stake bucket cannot be withdrawn while the candidate is active

- Enforced in `handlers.go` / `bucket_validation.go`. Revoking the
  self-stake endorsement triggers either an exit-queue scheduling (when
  `featureCtx.NoCandidateExitQueue` is false) or immediate deactivation
  (when true).
- Never bypass `isSelfStakeBucketSettled` in withdrawal paths.

### Endorsement `ExpireHeight` is a sentinel-laden height

- Legacy mode: `MaxUint64` = endorsed indefinitely.
- New mode: a real block height = the earliest block at which revoke
  becomes effective.
- Use `endorsement.Status(height)` to compare. Do not treat `MaxUint64`
  as an enum value.
- Branching governed by `featureCtx.EnforceLegacyEndorsement`.

### Three contract-staking indexers run in parallel

- `contractStakingIndexer` (V1), `contractStakingIndexerV2`,
  `contractStakingIndexerV3`. All three contribute to the `contractsStake`
  view. Event handling in `Handle()` calls processors for **each** of them.
- Do not assume only one is active. Do not rename
  `MigrateContractAddress` or `TimestampedMigrateContractAddress` in
  genesis without updating the corresponding indexer wiring.
- See `protocol.go`.

### Bucket state transitions are not idempotent — guard them

- Lifecycle: `Create` → `Restake`* → `Unstake` → `Withdraw`.
- `UnstakeStartTime != 0` means "already unstaked." Calling `Unstake`
  again must error.
- `Withdraw` requires `UnstakeStartTime + WithdrawWaitingPeriod ≤ now`.
- Native buckets use block heights for timing; contract-staking buckets
  use Unix timestamps. Do not mix the two in comparisons.

### State key prefixes are a 1-byte namespace; do not invent new ones

- `_const = 0`, `_bucket = 1`, `_voterIndex = 2`, `_candIndex = 3`,
  `_endorsement = 4`. Adding a new state type means adding a tag in
  `protocol.go` — review for collisions.

### Stake migration must snapshot + revert on contract failure

- `handler_stake_migrate.go` snapshots the state manager before calling
  the contract; on failure the snapshot is restored. Any new
  state-mutating path that calls into the EVM must do the same.

---

## Where to look (non-obvious mappings only)

- One file per action type: `handler_*.go` (plus `handlers.go` for shared logic).
- Contract staking V1/V2/V3 indexers: `contractstake_indexer*.go`,
  `contractstaking/`.
