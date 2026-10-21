# QSB action and state integration draft

Status: **draft, non-activated**. The reference implementation is wired behind
a disabled fork gate. It does not select a hardfork height. The protobuf change
lives on `iotexproject/iotex-proto` branch `qsb/guarded-bucket-proto` at commit
`4f59d11` and still requires maintainer sign-off before merge.

See [`qsb-spec-and-vectors.md`](./qsb-spec-and-vectors.md) for the frozen
encoding, statuses, event, and vectors, and
[`qsb-legacy-path-audit.md`](./qsb-legacy-path-audit.md) for the bypass audit.

## 1. Fork gate

Add one feature flag to `action/protocol/context.go`:

```go
// EnableBucketGuard gates the QSB bind/guarded-op actions and every
// legacy-path rejection. Zero-value is disabled.
EnableBucketGuard bool
```

Populate it in `WithFeatureCtx` from a new genesis height:

```go
EnableBucketGuard: g.IsQuantumSafeBuckets(height),
```

Add the height to `blockchain/genesis/genesis.go` with the same pattern as the
existing forks. The draft must use `0` (disabled) until maintainers approve a
name and height. Never reuse or edit an existing mainnet height.

`bucketguard.Active(blockHeight, forkHeight)` already implements the boundary
rule. Tests must cover `height = fork-1` (action invalid/not dispatched; legacy
paths unchanged) and `height = fork` (new actions valid; legacy paths reject
guarded buckets). A zero height means permanently disabled.

## 2. Protobuf and action types

### 2.1 `iotex-proto`

Implemented on `iotexproject/iotex-proto` branch `qsb/guarded-bucket-proto`
(commit `4f59d11`): messages from
[`qsb-proto-draft.proto`](./qsb-proto-draft.proto), `ActionCore` oneof fields
59/60, and receipt statuses 216–221. Go bindings were regenerated with
`protoc-gen-go v1.34.2`. `iotex-core` depends on the pseudo-version
`v0.6.14-0.20261021040621-4f59d11d9ea3`; replace it with the merged release
before production. Final field/oneof/status numbers still need maintainer
approval.

### 2.2 `action/`

Implemented:

- `action/bind_bucket_guard.go` — `BindBucketGuard`.
- `action/guarded_bucket_op.go` — `GuardedBucketOp`.

Both wrap the canonical structs in `bucketguard` and implement the existing
`actionPayload` contract:

```go
IntrinsicGas() (uint64, error)
SanityCheck() error
FillAction(*iotextypes.ActionCore)
```

and a `LoadProto`/`Proto` pair. `SanityCheck` calls the stateless validators in
`action/protocol/staking/bucketguard` and returns `ErrInvalidAct` on failure.
`IntrinsicGas` returns the canonical gas, not generic staking payload pricing.

Envelope/deserializer cases are implemented in `action/envelope.go`:

```go
case pbAct.GetBindBucketGuard() != nil:
    act := &BindBucketGuard{}
    if err := act.LoadProto(pbAct.GetBindBucketGuard()); err != nil {
        return err
    }
    elp.payload = act
case pbAct.GetGuardedBucketOp() != nil:
    act := &GuardedBucketOp{}
    if err := act.LoadProto(pbAct.GetGuardedBucketOp()); err != nil {
        return err
    }
    elp.payload = act
```

`FillAction` is implemented on both payloads. Neither action is wrapped in a
generic inner-action envelope.

## 3. Staking dispatch

Implemented in `action/protocol/staking/protocol.go`:

```go
case *action.BindBucketGuard:
    return p.validateBindBucketGuard(ctx, act)
case *action.GuardedBucketOp:
    return p.validateGuardedBucketOp(ctx, act)
```

and:

```go
case *action.BindBucketGuard:
    rLog, tLogs, err = p.handleBindBucketGuard(ctx, act, csm)
case *action.GuardedBucketOp:
    rLog, tLogs, err = p.handleGuardedBucketOp(ctx, act, csm)
```

Both handlers live in `handler_bucket_guard.go` and are behind
`FeatureCtx.EnableBucketGuard`; before the fork `Validate` returns
`action.ErrInvalidAct`. `Validate` is stateless; `handle` re-checks
execution-time state. The guarded path uses an explicit per-bucket authorization
context when it reuses legacy effects, so legacy handlers still reject guarded
buckets when called directly.

### 3.1 `BindBucketGuard`

Execution:

1. Snapshot the state manager.
2. `fetchBucket`; absent = `ErrInvalidBucketIndex`.
3. Require the bucket is native, not a contract/NFT bucket, and not a self-stake
   bucket (`ErrGuardScope`). Use the existing `isSelfStakeBucket` helper.
