# IOSwarm ePBS — Agent Block Building Design

Agents evolve from transaction validators to block builders, following Ethereum's ePBS (EIP-7732) model adapted for IoTeX's DPoS architecture.

## Motivation

Current IOSwarm: agents validate individual transactions → Coordinator assembles block.
L3 shadow accuracy has reached **100%** on mainnet via `SimulateAccessList` (EVM simulation-based storage prefetch, 230+ txs verified).

To achieve fully independent validation (no coordinator-provided state), we introduce stateful agents and progressively evolve them toward full block building:

| Level | Capability | State Requirement |
|-------|-----------|-------------------|
| L1-L3 (current) | Per-tx validation, stateless (L3 = 100% via SimulateAccessList) | None / coordinator-provided state snapshots |
| **L4 (next)** | **Per-tx EVM execution, full local state, independent validation** | **Full hot state via snapshot + diffs** |
| **L5 (future)** | **Full block building (ePBS)** | **Full hot state + mempool access** |

**L4 is the immediate target** — it gives agents fully independent state so they don't depend on coordinator-provided snapshots. L5 (block building) builds on the same state infrastructure.

## Role Mapping (L5 Target)

| Ethereum ePBS | IoTeX IOSwarm | Responsibility |
|--------------|---------------|----------------|
| Proposer (Validator) | **Delegate** | Select best block, sign, broadcast, consensus |
| Builder (staked) | **Agent** | Select txs, order, execute, build candidate block |
| Relay | **Coordinator** | Route candidate blocks, validate, manage agents |
| Unconditional payment | **Agent reward / slash** | Agent staked via RewardPool contract |
| Inclusion list (FOCIL) | **Delegate-mandated tx order** | Delegate specifies tx ordering (Phase 1) |

## Key Advantage: deltaStateDigest

IoTeX's block header contains `deltaStateDigest` — a hash of the ordered state change queue:

```
deltaStateDigest = hash(serializeQueue)  // ordered (namespace, key, value) changes
```

This is NOT a Merkle Trie root. This means:
- Agent does NOT need to maintain a full Merkle Patricia Trie
- Agent only needs a flat KV store that can execute transactions and track ordered writes
- State root computation is cheap — just hash the write queue
- Delegate re-execution can verify the digest matches

## What State Data the Agent Needs

### Namespaces in trie.db

| Namespace | Content | Agent Needs? |
|-----------|---------|-------------|
| **Account** | All accounts: nonce, balance, codeHash, storageRoot | **YES** |
| **Code** | Contract bytecode, keyed by codeHash | **YES** |
| **Contract** | Contract storage slots (per-contract trie) | **YES** |
| Preimage | Keccak256 preimages | Optional |
| Staking | Vote buckets, delegations | No |
| Rewarding | Reward pools, admin addresses | No |
| Candidate | Delegate info, candidate maps | No |
| System | Block metadata, candidates list | No |

**Only 3 of 10+ namespaces are needed.** Staking, Rewarding, and all native protocol state is irrelevant to EVM execution.

### Size Estimates (IoTeX Mainnet)

| Data | Estimate | Notes |
|------|----------|-------|
| Accounts | ~50-100 MB | ~200K-500K cumulative addresses, ~150 bytes each |
| Contract Code | ~50-100 MB | ~1,000-5,000 deployed contracts, ~5-20 KB avg bytecode |
| Contract Storage | ~200 MB - 1 GB | IoTeX DeFi TVL is small; far fewer storage slots than Ethereum |
| **Total pure state** | **~300 MB - 1.2 GB** | ~1000x smaller than Ethereum's ~245 GiB |

For comparison:
- Ethereum state: ~245 GiB (81.7% is contract storage)
- IoTeX testnet full node: ~10 GB (includes blocks, receipts, indices — state is a fraction)
- IoTeX mainnet full node: ~100-200 GB total

Even at the high end (~1.2 GB), this is trivially downloadable and storable on a $5 VPS.

## State Diff Reliability (Critical)

