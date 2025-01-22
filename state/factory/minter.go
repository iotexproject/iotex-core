// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"time"

	"github.com/iotexproject/iotex-core/v2/action"
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
func NewMinter(f Factory, ap actpool.ActPool, opts ...MintOption) blockchain.BlockBuilderFactory {
	m := &minter{f: f, ap: ap}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// NewBlockBuilder implements the BlockMinter interface
func (m *minter) NewBlockBuilder(ctx context.Context, sign func(action.Envelope) (*action.SealedEnvelope, error)) (*block.Builder, error) {
	if m.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, protocol.MustGetBlockCtx(ctx).BlockTimeStamp.Add(m.timeout))
		defer cancel()
	}
	return m.f.NewBlockBuilder(ctx, m.ap, sign)
}

func (m *minter) OngoingBlockHeight() uint64 {
	return m.f.OngoingBlockHeight()
}

func (m *minter) PendingBlockHeader(height uint64) (*block.Header, error) {
	return m.f.PendingBlockHeader(height)
}

func (m *minter) PutBlockHeader(header *block.Header) {
	m.f.PutBlockHeader(header)
}

func (m *minter) CancelBlock(height uint64) {
	m.f.CancelBlock(height)
}
