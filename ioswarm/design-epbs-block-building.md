# IOSwarm ePBS — Agent Block Building Design

Agents evolve from transaction validators to block builders, following Ethereum's ePBS (EIP-7732) model adapted for IoTeX's DPoS architecture.

## Motivation

Current IOSwarm: agents validate individual transactions → Coordinator assembles block.
The L3 shadow accuracy is ~14% because agents lack sufficient state to execute EVM.

Instead of patching state delivery for per-tx validation, we shift to a fundamentally stronger model: **agents build entire blocks**, the delegate just signs and proposes.

## Role Mapping

| Ethereum ePBS | IoTeX IOSwarm | Responsibility |
|--------------|---------------|----------------|
| Proposer (Validator) | **Delegate** | Select best block, sign, broadcast, consensus |
| Builder (staked) | **Agent** | Select txs, order, execute, build candidate block |
| Relay | **Coordinator** | Route candidate blocks, validate, manage agents |
| Unconditional payment | **Agent reward / slash** | Agent staked via RewardPool contract |
| Inclusion list (FOCIL) | **Delegate-mandated txs** | Delegate can require specific txs be included |

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

### Phase 1: Cold Start — State Snapshot

Delegate exports a flat KV dump of the 3 required namespaces:

```
snapshot = {
    height:   uint64,
    accounts: map[address → Account],     // nonce, balance, codeHash
    code:     map[codeHash → bytecode],
    storage:  map[(address, slot) → value],
}
```

Format: compressed flat KV (e.g., snappy-compressed protobuf or custom binary).
No trie structure needed — just leaf data.

**Source**: iterate trie.db's Account, Code, Contract namespaces at tip height.

### Phase 2: Incremental Sync — State Diffs per Block

Every block, the Delegate pushes the state changeset to agents:

```
block_state_diff = {
    height:    uint64,
    prev_hash: hash,
    timestamp: time,
    gas_limit: uint64,
    base_fee:  big.Int,
    changes:   [(namespace, key, old_value, new_value)],  // ordered
}
```

**Source**: `workingset.go` already computes `flusher.SerializeQueue()` — this is exactly the ordered changeset we need. We just serialize and broadcast it before flushing to DB.

This is the most critical integration point. The changeset already exists in memory during `Commit()`.

### Phase 3: Pending Transactions

**Already implemented.** The coordinator polls actpool and pushes tasks to agents via server-side gRPC streaming (`GetTasks`). For block building, instead of sending individual tasks, we send the full pending tx set:

```
pending_txs = {
    txs: [SealedEnvelope],  // all pending, sorted by gas price
    block_context: {
        height, timestamp, gas_limit, base_fee, coinbase, prev_hash
    },
}
```

### Phase 4: Agent Builds Candidate Block

Agent receives pending txs + has up-to-date state → builds block:

1. Select and order transactions (gas price priority, MEV extraction)
2. Execute each tx against local state (same logic as `pickAndRunActions()` in `workingset.go`)
3. Collect receipts, gas usage, logs
4. Compute `deltaStateDigest` = `hash(serializeQueue)`
5. Compute `receiptRoot`, `logsBloom`
6. Return candidate block to Delegate

```
candidate_block = {
    height:            uint64,
    txs:               [SealedEnvelope],      // ordered
    receipts:          [Receipt],
    delta_state_digest: hash,
    receipt_root:       hash,
    logs_bloom:         bloom,
    gas_used:           uint64,
}
```

### Phase 5: Delegate Verification

**Phase 1 (conservative): Re-execute**

Delegate re-executes the candidate block's transactions against its own state:
- If `deltaStateDigest` matches → sign and broadcast
- If mismatch → reject, penalize agent

This is safe but the delegate still does full execution. The benefit is parallelism — multiple agents compete, delegate picks the best block (most gas revenue) and only re-executes the winner.

**Phase 2 (future): Optimistic + Slash**

