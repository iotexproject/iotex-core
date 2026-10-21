# QSB spec and conformance vectors

Status: **draft, non-activated**. This document freezes the consensus surface
that the `bucketguard` scaffold implements. It does not select a hardfork
height, register an action, or change mainnet behavior. Protobuf field numbers
and receipt statuses remain proposals until `iotex-proto` review and maintainer
sign-off.

Companion documents:

- [`qsb-consensus-draft.md`](./qsb-consensus-draft.md) — integration sequence.
- [`qsb-production-readiness.md`](./qsb-production-readiness.md) — gates.
- [`qsb-legacy-path-audit.md`](./qsb-legacy-path-audit.md) — legacy bypass audit.
- [`qsb-proto-draft.proto`](./qsb-proto-draft.proto) — draft protobuf.
- [`qsb-action-integration.md`](./qsb-action-integration.md) — action/state wiring.

## 1. Consensus constants

| Item | Value |
|---|---|
| Algorithm | SLH-DSA-SHA2-192s (FIPS 205) |
| `algID` | `0x12` |
| Public key | 48 bytes |
| Signature | 16,224 bytes |
| Signing context | `IOTX-PQ-BUCKET-v1` |
| CALL domain | `IOTX-QSB-CALL` |
| ACCEPT domain | `IOTX-QSB-ACCEPT` |
| Max BIND size | 17,000 bytes |
| Max op size | 17,000 bytes |
| Max op size with `newGuard` | 34,000 bytes |
| Intrinsic gas | `10,000 + 16*size + k*30,000` |
| `k` | 1 for BIND and ordinary ops; 2 when an op carries `newGuard` |
| Native chain ID used in vectors | 4689 |

`G_VERIFY = 30,000` is the current draft value. It is normative only after
native linux/amd64 full-action calibration; emulated builds are not a
calibration source. No value in this document changes `G_VERIFY`.

## 2. Protobuf surface

Draft messages are in [`qsb-proto-draft.proto`](./qsb-proto-draft.proto).
Field numbers below are the proposed assignment; final numbers must be approved
in `iotex-proto`.

### 2.1 `BindBucketGuard`

| Field | Number | Wire type | Meaning |
|---|---:|---|---|
| `bucketIndex` | 1 | varint | Native bucket index |
| `algID` | 2 | varint | Must be `0x12` |
| `publicKey` | 3 | bytes | 48-byte SLH-DSA public key |
| `acceptSig` | 4 | bytes | 16,224-byte ACCEPT signature |

### 2.2 `NewGuard`

| Field | Number | Wire type | Meaning |
|---|---:|---|---|
| `algID` | 1 | varint | Must be `0x12` |
| `publicKey` | 2 | bytes | 48-byte SLH-DSA public key |
| `acceptSig` | 3 | bytes | 16,224-byte ACCEPT signature |

### 2.3 `GuardedBucketOp`

| Field | Number | Wire type | Meaning |
|---|---:|---|---|
| `bucketIndex` | 1 | varint | Native bucket index |
| `generation` | 2 | varint | Must equal stored generation |
| `nonce` | 3 | varint | Must equal stored nonce |
| `deadline` | 4 | varint | Absolute block height; must be > 0 |
| `op` | 5 | varint | `1..6`; unknown values invalid |
| `recipient` | 6 | bytes | WITHDRAW only; 20 bytes |
| `newOwner` | 7 | bytes | TRANSFER only; 20 bytes |
| `candidateName` | 8 | string | CHANGE_CANDIDATE only |
| `expectedCandidateID` | 9 | bytes | CHANGE_CANDIDATE only; 20 bytes |
| `duration` | 10 | varint | RESTAKE only; `1..1050` days |
| `autoStake` | 11 | varint | RESTAKE only |
| `newGuard` | 12 | message | TRANSFER/ROTATE only |
| `clearGuard` | 13 | varint | TRANSFER only; mutually exclusive with `newGuard` |
| `sig` | 14 | bytes | 16,224-byte CALL signature |

