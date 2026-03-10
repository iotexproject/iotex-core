# IOSwarm — Delegate AI Swarm for IoTeX

A delegate offloads transaction validation to a swarm of cheap agents ($5/mo VPS). Shadow mode on testnet first — prove agent results match iotex-core 100%, then progressively take on real work.

## Architecture

```
Delegate's iotex-core (fork)             $5 VPS (anywhere)
┌────────────────────────┐              ┌──────────────────┐
│  iotex-core             │    gRPC      │  ioswarm-agent   │
│  ├── actpool            │◄────────────│  - connect        │
│  ├── statedb            │   :14689    │  - verify sigs    │
│  └── ioswarm/           │             │  - verify state   │
│       coordinator ──────┼────────────►│  - return results │
│       prefetcher        │  streaming  │  - heartbeat 10s  │
│       scheduler         │             │  - LRU cache      │
│       shadow comparator │             │  - Prometheus     │
│       reward distributor│             └──────────────────┘
└────────────────────────┘                     × N agents
```

## Source Layout

### Coordinator (`iotex2026/ioswarm/`)

| File | Lines | Purpose |
|------|-------|---------|
| `config.go` | Config struct, defaults (port 14689, L2, shadow on) |
| `coordinator.go` | Main loop: poll actpool → prefetch → build tasks → dispatch |
| `registry.go` | Agent tracking: register, heartbeat, evict stale (sync.Once safe close) |
| `scheduler.go` | Round-robin dispatch to live agents via buffered channels |
| `prefetcher.go` | Concurrent batch state reads with channel-based collection (no race) |
| `shadow.go` | Compare agent results vs actual block execution, track accuracy |
| `reward.go` | Per-epoch reward distribution: delegate cut + proportional agent payouts |
| `grpc_handler.go` | gRPC service: Register, GetTasks (server-stream), SubmitResults, Heartbeat |
| `integration.go` | Docs + adapter stubs for wiring into iotex-core fork |
| `proto/ioswarm.proto` | gRPC service definition (protobuf) |
| `proto/types.go` | Go types matching proto (manual, no protoc needed for MVP) |
| `proto/codec.go` | JSON codec override for gRPC (replaces protobuf for MVP) |

### Agent (`ioswarm-agent/`)

| File | Purpose |
|------|---------|
| `main.go` | CLI entry: `--coordinator`, `--id`, `--level`, `--region`, `--metrics-port` |
| `client.go` | gRPC client: register → stream tasks → process → submit, auto-reconnect 5s |
| `worker.go` | L1 sig verify (ECDSA r/s bounds) + L2 state verify (nonce/balance) |
| `cache.go` | Thread-safe LRU cache for progressive state accumulation |
| `metrics.go` | Prometheus: tasks, sig/state verify, batch latency, cache hits/misses |
| `Dockerfile` | Alpine-based, ~20MB image |

### Simulation (`ioswarm/cmd/sim/`)

| File | Purpose |
|------|---------|
| `main.go` | Full in-process simulation: mock actpool + statedb + N agents over real gRPC |

## Interfaces

Coordinator accesses iotex-core via two interfaces (in-memory, zero serialization):

```go
type ActPoolReader interface {
    PendingActions() []*PendingTx
    BlockHeight() uint64
}

type StateReader interface {
    AccountState(address string) (*pb.AccountSnapshot, error)
}
```

In iotex-core fork, implement via `ActPoolAdapter` and `StateReaderAdapter` (see `integration.go`).

## gRPC Protocol

```protobuf
service IOSwarm {
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc GetTasks(GetTasksRequest) returns (stream TaskBatch);   // server push
  rpc SubmitResults(BatchResult) returns (SubmitResponse);
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
}
```

Task levels:
- **L1**: Signature verification only (~1μs/tx)
- **L2**: + Nonce/balance checks (~2μs/tx)
- **L3**: + Full EVM execution (future)

## Reward Distribution

```
Every epoch (~1 hour, 360 blocks):

Delegate epoch reward (e.g. 800 IOTX)
    │
    ├── 10% → Delegate (operating costs)     = 80 IOTX
    │
    └── 90% → Agent pool                     = 720 IOTX
              │
              ├── weight = tasks × (accuracy ≥ 99.5% ? 1.2 : 1.0)
              ├── payout = pool × (my_weight / total_weight)
              └── min 10 tasks/epoch to qualify
```

Config:
```yaml
reward:
  delegateCutPct: 10        # delegate keeps 10%
  epochBlocks: 360          # 1 hour at 10s blocks
  minTasksForReward: 10     # freeload protection
  bonusAccuracyPct: 99.5    # accuracy threshold for bonus
  bonusMultiplier: 1.2      # 20% bonus for high accuracy
```

