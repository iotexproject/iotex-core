// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"bytes"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/iotexproject/go-ethereum/crypto"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
	"github.com/iotexproject/iotex-core/state/factory"
)

// Block defines the struct of block
type Block struct {
	Header
	Footer

	Actions []action.SealedEnvelope
	// TODO: move receipts out of block struct
	Receipts []*action.Receipt

	WorkingSet factory.WorkingSet
}

// ByteStream returns a byte stream of the block
func (b *Block) ByteStream() []byte {
	stream := b.Header.ByteStream()

	// Add the stream of blockSig
	stream = append(stream, b.Header.blockSig...)

	for _, act := range b.Actions {
		stream = append(stream, act.ByteStream()...)
	}
	return stream
}

// ConvertToBlockHeaderPb converts BlockHeader to BlockHeader
func (b *Block) ConvertToBlockHeaderPb() *iotextypes.BlockHeader {
	pbHeader := iotextypes.BlockHeader{}

	pbHeader.Version = b.Header.version
	pbHeader.Height = b.Header.height
	pbHeader.Timestamp = &timestamp.Timestamp{
		Seconds: b.Header.Timestamp(),
	}
	pbHeader.PrevBlockHash = b.Header.prevBlockHash[:]
	pbHeader.TxRoot = b.Header.txRoot[:]
	pbHeader.DeltaStateDigest = b.Header.deltaStateDigest[:]
	pbHeader.ReceiptRoot = b.Header.receiptRoot[:]
	pbHeader.Signature = b.Header.blockSig
	pbHeader.Pubkey = keypair.PublicKeyToBytes(b.Header.pubkey)
	return &pbHeader
}

// ConvertToBlockPb converts Block to Block
func (b *Block) ConvertToBlockPb() *iotextypes.Block {
	actions := []*iotextypes.Action{}
	for _, act := range b.Actions {
		actions = append(actions, act.Proto())
	}
	return &iotextypes.Block{
		Header:  b.ConvertToBlockHeaderPb(),
		Actions: actions,
		Footer:  b.ConvertToBlockFooterPb(),
	}
}

// Serialize returns the serialized byte stream of the block
func (b *Block) Serialize() ([]byte, error) {
	return proto.Marshal(b.ConvertToBlockPb())
}

// ConvertFromBlockHeaderPb converts BlockHeader to BlockHeader
func (b *Block) ConvertFromBlockHeaderPb(pbBlock *iotextypes.Block) {
	b.Header = Header{}

	b.Header.version = pbBlock.GetHeader().GetVersion()
	b.Header.height = pbBlock.GetHeader().GetHeight()
	b.Header.timestamp = pbBlock.GetHeader().GetTimestamp().GetSeconds()
	copy(b.Header.prevBlockHash[:], pbBlock.GetHeader().GetPrevBlockHash())
	copy(b.Header.txRoot[:], pbBlock.GetHeader().GetTxRoot())
	copy(b.Header.deltaStateDigest[:], pbBlock.GetHeader().GetDeltaStateDigest())
	copy(b.Header.receiptRoot[:], pbBlock.GetHeader().GetReceiptRoot())
	b.Header.blockSig = pbBlock.GetHeader().GetSignature()

	pubKey, err := keypair.BytesToPublicKey(pbBlock.GetHeader().GetPubkey())
	if err != nil {
		log.L().Panic("Failed to unmarshal public key.", zap.Error(err))
	}
	b.Header.pubkey = pubKey
}

// ConvertFromBlockPb converts Block to Block
func (b *Block) ConvertFromBlockPb(pbBlock *iotextypes.Block) error {
	b.ConvertFromBlockHeaderPb(pbBlock)

	b.Actions = []action.SealedEnvelope{}

	for _, actPb := range pbBlock.Actions {
		act := action.SealedEnvelope{}
		if err := act.LoadProto(actPb); err != nil {
			return err
		}
		b.Actions = append(b.Actions, act)
	}

	return b.ConvertFromBlockFooterPb(pbBlock.GetFooter())
}

// Deserialize parses the byte stream into a Block
func (b *Block) Deserialize(buf []byte) error {
	pbBlock := iotextypes.Block{}
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
func (b *Block) CalculateTxRoot() hash.Hash256 {
	return calculateTxRoot(b.Actions)
}

// HashBlock return the hash of this block (actually hash of block header)
func (b *Block) HashBlock() hash.Hash256 {
	return blake2b.Sum256(b.Header.ByteStream())
}

// VerifyDeltaStateDigest verifies the delta state digest in header
func (b *Block) VerifyDeltaStateDigest(digest hash.Hash256) error {
	if b.Header.deltaStateDigest != digest {
		return errors.Errorf(
			"delta state digest doesn't match, expected = %x, actual = %x",
			b.Header.deltaStateDigest,
			digest,
		)
	}
	return nil
}

// VerifySignature verifies the signature saved in block header
func (b *Block) VerifySignature() bool {
	blkHash := b.HashBlock()

	if len(b.Header.blockSig) != action.SignatureLength {
		return false
	}
	return crypto.VerifySignature(keypair.PublicKeyToBytes(b.Header.pubkey), blkHash[:],
		b.Header.blockSig[:action.SignatureLength-1])
}

// VerifyReceiptRoot verifies the receipt root in header
func (b *Block) VerifyReceiptRoot(root hash.Hash256) error {
	if b.Header.receiptRoot != root {
		return errors.New("receipt root hash does not match")
	}
	return nil
}

// ProducerAddress returns the address of producer
func (b *Block) ProducerAddress() string {
	pkHash := keypair.HashPubKey(b.Header.pubkey)
	addr, _ := address.FromBytes(pkHash[:])
	return addr.String()
}

// RunnableActions abstructs RunnableActions from a Block.
func (b *Block) RunnableActions() RunnableActions {
	pkHash := keypair.HashPubKey(b.Header.pubkey)
	addr, _ := address.FromBytes(pkHash[:])
	return RunnableActions{
		blockHeight:         b.Header.height,
		blockTimeStamp:      b.Header.timestamp,
		blockProducerPubKey: b.Header.pubkey,
		blockProducerAddr:   addr.String(),
		actions:             b.Actions,
		txHash:              b.txRoot,
	}
}

// Finalize creates a footer for the block
func (b *Block) Finalize(set *endorsement.Set, ts time.Time) error {
	if b.endorsements != nil {
		return errors.New("the block has been finalized")
	}
	if set == nil {
		return errors.New("endorsement set is nil")
	}
	commitEndorsements, err := set.SubSet(endorsement.COMMIT)
	if err != nil {
		return err
	}
	b.endorsements = commitEndorsements
	b.commitTimestamp = ts.Unix()

	return nil
}

// FooterLogger logs the endorsements in block footer
func (b *Block) FooterLogger(l *zap.Logger) *zap.Logger {
	if b.endorsements == nil {
		h := b.HashBlock()
		return l.With(
			log.Hex("blockHash", h[:]),
			zap.Uint64("blockHeight", b.Height()),
			zap.Int("numOfEndorsements", 0),
		)
	}
	return b.endorsements.EndorsementsLogger(l)
}
