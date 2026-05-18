# AGENTS.md

Guidance for AI coding agents (and humans) working in this repository.

This file is the single source of truth at the project level; `CLAUDE.md`
points here. Sub-packages may have their own `CLAUDE.md` for module-specific
constraints. For build/test setup, see [CONTRIBUTING.md](./CONTRIBUTING.md).

---

## Project context

iotex-core is the reference Go implementation of the IoTeX Layer-1 blockchain.
The code runs on production mainnet and testnet. Any change that affects state
transitions, receipts, or consensus is, by default, **breaking**. Treat the
codebase as production infrastructure, not application code.

---

## Hard rules (red lines)

These apply repo-wide. Crossing them has caused, or would cause, on-chain
incidents. Do not cross without a maintainer's explicit sign-off in the PR.

### 1. Behavior changes must be gated on a new hardfork height

- Hardfork heights are defined in `blockchain/genesis/genesis.go` and fan out
  through `FeatureCtx` in `action/protocol/context.go`.
- An existing hardfork height that is non-zero on mainnet is **immutable**.
  Never edit, reorder, or delete one. New behavior goes behind a new fork.
- New feature gates must be checked at every code path that runs the feature.
  Tests must cover both pre-fork and post-fork branches.

### 2. Determinism — on-chain code is pure with respect to host state

- On-chain time comes from `ctx.BlockCtx.BlockTimeStamp`. Do not call
  `time.Now()` in `action/protocol/`, `state/`, or `consensus/` execution paths.
- Do not iterate Go maps in code that produces state or receipts. Sort keys.
- Do not introduce randomness, file I/O, network calls, or any other source of
  host-dependent input into action handling or state transitions.

### 3. On-chain schemas are append-only after mainnet deployment

- State and receipt protobuf schemas (e.g. `*.proto` files under
  `action/protocol/*/pb/`) are immutable once shipped. Never reorder, renumber,
  or delete fields. Add new fields at the next available number; retire fields
  by marking them `reserved`.
- The same rule applies to Go structs serialized to the chain: do not reorder
  or remove fields without a hardfork-gated migration.

### 4. Deprecated fields stay; never remove them

- Fields marked `// deprecated` (in genesis, state structs, or proto messages)
  remain in place for node startup and historical replay compatibility. Mark,
  do not delete.

### 5. Protocol execution order = registration order

- Protocols are registered into `action/protocol/registry.go`. `All()` returns
  them in registration order, and handlers run in that order. Do not rely on
  any other ordering, and do not reorder existing registrations without a
  hardfork plan.

---

## Conventions

Long-standing project practice. Follow unless you have a concrete reason not to.

- **Hardfork naming.** New hardforks are named after places (Hawaii, Iceland,
  Tsunami, Yap, ...). See `blockchain/genesis/genesis.go` for the running list.
- **Hardfork tests.** When a fork gates a behavior, test both `height = fork - 1`
  and `height = fork`.
- **State versioning across forks.** Prefer a new type or namespace (e.g.
  `FooV2`, `FooV3`) over mutating an existing on-chain struct.
- **Linter strictness** (`.golangci.yml`): cyclomatic complexity ≤ 15, function
  length ≤ 150 lines / 50 statements, line length ≤ 140, goimports with local
  prefix `github.com/iotexproject/iotex-core`. Run `make fmt` before committing.

---

## Workflow

- Before opening a PR, run:
  ```
  make fmt
  make lint
  make test
  ```
  Do not bypass with `--no-verify` or by skipping lints.
- Commits follow conventional-commit style with a package scope:
  `feat(staking): …`, `fix(consensus): …`, `refactor(state): …`,
  `test(blockchain): …`. Link the relevant issue.
- One concern per PR. If a refactor and a fix belong together, the refactor
  goes first as its own commit.

---

## When to ask before acting

Confirm with a maintainer in the issue or PR before:

- choosing a hardfork height,
- changing any proto schema or on-chain struct layout,
- modifying anything under `consensus/`,
- removing or renaming a field in `blockchain/genesis/genesis.go`,
- altering action serialization or receipt-log emission paths.

---

## Where to look next

| You are working on... | Read first |
|---|---|
| Staking / candidates / buckets | `action/protocol/staking/CLAUDE.md` |
| Block / epoch rewards | `action/protocol/rewarding/CLAUDE.md` |
| Block production / validation | `blockchain/CLAUDE.md` |
| Account / state factory / MPT | `state/CLAUDE.md` |
| RollDPoS / endorsements | `consensus/CLAUDE.md` |
| Dev setup, local tests | [CONTRIBUTING.md](./CONTRIBUTING.md) |
| Past architectural decisions | `docs/adr/` |
| Domain terms | `docs/glossary.md` |