State diff reliability is the **#1 engineering risk**. If an agent misses a single diff, every subsequent block building attempt produces wrong `deltaStateDigest`.

### Chained State Hash

Every diff carries a `prev_state_hash` forming a chain, analogous to `prevBlockHash` in blocks:

```
state_diff = {
    height:          uint64,
    prev_state_hash: hash,   // hash of agent's state BEFORE applying this diff
    post_state_hash: hash,   // expected hash of agent's state AFTER applying this diff
    block_context:   { timestamp, gas_limit, base_fee, prev_block_hash },
    changes:         [(namespace, key, old_value, new_value)],  // ordered
}
```

### Agent-Side Verification

After applying each diff, the agent computes its local state hash and verifies:

```
apply(diff)
local_hash = hash(local_state)
if local_hash != diff.post_state_hash:
    // STATE DIVERGENCE — agent is out of sync
    request_resync()
```

### Recovery Strategies (in priority order)

1. **Request missing diff range**: if agent knows it missed height N, request diffs [N, tip] from coordinator
2. **Incremental re-snapshot**: coordinator sends only the changed KV pairs since agent's last known good height
3. **Full re-snapshot**: last resort, re-download the complete state snapshot

### Coordinator-Side

Coordinator maintains a rolling window of recent diffs (e.g., last 100 blocks) so agents can catch up without full re-snapshot. Beyond the window, agent must re-snapshot.

## Data Flow

```
                        ┌──────────────┐
                        │   Delegate   │
                        │  (Proposer)  │
                        └──────┬───────┘
                               │
            ┌──────────────────┼──────────────────┐
            │                  │                  │
            ▼                  ▼                  ▼
     ┌──────────┐       ┌──────────┐       ┌──────────┐
     │ Agent A  │       │ Agent B  │       │ Agent C  │
     │ (Builder)│       │ (Builder)│       │ (Builder)│
     └──────────┘       └──────────┘       └──────────┘
```

### Step 1: Cold Start — State Snapshot

Delegate exports a flat KV dump of the 3 required namespaces:

```
snapshot = {
    height:          uint64,
    post_state_hash: hash,               // hash of state at this height
    accounts:        map[address → Account],     // nonce, balance, codeHash
    code:            map[codeHash → bytecode],
    storage:         map[(address, slot) → value],
}
```

Format: compressed flat KV (e.g., snappy-compressed protobuf or custom binary).
No trie structure needed — just leaf data.

**Source**: iterate trie.db's Account, Code, Contract namespaces at tip height.

### Step 2: Incremental Sync — State Diffs per Block

Every block, the Delegate pushes the state changeset to agents (see State Diff Reliability section above for format).

**Source**: `workingset.go` already computes `flusher.SerializeQueue()` — this is exactly the ordered changeset we need. We serialize and broadcast it during `Commit()`.

This is the most critical integration point. The changeset already exists in memory during `Commit()`.

### Step 3: Pending Transactions

**Already implemented.** The coordinator polls actpool and pushes tasks to agents via server-side gRPC streaming (`GetTasks`).

- **L4**: same as today — coordinator pushes individual txs, agent validates with full state
- **L5**: coordinator pushes the full pending tx set + delegate-specified tx ordering

### Step 4 (L5 only): Agent Builds Candidate Block

Agent receives pending txs + has up-to-date state → builds block:

1. Execute transactions in delegate-specified order (Phase 1, no agent reordering)
2. Execute each tx against local state (same logic as `pickAndRunActions()` in `workingset.go`)
3. Collect receipts, gas usage, logs
4. Compute `deltaStateDigest` = `hash(serializeQueue)`
5. Compute `receiptRoot`, `logsBloom`
6. Return candidate block to Delegate

```
candidate_block = {
    height:             uint64,
    txs:                [SealedEnvelope],      // in delegate-specified order
    receipts:           [Receipt],
    delta_state_digest: hash,
    receipt_root:       hash,
    logs_bloom:         bloom,
    gas_used:           uint64,
}
```

### Step 5: Delegate Verification

**Re-execute (Phase 1, conservative)**:

