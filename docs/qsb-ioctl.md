# Quantum-safe staking buckets with ioctl

Status: **draft, non-activated**. `ioctl stake2 guard` is the reference wallet
CLI for IIP-65. It works against a network that has activated the feature
(today: local and nightly networks).

A *guard* is a post-quantum key (SLH-DSA-SHA2-192s) bound to a native staking
bucket. Once a bucket is guarded, the owner's ECDSA key alone can no longer
unstake, withdraw, transfer, restake or change the candidate of the bucket.
Each of these is a *guarded op*: a CALL signed by the guard key, which any
account (the *sponsor*, `-s`) submits and pays for. Deposits still work
without the guard.

The code is in `ioctl/qsb` (vault, request files, signing; no network) and
`ioctl/cmd/action/stake2guard*.go` (commands).

## 1. The guard vault

Guard keys are derived from a dedicated BIP-39 mnemonic, the *guard vault*
(IIP §12.2, wallet profile `QSB-KDF-v1`, see
[`qsb-spec-and-vectors.md`](./qsb-spec-and-vectors.md) §9). Do not reuse the
mnemonic of an account: the guard exists to survive a compromise of the ECDSA
keys.

```sh
ioctl stake2 guard vault create [--words 24] [--passphrase]
ioctl stake2 guard vault import [--passphrase]       # restore from a mnemonic
ioctl stake2 guard vault export                      # print the mnemonic
ioctl stake2 guard vault delete
ioctl stake2 guard vault key [--key-index N] [--count K]
ioctl stake2 guard vault find (PUBKEY | --bucket N) [--scan-window 1024]
```

- The vault is the mnemonic encrypted with scrypt and the keystore v3 cipher
  and MAC, in `--vault-file` (default `<wallet>/guardvault`, mode 0600).
- `--passphrase` adds a BIP-39 passphrase. It is never stored and is asked for
  with every use; a check value in the vault detects a wrong one.
- `--vault-index` selects the QSB-KDF vault index (default 0). Key indices
  have no upper bound; recovery scans `0..1023` by default, so `bind` and new
  keys beyond 1023 need `--allow-high-index`.
- Back the mnemonic (and passphrase) up offline. **A guarded bucket whose
  guard key is lost is locked for good.**
- `--vault-password` exists for scripts; otherwise passwords are prompted
  (piped stdin works).

Deriving one key takes about 35 ms; scans run on all cores (64 indices take
about 0.3 s on a 10-core laptop, 1024 about 4 s).

## 2. Online, one step

The commands read the bucket and its guard at the tip, print a summary, unlock
the vault, sign, check the request against the chain, and send it with the
usual write flags (`-s -n -l -p -P -y -w`). `--endpoint` and `--insecure`
apply as for every `stake2` command.

```sh
ioctl stake2 guard bind BUCKET --key-index 0 -s OWNER          # once, by the owner
ioctl stake2 guard show BUCKET [--height H]

ioctl stake2 guard unstake BUCKET -s SPONSOR
ioctl stake2 guard withdraw BUCKET --recipient ADDR -s SPONSOR
ioctl stake2 guard change-candidate BUCKET NAME -s SPONSOR
ioctl stake2 guard restake BUCKET DAYS [--auto-stake] -s SPONSOR
ioctl stake2 guard rotate BUCKET --new-key-index 1 -s SPONSOR
ioctl stake2 guard transfer BUCKET --new-owner ADDR --clear-guard -s SPONSOR
ioctl stake2 guard transfer BUCKET --new-owner ADDR --new-key-index 0 \
    --new-vault-file OTHER_VAULT -s SPONSOR
```

- `bind` must be sent by the owner and asks for an explicit confirmation.
- The current guard key is found by scanning `--scan-window` (64) key indices
  unless `--key-index` is given.
- The deadline is 100 blocks after the tip by default (`--deadline-blocks`,
  or an absolute `--deadline`); deadlines over 17,280 blocks (≈ 1 day at
  2.5 s per block) get a warning.