## Simulation Results

### 10 Agents × 50 TPS (IoTeX mainnet-like)

```
Txs generated:   1,023
Txs validated:   850 (83% — last block didn't drain)
Effective TPS:   27
Avg latency:     <1μs/tx
Load balance:    78-99 tx/agent (even)
Agent errors:    0
Cost:            $50/mo = $0.70/M tx
```

### 10 Agents × 100 TPS (high load)

```
Txs validated:   1,845
Effective TPS:   60
Load balance:    180-196 tx/agent (±4%)
Agent errors:    0
Cost:            $50/mo = $0.32/M tx
```

### 100 Agents × 1000 TPS (stress test)

```
Txs validated:   20,475
Validation rate: 100%
Effective TPS:   660
Load balance:    180-230 tx/agent (±12%)
Agent errors:    0
Agent idle:      0 (all working)
Cost:            $500/mo = $0.29/M tx
Monthly volume:  1.7B tx
```

### Key Findings

1. **Linear scale**: 10→100 agents, 60→660 TPS
2. **Bottleneck is tx supply, not agents**: 100 TPS can't feed 100 agents; need 1000 TPS
3. **Sub-microsecond validation**: sig verify + state check in <2μs on commodity hardware
4. **Unit cost drops with scale**: $0.70 → $0.29/M tx
5. **IoTeX mainnet at ~10-30 TPS**: 10 agents is plenty; 100 agents for future DePIN growth

## Reward Economics (100 agents)

```
Delegate epoch income:   ~800 IOTX
Delegate keeps 10%:      80 IOTX
Agent pool 90%:          720 IOTX
Average per agent:       7.2 IOTX/epoch
Per agent per month:     ~5,184 IOTX  (720 epochs)
VPS cost:                $5/mo ≈ 100 IOTX
Agent net profit/month:  ~5,084 IOTX
```

Top earner (199 tasks, 100% accuracy): 8.35 IOTX/epoch
Bottom earner (150 tasks, 96.7% accuracy): 5.24 IOTX/epoch — still profitable.

## Tests

```
Coordinator:  22 tests (registry, prefetcher, scheduler, shadow, reward, integration E2E)
Agent:        11 tests (cache, worker L1/L2)
Total:        33 tests, all passing
```

Integration test covers full gRPC round-trip:
register → dispatch → stream tasks → validate → submit → heartbeat → shadow compare → 100% match.

## Running

```bash
# Build
cd ioswarm && go build ./...
cd ioswarm-agent && go build .

# Test
cd ioswarm && go test ./...
cd ioswarm-agent && go test ./...

# Simulation
cd ioswarm
go run ./cmd/sim --agents=10 --duration=30s --tps=50
go run ./cmd/sim --agents=100 --duration=30s --tps=1000

# Manual E2E (two terminals)
go run ./cmd/testcoord
./ioswarm-agent --coordinator=localhost:14689 --level=L2 --id=ant-1
```

## Integration into iotex-core

```go
// config/config.go
type Config struct {
    // ...
    IOSwarm ioswarm.Config `yaml:"ioswarm"`
}

// server/itx/server.go — newServer()
if cfg.IOSwarm.Enabled {
    cs := svr.rootChainService
    actPoolAdapter := &ioswarm.ActPoolAdapter{Pool: cs.ActPool(), BC: cs.Blockchain()}
    stateAdapter := &ioswarm.StateReaderAdapter{SF: cs.StateFactory()}
    svr.ioswarmCoord = ioswarm.NewCoordinator(cfg.IOSwarm, actPoolAdapter, stateAdapter)
}

// server/itx/server.go — Start()
if svr.ioswarmCoord != nil {
    svr.ioswarmCoord.Start(ctx)
}

// server/itx/server.go — Stop()
if svr.ioswarmCoord != nil {
    svr.ioswarmCoord.Stop()
}
```

Config YAML:
```yaml
ioswarm:
  enabled: true
  grpcPort: 14689
  maxAgents: 100
  taskLevel: "L2"
  shadowMode: true
  pollIntervalMs: 1000
```

## Agent Operator ("Lobster") Experience

### One-Line Deploy

```bash
docker run -d --name ioswarm-ant \
  -p 9090:9090 \
  iotex/ioswarm-agent \
  --coordinator=delegate.example.com:14689 \
  --id=ant-lobster-1 \
  --level=L2 \
  --wallet=io1abc123... \
  --webhook=https://hooks.slack.com/services/...
```

