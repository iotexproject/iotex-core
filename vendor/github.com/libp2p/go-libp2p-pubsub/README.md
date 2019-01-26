# go-libp2p-pubsub

[![](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square)](http://ipn.io)
[![](https://img.shields.io/badge/project-libp2p-blue.svg?style=flat-square)](http://github.com/libp2p/libp2p)
[![](https://img.shields.io/badge/freenode-%23ipfs-blue.svg?style=flat-square)](http://webchat.freenode.net/?channels=%23ipfs)

> A pubsub system with flooding and gossiping variants.

PubSub is a work in progress, with floodsub as an initial protocol, followed by gossipsub ([spec](https://github.com/libp2p/specs/tree/master/pubsub/gossipsub), [gossipsub.go](https://github.com/libp2p/go-libp2p-pubsub/blob/master/gossipsub.go)).

## Table of Contents

- [Install](#install)
- [Usage](#usage)
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

## Contribute

Contributions welcome. Please check out [the issues](https://github.com/libp2p/go-libp2p-pubsub/issues).

Check out our [contributing document](https://github.com/libp2p/community/blob/master/contributing.md) for more information on how we work, and about contributing in general. Please be aware that all interactions related to multiformats are subject to the IPFS [Code of Conduct](https://github.com/ipfs/community/blob/master/code-of-conduct.md).

Small note: If editing the README, please conform to the [standard-readme](https://github.com/RichardLitt/standard-readme) specification.

## License

[MIT](LICENSE) © Jeromy Johnson
