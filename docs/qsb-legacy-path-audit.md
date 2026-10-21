# QSB legacy-path audit

Status: **draft, non-activated**. This audit maps every staking path that can
touch a native bucket and states what the QSB integration must do. It is a
design review, not an implementation: the scaffold does not register actions or
mutate state.

Scope: native buckets only. Contract/NFT staking is outside QSB. The only native
to contract path is `MigrateStake`, which is blocked for a guarded bucket.

## 1. Invariants

1. A guarded bucket's principal, owner, candidate, duration, auto-stake flag,
   votes, and endorsement state cannot change without a valid CALL signature.
2. `DepositToStake` and IIP-59 `AddDepositForCompound` may change the amount
   without a guard signature; they must not create, delete, or alter a guard.
3. A guarded bucket is never a self-stake bucket. Binding rejects self-stake;
   candidate activation rejects a guarded target. Candidate paths that touch
   `SelfStakeBucketIdx` must enforce this as defense-in-depth.
4. When a bucket is permanently deleted, its guard record (including a
   tombstone) is deleted in the same snapshot.
5. `totalBucketCount` is monotonic; bucket indices are never reused.
6. Every rejection is a deterministic failed receipt with state reverted, no
   success logs, sponsor gas charged, and sponsor nonce advanced once. Backend
   failures are fatal, not failed receipts.

## 2. Path audit

### 2.1 `CreateStake`

- Entry: `handleCreateStake` in `action/protocol/staking/handlers.go`.
- Effect: creates a new native bucket through `putBucket`; no guard record.
- Required: allow. A new bucket is unguarded until BIND.
- Check: none.
- Tests: create then bind; create and legacy unstake/withdraw unchanged.

### 2.2 `BindBucketGuard` (new)

- Entry: new `handleBindBucketGuard`.
- Effect: creates a guard record; bucket unchanged.
- Required: caller must be bucket owner; bucket must be native and not
  self-stake; record must be absent or a tombstone; `generation+1` no overflow;
  ACCEPT must verify. BIND is never closed.
- Check: owner, native/self-stake, record state, generation, ACCEPT.
- Tests: success; wrong owner; absent bucket; contract bucket; self-stake bucket;
  existing guard; tombstone rebind; overflow; bad ACCEPT; pre-fork invalid.

### 2.3 `Unstake`

- Entry: `handleUnstake` in `action/protocol/staking/handlers.go`.
- Effect: sets `UnstakeStartTime`, subtracts votes, clears self-stake if
  applicable.
- Required: reject guarded bucket with `ErrBucketGuarded` before the first
  bucket or candidate write.
- Check: after `fetchBucketAndValidate`, before the `UnstakedButNotClear...`
  branch and vote updates.
- Tests: guarded reject; state unchanged; guard nonce unchanged; unguarded
  behavior unchanged; self-stake path unchanged.

### 2.4 `WithdrawStake`

- Entry: `handleWithdrawStake` in `action/protocol/staking/handlers.go`.
- Effect: deletes bucket and indexes, credits caller, credits bucket pool.
- Required: reject guarded bucket with `ErrBucketGuarded` before
  `delBucketAndIndex`. A guarded WITHDRAW is a separate `GuardedBucketOp` that
  credits `recipient` and deletes the guard.
- Check: after `fetchBucketAndValidate`, before deletion.
- Tests: guarded reject; bucket and guard untouched; unguarded behavior
  unchanged; guarded WITHDRAW deletes both and credits recipient.
- After `clearGuard`: the bucket is ECDSA-controlled again; legacy withdraw is
  allowed and must delete the tombstone in the same snapshot.

### 2.5 `ChangeCandidate`

- Entry: `handleChangeCandidate` in `action/protocol/staking/handlers.go`.
- Effect: changes candidate, voter/candidate indexes, and votes; may clear a
  previous candidate's self-stake pointer if the bucket was its self-stake.
- Required: reject guarded bucket with `ErrBucketGuarded` before index or vote
  writes.
- Check: after `fetchBucketAndValidate`, before `delCandBucketIndex`.
- Tests: guarded reject; no index/vote change; guard nonce unchanged.

### 2.6 `TransferStake` and consignment

- Entry: `handleTransferStake` and `handleConsignmentTransfer` in
  `action/protocol/staking/handlers.go`.
- Effect: changes bucket owner, voter index, and guard (for guarded TRANSFER).
  The legacy path supports a signed consignment payload when the caller is not
  the owner.
- Required: reject guarded bucket with `ErrBucketGuarded` before the
  consignment fallback and before `delVoterBucketIndex`. The consignment path
  must not bypass the guard check: `fetchBucketAndValidate` returns
  `ErrUnauthorizedOperator` together with the bucket, and the fallback checks
  signer/transferee/asset ID/nonce. The guard check must run before that
  fallback and must not be converted into a consignment attempt.
- Check: immediately after `fetchBucketAndValidate`, before the
  `handleConsignmentTransfer` call.
