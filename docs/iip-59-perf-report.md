# IIP-59 performance report — epoch-grant on-chain reads

**Scope.** IIP-59 moves two Hermes off-chain lookups into `GrantEpochReward`.
Both go through the in-process EVM (`evm.SimulateExecution`), not RPC. Mainnet
runs on a **2.5s block interval** (Dardanelles), so per-call latency at mainnet
scale drives whether the current wiring is shippable or needs mitigation before
PR 6 (mainnet fork activation).

**Two levers landed** to keep the drain inside budget:

1. **PR C0/C1 — direct-slot AutoDeposit reader** replaces the
   `SimulateExecution` code path with a `NewStateDBAdapter + GetState`
   reader; ~26× per-voter latency reduction. Detail in
   `## AutoDeposit.bucket per-call cost` below.
2. **PR C2 — era-based chunked drain** spreads voter-reward distribution
   across N continuation blocks inside the era window, driven by a
   persisted cursor. Detail in `## Chunked drain (era-based v2)
   end-to-end` below.

These are complementary, not redundant: (1) shrinks per-voter cost,
(2) caps per-block work regardless of voter count. Combined verdict at
the bottom.

Two contracts on the hot path:

| Contract | Reader | Call site | Calls per epoch |
|---|---|---|---|
| `DelegateProfile` (`0xfa7f50866ac45d84adf54bc767c885f92750e258`) | `getProfileByField(address, string)` | `PutPollResult` → `SnapshotCommissionRates` | 24 delegates × ~2 fields ≈ **48** |
| `AutoDeposit` (`0x79f1670BE20daecEfB134E33D97f9E77340fd2C0`) | `bucket(address)` | `distributeCombinedReward` (voter drain) | up to **107,200** (100k native + 7k V1 CSC + 200 V2/V3) |

The 2.5s block budget is the fixed constant these numbers get graded against.

---

## AutoDeposit.bucket per-call cost

Bench: `action/protocol/execution/protocol_iip59_bench_test.go` (build tag
`iip59bench`). Deploys the mainnet runtime bytecode
(`e2etest/autodeposit_bytecode`) via a CODECOPY+RETURN preamble, seeds 30
distinct registered voters via real `register(int256)` txs, hot-loops
against a rotating registered voter. Three read paths compared:

1. **SimulateExecution** — the original wiring: full EVM dispatch, build
   ctx + working set + zero-address caller once, dispatch per call.
2. **`evm.ReadContractStorage` per-call** — bypasses EVM, but constructs a
   fresh `StateDBAdapter` for each of the two SLOADs (`registrants[owner]`
   then `buckets[owner]`).
3. **`NewStateDBAdapter` + `GetState`** — build the adapter ONCE per drain,
   call `GetState` twice per voter directly. **This is what
   `autoDepositContractReader` now uses in production** (PR C0/C1).

Storage layout is validated at bench setup: voter 2's `register(2)` value
disambiguates `buckets` (slot 1) from `registrants` (slot 2). Layout was
verified empirically, not derived from `contract ... is Pausable, Ownable`
naïvely — see the constants block comment for why the mapping lands at
slot 1 not slot 2.

Invocation:

```
go test -tags=iip59bench \
  -bench='BenchmarkAutoDeposit_bucket|BenchmarkAutoDeposit_bucket_DirectRead|BenchmarkAutoDeposit_bucket_AdapterReuse' \
  -benchmem -count=3 -run=^$ ./action/protocol/execution/
```

Results (Apple silicon dev host, three consecutive runs each):

| path                                | ns/op   | ns/voter | B/op    | allocs/op |
|-------------------------------------|---------|----------|---------|-----------|
| SimulateExecution (baseline)        | 66,504  | 66,504   | 68,796  | 810       |
| ReadContractStorage per-call        | 29,453  | 29,453   | 46,729  | 598       |
| **NewStateDBAdapter + GetState reuse** | **2,518** | **2,518** | **3,171** | **58** |
| Wrapper contract batch (30 voters) | 362,900 | ~12,100 | 260,213 | 3,412     |

