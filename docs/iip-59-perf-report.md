# IIP-59 performance report — era freeze and voter drain

**Budget.** Mainnet runs on a **2.5s block interval** (Dardanelles). Every
number below is graded against 2,500ms.

**Host.** Apple silicon dev host, 10 cores, Go 1.25.5, darwin/arm64. Timings
are wall-clock per minted block as reported by the harness itself. Repeated
runs of the mainnet tier on this host spread about ±7% around the figures
below, so treat the third significant digit as noise.

---

## 0. Every previously published drain figure in this file was invalid

The previous revision of this document reported a mainnet-tier drain at
**p50 26.9ms / p95 28.0ms** and concluded the drain cost "~1.1% of the block
budget". Those numbers are withdrawn, not merely stale.

`e2etest/iip59_perf_test.go` and `e2etest/iip59_stress_test.go` ran on
`poll.NewLifeLongDelegatesProtocol`, which never calls `FreezeCandidateRewardSnapshots`. No
era was ever frozen, so every `CandidateRewardSnapshot` the harness produced
carried `FreezeHeight 0` — the value that makes a delegate **unpayable**. The
drain dutifully created a cursor, walked the voter index, paid **nobody**, and swept
the entire pool as residual. The only assertions in place were cursor-lifecycle
and fund-conservation ones, and fund conservation holds perfectly well when
every voter is paid zero. The harness was measuring an empty walk.

A test-only `iip59EraFreezer` protocol now drives a real freeze at every
era-boundary epoch, and both harnesses assert per-voter payout amounts (see
`e2etest/iip59_payout_test.go`). The figures in §1 are the first measurements of
a settlement that actually pays.

**Old and new figures are not comparable in either direction.** The new numbers
are ~18× larger at the mainnet tier, and essentially all of that is work the old
harness skipped: the copy-on-write freeze, the on-demand per-voter weight
recompute at the frozen height, and the balance writes themselves.

---

## 1. End-to-end chunked drain

Bench: `e2etest/iip59_perf_test.go::TestIIP59EpochGrantPerf`. Drives a real
`chainservice` from the era-boundary block through every continuation chunk to
cursor deletion. Contract dispatch (`DelegateProfile`, `AutoDeposit`) is stubbed
by test-only injection seams, so these numbers are **drain machinery plus era
freeze plus weight recompute**, not contract reads.

```
IIP59_PERF_TIER=small|medium|mainnet \
  go test -run TestIIP59EpochGrantPerf -count=1 -v -timeout 120m ./e2etest/
```

| tier | delegates | voters | era epochs | voter budget | blocks in window | continuation chunks | p50 | p95 | max | total |
|---|---|---|---|---|---|---|---|---|---|---|
| small | 3 | 100 | 2 | 50 | 4 | 2 | 32.7ms | 34.6ms | 34.6ms | 122.8ms |
| medium | 10 | 1,000 | 4 | 250 | 6 | 4 | 50.9ms | 56.9ms | 56.9ms | 267.2ms |
| mainnet | 24 | 27,020 | 24 | 4,504 | 8 | 6 | 482.0ms | 509.0ms | 509.0ms | 3.006s |

"Blocks in window" counts the era-boundary block (Phase A) and the first block
after the cursor disappears, both of which the harness samples; "continuation
chunks" is the number of blocks that actually paid voters.

Per-block detail, mainnet tier — the shape is what matters:

| height | role | wall |
|---|---|---|
| 1152 | era boundary (Phase A: freeze + cursor write) | 34.0ms |
| 1153 | chunk 1 | 509.0ms |
| 1154 | chunk 2 | 498.6ms |
| 1155 | chunk 3 | 495.9ms |
| 1156 | chunk 4 | 482.0ms |
| 1157 | chunk 5 | 479.2ms |
| 1158 | chunk 6 (final: residual sweep, seal, complete) | 479.9ms |
| 1159 | first block after completion | 27.8ms |

**Against the 2.5s budget:** the worst continuation block is **509.0ms, 20.4%**
of a block. Phase A — the era boundary, which also carries `PutPollResult` — is
34.0ms. Continuation cost is flat across chunks (509 → 480ms, mildly
*decreasing*), which is what a correctly budgeted walk should look like: each
chunk does `VoterBudgetPerBlock` voters' worth of work regardless of where in
the key space it is.

**Per era:** on mainnet an era is 24 epochs ≈ 8,640 blocks. The settlement
occupies 6 of them at the head of the era. The drain has ~1,400× more block
budget than it uses.

### 1.1 What these numbers do not cover

Read these as a lower bound. Four reasons, in decreasing order of size:

1. **In-memory state, not a trie.** The harness runs the chainservice's default
   factory over the test DB paths, not a mainnet-sized trie under leveldb/pebble
   with hash verification. Real per-read cost is higher by an
   implementation-dependent factor.
2. **The historical fixture concentrated addresses under one prefix.** This is
   not representative of the mainnet key distribution. The current scale
   fixture spreads voter addresses over the full key space; new measurements
   should not be compared directly with this historical table.
3. **Contract dispatch is stubbed.** No `AutoDeposit.bucket` read, no
   `DelegateProfile.getProfileByField`. See §2.
