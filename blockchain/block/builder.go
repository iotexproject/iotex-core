// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/crypto"
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

// SetChainID sets the chain id for block which is building.
func (b *Builder) SetChainID(c uint32) *Builder {
	b.blk.Header.chainID = c
	return b
}

// SetPrevBlockHash sets the previous block hash for block which is building.
func (b *Builder) SetPrevBlockHash(h hash.Hash32B) *Builder {
	b.blk.Header.prevBlockHash = h
	return b
}

// SetStateRoot sets the new state root after running actions included in this building block.
func (b *Builder) SetStateRoot(h hash.Hash32B) *Builder {
	b.blk.Header.stateRoot = h
	return b
}

// SetReceipts sets the receipts after running actions included in this building block.
func (b *Builder) SetReceipts(rm map[hash.Hash32B]*action.Receipt) *Builder {
	b.blk.Receipts = make(map[hash.Hash32B]*action.Receipt)
	for h, r := range rm {
		b.blk.Receipts[h] = r
	}
	return b
}

// SetSecretProposals sets the secret proposals for block which is building.
func (b *Builder) SetSecretProposals(sp []*action.SecretProposal) *Builder {
	b.blk.SecretProposals = sp
	return b
}

// SetSecretWitness sets the secret witness for block which is building.
func (b *Builder) SetSecretWitness(sw *action.SecretWitness) *Builder {
	b.blk.SecretWitness = sw
	return b
}

// SetDKG sets the DKG parts for block which is building.
func (b *Builder) SetDKG(id, pk, sig []byte) *Builder {
	b.blk.Header.dkgID = make([]byte, len(id))
	copy(b.blk.Header.dkgID, id)
	b.blk.Header.dkgPubkey = make([]byte, len(pk))
	copy(b.blk.Header.dkgPubkey, pk)
	b.blk.Header.dkgBlockSig = make([]byte, len(sig))
	copy(b.blk.Header.dkgBlockSig, sig)
	return b
}

// SignAndBuild signs and then builds a block.
func (b *Builder) SignAndBuild(signerPubKey keypair.PublicKey, signerPriKey keypair.PrivateKey) (Block, error) {
	b.blk.Header.pubkey = signerPubKey
	blkHash := b.blk.HashBlock()
	sig := crypto.EC283.Sign(signerPriKey, blkHash[:])
	if len(sig) == 0 {
		return Block{}, errors.New("Failed to sign block")
	}
	b.blk.Header.blockSig = sig
	return b.blk, nil
}
