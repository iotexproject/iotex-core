// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

// AbstractAction is an abstract implementation of Action interface
type AbstractAction struct {
	version   uint32
	nonce     uint64
	srcPubkey keypair.PublicKey
	gasLimit  uint64
	gasPrice  *big.Int
	hash      hash.Hash256
}

// Version returns the version
func (act *AbstractAction) Version() uint32 { return act.version }

// Nonce returns the nonce
func (act *AbstractAction) Nonce() uint64 { return act.nonce }

// SrcPubkey returns the source public key
func (act *AbstractAction) SrcPubkey() keypair.PublicKey { return act.srcPubkey }

// GasLimit returns the gas limit
func (act *AbstractAction) GasLimit() uint64 { return act.gasLimit }

// GasPrice returns the gas price
func (act *AbstractAction) GasPrice() *big.Int {
	p := &big.Int{}
	if act.gasPrice == nil {
		return p
	}
	return p.Set(act.gasPrice)
}

// Hash returns the hash value of referred SealedActionEnvelope hash.
func (act *AbstractAction) Hash() hash.Hash256 { return act.hash }

// BasicActionSize returns the basic size of action
func (act *AbstractAction) BasicActionSize() uint32 {
	// VersionSizeInBytes + NonceSizeInBytes + GasSizeInBytes
	size := 4 + 8 + 8
	if act.srcPubkey != nil {
		size += len(act.srcPubkey.Bytes())
	}
	if act.gasPrice != nil && len(act.gasPrice.Bytes()) > 0 {
		size += len(act.gasPrice.Bytes())
	}

	return uint32(size)
}

// SetEnvelopeContext sets the SealedEnvelope context to action context.
func (act *AbstractAction) SetEnvelopeContext(selp SealedEnvelope) {
	if act == nil {
		return
	}
	ab := &Builder{}
	*act = ab.SetVersion(selp.Version()).
		SetNonce(selp.Nonce()).
		SetSourcePublicKey(selp.SrcPubkey()).
		SetGasLimit(selp.GasLimit()).
		SetGasPrice(selp.GasPrice()).
		Build()

	// the reason to set hash here, after set act context, is because some actions use envelope information in their proto define. for example transfer use des addr as Receipt.
	act.hash = selp.Hash()
}