### Dashboard (port 9090)

| Endpoint | Purpose |
|----------|---------|
| `GET /` | ASCII status page (human-readable) |
| `GET /status` | JSON agent status |
| `GET /earnings` | JSON earnings & payout history |
| `GET /healthz` | Health check (200 if connected, 503 if not) |
| `GET /metrics` | Prometheus metrics |

### Webhook Notifications

If `--webhook` is set, the agent fires JSON webhooks on key events:

| Event | Trigger |
|-------|---------|
| `connected` | First connection to coordinator |
| `disconnected` | Lost connection |
| `reconnected` | Re-established after disconnect |
| `payout` | Epoch reward distributed |
| `error` | Significant error |

Payload example:
```json
{
  "type": "payout",
  "agent_id": "ant-lobster-1",
  "timestamp": "2026-01-15T12:00:00Z",
  "message": "Epoch 42 payout: 7.50 IOTX (rank #3/50)",
  "data": {
    "amount_iotx": 7.5,
    "rank": 3,
    "total_agents": 50,
    "epoch": 42
  }
}
```

### CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--coordinator` | `localhost:14689` | Coordinator gRPC address |
| `--id` | auto | Agent ID |
| `--level` | `L2` | Task level: L1, L2, L3 |
| `--region` | `local` | Region identifier |
| `--dashboard-port` | `9090` | Dashboard + metrics port (0 to disable) |
| `--cache-size` | `10000` | LRU cache size |
| `--webhook` | _(empty)_ | Webhook URL for notifications |
| `--wallet` | _(empty)_ | IOTX wallet for reward payouts |

## Coordinator Swarm API

The coordinator exposes HTTP endpoints for the delegate operator to monitor the swarm.

| Endpoint | Purpose |
|----------|---------|
| `GET /swarm/status` | Overall swarm status: agents, tasks dispatched, shadow accuracy |
| `GET /swarm/agents` | List all connected agents with stats |
| `GET /swarm/leaderboard` | Agents ranked by tasks processed this epoch |
| `GET /swarm/epoch` | Current epoch stats: tasks, accuracy, elapsed time |
| `GET /swarm/shadow` | Shadow mode comparison statistics |
| `GET /healthz` | Health check (200 if any agents live, 503 if none) |

## Distribution Strategy

### Docker Image

The agent is packaged as a single Docker image (~20MB):

```
iotex/ioswarm-agent:latest
```

Build:
```bash
cd ioswarm-agent
docker build -t iotex/ioswarm-agent .
docker push iotex/ioswarm-agent
```

### Distribution Channels

1. **Docker Hub** (`iotex/ioswarm-agent`) — primary distribution
2. **GitHub Releases** — pre-built binaries for linux/amd64, linux/arm64, darwin/amd64
3. **One-liner install script** — `curl -fsSL https://ioswarm.iotex.io/install.sh | sh`

### As a DePIN "Skill"

IOSwarm agent can be distributed as a **W3bstream skill** — a containerized micro-service that any DePIN node operator can opt-in to:

```yaml
# w3bstream skill manifest
name: ioswarm-agent
version: 1.0.0
image: iotex/ioswarm-agent:latest
ports:
  - 9090:9090
env:
  - COORDINATOR=delegate.example.com:14689
resources:
  cpu: 0.1
  memory: 64Mi
reward:
  token: IOTX
  estimated: "~5000 IOTX/month"
```

Key properties for skill distribution:
- **Self-contained**: single binary, no dependencies
- **Low resource**: <64MB RAM, <0.1 CPU
- **Observable**: Prometheus metrics, JSON API, webhook notifications
- **Profitable**: VPS cost $5/mo, earns ~5,000 IOTX/mo

## Tests

```
Coordinator:  29 tests (registry, prefetcher, scheduler, shadow, reward, swarm API, integration E2E)
Agent:        25 tests (cache, worker L1/L2, dashboard, earnings, notifier)
Total:        54 tests, all passing
```

## Next Steps

1. **Testnet shadow run**: Fork iotex-core, wire adapters, run on testnet with 3 agents
2. **Network latency simulation**: Add per-region delay in sim to model real VPS latency
3. **Agent fault injection**: Random disconnect, slow agents, malicious results
4. **RewardPool contract**: On-chain distribution so agents can claim directly
5. **L3 execution**: EVM execution offload (requires state trie sync)
6. **Multi-arch Docker build**: Support linux/arm64 for Raspberry Pi agents
7. **W3bstream skill integration**: Package as distributable skill for DePIN node operators
