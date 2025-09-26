// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package block

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/endorsement"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

type BlockValidator func(context.Context, *Block) error

// Block defines the struct of block
type Block struct {
	Header
	Body
	Footer

	// TODO: move receipts out of block struct
	Receipts []*action.Receipt
}

// ConvertToBlockPb converts Block to Block
func (b *Block) ConvertToBlockPb() *iotextypes.Block {
	return &iotextypes.Block{
		Header: b.Header.Proto(),
		Body:   b.Body.Proto(),
		Footer: b.Footer.Proto(),
	}
}

// ProtoWithoutSidecar returns the protobuf without sidecar
func (b *Block) ProtoWithoutSidecar() *iotextypes.Block {
	pb := b.ConvertToBlockPb()
	for _, v := range pb.GetBody().GetActions() {
		blobdata := v.GetCore().GetBlobTxData()
		if blobdata != nil {
			blobdata.BlobTxSidecar = nil
		}
	}
	return pb
}

// Serialize returns the serialized byte stream of the block
func (b *Block) Serialize() ([]byte, error) {
	return proto.Marshal(b.ConvertToBlockPb())
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

// ActionByHash returns the action of a given hash
func (b *Block) ActionByHash(h hash.Hash256) (*action.SealedEnvelope, uint32, error) {
	for i, act := range b.Actions {
		actHash, err := act.Hash()
		if err != nil {
			return nil, 0, errors.Errorf("hash failed for action %d", i)
		}
		if actHash == h {
			return act, uint32(i), nil
		}
	}
	return nil, 0, errors.Errorf("block does not have action %x", h)
}

// HasBlob returns whether the block contains blobs
func (b *Block) HasBlob() bool {
	for _, act := range b.Actions {
		if act.BlobTxSidecar() != nil {
			return true
		}
	}
	return false
}

func (b *Block) WithBlobSidecars(sidecars []*types.BlobTxSidecar, txhash []string, deser *action.Deserializer) (*Block, error) {
	scMap := make(map[hash.Hash256]*types.BlobTxSidecar)
	for i := range txhash {
		h, err := hash.HexStringToHash256(txhash[i])
		if err != nil {
			return nil, err
		}
		scMap[h] = sidecars[i]
	}
	for i, act := range b.Actions {
		h, _ := act.Hash()
		if sc, ok := scMap[h]; ok {
			// add the sidecar to this action
			pb := act.Proto()
			blobData := pb.GetCore().GetBlobTxData()
			if blobData == nil {
				// this is not a blob tx, something's wrong
				return nil, errors.Wrap(action.ErrInvalidAct, "tx is not blob type")
			}
			blobData.BlobTxSidecar = action.ToProtoSideCar(sc)
			actWithBlob, err := deser.ActionToSealedEnvelope(pb)
			if err != nil {
				return nil, err
			}
			b.Actions[i] = actWithBlob
		}
	}
	return b, nil
}