Stddev < 2% within each row. Adapter reuse is **~26× faster than baseline**
and ~12× faster than per-call ReadContractStorage — the savings come almost
entirely from amortizing the `StateDBAdapter` construction (contract cache,
snapshot buffers, access list, transient storage). Actual trie SLOAD is
sub-microsecond.

The wrapper-contract row is a single `SimulateExecution` of
`AutoDepositBatch.buckets(address[30])`, deployed once with the mainnet
AutoDeposit address as ctor arg. The per-voter cost is ~5× the adapter-reuse
path — the batch amortises EVM setup, but every bucket lookup still pays
STATICCALL + parameter marshalling + int256 abi-encoding overhead. See
`e2etest/autodeposit_batch.sol` for the source and
`e2etest/autodeposit_batch_init_bytecode` for the paris-EVM init bytecode
consumed by the bench.

### Extrapolation vs the 2.5s block

Per-call cost × voter count, held against a 2,500ms block budget (assuming
the drain runs to completion inside a single block — the pre-C2 shape):

| voters   | baseline SimExec | ReadContractStorage | Wrapper batch | **AdapterReuse** |
|----------|------------------|---------------------|---------------|-------------------|
| 1,000    | 66 ms (2.7%)     | 30 ms (1.2%)        | 12 ms (0.5%)  | **2.5 ms (0.1%)** |
| 10,000   | 665 ms (27%)     | 295 ms (12%)        | 121 ms (4.9%) | **25 ms (1.0%)**  |
| 50,000   | **3.33 s (133%)**| **1.47 s (59%)**    | 605 ms (24%)  | **126 ms (5.0%)** |
| 100,000  | **6.65 s (266%)**| **2.95 s (118%)**   | 1.21 s (48%)  | **252 ms (10%)**  |
| 107,200  | **7.13 s (285%)**| **3.16 s (126%)**   | 1.30 s (52%)  | **270 ms (11%)**  |

Wrapper-batch extrapolation assumes per-voter cost holds at scale — in
practice the batch would chunk (e.g. 1000 voters/call) to keep gas below the
block limit, adding a small per-chunk setup surcharge that doesn't move the
verdict.

Verdict flip at 2.5s: baseline breaches decisively at mainnet scale (285%);
even `ReadContractStorage` per-call breaches (126%). Adapter reuse fits with
~89% headroom (270ms at 107k voters). This is the "shrink per-voter cost"
lever; the "cap per-block work" lever is documented below.

---

## DelegateProfile.getProfileByField per-call cost

**Not measured.** Skipped for this pass — the setter surface requires
staging a `Register` contract, per-field `Verifier` contracts, `newField`
provisioning, and `updateProfileForDelegate` calls behind an owner-only
gate, so the seeding scaffold is substantially larger than AutoDeposit's
one-liner `register(int256)`.

Bounding argument: at 48 calls/epoch, even the pessimistic assumption that
`getProfileByField` costs the same 65μs as `AutoDeposit.bucket` yields
**~3.1ms per epoch** — three orders of magnitude below the block budget and
noise-level next to the voter drain. If AutoDeposit gets mitigated and the
combined budget still looks tight, re-open this bench; otherwise it stays
deferred.

---

## Chunked drain (era-based v2) end-to-end

Bench: `e2etest/iip59_perf_test.go::TestIIP59EpochGrantPerf` (build tag
`e2e`). Drives a real chainservice through Phase A / Phase B chunk
iterations / Phase C sentinel; measures wall-clock per continuation
block via the `blk.Actions()` cursor progression. Contract dispatch is
stubbed by test-only injection seams (`autoDepositRewardsReader`,
poll snapshot hook, staking hook) so the numbers isolate the **drain
machinery cost** — the cursor read/write, admin/exempt/candidate
re-load, `distributeCombinedReward` orchestration — not the AutoDeposit
reader itself (measured separately above; combines additively).

