// Copyright (c) 2019 IoTeX Foundation
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

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// Block defines the struct of block
type Block struct {
	Header
	Body
	Footer

	// TODO: move receipts out of block struct
	Receipts   []*action.Receipt
	WorkingSet factory.WorkingSet
}

// ConvertToBlockHeaderPb converts BlockHeader to BlockHeader
func (b *Block) ConvertToBlockHeaderPb() *iotextypes.BlockHeader {
	return b.Header.BlockHeaderProto()
}

// ConvertToBlockPb converts Block to Block
func (b *Block) ConvertToBlockPb() *iotextypes.Block {
	footer, err := b.ConvertToBlockFooterPb()
	if err != nil {
		log.L().Panic("failed to convert block footer to protobuf message")
	}
	return &iotextypes.Block{
		Header: b.ConvertToBlockHeaderPb(),
		Body:   b.Body.Proto(),
		Footer: footer,
	}
}

// Serialize returns the serialized byte stream of the block
func (b *Block) Serialize() ([]byte, error) {
	return proto.Marshal(b.ConvertToBlockPb())
}

// ConvertFromBlockPb converts Block to Block
func (b *Block) ConvertFromBlockPb(pbBlock *iotextypes.Block) error {
	b.Header = Header{}
	if err := b.Header.LoadFromBlockHeaderProto(pbBlock.GetHeader()); err != nil {
		return err
	}
	b.Body = Body{}
	if err := b.Body.LoadProto(pbBlock.GetBody()); err != nil {
		return err
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
	b.Receipts = nil

	// verify merkle root can match after deserialize
	txroot := b.CalculateTxRoot()
	if !bytes.Equal(b.Header.txRoot[:], txroot[:]) {
		return errors.New("Failed to match merkle root after deserialize")
	}
	return nil
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

// VerifyReceiptRoot verifies the receipt root in header
func (b *Block) VerifyReceiptRoot(root hash.Hash256) error {
	if b.Header.receiptRoot != root {
		return errors.New("receipt root hash does not match")
	}
	return nil
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
func (b *Block) Finalize(endorsements []*endorsement.Endorsement, ts time.Time) error {
	if len(b.endorsements) != 0 {
		return errors.New("the block has been finalized")
	}
	b.endorsements = endorsements
	b.commitTime = ts

	return nil
}
