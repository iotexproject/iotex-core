// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
)

type minter struct {
	f  Factory
	ap actpool.ActPool
}

// NewMinter creates a wrapper instance
func NewMinter(f Factory, ap actpool.ActPool) blockchain.BlockBuilderFactory {
	return &minter{f: f, ap: ap}
}

// NewBlockBuilder implements the BlockMinter interface
func (m *minter) NewBlockBuilder(ctx context.Context, sign func(action.Envelope) (action.SealedEnvelope, error)) (*block.Builder, error) {
	return m.f.NewBlockBuilder(ctx, m.ap, sign)
}