4. Require `actionCtx.Caller` equals `bucket.Owner` (`ErrUnauthorizedOperator`).
5. Load the guard record. It must be absent or a tombstone (`ErrGuardScope`).
6. `generation+1` must not overflow (`ErrGuardMismatch`).
7. Verify `acceptSig` over ACCEPT with `newGeneration=generation+1`,
   `resultingOwner=owner`, `deadline=0` (`ErrGuardSignature`).
8. Write the guard record and emit the BIND event. Do not change the bucket,
   owner, candidate, votes, or pool.
9. Settle gas and nonce.

Binding is never closed. An unguarded bucket can be bound at any height.

### 3.2 `GuardedBucketOp`

Execution:

1. Snapshot the state manager.
2. `fetchBucket`; absent = `ErrInvalidBucketIndex`.
3. Require a guarded record (`ErrBucketNotGuarded`).
4. Require `generation` and `nonce` equal stored values, no counter overflow, and
   `blockHeight <= deadline` (`ErrGuardMismatch`).
5. Run op-specific preconditions through the existing handlers/helpers. Do not
   duplicate maturity, auto-stake, endorsement, duration, or candidate checks.
6. For CHANGE_CANDIDATE, resolve `candidateName` at execution time and require
   its identifier equals `expectedCandidateID` (`ErrCandidateMismatch`).
7. Verify the CALL signature with the current key (`ErrGuardSignature`). Verify
   ACCEPT if `newGuard` is present (`ErrGuardSignature`).
8. Apply the bucket effect through existing staking helpers. The outer ECDSA
   sender is only a sponsor: it pays gas and advances its own account nonce.
9. Update/delete the guard record per the transition table; emit exactly one
   guard event; settle.

The sponsor may be any account. Do not pass a general "guard authorized" flag
into legacy handlers; pass an explicit authorization scoped to
`(bucketIndex, generation, op)`, and always act on `act.BucketIndex()`.

### 3.3 Effect reuse

Use existing state-manager helpers rather than copying logic:

| Op | Reuse |
|---|---|
| UNSTAKE | `handleUnstake` effect, but skip the owner/self-stake checks that the guard path replaces |
| WITHDRAW | `delBucketAndIndex`, `CreditBucketPool`, credit `recipient` instead of caller, delete guard |
| TRANSFER | `delVoterBucketIndex` / `putVoterBucketIndex`, update owner, update guard |
| CHANGE_CANDIDATE | `delCandBucketIndex` / `putCandBucketIndex`, update candidate, votes |
| RESTAKE | duration/start/autoStake update, vote delta |
| ROTATE | guard record only |

For WITHDRAW, the transaction log recipient is `recipient`, not the sponsor. The
legacy `WithdrawStake` handler is not called for a guarded withdraw.

## 4. State

### 4.1 Namespace and key

Use the staking namespace prefix `_bucketGuard` and key
`uint64_be(bucketIndex)`. The value is the protobuf `BucketGuard` from
[`qsb-proto-draft.proto`](./qsb-proto-draft.proto).

The scaffold now provides `BucketGuardStateManager` and
`BucketGuardStateReader` in `action/protocol/staking/bucket_guard.go`:

```go
func (bsr *BucketGuardStateReader) Get(index uint64) (*BucketGuard, error)
func (bsm *BucketGuardStateManager) Put(index uint64, g *BucketGuard) error
func (bsm *BucketGuardStateManager) Delete(index uint64) error
```

`Get` returns `nil, nil` for an absent record; the caller treats it as
`generation=0, nonce=0`. `Put`/`Delete` use the `_bucketGuard` key prefix under
`state.StakingNamespace`. Erigon registration and migration are still pending.
A tombstone is a present record with empty `publicKey`.

### 4.2 Snapshot and delete rules

- Write the guard record through the same state manager and snapshot as the
  bucket. Revert must cover guard, bucket, owner/candidate indexes, pool, and
  votes.
- Delete the guard record, including a tombstone, in the same snapshot whenever
  the bucket is permanently deleted:
  - guarded WITHDRAW;
  - legacy `WithdrawStake` on a bucket that was cleared earlier;
  - legacy `MigrateStake` on a bucket that was cleared earlier.
- Never reset or decrement `totalBucketCount`; bucket indices are not reused.
- No migration of existing buckets is needed. The new prefix is empty at the
  fork. Old nodes do not read it.

### 4.3 Erigon

Register the new prefix as a storage object in the Erigon backend and add it to
state readers/views. Test both the default and Erigon backends, including
historical reads and restart/reorg.

## 5. Receipts and logs

Use the IIP statuses. Deterministic failures revert to the snapshot, emit no
success logs, charge the sponsor gas, and advance the sponsor nonce exactly once;
the guard nonce does not advance. Backend/corruption errors and failed snapshot
reverts are fatal block-processing errors, not failed receipts.

