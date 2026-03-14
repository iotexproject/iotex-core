# ioSwarm Coordinator

The ioSwarm coordinator runs as a module inside iotex-core on a delegate node. It distributes transaction validation to a swarm of agents over gRPC, compares agent results against on-chain execution (shadow mode), and settles rewards on-chain via the AgentRewardPool contract.

**IIP-58**: [ioSwarm — Distributed AI Execution Layer for IoTeX Delegates](https://github.com/iotexproject/iips/pull/64)

## Architecture

```
Delegate Machine
┌──────────────────────────────────────────────────┐
│  iotex-core (ioswarm-v2.3.5 branch)              │
│  ├── actpool         ← pending transactions      │
│  ├── statedb         ← account/contract state    │
│  └── ioswarm/        ← coordinator module        │
│       ├── Poll actpool for pending txs            │
│       ├── SimulateAccessList → prefetch state     │
│       ├── Build TaskPackages (tx + state)         │
│       ├── Dispatch to agents via gRPC streaming   │
│       ├── Collect results, shadow-compare         │
│       ├── Stream state diffs to L4 agents         │
│       └── On-chain reward settlement              │
│                         │                         │
│  gRPC :14689            │  HTTP :14690            │
└─────────────────────────┼─────────────────────────┘
                          │
          ┌───────────────┼───────────────┐
          │               │               │
       Agent-1         Agent-2         Agent-N
       L3 stateless    L4 stateful     anywhere
```

## Delegate Guide: Enable ioSwarm on Your Node

Any IoTeX delegate can opt-in to ioSwarm with zero consensus changes. Your delegate continues to sign blocks exactly as before — ioSwarm adds a sidecar that distributes execution work to agents and rewards them on-chain.

### Step 1 — Pull the ioSwarm-enabled image

```bash
docker pull raullen/iotex-core:ioswarm-v12
```

Or build from source (`ioswarm-v2.3.5` branch):
```bash
cd iotex-core && git checkout ioswarm-v2.3.5
docker buildx build --platform linux/amd64 -t your-image:tag .
```

### Step 2 — Add ioSwarm config to your `config.yaml`

```yaml
ioswarm:
  enabled: true
  grpcPort: 14689                # agent gRPC connections
  swarmApiPort: 14690            # HTTP monitoring API (0 to disable)
  maxAgents: 100
  taskLevel: "L4"                # L1, L2, L3, or L4
  shadowMode: true               # compare agent results vs on-chain (recommended)
  pollIntervalMs: 1000           # actpool poll interval
  masterSecret: "<your-secret>"  # HMAC master secret for agent auth (hex string)
  delegateAddress: ""            # delegate address (auto-detected if empty)
  epochRewardIOTX: 0.5           # IOTX reward per epoch

  # On-chain reward settlement (optional — omit to run without rewards)
  rewardContract: "0x236CBF52125E68Db8fA88b893CcaFB2EE542F2d9"
  rewardSignerKey: "<hex-key>"   # coordinator hot wallet private key (hex, no 0x prefix)

  # State diff for L4 agents
  diffStoreEnabled: true
  diffStorePath: "/var/data/statediffs.db"

  # Reward parameters
  reward:
    delegateCutPct: 10           # delegate keeps 10% of epoch reward
    epochBlocks: 1               # 1 block per epoch (= 10s at IoTeX 10s block time)
    minTasksForReward: 1         # minimum tasks to qualify for reward
    bonusAccuracyPct: 99.5       # accuracy threshold for bonus multiplier
    bonusMultiplier: 1.2         # weight multiplier for high-accuracy agents
```

**Config field reference:**

| Field | Required | Description |
|-------|----------|-------------|
| `masterSecret` | Yes | Hex string used to derive agent API keys via HMAC-SHA256 |
| `taskLevel` | Yes | Highest validation level to dispatch. `L4` enables state diff streaming |
| `rewardContract` | For rewards | AgentRewardPool v2 contract address on IoTeX mainnet |
| `rewardSignerKey` | For rewards | Private key of the coordinator hot wallet (hex, no 0x prefix). This wallet calls `depositAndSettle()` and must have IOTX balance |
| `diffStoreEnabled` | For L4 | Enable persistent diff storage so L4 agents can catch up after disconnect |
| `diffStorePath` | For L4 | Path to BoltDB file for diff storage |
| `epochBlocks` | Yes | Blocks per reward epoch. `1` = every block (10s), `3` = every 30s |
| `shadowMode` | Recommended | When true, compares agent results against on-chain receipts for accuracy tracking |

### Step 3 — Open port 14689 and restart

```bash
# Open gRPC port for agent connections
ufw allow 14689/tcp

# Restart delegate with ioSwarm ports exposed
docker run -d --name iotex \
  -p 4689:4689 -p 14014:14014 -p 14689:14689 -p 14690:14690 \
  -v /data:/var/data \
  -v /etc/iotex/config.yaml:/etc/iotex/config_override.yaml \
  raullen/iotex-core:ioswarm-v12

# Verify coordinator is running
curl http://localhost:14690/swarm/status
```

### Step 4 — Deploy AgentRewardPool contract (optional)

Skip this step if you use the shared mainnet contract (`0x236CBF52125E68Db8fA88b893CcaFB2EE542F2d9`).

To deploy your own:
```bash
./ioswarm-agent deploy \
  --private-key=<deployer-key> \
  --coordinator=0x<coordinator-hot-wallet>
```

Set the returned contract address in `rewardContract`.

### Step 5 — Export L4 snapshot (for L4 agents)

```bash
# Build the snapshot exporter
go build -o l4baseline ./ioswarm/cmd/l4baseline

# Export Account+Code snapshot (sufficient for L4, ~209 MB)
./l4baseline --source /data/trie.db --output acctcode.snap.gz --namespaces Account,Code

# Host it for agent download (CDN, object store, or file server)
```

### Step 6 — Generate agent API keys and onboard agents

Keys are HMAC-SHA256 derived from the master secret:
```
key = "iosw_" + hex(HMAC-SHA256(masterSecret, agentID))
```

Share the key + agent ID with each agent operator. They will use them to connect:
```bash
ioswarm-agent \
  --coordinator=<your-delegate-ip>:14689 \
  --agent-id=<agent-id> \
  --api-key=iosw_<key> \
  --wallet=<agent-reward-wallet>
```

See the [ioswarm-agent README](https://github.com/iotexproject/ioswarm-agent) for full agent setup instructions.

### Step 7 — Monitor your swarm

```bash
# Public stats (CORS-enabled, no auth — safe to expose to community dashboards)
curl http://localhost:14690/api/stats

# Swarm status (detailed)
curl http://localhost:14690/swarm/status

# Connected agents
curl http://localhost:14690/swarm/agents

# Shadow accuracy
curl http://localhost:14690/swarm/shadow

# Agent leaderboard
curl http://localhost:14690/swarm/leaderboard
```

### Public Stats API

`GET /api/stats` returns a JSON summary designed for community dashboards and landing pages. It requires no authentication and includes CORS headers (`Access-Control-Allow-Origin: *`).

```json
{
  "active_agents": 47,
  "agents_by_level": {"L3": 35, "L4": 12},
  "tasks_dispatched": 128473,
  "tasks_received": 128470,
  "shadow_accuracy": 99.87,
  "shadow_compared": 100000,
  "shadow_matched": 99876,
  "current_epoch": 24,
  "epoch_tasks": 4562,
  "epoch_accuracy": 99.83,
  "task_level": "L4",
  "uptime": "24h12m30s",
  "updated_at": 1710340200
}
```

Response is cached for 30 seconds (`Cache-Control: public, max-age=30`).

### What happens after you enable ioSwarm

1. **Zero risk**: Your delegate continues signing blocks exactly as before. ioSwarm runs in shadow mode — agent results are compared but never used for block production.
2. **Agents connect**: They receive transaction validation tasks via gRPC streaming.
3. **Rewards flow**: Every epoch (~10s), the coordinator calls `depositAndSettle()` on the AgentRewardPool contract, distributing IOTX to agents proportional to their work.
4. **Shadow accuracy**: You can monitor agent accuracy in real-time via `/swarm/shadow`. If all agents disappear, the delegate falls back to running solo.

---

## Validation Levels

| Level | What the agent does | State provided by coordinator |
|-------|--------------------|-----------------------------|
| **L1** | Signature verification | None |
| **L2** | + Nonce/balance checks | `AccountSnapshot` (sender, receiver) |
| **L3** | + Full EVM execution | Account snapshots + contract code + storage slots (via `SimulateAccessList`) |
| **L4** | Same as L3, but agent uses its own local state (BoltDB) | State diffs streamed in real-time |

### L3 State Prefetch: SimulateAccessList

For contract calls, the coordinator runs a **read-only EVM simulation** (similar to `eth_createAccessList`) to discover every storage slot the transaction will access. These slots are sent with the task so the agent has complete state for deterministic EVM execution.

**Mainnet result**: 230+ transactions, 100% shadow accuracy (agent results identical to on-chain execution).

Key implementation details:
- Uses `evm.SimulateAndCollectAccessList()` in `adapter.go`
- Requires full blockchain context (`bc.Context()`) including BaseFee, GetBlockHash, GetBlockTime
- All io1 addresses are converted to 0x hex format before sending to agents (go-ethereum only accepts 20-byte hex addresses)
- `BlockContext.Random` must be set (even to zero hash) for Shanghai/Cancun opcode support

### L4 State Sync

L4 agents maintain a full copy of IoTeX EVM state locally (BoltDB). This allows fully independent transaction validation without relying on coordinator-provided state.

**How it works:**

1. **Bootstrap**: Agent loads an IOSWSNAP snapshot (~209 MB for Account+Code, ~1.4 GB for full state including Contract trie nodes)
2. **State diffs**: After each block commit, coordinator captures the ordered changeset from `workingset.SerializeQueue()` and streams to agents via `StreamStateDiffs` gRPC
3. **Agent applies**: diffs to local BoltDB, maintaining state in sync with the delegate
4. **Recovery**: On disconnect, agent requests missing diffs from coordinator's DiffStore (last 1000 blocks retained)

**Production results** (mainnet, March 2026):
- Shadow accuracy: 99.5% (423/425 matched), mismatches due to state sync lag on nonce races
- Kill/restart recovery: <200ms from BoltDB state store, no snapshot reload needed
- Memory: ~679 MiB, CPU: ~24.5%
- State store: ~931 MB BoltDB after sync

**State data needed** (3 of 10+ namespaces):

| Namespace | Content | Size Estimate |
|-----------|---------|--------------|
| **Account** | All accounts: nonce, balance, codeHash | ~50-100 MB |
| **Code** | Contract bytecode, keyed by codeHash | ~50-100 MB |
| **Contract** | Contract storage (MPT trie nodes) | ~200 MB - 1 GB |

### IOSWSNAP Snapshot Format

Binary format for bootstrapping L4 agents:

```
header:  magic("IOSWSNAP") + version(uint32) + height(uint64)
entries: [marker(0x01) + ns_len(uint8) + ns + key_len(uint32) + key + val_len(uint32) + val]*
end:     marker(0x00)
trailer: count(uint64) + sha256(32 bytes) + end_magic("SNAPEND\0")
```

Export tool: `ioswarm/cmd/l4baseline` reads trie.db (BoltDB) and outputs gzip-compressed IOSWSNAP.

```bash
# Stats only
go run ./ioswarm/cmd/l4baseline --source trie.db --stats

# Export Account+Code only (~209 MB compressed)
go run ./ioswarm/cmd/l4baseline --source trie.db --output acctcode.snap.gz --namespaces Account,Code

# Export full baseline (~1.4 GB compressed)
go run ./ioswarm/cmd/l4baseline --source trie.db --output baseline.snap.gz
```

## On-Chain Reward Settlement

Every epoch (configurable, default 1 block = 10s), the coordinator:

1. Calculates each agent's weight: `tasks x (accuracy >= 99.5% ? 1.2 : 1.0)`
2. Calls `depositAndSettle(agents[], weights[])` on the AgentRewardPool contract, sending `epochReward x (1 - delegateCut)` as IOTX value
3. The contract uses F1 (cumulative reward-per-weight) algorithm for O(1) proportional distribution
4. Agents call `claim()` at any time to withdraw accumulated rewards

**AgentRewardPool v2 contract** (mainnet): [`0x236CBF52125E68Db8fA88b893CcaFB2EE542F2d9`](https://iotexscan.io/address/0x236CBF52125E68Db8fA88b893CcaFB2EE542F2d9)

Source in [`ioswarm-agent/contracts/`](https://github.com/iotexproject/ioswarm-agent/tree/main/contracts).

**E2E test results** (mainnet, March 2026):
- Single agent payout: PASS
- Multi-agent proportional split: PASS (50/50)
- 10 agents x 5 epochs: PASS (0.225 IOTX each)
- Dynamic join/leave: PASS (contract balance = 0 after all claims)
- MinTasks threshold: PASS (weight=0 gets 0)
- Delegate cut: PASS (10% retained, 90% to agents)

## gRPC Protocol

```protobuf
service IOSwarm {
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc GetTasks(GetTasksRequest) returns (stream TaskBatch);       // server push
  rpc SubmitResults(BatchResult) returns (SubmitResponse);
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
  rpc StreamStateDiffs(StateDiffRequest) returns (stream StateDiffBatch);  // L4
}
```

Agents connect via `Register`, authenticate with HMAC key, then open a server-streaming `GetTasks` call. Results flow back via `SubmitResults`. Epoch payouts are delivered in `HeartbeatResponse`.

L4 agents additionally open `StreamStateDiffs` to receive per-block state mutations. The coordinator's `StateDiffBroadcaster` fans out diffs from a ring buffer, and `DiffStore` persists recent diffs for catch-up.

## Integration with iotex-core

The coordinator is wired into iotex-core via `server/itx/server.go`:

```go
// In newServer():
if cfg.IOSwarm.Enabled {
    actPoolAdapter := ioswarm.NewActPoolAdapter(cs.ActPool(), cs.Blockchain())
    stateAdapter := ioswarm.NewStateReaderAdapter(cs.StateFactory(), cs.Blockchain(), cs.Genesis())
    svr.ioswarmCoord = ioswarm.NewCoordinator(cfg.IOSwarm, actPoolAdapter, stateAdapter)
}

// In Start():
if svr.ioswarmCoord != nil {
    svr.ioswarmCoord.Start(ctx)
}
```

Two interfaces decouple the coordinator from iotex-core internals:

```go
type ActPoolReader interface {
    PendingActions() []*PendingTx
    BlockHeight() uint64
}

type StateReader interface {
    AccountState(addr string) (*pb.AccountSnapshot, error)
    GetCode(addr string) ([]byte, error)
    GetStorageAt(addr, slot string) (string, error)
    SimulateAccessList(from, to string, data []byte, value string, gasLimit uint64) (map[string][]string, error)
}
```

State diff integration hooks into `stateDB.PutBlock()`:

```go
// In server/itx/server.go:
svr.ioswarmCoord.SetupStateDiffCallback(cs.StateFactory())
// Callback chain: stateDB.Finalize() → diffCallback → coordinator.ReceiveStateDiff()
//   → DiffStore.Append() + StateDiffBroadcaster.Publish()
//   → gRPC StreamStateDiffs to all connected L4 agents
```

## Source Layout

| File | Purpose |
|------|---------|
| `config.go` | Config struct with YAML tags, defaults |
| `coordinator.go` | Main loop: poll → prefetch → build tasks → dispatch → shadow compare |
| `adapter.go` | `ActPoolAdapter` + `StateReaderAdapter` (wraps iotex-core internals) |
| `prefetcher.go` | Concurrent state prefetch + `SimulateAccessList` for EVM access lists |
| `registry.go` | Agent lifecycle: register, heartbeat, evict stale, HMAC auth |
| `scheduler.go` | Round-robin dispatch to agents via buffered channels |
| `shadow.go` | Compare agent results vs actual block receipts, track accuracy |
| `grpc_handler.go` | gRPC service: Register, GetTasks (server-stream), SubmitResults, Heartbeat, StreamStateDiffs |
| `auth.go` | HMAC-SHA256 agent authentication |
| `swarm_api.go` | HTTP API for monitoring (`:14690`), including public `/api/stats` |
| `reward.go` | Epoch-based reward calculation (weight = tasks x accuracy bonus) |
| `reward_onchain.go` | On-chain `depositAndSettle()` to AgentRewardPool contract |
| `statediff.go` | State diff capture from stateDB commits and broadcasting to L4 agents |
| `diffstore.go` | Persistent diff storage (BoltDB) for agent catch-up |
| `integration.go` | Wiring docs + adapter interface definitions |
| `proto/` | gRPC service definition + Go types + codec |

### CLI Tools (`cmd/`)

| Tool | Usage | Purpose |
|------|-------|---------|
| `l4baseline` | `go run ./ioswarm/cmd/l4baseline --source trie.db --output snap.gz` | Export IOSWSNAP snapshot from trie.db for L4 agent bootstrap |
| `keygen` | `go run ./ioswarm/cmd/keygen --secret <master> --agent-id <id>` | Generate HMAC-SHA256 API key for an agent |
| `sim` | `go run ./ioswarm/cmd/sim --agents=10 --duration=30s` | L1-L3 simulation with synthetic workload |
| `l4sim` | `go run ./ioswarm/cmd/l4sim --agents=10 --duration=60s` | L4 multi-agent stress test (9/9 checks PASS, race-clean) |
| `l4test` | `go run ./ioswarm/cmd/l4test --coordinator=host:14689` | Live validation of StreamStateDiffs |

## L5 Roadmap: Agent Block Building (ePBS)

L4 stateful agents are the foundation for L5 block building, following Ethereum's ePBS ([EIP-7732](https://eips.ethereum.org/EIPS/eip-7732)) model adapted for IoTeX's DPoS architecture.

| Level | Capability | Status |
|-------|-----------|--------|
| L1-L3 | Per-tx validation (L3 = 100% via SimulateAccessList) | Production |
| **L4** | Per-tx EVM execution, full local state, independent validation | **Production** |
| L5 | Full block building (ePBS) | Design phase |

**Key advantage**: IoTeX's `deltaStateDigest` is a hash of the ordered state change queue (not a Merkle Trie root). Agents need only a flat KV store — no full MPT required. This makes block building feasible on commodity hardware.

For detailed L5 design, see [design-epbs-block-building.md](design-epbs-block-building.md).

## Related

| Repository | Description |
|------------|-------------|
| [ioswarm-agent](https://github.com/iotexproject/ioswarm-agent) | Agent binary (L1-L4 validation, claim/deploy/fund tools) |
| [ioswarm-portal](https://github.com/iotexproject/ioswarm-portal) | Dashboard and monitoring UI |
| [IIP-58](https://github.com/iotexproject/iips/pull/64) | ioSwarm protocol specification |