Delegate re-executes the candidate block's transactions against its own state:
- If `deltaStateDigest` matches → sign and broadcast
- If mismatch → reject, penalize agent

The benefit is parallelism — the delegate offloads the tx selection / ordering decision and the initial execution to agents, and only needs to verify the winner.

**Optimistic + Slash (Phase 2, future)**:

Delegate trusts the agent's block without re-execution:
- Agent must be staked (via RewardPool contract)
- If fraud is detected (by any network participant), agent is slashed
- Similar to Ethereum ePBS's unconditional payment model
- Opens the door to true proposer-builder separation

## Agent Assignment Model

**Not pure competition.** Pure competition (all agents build, only winner gets paid) wastes resources and contradicts IOSwarm's "everyone participates" model.

### Primary + Standby Model

```
Each block slot:
  1 primary builder  — assigned by coordinator (round-robin or stake-weighted)
  N standby builders — build in parallel as backup

  If primary submits valid block in time → primary gets full reward
  If primary fails (timeout / invalid) → best standby gets reward
  All participating agents (primary + standby) earn a base participation reward
```

### Reward Structure

```
Block building reward pool:
  ├── 70% → Primary builder (if successful)
  ├── 20% → Participation pool (split among all active agents proportional to uptime/quality)
  └── 10% → Best standby (insurance for primary failure)

If primary fails:
  ├── 70% → Best standby (promoted to winner)
  ├── 20% → Participation pool
  └── 10% → Penalty deducted from primary's stake
```

This ensures:
- Most agents earn steadily (participation pool) — no "lottery" effect
- Primary builder has strong incentive to perform
- Standby agents are compensated for their backup role
- Primary failure is penalized and recovered from

## MEV Policy

### Phase 1: No Agent Reordering (Current Target)

- Delegate specifies the transaction ordering via an **inclusion list**
- Agent executes transactions in the specified order — no reordering allowed
- This prevents front-running, sandwich attacks, and censorship by agents
- Agent's job is execution quality, not tx ordering

### Phase 2: Controlled Reordering (Future)

- Agent may propose alternative orderings within delegate-defined constraints
- Delegate provides a mandatory inclusion list (FOCIL-like): these txs MUST be included
- Agent can reorder non-mandatory txs for gas optimization
- Any MEV extracted is shared between agent and delegate per reward contract

IoTeX currently has minimal MEV opportunity (low DeFi activity). Phase 2 is not a priority.

## Implementation Phases

### Phase A: State Snapshot Export (Delegate side)

Add a state export function to `state/factory/statedb.go`:

```go
func (sdb *stateDB) ExportSnapshot(ctx context.Context) (*StateSnapshot, error) {
    // iterate Account, Code, Contract namespaces
    // serialize to flat KV format
    // compute post_state_hash
    // compress
}
```

Serve via new gRPC endpoint or HTTP download.

### Phase B: State Diff Broadcast (Delegate side)

Hook into `workingset.go`'s `Commit()` to capture the serialize queue:

```go
func (ws *workingSet) Commit(ctx context.Context, retention uint64) error {
    // ... existing commit logic ...

    // NEW: broadcast state diff to IOSwarm coordinator
    if ws.diffBroadcaster != nil {
        diff := ws.store.SerializeQueue()
        prevHash := ws.prevStateHash
        postHash := hash(localState)
        ws.diffBroadcaster.Broadcast(height, prevHash, postHash, diff)
    }
}
```

Coordinator buffers last ~100 diffs for agent catch-up, then forwards to all connected agents.

### Phase C: Agent State Store

Replace `MemStateDB` (map-based) with a proper embedded KV store.

**Recommended: Pebble** (Go-native, CockroachDB team, no CGo dependency).

Rationale:
- Pure Go — no CGo build complexity, cross-compilation friendly, keeps agent lightweight
- Handles GB-scale state comfortably
- LSM-tree with good read/write performance for our access pattern
- Avoid MDBX (CGo dependency makes the agent heavier and harder to distribute)

