# iotex-core 

Official Golang implementation of the IoTeX protocol.

[![Join the forum](https://img.shields.io/badge/Discuss-IoTeX%20Community-blue)](https://community.iotex.io/c/research-development/protocol)
[![Go version](https://img.shields.io/badge/go-1.18.5-blue.svg)](https://github.com/moovweb/gvm)
[![Go Report Card](https://goreportcard.com/badge/github.com/iotexproject/iotex-core)](https://goreportcard.com/report/github.com/iotexproject/iotex-core)
[![Coverage](https://codecov.io/gh/iotexproject/iotex-core/branch/master/graph/badge.svg)](https://codecov.io/gh/iotexproject/iotex-core)
[![Godoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](https://godoc.org/github.com/iotexproject/iotex-core)
[![Releases](https://img.shields.io/github/release/iotexproject/iotex-core/all.svg?style=flat-square)](https://github.com/iotexproject/iotex-core/releases)
[![LICENSE](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

<a href="https://iotex.io/"><img src="logo/IoTeX.png" height="200px"/></a>


Welcome to the official Go implementation of IoTeX protocol! IoTeX is building the next generation of the decentralized blockchain protocol for powering real-world information marketplace in a decentralized-yet-scalable way. Refer to IoTeX [whitepaper](https://iotex.io/research/) for details.

<a href="https://iotex.io/devdiscord" target="_blank">
  <img src="https://github.com/iotexproject/halogrants/blob/880eea4af074b082a75608c7376bd7a8eaa1ac21/img/btn-discord.svg" height="36px">
</a>

#### New to IoTeX?

Please visit https://iotex.io official website or [IoTeX onboard pack](https://onboard.iotex.io/) to learn more about IoTeX network.

#### Run a delegate?

Please visit [IoTeX Delegate Manual](https://github.com/iotexproject/iotex-bootstrap) for detailed setup process.

## Building the source code

### Minimum requirements

| Components | Version | Description |
|----------|-------------|-------------|
| [Golang](https://golang.org) | &ge; 1.18.5 | Go programming language |
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

## Contributors

Thank you for considering contributing to the IoTeX framework!

<a href="https://github.com/iotexproject/iotex-core/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=iotexproject/iotex-core" />
</a>

## License
This project is licensed under the [Apache License 2.0](LICENSE).
