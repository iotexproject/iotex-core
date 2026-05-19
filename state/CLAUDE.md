# CLAUDE.md — state

Module-specific guidance for `state/` and `state/factory/`. Read
[/AGENTS.md](../AGENTS.md) first for repo-wide rules.

## What this package does

Protocol state storage: accounts, candidate records, and per-protocol state.
The `factory/` subpackage owns the **WorkingSet** — a per-block,
transactional view that all action handlers mutate through.

Major files: `state/account.go`, `state/candidate.go`, `state/tables.go`
(namespace list), `factory/factory.go`, `factory/workingset.go`,
`factory/workingsetstore*.go`.

---

## Module red lines

### `WorkingSet` is single-threaded; do not share across goroutines

- No internal locks beyond the underlying store. Concurrent reads/writes
  from action handlers will corrupt state silently.
- One working set per block. Height is fixed at construction (`Height()`,
  workingset.go:112).

### `finalized` is a one-way gate

- After `Finalize(ctx)`, all `PutState`/`DelState` calls return an error
  (workingset.go:99,105,117). The set becomes read-only forever.
- Do not work around this with reflection or by re-creating a working
  set at the same height — Commit() expects the original.

### Snapshot/Revert must coordinate store and views

- `Snapshot()` returns an opaque int ID. The ID maps both a store
  snapshot and a views snapshot (`viewsSnapshots`, workingset.go:89).
- `RevertSnapshot(id)` rolls back **both**. Reverting only one half
  leaves state inconsistent. Reverting to an invalid ID panics.

### Dual-storage writes — primary + Erigon — both must succeed

- The factory writes to the native MPT (primary) and to Erigon
  IntraBlockState (secondary). Migration is in progress; both stores
  must reflect identical state. A partial write poisons the block.
- See `factory/workingsetstore_with_secondary.go`. Check `git log` on
  this directory before touching it — the dual-write path is under
  active development.

### Namespace ownership — do not write into another protocol's namespace

- `state.RewardingNamespace`, `state.StakingNamespace`,
  `state.CandidateNamespace`, etc. are owned by their respective
  protocols. Cross-protocol writes must go through the protocol's API,
  not by writing keys directly.
- Master list of namespaces: `state/tables.go`.

### Account nonce semantics depend on type

- Type 0 (legacy): `PendingNonce() = nonce + 1`. Starts at 0.
- Type 1 (post-conversion): `PendingNonce() = nonce`. Starts at 1.
- `ConvertFreshAccountToZeroNonceType` (account.go:181–189) flips the
  type on first action. Hand-rolling nonce logic in new code is almost
  always wrong; use the helpers.

### Balance is a base-10 string in proto

- `accountpb.Account.Balance` is a decimal string, not bytes. `SetString(s, 10)`
  with the `ok` check at the deserialize boundary. Malformed strings
  produce a nil balance silently if `ok` is ignored.

---

## Sentinels and gotchas

- **Clone before mutating shared state objects.** `AddBalance` and
  `SubBalance` mutate in place. If the object came from a shared cache or
  view, the change persists across revert boundaries.
- **No version pruning in core; retention is per-Commit.** `Commit(ctx,
  retention)` controls how many historical heights stay accessible.
  Querying beyond retention returns an error, not stale data.

---

## Where to look

| Topic | File |
|---|---|
| StateManager / StateReader interfaces | `action/protocol/managers.go` |
| Account struct, nonce types, balance ops | `state/account.go` |
| Namespace constants | `state/tables.go` |
| WorkingSet lifecycle | `factory/workingset.go` |
| Factory + historical state | `factory/factory.go` |
| Dual-storage (MPT + Erigon) | `factory/workingsetstore_with_secondary.go`, `factory/erigonstore/` |