- Without `-l`, the gas limit is the action's intrinsic gas plus 10,000.
- Before sending, the domain, owner, guard key, generation, nonce, deadline
  and, for `change-candidate`, the candidate ID are compared with the chain,
  so a stale op is refused instead of burning gas on a failed receipt.
- With `-w`, the receipt and the new guard state are printed. Receipt
  statuses 216–221 are the IIP-65 failures (216 bucket is guarded, 217 not
  guarded, 218 generation/nonce/deadline mismatch, 219 bad signature, 220
  cannot be guarded, 221 candidate mismatch).

## 3. Offline, several steps

Every op command (and `bind`) takes `--out FILE`: it stops after preparing
the request and needs no vault. The deadline then defaults to 720 blocks
(≈ 30 minutes).

```sh
# online machine
ioctl stake2 guard unstake BUCKET --out op.json
# vault device (no network needed)
ioctl stake2 guard inspect op.json
ioctl stake2 guard sign op.json [--key-index N]
# any online machine, any account
ioctl stake2 guard submit op.json -s SPONSOR -w
```

`sign` verifies the signatures already in the file, shows the summary, asks
for confirmation (`-y` skips it), signs, verifies the result and rewrites the
file (or writes `--out`). For `bind` and `rotate` prepared without a key, pass
the new key's index to `sign` (`--key-index` for bind, `--new-key-index` for
rotate).

**A complete request is a bearer authorization**: anyone holding it can
submit it until its deadline. Transfer it like a signed cheque and keep the
deadline short. Request files carry no sponsor.

### Two-party transfer

The new owner keeps their own vault and accepts the incoming guard key:

```sh
# new owner: give the current owner a public key
ioctl stake2 guard vault key --key-index 0
# current owner, online
ioctl stake2 guard transfer BUCKET --new-owner NEW_OWNER \
    --new-guard-pubkey PUBKEY --out transfer.json
# new owner's vault device
ioctl stake2 guard accept transfer.json
# current owner's vault device
ioctl stake2 guard sign transfer.json
# anyone
ioctl stake2 guard submit transfer.json -s SPONSOR
```

`accept` and `sign` can run in either order; neither signature covers the
other. Accepting first lets the new owner check the transfer before the
current owner commits to it.

### Request file v1

JSON, read strictly (unknown fields are rejected). uint64 values are decimal
strings, bytes are lowercase hex, addresses are `io1…`.

```json
{
  "type": "iotex-qsb-request", "version": 1, "kind": "op",
  "network": {"chainID": 2, "genesisHash": "…", "domain": "…"},
  "bucket": {"index": "7", "owner": "io1…", "candidate": "io1…", "amount": "…", "readHeight": "1000"},
  "guard": {"algID": 18, "publicKey": "…", "generation": "1", "nonce": "0"},
  "op": {"op": "TRANSFER", "deadline": "1720", "newOwner": "io1…",
         "newGuard": {"algID": 18, "publicKey": "…", "acceptSig": "…"}, "sig": "…"},
  "callHash": "…"
}
```

- `domain` must equal `NewDomain(chainID, genesisHash)` and `callHash` the
  hash of the CALL transcript; both are checked on load. The summary names
  mainnet (chain 1) and testnet (chain 2) and shows any other domain as
  "unknown network".
- `bucket.owner` is signed; `candidate`, `amount` and `readHeight` are
  informational (ops do not commit to the amount).
- A `bind` request carries `"bind": {"algID", "publicKey", "acceptSig"}`
  instead of `op` and no `callHash`; `guard` holds the counters of the
  unguarded bucket.
- An empty `publicKey` in `newGuard` or `bind` means the key is not chosen
  yet.
- A TRANSFER with a new key is about 65 KB.

## 4. Out of scope

- Binding from a cold ECDSA owner key: `bind` is sent by the owner from
  ioctl's own keystore.
- A sponsor-restricted op: the action proto does not carry `sponsor` yet.
- QR transport of request files.