### 2.4 `ActionCore` oneof

Proposed additions, with no reuse or renumbering of existing fields:

```proto
BindBucketGuard bindBucketGuard = 59;
GuardedBucketOp guardedBucketOp = 60;
```

### 2.5 Receipt statuses

Proposed `iotextypes.ReceiptStatus` values:

| Name | Value |
|---|---:|
| `ErrBucketGuarded` | 216 |
| `ErrBucketNotGuarded` | 217 |
| `ErrGuardMismatch` | 218 |
| `ErrGuardSignature` | 219 |
| `ErrGuardScope` | 220 |
| `ErrCandidateMismatch` | 221 |

Existing statuses reused by the draft:

- `ErrInvalidBucketIndex = 202`
- `ErrUnauthorizedOperator = 203`

Final numeric assignment belongs to `iotex-proto`. A mismatch between the
document and generated bindings is a consensus bug.

## 3. Canonical encoding

`C(m)` is deterministic proto3 encoding:

- fields emitted in ascending field order;
- default-valued scalars and empty bytes/strings omitted;
- minimal varints and length prefixes;
- nested messages emitted only when present;
- no unknown fields;
- addresses are raw 20-byte native addresses, not `0x` strings.

`CanonicalSize()` is `len(C(m))`. Size limits and intrinsic gas use this
canonical size, not the RLP envelope size.

### 3.1 Per-op field rules

| Op | Tag | Required fields | Forbidden fields |
|---|---:|---|---|
| UNSTAKE | 1 | none | all op-specific fields |
| WITHDRAW | 2 | `recipient` (20 bytes) | all other op-specific fields |
| TRANSFER | 3 | `newOwner` (20 bytes), exactly one of `newGuard` / `clearGuard` | all other op-specific fields |
| CHANGE_CANDIDATE | 4 | `candidateName` (1..12 lowercase ASCII alnum), `expectedCandidateID` (20 bytes) | all other op-specific fields |
| RESTAKE | 5 | `duration` (1..1050) | all other op-specific fields |
| ROTATE | 6 | `newGuard`, `clearGuard=false` | all other op-specific fields |

`newGuard` is valid only when `algID=0x12`, `publicKey` is 48 bytes, and
`acceptSig` is 16,224 bytes. An op with `newGuard` has a 34,000-byte limit and
pays two verifications; all other ops have a 17,000-byte limit and pay one.

## 4. Transcripts

Transcripts are not protobuf. They are fixed byte strings with big-endian
integers and `uint32_be` length prefixes.

### 4.1 CALL

```text
"IOTX-QSB-CALL"
uint32_be(chainID)
uint64_be(bucketIndex)
owner[20]
uint64_be(generation)
uint64_be(nonce)
uint64_be(deadline)
uint32_be(op)
uint32_be(len(recipient)) || recipient
uint32_be(len(newOwner)) || newOwner
uint32_be(len(candidateName)) || candidateName
uint32_be(len(expectedCandidateID)) || expectedCandidateID
uint32_be(duration)
0x01 if autoStake else 0x00
0x01 if newGuard present else 0x00
if newGuard:
    uint32_be(newGuard.algID)
    uint32_be(len(newGuard.publicKey)) || newGuard.publicKey
0x01 if clearGuard else 0x00
```

The CALL transcript intentionally does not commit to the bucket amount.
TRANSFER and WITHDRAW act on the execution-time amount, including deposits and
IIP-59 compounding that arrived after signing.

### 4.2 ACCEPT

```text
"IOTX-QSB-ACCEPT"
uint32_be(chainID)
uint64_be(bucketIndex)
uint64_be(newGeneration)
resultingOwner[20]
uint32_be(algID)
uint32_be(len(publicKey)) || publicKey
uint64_be(deadline)
```

ACCEPT is signed by the incoming key. For BIND, `deadline` is 0. For
TRANSFER/ROTATE with `newGuard`, it is the op deadline. `resultingOwner` is the
owner after the operation.

