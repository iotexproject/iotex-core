// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"bytes"
	"errors"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/state/factory"
)

// Payee defines the struct of payee
type Payee struct {
	Address string
	Amount  uint64
}

// Header defines the struct of block header
// make sure the variable type and order of this struct is same as "BlockHeaderPb" in blockchain.pb.go
type Header struct {
	version       uint32            // version
	chainID       uint32            // this chain's ID
	height        uint64            // block height
	timestamp     uint64            // unix timestamp
	prevBlockHash hash.Hash32B      // hash of previous block
	txRoot        hash.Hash32B      // merkle root of all transactions
	stateRoot     hash.Hash32B      // root of state trie
	receiptRoot   hash.Hash32B      // root of receipt trie
	blockSig      []byte            // block signature
	pubkey        keypair.PublicKey // block producer's public key
	dkgID         []byte            // dkg ID of producer
	dkgPubkey     []byte            // dkg public key of producer
	dkgBlockSig   []byte            // dkg signature of producer
}

// Timestamp returns the timestamp in the block header
func (bh *Header) Timestamp() time.Time {
	return time.Unix(int64(bh.timestamp), 0)
}

// Footer defines a set of proof of this block
type Footer struct {
	// endorsements contain COMMIT endorsements from more than 2/3 delegates
	endorsements    *endorsement.Set
	commitTimestamp uint64
}

// Block defines the struct of block
type Block struct {
	Header          *Header
	Actions         []action.SealedEnvelope
	SecretProposals []*action.SecretProposal
	SecretWitness   *action.SecretWitness
	Receipts        map[hash.Hash32B]*action.Receipt
	Footer          *Footer

	WorkingSet factory.WorkingSet
}

// Height returns the height of this block
func (b *Block) Height() uint64 {
	return b.Header.height
}

// ChainID returns the chain id of this block
func (b *Block) ChainID() uint32 {
	return b.Header.chainID
}

// Timestamp returns the Timestamp of this block
func (b *Block) Timestamp() uint64 {
	return b.Header.timestamp
}

// Version returns the version of this block
func (b *Block) Version() uint32 {
	return b.Header.version
}

// PrevHash returns the hash of prev block
func (b *Block) PrevHash() hash.Hash32B {
	return b.Header.prevBlockHash
}

// TxRoot returns the hash of all actions in this block.
func (b *Block) TxRoot() hash.Hash32B {
	return b.Header.txRoot
}

// PublicKey returns the public key of this block
func (b *Block) PublicKey() keypair.PublicKey {
	return b.Header.pubkey
}

// StateRoot returns the state root after apply this block.
func (b *Block) StateRoot() hash.Hash32B {
	return b.Header.stateRoot
}

// DKGPubkey returns DKG PublicKey.
func (b *Block) DKGPubkey() []byte {
	pk := make([]byte, len(b.Header.dkgPubkey))
	copy(pk, b.Header.dkgPubkey)
	return pk
}

// DKGID returns DKG ID.
func (b *Block) DKGID() []byte {
	id := make([]byte, len(b.Header.dkgID))
	copy(id, b.Header.dkgID)
	return id
}

// DKGSignature returns DKG Signature.
func (b *Block) DKGSignature() []byte {
	sig := make([]byte, len(b.Header.dkgBlockSig))
	copy(sig, b.Header.dkgBlockSig)
	return sig
}

// HeaderLogger returns a new logger with block header fields' value.
func (b *Block) HeaderLogger(l *zerolog.Logger) *zerolog.Logger {
	ctxl := l.With().
		Uint32("version", b.Header.version).
		Uint32("chainID", b.Header.chainID).
		Uint64("height", b.Header.height).
		Uint64("timeStamp", b.Header.timestamp).
		Hex("prevBlockHash", b.Header.prevBlockHash[:]).
		Hex("txRoot", b.Header.txRoot[:]).
		Hex("stateRoot", b.Header.stateRoot[:]).
		Hex("receiptRoot", b.Header.receiptRoot[:]).Logger()
	return &ctxl
}

// ByteStreamHeader returns a byte stream of the block header
func (b *Block) ByteStreamHeader() []byte {
	stream := make([]byte, 4)
	enc.MachineEndian.PutUint32(stream, b.Header.version)
	tmp4B := make([]byte, 4)
	enc.MachineEndian.PutUint32(tmp4B, b.Header.chainID)
	stream = append(stream, tmp4B...)
	tmp8B := make([]byte, 8)
	enc.MachineEndian.PutUint64(tmp8B, b.Header.height)
	stream = append(stream, tmp8B...)
	enc.MachineEndian.PutUint64(tmp8B, b.Header.timestamp)
	stream = append(stream, tmp8B...)
	stream = append(stream, b.Header.prevBlockHash[:]...)
	stream = append(stream, b.Header.txRoot[:]...)
	stream = append(stream, b.Header.stateRoot[:]...)
	stream = append(stream, b.Header.receiptRoot[:]...)
	return stream
}

// ByteStream returns a byte stream of the block
func (b *Block) ByteStream() []byte {
	stream := b.ByteStreamHeader()

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
	b.Header = new(Header)

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
	return blake2b.Sum256(b.ByteStreamHeader())
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
