# bucketguard (draft scaffold)

**Status: non-activated.** This package does not register an action, choose a
hardfork height, or mutate staking state. It is a reviewable, consensus-neutral
port of the IIP "Quantum-Safe Staking Buckets" pieces that can be validated
before wiring the feature into iotex-core.

Design and conformance documents:

- [`docs/qsb-spec-and-vectors.md`](../../../../docs/qsb-spec-and-vectors.md)
- [`docs/qsb-legacy-path-audit.md`](../../../../docs/qsb-legacy-path-audit.md)
- [`docs/qsb-proto-draft.proto`](../../../../docs/qsb-proto-draft.proto)
- [`docs/qsb-action-integration.md`](../../../../docs/qsb-action-integration.md)

## What is here

- `types.go` — IIP constants, `Guard`, `BindBucketGuard`, `GuardedBucketOp`,
  `NewGuard`, operation tags, and a fork-gate helper.
- `transcript.go` — byte-exact CALL and ACCEPT transcripts (§7).
- `encode.go` — canonical proto3 encoding, size limits, and intrinsic gas (§8).
- `validate.go` — stateless validation (§5, §9.1) and the existing staking
  bounds that guarded actions must reuse.
- `slhdsa.go` — pure SLH-DSA-SHA2-192s verification with the fixed QSB context.
- `state.go` — canonical `BucketGuard` state serialization/deserialization and
  the state-manager/reader helper in the staking package.
- `bucketguard_test.go` — transcript, size/gas, validation, fork-gate, and
  signature round-trip tests.
- `state_test.go` — guard state round-trip, tombstone, and malformed-encoding
  tests.

## Integration checklist (not done here)

1. Add protobuf messages and `ActionCore` oneof fields in `iotex-proto`.
2. Add action types, deserialization, and envelope handling in `action/`.
3. Add `FeatureCtx` gates and a new hardfork height. Do not reuse an existing
   mainnet height.
4. Add `BucketGuard` state storage and a migration under a new namespace.
5. Route `BindBucketGuard` and `GuardedBucketOp` through the staking protocol.
6. Audit every legacy path that can mutate a guarded bucket: `Unstake`,
   `WithdrawStake`, `TransferStake`, `ChangeCandidate`, `Restake`,
   `MigrateStake`, consignment, candidate activation/endorsement, contract
   staking, and delegate self-stake.
7. Emit receipts/logs with the IIP statuses and event layout.
8. Add pre-fork/post-fork tests, backend-failure tests, fuzzing, ACVP vectors,
   and native linux/amd64 full-action benchmarks.
9. Get a security audit and maintainer sign-off on the fork height and
   serialization.

## Corrections to the standalone MVP

The MVP was a simulator and deliberately simplified some staking rules. This
scaffold uses the production bounds:

- RESTAKE duration: `1..1050` days (`staking._stakeDurationLimit`), not the
  MVP's 30 days.
- Candidate name: 1..12 lowercase ASCII letters or digits
  (`action.IsValidCandidateName`), not the MVP's 1..40 printable characters.
- The CALL transcript intentionally does not commit to an amount. TRANSFER and
  WITHDRAW act on the bucket's execution-time amount, including deposits and
  IIP-59 compounding that arrived after signing.

## Security notes

- **Pre-bind compromise:** binding is authorized by the current ECDSA owner. An
  attacker who compromises that key before a guard is bound can install an
  attacker-chosen guard key. There is no retroactive protection.
- **ClearGuard is an opt-out:** a guard-signed TRANSFER with `clearGuard`
  returns the bucket to ECDSA control. A later legacy withdraw or migration
  deletes the tombstone. This is not a lost-key recovery path.
- **Signed ops are bearer authorizations:** anyone can submit a valid signed op
  before its deadline. There is no cancel path except changing state with
  another guarded op; wallets should treat signing as final.
- **Lost guard key:** the bucket is permanently locked unless the wallet seed is
  available for derivation.
- **No activation:** nothing in this package changes mainnet behavior.
