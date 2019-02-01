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

// TestingBuilder is used to construct Block.
type TestingBuilder struct{ blk Block }

// NewTestingBuilder creates a Builder.
func NewTestingBuilder() *TestingBuilder {
	return &TestingBuilder{
		blk: Block{
			Header: Header{
				version: version.ProtocolVersion,
			},
		},
	}
}

// SetVersion sets the protocol version for block which is building.
func (b *TestingBuilder) SetVersion(v uint32) *TestingBuilder {
	b.blk.Header.version = v
	return b
}

// SetChainID sets the chain id for block which is building.
func (b *TestingBuilder) SetChainID(c uint32) *TestingBuilder {
	b.blk.Header.chainID = c
	return b
}

// SetHeight sets the block height for block which is building.
func (b *TestingBuilder) SetHeight(h uint64) *TestingBuilder {
	b.blk.Header.height = h
	return b
}

// SetTimeStamp sets the time stamp for block which is building.
func (b *TestingBuilder) SetTimeStamp(ts int64) *TestingBuilder {
	b.blk.Header.timestamp = ts
	return b
}

// SetPrevBlockHash sets the previous block hash for block which is building.
func (b *TestingBuilder) SetPrevBlockHash(h hash.Hash32B) *TestingBuilder {
	b.blk.Header.prevBlockHash = h
	return b
}

// AddActions adds actions for block which is building.
func (b *TestingBuilder) AddActions(acts ...action.SealedEnvelope) *TestingBuilder {
	if b.blk.Actions == nil {
		b.blk.Actions = make([]action.SealedEnvelope, 0)
	}
	b.blk.Actions = append(b.blk.Actions, acts...)
	return b
}

// SetStateRoot sets the new state root after running actions included in this building block.
func (b *TestingBuilder) SetStateRoot(h hash.Hash32B) *TestingBuilder {
	b.blk.Header.stateRoot = h
	return b
}

// SetReceipts sets the receipts after running actions included in this building block.
func (b *TestingBuilder) SetReceipts(receipts []*action.Receipt) *TestingBuilder {
	b.blk.Receipts = receipts // make a shallow copy
	return b
}

// SignAndBuild signs and then builds a block.
func (b *TestingBuilder) SignAndBuild(signerPubKey keypair.PublicKey, signerPriKey keypair.PrivateKey) (Block, error) {
	b.blk.Header.txRoot = b.blk.CalculateTxRoot()
	b.blk.Header.pubkey = signerPubKey
	blkHash := b.blk.HashBlock()
	sig, err := crypto.Sign(blkHash[:], signerPriKey)
	if err != nil {
		return Block{}, errors.New("Failed to sign block")
	}
	b.blk.Header.blockSig = sig
	return b.blk, nil
}

// NewBlockDeprecated returns a new block
// This method is deprecated. Only used in old tests.
func NewBlockDeprecated(
	chainID uint32,
	height uint64,
	prevBlockHash hash.Hash32B,
	timestamp int64,
	producer keypair.PublicKey,
	actions []action.SealedEnvelope,
) *Block {
	block := &Block{
		Header: Header{
			version:       version.ProtocolVersion,
			chainID:       chainID,
			height:        height,
			timestamp:     timestamp,
			prevBlockHash: prevBlockHash,
			pubkey:        producer,
			txRoot:        hash.ZeroHash32B,
			stateRoot:     hash.ZeroHash32B,
			receiptRoot:   hash.ZeroHash32B,
		},
		Actions: actions,
	}

	block.Header.txRoot = block.CalculateTxRoot()
	return block
}
