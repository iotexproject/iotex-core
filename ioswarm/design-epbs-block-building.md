# IOSwarm ePBS — Agent Block Building Design

Agents evolve from transaction validators to block builders, following Ethereum's ePBS (EIP-7732) model adapted for IoTeX's DPoS architecture.

## Current Status (March 2026)

| Level | Capability | Status |
|-------|-----------|--------|
| L1-L3 | Per-tx validation, stateless (L3 = 100% via SimulateAccessList) | **Production** |
| **L4** | **Per-tx EVM execution, full local state, independent validation** | **Production** (99.5%→100% accuracy) |
| L5 | Full block building (ePBS) | **Design phase** (this document) |

### L4 Production Results (mainnet)

- Shadow accuracy: 100% (32/32 post-restart), 99.5% (423/425 over 24h soak test)
- Kill/restart recovery: <200ms from BoltDB state store
- State store: ~931 MB BoltDB after sync
- Memory: ~679 MiB, CPU: ~24.5%
- All 6 E2E reward tests PASS
- Snapshot server: `https://ts.iotex.me` (Cloudflare CDN)

### 0.5% Accuracy Gap — Root Cause

The 2 mismatches in 425 txs are caused by **nonce races during state sync lag**:

1. Agent receives state diff for block N
2. Before diff is applied, agent receives task for tx in block N+1
3. Agent reads stale nonce → produces different execution result
4. By the time shadow comparison runs, the correct state is available

**Fix approaches:**
- **Sequence barrier**: Agent must not process tasks for height H until state diffs for H-1 are fully applied
- **Nonce pre-check**: Compare task nonce vs local nonce; if local is behind, defer the task
- **Height-gated dispatch**: Coordinator only dispatches tasks for height H after broadcasting diffs for H-1

This is an engineering fix, not a fundamental limitation. Target: 100% accuracy over 10,000+ txs.

## L5 Design: Agent Block Building

### Architecture

```
                         ┌──────────────────────┐
                         │      Delegate         │
                         │    (DPoS Proposer)    │
                         │                       │
                         │  ┌─── actpool ──────┐ │
                         │  │ pending txs       │ │
                         │  └───────┬───────────┘ │
                         │          │              │
                         │   1. Send pending txs   │
                         │          │              │
                         └──────────┼──────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    │               │               │
                    ▼               ▼               ▼
             ┌──────────┐   ┌──────────┐   ┌──────────┐
             │ Agent A  │   │ Agent B  │   │ Agent C  │
             │ Primary  │   │ Standby  │   │ Standby  │
             │          │   │          │   │          │
             │ 2. Order │   │ 2. Order │   │ 2. Order │
             │ 3. Execute│  │ 3. Execute│  │ 3. Execute│
             │ 4. Build  │  │ 4. Build  │  │ 4. Build  │
             │   block   │  │   block   │  │   block   │
             └─────┬─────┘  └─────┬─────┘  └─────┬─────┘
                   │              │               │
                   │    5. Submit candidate blocks │
                   └──────────────┼───────────────┘
                                  │
                         ┌────────▼────────────────┐
                         │      Delegate            │
                         │  6. Verify (re-execute)  │
                         │  7. Sign + P2P broadcast │
                         └──────────────────────────┘
```

### Role Separation

| Component | Ethereum ePBS | IoTeX IOSwarm |
|-----------|--------------|---------------|
| Proposer | Validator | **Delegate** — has consensus authority, signs blocks, P2P broadcasts |
| Builder | Builder (staked) | **Agent** — has full state, executes txs, builds candidate blocks |
| Relay | MEV relay | **Coordinator** — routes txs and candidate blocks, manages agents |
| Tx source | Public mempool | **Delegate actpool** — delegate owns the tx source, agents don't need P2P |

### Key Design Decisions

**1. Delegate owns txpool, Agent builds blocks**

Delegate's actpool is the single source of pending transactions. Agents don't connect to P2P or maintain their own mempool. This simplifies agent design and prevents MEV extraction by agents in Phase 1.

**2. Agent can order transactions**

In Phase 1, delegate sends tx ordering hints. In Phase 2, agent may reorder within delegate-defined constraints. Agent's ordering power is the value they bring beyond pure execution.

**3. deltaStateDigest enables lightweight verification**

IoTeX block headers contain `deltaStateDigest` — hash of the ordered state change queue (NOT a Merkle trie root). Agent computes this after execution. Delegate re-executes and compares. Match → sign block. No need for full MPT — flat KV store is sufficient.

```
deltaStateDigest = hash(serializeQueue)  // ordered (namespace, key, value) changes
receiptRoot      = merkle(receipts)
logsBloom        = OR(tx_blooms)
```

All three are deterministic given the same tx ordering + state. L4 agents can already compute these.

## Block Building Flow (L5)

### Step 1: Delegate sends pending txs to agents

