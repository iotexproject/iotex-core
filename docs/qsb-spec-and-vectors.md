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
| CALL signing context | `IoTeX-PQ-Bucket-Call-v1` (23 ASCII bytes) |
| ACCEPT signing context | `IoTeX-PQ-Bucket-Accept-v1` (25 ASCII bytes) |
| Chain-instance domain | `qsbDomain = SHA-256("IoTeX-PQ-Bucket-Domain-v1" \|\| uint32_be(chainID) \|\| genesisHash)` |
| Max BIND size | 17,000 bytes |
| Max op size | 17,000 bytes |
| Max op size with `newGuard` | 34,000 bytes |
| Intrinsic gas | `10,000 + 100*size + k*30,000` |
| `k` | 1 for BIND and ordinary ops; 2 when an op carries `newGuard` |
| Domain used in vectors | Mainnet: native chain ID 1 (`BlockchainCtx.ChainID`, **not** the EVM network ID 4689) and genesis hash `b3379837…cfe8b`; `qsbDomain = 49d87941a7e817ba842f0f3ec9252840a56d0d090ec50f1d6938fc8a82bed871` |

`genesisHash` is `block.GenesisHash()`, which equals `genesis.Genesis.Hash()`
(Blake2b-256 over a subset of the genesis fields). Wallets cannot recompute it
from public data; they read it from a node as the parent hash of block 1 (or
`eth_getBlockByNumber(0).hash`), or use a published value.

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
| `duration` | 10 | varint | RESTAKE only; days, bounded by the existing `Restake` validation (≤ 1050, 0 allowed) |
| `autoStake` | 11 | varint | RESTAKE only |
| `newGuard` | 12 | message | TRANSFER/ROTATE only |
| `clearGuard` | 13 | varint | TRANSFER only; mutually exclusive with `newGuard` |
| `sig` | 14 | bytes | 16,224-byte CALL signature |
| `sponsor` | 15 | bytes | Optional; empty (bearer) or the 20-byte address of the only allowed sender. Signed in CALL. Not yet in the action proto or enforced (IIP-65 §6) |

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
| CHANGE_CANDIDATE | 4 | `candidateName` (non-empty; format per `action.IsValidCandidateName`), `expectedCandidateID` (20 bytes) | all other op-specific fields |
| RESTAKE | 5 | none (`duration`, `autoStake` optional; bounds per existing `Restake` validation) | all other op-specific fields |
| ROTATE | 6 | `newGuard`, `clearGuard=false` | all other op-specific fields |

`newGuard` is valid only when `algID=0x12`, `publicKey` is 48 bytes, and
`acceptSig` is 16,224 bytes. An op with `newGuard` has a 34,000-byte limit and
pays two verifications; all other ops have a 17,000-byte limit and pay one.

Staking rules of the underlying operation (restake duration bound,
candidate-name format) are the existing staking checks, applied to guarded ops
exactly as to legacy actions; the guard layer does not restate them. This
keeps RESTAKE to 0 days available, as legacy `Restake` allows it.

## 4. Transcripts

Transcripts are not protobuf. They are fixed byte strings with big-endian
integers and `uint32_be` length prefixes. They carry no domain tag: each
message type has its own FIPS 205 signing context (§1), which FIPS 205
length-prefixes, so CALL and ACCEPT cannot be confused. Both begin with the
32-byte chain-instance domain `qsbDomain`.

### 4.1 CALL

Signed by the current guard key under `IoTeX-PQ-Bucket-Call-v1`.

```text
qsbDomain[32]
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
uint32_be(len(sponsor)) || sponsor
```

The CALL transcript intentionally does not commit to the bucket amount.
TRANSFER and WITHDRAW act on the execution-time amount, including deposits and
IIP-59 compounding that arrived after signing.

### 4.2 ACCEPT

Signed by the incoming key under `IoTeX-PQ-Bucket-Accept-v1`.

```text
qsbDomain[32]
uint64_be(bucketIndex)
uint64_be(newGeneration)
resultingOwner[20]
uint32_be(algID)
uint32_be(len(publicKey)) || publicKey
callHash[32]
```

For BIND, `callHash` is 32 zero bytes. For TRANSFER/ROTATE with `newGuard`, it
is `SHA-256(CALL)` of the operation (`bucketguard.CallHash`), so an ACCEPT is
bound to exactly one CALL. CALL covers the incoming key but not its ACCEPT
signature, so the wallet builds CALL, has the incoming key sign ACCEPT over its
hash, then signs CALL with the current key. `resultingOwner` is the owner after
the operation.

### 4.3 Transcript vectors

All vectors use the mainnet domain
`qsbDomain = 49d87941a7e817ba842f0f3ec9252840a56d0d090ec50f1d6938fc8a82bed871`
(chain ID 1, genesis hash
`b337983730981c2d50f114eed5da9dd20b83c8c5e130beefdb3001dc858cfe8b`) and owner
`0x11` repeated 20 times.

