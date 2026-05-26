# iotex-core 

Official Golang implementation of the IoTeX protocol, the modular DePIN Layer-1 network.

[![Join the forum](https://img.shields.io/badge/Discuss-IoTeX%20Community-blue)](https://community.iotex.io/c/research-development/protocol)
[![Go version](https://img.shields.io/badge/go-1.18.5-blue.svg)](https://github.com/moovweb/gvm)
[![Go Report Card](https://goreportcard.com/badge/github.com/iotexproject/iotex-core)](https://goreportcard.com/report/github.com/iotexproject/iotex-core)
[![Coverage](https://codecov.io/gh/iotexproject/iotex-core/branch/master/graph/badge.svg)](https://codecov.io/gh/iotexproject/iotex-core)
[![Godoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](https://godoc.org/github.com/iotexproject/iotex-core)
[![Releases](https://img.shields.io/github/release/iotexproject/iotex-core/all.svg?style=flat-square)](https://github.com/iotexproject/iotex-core/releases)
[![LICENSE](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

<a href="https://iotex.io/"><img src="logo/IoTeX.png" height="200px"/></a>

<a href="https://iotex.io/devdiscord" target="_blank">
  <img src="https://github.com/iotexproject/halogrants/blob/880eea4af074b082a75608c7376bd7a8eaa1ac21/img/btn-discord.svg" height="36px">
</a>

## What is IoTeX?

IoTeX is the modular infrastructure for DePIN projects to deploy in full or integrate modules into existing frameworks. Please visit [IoTeX homepage](https://iotex.io) official website to learn more about IoTeX network.

## What is DePIN?

DePIN stands for Decentralized Physical Infrastructure Networks, a new approach to building and maintaining physical world infrastructure. This infrastructure can range from WiFi hotspots in wireless networks to solar-powered home batteries in energy networks. DePINs are developed in a decentralized manner by individuals and companies globally, making them accessible to everyone. In return, contributors receive financial compensation and an ownership stake in the network they’re building and the services they provide through token incentives. DePINs are enabled by widespread internet connectivity and advancements in blockchain infrastructure and cryptography. To learn more about DePIN, please visit [What is DePIN?](https://iotex.io/blog/what-are-decentralized-physical-infrastructure-networks-depin/).

### Explore DePIN Projects?

[DePIN Scan](https://depinscan.io/)  is the go-to explorer for DePIN projects. DePIN Scan tracks crypto token prices, real-time device data, and offers a variety of views for DePIN projects.

## Run a delegate?

Please visit [IoTeX Delegate Manual](https://github.com/iotexproject/iotex-bootstrap) for detailed setup process.

## Building the source code

### Minimum requirements

| Components | Version | Description |
|----------|-------------|-------------|
| [Golang](https://golang.org) | &ge; 1.22.12 | Go programming language |
| [Protoc](https://developers.google.com/protocol-buffers/) | &ge; 3.6.0 | Protocol buffers, required only when you rebuild protobuf messages |

### Compile

Download the code to your desired local location (doesn't have to be under `$GOPATH/src`)
```
git clone git@github.com:iotexproject/iotex-core.git
cd iotex-core
```

If you put the project code under your `$GOPATH\src`, you will need to set up an environment variable
```
export GO111MODULE=on
set GO111MODULE=on (for windows)
```

Build the project for general purpose (server, ioctl) by

```
make
```

Build the project for broader purpose (server, ioctl, injector...) by
```
make all 
```

If the dependency needs to be updated, run

```
go get -u
go mod tidy
```
If you want to learn more advanced usage about `go mod`, you can find out [here](https://github.com/golang/go/wiki/Modules).

Run unit tests only by

```
make test
```

Build the docker image by

```
make docker
```

### Run iotex-core

Start (or resume) a standalone server to operate on a blockchain by

```
make run
```

Restart the server from a clean state by

```
make reboot
```

If "make run" fails due to corrupted or missing state database while block database is in normal condition, e.g.,
failing to get factory's height from underlying DB, please try to recover state database by

```
make recover
```

Then, "make run" again.

### Use CLI

Users could interact with iotex blockchain by

```
ioctl [command]
```

Refer to [CLI document](https://docs.iotex.io/developer/ioctl/install.html) for more details.

## IOSwarm: Distributed Transaction Validation

> **Branch:** `ioswarm-v2.3.5` | **Docker:** `raullen/iotex-core:ioswarm-v14`

IOSwarm adds an optional coordinator to iotex-core that dispatches pending transaction validation tasks to external agents over gRPC. It runs in **shadow mode** — observational only, with zero impact on consensus or block production. Supports L1–L4 validation levels, including full EVM re-execution (L3) and stateful local validation (L4).

### How it works

```
Delegate Node (iotex-core + IOSwarm coordinator)
  ├── actpool → pending txs
  ├── stateDB → account state prefetch + SimulateAccessList (L3)
  ├── gRPC :14689 → dispatch tasks to agents
  ├── HTTP :14690 → monitoring API (/api/stats, /swarm/shadow)
  └── StateDiff stream → real-time state sync for L4 agents

Agent Swarm (external processes)
  ├── L1: signature verify
  ├── L2: + nonce/balance checks
  ├── L3: + full EVM execution (100% shadow accuracy)
  ├── L4: + local state (BoltDB), independent validation
  └── Earn IOTX rewards per epoch (on-chain settlement)
```

### Quick Deploy (existing delegate)

See [ioswarm/README.md](ioswarm/README.md) for the full delegate onboarding guide.

**1. Pull the image:**
```bash
docker pull raullen/iotex-core:ioswarm-v14
```

**2. Add IOSwarm config to your `config.yaml`:**
```yaml
ioswarm:
  enabled: true
  grpcPort: 14689
  swarmApiPort: 14690
  maxAgents: 100
  taskLevel: "L4"
  shadowMode: true
  pollIntervalMs: 1000
  masterSecret: "<your-secret>"
  epochRewardIOTX: 0.5
  diffStoreEnabled: true
  diffStorePath: "/var/data/statediffs.db"
  rewardContract: "0x236CBF52125E68Db8fA88b893CcaFB2EE542F2d9"
  rewardSignerKey: "<hex-key>"
  reward:
    delegateCutPct: 10
    epochBlocks: 1
    minTasksForReward: 1
    bonusAccuracyPct: 99.5
    bonusMultiplier: 1.2
```

**3. Restart delegate with extra ports:**
```bash
docker stop iotex && docker rm iotex
docker run -d --restart on-failure --name iotex \
  -p 4689:4689 -p 14014:14014 \
  -p 127.0.0.1:14689:14689 -p 127.0.0.1:14690:14690 \
  -v=$IOTEX_HOME/data:/var/data:rw \
  -v=$IOTEX_HOME/log:/var/log:rw \
  -v=$IOTEX_HOME/etc/config.yaml:/etc/iotex/config_override.yaml:ro \
  -v=$IOTEX_HOME/etc/genesis.yaml:/etc/iotex/genesis.yaml:ro \
  raullen/iotex-core:ioswarm-v14 \
  iotex-server \
  -config-path=/etc/iotex/config_override.yaml \
  -genesis-path=/etc/iotex/genesis.yaml
```

> **Note:** Bind gRPC/HTTP to localhost and use a reverse proxy (nginx) for TLS termination. See [ioswarm/README.md](ioswarm/README.md) for TLS setup.

**4. Connect an agent:**
```bash
# Generate agent key
go run ./ioswarm/cmd/keygen --master=<your-secret> --agent=agent-01

# Start agent
./ioswarm-agent \
  --coordinator=<delegate-ip>:14689 \
  --agent-id=agent-01 \
  --api-key=<derived-key> \
  --level=L2
```

**5. Monitor:**
```bash
# Logs
docker logs -f iotex | grep -i ioswarm

# Status API
curl http://localhost:14690/swarm/status
curl http://localhost:14690/swarm/agents
curl http://localhost:14690/swarm/shadow
```

### Safety guarantees

- **Default off:** `ioswarm.enabled` defaults to `false` — zero impact unless explicitly enabled
- **Non-fatal:** If IOSwarm fails to start (port conflict, etc.), the delegate logs a warning and continues normally
- **Panic-isolated:** All IOSwarm goroutines have `defer recover()` — a panic in IOSwarm cannot crash the node
- **Graceful shutdown:** gRPC stop has a 5s timeout to prevent blocking node shutdown
- **Shadow mode:** Agent results are compared but never influence block production

### What changed vs v2.3.5

The IOSwarm integration touches only 2 existing files:

| File | Change |
|------|--------|
| `config/config.go` | +3 lines: `IOSwarm` field in Config struct |
| `server/itx/server.go` | +21 lines: conditional Start/Stop of coordinator |

Everything else is in the new `ioswarm/` directory. When `ioswarm.enabled: false` (default), zero new code paths execute.

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=iotexproject/iotex-core&type=Date)](https://star-history.com/#iotexproject/iotex-core&Date)

## Contact

- Mailing list: [iotex-dev](iotex-dev@iotex.io)
- Dev Forum: [forum](https://community.iotex.io/c/research-development/protocol)
- Bugs: [issues](https://github.com/iotexproject/iotex-core/issues)

## Contribution
We are glad to have contributors out of the core team; contributions, including (but not limited to) style/bug fixes,
implementation of features, proposals of schemes/algorithms, and thorough documentation, are welcomed. Please refer to
our [contribution guideline](CONTRIBUTING.md) for more
information. Development guide documentation is [here](https://github.com/iotexproject/iotex-core/wiki/Developers%27-Guide).

For any major protocol level changes, we use [IIP](https://github.com/iotexproject/iips) to track the proposal, decision
and etc.

## Contributors

Thank you for considering contributing to the IoTeX framework!

<a href="https://github.com/iotexproject/iotex-core/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=iotexproject/iotex-core" />
</a>

## License
This project is licensed under the [Apache License 2.0](LICENSE).
