# CLAUDE.md — consensus

Module-specific guidance for `consensus/` (RollDPoS). Read
[/AGENTS.md](../AGENTS.md) first for repo-wide rules.

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

- `calculateProposer` (`roundcalculator.go`) picks
  `proposers[idx % numProposers]` where `idx = height` (or `height + round`
  when `timeBasedRotation` is enabled). It is a plain modulo over the
  epoch's delegate list — no hashing is involved.
- Adding any nondeterministic input (RNG, wall clock not derived from
  round start, network state) forks the chain. Do not "improve" this.

### TTL sum invariant: `AcceptBlockTTL + AcceptProposalEndorsementTTL + AcceptLockEndorsementTTL + CommitTTL ≤ BlockInterval`

- Validated when the RollDPoS context is constructed
  (`scheme/rolldpos/rolldposctx.go`); per-height TTL getters live in
  `consensusfsm/consensus_ttl.go`.
- Changing one TTL almost always requires rebalancing the others. The
  Dardanelles and Wake upgrades adjusted all four simultaneously.

### Consensus endorsement ≠ staking endorsement

- Three consensus endorsement topics: `PROPOSAL`, `LOCK`, `COMMIT`
  (`consensusvote.go`). These are crypto signatures over
  `(blockHash, topic)` by a delegate.
- The staking-protocol "endorsement" (delegated voting) is a different
  concept entirely. Do not share types, validators, or storage between
  the two.

### Message validation must happen before FSM event emission

- Signature verification and proposer/endorser-is-a-valid-delegate checks
  run before `ProduceReceiveBlockEvent` etc. are emitted
  (`rolldpos.go`). Reordering this is a security break.

### Do not retain endorsements past round start

- `endorsementmanager.go` discards endorsements older than the
  current round; COMMIT endorsements alone persist into the next round
  for Proof-of-Lock. The cleanup is intentional.

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

## Where to look (non-obvious mappings only)

- Hardfork-gated TTL / block-interval config: `consensusfsm/consensus_ttl.go`.
- Consensus vote types (distinct from staking-protocol endorsements):
  `scheme/rolldpos/consensusvote.go`.