CALL inputs: bucket 1, generation 1, nonce 0, deadline 1010, op UNSTAKE, no
sponsor.

```text
49d87941a7e817ba842f0f3ec9252840a56d0d090ec50f1d6938fc8a82bed871000000000000000111111111111111111111111111111111111111110000000000000001000000000000000000000000000003f200000001000000000000000000000000000000000000000000000000000000
```

BIND ACCEPT inputs: bucket 1, new generation 1, `algID=0x12`, public key
`c3530472b05ab2c631028bf08d7fa740f87ccbe26c8290b286f8317dbc30787febe5366d8be38345ec93d0fca288459a`,
zero `callHash`.

```text
49d87941a7e817ba842f0f3ec9252840a56d0d090ec50f1d6938fc8a82bed8710000000000000001000000000000000111111111111111111111111111111111111111110000001200000030c3530472b05ab2c631028bf08d7fa740f87ccbe26c8290b286f8317dbc30787febe5366d8be38345ec93d0fca288459a0000000000000000000000000000000000000000000000000000000000000000
```

ROTATE inputs: bucket 1, generation 1, nonce 2, deadline 1010, incoming key
the public key above, sponsor `0x22` repeated 20 times.

| Value | Hex |
|---|---|
| `callHash = SHA-256(CALL)` | `bb4e4bdeb3b03dee7cc5e6822fa2b13a23ca4cdca1615087039f55cff185fec6` |
| SHA-256 of ACCEPT (new generation 2) | `93375a34350d3b8e0045ff9b6c9f440dc963d7b5afa4d0269f8c3316cc47a408` |

These vectors were computed independently of the Go code (Python, standard
library only) and are asserted by `TestTranscriptVectors` in
`action/protocol/staking/bucketguard/bucketguard_test.go`.

## 5. Canonical size and gas vectors

The objects below use a 48-byte zero public key and a 16,224-byte zero
signature. `sha256` is over the canonical bytes `C(m)`, not over the transcript
or the envelope.

| Action / variant | Size | Intrinsic gas | SHA-256 of `C(m)` |
|---|---:|---:|---|
| `BindBucketGuard` | 16,281 | 1,668,100 | `e39fec5a6a1739ff165ab0c0ffe59878fcec475778da8bdddd08f4e4aad09ce4` |
| UNSTAKE | 16,236 | 1,663,600 | `15f253feb5642b8b3976bd822a83a4d1ca22f9bbdd2eaa4ddc44c59b975f2b85` |
| WITHDRAW | 16,258 | 1,665,800 | `ed7811a696608574581c53db032d0ec1972375e83f73a2de64adadb122459856` |
| TRANSFER + `clearGuard` | 16,260 | 1,666,000 | `cb8bee995cd6ec0088914ce227831453f85291d0db683e04a0087adeac565c9e` |
| CHANGE_CANDIDATE | 16,270 | 1,667,000 | `34419e212ee93d25331bb8f108795e85a9902b0f74b4a0a9bd7d38688ec98c80` |
| RESTAKE | 16,240 | 1,664,000 | `27c100e25c9665f6d7dbf35461d8f17138ef36359e81e99e8cfaaf30f33a0679` |
| ROTATE | 32,518 | 3,321,800 | `4435a0e0f21a208232a0ef38f6a733d1207b1e731452ff40aadf19fca3379ec0` |
| TRANSFER + `newGuard` | 32,540 | 3,324,000 | `1d29df46e4678ad90b6a7578ca4a1caef6f62fccd8ee23824f72a35fe36834b3` |
| UNSTAKE + `sponsor` | 16,258 | 1,665,800 | `3a588d82cff23452522fe0c392a23a68d516dff90b7276f6b597c800347e8c71` |

Every field number is at most 15, so `len(C(m))` equals the IIP-65 §8
`QSBSizeV1` formula. `TestCanonicalVectors` enforces these values. Any encoding change must update the IIP, the vectors, and the tests in
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
| TRANSFER + `clearGuard` | Owner := `newOwner` | Tombstone: `algID=0`, empty key; `generation+1`; `nonce+1`; `boundHeight := height` |
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
`publicKey`, the clearing height as `boundHeight`, and retained counters. Counters never decrease or
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
data     = abi.encode(uint64 generation, uint64 nonce, address sponsor, address counterparty)
```

`data` is 128 bytes: four 32-byte words, each value left-padded. `generation` and `nonce` are the resulting values. For
WITHDRAW the record is deleted and `nonce` is the consumed value plus one.
`counterparty` is `newOwner` for TRANSFER, `recipient` for WITHDRAW, and 20 zero
address otherwise. Legacy per-handler logs are not emitted for guarded ops.
Transaction logs follow existing rules: gas-fee logs always, and one
`WITHDRAW_BUCKET` log from the bucket pool to `recipient` for WITHDRAW.

`topic[0]` is Keccak-256 of the ASCII string
`BucketGuard(uint64,uint64,uint64,uint64,address,address)`, so Ethereum tooling
decodes the log with the event ABI.

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
prk = HKDF-Extract(salt="IoTeX-PQ-Bucket-KDF-v1", IKM=bip39_seed)   // HMAC-SHA-512
info = "slh-dsa-sha2-192s" || uint32_be(vaultIndex) || uint32_be(keyIndex)
okm = HKDF-Expand(prk, info, L=72)
(SK.seed, SK.prf, PK.seed) = okm[0:24], okm[24:48], okm[48:72]
(sk, pk) = slh_keygen_internal(SK.seed, SK.prf, PK.seed)            // FIPS 205 Alg. 18
```