```protobuf
message BlockBuildRequest {
    uint64 height = 1;
    bytes  prev_block_hash = 2;
    repeated bytes sealed_actions = 3;  // pending txs from actpool
    BlockContext context = 4;           // timestamp, gas_limit, base_fee
    TxOrderHint order_hint = 5;         // Phase 1: mandatory order; Phase 2: constraints
}
```

Coordinator pushes `BlockBuildRequest` to primary + standby agents via gRPC streaming (extends existing `GetTasks`).

### Step 2: Agent orders + executes + builds

Agent receives pending txs + has up-to-date local state:

1. Order transactions (follow `order_hint` in Phase 1, optimize in Phase 2)
2. Execute each tx sequentially against local BoltDB state
3. Enforce block gas limit, skip txs that exceed it
4. Collect receipts, gas usage, logs
5. Compute `deltaStateDigest`, `receiptRoot`, `logsBloom`
6. Submit candidate block back to delegate

```protobuf
message CandidateBlock {
    uint64 height = 1;
    bytes  agent_id = 2;
    repeated bytes tx_hashes = 3;       // txs included, in order
    repeated bytes receipts = 4;
    bytes  delta_state_digest = 5;
    bytes  receipt_root = 6;
    bytes  logs_bloom = 7;
    uint64 gas_used = 8;
    bool   is_primary = 9;
}
```

### Step 3: Delegate verifies + signs + broadcasts

**Phase 1 (Re-execute, conservative):**

Delegate re-executes the candidate block's tx list in the same order:
- If `deltaStateDigest` matches → assemble full block, sign, broadcast via P2P
- If mismatch → reject, penalize agent, try standby's candidate
- If no valid candidate within timeout → delegate builds block itself (fallback)

Phase 1 doesn't reduce delegate computation (still re-executes), but proves the entire pipeline works.

**Phase 2 (Optimistic + slash):**

Delegate trusts the agent's execution without re-executing:
- Agent must be staked (via AgentRewardPool contract)
- If fraud detected post-facto (by any network participant) → slash agent stake
- Requires fraud proof mechanism (separate design)

## Agent Assignment Model

### Primary + Standby (not pure competition)

```
Each block slot:
  1 primary builder  — assigned by coordinator (round-robin or stake-weighted)
  N standby builders — build in parallel as backup

  Primary submits first → used if valid
  Primary fails (timeout/invalid) → best standby promoted
  All participating agents earn base participation reward
```

### Reward Structure

```
Block building reward per epoch:
  ├── 70% → Primary builder (if successful)
  ├── 20% → Participation pool (all active agents, proportional to uptime)
  └── 10% → Standby pool (best standby, insurance for primary failure)

If primary fails:
  ├── 70% → Best standby (promoted to winner)
  ├── 20% → Participation pool
  └── 10% → Penalty from primary's future rewards
```

### Timeout Handling

```
Block interval: 2.5s (Wake upgrade)
  T+0.0s  Coordinator sends BlockBuildRequest
  T+1.5s  Deadline for primary candidate
  T+2.0s  Deadline for standby candidates
  T+2.2s  Delegate selects winner, re-executes, signs
  T+2.5s  Block broadcast deadline
```

If no agent delivers → delegate builds block itself (same as today). IOSwarm failure is always non-fatal.

## MEV Policy

### Phase 1: Delegate-Controlled Ordering

- Delegate specifies tx ordering via `order_hint` (mandatory)
- Agent executes in specified order — no reordering
- Prevents front-running, sandwich attacks, censorship by agents
- Agent's value is execution speed, not tx ordering

### Phase 2: Agent Ordering with Constraints

- Agent may reorder non-mandatory txs
- Delegate provides inclusion list: these txs MUST be included
- MEV extracted is shared between agent and delegate per contract
- IoTeX currently has minimal MEV (low DeFi activity) — Phase 2 is not urgent

## Slash Conditions (L5 Core Work)

### What Gets Slashed

| Condition | Severity | Slash Amount |
|-----------|----------|-------------|
| deltaStateDigest mismatch | Critical | 100% of epoch stake |
| Missing mandatory txs (inclusion list violation) | High | 50% of epoch stake |
| Timeout (no candidate submitted) | Low | No slash, lose primary reward |
| Repeated timeouts (3+ consecutive) | Medium | Demoted from primary rotation |

### Fraud Proof Mechanism (Phase 2)

1. Any network participant can submit a fraud proof: "Block H built by Agent X has wrong deltaStateDigest"
2. Proof: re-execute block H txs against state at H-1, show different digest
3. On-chain verification: smart contract re-computes digest (or trusted committee verifies)
4. If fraud confirmed: slash agent's stake, reward fraud prover

