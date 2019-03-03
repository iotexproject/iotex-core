// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"bytes"

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
				pubkey:    ra.blockProducerPubKey,
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
func (b *Builder) SignAndBuild(signerPrvKey keypair.PrivateKey) (Block, error) {
	if !bytes.Equal(b.blk.Header.pubkey.Bytes(), signerPrvKey.PublicKey().Bytes()) {
		return Block{}, errors.New("public key from the signer doesn't match that from runnable actions")
	}

	h := b.blk.Header.HashHeaderCore()
	sig, err := signerPrvKey.Sign(h[:])
	if err != nil {
		return Block{}, errors.New("failed to sign block")
	}
	b.blk.Header.blockSig = sig
	return b.blk, nil
}
