[![Go version](https://img.shields.io/badge/go-1.9.2-blue.svg)](https://github.com/moovweb/gvm)
[![CircleCI](https://circleci.com/gh/iotexproject/iotex-core.svg?style=svg&circle-token=fe0817d127f251a34b8bdd3336a808c7537e5ec0)](https://circleci.com/gh/iotexproject/iotex-core)

# iotex-core
Welcome to the official Go implementation of IoTeX protocol! IoTeX is building the next generation of the decentralized 
network for IoT powered by scalability- and privacy-centric blockchains. Please refer to 
IoTeX [whitepaper](https://iotex.io/white-paper) for details.

Currently, This repo is of alpha-quality with limited features supported and it is subjected to rapid change. Please 
contact us if you intend to run it in production.

## System Components and Flowchart
![systemflowchart](https://user-images.githubusercontent.com/15241597/38832065-3e57ca3a-4176-11e8-9bff-110387cf2378.png)

## Feature List
### Testnet Preview (codename: StoneVan)
1. TBC (Transactions, Block & Chain)
* Bech32-encoded address
* Serialization and deserialize of messages on the wire
* Merkle tree
* Transactions, blocks and chain
* Transaction pool
* Fast and reliable blockchain storage and query using BoltDB
* Block sync from network peers
* Basic framework for script and VM
2. Network
* Efficient gossip protocol over TLS
* Broadcast & unicast semantics
* Seeding through network config
* Rate-limit requests per connection
* Peer discovery
* Large-scale simulation and load test
3. Consensus
* Framework for plugable consensus 
* Standalone and NOOP schemes
* Basic implementation of R-DPoS scheme
4. Clients
* Initial RPC support
* Tools for injecting transactions/blocks
5. Testing \& Integration
* Unit test coverage > 50%
* Integration tests
* Staging development to 50 nodes (for internal use only)

### Testnet Alpha \& Beta
* libsect283k1 and integration
* Random beacon and full R-DPoS with voting support
* Lightweight stealth address
* Cross Chain Communication (CCC)
* Fast block sync and checkpointing
* Script and VM
* Full explorer and wallet supporting Hierarchical Deterministic (HD) addresses
* SPV clients
* Seeding through IPFS and version negotiation
* Pluggable transportation framework w/ UDP + TCP support
* Peer metrics
* Unit test coverage > 70%
* e2e demo among 500-1000 peers
* Enhancement of existing features
* And much more ...


## Dev Tips
## Minimum requirements

| Components | Version | Description |
|----------|-------------|-------------|
|[Golang](https://golang.org) | >= 1.9.2| The Go Programming Language |
|[Glide](https://github.com/Masterminds/glide) | >= 0.13.0 | Glide is a dependency management tool for Go |

### Setup Dev Environment
```
mkdir -p ~/go/src/github.com/iotexproject
cd ~/go/src/github.com/iotexproject
git clone git@github.com:iotexproject/iotex-core.git
cd iotex-core
```

```glide install```

```make fmt; make build```

#### Setup Precommit Hook

Install git hook tools from [precommit hook](https://pre-commit.com/) first and then

```pre-commit install```

### Run Unit Tests
```make test```

### Run 
```make run```
You will see log message output like:
```
W0416 12:52:15.652394    1576 blocksync.go:220] ====== receive tip block 116
W0416 12:52:15.653014    1576 blocksync.go:276] ------ [127.0.0.1:4689] receive block 116 in 616.939µs
W0416 12:52:15.653967    1576 blocksync.go:293] ------ commit block 116 time = 923.364µs

W0416 12:52:16.648667    1576 blocksync.go:276] ------ [127.0.0.1:4689] receive block 117 in 994.656929ms
W0416 12:52:16.649745    1576 blocksync.go:293] ------ commit block 117 time = 1.029462ms

W0416 12:52:17.648360    1576 blocksync.go:276] ------ [127.0.0.1:4689] receive block 118 in 998.560518ms
W0416 12:52:17.649604    1576 blocksync.go:293] ------ commit block 118 time = 1.186833ms

W0416 12:52:18.648309    1576 blocksync.go:276] ------ [127.0.0.1:4689] receive block 119 in 998.658293ms
W0416 12:52:18.649432    1576 blocksync.go:293] ------ commit block 119 time = 1.075772ms

W0416 12:52:19.653393    1576 blocksync.go:276] ------ [127.0.0.1:4689] receive block 120 in 1.003920977s
W0416 12:52:19.654698    1576 blocksync.go:293] ------ commit block 120 time = 1.256924ms

W0416 12:52:20.649165    1576 blocksync.go:276] ------ [127.0.0.1:4689] receive block 121 in 994.424355ms
W0416 12:52:20.650254    1576 blocksync.go:293] ------ commit block 121 time = 1.0282ms

W0416 12:52:21.653420    1576 blocksync.go:276] ------ [127.0.0.1:4689] receive block 122 in 1.00312559s
W0416 12:52:21.654650    1576 blocksync.go:293] ------ commit block 122 time = 1.182673ms
```

# Deploy w/ Docker Image

```make docker```

## Contribution
We are glad to have contributors out of the core team; contributions, including (but not limited to) style/bug fixes, implementation of features, proposals of schemes/algorithms, and thorough documentation, are 
welcomed. Please refer to our [contribution guideline](https://github.com/iotexproject/iotex-core/blob/master/CONTRIBUTING.md) for more information.

## License
This project is licensed under the [Apache License 2.0](https://github.com/iotexproject/iotex-core/blob/master/LICENSE.md).
