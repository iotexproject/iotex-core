# QSB production readiness

Status: **draft, non-activated**. Companion to
[`qsb-consensus-draft.md`](./qsb-consensus-draft.md),
[`qsb-spec-and-vectors.md`](./qsb-spec-and-vectors.md),
[`qsb-legacy-path-audit.md`](./qsb-legacy-path-audit.md),
[`qsb-proto-draft.proto`](./qsb-proto-draft.proto),
[`qsb-action-integration.md`](./qsb-action-integration.md), and the scaffold at
`action/protocol/staking/bucketguard`.

This is a gate-based plan, not a schedule. No hardfork height is selected here;
maintainers must approve the height, protobuf changes, and activation.

## Gate 0 — spec and conformance vectors

- [ ] IIP is final enough to implement: action field numbers, receipt statuses,
      event layout, transcript encoding, size/gas formula.
- [ ] Exact vectors for bind and every op variant, including failures.
- [ ] Decision on `G_VERIFY` after native linux/amd64 full-action benchmarks,
      with and without SHA-NI. Do not calibrate on emulated builds.
- [x] Confirm bucket indices are never reused after deletion. `putBucket` uses
      the monotonic `totalBucketCount`; deletion does not decrement it. See
      [`qsb-spec-and-vectors.md`](./qsb-spec-and-vectors.md) §8.
- [ ] Confirm wallet-profile details: `QSB-KDF-v1`, key index range, recovery
      vectors, and cross-wallet interoperability.
- [x] Document the residual ECDSA exposure: delegate operator keys, delegate
      self-stake, contract staking, liquid balances, IIP-59 rewards paid to
      addresses, candidate operator identity. See
      [`qsb-spec-and-vectors.md`](./qsb-spec-and-vectors.md) §10.

## Gate 1 — reference implementation behind a disabled gate

- [ ] `iotex-proto`: messages, `ActionCore` oneof fields, receipt statuses;
      regenerate Go bindings and update the dependency.
- [ ] `action/`: action types, envelope/deserializer support, intrinsic gas,
      size limits, stateless validation.
- [ ] `action/protocol/staking`: dispatch, guard state, handlers, receipt/log
      emission, snapshot/revert, deterministic-vs-fatal error classification.
- [ ] `state/`: new namespace, append-only schema, migration, Erigon backend,
      state reader/view. Partial: `_bucketGuard` record serialization and a
      state manager/reader helper exist; no action writes them and Erigon/
      migration wiring is pending.
- [ ] `FeatureCtx` + genesis: new hardfork gate, default disabled. Partial:
      `QuantumSafeBucketsBlockHeight` defaults to `math.MaxUint64` and
      `EnableBucketGuard` is covered at `fork - 1` / `fork`; no QSB action is
      wired to the gate yet.
- [ ] Audit every legacy path that can mutate a guarded bucket: `Unstake`,
      `WithdrawStake`, `TransferStake`, `ChangeCandidate`, `Restake`,
      `MigrateStake`, consignment, candidate activation/endorsement, contract
      staking, delegate self-stake. Deposit remains allowed. Partial: guard
      checks are wired for every native legacy path, guarded-path and
      deposit-allowed tests pass; contract staking, delegate self-stake, and
      the fork/backend/reorg matrix remain.
- [ ] Unit, integration, replay, reorg, backend-failure, and cross-backend tests.
- [ ] Fuzzing for canonical decoding and transcript parsing.
- [ ] ACVP vectors for the SLH-DSA primitive; protocol transcript and KDF
      vectors published with the implementation.

## Gate 2 — testnet activation and ecosystem readiness

- [ ] Testnet hardfork activates the feature; mainnet remains disabled.
- [ ] Soak test for multiple weeks with real wallets, indexers, explorers, and
      staking dashboards.
- [ ] Wallet support: SLH-DSA keygen/sign, seed-derived recovery, guard nonce
      tracking, deadline UX, signed-op warnings, multi-device behavior.
- [ ] Exchange/custodian/staking-provider support for binding and guarded ops.
- [ ] Recovery tooling and support documentation; 16 KB signatures and offline
      signing workflows tested.
- [ ] Adversarial load: invalid signatures, 16 KB/32 KB spam, competing sponsors,
      mempool pressure, block-gas exhaustion, RPC body limits, p2p propagation.
- [ ] Reorg, restart, snapshot, migration, and downgrade/upgrade tests.
- [ ] Monitoring and alerting for guard binds, failed guarded ops, verification
      latency, block gas, mempool rejection, and recovery failures.

## Gate 3 — security and supply chain

- [ ] External audit of the consensus-critical crypto, transcripts, encoding,
      state transitions, and failure classification.
- [ ] Bug bounty before mainnet activation.
- [ ] Pinned CIRCL version, reproducible builds, dependency and supply-chain
      review.
- [ ] Side-channel and fault-injection review of the signing path.
- [ ] Incident-response plan, including a rollback/disable plan that does not
      create an ECDSA escape hatch.

## Gate 4 — mainnet activation

- [ ] Maintainer sign-off on hardfork name, height, protobuf, and activation.
- [ ] Validator upgrade coordination and monitoring; old/new node compatibility
      tested on a mainnet replay.
- [ ] Binding campaign and user education: bind before compromise, clearGuard is
      an opt-out, lost guard key means a locked bucket, signed ops are final.
- [ ] Gas/mempool policy for 16 KB/32 KB actions; fee market and rate limits.
- [ ] Mainnet canary/monitoring window; rollback criteria defined in advance.
- [ ] Governance communication and ecosystem coordination completed.

## Non-negotiables

- No mainnet activation without native linux/amd64 calibration and an audit.
- No ECDSA escape hatch or emergency recovery mode.
- No activation that leaves most buckets unbound and assumes opt-in is enough.
- No legacy path left unaudited; one missed handler is a bypass.
- No emulated build used for gas calibration or performance claims.
- No tombstone deletion without confirming bucket-index non-reuse.
- No wallet UX that implies a signed guarded op can be cancelled.

## Open decisions

- Hardfork name and height.
- Protobuf field and oneof numbering.
- `G_VERIFY` value after native linux/amd64 measurements.
- Minimum validator hardware baseline (SHA-NI or equivalent).
- Whether to support category 5 (SLH-DSA-SHA2-256s) later; two 256s signatures do
  not fit the current 34 KB action limit.
- Scope of a future batch-bind action for large bucket holders.
