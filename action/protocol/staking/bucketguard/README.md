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
- [`docs/qsb-ioctl.md`](../../../../docs/qsb-ioctl.md) (wallet CLI)

## What is here

| File | Contents |
|---|---|
| `types.go` | `Op`, `BindBucketGuard`, `GuardedBucketOp`, `NewGuard`, signing contexts, chain-instance `Domain` |
| `params.go` | `Params` and `ParamsV1()`: sizes, gas, accepted algorithms, versioned by fork |
| `errors.go` | Sentinel errors, one per IIP receipt status, plus `ErrMalformed` |
| `encode.go` | Canonical proto3 encoding C(m), `CanonicalSize`, `IntrinsicGas(p)` (§8) |
| `validate.go` | Stateless guard checks (§5, §9.1) |
| `transcript.go` | `EncodeCall` / `CallHash` / `EncodeAccept`, the byte-exact signing messages (§7) |
| `guard.go` | `Guard` record with canonical `Serialize` / `Deserialize` (§4) |
| `authorize.go` | `AuthorizeBind`, `CheckOp`, `CheckCandidate`, `VerifyOp`, `Next`, `RejectIfGuarded` |
| `web3.go` | ABI envelope: one method per op, `EthData` / `*FromEthData`, priced over the decoded value |

Signature verification and the algorithm registry live in `pkg/pqcrypto`;
wallet-side key derivation (`QSB-KDF-v1`) and signing in `pkg/pqcrypto/signer`.
`example_test.go` walks a bind and a guarded withdraw through the API.

## Integration status

Wired behind `FeatureCtx.EnableBucketGuard` (= `ToBeEnabled && ZanzibarGamma`):

| Item | Where | Status |
|---|---|---|
| Protobuf messages, `ActionCore` oneof 59/60, receipt statuses 216–221; Draft 4 `sponsor`/`anchorHeight`/`anchorHash` (15–17) | iotex-proto `qsb/actions` | draft; `go.mod` replaces iotex-proto locally — **must be released before merge** |
| Action types, envelope, web3 builder | `action/bucket_guard.go` | done |
| Fork gate | `action/protocol/context.go` | done (WIP height `ToBeEnabled`; name and height to be chosen by maintainers) |
| Guard record state (`_bucketGuard`), Erigon `Encode`/`Decode` and registration | `staking/bucket_guard.go`, `state/factory/erigonstore/registry.go` | done; Erigon store round trip and e2e incl. historical reads |
| Bind / guarded-op handlers, receipts, BucketGuard event with `guardKeyID` topic | `staking/bucket_guard.go` | done (Draft 4) |
| Draft 4 anchor (§6, §9.2 step 4), SHA-512 `callHash` (§7), sponsor binding (step 3, 203) | `bucketguard` (`CheckOp`, transcripts), `staking/bucket_guard.go` | done |
| Address rules (§6): non-zero 20-byte recipient/newOwner/sponsor; TRANSFER `newOwner` ≠ owner; `clearGuard` not to a contract or native protocol address | `bucketguard/validate.go`, `staking/bucket_guard.go` | done |
| `BUCKETS_BY_GUARD_KEY` node index (MAY in Draft 4) | — | not implemented; indexers use the event's `guardKeyID` topic |
| Legacy-path rejection (§10), tombstone deletion with the bucket | `staking/handlers.go`, `handler_*.go` | done |
| Verify before writing (§9.2): each shared bucket operation split into a check and an apply | `staking/bucket_ops.go` | done; legacy actions use it from the QSB fork, and keep the original `*BeforeQSB` helpers before it |
| Write layer (§10): bucket writes carry a `bucketAuth` (legacy, or a `Grant` from `VerifyOp`); self-stake and endorsement writes refuse guarded buckets | `staking/candidate_statemanager.go`, `bucket_guard.go` | done |
| Query API for the guard record | `ReadStakingData` `BUCKET_GUARD`, eth_call `bucketGuard(uint64)` | done |
| e2e | `e2etest/bucket_guard_test.go` | done |
| Wallet CLI: vault, bind, guarded ops, offline signing, anchor and `--sponsor` ([guide](../../../../docs/qsb-ioctl.md)) | `ioctl stake2 guard` (`ioctl/qsb`, `ioctl/cmd/action/stake2guard*.go`) | done (request file v2) |
| NIST ACVP vectors (keyGen, sigGen, sigVer for SLH-DSA-SHA2-192s) | `pkg/pqcrypto/testdata`, `acvp_test.go` | done |
| linux/amd64 benchmarks for `G_VERIFY` | `bucketguard/bench_test.go`, `e2etest/bucket_guard_perf_test.go` | done |
| Fuzzing of action decoding, audit | — | open |

## Staking rules are not duplicated

The guard layer checks only what belongs to the guard: sizes, deadline and
anchor, addresses, algorithms and which fields each op carries. The staking rules of the
underlying operation (restake duration bound, candidate-name format) are
applied by the staking protocol with its existing checks, so guarded and
legacy actions cannot drift apart. In particular RESTAKE accepts any duration
here, including 0, as legacy `Restake` does.

The CALL transcript intentionally does not commit to an amount. TRANSFER and
WITHDRAW act on the bucket's execution-time amount, including deposits and
IIP-59 compounding that arrived after signing.

## Security notes

- **Pre-bind compromise:** binding is authorized by the current ECDSA owner. An
  attacker who compromises that key before a guard is bound can install an
  attacker-chosen guard key. There is no retroactive protection.
