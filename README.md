# iotex-core

[![Join the forum](https://img.shields.io/badge/Discuss-IoTeX%20Community-blue)](https://community.iotex.io/c/research-development/protocol)
[![Go version](https://img.shields.io/badge/go-1.14.4-blue.svg)](https://github.com/moovweb/gvm)
[![CircleCI](https://circleci.com/gh/iotexproject/iotex-core.svg?style=svg&circle-token=fe0817d127f251a34b8bdd3336a808c7537e5ec0)](https://circleci.com/gh/iotexproject/iotex-core)
[![Go Report Card](https://goreportcard.com/badge/github.com/iotexproject/iotex-core)](https://goreportcard.com/report/github.com/iotexproject/iotex-core)
[![Coverage](https://codecov.io/gh/iotexproject/iotex-core/branch/master/graph/badge.svg)](https://codecov.io/gh/iotexproject/iotex-core)
[![Godoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](https://godoc.org/github.com/iotexproject/iotex-core)
[![Releases](https://img.shields.io/github/release/iotexproject/iotex-core/all.svg?style=flat-square)](https://github.com/iotexproject/iotex-core/releases)
[![LICENSE](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

![IoTeX Logo](logo/IoTeX.png)
----

Welcome to the official Go implementation of IoTeX protocol! IoTeX is building the next generation of the decentralized blockchain protocol for powering real-world information marketplace in a decentralized-yet-scalable way. Refer to IoTeX [whitepaper](https://iotex.io/research/) for details.

<a href="https://iotex.io/devdiscord" target="_blank"><img src="https://github.com/iotexproject/halogrants/blob/0082107b6d4381ba2237adce01dd48672644cc25/img/btn-discord.png" align="right"></a>

## Get started

### Minimum requirements

| Components | Version | Description |
|----------|-------------|-------------|
| [Golang](https://golang.org) | &ge; 1.14.4 | Go programming language |
| [Protoc](https://developers.google.com/protocol-buffers/) | &ge; 3.6.0 | Protocol buffers, required only when you rebuild protobuf messages |

### Get iotex-core

The easiest way to get iotex-core is to use one of release packages which are available for OSX, Linux on the 
[release page](https://github.com/iotexproject/iotex-core/releases). Iotex-core is also distributed via docker image
on [docker hub](https://hub.docker.com/r/iotex/iotex-core).


### Build iotex-core from code

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
If you want learn more advanced usage about `go mod`, you can find out [here](https://github.com/golang/go/wiki/Modules).

Run unit tests only by

```
make test
```

Build the docker image by

```
make docker
```

### Run iotex-core

Start (or resume) a standalone server to operate on an blockchain by

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

## License
This project is licensed under the [Apache License 2.0](LICENSE).
