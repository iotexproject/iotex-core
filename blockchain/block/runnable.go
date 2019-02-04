// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

// RunnableActions is abstructed from block which contains information to execute all actions in a block.
type RunnableActions struct {
	blockHeight         uint64
	blockTimeStamp      int64
	blockProducerPubKey []byte
	blockProducerAddr   string
	txHash              hash.Hash32B
	actions             []action.SealedEnvelope
}

// BlockHeight returns block height.
func (ra RunnableActions) BlockHeight() uint64 {
	return ra.blockHeight
}

// BlockTimeStamp returns blockTimeStamp.
func (ra RunnableActions) BlockTimeStamp() int64 {
	return ra.blockTimeStamp
}

// BlockProducerPubKey return BlockProducerPubKey.
func (ra RunnableActions) BlockProducerPubKey() []byte {
	return ra.blockProducerPubKey
}

// BlockProducerAddr returns BlockProducerAddr.
func (ra RunnableActions) BlockProducerAddr() string {
	return ra.blockProducerAddr
}

// TxHash returns TxHash.
func (ra RunnableActions) TxHash() hash.Hash32B { return ra.txHash }

// Actions returns Actions.
func (ra RunnableActions) Actions() []action.SealedEnvelope {
	return ra.actions
}

// RunnableActionsBuilder is used to construct RunnableActions.
type RunnableActionsBuilder struct{ ra RunnableActions }

// NewRunnableActionsBuilder creates a RunnableActionsBuilder.
func NewRunnableActionsBuilder() *RunnableActionsBuilder { return &RunnableActionsBuilder{} }

// SetHeight sets the block height for block which is building.
func (b *RunnableActionsBuilder) SetHeight(h uint64) *RunnableActionsBuilder {
	b.ra.blockHeight = h
	return b
}

// SetTimeStamp sets the time stamp for block which is building.
func (b *RunnableActionsBuilder) SetTimeStamp(ts int64) *RunnableActionsBuilder {
	b.ra.blockTimeStamp = ts
	return b
}

// AddActions adds actions for block which is building.
func (b *RunnableActionsBuilder) AddActions(acts ...action.SealedEnvelope) *RunnableActionsBuilder {
	if b.ra.actions == nil {
		b.ra.actions = make([]action.SealedEnvelope, 0)
	}
	b.ra.actions = append(b.ra.actions, acts...)
	return b
}

// Build signs and then builds a block.
func (b *RunnableActionsBuilder) Build(producerAddr string, producerPubKey []byte) RunnableActions {
	b.ra.blockProducerAddr = producerAddr
	b.ra.blockProducerPubKey = producerPubKey
	b.ra.txHash = calculateTxRoot(b.ra.actions)
	return b.ra
}
