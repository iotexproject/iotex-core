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

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/blockchain/trx"
	"github.com/iotexproject/iotex-core/common"
	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/proto"
)

// Payee defines the struct of payee
type Payee struct {
	Address string
	Amount  uint64
}

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
	blockSig      []byte         // block signature
}

// Block defines the struct of block
type Block struct {
	Header    *BlockHeader
	Tranxs    []*trx.Tx
	Transfers []*action.Transfer
	Votes     []*action.Vote
}

// NewBlock returns a new block
func NewBlock(chainID uint32, height uint64, prevBlockHash common.Hash32B,
	txs []*trx.Tx, tsf []*action.Transfer, vote []*action.Vote) *Block {
	block := &Block{
		Header: &BlockHeader{
			version:       common.ProtocolVersion,
			chainID:       chainID,
			height:        height,
			timestamp:     uint64(time.Now().Unix()),
			prevBlockHash: prevBlockHash,
			txRoot:        common.ZeroHash32B,
			stateRoot:     common.ZeroHash32B,
		},
		Tranxs:    txs,
		Transfers: tsf,
		Votes:     vote,
	}

	block.Header.txRoot = block.TxRoot()
	return block
}

// Height returns the height of this block
func (b *Block) Height() uint64 {
	return b.Header.height
}

// PrevHash returns the hash of prev block
func (b *Block) PrevHash() common.Hash32B {
	return b.Header.prevBlockHash
}

// ByteStreamHeader returns a byte stream of the block header
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
	stream = append(stream, b.Header.blockSig...)
	return stream
}

// ByteStream returns a byte stream of the block
func (b *Block) ByteStream() []byte {
	stream := b.ByteStreamHeader()

	// Add the stream of blockSig
	stream = append(stream, b.Header.blockSig[:]...)

	for _, t := range b.Tranxs {
		stream = append(stream, t.ByteStream()...)
	}
	for _, t := range b.Transfers {
		stream = append(stream, t.ByteStream()...)
	}
	for _, v := range b.Votes {
		stream = append(stream, v.ByteStream()...)
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
	pbHeader.Signature = b.Header.blockSig[:]
	return &pbHeader
}

// ConvertToBlockPb converts Block to BlockPb
func (b *Block) ConvertToBlockPb() *iproto.BlockPb {
	if len(b.Tranxs)+len(b.Transfers)+len(b.Votes) == 0 {
		return nil
	}

	actions := []*iproto.ActionPb{}
	for _, tx := range b.Tranxs {
		actions = append(actions, &iproto.ActionPb{&iproto.ActionPb_Tx{tx.ConvertToTxPb()}})
	}
	for _, tsf := range b.Transfers {
		actions = append(actions, &iproto.ActionPb{&iproto.ActionPb_Transfer{tsf.ConvertToTransferPb()}})
	}
	for _, vote := range b.Votes {
		actions = append(actions, &iproto.ActionPb{&iproto.ActionPb_Vote{vote.ConvertToVotePb()}})
	}
	return &iproto.BlockPb{b.ConvertToBlockHeaderPb(), actions}
}

// Serialize returns the serialized byte stream of the block
func (b *Block) Serialize() ([]byte, error) {
	return proto.Marshal(b.ConvertToBlockPb())
}

// ConvertFromBlockHeaderPb converts BlockHeaderPb to BlockHeader
func (b *Block) ConvertFromBlockHeaderPb(pbBlock *iproto.BlockPb) {
	b.Header = new(BlockHeader)

	b.Header.version = pbBlock.GetHeader().GetVersion()
	b.Header.chainID = pbBlock.GetHeader().GetChainID()
	b.Header.height = pbBlock.GetHeader().GetHeight()
	b.Header.timestamp = pbBlock.GetHeader().GetTimestamp()
	copy(b.Header.prevBlockHash[:], pbBlock.GetHeader().GetPrevBlockHash())
	copy(b.Header.txRoot[:], pbBlock.GetHeader().GetTxRoot())
	copy(b.Header.stateRoot[:], pbBlock.GetHeader().GetStateRoot())
	b.Header.blockSig = pbBlock.GetHeader().GetSignature()
}

// ConvertFromBlockPb converts BlockPb to Block
func (b *Block) ConvertFromBlockPb(pbBlock *iproto.BlockPb) {
	b.ConvertFromBlockHeaderPb(pbBlock)

	b.Tranxs = []*trx.Tx{}
	b.Transfers = []*action.Transfer{}
	b.Votes = []*action.Vote{}

	for _, act := range pbBlock.Actions {
		if tfPb := act.GetTransfer(); tfPb != nil {
			tf := &action.Transfer{}
			tf.ConvertFromTransferPb(tfPb)
			b.Transfers = append(b.Transfers, tf)
		} else if txPb := act.GetTx(); txPb != nil {
			tx := &trx.Tx{}
			tx.ConvertFromTxPb(txPb)
			b.Tranxs = append(b.Tranxs, tx)
		} else if votePb := act.GetVote(); votePb != nil {
			vote := &action.Vote{}
			vote.ConvertFromVotePb(votePb)
			b.Votes = append(b.Votes, vote)
		} else {
			logger.Fatal().Msg("unexpected action")
		}
	}
}

// Deserialize parses the byte stream into a Block
func (b *Block) Deserialize(buf []byte) error {
	pbBlock := iproto.BlockPb{}
	if err := proto.Unmarshal(buf, &pbBlock); err != nil {
		return err
	}

	b.ConvertFromBlockPb(&pbBlock)

	// verify merkle root can match after deserialize
	txroot := b.TxRoot()
	if !bytes.Equal(b.Header.txRoot[:], txroot[:]) {
		return errors.New("Failed to match merkle root after deserialize")
	}
	return nil
}

// TxRoot returns the Merkle root of all txs and actions in this block.
func (b *Block) TxRoot() common.Hash32B {
	var hash []common.Hash32B
	for _, t := range b.Tranxs {
		hash = append(hash, t.Hash())
	}
	for _, t := range b.Transfers {
		hash = append(hash, t.Hash())
	}
	for _, v := range b.Votes {
		hash = append(hash, v.Hash())
	}

	if len(hash) == 0 {
		return common.ZeroHash32B
	}
	return cp.NewMerkleTree(hash).HashTree()
}

// HashBlock return the hash of this block (actually hash of block header)
func (b *Block) HashBlock() common.Hash32B {
	hash := blake2b.Sum256(b.ByteStreamHeader())
	hash = blake2b.Sum256(hash[:])
	return hash
}
