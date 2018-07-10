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
### Testnet Alpha (codename: Strive)
1. TBC (Transactions, Block & Chain)
* Bech32-encoded address
* Serialization and deserialize of messages on the wire
* Merkle tree
* Actions, transfers, votes, blocks and chain
* Fast and reliable blockchain storage and query using BoltDB
* Block sync from network peers
* Basic framework for script and VM
* Account/state layer built on top of Merkle Patricia tree
* Voting and unvoting for block producer candidates
* New account-based action pool

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
* Full implementation of FSM-based Roll-DPoS
* Roll-DPoS simulator
* Initial implementation of random beacon

4. Clients
* JSON RPC support
* UI Design and backend implementation of explorer
* Command line console

5. Crypto
* libsect283 -- lightweight crypto library, with cgo binding
* libtblsmnt -- Intial BLS signature parameterization and implementation, with cgo binding

6. Testing \& Integration & Deployment
* Unit test coverage > 60%
* Thorough integration tests
* Dockerization of IoTeX server
* Testnet deployment of 20+ block producer delegates in production
* Action injection and address generation tools
* Enhancement of existing features, performance improvement and code refactoring

### Testnet Beta

* Random beacon, roll-DPoS and voting fully integration
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

### Setup Dev Environment
```
mkdir -p ~/go/src/github.com/iotexproject
cd ~/go/src/github.com/iotexproject
git clone git@github.com:iotexproject/iotex-core.git
cd iotex-core
```

```dep ensure```

```make fmt; make build```

~~#### Setup Precommit Hook~~

~~Install git hook tools from [precommit hook](https://pre-commit.com/) first and then~~

~~```pre-commit install```~~

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

Add `SKIP_DEP=true` to skip re-installing dependencies via `dep`.

## Dev Tools
### Use actioninjector to inject actions
Open one terminal window and run the command below to compile and start the test chain server with the configuration specified in "config_local_delegate.yaml" (This is optional, just in case you don't have a node running).

`make; make run`

Open a new terminal window and start running actioninjector.

`./bin/actioninjector`

You can use command line flags to customize the injector.

```
-config-path=path_of_config_file_of_genesis_transfer_addresses
-addr=target_address_for_jrpc_connection
-transfer-num=number_of_transfers
-vote-num=number_of_votes
-interval=sleeping_interval_in_seconds
-aps=actions_to_be_injected_per_second_APS_MODE_ONLY
-duration=duration_of_injector_running_in_seconds_APS_MODE_ONLY
```

Default flag values:
* config-path="./tools/actioninjector/gentsfaddrs.yaml"
* addr="127.0.0.1:14004"
* transfer-num=50
* vote-num=50
* interval=5
* aps=0
* duration=60

Note: APS is a running mode option which is disabled by default. When aps > 0, APS mode is enabled and the injector alternates to inject transfers and votes within the time frame specified by duration.

### Use addrgen to generate addresses
Open a terminal window and run the command below to start running addrgen.

`make build; ./bin/addrgen`

You can use command line flag to customize the address generator.

`-number=numer_of_addresses_to_be_generated`

Default flag value:
* number=10

### Use iotc to query the blockchain
Open a terminal window and run the command below to compile and start the test chain server with the configuration specified in "config_local_delegate.yaml" (This is optional, just in case you don't have a node running).

`make; make run`

Open a new terminal window and run iotc with the following command.

`./bin/iotc [commands] [flags]`

The following is the complete current usage of iotc. More commands will be added in the future.

```
Usage:
  iotc [command] [flags]

Available Commands:
  balance     Returns the current balance of given address
  details     Returns the details of given account
  height      Returns the current height of the blockchain
  help        Help about any command
  self        Returns this node's address
  transfers   Returns the transfers associated with a given address

Flags:
  -h, --help   help for iotc

Use "iotc [command] --help" for more information about a command.
```

## Contribution
We are glad to have contributors out of the core team; contributions, including (but not limited to) style/bug fixes, implementation of features, proposals of schemes/algorithms, and thorough documentation, are 
welcomed. Please refer to our [contribution guideline](https://github.com/iotexproject/iotex-core/blob/master/CONTRIBUTING.md) for more information.

## License
This project is licensed under the [Apache License 2.0](https://github.com/iotexproject/iotex-core/blob/master/LICENSE.md).
