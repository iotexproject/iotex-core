# IIP-59 voter weight seeding (A4) — withdrawn

**Status:** withdrawn. Nothing in this document is implemented, and nothing in
it is planned. It is kept as a short record of a blocker that was closed by
removing its cause rather than by building the thing described here.

The previous revision of this file was a ~430-line design for
`voterWeightSeedCursor`: a chunked, per-block flush of the in-memory
`VoterWeightView` into committed state at the IIP-59 activation height. It also
carried the sentence *"until this lands, the activation height must not be
set."* Both the design and that blocker are void. This note exists so that
sentence is not left standing in isolation, and so a reader who finds
`VoterWeightSeedBatchSize` in `genesis.go` or `_voterWeightSeed` in git history
knows what happened to them.

## What the problem was

An earlier IIP-59 design made per-`(candidate, voter)` vote weights **committed
state**: a `VoterWeightView` maintained incrementally by nine staking handler
hooks, loaded at startup instead of recomputed.

Committed state has to start somewhere. At the activation height every existing
pair would be missing, and `loadVoterWeightView` could not tell "pre-activation,
seed from buckets" apart from "activated, but only the pairs touched since
activation have been written". In the second state the view silently omits every
voter who has not staked since the fork, and the first era freeze pays out
against it.

Writing all the entries in the activation block was measured at roughly **7s at
30k buckets**, about 3× the 2.5s Dardanelles block budget, so the flush had to
be chunked across blocks under its own cursor — which is what this document
specified, together with the ordering, restart and concurrent-mutation rules
that a multi-block flush needs.

## Why it is withdrawn

The view it was seeding no longer exists. `VoterWeightView` and both of its
state tags (`_voterWeights`, `_voterWeightSeed`) were deleted; per-voter weights
are now recomputed on demand at the frozen era height by
`staking.FrozenVoterWeight` / `staking.FrozenVoterCandidates`
(`action/protocol/staking/era_voter_scan.go`), reading buckets through the era
copy-on-write window.

With no derived table there is no table to seed, no third unrecognisable state
to guard against, and no startup rebuild-and-compare to disagree with. The
seeding cursor, its genesis batch-size parameter, its ordering rule and its
restart semantics all became unreachable code, and A4 stopped being a
prerequisite for setting the activation height.

`genesis.Rewarding.VoterWeightSeedBatchSize` survives as a parsed-but-unused
field (`blockchain/genesis/genesis.go`, marked deprecated at its declaration).
It is deliberately not removed: it is a genesis-file field, and dropping it
would reject existing configs that set it.

## Where the current design is written down

- `docs/iip-59-distribution-architecture.md` — the era freeze, the on-demand
  weight recompute, and the drain that consumes them.
- `action/protocol/staking/era_window.go`, `action/protocol/staking/eracow/` —
  the copy-on-write window that made the recompute answerable at a past height.
- `action/protocol/staking/era_voter_scan.go` — the recompute and the sharded
  voter walk.