**Open question:** On-chain re-execution is expensive. Options:
- Trusted committee of delegates verifies (practical for IoTeX's 24-delegate set)
- ZK proof of execution (future, expensive to generate)
- Optimistic rollup style: fraud proof window, slash if contested

### Agent Staking

To become a block builder (L5), agent must stake:
- Minimum stake: TBD (e.g., 10,000 IOTX)
- Staked via AgentRewardPool contract (extends existing contract)
- Stake locked during active builder period
- Unstake cooldown: 72h (match IoTeX staking withdrawal period)

## Implementation Phases

### Phase 0: Fix 0.5% Accuracy Gap (1-2 weeks) ← **CURRENT PRIORITY**

Target: 100% shadow accuracy over 10,000+ transactions.

| Task | File | Description |
|------|------|-------------|
| Sequence barrier | `coordinator.go` | Don't dispatch tasks for height H until state diffs for H-1 are broadcast |
| Height tracking | `grpc_handler.go` | Track per-agent synced height, gate task dispatch |
| Agent-side check | (ioswarm-agent) `statesync.go` | Reject tasks if local state height < task height - 1 |
| Extended soak test | — | Run L4 agent 72h+, verify 100% accuracy |

### Phase 1: L5 Block Building Protocol (3-4 weeks)

| Task | File | Description |
|------|------|-------------|
| BlockBuildRequest proto | `proto/ioswarm.proto` | New message types for block building |
| Builder scheduler | `builder_scheduler.go` | Primary/standby assignment per block slot |
| Candidate pool | `candidate_pool.go` | Collect and select candidate blocks |
| Agent block builder | (ioswarm-agent) `blockbuilder.go` | Execute txs, compute digests, submit candidate |
| Delegate verification | `coordinator.go` | Re-execute candidate, compare deltaStateDigest |
| SubmitCandidateBlock RPC | `grpc_handler.go` | New gRPC endpoint |

### Phase 2: Slash + Optimistic Execution (4-6 weeks)

| Task | File | Description |
|------|------|-------------|
| Agent staking contract | `contracts/` | Extend AgentRewardPool with stake/slash |
| Fraud proof design | `design-fraud-proofs.md` | Detailed fraud proof mechanism |
| Optimistic mode | `coordinator.go` | Skip re-execution when agent is staked |
| Slash execution | `reward_onchain.go` | On-chain slash via contract call |

### Timeline

```
NOW                    Phase 0              Phase 1              Phase 2
 │                      │                    │                    │
 │  Fix 0.5% gap       │  L5 protocol       │  Slash mechanism   │
 │  Sequence barrier    │  BlockBuildRequest  │  Agent staking     │
 │  100% accuracy       │  CandidateBlock     │  Fraud proofs      │
 │  Extended soak test  │  Primary/standby    │  Optimistic exec   │
 │                      │  Re-execute verify  │                    │
 │  1-2 weeks           │  3-4 weeks          │  4-6 weeks         │
 ▼                      ▼                    ▼                    ▼
L4 proven 100%         L5 conservative       L5 full ePBS
```

## gRPC Protocol Extensions

```protobuf
service IOSwarm {
    // Existing
    rpc Register(RegisterRequest) returns (RegisterResponse);
    rpc GetTasks(GetTasksRequest) returns (stream TaskBatch);
    rpc SubmitResults(BatchResult) returns (SubmitResponse);
    rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
    rpc StreamStateDiffs(StateDiffRequest) returns (stream StateDiffBatch);

    // L5 new
    rpc SubmitCandidateBlock(CandidateBlock) returns (BlockAcceptance);
}
```

## Comparison Across Levels

| Aspect | L1-L3 | L4 (current) | L5 Phase 1 | L5 Phase 2 |
|--------|-------|-------------|------------|------------|
| Agent state | None/partial | Full (BoltDB) | Full (BoltDB) | Full (BoltDB) |
| Agent work | Validate 1 tx | Validate 1 tx (full EVM) | Build entire block | Build entire block |
| Delegate work | Everything | Execute + shadow compare | Re-execute candidate | Trust agent (slash) |
| Accuracy | 100% (L3) | 99.5%→100% | 100% (required) | 100% (required) |
| MEV | Delegate only | Delegate only | Delegate-controlled | Shared |
| Trust model | Shadow | Shadow | Re-execute | Optimistic + slash |
| Agent cost | $5/mo | $5-10/mo | $10-20/mo | $10-20/mo |

## References

- [EIP-7732: Enshrined Proposer-Builder Separation](https://eips.ethereum.org/EIPS/eip-7732)
- [EIP-7805: FOCIL (Fork-Choice Enforced Inclusion Lists)](https://eips.ethereum.org/EIPS/eip-7805)
- [IOSwarm Coordinator README](./README.md)
- [IOSwarm Agent README](https://github.com/iotexproject/ioswarm-agent)
- [IIP-58: IOSwarm Protocol Specification](https://github.com/iotexproject/iips/pull/64)
