// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"math/big"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action"
)

// Body defines the struct of body
type Body struct {
	Actions []action.SealedEnvelope
}

// Proto converts Body to Protobuf
func (b *Body) Proto() *iotextypes.BlockBody {
	actions := []*iotextypes.Action{}
	for _, act := range b.Actions {
		actions = append(actions, act.Proto())
	}
	return &iotextypes.BlockBody{
		Actions: actions,
	}
}

// Serialize returns the serialized byte stream of the block
func (b *Body) Serialize() ([]byte, error) {
	return proto.Marshal(b.Proto())
}

// LoadProto loads body from proto
func (b *Body) LoadProto(pbBlock *iotextypes.BlockBody) error {
	b.Actions = []action.SealedEnvelope{}
	for _, actPb := range pbBlock.Actions {
		act := action.SealedEnvelope{}
		if err := act.LoadProto(actPb); err != nil {
			return err
		}
		b.Actions = append(b.Actions, act)
	}
	return nil
}

// LoadProtoWithChainID loads body from proto
func (b *Body) LoadProtoWithChainID(pbBlock *iotextypes.BlockBody) error {
	b.Actions = []action.SealedEnvelope{}
	for _, actPb := range pbBlock.Actions {
		act := action.SealedEnvelope{}
		if err := act.LoadProtoWithChainID(actPb); err != nil {
			return err
		}
		b.Actions = append(b.Actions, act)
	}
	return nil
}

// Deserialize parses the byte stream into a Block
func (b *Body) Deserialize(buf []byte) error {
	pb := iotextypes.BlockBody{}
	if err := proto.Unmarshal(buf, &pb); err != nil {
		return err
	}

	return b.LoadProto(&pb)
}

// CalculateTxRoot returns the Merkle root of all txs and actions in this block.
func (b *Body) CalculateTxRoot() (hash.Hash256, error) {
	return calculateTxRoot(b.Actions)
}

// CalculateTransferAmount returns the calculated transfer amount in this block.
func (b *Body) CalculateTransferAmount() *big.Int {
	return calculateTransferAmount(b.Actions)
}
