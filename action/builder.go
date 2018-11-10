// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/iotexproject/iotex-core/pkg/keypair"
)

// Builder is used to build an action.
type Builder struct {
	act AbstractAction
}

// SetVersion sets action's version.
func (b *Builder) SetVersion(v uint32) *Builder {
	b.act.version = v
	return b
}

// SetNonce sets action's nonce.
func (b *Builder) SetNonce(n uint64) *Builder {
	b.act.nonce = n
	return b
}

// SetSourceAddress sets action's source address.
func (b *Builder) SetSourceAddress(addr string) *Builder {
	b.act.srcAddr = addr
	return b
}

// SetDestinationAddress sets action's destination address.
func (b *Builder) SetDestinationAddress(addr string) *Builder {
	b.act.dstAddr = addr
	return b
}

// SetSourcePublicKey sets action's source's public key.
func (b *Builder) SetSourcePublicKey(key keypair.PublicKey) *Builder {
	b.act.srcPubkey = key
	return b
}

// SetGasLimit sets action's gas limit.
func (b *Builder) SetGasLimit(l uint64) *Builder {
	b.act.gasLimit = l
	return b
}

// SetGasPrice sets action's gas price.
func (b *Builder) SetGasPrice(p *big.Int) *Builder {
	if p == nil {
		return b
	}
	b.act.gasPrice = &big.Int{}
	b.act.gasPrice.Set(p)
	return b
}

// SetGasPriceByBytes sets action's gas price from a byte slice source.
func (b *Builder) SetGasPriceByBytes(buf []byte) *Builder {
	if len(buf) == 0 {
		return b
	}
	b.act.gasPrice = &big.Int{}
	b.act.gasPrice.SetBytes(buf)
	return b
}

// Build builds a new action.
func (b *Builder) Build() AbstractAction {
	if b.act.gasPrice == nil {
		b.act.gasPrice = big.NewInt(0)
	}
	return b.act
}
