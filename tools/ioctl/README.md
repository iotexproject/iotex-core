# ioctl

`ioctl` is the official IoTeX command-line client for interacting with an IoTeX
blockchain — managing accounts and HD wallets, sending actions and transfers,
deploying and invoking smart contracts, staking, running node/blockchain
queries, and more.

## Install

### Install the released build
```
curl --silent https://raw.githubusercontent.com/iotexproject/iotex-core/master/install-cli.sh | sh
```

### Install the latest (unstable) build
```
curl --silent https://raw.githubusercontent.com/iotexproject/iotex-core/master/install-cli.sh | sh -s "unstable"
```

## Build from source

From the repository root:
```
make ioctl
```
The binary is written to `./bin/ioctl`.

Alternatively, build the release artifacts (used by `install-cli.sh`) with
`./tools/ioctl/buildcli.sh`; the output is placed under `./release/`.

To build `ioctl` on Windows you need [mingw](https://chocolatey.org/); the
[Chocolatey](https://chocolatey.org/) package manager installs it with
`choco install mingw`.

## Usage

```
ioctl [command] [flags]
```

Run `ioctl --help`, or `ioctl [command] --help`, for the full, always-up-to-date
list of subcommands and flags.

### Top-level commands

| Command | Description |
|---|---|
| `account` | Manage accounts of the IoTeX blockchain |
| `action` | Manage actions of the IoTeX blockchain |
| `alias` | Manage aliases of IoTeX addresses |
| `bc` | Deal with the block chain of the IoTeX blockchain |
| `config` | Get, set, or reset configuration for ioctl |
| `contract` | Deal with smart contracts of the IoTeX blockchain |
| `did` | DID command |
| `hdwallet` | Manage HD wallets of the IoTeX blockchain |
| `ins` | Manage INS of the IoTeX blockchain |
| `ioid` | Manage ioID |
| `jwt` | Manage JSON Web Tokens on the IoTeX blockchain |
| `node` | Deal with nodes of the IoTeX blockchain |
| `stake2` | Support native staking of the IoTeX blockchain |
| `update` | Update ioctl to the latest version |
| `version` | Print the version and build info of ioctl |
| `ws` | W3bstream node operations |
| `xrc20` | Support the ERC20 standard command-line |

## Documentation

For guides and the full command reference, see the official
[ioctl CLI documentation](https://docs.iotex.io/blockchain/build/web3-development/ioctl-cli).