Emit one guard event per successful bind/op:

```text
topic[0] = 0x73a1cc2c5c34de8d095f5003eb096fa77f44357dd83dcea0225ca286942426ec
topic[1] = uint64_be(bucketIndex), 32-byte left-padded
topic[2] = uint64_be(op), 32-byte left-padded; 0 = BIND
data     = uint64_be(generation) || uint64_be(nonce) || sponsor[20] || counterparty[20]
```

Do not emit the legacy per-handler logs for guarded ops. Keep gas-fee logs on
failure and one `WITHDRAW_BUCKET` transaction log for guarded WITHDRAW.

## 6. Legacy-path guards

The scaffold provides a helper that returns a deterministic `ErrBucketGuarded`
receipt error when a record is present and guarded, and a fatal error for
backend failures:

```go
func requireUnguarded(enabled bool, sr protocol.StateReader, index uint64) error
```

`enabled` is `FeatureCtx.EnableBucketGuard`; before the fork the helper returns
nil without touching state, so pre-fork behavior is unchanged. It is called
after fetching the bucket and before any mutation in the native
paths listed in [`qsb-legacy-path-audit.md`](./qsb-legacy-path-audit.md). Do not
put it inside `fetchBucketAndValidate` unconditionally: `DepositToStake` and
IIP-59 compounding must remain allowed, and `CandidateActivate` needs the target
bucket, not the caller's bucket. The draft statuses are 216 for
`ErrBucketGuarded` and 220 for `ErrGuardScope`; replace them with the generated
`iotextypes` values when `iotex-proto` adds them.

Minimum placements:

- `handleUnstake`: after `fetchBucketAndValidate`, before the first bucket write.
- `handleWithdrawStake`: after `fetchBucketAndValidate`, before `delBucketAndIndex`.
- `handleChangeCandidate`: after `fetchBucketAndValidate`, before index updates.
- `handleTransferStake`: after `fetchBucketAndValidate`, before the consignment
  fallback and before index updates. This must also cover consignment.
- `handleRestake`: after `fetchBucketAndValidate`, before the first bucket write.
- `handleStakeMigrate`: after `fetchBucket`, before `validateStakeMigrate`,
  snapshot, or `withdrawBucket`.
- `handleCandidateActivate`: on the target bucket, before mutating the previous
  self-stake or candidate weights.
- `handleCandidateEndorsement`: on the bucket, before endorsement state writes;
  revoke can clear a candidate's self-stake.
- `handleCandidateDeactivate`, `handleCandidateTransferOwnership`,
  `handleScheduleCandidateDeactivation`: defense-in-depth check when they touch
  `cand.SelfStakeBucketIdx`; a guarded bucket must never be a self-stake bucket.
- `handleDepositToStake` and `AddDepositForCompound`: no guard check; assert the
  guard record is unchanged.
- `handleCandidateRegister`, `handleCandidateUpdate`, `SetVoterRewardOptIn`:
  candidate-level paths; no bucket guard check, but self-stake creation must
  preserve the invariant that a guarded bucket is never self-stake.
- Contract staking/indexers: no QSB guard; `MigrateStake` is the only native-to-
  contract path and is blocked. Do not create guard records for contract/NFT
  buckets.

## 7. Tests

Minimum suite:

- Fork boundary: `fork-1` vs `fork`; zero height disabled.
- BIND: success, wrong owner, absent bucket, contract bucket, self-stake bucket,
  existing guard, tombstone rebind, generation overflow, bad ACCEPT.
- Each op: success and every precondition failure; wrong generation/nonce,
  expired deadline, wrong chain, wrong owner, wrong candidate ID, malformed
  fields, bad CALL, bad ACCEPT.
- Replay after rotation; replay after clear/rebind; competing sponsors.
- Guarded WITHDRAW deletes bucket and guard; recipient credited; legacy withdraw
  and migrate blocked.
- TRANSFER newGuard/clearGuard; ROTATE; tombstone retained counters.
- Every legacy rejected path in the audit, with state unchanged and guard nonce
  unchanged.
- Deposit and IIP-59 compound on a guarded bucket succeed and leave the guard
  untouched.
- Snapshot rollback on deterministic failure; fatal backend failure injection.
- Both storage backends, reorg, restart, historical read, migration.
- Fuzzing for canonical decoding and transcript parsing.
- Exact size/gas/digest vectors from `qsb-spec-and-vectors.md`.

## 8. Non-goals and blockers

- No fork height or fork name in this draft.
- No action registration or state mutation in the scaffold.
- No contract staking, liquid staking, or delegate self-stake protection.
- No ECDSA escape hatch or emergency recovery.
- Native linux/amd64 calibration and external audit remain blockers before
  activation.