Delegate trusts the agent's block without re-execution:
- Agent must be staked (via RewardPool contract)
- If fraud is detected (by any network participant), agent is slashed
- Similar to Ethereum ePBS's unconditional payment model

## Implementation Phases

### Phase A: State Snapshot Export (Delegate side)

Add a state export function to `state/factory/statedb.go`:

```go
func (sdb *stateDB) ExportSnapshot(ctx context.Context) (*StateSnapshot, error) {
    // iterate Account, Code, Contract namespaces
    // serialize to flat KV format
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
        ws.diffBroadcaster.Broadcast(height, diff)
    }
}
```

Coordinator forwards diffs to all connected agents.

### Phase C: Agent State Store

Replace `MemStateDB` (map-based) with a proper state store on the agent side:

Option A: **Embedded KV (Pebble/BoltDB)** — persistent, handles GB-scale state
Option B: **MDBX** — same engine as coordinator, best performance
Option C: **In-memory with mmap** — if state stays under ~1 GB

The agent state store must implement the same `StateManager` read interface that `evmstatedbadapter.go` uses:
- `State(obj, NamespaceOption, KeyOption)` — read account/code/storage
- `PutState(obj, NamespaceOption, KeyOption)` — write during execution
- `Snapshot() / Revert(id)` — EVM transaction rollback

### Phase D: Block Building Logic (Agent side)

Port the core of `workingset.go`'s `pickAndRunActions()` to the agent:
- Transaction selection by gas price (existing `actioniterator`)
- Sequential EVM execution (reuse `evmstatedbadapter.go`)
- Receipt collection, bloom filter, state digest computation
- Block gas limit enforcement

### Phase E: Candidate Block Submission

New gRPC endpoint:

```protobuf
service IOSwarm {
    // ... existing RPCs ...
    rpc SubmitCandidateBlock(CandidateBlock) returns (BlockAcceptance);
}
```

Multiple agents can submit candidates. Delegate picks the most profitable one (highest total gas revenue).

## Comparison with Current Architecture

| Aspect | Current (tx validation) | ePBS (block building) |
|--------|------------------------|----------------------|
| Agent state | None (stateless) | Full hot state (~300MB-1.2GB) |
| Agent work | Validate 1 tx | Build entire block |
| Delegate work | Select txs + execute + build block | Re-execute winner block |
| Parallelism | Per-tx (many agents, small tasks) | Per-block (agents compete) |
| MEV | Delegate captures all | Agents compete for MEV |
| L3 accuracy | ~14% (incomplete state) | 100% (full state) |
| Agent value | Cost reduction | Block building + MEV |
| Trust model | Shadow comparison | Re-execute → Optimistic + slash |

## Open Questions

1. **Snapshot distribution**: HTTP download? BitTorrent? IPFS? How often to regenerate full snapshots vs rely on diffs?
2. **State diff reliability**: What if agent misses a diff? Need catch-up mechanism (request missing diffs, or re-snapshot).
3. **Multiple competing agents**: How many agents compete per block? How does delegate select winner?
4. **MEV policy**: Is MEV extraction by agents desired? Should delegate enforce an ordering policy?
5. **Inclusion guarantees**: Should delegate provide a mandatory inclusion list (like FOCIL) to prevent agent censorship?
6. **Genesis config**: Agents need chain config (chain ID, fork heights, gas limits). Distribute as part of snapshot or separate config endpoint?

## References

- [EIP-7732: Enshrined Proposer-Builder Separation](https://eips.ethereum.org/EIPS/eip-7732)
- [EIP-7805: FOCIL (Fork-Choice Enforced Inclusion Lists)](https://eips.ethereum.org/EIPS/eip-7805)
- [Ethereum PBS Roadmap](https://ethereum.org/roadmap/pbs)
- [IoTeX State Architecture: trie.db namespaces](../state/factory/)
- [IOSwarm Current Architecture](./doc.md)
