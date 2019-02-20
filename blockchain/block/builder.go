// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"github.com/iotexproject/go-ethereum/crypto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/version"
)

// Builder is used to construct Block.
type Builder struct{ blk Block }

// NewBuilder creates a Builder.
func NewBuilder(ra RunnableActions) *Builder {
	return &Builder{
		blk: Block{
			Header: Header{
				version:   version.ProtocolVersion,
				height:    ra.blockHeight,
				timestamp: ra.blockTimeStamp,
				txRoot:    ra.txHash,
			},
			Actions: ra.actions,
		},
	}
}

// SetVersion sets the protocol version for block which is building.
func (b *Builder) SetVersion(v uint32) *Builder {
	b.blk.Header.version = v
	return b
}

// SetPrevBlockHash sets the previous block hash for block which is building.
func (b *Builder) SetPrevBlockHash(h hash.Hash256) *Builder {
	b.blk.Header.prevBlockHash = h
	return b
}

// SetDeltaStateDigest sets the new delta state digest after running actions included in this building block
func (b *Builder) SetDeltaStateDigest(h hash.Hash256) *Builder {
	b.blk.Header.deltaStateDigest = h
	return b
}

// SetReceipts sets the receipts after running actions included in this building block.
func (b *Builder) SetReceipts(receipts []*action.Receipt) *Builder {
	b.blk.Receipts = receipts // make a shallow copy
	return b
}

// SetReceiptRoot sets the receipt root after running actions included in this building block.
func (b *Builder) SetReceiptRoot(h hash.Hash256) *Builder {
	b.blk.Header.receiptRoot = h
	return b
}

// SignAndBuild signs and then builds a block.
func (b *Builder) SignAndBuild(signerPubKey keypair.PublicKey, signerPriKey keypair.PrivateKey) (Block, error) {
	b.blk.Header.pubkey = signerPubKey
	h := b.blk.Header.HashHeaderCore()
	sig, err := crypto.Sign(h[:], signerPriKey)
	if err != nil {
		return Block{}, errors.New("Failed to sign block")
	}
	b.blk.Header.blockSig = sig
	return b.blk, nil
}
