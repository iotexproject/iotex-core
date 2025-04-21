// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"sync"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

type MintOption func(*Minter)

// WithTimeoutOption sets the timeout for NewBlockBuilder
func WithTimeoutOption(timeout time.Duration) MintOption {
	return func(m *Minter) {
		m.timeout = timeout
	}
}

// Minter is a wrapper of Factory to mint blocks
type Minter struct {
	f             Factory
	ap            actpool.ActPool
	timeout       time.Duration
	blockPreparer *blockPreparer
	mu            sync.Mutex
}

// NewMinter creates a wrapper instance
func NewMinter(f Factory, ap actpool.ActPool, opts ...MintOption) *Minter {
	m := &Minter{
		f:             f,
		ap:            ap,
		blockPreparer: newBlockPreparer(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Mint creates a block with the given private key
func (m *Minter) Mint(ctx context.Context, pk crypto.PrivateKey) (*block.Block, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	return m.blockPreparer.PrepareOrWait(ctx, bcCtx.Tip.Hash[:], blkCtx.BlockTimeStamp, func() (*block.Block, error) {
		return m.mint(ctx, pk)
	})
}

// ReceiveBlock receives a confirmed block
func (m *Minter) ReceiveBlock(blk *block.Block) error {
	return m.blockPreparer.ReceiveBlock(blk)
}

func (m *Minter) mint(ctx context.Context, pk crypto.PrivateKey) (*block.Block, error) {
	if m.timeout > 0 {
		// set deadline for NewBlockBuilder
		// ensure that minting finishes before `block start time + timeout` and that its duration does not exceed the timeout.
		var (
			cancel context.CancelFunc
			now    = time.Now()
			blkTs  = protocol.MustGetBlockCtx(ctx).BlockTimeStamp
			ddl    = blkTs.Add(m.timeout)
		)
		if now.Before(blkTs) {
			ddl = now.Add(m.timeout)
		}
		ctx, cancel = context.WithDeadline(ctx, ddl)
		defer cancel()
	}
	return m.f.Mint(ctx, m.ap, pk)
}
