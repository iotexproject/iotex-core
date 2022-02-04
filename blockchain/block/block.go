// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"encoding/hex"
	"time"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// Block defines the struct of block
type Block struct {
	Header
	Body
	Footer

	// TODO: move receipts out of block struct
	Receipts []*action.Receipt
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
	b.Receipts = nil

	// verify merkle root can match after deserialize
	if err := b.VerifyTxRoot(); err != nil {
		return err
	}
	return nil
}

// VerifyTxRoot verifies the transaction root hash
func (b *Block) VerifyTxRoot() error {
	root, err := b.CalculateTxRoot()
	if err != nil {
		log.L().Debug("error in getting hash", zap.Error(err))
		return err
	}
	if !b.Header.VerifyTransactionRoot(root) {
		return ErrTxRootMismatch
	}
	return nil
}

// RunnableActions abstructs RunnableActions from a Block.
func (b *Block) RunnableActions() RunnableActions {
	return RunnableActions{actions: b.Actions, txHash: b.txRoot}
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

// TransactionLog returns transaction logs in the block
func (b *Block) TransactionLog() *BlkTransactionLog {
	if len(b.Receipts) == 0 {
		return nil
	}

	blkLog := BlkTransactionLog{
		actionLogs: []*TransactionLog{},
	}
	for _, r := range b.Receipts {
		if log := ReceiptTransactionLog(r); log != nil {
			blkLog.actionLogs = append(blkLog.actionLogs, log)
		}
	}

	if len(blkLog.actionLogs) == 0 {
		return nil
	}
	return &blkLog
}

// ActionHashs returns action hashs in the block
func (b *Block) ActionHashs() []string {
	actHash := make([]string, len(b.Actions))
	for i := range b.Actions {
		h, err := b.Actions[i].Hash()
		if err != nil {
			log.L().Debug("Skipping action due to hash error", zap.Error(err))
			continue
		}
		actHash[i] = hex.EncodeToString(h[:])
	}
	return actHash
}
