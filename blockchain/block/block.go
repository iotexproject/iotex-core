// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"bytes"
	"errors"

	"github.com/golang/protobuf/proto"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/state/factory"
)

// Footer defines a set of proof of this block
type Footer struct {
	// endorsements contain COMMIT endorsements from more than 2/3 delegates
	endorsements    *endorsement.Set
	commitTimestamp uint64
}

// Block defines the struct of block
type Block struct {
	Header

	Actions         []action.SealedEnvelope
	SecretProposals []*action.SecretProposal
	SecretWitness   *action.SecretWitness
	Receipts        map[hash.Hash32B]*action.Receipt
	Footer          *Footer

	WorkingSet factory.WorkingSet
}

// ByteStream returns a byte stream of the block
func (b *Block) ByteStream() []byte {
	stream := b.Header.ByteStream()

	// Add the stream of blockSig
	stream = append(stream, b.Header.blockSig[:]...)
	stream = append(stream, b.Header.dkgID[:]...)
	stream = append(stream, b.Header.dkgPubkey[:]...)
	stream = append(stream, b.Header.dkgBlockSig[:]...)

	for _, act := range b.Actions {
		stream = append(stream, act.ByteStream()...)
	}
	return stream
}

// ConvertToBlockHeaderPb converts BlockHeader to BlockHeaderPb
func (b *Block) ConvertToBlockHeaderPb() *iproto.BlockHeaderPb {
	pbHeader := iproto.BlockHeaderPb{}

	pbHeader.Version = b.Header.version
	pbHeader.ChainID = b.Header.chainID
	pbHeader.Height = b.Header.height
	pbHeader.Timestamp = b.Header.timestamp
	pbHeader.PrevBlockHash = b.Header.prevBlockHash[:]
	pbHeader.TxRoot = b.Header.txRoot[:]
	pbHeader.StateRoot = b.Header.stateRoot[:]
	pbHeader.ReceiptRoot = b.Header.receiptRoot[:]
	pbHeader.Signature = b.Header.blockSig[:]
	pbHeader.Pubkey = b.Header.pubkey[:]
	pbHeader.DkgID = b.Header.dkgID[:]
	pbHeader.DkgPubkey = b.Header.dkgPubkey[:]
	pbHeader.DkgSignature = b.Header.dkgBlockSig[:]
	return &pbHeader
}

// ConvertToBlockPb converts Block to BlockPb
func (b *Block) ConvertToBlockPb() *iproto.BlockPb {
	actions := []*iproto.ActionPb{}
	for _, act := range b.Actions {
		actions = append(actions, act.Proto())
	}
	return &iproto.BlockPb{Header: b.ConvertToBlockHeaderPb(), Actions: actions}
}

// Serialize returns the serialized byte stream of the block
func (b *Block) Serialize() ([]byte, error) {
	return proto.Marshal(b.ConvertToBlockPb())
}

// ConvertFromBlockHeaderPb converts BlockHeaderPb to BlockHeader
func (b *Block) ConvertFromBlockHeaderPb(pbBlock *iproto.BlockPb) {
	b.Header = Header{}

	b.Header.version = pbBlock.GetHeader().GetVersion()
	b.Header.chainID = pbBlock.GetHeader().GetChainID()
	b.Header.height = pbBlock.GetHeader().GetHeight()
	b.Header.timestamp = pbBlock.GetHeader().GetTimestamp()
	copy(b.Header.prevBlockHash[:], pbBlock.GetHeader().GetPrevBlockHash())
	copy(b.Header.txRoot[:], pbBlock.GetHeader().GetTxRoot())
	copy(b.Header.stateRoot[:], pbBlock.GetHeader().GetStateRoot())
	copy(b.Header.receiptRoot[:], pbBlock.GetHeader().GetReceiptRoot())
	b.Header.blockSig = pbBlock.GetHeader().GetSignature()
	copy(b.Header.pubkey[:], pbBlock.GetHeader().GetPubkey())
	b.Header.dkgID = pbBlock.GetHeader().GetDkgID()
	b.Header.dkgPubkey = pbBlock.GetHeader().GetDkgPubkey()
	b.Header.dkgBlockSig = pbBlock.GetHeader().GetDkgSignature()
}

// ConvertFromBlockPb converts BlockPb to Block
func (b *Block) ConvertFromBlockPb(pbBlock *iproto.BlockPb) error {
	b.ConvertFromBlockHeaderPb(pbBlock)

	b.Actions = []action.SealedEnvelope{}

	for _, actPb := range pbBlock.Actions {
		act := action.SealedEnvelope{}
		if err := act.LoadProto(actPb); err != nil {
			return err
		}
		b.Actions = append(b.Actions, act)
		// TODO handle SecretProposal and SecretWitness
	}
	return nil
}

// Deserialize parses the byte stream into a Block
func (b *Block) Deserialize(buf []byte) error {
	pbBlock := iproto.BlockPb{}
	if err := proto.Unmarshal(buf, &pbBlock); err != nil {
		return err
	}

	if err := b.ConvertFromBlockPb(&pbBlock); err != nil {
		return err
	}
	b.WorkingSet = nil

	// verify merkle root can match after deserialize
	txroot := b.CalculateTxRoot()
	if !bytes.Equal(b.Header.txRoot[:], txroot[:]) {
		return errors.New("Failed to match merkle root after deserialize")
	}
	return nil
}

// CalculateTxRoot returns the Merkle root of all txs and actions in this block.
func (b *Block) CalculateTxRoot() hash.Hash32B {
	return calculateTxRoot(b.Actions)
}

// HashBlock return the hash of this block (actually hash of block header)
func (b *Block) HashBlock() hash.Hash32B {
	return blake2b.Sum256(b.Header.ByteStream())
}

// VerifyStateRoot verifies the state root in header
func (b *Block) VerifyStateRoot(root hash.Hash32B) error {
	if b.Header.stateRoot != root {
		return errors.New("state root hash does not match")
	}
	return nil
}

// VerifySignature verifies the signature saved in block header
func (b *Block) VerifySignature() bool {
	blkHash := b.HashBlock()

	return crypto.EC283.Verify(b.Header.pubkey, blkHash[:], b.Header.blockSig)
}

// ProducerAddress returns the address of producer
func (b *Block) ProducerAddress() string {
	pkHash := keypair.HashPubKey(b.Header.pubkey)
	addr := address.New(b.Header.chainID, pkHash[:])

	return addr.IotxAddress()
}

// RunnableActions abstructs RunnableActions from a Block.
func (b *Block) RunnableActions() RunnableActions {
	pkHash := keypair.HashPubKey(b.Header.pubkey)
	addr := address.New(b.Header.chainID, pkHash[:])
	return RunnableActions{
		blockHeight:         b.Header.height,
		blockTimeStamp:      b.Header.timestamp,
		blockProducerPubKey: b.Header.pubkey,
		blockProducerAddr:   addr.IotxAddress(),
		actions:             b.Actions,
	}
}