### 4.3 Transcript vectors

Inputs for both vectors: chain ID 4689, bucket 1, generation 1, nonce 0,
deadline 1010, owner `0x11` repeated 20 times, op UNSTAKE.

CALL:

```text
494f54582d5153422d43414c4c00001251000000000000000111111111111111111111111111111111111111110000000000000001000000000000000000000000000003f2000000010000000000000000000000000000000000000000000000
```

ACCEPT inputs: chain ID 4689, bucket 1, new generation 1, resulting owner
`0x11` repeated 20 times, `algID=0x12`, public key
`c3530472b05ab2c631028bf08d7fa740f87ccbe26c8290b286f8317dbc30787febe5366d8be38345ec93d0fca288459a`,
deadline 0.

ACCEPT:

```text
494f54582d5153422d414343455054000012510000000000000001000000000000000111111111111111111111111111111111111111110000001200000030c3530472b05ab2c631028bf08d7fa740f87ccbe26c8290b286f8317dbc30787febe5366d8be38345ec93d0fca288459a0000000000000000
```

The Go tests in
`action/protocol/staking/bucketguard/bucketguard_test.go` assert both vectors.

## 5. Canonical size and gas vectors

The objects below use a 48-byte zero public key and a 16,224-byte zero
signature. `sha256` is over the canonical bytes `C(m)`, not over the transcript
or the envelope.

| Action / variant | Size | Intrinsic gas | SHA-256 of `C(m)` |
|---|---:|---:|---|
| `BindBucketGuard` | 16,281 | 300,496 | `e39fec5a6a1739ff165ab0c0ffe59878fcec475778da8bdddd08f4e4aad09ce4` |
| UNSTAKE | 16,236 | 299,776 | `15f253feb5642b8b3976bd822a83a4d1ca22f9bbdd2eaa4ddc44c59b975f2b85` |
| WITHDRAW | 16,258 | 300,128 | `ed7811a696608574581c53db032d0ec1972375e83f73a2de64adadb122459856` |
| TRANSFER + `clearGuard` | 16,260 | 300,160 | `cb8bee995cd6ec0088914ce227831453f85291d0db683e04a0087adeac565c9e` |
| CHANGE_CANDIDATE | 16,270 | 300,320 | `34419e212ee93d25331bb8f108795e85a9902b0f74b4a0a9bd7d38688ec98c80` |
| RESTAKE | 16,240 | 299,840 | `27c100e25c9665f6d7dbf35461d8f17138ef36359e81e99e8cfaaf30f33a0679` |
| ROTATE | 32,518 | 590,288 | `4435a0e0f21a208232a0ef38f6a733d1207b1e731452ff40aadf19fca3379ec0` |
| TRANSFER + `newGuard` | 32,540 | 590,640 | `1d29df46e4678ad90b6a7578ca4a1caef6f62fccd8ee23824f72a35fe36834b3` |

`TestCanonicalSizeAndGasVectors` and `TestCanonicalDigestVectors` enforce these
values. Any encoding change must update the IIP, the vectors, and the tests in
the same change.

## 6. Execution and guard transitions

Execution order:

1. Snapshot the state manager.
2. Load the native bucket; absent = `ErrInvalidBucketIndex`.
3. Require a guarded record; absent = `ErrBucketNotGuarded`.
4. Require `generation` and `nonce` equal to stored values, no overflow, and
   `blockHeight <= deadline`; otherwise `ErrGuardMismatch`.
5. Apply op-specific preconditions using existing handler statuses; for
   CHANGE_CANDIDATE require the resolved candidate ID to equal
   `expectedCandidateID` (`ErrCandidateMismatch`).
6. Verify CALL, then ACCEPT if present; failure = `ErrGuardSignature`.
7. Apply the bucket effect through existing staking helpers.
8. Update or delete the guard record and emit one guard event.
9. Settle gas and nonce.

