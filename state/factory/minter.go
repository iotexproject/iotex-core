// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

type MintOption func(*minter)

// WithTimeoutOption sets the timeout for NewBlockBuilder
func WithTimeoutOption(timeout time.Duration) MintOption {
	return func(m *minter) {
		m.timeout = timeout
	}
}

type minter struct {
	f       Factory
	ap      actpool.ActPool
	timeout time.Duration
}

// NewMinter creates a wrapper instance
func NewMinter(f Factory, ap actpool.ActPool, opts ...MintOption) blockchain.BlockMinter {
	m := &minter{f: f, ap: ap}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// NewBlockBuilder implements the BlockMinter interface
func (m *minter) Mint(ctx context.Context, pk crypto.PrivateKey) (*block.Block, error) {
	if m.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, protocol.MustGetBlockCtx(ctx).BlockTimeStamp.Add(m.timeout))
		defer cancel()
	}
	return m.f.Mint(ctx, m.ap, pk)
}
