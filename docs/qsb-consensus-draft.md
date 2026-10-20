# QSB consensus integration draft

This document describes a **draft, non-activated** integration plan for
[IIP draft PR #86](https://github.com/iotexproject/iips/pull/86), Quantum-Safe
Staking Buckets (QSB). It accompanies the scaffold package at
`action/protocol/staking/bucketguard`.

Nothing in this branch changes mainnet behavior. No action is registered, no
hardfork height is selected, and no staking state is mutated.

## Scope of the scaffold

The package implements the consensus-neutral pieces:

- guard record and action types;
- byte-exact CALL/ACCEPT transcripts;
- canonical proto3 encoding, size limits, and intrinsic gas;
- stateless validation;
- SLH-DSA-SHA2-192s verification with `IOTEX-PQ-BUCKET-v1`;
- a fork-gate helper that requires an explicit non-zero activation height.

It does **not** include protobuf changes, action registration, state storage,
receipts, handler wiring, or a hardfork height.

## Integration sequence

1. **Protobuf and action types.** Add `BindBucketGuard`, `GuardedBucketOp`,
   `NewGuard`, and the new receipt statuses to `iotex-proto`; add the
   `ActionCore` oneof fields. Serialization changes need maintainer sign-off.
2. **Fork gate.** Add a new `FeatureCtx` field and a new hardfork height in
   `blockchain/genesis/genesis.go`. Never reuse or edit an existing mainnet
   height.
3. **State.** Store `BucketGuard` under a new namespace with an append-only
   schema. Include tombstones and the generation/nonce counters.
4. **Execution.** Dispatch `BindBucketGuard` and `GuardedBucketOp` from the
   staking protocol. Snapshot before writes; classify deterministic failures as
   failed receipts and backend failures as fatal block errors.
5. **Legacy-path audit.** Block every legacy path that can change a guarded
   bucket: `Unstake`, `WithdrawStake`, `TransferStake`, `ChangeCandidate`,
   `Restake`, `MigrateStake`, consignment, candidate activation/endorsement,
   contract staking, and delegate self-stake. Deposit remains allowed.
6. **Receipts and logs.** Emit the IIP statuses and one guard event per
   successful bind/op; keep the gas-settlement logs on failure.
7. **Testing.** Cover pre-fork/post-fork branches, every rejected path,
   replay/rotation/clear/rebind, competing sponsors, snapshot rollback, backend
   failure injection, fuzzing, ACVP vectors, and exact size/gas/receipt vectors.
8. **Performance.** Run the full-action harness on native linux/amd64, with and
   without SHA-NI, before fixing `G_VERIFY`. The current draft uses 30,000.
9. **Audit.** Consensus-critical crypto and state transitions need an external
   audit before activation.

## Known open questions

- Hardfork name and height.
- Protobuf field numbers and `ActionCore` oneof numbering.
- Whether bucket indices are guaranteed never to be reused after deletion; if
  not, tombstone deletion plus index reuse could create a replay surface.
- Wallet/exchange support for SLH-DSA signing and seed-derived recovery.
- Native linux/amd64 gas calibration and a minimum validator hardware baseline.

## Non-goals

- Wallet KDF implementation (`QSB-KDF-v1` is a wallet profile, not consensus).
- Contract staking, liquid staking, or delegate self-stake protection.
- Any ECDSA escape hatch or emergency recovery mode.