- Tests: guarded owner transfer reject; guarded consignment reject; no owner or
  index change; unguarded consignment unchanged; guarded TRANSFER with
  `newGuard`/`clearGuard` works.

### 2.7 `DepositToStake`

- Entry: `handleDepositToStake` in `action/protocol/staking/handlers.go`.
- Effect: increases `StakedAmount`, updates votes; requires auto-stake.
- Required: allow on a guarded bucket under existing conditions; guard record
  unchanged.
- Check: no guard rejection. Assert guard record is unchanged after the write.
- Tests: deposit on guarded bucket succeeds; amount and votes update; guard
  unchanged; pre-signed guarded op acts on the new amount.

### 2.8 `Restake`

- Entry: `handleRestake` in `action/protocol/staking/handlers.go`.
- Effect: changes duration, start time, auto-stake, votes.
- Required: reject guarded bucket with `ErrBucketGuarded` before the first
  bucket write.
- Check: after `fetchBucketAndValidate`, before duration/start updates.
- Tests: guarded reject; state unchanged; unguarded behavior unchanged;
  duration bounds reused.

### 2.9 `MigrateStake`

- Entry: `handleStakeMigrate` and `validateStakeMigrate` in
  `action/protocol/staking/handler_stake_migrate.go`.
- Effect: withdraws the native bucket, credits the caller, calls the staking
  contract, deletes the bucket and indexes.
- Required: reject guarded bucket with `ErrBucketGuarded` before
  `validateStakeMigrate`, before the snapshot/withdraw, and before any contract
  call. Leaving to ERC-721 control is not permitted.
- Check: after `fetchBucket`, before `validateStakeMigrate`/`withdrawBucket`.
- Tests: guarded reject; bucket, guard, pool, votes, and contract state
  unchanged; no execution receipt; unguarded migration unchanged.
- After `clearGuard`: migration is allowed and must delete the tombstone.

### 2.10 `CandidateRegister`

- Entry: `handleCandidateRegister` in `action/protocol/staking/handlers.go`.
- Effect: creates a candidate; if amount > 0, creates a self-stake bucket.
- Required: allow. The new self-stake bucket is unguarded; BIND must reject
  self-stake buckets. `CandidateRegisterMustWithStake` behavior is unchanged.
- Check: none for the guard; enforce self-stake invariant in BIND.
- Tests: register with self-stake; bind self-stake rejected; register without
  self-stake; candidate metadata unchanged by QSB.

### 2.11 `CandidateUpdate` and `CandidateUpdateByOperator`

- Entry: `handleCandidateUpdate` and `handleCandidateUpdateByOperator` in
  `action/protocol/staking/handlers.go`.
- Effect: candidate name/operator/reward/BLS metadata; no bucket mutation.
- Required: allow; no guard check.
- Tests: guarded bucket owner updates candidate metadata; guard unchanged.

### 2.12 `CandidateActivate`

- Entry: `handleCandidateActivate` in
  `action/protocol/staking/handler_candidate_selfstake.go`.
- Effect: converts a vote bucket to self-stake, changes candidate weights and
  `SelfStakeBucketIdx`.
- Required: reject if the target bucket is guarded with `ErrBucketGuarded`
  before mutating the previous self-stake or candidate weights.
- Check: after `fetchBucket`, before the previous-self-stake block.
- Tests: guarded target reject; candidate weights and `SelfStakeBucketIdx`
  unchanged; unguarded activation unchanged; self-stake invariant.

### 2.13 `CandidateDeactivate`

- Entry: `handleCandidateDeactivate` in
  `action/protocol/staking/handler_candidate_selfstake.go`.
- Effect: request/confirm deactivation; confirm clears the candidate's
  self-stake pointer/weights. No bucket delete.
- Required: a guarded bucket must never be a self-stake bucket. Add a
  defense-in-depth check on `cand.SelfStakeBucketIdx`: if a guard record is
  present and guarded, reject with `ErrGuardScope` or fail the invariant
  explicitly. Do not silently clear it.
- Check: before `csm.deactivate`.
- Tests: invariant test with a corrupted guarded self-stake record; normal
  deactivation unchanged.

### 2.14 `CandidateEndorsement`

- Entry: `handleCandidateEndorsement` in
  `action/protocol/staking/handler_candidate_endorsement.go`.
- Effect: endorse/intent-to-revoke/revoke; writes or deletes endorsement state;
  revoke can clear a candidate's self-stake.
- Required: reject if the bucket is guarded with `ErrBucketGuarded` before any
  endorsement state write or self-stake clear.
- Check: after `fetchBucket`, before `esm.Put`/`esm.Delete` and before the
  revoke self-stake branch.
- Tests: all three ops on guarded bucket reject; endorsement state unchanged;
  candidate self-stake unchanged; unguarded behavior unchanged.

### 2.15 `CandidateTransferOwnership`

- Entry: `handleCandidateTransferOwnership` in
  `action/protocol/staking/handler_candidate_transfer_ownership.go`.
- Effect: changes candidate owner; may clear an expired-endorsement self-stake
  bucket to a vote bucket.