`bip39_seed` is the 64-byte BIP-39 seed (PBKDF2 output, NFKD-normalized
mnemonic and passphrase), never a BIP-32 node or a secp256k1 key.
`vaultIndex` is independent of BIP-44 account numbering and is 0 by default.
`keyIndex` has no upper bound; reuse is safe because SLH-DSA is stateless.
Recovery scans key indices `0..1023` per vault index by default
(`signer.DefaultQSBScanWindow`); the window is a default, not a limit.

Test seed:

```text
000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f
```

| Vault | Key index | OKM (72 bytes) | Public key |
|---:|---:|---|---|
| 0 | 0 | `c5846824b81db2e3ac1bccb6044310f35a47e5b697aab627a8aaf2e88d27c9da3a819650779aff30b653ac27236d5d4a19cefae20e485beb73cd3baf3c84b9e452018c51850faac1` | `19cefae20e485beb73cd3baf3c84b9e452018c51850faac1e419836bc96f267a5913d095b8d796eecf68f4ca43eed03f` |
| 3 | 2 | `f89578d51bdd62faab28ec2327275708165db4edadb615931dd6386c547ae400be65661f1f6f8941c2912dab72afce534384023dc938bad1b2397fe831a94bd6215e8cfad1c7a207` | `4384023dc938bad1b2397fe831a94bd6215e8cfad1c7a2074147d24e68cc0c0f9f13216c66e524af9d77972dcf6e87c5` |
| 0 | 1024 | `8a480e5bad2e75dea2ca02d216ba11478209bce0cd80c642e0fcc45f72eaa83df5695282c9148417146a48fca9652b47953096e87852f078eb9e759930ffe901aa1d956c6275d3d5` | `953096e87852f078eb9e759930ffe901aa1d956c6275d3d5feaf66e629287fb976157c6263dbd4350a4263f9acc3311b` |
| 0 | 4294967295 | `c91f5899055d18790e5ea69bd6732a5cf83b57ef8f6c1cd899d82d23717b6a27e38306f3779641221f996c963294bc73758637538cc8ae78652d57d9b200c86bafb63c3735574f37` | `758637538cc8ae78652d57d9b200c86bafb63c3735574f3713ab8d545e03293a93d9a9af340663f38d05211234002e31` |

Mnemonic vectors (vault 0, key index 0), mnemonic `abandon` ×23 + `art`:

| Passphrase | BIP-39 seed | OKM (72 bytes) | Public key |
|---|---|---|---|
| (none) | `408b285c123836004f4b8842c89324c1f01382450c0d439af345ba7fc49acf705489c6fc77dbd4e3dc1dd8cc6bc9f043db8ada1e243c4a0eafb290d399480840` | `fbb67bf81b6cfbe89b8d03f293068b8b6f7b9c4036bb3a1f832ba361377f955e9a0618b489ac50b900ce6417ec10a29eb06841df29502ec67c3160c530f7b7d26e0a76838cced6e7` | `b06841df29502ec67c3160c530f7b7d26e0a76838cced6e77b545a16fb3d435ce5bac69926bc389a83d32a7c6f2876ee` |
| `TREZOR` | `bda85446c68413707090a52022edd26a1c9462295029f2e60cd7c4f2bbd3097170af7a4d73245cafa9c3cca8d561a7c3de6f5d4a10be8ed2a5e608d68f92fcc8` | `7831b06300574c17e6efff46d5a9ba2a8998dc95e89e816cde838f2d7e524b2c5b79cf5deb880a964da9a232bae15cf634fbd17dc298c306ca1d49a7cc9624a201bb7e132434b190` | `34fbd17dc298c306ca1d49a7cc9624a201bb7e132434b190201d121d4fbd886a422c45c509e0d143edb350a1e1b1f417` |

The BIP-39 seeds are the standard BIP-39 test vectors. OKM was computed
independently of the Go code (Python, standard library only); the last 24
bytes of OKM are PK.seed, the first half of each public key. PK.root comes
from FIPS 205 key generation (CIRCL). `TestDeriveQSBKeyVectors` asserts the
public keys.

Recovery vector: seed above, vault 3, key indices `0..4`, target public key
`4384023d…6e87c5`; the recovered index is `2` (`TestRecoverQSBKeyIndex`).

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
