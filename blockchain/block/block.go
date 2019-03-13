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
	"github.com/pkg/errors"
	"go.uber.org/zap"

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

// ConvertToBlockHeaderPb converts BlockHeader to BlockHeader
func (b *Block) ConvertToBlockHeaderPb() *iotextypes.BlockHeader {
	return b.Header.BlockHeaderProto()
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
func (b *Block) ConvertFromBlockHeaderPb(pbBlock *iotextypes.Block) error {
	b.Header = Header{}

	b.Header.version = pbBlock.GetHeader().GetCore().GetVersion()
	b.Header.height = pbBlock.GetHeader().GetCore().GetHeight()
	b.Header.timestamp = pbBlock.GetHeader().GetCore().GetTimestamp().GetSeconds()
	copy(b.Header.prevBlockHash[:], pbBlock.GetHeader().GetCore().GetPrevBlockHash())
	copy(b.Header.txRoot[:], pbBlock.GetHeader().GetCore().GetTxRoot())
	copy(b.Header.deltaStateDigest[:], pbBlock.GetHeader().GetCore().GetDeltaStateDigest())
	copy(b.Header.receiptRoot[:], pbBlock.GetHeader().GetCore().GetReceiptRoot())
	b.Header.blockSig = pbBlock.GetHeader().GetSignature()

	pubKey, err := keypair.BytesToPublicKey(pbBlock.GetHeader().GetProducerPubkey())
	if err != nil {
		return err
	}
	b.Header.pubkey = pubKey
	return nil
}

// ConvertFromBlockPb converts Block to Block
func (b *Block) ConvertFromBlockPb(pbBlock *iotextypes.Block) error {
	if err := b.ConvertFromBlockHeaderPb(pbBlock); err != nil {
		return err
	}

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
func (b *Block) HashBlock() hash.Hash256 { return b.Header.HashHeader() }

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
	h := b.Header.HashHeaderCore()

	if b.Header.pubkey == nil || len(b.Header.blockSig) != action.SignatureLength {
		return false
	}
	return b.Header.pubkey.Verify(h[:], b.Header.blockSig)
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
	addr, _ := address.FromBytes(b.Header.pubkey.Hash())
	return addr.String()
}

// RunnableActions abstructs RunnableActions from a Block.
func (b *Block) RunnableActions() RunnableActions {
	return RunnableActions{
		blockHeight:         b.Header.height,
		blockTimeStamp:      b.Header.timestamp,
		blockProducerPubKey: b.Header.pubkey,
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