- **ClearGuard is an opt-out:** a guard-signed TRANSFER with `clearGuard`
  returns the bucket to ECDSA control. A later legacy withdraw or migration
  deletes the tombstone. This is not a lost-key recovery path.
- **Unbound ops are bearer authorizations:** without a `sponsor`, anyone can
  submit a valid signed op between its anchor block and its deadline, at most
  `MAX_DEADLINE_WINDOW` blocks. A non-empty `sponsor` restricts submission to
  that sender. There is no cancel path except changing state with another
  guarded op; wallets should treat signing as final.
- **Lost guard key:** the bucket is permanently locked unless the wallet seed is
  available for derivation.
- **No activation:** nothing in this package changes mainnet behavior.

## Draft 5 engine and native adapter (§13)

`vault_types.go`, `vault_transcript.go`, `vault_engine.go`, `vault_store.go`,
and `vault_recovery.go` implement a separate, serial in-memory engine. This is
an executable specification; `vault_transition.go` hydrates one transition
for the native adapter described below. No fork height is selected here.
Draft 4 types and encodings remain intact.

`ParamsV2().MinVaultBalance` defaults to **100 IOTX (100 × 10^18 Rau)**.
`NewVaultEngine(domain, minVaultBalance)` accepts an explicit positive Rau
minimum; older engine-only tests use 100 Rau as a small fixture.
The 3,110,400-block unbind delay and 241,920-block cancel cooldown are fixed.

Class A implements all nine actions, liquid-account/pool accounting, race-free
creation PoP, immutable class namespaces, and permanent tombstones. `State`
exposes the exact `prefix || uint64_be(index)` keys with canonical proto3
vault/guard values; meta key `{12}` contains big-endian `totalVaultCount`.
`StateEntries` returns sorted copies. Amount storage is minimal unsigned
big-endian; the signed withdrawal amount is fixed-width uint256.

`VaultEnv` / `OpEnv` represent already authenticated ECDSA outer transactions
and consensus block data. Callers must verify outer signatures/account nonces;
creation additionally rejects reuse of a successful `(owner, txNonce)` PoP.
Private keys, signing randomness, and wallet code appear only in tests and
benchmarks. Every engine authorization uses the real `pkg/pqcrypto` FIPS 205
SLH-DSA-SHA2-192s verifier.

Class B fixtures enter through `SeedStakingBucket`; `BindGuardBV2` uses the
new `ACCEPT_B_V2` layout and preserves tombstone counters/cooldown on rebind.
Recovery implements all five cancel branches. `StakingDeposit` returns
`(false, nil)` for a compound skipped during pending recovery, so its caller
keeps the reward claimable. `StakingGuardedOp` models the Draft 5 pending and
counter write-sets, requires a read-only native-precondition callback, and
never pays out Class B funds. Native PoS maturity, reward batches, voting and
withdrawal waiting periods remain the production staking protocol's job.

Run the existing and new scenarios with:

```sh
go test -v ./pkg/pqcrypto/... ./action/protocol/staking/bucketguard/...
go test -run='^$' -bench='BenchmarkVault|BenchmarkGuardUnbindCancel' -benchmem ./action/protocol/staking/bucketguard/...
```

The four handler benchmarks exclude wallet keygen/signing and independent-state
reset from timing. Each measured call performs its real signature verification
(two for guard rotation) and applies state. `BenchmarkVaultSign` separately
measures hedged offline signing. No verification is mocked or cached.

The native Draft 5 adapter lives in `staking/vault_bucket.go`. It uses
`accountutil.LoadAccount`/`StoreAccount` and a StateManager snapshot around
single-record transitions. Class A state uses tags 10–12; Class B recovery
metadata uses tag 13 and mirrors the immutable Draft 4 tag-9 guard for the
native bucket write firewall. The schemas at tag 9 remain unchanged.

`action.VaultAction` transports typed ABI calls through the existing
GuardedBucketOp protobuf using the new envelope tag `0x6505` and ABI bytes in
its signature field. The old operation range remains unchanged; this carrier
was invalid under Draft 4, and existing Execution messages keep their historical
interpretation. The Web3 builder decodes the same calldata and rejects nonzero ETH
value. `vaultBucket(uint64)` and `stakingGuardV2(uint64)` getters expose ABI
results through eth_call and ReadState. The latter accepts the getter name as
its method and one ABI-packed uint64 argument, preserving existing protobuf
method numbers. Every adapter entry and getter checks EnableBucketGuard.

`AddDepositForCompound` checks `PendingGuardUnbind` and returns
`ErrCompoundSkippedPendingUnbind` before any write for a pending Class B bucket.
Reward routing explicitly handles that sentinel by crediting the reward
destination. Backend read errors propagate and never select a payout route.
Native bucket deletion removes both the Draft 4 guard and Class B V2 recovery
metadata; Class A tombstones remain permanent.
`vaultRotateOwner` rejects accounts with contract code using on-chain account
state, while an account that does not exist is accepted as an EOA.


`VaultTransition(string,uint64,uint32,uint64,uint64,uint64)` is published in the
native contract ABI and emitted through AddEvent. Failed Draft 5 operations
include the deterministic reason in the receipt's execution revert message;
backend errors abort execution and revert the complete StateManager snapshot.

`e2etest/vault_bucket_test.go` starts the node's real chain service without RPC
or P2P listeners and submits EIP-155 TX_CONTAINER transactions. It checks both
bucket classes, pool/recipient balances, query results, failed-action gas,
rotation, revocation, cancellation and the transaction pool's admission gas
accounting. Expiry is tested at exact heights with real working sets in
`staking/vault_bucket_atomic_test.go`; this test does not mine 3,110,400 blocks.
