# QSB action and state integration draft

Status: **draft, non-activated**. This is the wiring plan for the reference
implementation behind a disabled fork gate. It does not select a hardfork height
or register an action in this branch. Protobuf changes require a separate
`iotex-proto` change and maintainer sign-off.

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

The flag selects the rule version as well: post-fork code uses
`bucketguard.ParamsV1()`, and any later change of sizes, gas or algorithms is a
new `ParamsVn` behind a new flag. Tests must cover `height = fork-1` (action invalid/not dispatched; legacy
paths unchanged) and `height = fork` (new actions valid; legacy paths reject
guarded buckets). A zero height means permanently disabled.

## 2. Protobuf and action types

### 2.1 `iotex-proto`

Add the messages and `ActionCore` oneof fields from
[`qsb-proto-draft.proto`](./qsb-proto-draft.proto), regenerate Go bindings, and
bump the dependency. Add the six receipt statuses to `receiptstatus.proto`.
Final field/oneof/status numbers need maintainer approval.

### 2.2 `action/`

Add two payload types:

- `action/bind_bucket_guard.go` — `BindBucketGuard`.
- `action/guarded_bucket_op.go` — `GuardedBucketOp`.

Each type implements the existing `actionPayload` contract:

```go
IntrinsicGas() (uint64, error)
SanityCheck() error
FillAction(*iotextypes.ActionCore)
```

and a `LoadProto`/`Proto` pair. `SanityCheck` calls the stateless validators in
`action/protocol/staking/bucketguard` and returns `ErrInvalidAct` on failure.
`IntrinsicGas` returns the canonical gas, not generic staking payload pricing.

Add cases to `action/envelope.go`:

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

Add the corresponding `FillAction` calls so the payload can be serialized back.
Do not wrap either action in a generic inner-action envelope.

### 2.3 Web3 entry

Every legacy staking action is also reachable as an eth transaction to the
native staking address (`EthData` / `New*FromABIBinary`). The guarded actions
get the same entry, one ABI method per operation, defined in
`bucketguard/web3.go`:

| Method | Decodes to |
|---|---|
| `bindBucketGuard(uint64,uint32,bytes,bytes)` | `BindBucketGuard` |
| `guardedUnstake(uint64,uint64,uint64,uint64,bytes)` | UNSTAKE |
| `guardedWithdraw(uint64,uint64,uint64,uint64,address,bytes)` | WITHDRAW |
| `guardedTransfer(uint64,uint64,uint64,uint64,address,bool,uint32,bytes,bytes,bytes)` | TRANSFER |
| `guardedChangeCandidate(uint64,uint64,uint64,uint64,string,address,bytes)` | CHANGE_CANDIDATE |
| `guardedRestake(uint64,uint64,uint64,uint64,uint32,bool,bytes)` | RESTAKE |
| `guardedRotate(uint64,uint64,uint64,uint64,uint32,bytes,bytes,bytes)` | ROTATE |

The ABI is only an envelope: decoding (`BindBucketGuardFromEthData`,
`GuardedBucketOpFromEthData`) yields the same values as the native encoding,
which then follow the same validation, authorization and transition path.
Size limits and intrinsic gas are computed over C(m) of the decoded value, so
an action costs the same through either entry; the eth tx data length must
not be priced with the generic per-byte staking payload gas. At integration the
methods join `action.NativeStakingContractABI`, and the action types'
`EthData` delegate to `bucketguard`.

The SLH-DSA signatures are produced outside the eth wallet (by a
`pkg/pqcrypto/signer`-based tool); any eth wallet can then submit the call and
pay gas as the sponsor.

## 3. Staking dispatch

Add cases to both `Protocol.Validate` and `Protocol.handle` in
`action/protocol/staking/protocol.go`:

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

Both handlers must be behind `FeatureCtx.EnableBucketGuard`; before the fork they
must return an invalid-action error and must not be dispatched. `Validate` is
stateless and may reject malformed actions early; `handle` re-checks execution-
time state.

### 3.1 `BindBucketGuard`

Execution:

Steps follow IIP §5 in order; the order fixes which receipt status wins when
several checks fail.

1. Snapshot the state manager.
2. `fetchBucket`; absent = `ErrInvalidBucketIndex`. The bucket must be native,
   not a contract/NFT bucket (`ErrGuardScope`).
3. Require `actionCtx.Caller` equals `bucket.Owner` (`ErrUnauthorizedOperator`).
4. Require the bucket is not a self-stake bucket and has no endorsement record
   in any state (`ErrGuardScope`). Both checks are structural, per IIP §3: no
   candidate's `SelfStakeBucketIdx` equals the bucket index, and an
   endorsement-state `Get` returns `ErrStateNotExist`. Do **not** substitute
   `isSelfStakeBucket` or the endorsement `Status`.
5. Load the guard record. It must be absent or a tombstone (`ErrGuardScope`).
6. `generation+1` must not overflow (`ErrGuardMismatch`).
7. Verify `acceptSig` over ACCEPT with `newGeneration=generation+1`,
   `resultingOwner=owner`, zero `callHash` (`ErrGuardSignature`).

   Steps 4–7 are `bucketguard.AuthorizeBind(p, domain, owner, scope, cur,
   act, height)`, which returns the record to store. `domain` is
   `bucketguard.NewDomain(chainID, genesisHash)` from the block context.
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

Mapping to `bucketguard`: steps 3–4 are `CheckOp(cur, act, height)`; step 6 is
`act.CheckCandidate(resolvedID)`; step 7 is `VerifyOp(p, domain, owner, cur,
act)`; step 9 is `Next(cur, act, height)`. Step 5 also covers the staking
sanity checks the guard layer does not repeat: `validateRestake` for RESTAKE
and `action.IsValidCandidateName` for CHANGE_CANDIDATE, run in
`Protocol.Validate` exactly as for the legacy actions.

Error mapping is one table keyed with `errors.Is`: `ErrMalformed` makes the
action invalid (not includable); `ErrBucketGuarded`, `ErrBucketNotGuarded`,
`ErrGuardMismatch`, `ErrGuardSignature`, `ErrGuardScope` and
`ErrCandidateMismatch` map to the receipt statuses of IIP §9.5.

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

Add a `BucketGuardStateManager` or equivalent helper around
`protocol.StateManager`:

```go
func (m *bucketGuardSM) Get(index uint64) (*BucketGuard, error)
func (m *bucketGuardSM) Put(index uint64, g *BucketGuard) error
func (m *bucketGuardSM) Delete(index uint64) error
```

`Get` maps `state.ErrStateNotExist` to an absent record with `generation=0,
nonce=0`. A tombstone is a present record with empty `publicKey`.

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

Add a helper that returns a deterministic `ErrBucketGuarded` when a record is
present and guarded, and a fatal error for backend failures:

```go
func requireUnguarded(csm CandidateStateManager, index uint64) ReceiptError
```

Call it after fetching the bucket and before any mutation in every path listed
in [`qsb-legacy-path-audit.md`](./qsb-legacy-path-audit.md). Do not put it inside
`fetchBucketAndValidate` unconditionally: `DepositToStake` and IIP-59 compounding
must remain allowed, and `CandidateActivate` needs the target bucket, not the
caller's bucket.

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
