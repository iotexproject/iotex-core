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

| File | Contents |
|---|---|
| `types.go` | `Op`, `BindBucketGuard`, `GuardedBucketOp`, `NewGuard`, signing context and domains |
| `params.go` | `Params` and `ParamsV1()`: sizes, gas, accepted algorithms, versioned by fork |
| `errors.go` | Sentinel errors, one per IIP receipt status, plus `ErrMalformed` |
| `encode.go` | Canonical proto3 encoding C(m), `CanonicalSize`, `IntrinsicGas(p)` (§8) |
| `validate.go` | Stateless guard checks (§5, §9.1) |
| `transcript.go` | `EncodeCall` / `EncodeAccept`, the byte-exact signing messages (§7) |
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
| Protobuf messages, `ActionCore` oneof 59/60, receipt statuses 216–221 | iotex-proto `qsb/actions` | draft; `go.mod` replaces iotex-proto locally — **must be released before merge** |
| Action types, envelope, web3 builder | `action/bucket_guard.go` | done |
| Fork gate | `action/protocol/context.go` | done (WIP height `ToBeEnabled`; name and height to be chosen by maintainers) |
| Guard record state (`_bucketGuard`), Erigon `Encode`/`Decode` | `staking/bucket_guard.go` | done; e2e on the Erigon backend incl. historical reads |
| Bind / guarded-op handlers, receipts, BucketGuard event | `staking/bucket_guard.go` | done |
| Legacy-path rejection (§10), tombstone deletion with the bucket | `staking/handlers.go`, `handler_*.go` | done |
| Shared bucket effects for legacy and guarded ops | `staking/handlers.go` | done (moved verbatim) |
| Query API for the guard record | `ReadStakingData` `BUCKET_GUARD`, eth_call `bucketGuard(uint64)` | done |
| e2e | `e2etest/bucket_guard_test.go` | done |
| Fuzzing of action decoding, ACVP, linux/amd64 benchmarks, audit | — | open |

## Staking rules are not duplicated

The guard layer checks only what belongs to the guard: sizes, deadline,
algorithms and which fields each op carries. The staking rules of the
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
- **Signed ops are bearer authorizations:** anyone can submit a valid signed op
  before its deadline. There is no cancel path except changing state with
  another guarded op; wallets should treat signing as final.
- **Lost guard key:** the bucket is permanently locked unless the wallet seed is
  available for derivation.
- **No activation:** nothing in this package changes mainnet behavior.
