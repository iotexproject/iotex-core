// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
)

// RunnableActions is abstructed from block which contains information to execute all actions in a block.
type RunnableActions struct {
	txHash  hash.Hash256
	actions []action.SealedEnvelope
}

// TxHash returns TxHash.
func (ra RunnableActions) TxHash() hash.Hash256 { return ra.txHash }

// Actions returns Actions.
func (ra RunnableActions) Actions() []action.SealedEnvelope {
	return ra.actions
}

// RunnableActionsBuilder is used to construct RunnableActions.
type RunnableActionsBuilder struct{ ra RunnableActions }

// NewRunnableActionsBuilder creates a RunnableActionsBuilder.
func NewRunnableActionsBuilder() *RunnableActionsBuilder { return &RunnableActionsBuilder{} }

// AddActions adds actions for block which is building.
func (b *RunnableActionsBuilder) AddActions(acts ...action.SealedEnvelope) *RunnableActionsBuilder {
	if b.ra.actions == nil {
		b.ra.actions = make([]action.SealedEnvelope, 0)
	}
	b.ra.actions = append(b.ra.actions, acts...)
	return b
}

// Build signs and then builds a block.
func (b *RunnableActionsBuilder) Build() RunnableActions {
	var err error
	b.ra.txHash, err = calculateTxRoot(b.ra.actions)
	if err != nil {
		log.L().Debug("error in getting hash ", zap.Error(err))
		return RunnableActions{}
	}
	return b.ra
}
