// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/version"
)

// EnvelopeBuilder is the builder to build Envelope.
type EnvelopeBuilder struct {
	elp Envelope
}

// SetVersion sets action's version.
func (b *EnvelopeBuilder) SetVersion(v uint32) *EnvelopeBuilder {
	b.elp.version = v
	return b
}

// SetNonce sets action's nonce.
func (b *EnvelopeBuilder) SetNonce(n uint64) *EnvelopeBuilder {
	b.elp.nonce = n
	return b
}

// SetGasLimit sets action's gas limit.
func (b *EnvelopeBuilder) SetGasLimit(l uint64) *EnvelopeBuilder {
	b.elp.gasLimit = l
	return b
}

// SetGasPrice sets action's gas price.
func (b *EnvelopeBuilder) SetGasPrice(p *big.Int) *EnvelopeBuilder {
	if p == nil {
		return b
	}
	b.elp.gasPrice = &big.Int{}
	b.elp.gasPrice.Set(p)
	return b
}

// SetGasPriceByBytes sets action's gas price from a byte slice source.
func (b *EnvelopeBuilder) SetGasPriceByBytes(buf []byte) *EnvelopeBuilder {
	if len(buf) == 0 {
		return b
	}
	b.elp.gasPrice = &big.Int{}
	b.elp.gasPrice.SetBytes(buf)
	return b
}

// SetAction sets the action for the Envelope Builder is building.
func (b *EnvelopeBuilder) SetAction(action Action) *EnvelopeBuilder {
	b.elp.payload = action
	return b
}

// Build builds a new action.
func (b *EnvelopeBuilder) Build() (Envelope, error) {
	if b.elp.gasPrice == nil {
		b.elp.gasPrice = big.NewInt(0)
	}
	if b.elp.gasPrice.Sign() < 0 {
		return Envelope{}, errors.New("invalid gas price")
	}
	if b.elp.version == 0 {
		b.elp.version = version.ProtocolVersion
	}
	return b.elp, nil
}