The agent state store must implement the same `StateManager` read interface that `evmstatedbadapter.go` uses:
- `State(obj, NamespaceOption, KeyOption)` — read account/code/storage
- `PutState(obj, NamespaceOption, KeyOption)` — write during execution
- `Snapshot() / Revert(id)` — EVM transaction rollback

### Phase D: Block Building Logic (Agent side) — L5 only

Port the core of `workingset.go`'s `pickAndRunActions()` to the agent:
- Transaction execution in delegate-specified order
- Sequential EVM execution (reuse `evmstatedbadapter.go`)
- Receipt collection, bloom filter, state digest computation
- Block gas limit enforcement

**Recommended approach**: import `iotex-core` as a Go module rather than re-implementing. The EVM execution path (`evmstatedbadapter.go`, `workingset.go`) contains extensive fork-specific logic, gas refund handling, and precompile contracts that would be error-prone to duplicate. The agent's `go.mod` adds `require github.com/iotexproject/iotex-core`, and the Pebble state store implements iotex-core's `StateManager` interface so the existing EVM adapter works directly.

### Phase E: Candidate Block Submission — L5 only

New gRPC endpoint:

```protobuf
service IOSwarm {
    // ... existing RPCs ...
    rpc SubmitCandidateBlock(CandidateBlock) returns (BlockAcceptance);
}
```

Primary agent submits candidate block. Standby agents submit backup candidates. Delegate selects and verifies.

## Transition Roadmap

```
NOW                        L4                              L5
 │                          │                               │
 │  ┌─────────────────┐    │  ┌──────────────────────┐    │  ┌─────────────────────┐
 │  │ Phase A:        │    │  │ L4 operational:      │    │  │ Phase D:            │
 │  │ Snapshot export │    │  │ Agent has full state  │    │  │ Block building      │
 │  │                 │    │  │ Per-tx EVM validation │    │  │ logic ported        │
 │  │ Phase B:        │    │  │ 100% L3 accuracy     │    │  │                     │
 │  │ State diff      │    │  │                      │    │  │ Phase E:            │
 │  │ broadcast       │    │  │ Shadow mode:         │    │  │ Candidate block     │
 │  │                 │    │  │ verify agent results  │    │  │ submission          │
 │  │ Phase C:        │    │  │ match delegate        │    │  │                     │
 │  │ Agent Pebble    │    │  │                      │    │  │ Primary + standby   │
 │  │ state store     │    │  │                      │    │  │ model active        │
 │  └─────────────────┘    │  └──────────────────────┘    │  └─────────────────────┘
 │                          │                               │
 ▼  Infrastructure          ▼  Prove accuracy               ▼  Prove block building
```

**L4 is the milestone that unlocks value**: agents own their state independently, no longer depending on coordinator-provided snapshots. L3 already achieves 100% accuracy via SimulateAccessList; L4 makes agents self-sufficient. L5 is an evolution on top of proven L4 infrastructure.

## Comparison with Current Architecture

| Aspect | L1-L3 (current) | L4 (stateful validation) | L5 (block building) |
|--------|-----------------|-------------------------|---------------------|
| Agent state | None / partial | Full hot state (~300MB-1.2GB) | Full hot state |
| Agent work | Validate 1 tx | Validate 1 tx with full EVM | Build entire block |
| Delegate work | Everything | Execute + verify agent results | Re-execute winner block |
| L3 accuracy | **100%** (SimulateAccessList) | **100%** (independent) | 100% |
| MEV | Delegate only | Delegate only | Delegate-controlled (Phase 1) |
| Trust model | Shadow comparison | Shadow comparison | Re-execute → Optimistic + slash |
| Agent cost | $5/mo VPS, <64MB RAM | $5-10/mo VPS, ~1-2 GB RAM | $10-20/mo VPS |

## Design Decisions (Resolved)

1. **Snapshot distribution**: HTTP download via coordinator gRPC endpoint (`DownloadSnapshot`). Snapshots are snappy-compressed flat KV (~300 MB–1.2 GB compressed). Regenerated on-demand when agents request; coordinator can cache the latest snapshot. BitTorrent/IPFS deferred to future optimization if agent count exceeds coordinator bandwidth.