| Op | Bucket effect | Guard after |
|---|---|---|
| UNSTAKE / CHANGE_CANDIDATE / RESTAKE | Existing handler effect | `nonce+1`; other fields unchanged |
| WITHDRAW | Bucket deleted; amount credited to `recipient` | Deleted; log reports consumed `nonce` and resulting `nonce+1` |
| TRANSFER + `newGuard` | Owner := `newOwner` | Key := new; `generation+1`; `nonce+1`; `boundHeight := height` |
| TRANSFER + `clearGuard` | Owner := `newOwner` | Tombstone: `algID=0`, empty key, `boundHeight=0`; generation retained; `nonce+1` |
| ROTATE | None | Key := new; `generation+1`; `nonce+1`; `boundHeight := height` |

`clearGuard` is guard-authorized. It is an opt-out, not an ECDSA recovery path.
A later legacy withdraw or migration deletes the tombstone.

### 6.1 Guard state record

The record lives in the staking namespace under the new key prefix
`_bucketGuard`, appended to the existing prefix enumeration:

```text
key   = _bucketGuard || uint64_be(bucketIndex)
value = protobuf BucketGuard {
          uint32 algID = 1;
          bytes  publicKey = 2;
          uint64 generation = 3;
          uint64 nonce = 4;
          uint64 boundHeight = 5;
        }
```

A bucket is guarded iff a record exists with a non-empty `publicKey`. An absent
record is `generation=0, nonce=0`. A tombstone has `algID=0`, an empty
`publicKey`, `boundHeight=0`, and retained counters. Counters never decrease or
reset across clear and rebind. The record must be written through the same state
manager and snapshot as the bucket, and must be deleted in the same snapshot
whenever the bucket is permanently deleted (guarded WITHDRAW, or a legacy
`WithdrawStake`/`MigrateStake` on a previously cleared bucket). On Erigon it must
be registered as a storage object.

## 7. Event layout

One event per successful BIND or guarded op. Emitter is the staking protocol
address. The event is produced by a single `AddEvent` call.

```text
topic[0] = 0x73a1cc2c5c34de8d095f5003eb096fa77f44357dd83dcea0225ca286942426ec
topic[1] = uint64_be(bucketIndex), left-padded to 32 bytes
topic[2] = uint64_be(op), left-padded to 32 bytes; op=0 is BIND, 1..6 are ops
data     = uint64_be(generation) || uint64_be(nonce) || sponsor[20] || counterparty[20]
```

`data` is 56 bytes. `generation` and `nonce` are the resulting values. For
WITHDRAW the record is deleted and `nonce` is the consumed value plus one.
`counterparty` is `newOwner` for TRANSFER, `recipient` for WITHDRAW, and 20 zero
bytes otherwise. Legacy per-handler logs are not emitted for guarded ops.
Transaction logs follow existing rules: gas-fee logs always, and one
`WITHDRAW_BUCKET` log from the bucket pool to `recipient` for WITHDRAW.

`topic[0]` is Keccak-256 of the ASCII string
`BucketGuard(uint64,uint64,uint64,uint64,address,address)`. The string is only
a derivation aid; the data is not ABI-encoded.

## 8. Bucket index non-reuse

Confirmed against `action/protocol/staking/candidate_statemanager.go` at commit
`49731d0`:

- `putBucket` reads `totalBucketCount`, uses `index := tc.Count()`, then
  increments the stored count.
- `delBucketAndIndex` deletes the bucket and voter/candidate index entries, but
  never decrements `totalBucketCount`.
- `TotalBucketKey` is a per-chain monotonic counter.

Therefore a deleted bucket index is not reused. Tombstone deletion plus rebind
cannot create a replay surface at the same index. The integration must preserve
this invariant: do not reset, decrement, or reuse `totalBucketCount`.

## 9. Wallet profile vectors (`QSB-KDF-v1`)

This section is wallet interoperability, not consensus. Consensus verifies
possession via ACCEPT and cannot verify derivation.

