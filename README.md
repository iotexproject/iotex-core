# iotex-core

[![Join the chat at https://gitter.im/iotex-dev-community/Lobby](https://badges.gitter.im/iotex-dev-community/Lobby.svg)](https://gitter.im/iotex-dev-community/Lobby?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)
[![Go version](https://img.shields.io/badge/go-1.11.5-blue.svg)](https://github.com/moovweb/gvm)
[![CircleCI](https://circleci.com/gh/iotexproject/iotex-core.svg?style=svg&circle-token=fe0817d127f251a34b8bdd3336a808c7537e5ec0)](https://circleci.com/gh/iotexproject/iotex-core)
[![Go Report Card](https://goreportcard.com/badge/github.com/iotexproject/iotex-core)](https://goreportcard.com/report/github.com/iotexproject/iotex-core)
[![Coverage](https://codecov.io/gh/iotexproject/iotex-core/branch/master/graph/badge.svg)](https://codecov.io/gh/iotexproject/iotex-core)
[![Godoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](https://godoc.org/github.com/iotexproject/iotex-core)
[![Releases](https://img.shields.io/github/release/iotexproject/iotex-core/all.svg?style=flat-square)](https://github.com/iotexproject/iotex-core/releases)
[![LICENSE](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

![IoTeX Logo](logo/IoTeX.png)
----

Welcome to the official Go implementation of IoTeX protocol! IoTeX is building the next generation of the decentralized 
network for IoT powered by scalability- and privacy-centric blockchains. Please refer to IoTeX
[whitepaper](https://iotex.io/academics) for details.

## Get started

### Minimum requirements

| Components | Version | Description |
|----------|-------------|-------------|
| [Golang](https://golang.org) | &ge; 1.11.5 | Go programming language |
| [Dep](https://golang.github.io/dep/) | &ge; 0.5.0 | Dependency management tool, required only when you update dependencies |
| [Protoc](https://developers.google.com/protocol-buffers/) | &ge; 3.6.0 | Protocol buffers, required only when you rebuild protobuf messages |

### Get iotex-core

The easiest way to get iotex-core is to use one of release packages which are available for OSX, Linux on the 
[release page](https://github.com/iotexproject/iotex-core/releases). Iotex-core is also distributed via docker image
on [docker hub](https://hub.docker.com/r/iotex/iotex-core).


### Build iotex-core from code

Download the code by
```
mkdir -p ~/go/src/github.com/iotexproject
cd ~/go/src/github.com/iotexproject
git clone git@github.com:iotexproject/iotex-core.git
cd iotex-core
```

Build the project by

```
make
```

If the dependency needs to be updated, run

```
dep ensure [--vendor-only]
```


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

Note that if your enviroment is in Linux, you need to add the share libraries into `$LD_LIBRARY_PATH` by

```
LD_LIBRARY_PATH=$LD_LIBRARY_PATH:$GOPATH/src/github.com/iotexproject/iotex-core/crypto/lib
```

### Use CLI

Users could interact with iotex blockchain by

```
ioctl [command]
```

Refer to [CLI document](cli/ioctl/README.md) for more details.

## Contact

- Mailing list: [iotex-dev](iotex-dev@iotex.io)
- IRC: [gitter](https://gitter.im/iotex-dev-community/Lobby)
- Bugs: [issues](https://github.com/iotexproject/iotex-core/issues) 


## Contribution
We are glad to have contributors out of the core team; contributions, including (but not limited to) style/bug fixes,
implementation of features, proposals of schemes/algorithms, and thorough documentation, are welcomed. Please refer to
our [contribution guideline](CONTRIBUTING.md) for more
information. Development guide documentation is [here](https://github.com/iotexproject/iotex-core/wiki/Developers%27-Guide).

## License
This project is licensed under the [Apache License 2.0](LICENSE).