2. **State hash computation**: `post_state_hash = SHA256(sorted KV pairs)`. Pebble's sorted iterator makes this natural — iterate all keys in order, feed into SHA256 streaming hash. Simple, deterministic, no additional data structures needed.

3. **Genesis config delivery**: Bundled with the state snapshot as a `chain_config` field (chain ID, fork heights, gas limits, coinbase). Updated via state diffs when protocol upgrades occur.

4. **iotex-core as library**: Yes — agent imports `iotex-core` as a Go module for EVM execution. Pebble state store implements `StateManager` interface as an adapter. This avoids re-implementing fork logic, precompiles, and gas accounting.

5. **Diff window sizing**: 100 blocks (~16 minutes at 10s block time). Each diff is small (typically <10 KB for IoTeX's current tx volume), so 100 diffs ≈ <1 MB memory. Beyond the window, agent must full re-snapshot.

## Implementation Plan

Concrete engineering plan building on existing ioSwarm infrastructure (coordinator, agent, reward pool — all tested and running on mainnet).

### Stage 0: L4 Infrastructure (~2-3 weeks)

Two repos change in parallel.

**Delegate side (`iotex-core/ioswarm/`)**:

| File | Operation | Description |
|------|-----------|-------------|
| `snapshot.go` | New | `ExportEVMSnapshot()` — iterate Account/Code/Contract namespaces, snappy compress, serve via gRPC |
| `statediff.go` | New | Hook `workingset.go` `Commit()`, capture `SerializeQueue()`, compute pre/post state hash, broadcast to coordinator |
| `diff_buffer.go` | New | Rolling buffer of last 100 diffs for agent catch-up |
| `proto/statediff.proto` | New | `StateDiff`, `StateSnapshot`, `DownloadSnapshotRequest` protobuf definitions |
| `grpc_handler.go` | Modify | Add `DownloadSnapshot` and `StreamStateDiffs` RPC endpoints |

Integration point: `workingset.go`'s `Commit()` already computes the ordered changeset via `flusher.SerializeQueue()`. The hook captures this before flushing to DB — zero performance overhead for the existing execution path.

**Agent side (`ioswarm-agent/`)**:

| File | Operation | Description |
|------|-----------|-------------|
| `statestore.go` | New | Pebble KV wrapper — Get/Put/Delete/Iterator, implements `StateManager` interface |
| `statesync.go` | New | Download snapshot, apply diffs, state hash verification, auto-resync on divergence |
| `go.mod` | Modify | Add `github.com/cockroachdb/pebble` dependency |

Agent startup flow:
1. Check local Pebble DB exists and has state
2. No → call `DownloadSnapshot`, load into Pebble (takes ~1 min for 1 GB)
3. Yes → compare local height vs coordinator tip, request missing diffs
4. Subscribe to `StreamStateDiffs`, apply each diff + verify `post_state_hash`
5. On hash mismatch → automatic re-snapshot

**Acceptance criteria**: Agent downloads snapshot, syncs diffs for 24 hours without divergence. Kill and restart agent — resumes from local Pebble state without re-downloading.

### Stage 1: L4 Stateful Validation (~2 weeks)

Minimal change — swap the EVM state source.

Current L3: `evm.go` reads from `MemStateDB` populated by coordinator per-task snapshots (state prefetched via `SimulateAccessList` → 100% accuracy).

L4: `evm.go` reads from local Pebble state store (full chain state → 100% accuracy).

| File | Operation | Description |
|------|-----------|-------------|
| `validator.go` | Modify | Add `ValidateL4()` — same as L3 but reads local Pebble state instead of task-provided snapshots |
| `stateadapter.go` | New | Adapter: Pebble KV → iotex-core `StateManager` interface → `evmstatedbadapter.go` |
| `main.go` | Modify | Add `--level=L4` flag, require `--datadir` for Pebble path |

**Key validation**: Shadow mode comparison over 1000+ blocks. Agent L4 execution results must match delegate execution exactly (same `deltaStateDigest`). Target: ≥99.9% accuracy. Any mismatch indicates a state sync bug that must be fixed before proceeding.

**This is the critical milestone.** L4 accuracy at 100% proves the entire state sync infrastructure works and agent execution results can be trusted.

### Stage 2: L4 Trusted Execution Cache (~1-2 weeks)

Agent results feed back into delegate block production.

| File | Operation | Description |
|------|-----------|-------------|
| `iotex-core/ioswarm/execache.go` | New | LRU cache of agent execution results keyed by tx hash |
| `iotex-core/state/factory/workingset.go` | Modify | `runAction()` checks execution cache before EVM execution; cache hit with matching preconditions → skip re-execution |
| `iotex-core/ioswarm/coordinator.go` | Modify | Forward L4 agent results to execution cache |

Cache hit conditions:
- `txHash` exists in cache
- Sender nonce matches current state
- Sender balance sufficient
- If all match → apply agent's state changes directly, skip EVM

**Acceptance criteria**: Cache hit rate >90% under normal load. Delegate block production time decreases measurably. Disabling ioSwarm falls back cleanly to full execution (no impact on block production).

### Stage 3: L5 Block Building (~3-4 weeks)

**3a: Agent Block Builder**

| File | Operation | Description |
|------|-----------|-------------|
| `ioswarm-agent/blockbuilder.go` | New | Import iotex-core's `pickAndRunActions()` logic, execute txs in delegate-specified order, compute deltaStateDigest/receiptRoot/logsBloom |
| `ioswarm-agent/stateadapter.go` | Modify | Add Snapshot/Revert support for EVM tx rollback during block building |
| `ioswarm-agent/main.go` | Modify | Add `--level=L5` mode, block building loop |

**3b: Coordinator Scheduling**

| File | Operation | Description |
|------|-----------|-------------|
| `iotex-core/ioswarm/builder_scheduler.go` | New | Primary/standby assignment (round-robin by block height % agent count) |
| `iotex-core/ioswarm/candidate_pool.go` | New | Collect candidate blocks from agents, select best, timeout handling |
| `iotex-core/ioswarm/proto/builder.proto` | New | `SubmitCandidateBlock`, `BlockAcceptance` protobuf definitions |
| `iotex-core/ioswarm/grpc_handler.go` | Modify | Add `SubmitCandidateBlock` RPC |

**3c: Delegate Verification**

Delegate re-executes the winning candidate block, compares `deltaStateDigest`. Match → sign and broadcast. Mismatch → reject + penalize agent, fall back to standby.

**Acceptance criteria**: Primary agent builds block whose `deltaStateDigest` matches delegate re-execution 100%. Primary timeout → standby takes over within 3 seconds. Reward distribution: 70/20/10 split verified on-chain.

### Timeline Summary

```
Week 1-3      Week 4-5       Week 6-7       Week 8-11
Stage 0        Stage 1        Stage 2        Stage 3
Infrastructure L4 Validation  L4 Cache       L5 Block Building
─────────────►─────────────►─────────────►─────────────►
Snapshot       100% accuracy  Delegate uses   Full ePBS
StateDiff      Shadow proof   agent results   Primary+Standby
Pebble store                  for execution
```

Each stage delivers independently: Stage 0 gives agents state access (useful beyond validation). Stage 1 proves 100% accuracy. Stage 2 reduces delegate CPU load. Stage 3 is full proposer-builder separation.

## References

- [EIP-7732: Enshrined Proposer-Builder Separation](https://eips.ethereum.org/EIPS/eip-7732)
- [EIP-7805: FOCIL (Fork-Choice Enforced Inclusion Lists)](https://eips.ethereum.org/EIPS/eip-7805)
- [Ethereum PBS Roadmap](https://ethereum.org/roadmap/pbs)
- [IoTeX State Architecture: trie.db namespaces](../state/factory/)
- [IOSwarm Coordinator README](./README.md)
