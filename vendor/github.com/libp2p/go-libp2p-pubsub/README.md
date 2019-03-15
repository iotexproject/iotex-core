# go-libp2p-pubsub

[![](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square)](http://protocol.ai)
[![](https://img.shields.io/badge/project-libp2p-yellow.svg?style=flat-square)](http://github.com/libp2p/libp2p)
[![](https://img.shields.io/badge/freenode-%23libp2p-yellow.svg?style=flat-square)](http://webchat.freenode.net/?channels=%23libp2p)

> A pubsub system with flooding and gossiping variants.

This is the canonical pubsub implementation for libp2p.

We currently provide three implementations:
- floodsub, which is the baseline flooding protocol.
- gossipsub, which is a more advanced router with mesh formation and gossip propagation.
  See [spec](https://github.com/libp2p/specs/tree/master/pubsub/gossipsub) and  [implementation](https://github.com/libp2p/go-libp2p-pubsub/blob/master/gossipsub.go) for more details.
- randomsub, which is a simple probabilistic router that propagates to random subsets of peers.

## Table of Contents

- [Install](#install)
- [Usage](#usage)
- [Documentation](#documentation)
- [Contribute](#contribute)
- [License](#license)

## Install

```
go get github.com/libp2p/go-libp2p-pubsub
```

## Usage

To be used for messaging in p2p instrastructure (as part of libp2p) such as IPFS, Ethereum, other blockchains, etc.

## Implementations

See [libp2p/specs/pubsub#Implementations](https://github.com/libp2p/specs/tree/master/pubsub#Implementations).

## Documentation

See the [libp2p specs](https://github.com/libp2p/specs/tree/master/pubsub) for high level documentation
and [godoc](https://godoc.org/github.com/libp2p/go-libp2p-pubsub) for API documentation.


## Contribute

Contributions welcome. Please check out [the issues](https://github.com/libp2p/go-libp2p-pubsub/issues).

Check out our [contributing document](https://github.com/libp2p/community/blob/master/contributing.md) for more information on how we work, and about contributing in general. Please be aware that all interactions related to multiformats are subject to the IPFS [Code of Conduct](https://github.com/ipfs/community/blob/master/code-of-conduct.md).

Small note: If editing the README, please conform to the [standard-readme](https://github.com/RichardLitt/standard-readme) specification.

## License

[MIT](LICENSE) © Jeromy Johnson
