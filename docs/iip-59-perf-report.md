# IIP-59 performance report — epoch-grant on-chain reads

**Scope.** IIP-59 moves two Hermes off-chain lookups into `GrantEpochReward`.
Both go through the in-process EVM (`evm.SimulateExecution`), not RPC. The
epoch-grant system action must complete inside a single ~5s block, so
per-call latency at mainnet scale drives whether the current wiring is
shippable or needs mitigation before PR 6 (mainnet fork activation).

Two contracts on the hot path:

| Contract | Reader | Call site | Calls per epoch |
|---|---|---|---|
| `DelegateProfile` (`0xfa7f50866ac45d84adf54bc767c885f92750e258`) | `getProfileByField(address, string)` | `PutPollResult` → `SnapshotCommissionRates` | 24 delegates × ~2 fields ≈ **48** |
| `AutoDeposit` (`0x79f1670BE20daecEfB134E33D97f9E77340fd2C0`) | `bucket(address)` | `distributeCombinedReward` (voter drain) | up to **107,200** (100k native + 7k V1 CSC + 200 V2/V3) |

The 5s block budget is the fixed constant these numbers get graded against.

---

## AutoDeposit.bucket per-call cost

Bench: `action/protocol/execution/protocol_iip59_bench_test.go` (build tag
`iip59bench`). Deploys the mainnet runtime bytecode
(`e2etest/autodeposit_bytecode`) via a CODECOPY+RETURN preamble, seeds 30
distinct registered voters via real `register(int256)` txs, hot-loops
against a rotating registered voter. Three read paths compared:

1. **SimulateExecution** — mirrors `autoDepositContractReader`
   (`action/protocol/rewarding/voter_reward.go:393`), the current PR 3′
   wiring. Full EVM dispatch: build ctx + working set + zero-address caller
   once, dispatch per call.
2. **`evm.ReadContractStorage` per-call** — bypasses EVM, but constructs a
   fresh `StateDBAdapter` for each of the two SLOADs (`registrants[owner]`
   then `buckets[owner]`).
3. **`NewStateDBAdapter` + `GetState`** — build the adapter ONCE per drain,
   call `GetState` twice per voter directly. This is the shape the
   production mitigation would take.

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

### Extrapolation vs the 5s block

Per-call cost × voter count, held against a 5,000ms block budget:

| voters   | baseline SimExec | ReadContractStorage | Wrapper batch | **AdapterReuse** |
|----------|------------------|---------------------|---------------|-------------------|
| 1,000    | 66 ms (1.3%)     | 30 ms (0.6%)        | 12 ms (0.2%)  | **2.5 ms (0.05%)** |
| 10,000   | 665 ms (13%)     | 295 ms (6%)         | 121 ms (2.4%) | **25 ms (0.5%)**  |
| 50,000   | 3.33 s (67%)     | 1.47 s (29%)        | 605 ms (12%)  | **126 ms (2.5%)** |
| 100,000  | **6.65 s (133%)**| 2.95 s (59%)        | 1.21 s (24%)  | **252 ms (5%)**   |
| 107,200  | **7.13 s (143%)**| 3.16 s (63%)        | 1.30 s (26%)  | **270 ms (5.4%)** |

Wrapper-batch extrapolation assumes per-voter cost holds at scale — in
practice the batch would chunk (e.g. 1000 voters/call) to keep gas below the
block limit, adding a small per-chunk setup surcharge that doesn't move the
verdict.

Verdict flip: baseline breaches the block budget at mainnet scale; adapter
reuse fits with ~95% headroom. Memory footprint drops from ~7.4 GB to
~340 MB of transient allocation across a full drain.

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

## Verdict

**PR 3′ as currently wired breaches the block budget** (7.1s vs 5s at
mainnet scale). **Mitigation via adapter reuse fits with ~95% headroom**
(0.27s at mainnet scale), needs no contract change, and is a small
surface diff on the rewarding side.

### Recommended mitigation

Replace `autoDepositContractReader` (`voter_reward.go:393`) with a batched
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

### Alternative mitigations, in decreasing preference

If the adapter-reuse approach hits an obstacle (safety review, subtle
adapter semantics not captured here, etc.), the fallback options in order:

1. **Deploy a batch wrapper contract** (measured: **1.3s / 26% of block**).
   New Hermes-owned contract `AutoDepositBatch` exposing
   `buckets(address[]) → int256[]` that STATICCALLs the existing
   `AutoDeposit.bucket` in a for-loop; the bench measures 12μs/voter in a
   batch of 30. One `SimulateExecution` per drain (or per chunk).
   Trade-off: keeps rewarding-side code on the plain SimulateExecution
   path (no adapter lifecycle, no hardcoded slot constants), but needs a
   Solidity contract + governance deploy + genesis config update, and pays
   ~5× the per-voter cost of adapter reuse. Prefer if the storage-layout
   coupling in the recommended path is judged too fragile despite the
   contract being non-upgradeable.
2. **`ReadContractStorage` per-call.** Simpler diff (one function call
   substitution, no adapter lifecycle) but at 3.2s it eats 63% of the
   block budget — thin margin for future growth. Only preferred if the
   adapter-reuse approach fails safety review AND the wrapper deploy is
   blocked.
3. **Spread drain across sub-epoch blocks.** Changes reward-emission
   semantics (rewards land over N blocks not 1) and needs a fork-gated
   migration. Only if the first three all fail.

---

## Reproducing

```
# Fixtures (one-time; overwrites hex-encoded runtime bytecode in-tree)
./scripts/fetch-mainnet-bytecode.sh

# Micro-bench
go test -tags=iip59bench -bench=BenchmarkAutoDeposit_bucket \
  -benchmem -count=3 -run=^$ ./action/protocol/execution/
```

Bench source: `action/protocol/execution/protocol_iip59_bench_test.go`.
Fixtures: `e2etest/autodeposit_bytecode`, `e2etest/delegateprofile_bytecode`.