- Required: candidate metadata path; no bucket principal change. If the
  self-stake bucket has a guard record, reject with `ErrGuardScope` as a
  defense-in-depth invariant violation. A guarded bucket must never be
  self-stake.
- Check: before the `needClear`/self-stake clear branch.
- Tests: normal ownership transfer unchanged; corrupted guarded self-stake
  invariant test.

### 2.16 `ScheduleCandidateDeactivation`

- Entry: `handleScheduleCandidateDeactivation` in
  `action/protocol/staking/handler_candidate_selfstake.go`.
- Effect: sets `DeactivatedAt` and exit epoch; no bucket mutation.
- Required: no bucket guard check, but enforce the self-stake invariant if the
  candidate's self-stake bucket has a guard record. This is a system action.
- Tests: normal scheduling unchanged; invariant test.

### 2.17 `SetVoterRewardOptIn`

- Entry: `handleSetVoterRewardOptIn` in
  `action/protocol/staking/handler_set_voter_reward_opt_in.go`.
- Effect: sets a candidate flag; no bucket mutation.
- Required: allow; no guard check.
- Tests: guarded bucket owner opts in; guard unchanged.

### 2.18 IIP-59 `AddDepositForCompound`

- Entry: `Protocol.AddDepositForCompound` in
  `action/protocol/staking/add_deposit_compound.go`.
- Effect: increases a bucket's `StakedAmount` during the era drain; no action
  dispatch.
- Required: allow; guard record unchanged. It is not a guarded operation and
  must not create/delete a guard.
- Check: assert guard unchanged after the compound write.
- Tests: compound on guarded bucket succeeds; amount updates; guard unchanged;
  no guard nonce change.

### 2.19 Contract staking and NFT buckets

- Entry: `contractstaking` indexers (`contractStakingIndexer`,
  `contractStakingIndexerV2`, `contractStakingIndexerV3`), `handleStakingIndexer`
  in `action/protocol/staking/protocol.go`.
- Effect: separate contract/NFT bucket state; no native bucket guard.
- Required: no QSB guard for contract buckets. `MigrateStake` is the only
  native-to-contract path and is blocked for guarded buckets. Do not create guard
  records for contract/NFT buckets. Contract staking remains ECDSA/contract
  controlled (residual exposure).
- Tests: guarded native bucket cannot migrate; contract bucket has no guard
  record; indexer behavior unchanged.

### 2.20 Delegate self-stake and operator paths

- Entry: `CandidateRegister` (self-stake creation), `CandidateActivate`
  (vote→self-stake), `CandidateDeactivate`, `CandidateEndorsement` revoke,
  `CandidateTransferOwnership`.
- Effect: self-stake pointer, candidate weights, candidate identity/operator.
- Required: self-stake buckets cannot be bound. CandidateActivate cannot target
  a guarded bucket. Candidate paths must enforce the invariant that a guarded
  bucket is never a self-stake bucket. Delegate operator keys and self-stake
  remain ECDSA-controlled (residual exposure).
- Tests: bind self-stake reject; activate guarded target reject; invariant tests
  for candidate paths; residual exposure documented.

## 3. Test matrix

For every rejected path:

- Guarded bucket → `ErrBucketGuarded` (or `ErrGuardScope` for invariant
  violations).
- Bucket, owner/candidate indexes, votes, pool, endorsement state, and guard
  record unchanged.
- Guard nonce unchanged.
- No success logs; only gas-settlement logs.
- Sponsor charged and sponsor nonce advanced once.
- Pre-fork: new actions invalid/not dispatched; legacy behavior unchanged.
- Post-fork: new actions valid; legacy behavior unchanged for unguarded buckets.

Additional cases:

- Guarded WITHDRAW deletes bucket and guard; recipient credited; legacy withdraw
  and migrate blocked.
- TRANSFER `newGuard` / `clearGuard`; ROTATE; tombstone counters retained.
- Deposit and IIP-59 compound succeed with guard unchanged.
- Consignment cannot bypass the guard.
- `MigrateStake` cannot reach the contract call for a guarded bucket.
- CandidateActivate/CandidateEndorsement cannot mutate a guarded bucket.
- Snapshot rollback, backend failure injection, both storage backends, reorg,
  restart, historical read, migration.
- Fuzzing for canonical decoding and transcript parsing.
- Exact size/gas/digest vectors from `qsb-spec-and-vectors.md`.

## 4. Residual risks

- Pre-bind ECDSA compromise: binding is authorized by the current owner; an
  attacker can bind an attacker-chosen key before the owner does.
- `clearGuard`: a guard-signed TRANSFER can opt out and return to ECDSA control.
- Lost guard key: locked bucket unless the wallet seed is available.
- Deposits/compounds change execution-time amount; guarded ops do not commit to
  amount.
- Candidate identity/operator, delegate self-stake, contract staking, liquid
  staking, and rewards remain outside the guard.
- No ECDSA escape hatch or emergency recovery.

These are documented in `qsb-spec-and-vectors.md` §10 and must be repeated in
user-facing material before activation.
