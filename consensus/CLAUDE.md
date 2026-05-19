# CLAUDE.md — consensus

Module-specific guidance for `consensus/` (RollDPoS). Read
[/AGENTS.md](../AGENTS.md) first for repo-wide rules. Changes here usually
warrant a maintainer review before significant work — coordinate early.

## What this package does

RollDPoS consensus: BFT-style proposer/endorsement protocol over
delegate-based rotation. Subpackages:

| Path | Purpose |
|---|---|
| `consensus/` | Scheme factory (RollDPoS, NOOP, Standalone), lifecycle |
| `consensus/consensusfsm/` | Finite state machine, event queue, timing config |
| `consensus/scheme/rolldpos/` | RollDPoS implementation, rounds, endorsements |
| `consensus/endorsement/` | Endorsement signature primitive |

---

## Module red lines

### Proposer selection is deterministic — do not introduce randomness

- `Proposer(height, blockInterval, roundStartTime)` is a deterministic
  hash-and-modulo over the epoch's delegate list (`roundcalculator.go:106`).
- Adding any nondeterministic input (RNG, wall clock not derived from
  round start, network state) forks the chain. Do not "improve" this.

### TTL sum invariant: `AcceptBlockTTL + AcceptProposalEndorsementTTL + AcceptLockEndorsementTTL + CommitTTL ≤ BlockInterval`

- Validated at context creation (`consensusfsm/consensus_ttl.go:141–149`).
- Changing one TTL almost always requires rebalancing the others. The
  Dardanelles and Wake upgrades adjusted all four simultaneously.

### Consensus endorsement ≠ staking endorsement

- Three consensus endorsement topics: `PROPOSAL`, `LOCK`, `COMMIT`
  (`consensusvote.go:16–26`). These are crypto signatures over
  `(blockHash, topic)` by a delegate.
- The staking-protocol "endorsement" (delegated voting) is a different
  concept entirely. Do not share types, validators, or storage between
  the two.

### Message validation must happen before FSM event emission

- Signature verification and proposer/endorser-is-a-valid-delegate checks
  run before `ProduceReceiveBlockEvent` etc. are emitted
  (`rolldpos.go:98–157`). Reordering this is a security break.

### Round endorsement cleanup is intentional, not a leak

- Endorsements older than the current round start are discarded by
  `endorsementmanager.go:82–98`. COMMIT endorsements persist longer
  (used in Proof-of-Lock on the next block). Do not "fix" this cleanup
  to retain old data.

### Hardforks gate timing by height, not by clock

- Dardanelles and Wake changed block interval and TTLs. Gating is by
  block height, evaluated at context creation. Do not introduce
  time-based feature flags — they break replay.

### Endorsement persistence is optional

- `eManagerDB` (BoltDB) is configured per node. Light/non-validator nodes
  may run in-memory only. Code paths that assume the DB exists must
  check `eManagerDB != nil`.

---

## Sentinels and gotchas

- **Wall-clock dependency.** Even small NTP drift can cause state
  thrashing (timeouts firing while quorums arrive). Production
  deployments require synchronized clocks; tests must use the mock clock
  in `facebookgo/clock`, not real time.
- **Crash recovery is implicit.** On restart, the FSM resumes from the
  blockchain tip with a fresh round calculation; persisted endorsements
  are reloaded but no explicit consensus checkpoint exists. Do not
  introduce one without coordinating with operators.
- **No built-in DoS protection.** Rate limiting is the P2P layer's job.
  Do not assume the consensus layer drops floods.

---

## Where to look

| Topic | File |
|---|---|
| Scheme factory, lifecycle | `consensus.go` |
| FSM states and transitions | `consensusfsm/fsm.go` |
| TTL config, hardfork-gated timing | `consensusfsm/consensus_ttl.go` |
| RollDPoS main loop, message routing | `scheme/rolldpos/rolldpos.go` |
| Round / proposer calculation | `scheme/rolldpos/roundcalculator.go` |
| Endorsement manager (in-memory + BoltDB) | `scheme/rolldpos/endorsementmanager.go` |
| Consensus vote / endorsement types | `scheme/rolldpos/consensusvote.go` |
