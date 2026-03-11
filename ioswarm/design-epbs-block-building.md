# IOSwarm ePBS — Agent Block Building Design

Agents evolve from transaction validators to block builders, following Ethereum's ePBS (EIP-7732) model adapted for IoTeX's DPoS architecture.

## Motivation

Current IOSwarm: agents validate individual transactions → Coordinator assembles block.
The L3 shadow accuracy is ~14% because agents lack sufficient state to execute EVM.

Instead of patching state delivery for per-tx validation, we introduce stateful agents and progressively evolve them toward full block building:

| Level | Capability | State Requirement |
|-------|-----------|-------------------|
| L1-L3 (current) | Per-tx validation, stateless | None / partial snapshots from coordinator |
| **L4 (next)** | **Per-tx EVM execution, full state, 100% accuracy** | **Full hot state via snapshot + diffs** |
| **L5 (future)** | **Full block building (ePBS)** | **Full hot state + mempool access** |

**L4 is the immediate target** — it solves the 14% accuracy problem with minimal architectural risk. L5 (block building) builds on the same state infrastructure.

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

Note: this requires porting significant logic from iotex-core. Evaluate whether the agent should import iotex-core as a library vs duplicating code.

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

**L4 is the milestone that unlocks value**: solving the 14% accuracy problem, proving agents can match delegate execution 100%. L5 is an evolution on top of proven L4 infrastructure.

## Comparison with Current Architecture

| Aspect | L1-L3 (current) | L4 (stateful validation) | L5 (block building) |
|--------|-----------------|-------------------------|---------------------|
| Agent state | None / partial | Full hot state (~300MB-1.2GB) | Full hot state |
| Agent work | Validate 1 tx | Validate 1 tx with full EVM | Build entire block |
| Delegate work | Everything | Execute + verify agent results | Re-execute winner block |
| L3 accuracy | ~14% | **100%** | 100% |
| MEV | Delegate only | Delegate only | Delegate-controlled (Phase 1) |
| Trust model | Shadow comparison | Shadow comparison | Re-execute → Optimistic + slash |
| Agent cost | $5/mo VPS, <64MB RAM | $5-10/mo VPS, ~1-2 GB RAM | $10-20/mo VPS |

## Open Questions

1. **Snapshot distribution**: HTTP download? BitTorrent? IPFS? How often to regenerate full snapshots vs rely on diffs?
2. **State hash computation**: What hash function for `post_state_hash`? Hash of sorted KV pairs? Or reuse `deltaStateDigest` chain?
3. **Genesis config delivery**: Agents need chain config (chain ID, fork heights, gas limits). Distribute as part of snapshot or separate config endpoint?
4. **iotex-core as library**: For Phase D, should the agent import iotex-core packages directly (EVM, state manager) rather than re-implementing?
5. **Diff window sizing**: How many blocks of diffs should coordinator buffer? Trade-off between memory and re-snapshot frequency.

## References

- [EIP-7732: Enshrined Proposer-Builder Separation](https://eips.ethereum.org/EIPS/eip-7732)
- [EIP-7805: FOCIL (Fork-Choice Enforced Inclusion Lists)](https://eips.ethereum.org/EIPS/eip-7805)
- [Ethereum PBS Roadmap](https://ethereum.org/roadmap/pbs)
- [IoTeX State Architecture: trie.db namespaces](../state/factory/)
- [IOSwarm Current Architecture](./doc.md)
