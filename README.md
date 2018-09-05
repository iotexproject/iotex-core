[![Go version](https://img.shields.io/badge/go-1.9.2-blue.svg)](https://github.com/moovweb/gvm)
[![CircleCI](https://circleci.com/gh/iotexproject/iotex-core.svg?style=svg&circle-token=fe0817d127f251a34b8bdd3336a808c7537e5ec0)](https://circleci.com/gh/iotexproject/iotex-core)

# iotex-core
Welcome to the official Go implementation of IoTeX protocol! IoTeX is building the next generation of the decentralized 
network for IoT powered by scalability- and privacy-centric blockchains. Please refer to 
IoTeX [whitepaper](https://iotex.io/white-paper) for details.

Currently, This repo is of alpha-quality with limited features supported and it is subjected to rapid change. Please 
contact us if you intend to run it in production.

## System Components and Flowchart
![systemflowchart_account](https://user-images.githubusercontent.com/15241597/44813900-e9717500-ab8f-11e8-8641-535c151c8df2.png)

## Feature List
### Testnet Beta (codename: Epik)
1. TBC (Transactions, Block & Chain)
* Bech32-encoded address
* Serialization and deserialize of messages on the wire
* Merkle tree
* Actions, transfers, votes, blocks and chain
* Fast and reliable blockchain/state storage via BoltDB and DB transaction support
* Improved block sync from network peers
* Basic framework for script and VM
* Account/state layer built on top of Merkle Patricia tree
* Voting and unvoting for block producer candidates
* New account-based action pool
* Initial implementation of secure keystore of private keys
* Initial integration with Ethereum Virtual Machine (EVM), smart contracts (Solidity)

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
* Full integration with delegates pool
* Roll-DPoS simulator
* Initial implementation of random beacon

4. Clients
* Full implementation of JSON RPC
* UI Design and backend implementation of explorer
* Command line console
* Basic wallet

5. Crypto
* libsect283 -- lightweight crypto library, with cgo binding
* libtblsmnt -- complete BLS signature parameterization and implementation, with cgo binding
* Implementation of distributed key generation (DKG) with cgo binding

6. Testing \& Integration & Deployment
* Improved action injection and address generation tools
* Work-preserving restart
* Dockerization of IoTeX server
* Large-scale testnet deployment via Kubernetes
* Unit test coverage ~70%
* Thorough integration tests
* Enhancement of existing features, performance and robustness

### Mainnet Preview

* Cross Chain Communication (CCC)
* Improving VM and smart contract
* Lightweight stealth address
* Fast block sync and checkpointing
* Full explorer and wallet supporting Hierarchical Deterministic (HD) addresses
* SPV clients
* Seeding through IPFS and version negotiation
* Pluggable transportation framework w/ UDP + TCP support
* Peer metrics
* e2e demo among 500-1000 peers
* Enhancement of existing features
* Performance and stability improvement
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

Install Go dependency management tool from [golang dep](https://github.com/golang/dep) first and then

```dep ensure --vendor-only```

```make fmt; make build```

~~#### Setup Precommit Hook~~

~~Install git hook tools from [precommit hook](https://pre-commit.com/) first and then~~

~~```pre-commit install```~~

### Run Unit Tests
```make test```

### Reboot
```make reboot``` reboots server from fresh database.

You will see log message output like:
```
2018-08-28T09:54:02-07:00 |INFO| commit a block height=0 iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T09:54:02-07:00 |INFO| Starting dispatcher iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T09:54:02-07:00 |INFO| Starting IotxConsensus scheme iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate scheme=STANDALONE
2018-08-28T09:54:02-07:00 |INFO| start RPC server on 127.0.0.1:4689 iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T09:54:02-07:00 |INFO| Starting Explorer JSON-RPC server on [::]:14004 iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T09:54:03-07:00 |INFO| No peer exist to sync with. iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T09:54:03-07:00 |INFO| created a new block at="2018-08-28 09:54:03.210120086 -0700 PDT m=+1.047466065" iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T09:54:03-07:00 |INFO| created a new block height=1 iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y length=1 networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T09:54:03-07:00 |INFO| commit a block height=1 iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T09:54:04-07:00 |INFO| No peer exist to sync with. iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T09:54:04-07:00 |INFO| created a new block at="2018-08-28 09:54:04.213299491 -0700 PDT m=+2.050689454" iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T09:54:04-07:00 |INFO| created a new block height=2 iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y length=1 networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T09:54:04-07:00 |INFO| commit a block height=2 iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
```

### Run
```make run``` restarts server with existing database.

You will see log message output like:
```
2018-08-28T10:03:40-07:00 |INFO| Restarting blockchain blockchain height=3 factory height=3 iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T10:03:40-07:00 |INFO| Starting dispatcher iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T10:03:40-07:00 |INFO| Starting IotxConsensus scheme iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate scheme=STANDALONE
2018-08-28T10:03:40-07:00 |INFO| start RPC server on 127.0.0.1:4689 iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T10:03:40-07:00 |INFO| Starting Explorer JSON-RPC server on [::]:14004 iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T10:03:41-07:00 |INFO| created a new block at="2018-08-28 10:03:41.17804365 -0700 PDT m=+1.034361469" iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T10:03:41-07:00 |INFO| No peer exist to sync with. iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T10:03:41-07:00 |INFO| created a new block height=4 iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y length=1 networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T10:03:41-07:00 |INFO| commit a block height=4 iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T10:03:42-07:00 |INFO| No peer exist to sync with. iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T10:03:42-07:00 |INFO| created a new block at="2018-08-28 10:03:42.175542402 -0700 PDT m=+2.031857345" iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T10:03:42-07:00 |INFO| created a new block height=5 iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y length=1 networkAddress=127.0.0.1:4689 nodeType=delegate
2018-08-28T10:03:42-07:00 |INFO| commit a block height=5 iotexAddr=io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y networkAddress=127.0.0.1:4689 nodeType=delegate
```

# Deploy w/ Docker Image

```make docker```

Add `SKIP_DEP=true` to skip re-installing dependencies via `dep`.

## Dev Tools
### Use actioninjector to inject actions
Open one terminal window and run the command below to compile and start the test chain server. (This is optional, just
in case you don't have a node running).

`make reboot`

Open a new terminal window and start running actioninjector.

`./bin/actioninjector`

You can use command line flags to customize the injector.

```
-injector-config-path=path_of_config_file_of_genesis_transfer_addresses
-addr=target_address_for_jrpc_connection
-transfer-num=number_of_transfers
-vote-num=number_of_votes
-execution-num=number_of_executions
-contract=smart_contract_address
-execution-amount=execution_amount
-execution-gas=execution_gas
-execution-gas-price=execution_gas_price
-execution-data=execution_data
-interval=sleeping_interval_in_seconds
-retry-num=maximum_number_of_rpc_retries
-retry-interval=sleeping_interval_between_two_consecutive_rpc_retries_in_seconds
-aps=actions_to_be_injected_per_second_APS_MODE_ONLY
-duration=duration_of_injector_running_in_seconds_APS_MODE_ONLY
```

Default flag values:
* injector-config-path="./tools/actioninjector/gentsfaddrs.yaml"
* addr="127.0.0.1:14004"
* transfer-num=50
* vote-num=50
* execution-num=50
* contract="io1qyqsyqcy3kcd2pyfwus69nzgvkwhg8mk8h336dt86pg6cj"
* execution-amount=0
* execution-gas=1200000
* execution-gas-price=10
* execution-data="2885ad2c"
* interval=5
* retry-num=5
* retry-interval=1
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
