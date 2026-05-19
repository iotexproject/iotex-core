# CLAUDE.md — blockchain

Module-specific guidance for the `blockchain/` package. Read
[/AGENTS.md](../AGENTS.md) first for repo-wide rules.

## What this package does

The core block production / validation / persistence engine. Subpackages:

| Path | Purpose |
|---|---|
| `blockchain/` | Main `Blockchain` interface, block lifecycle, subscriptions |
| `blockchain/block/` | Block, Header, Body, Footer; serialization; validators |
| `blockchain/blockdao/` | Block DAO interface + receipt/log indexers |
| `blockchain/filedao/` | File-based persistence (legacy v1, v2, v2 manager) |
| `blockchain/genesis/` | Genesis config, hardfork heights |

Block lifecycle: **Propose → Validate → Commit → Finalize**.

---

## Module red lines

### Block fields are immutable after construction

- `Header.prevBlockHash`, `txRoot`, `receiptRoot`, `deltaStateDigest`,
  `baseFee`, `blobGasUsed`, `excessBlobGas`, and `logsBloom` are computed
  once and never mutated. Mutating after build invalidates the signature.
- The signature is over **`HashHeaderCore()`** (block/header.go:207–226),
  which excludes the producer pubkey and signature itself — preventing
  circular dependency. Do not change what's included in the core.
- The bloom filter (2048-bit, 3 hash functions) is part of the signed
  header. Adding new bloom-indexed event types means changing block
  production, not patching after the fact.

### `Finalize()` is one-shot

- `block.Finalize()` adds endorsements and the commit timestamp. The
  second call **returns an error** (`"the block has been finalized"`);
  do not ignore it and re-finalize over a copy. See `block.go:78–86`.

### `CommitBlock()` is idempotent

- If a block at the same height/hash already exists, `dao.PutBlock()`
  returns nil and the commit proceeds without re-emitting subscriber
  events. See `blockchain.go:545–582`.

### Receipts are not in the block file

- Receipts and transaction logs are written to separate indexers
  (`_receiptsNS`, `_systemLogNS`). Reading a block back without the
  indexer gives you no receipts. Code that needs receipts must check the
  indexer exists.

### Subscribers can backpressure block commit

- `SendBlockToSubscribers()` does a **blocking send** into each
  subscriber's buffered channel (`pubsubmanager.go:94–100`). A slow
  subscriber whose buffer fills will stall the caller — which today is
  the commit path. Do not add subscribers that block on slow I/O or
  network calls without a bounded internal timeout.

### Genesis is loaded once via `sync.Once`

- `blockchain/genesis/genesis.go:36` — process-wide singleton. Runtime
  mutation of `Genesis` after `Default()` returns is a no-op for already-
  initialized callers. Pass genesis via `WithGenesisContext()`; do not
  reach for the package-level default in code paths that need overrides.

### FileDAO has multiple on-disk formats

- Legacy v1 (monolithic), v2 (split by height range), v2 manager. The file
  header carries the version tag. Code reading raw file bytes must respect
  the tag. Do not "auto-upgrade" a legacy file silently.

---

## Sentinels and gotchas

- **Test blocks are generated, not frozen.** `block/testing.go` builds
  blocks from `identityset` keys at run time. Tests that depend on a
  specific block hash will break if the builder changes; prefer behavior
  assertions over hash equality.
- **`Validate` and `Mint` both run actions**, but only `Mint` writes
  state. Validating a block produced by another node executes the same
  actions in a sandbox — be aware before adding side effects to action
  handlers.

---

## Where to look (non-obvious mappings only)

- Header signing scope (what `HashHeaderCore` excludes): `block/header.go:207–226`.
- Persistence is split across two subpackages: `blockdao/` (abstraction +
  indexers) and `filedao/` (concrete file format, v1/v2).
- Use `block/testing.go` builder only in tests; production goes through
  `block/builder.go`.