4. **The freeze is driven by a test-only protocol.** `iip59EraFreezer` opens the
   copy-on-write window and stamps `FreezeHeight` at the first block of every
   era-boundary epoch; production drives the same path from `PutPollResult` →
   `FreezeCandidateRewardSnapshots`. The COW window and the recompute are the real ones.

## 2. Contract-read cost (unreproducible from this tree)

The previous revision reported an `AutoDeposit.bucket` micro-bench comparing
`SimulateExecution` (66.5μs/voter), per-call `ReadContractStorage` (29.5μs) and
`NewStateDBAdapter` + `GetState` with adapter reuse (2.5μs), concluding a ~26×
saving for the direct-slot reader now in production.

**Those numbers cannot be re-measured on this branch.** The bench file it names,
`action/protocol/execution/protocol_iip59_bench_test.go`, does not exist; the
only surviving `iip59bench`-tagged file in that package is
`autodeposit_slot_reader_sanity_test.go`, which is a correctness test, not a
benchmark. Running the documented invocation matches zero benchmarks:

```
$ go test -tags=iip59bench -bench='BenchmarkAutoDeposit_bucket' \
    -benchmem -count=3 -run=^$ ./action/protocol/execution/
PASS
ok  github.com/iotexproject/iotex-core/v2/action/protocol/execution  1.190s
```

The architectural conclusion (use the direct-slot reader, not
`SimulateExecution`) is already implemented and is not in question. What is gone
is the ability to reproduce the measurement. Anyone who needs a defensible
contract-read number before activation has to restore the bench.

`DelegateProfile.getProfileByField` remains unmeasured, as before. At ~48 calls
per epoch it is bounded well below noise even under pessimistic assumptions.

## 3. Staking-side micro-benchmarks

```
go test -tags=iip59bench -run=^$ -benchmem -count=1 \
  -bench='BenchmarkFreezeSnapshotNativeEnumeration|BenchmarkAddDepositForCompound' \
  ./action/protocol/staking/
```

**`BenchmarkAddDepositForCompound`** — the write path taken once per compounding
voter:

| shape | ns/op | B/op | allocs/op |
|---|---|---|---|
| cand=10, voters=100 | 40,264 | 29,941 | 414 |
| cand=100, voters=1,000 | 44,325 | 29,948 | 414 |
| cand=100, voters=10,000 | 42,954 | 29,937 | 414 |

Flat in population, ~40–44μs per compounding voter over the in-memory mock state
manager. At 27,020 voters, if every one of them compounded, this is ~1.2s of
work — which is why it is chunked, and it is consistent with the ~500ms
continuation blocks in §1 where routing falls to the direct-credit path.

**`BenchmarkFreezeSnapshotNativeEnumeration`** — note this measures an *in-test
replica* (`benchAggregateNativeVoterEntries`) of the **retired** enumerate-at-
freeze design, not production code. The production freeze no longer enumerates
buckets at all; it opens a copy-on-write window and writes per-delegate scalars.
The numbers survive as an order-of-magnitude for what per-bucket weight
computation costs, since the drain still does that work — just later and
chunked:

| tier | delegates | buckets | per-freeze |
|---|---|---|---|
| small_5x40 | 5 | 200 | 1.91ms |
| mainnet_uneven_52d_7508b | 52 | 6,077 | 56.7ms |
| ceiling_1x30000 | 1 | 30,000 | 281.1ms |

≈9.4μs per bucket at the ceiling tier, in memory.

## 4. Verdict

- The settlement, measured for the first time on a harness that actually pays
  voters, costs **509ms in its worst block — 20.4% of a 2.5s block** at
  mainnet tier (24 delegates, 27,020 voters, `VoterBudgetPerBlock = 4,504`).
- The era-boundary block itself is cheap (34ms); the cost is in the six
  continuation chunks, and it is flat across them.
- Total settlement wall-clock is ~3.0s spread over 6 blocks inside an ~8,640
  block era.
- Headroom is real but it is **4.9×, not the ~90× the withdrawn numbers
  implied**. Lowering `VoterBudgetPerBlock` trades chunks for per-block cost
  linearly and is the available lever: at 2,000 (the genesis default) the same
  population would take 14 chunks at roughly 230ms each.

**Before activation, two things are still outstanding** and neither is closed by
this report:

1. A trie-backed measurement. Everything in §1 and §3 is in-memory; the
   multiplier from real trie reads is the single largest unknown in the budget.
2. A run of the current spread-address fixture against a trie-backed multi-node
   cluster (§1.1 item 2).

## 5. Reproducing

```
# End-to-end chunked drain (tiers: small | medium | mainnet)
IIP59_PERF_TIER=mainnet go test -run TestIIP59EpochGrantPerf \
  -count=1 -v -timeout 120m ./e2etest/

# Staking micro-benchmarks
go test -tags=iip59bench -run=^$ -benchmem -count=1 \
  -bench='BenchmarkFreezeSnapshotNativeEnumeration|BenchmarkAddDepositForCompound' \
  ./action/protocol/staking/
```

Bench source: `e2etest/iip59_perf_test.go`,
`action/protocol/staking/bench_freeze_snapshot_test.go`,
`action/protocol/staking/add_deposit_compound_bench_test.go`.
