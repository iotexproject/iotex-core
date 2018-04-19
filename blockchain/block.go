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

	cm "github.com/iotexproject/iotex-core/common"
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
	version       uint32     // version
	chainID       uint32     // this chain's ID
	height        uint32     // block height
	timestamp     uint64     // timestamp
	prevBlockHash cp.Hash32B // hash of previous block
	merkleRoot    cp.Hash32B // merkle root of all trn
	trnxNumber    uint32     // number of transaction in this block
	trnxDataSize  uint32     // size (in bytes) of transaction data in this block
}

// Block defines the struct of block
// make sure the variable type and order of this struct is same as "type Block" in blockchain.pb.go
type Block struct {
	Header *BlockHeader
	Tranxs []*Tx
}

// NewBlock returns a new block
func NewBlock(chainID uint32, height uint32, prevBlockHash cp.Hash32B, transactions []*Tx) *Block {
	block := &Block{
		Header: &BlockHeader{Version, chainID, height, uint64(time.Now().Unix()), prevBlockHash, cp.ZeroHash32B, uint32(len(transactions)), 0},
		Tranxs: transactions,
	}

	block.Header.merkleRoot = block.MerkleRoot()
	for _, tx := range transactions {
		// add up trnx size
		block.Header.trnxDataSize += tx.TotalSize()
	}
	return block
}

// Height returns the height of this block
func (b *Block) Height() uint32 {
	return b.Header.height
}

// TranxsSize returns the size of transactions in this block
func (b *Block) TranxsSize() uint32 {
	return b.Header.trnxDataSize
}

// PrevHash returns the hash of prev block
func (b *Block) PrevHash() cp.Hash32B {
	return b.Header.prevBlockHash
}

// ByteStream returns a byte stream of the block
// used to calculate the block hash
func (b *Block) ByteStream() []byte {
	stream := make([]byte, 4)
	cm.MachineEndian.PutUint32(stream, b.Header.version)

	temp := make([]byte, 4)
	cm.MachineEndian.PutUint32(temp, b.Header.chainID)
	stream = append(stream, temp...)
	cm.MachineEndian.PutUint32(temp, b.Header.height)
	stream = append(stream, temp...)

	time := make([]byte, 8)
	cm.MachineEndian.PutUint64(time, b.Header.timestamp)
	stream = append(stream, time...)

	stream = append(stream, b.Header.prevBlockHash[:]...)
	stream = append(stream, b.Header.merkleRoot[:]...)

	cm.MachineEndian.PutUint32(temp, b.Header.trnxNumber)
	stream = append(stream, temp...)
	cm.MachineEndian.PutUint32(temp, b.Header.trnxDataSize)
	stream = append(stream, temp...)

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
	pbHeader.MerkleRoot = b.Header.merkleRoot[:]
	pbHeader.TrnxNumber = b.Header.trnxNumber
	pbHeader.TrnxDataSize = b.Header.trnxDataSize

	return &pbHeader
}

// ConvertToBlockPb converts Block to BlockPb
func (b *Block) ConvertToBlockPb() *iproto.BlockPb {
	tx := make([]*iproto.TxPb, len(b.Tranxs))
	for i, in := range b.Tranxs {
		tx[i] = in.ConvertToTxPb()
	}

	return &iproto.BlockPb{b.ConvertToBlockHeaderPb(), tx}
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
	copy(b.Header.merkleRoot[:], pbBlock.GetHeader().GetMerkleRoot())
	b.Header.trnxNumber = pbBlock.GetHeader().GetTrnxNumber()
	b.Header.trnxDataSize = pbBlock.GetHeader().GetTrnxDataSize()
}

// ConvertFromBlockPb converts BlockPb to Block
func (b *Block) ConvertFromBlockPb(pbBlock *iproto.BlockPb) {
	b.ConvertFromBlockHeaderPb(pbBlock)

	b.Tranxs = nil
	b.Tranxs = make([]*Tx, len(pbBlock.Transactions))
	for i, tx := range pbBlock.Transactions {
		b.Tranxs[i] = &Tx{}
		b.Tranxs[i].ConvertFromTxPb(tx)
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
	merkle := b.MerkleRoot()
	if bytes.Compare(b.Header.merkleRoot[:], merkle[:]) != 0 {
		return errors.New("Failed to match merkle root after deserialize")
	}

	return nil
}

// MerkleRoot returns the Merkle root of this block.
func (b *Block) MerkleRoot() cp.Hash32B {
	// create hash list of all trnx
	var txHash []cp.Hash32B
	for _, tx := range b.Tranxs {
		txHash = append(txHash, tx.Hash())
	}

	return cp.NewMerkleTree(txHash).HashTree()
}

// HashBlock return the hash of this block (actually hash of block header)
func (b *Block) HashBlock() cp.Hash32B {
	stream := make([]byte, 4)
	cm.MachineEndian.PutUint32(stream, b.Header.version)
	tmp4B := make([]byte, 4)
	cm.MachineEndian.PutUint32(tmp4B, b.Header.chainID)
	stream = append(stream, tmp4B...)
	cm.MachineEndian.PutUint32(tmp4B, b.Header.height)
	stream = append(stream, tmp4B...)
	tmp8B := make([]byte, 8)
	cm.MachineEndian.PutUint64(tmp8B, b.Header.timestamp)
	stream = append(stream, tmp8B...)
	stream = append(stream, b.Header.prevBlockHash[:]...)
	stream = append(stream, b.Header.merkleRoot[:]...)
	cm.MachineEndian.PutUint32(tmp4B, b.Header.trnxNumber)
	stream = append(stream, tmp4B...)
	cm.MachineEndian.PutUint32(tmp4B, b.Header.trnxDataSize)
	stream = append(stream, tmp4B...)

	hash := blake2b.Sum256(stream)
	hash = blake2b.Sum256(hash[:])
	return hash
}
