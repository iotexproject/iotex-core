// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"errors"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/common"
	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/proto"
)

const (
	// Version of blockchain protocol
	Version = 1
)

// BlockHeader defines the struct of block header
// make sure the variable type and order of this struct is same as "BlockHeaderPb" in blockchain.pb.go
type BlockHeader struct {
	version       uint32         // version
	chainID       uint32         // this chain's ID
	height        uint64         // block height
	timestamp     uint64         // timestamp
	prevBlockHash common.Hash32B // hash of previous block
	txRoot        common.Hash32B // merkle root of all transactions
	stateRoot     common.Hash32B // merkle root of all states
	trnxNumber    uint32         // number of transaction in this block
	trnxDataSize  uint32         // size (in bytes) of transaction data in this block
	blockSig      []byte         // block signature
}

// Block defines the struct of block
type Block struct {
	Header *BlockHeader
	Tranxs []*Tx
	Votes  []*iproto.VotePb
}

// NewBlock returns a new block
func NewBlock(chainID uint32, height uint64, prevBlockHash common.Hash32B, transactions []*Tx) *Block {
	block := &Block{
		Header: &BlockHeader{Version, chainID, height, uint64(time.Now().Unix()),
			prevBlockHash, common.ZeroHash32B, common.ZeroHash32B,
			uint32(len(transactions)), 0, []byte{}},
		Tranxs: transactions,
	}

	block.Header.txRoot = block.TxRoot()
	for _, tx := range transactions {
		// add up trnx size
		block.Header.trnxDataSize += tx.TotalSize()
	}
	return block
}

// Height returns the height of this block
func (b *Block) Height() uint64 {
	return b.Header.height
}

// TranxsSize returns the size of transactions in this block
func (b *Block) TranxsSize() uint32 {
	return b.Header.trnxDataSize
}

// PrevHash returns the hash of prev block
func (b *Block) PrevHash() common.Hash32B {
	return b.Header.prevBlockHash
}

// ByteStream returns a byte stream of the block header
func (b *Block) ByteStreamHeader() []byte {
	stream := make([]byte, 4)
	common.MachineEndian.PutUint32(stream, b.Header.version)
	tmp4B := make([]byte, 4)
	common.MachineEndian.PutUint32(tmp4B, b.Header.chainID)
	stream = append(stream, tmp4B...)
	tmp8B := make([]byte, 8)
	common.MachineEndian.PutUint64(tmp8B, uint64(b.Header.height))
	stream = append(stream, tmp8B...)
	common.MachineEndian.PutUint64(tmp8B, b.Header.timestamp)
	stream = append(stream, tmp8B...)
	stream = append(stream, b.Header.prevBlockHash[:]...)
	stream = append(stream, b.Header.txRoot[:]...)
	stream = append(stream, b.Header.stateRoot[:]...)
	common.MachineEndian.PutUint32(tmp4B, b.Header.trnxNumber)
	stream = append(stream, tmp4B...)
	common.MachineEndian.PutUint32(tmp4B, b.Header.trnxDataSize)
	stream = append(stream, tmp4B...)
	return stream
}

// ByteStream returns a byte stream of the block
// used to calculate the block hash
func (b *Block) ByteStream() []byte {
	stream := b.ByteStreamHeader()

	// Add the stream of blockSig
	stream = append(stream, b.Header.blockSig[:]...)

	// write all trnx
	for _, tx := range b.Tranxs {
		stream = append(stream, tx.ByteStream()...)
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
	pbHeader.TrnxNumber = b.Header.trnxNumber
	pbHeader.TrnxDataSize = b.Header.trnxDataSize
	pbHeader.Signature = b.Header.blockSig[:]
	return &pbHeader
}

// ConvertToBlockPb converts Block to BlockPb
func (b *Block) ConvertToBlockPb() *iproto.BlockPb {
	if len(b.Tranxs)+len(b.Votes) == 0 {
		return nil
	}

	actions := []*iproto.ActionPb{}
	for _, tx := range b.Tranxs {
		actions = append(actions, &iproto.ActionPb{&iproto.ActionPb_Tx{tx.ConvertToTxPb()}})
	}

	for _, vote := range b.Votes {
		actions = append(actions, &iproto.ActionPb{&iproto.ActionPb_Vote{vote}})
	}
	return &iproto.BlockPb{b.ConvertToBlockHeaderPb(), actions}
}

// Serialize returns the serialized byte stream of the block
func (b *Block) Serialize() ([]byte, error) {
	return proto.Marshal(b.ConvertToBlockPb())
}

// ConvertFromBlockHeaderPb converts BlockHeaderPb to BlockHeader
func (b *Block) ConvertFromBlockHeaderPb(pbBlock *iproto.BlockPb) {
	b.Header = nil
	b.Header = new(BlockHeader)

	b.Header.version = pbBlock.GetHeader().GetVersion()
	b.Header.chainID = pbBlock.GetHeader().GetChainID()
	b.Header.height = pbBlock.GetHeader().GetHeight()
	b.Header.timestamp = pbBlock.GetHeader().GetTimestamp()
	copy(b.Header.prevBlockHash[:], pbBlock.GetHeader().GetPrevBlockHash())
	copy(b.Header.txRoot[:], pbBlock.GetHeader().GetTxRoot())
	copy(b.Header.stateRoot[:], pbBlock.GetHeader().GetStateRoot())
	b.Header.trnxNumber = pbBlock.GetHeader().GetTrnxNumber()
	b.Header.trnxDataSize = pbBlock.GetHeader().GetTrnxDataSize()
	b.Header.blockSig = pbBlock.GetHeader().GetSignature()
}

// ConvertFromBlockPb converts BlockPb to Block
func (b *Block) ConvertFromBlockPb(pbBlock *iproto.BlockPb) {
	b.ConvertFromBlockHeaderPb(pbBlock)

	b.Tranxs = nil
	b.Votes = nil
	hasTrnx := false
	hasVote := false

	for _, action := range pbBlock.Actions {
		if txPb := action.GetTx(); txPb != nil {
			if !hasTrnx {
				b.Tranxs = []*Tx{}
				hasTrnx = true
			}
			tx := Tx{}
			tx.ConvertFromTxPb(txPb)
			b.Tranxs = append(b.Tranxs, &tx)
			continue
		}

		if votePb := action.GetVote(); votePb != nil {
			if !hasVote {
				b.Votes = []*iproto.VotePb{}
				hasVote = true
			}
			b.Votes = append(b.Votes, votePb)
			continue
		}
	}
}

// Deserialize parse the byte stream into Block
func (b *Block) Deserialize(buf []byte) error {
	pbBlock := iproto.BlockPb{}
	if err := proto.Unmarshal(buf, &pbBlock); err != nil {
		return err
	}

	b.ConvertFromBlockPb(&pbBlock)

	// verify merkle root can match after deserialize
	txroot := b.TxRoot()
	if bytes.Compare(b.Header.txRoot[:], txroot[:]) != 0 {
		return errors.New("Failed to match merkle root after deserialize")
	}
	return nil
}

// TxRoot returns the Merkle root of all transactions in this block.
func (b *Block) TxRoot() common.Hash32B {
	// create hash list of all trnx
	var txHash []common.Hash32B
	for _, tx := range b.Tranxs {
		txHash = append(txHash, tx.Hash())
	}
	return cp.NewMerkleTree(txHash).HashTree()
}

// HashBlock return the hash of this block (actually hash of block header)
func (b *Block) HashBlock() common.Hash32B {
	hash := blake2b.Sum256(b.ByteStreamHeader())
	hash = blake2b.Sum256(hash[:])
	return hash
}
