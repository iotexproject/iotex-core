// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

type MintOption func(*minter)

// WithTimeoutOption sets the timeout for NewBlockBuilder
func WithTimeoutOption(timeout time.Duration) MintOption {
	return func(m *minter) {
		m.timeout = timeout
	}
}

func WithPrivateKeyOption(sk crypto.PrivateKey) MintOption {
	return func(m *minter) {
		m.sk = sk
	}
}

type minter struct {
	f             Factory
	ap            actpool.ActPool
	timeout       time.Duration
	blockPreparer *blockPreparer
	proposalPool  *proposalPool
	sk            crypto.PrivateKey
	mu            sync.Mutex
}

// NewMinter creates a wrapper instance
func NewMinter(f Factory, ap actpool.ActPool, opts ...MintOption) blockchain.BlockBuilderFactory {
	m := &minter{
		f:             f,
		ap:            ap,
		blockPreparer: newBlockPreparer(),
		proposalPool:  newProposalPool(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *minter) Init(root hash.Hash256) {
	m.proposalPool.Init(root)
}

func (m *minter) Mint(ctx context.Context) (*block.Block, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	// create a new block
	blk, err := m.blockPreparer.PrepareOrWait(ctx, bcCtx.Tip.Hash[:], blkCtx.BlockTimeStamp, func() (*block.Block, error) {
		blk, err := m.mint(ctx)
		if err != nil {
			return nil, err
		}
		if err = m.proposalPool.AddBlock(blk); err != nil {
			log.L().Error("failed to add block to proposal pool", zap.Error(err))
		}
		return blk, nil
	})
	if err != nil {
		return nil, err
	}
	return blk, nil
}

func (m *minter) AddProposal(blk *block.Block) error {
	return m.proposalPool.AddBlock(blk)
}

func (m *minter) ReceiveBlock(blk *block.Block) error {
	prevHash := blk.PrevHash()
	l := log.L().With(zap.Uint64("height", blk.Height()), log.Hex("prevHash", prevHash[:]), zap.Time("timestamp", blk.Timestamp()))
	if err := m.blockPreparer.ReceiveBlock(blk); err != nil {
		l.Error("failed to receive block", zap.Error(err))
	}
	if err := m.proposalPool.ReceiveBlock(blk); err != nil {
		l.Error("failed to receive block", zap.Error(err))
	}
	return nil
}

func (m *minter) Block(hash hash.Hash256) *block.Block {
	return m.proposalPool.BlockByHash(hash)
}

func (m *minter) BlockByHeight(height uint64) *block.Block {
	return m.proposalPool.Block(height)
}

func (m *minter) mint(ctx context.Context) (*block.Block, error) {
	builder, err := m.newBlockBuilder(ctx, func(e action.Envelope) (*action.SealedEnvelope, error) {
		return action.Sign(e, m.sk)
	})
	if err != nil {
		return nil, err
	}
	blk, err := builder.SignAndBuild(m.sk)
	if err != nil {
		return nil, err
	}
	return &blk, nil
}

func (m *minter) newBlockBuilder(ctx context.Context, sign func(action.Envelope) (*action.SealedEnvelope, error)) (*block.Builder, error) {
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
	return m.f.NewBlockBuilder(ctx, m.ap, sign)
}
