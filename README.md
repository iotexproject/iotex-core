# iotex-core

[![Join the chat at https://gitter.im/iotex-dev-community/Lobby](https://badges.gitter.im/iotex-dev-community/Lobby.svg)](https://gitter.im/iotex-dev-community/Lobby?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)
[![Go version](https://img.shields.io/badge/go-1.11.5-blue.svg)](https://github.com/moovweb/gvm)
[![CircleCI](https://circleci.com/gh/iotexproject/iotex-core.svg?style=svg&circle-token=fe0817d127f251a34b8bdd3336a808c7537e5ec0)](https://circleci.com/gh/iotexproject/iotex-core)
[![Go Report Card](https://goreportcard.com/badge/github.com/iotexproject/iotex-core)](https://goreportcard.com/report/github.com/iotexproject/iotex-core)
[![Coverage](https://codecov.io/gh/iotexproject/iotex-core/branch/master/graph/badge.svg)](https://codecov.io/gh/iotexproject/iotex-core)
[![Godoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](https://godoc.org/github.com/iotexproject/iotex-core)
[![Releases](https://img.shields.io/github/release/iotexproject/iotex-core/all.svg?style=flat-square)](https://github.com/iotexproject/iotex-core/releases)
[![LICENSE](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/iotexproject/iotex-core/blob/master/LICENSE)

![IoTeX Logo](logo/IoTeX.png)
----

Welcome to the official Go implementation of IoTeX protocol! IoTeX is building the next generation of the decentralized 
network for IoT powered by scalability- and privacy-centric blockchains. Please refer to 
IoTeX [whitepaper](https://iotex.io/academics) for details.

Currently, This repo is of alpha-quality with limited features supported and it is subjected to rapid change. Please 
contact us if you intend to run it in production.

## Minimum requirements

| Components | Version | Description |
|----------|-------------|-------------|
|[Golang](https://golang.org) | >= 1.11.5 | The Go Programming Language |

### Setup Dev Environment

Download the code by
```
mkdir -p ~/go/src/github.com/iotexproject
cd ~/go/src/github.com/iotexproject
git clone git@github.com:iotexproject/iotex-core.git
cd iotex-core
```

Build the project by

```make```


If you need to update the dependency, install Go dependency management tool from
[golang dep](https://github.com/golang/dep). Then, run

```dep ensure```

Note: If your Dev Environment is in Ubuntu, you need to export the following Path:

LD_LIBRARY_PATH=$LD_LIBRARY_PATH:$GOPATH/src/github.com/iotexproject/iotex-core/crypto/lib:$GOPATH/src/github.com/iotexproject/iotex-core/crypto/lib/blslib

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

### Minicluster
```make minicluster``` runs a cluster of 4 delegate nodes with RollDPoS consensus scheme while actions are automatically injected to one of the nodes.

You will see log message output like:
```
2018-10-18T14:23:18-07:00 |INFO| commit a block height=0
2018-10-18T14:23:18-07:00 |INFO| Starting IotxConsensus scheme scheme=ROLLDPOS
2018-10-18T14:23:18-07:00 |INFO| commit a block height=0
2018-10-18T14:23:18-07:00 |INFO| Starting IotxConsensus scheme scheme=ROLLDPOS
2018-10-18T14:23:18-07:00 |INFO| commit a block height=0
2018-10-18T14:23:18-07:00 |INFO| Starting IotxConsensus scheme scheme=ROLLDPOS
2018-10-18T14:23:18-07:00 |INFO| commit a block height=0
2018-10-18T14:23:18-07:00 |INFO| Starting IotxConsensus scheme scheme=ROLLDPOS
2018-10-18T14:23:18-07:00 |INFO| Starting Explorer JSON-RPC server on [::]:14004
2018-10-18T14:23:18-07:00 |INFO| Starting dispatcher
2018-10-18T14:23:18-07:00 |INFO| Starting Explorer JSON-RPC server on [::]:14006
2018-10-18T14:23:18-07:00 |INFO| Starting dispatcher
2018-10-18T14:23:18-07:00 |INFO| Starting Explorer JSON-RPC server on [::]:14005
2018-10-18T14:23:18-07:00 |INFO| Starting dispatcher
2018-10-18T14:23:18-07:00 |INFO| start RPC server on 127.0.0.1:4689
2018-10-18T14:23:18-07:00 |INFO| start RPC server on 127.0.0.1:4691
2018-10-18T14:23:18-07:00 |INFO| start RPC server on 127.0.0.1:4690
2018-10-18T14:23:18-07:00 |INFO| Starting Explorer JSON-RPC server on [::]:14007
2018-10-18T14:23:18-07:00 |INFO| Starting dispatcher
2018-10-18T14:23:18-07:00 |INFO| start RPC server on 127.0.0.1:4692
```

# Deploy w/ Docker Image

```make docker```

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
-chain=id_of_target_chain
-addr=target_address_for_jrpc_connection
-transfer-num=number_of_transfers
-transfer-gas-limit=transfer_gas_limit
-transfer-gas-price=transfer_gas_price
-transfer-payload=transfer_payload
-vote-num=number_of_votes
-vote-gas-limit=vote_gas_limit
-vote-gas-price=vote_gas_price
-execution-num=number_of_executions
-contract=smart_contract_address
-execution-amount=execution_amount
-execution-gas-limit=execution_gas_limit
-execution-gas-price=execution_gas_price
-execution-data=execution_data
-interval=sleeping_interval_in_seconds
-retry-num=maximum_number_of_rpc_retries
-retry-interval=sleeping_interval_between_two_consecutive_rpc_retries_in_seconds
-aps=actions_to_be_injected_per_second_APS_MODE_ONLY
-duration=duration_of_injector_running_in_seconds_APS_MODE_ONLY
-reset-interval=time_interval_to_reset_nonce_counter_in_seconds
```

Default flag values:
* injector-config-path="./tools/actioninjector/gentsfaddrs.yaml"
* chain=1
* addr="127.0.0.1:14004"
* transfer-num=50
* transfer-gas-limit=1000000
* transfer-gas-price=10
* transfer-payload=""
* vote-num=50
* vote-gas-limit=1000000
* vote-gas-price=10
* execution-num=50
* contract="io1pmjhyksxmz2xpxn2qmz4gx9qq2kn2gdr8un4xq"
* execution-amount=0
* execution-gas-limit=1200000
* execution-gas-price=10
* execution-data="2885ad2c"
* interval=5
* retry-num=5
* retry-interval=1
* aps=0
* duration=60
* reset-interval=10

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