```text
prk = HKDF-Extract(salt="IOTX-QSB-KDF-v1", IKM=bip39_seed)          // HMAC-SHA-512
info = "slh-dsa-sha2-192s" || uint32_be(account) || uint32_be(keyIndex)
okm = HKDF-Expand(prk, info, L=72)
(SK.seed, SK.prf, PK.seed) = okm[0:24], okm[24:48], okm[48:72]
(sk, pk) = slh_keygen_internal(SK.seed, SK.prf, PK.seed)            // FIPS 205 Alg. 18
```

`account` is the BIP-44 account of the owner after installation. `keyIndex`
ranges over `0..1023`; reuse is safe because SLH-DSA is stateless. Recovery
derives all indices for each account and matches on-chain public keys.

Test seed:

```text
000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f
```

| Account | Key index | OKM (72 bytes) | Public key |
|---:|---:|---|---|
| 0 | 0 | `48baa64a719fc565fa6ae07421d59d8ec99beb663979aa857add00290a7039289b94ba311f93c97d1cbff9b68d97f700c3530472b05ab2c631028bf08d7fa740f87ccbe26c8290b2` | `c3530472b05ab2c631028bf08d7fa740f87ccbe26c8290b286f8317dbc30787febe5366d8be38345ec93d0fca288459a` |
| 3 | 2 | `d952c85607cfc763e465c3c91c3c47df7efc74b9e89e928749c620203d1fef5ea2bc0db0e2b68c044c2521f3f020b5c690d6316353ed3740537e3697662900b4f5e21aafeef9e670` | `90d6316353ed3740537e3697662900b4f5e21aafeef9e6706df36603691ed44ae7aaf5d06edf82b68587becd64e17a9c` |

Recovery vector: seed above, `account=3`, `maxIndex=4`, target public key
`90d631...e17a9c`; the recovered index is `2`. This vector is covered by the
MVP's `TestRecoverGuardKey` and should be ported to the wallet profile tests.

## 10. Residual ECDSA exposure

QSB protects native bucket principal against ECDSA compromise after a guard is
bound. It does not eliminate every ECDSA role:

- **Pre-bind compromise:** binding is authorized by the current ECDSA owner. An
  attacker who compromises the key before binding can install an attacker-chosen
  guard. There is no retroactive protection.
- **Deposits and IIP-59 compounding:** `DepositToStake` and
  `AddDepositForCompound` remain allowed without a guard signature and change
  the execution-time amount. Guarded ops intentionally do not commit to amount.
- **Candidate identity and operator keys:** candidate ownership, operator
  updates, BLS keys, and delegate metadata remain ECDSA-controlled.
- **Delegate self-stake:** self-stake buckets cannot be bound; they remain
  ECDSA-controlled. Candidate deactivation/activation/endorsement paths must
  enforce that a guarded bucket is never a self-stake bucket.
- **Contract staking and liquid staking:** contract/NFT buckets are outside QSB.
  `MigrateStake` is the native-to-contract path and must be blocked for guarded
  buckets; contract buckets have no QSB guard.
- **Rewards:** rewards paid to addresses, reward opt-in, and off-chain reward
  distribution are outside the guard.
- **`clearGuard`:** a guard-signed TRANSFER can deliberately return the bucket to
  ECDSA control. After that, legacy ECDSA paths work again, and a later withdraw
  or migration deletes the tombstone. This is an opt-out, not recovery.
- **Lost guard key:** without the wallet seed, the bucket is permanently locked.
  There is no ECDSA escape hatch or emergency mode.

## 11. Open items

- Native linux/amd64 full-action calibration, with and without SHA-NI; then
  confirm or amend `G_VERIFY`.
- Full SLH-DSA signature vectors and FIPS 205 ACVP vectors, beyond the digest
  vectors above.
- Cross-wallet KDF/recovery interoperability tests.
- Final protobuf field/oneof numbering and receipt status values.
- Hardfork name and height, selected by maintainers.
- External audit before any activation.