Three tiers, all with `epochsPerEra` set so exactly one era boundary
fires inside the harness's block-mint budget:

| tier    | delegates | voters | era_epochs | batch | drain_blocks | p50    | p95    | max    | total   |
|---------|-----------|--------|------------|-------|--------------|--------|--------|--------|---------|
| small   | 3         | 100    | 2          | 2     | 2            | 34.0ms | 34.0ms | 34.0ms | 64.9ms  |
| medium  | 10        | 1,000  | 4          | 4     | 3            | 34.8ms | 37.5ms | 37.5ms | 98.1ms  |
| mainnet | 24        | 27,020 | 24         | 4     | 6            | 26.9ms | 28.0ms | 28.0ms | 155.6ms |

Invocation (mainnet tier shown; small is the default):

```
IIP59_PERF_TIER=mainnet go test -tags e2e -v -timeout 900s \
  -run TestIIP59EpochGrantPerf ./e2etest/
```

### Reading the numbers vs the 2.5s block budget

- **Per-block:** mainnet p95 is 28ms — **1.1% of the 2.5s block budget**.
  Even summed with the AdapterReuse extrapolation (270ms at 107k voters,
  spread across ~6 chunks ≈ 45ms/chunk), a mainnet continuation block
  runs at ~73ms total against 2500ms — **~2.9% budget, ~97% headroom**.
- **Per-era:** the drain must complete before the *next* era boundary
  fires `PutPollResult`. On mainnet an era spans 24 epochs × 360 blocks
  ≈ 8,640 blocks between boundaries; the drain uses **6 non-boundary
  blocks** at the head of the era. Effectively unbounded budget for the
  drain relative to the era window.
- **Chunk sizing:** `CompoundBatchSize = 4` was chosen so 24 delegates
  fit in 6 chunks (24 / 4 = 6). Scaling either way is linear; raising
  batch to 8 would halve `drain_blocks` at the cost of doubling
  per-block work — irrelevant at current headroom.

### What the bench does and doesn't cover

The harness stubs contract dispatch, so the drain-block numbers do
**not** reflect the AutoDeposit reader path — that cost is measured
independently in the section above and combines additively. What the
bench **does** validate:

- Chunked-drain cursor invariants (advance, resume, delete-on-final).
- The one-continuation-per-block emission rule in
  `CreatePostSystemActions`.
- Cross-block resume of the freeze snapshot
  (`epochDrainDelegateWork.PoolAmountFrozen`).
- The absence of duplicated foundation-bonus or sentinel writes
  across chunks (checked via `assertNoRewardYet` + cursor delete
  ordering).

Real-world drift from these numbers will come almost entirely from
the AdapterReuse column above — the drain machinery cost is bounded
by delegate count, not voter count.

---

## Verdict

**Both levers landed and the mainnet-scale drain fits with wide margin.**

- **Baseline as originally wired breached** (7.1s / 285% of a 2.5s block
  at 107k voters, if executed as a single-block system action).
- **Direct-slot AutoDeposit reader** (PR C0/C1) cuts per-voter cost 26×
  → 270ms / 11% of block at 107k voters.
- **Era-based chunked drain** (PR C2) further caps per-block work at
  ~28ms of drain machinery (mainnet tier bench) by spreading distribution
  across 6 continuation blocks inside the era.
- **Combined mainnet estimate:** ~73ms per continuation block (2.9%
  budget) × 6 blocks = ~440ms of total drain wall-clock, against a
  ~21,600s (8,640-block) era window. Real headroom is measured in
  orders of magnitude.

No further mitigation needed for PR 6 (mainnet fork activation).

### Recommended mitigation — landed

Landed in PR C0/C1. `autoDepositContractReader` is now a batched
adapter-reuse reader constructed once at drain start:

```go
// Sketch — production would live in the autodeposit package as a
// batchReader that satisfies the same ContractReader interface, or a
// new BatchContractReader interface if we want to keep both codepaths.
func autoDepositBatchReader(sm protocol.StateManager, contractAddr address.Address) BatchReader {
    return func(voters []address.Address) ([]int64, error) {
        adapter, err := evm.NewStateDBAdapter(sm, blockHeight, hash.ZeroHash256)
        if err != nil { return nil, err }
        contractEvm := common.BytesToAddress(contractAddr.Bytes())
        out := make([]int64, len(voters))
        for i, v := range voters {
            regKey := mappingSlotKey(v, autoDepositSlotRegistrants)
            if adapter.GetState(contractEvm, regKey)[31] == 0 {
                out[i] = -1
                continue
            }
            buckKey := mappingSlotKey(v, autoDepositSlotBuckets)
            out[i] = int64(new(big.Int).SetBytes(
                adapter.GetState(contractEvm, buckKey).Bytes(),
            ).Int64())
        }
        return out, nil
    }
}
```

**Coupling risk is bounded** because `AutoDeposit` is not upgradeable
(`Ownable + Pausable` only, no proxy pattern) — the storage layout is
frozen for the lifetime of the contract. The two constants
(`autoDepositSlotBuckets = 1`, `autoDepositSlotRegistrants = 2`) can be
hardcoded with a sanity-check unit test that reads a known-registered
voter and asserts the value.

### Second lever — landed

Spreading distribution across continuation blocks landed as PR C2
(era-based chunked drain). Original framing had this as a fallback if
the adapter-reuse path failed safety review; in practice it was
promoted to a first-class defense-in-depth mechanism because:

- It bounds per-block latency by delegate count (fixed at 24 mainnet)
  rather than voter count (~27k and growing). Any future voter growth
  no longer affects per-block budget headroom.
- It cleanly separates the era-boundary "who gets what" freeze
  (Phase A) from the compute-heavy distribution (Phase B), which makes
  each phase idempotent-per-block and reviewable in isolation.
- It's fork-gated (`NoVoterRewardDistribution`) so legacy chains are
  untouched.

Semantics: rewards for a given era land across the first ~6 non-boundary
blocks of the next era rather than in the single era-boundary block. The
foundation bonus and sentinel write happen in Phase C (final chunk), so
downstream reward claims see a single moment of "era N is fully paid"
just as before.

### Historical alternatives (superseded)

Preserved for context. Both were considered before mitigation 2 + PR C2
landed and are no longer needed:

- **Batch wrapper contract** (measured: **1.3s / 52% of a 2.5s block**).
  New Hermes-owned `AutoDepositBatch` exposing `buckets(address[]) →
  int256[]`. Would keep rewarding-side code on plain `SimulateExecution`
  but at ~5× the per-voter cost of adapter reuse plus a governance
  deploy + genesis update. Rejected in favour of the direct-slot reader.
- **`ReadContractStorage` per-call** (measured: **3.2s / 126% of block**).
  Simpler diff but breaches budget even at mainnet voter count. Never
  viable at the 2.5s budget.

---

## Reproducing

```
# Fixtures (one-time; overwrites hex-encoded runtime bytecode in-tree)
./scripts/fetch-mainnet-bytecode.sh

# Micro-bench — AutoDeposit.bucket per-call cost
go test -tags=iip59bench -bench=BenchmarkAutoDeposit_bucket \
  -benchmem -count=3 -run=^$ ./action/protocol/execution/

# E2E chunked-drain bench (tiers: small | medium | mainnet)
IIP59_PERF_TIER=mainnet go test -tags e2e -v -timeout 900s \
  -run TestIIP59EpochGrantPerf ./e2etest/
```

Bench source:

- Micro: `action/protocol/execution/protocol_iip59_bench_test.go`.
- E2E: `e2etest/iip59_perf_test.go`.

Fixtures: `e2etest/autodeposit_bytecode`, `e2etest/delegateprofile_bytecode`.
