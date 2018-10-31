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

// SetNonce sets action's version.
func (b *Builder) SetNonce(n uint64) *Builder {
	b.act.nonce = n
	return b
}

// SetSourceAddress sets
func (b *Builder) SetSourceAddress(addr string) *Builder {
	b.act.srcAddr = addr
	return b
}

// SetDestinationAddress sets
func (b *Builder) SetDestinationAddress(addr string) *Builder {
	b.act.dstAddr = addr
	return b
}

// SetSourcePublicKey sets
func (b *Builder) SetSourcePublicKey(key keypair.PublicKey) *Builder {
	b.act.srcPubkey = key
	return b
}

// SetGasLimit sets
func (b *Builder) SetGasLimit(l uint64) *Builder {
	b.act.gasLimit = l
	return b
}

// SetGasPrice sets
func (b *Builder) SetGasPrice(p *big.Int) *Builder {
	b.act.gasPrice = &big.Int{}
	b.act.gasPrice.Set(p)
	return b
}

// Build builds a new action.
func (b *Builder) Build() AbstractAction { return b.act }
