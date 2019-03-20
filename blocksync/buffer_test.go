// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/test/mock/mock_consensus"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestBlockBufferFlush(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx := context.Background()
	cfg, err := newTestConfig()
	require.Nil(err)

	registry := protocol.Registry{}
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(registry.Register(rolldpos.ProtocolID, rp))
	chain := blockchain.NewBlockchain(
		cfg,
		blockchain.InMemStateFactoryOption(),
		blockchain.InMemDaoOption(),
		blockchain.RegistryOption(&registry),
	)
	vp := vote.NewProtocol(chain)
	require.NoError(registry.Register(vote.ProtocolID, vp))
	chain.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(chain, genesis.Default.ActionGasLimit))
	chain.Validator().AddActionValidators(account.NewProtocol())
	require.NoError(chain.Start(ctx))
	require.NotNil(chain)
	ap, err := actpool.NewActPool(chain, cfg.ActPool)
	require.NotNil(ap)
	require.Nil(err)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cs := mock_consensus.NewMockConsensus(ctrl)
	cs.EXPECT().ValidateBlockFooter(gomock.Any()).Return(nil).Times(1)
	cs.EXPECT().Calibrate(gomock.Any()).Times(1)
	defer func() {
		require.Nil(chain.Stop(ctx))
	}()

	b := blockBuffer{
		bc:         chain,
		ap:         ap,
		cs:         cs,
		blocks:     make(map[uint64]*block.Block),
		bufferSize: 16,
	}
	moved, re := b.Flush(nil)
	assert.Equal(false, moved)
	assert.Equal(bCheckinSkipNil, re)

	blk, err := chain.MintNewBlock(
		nil,
		testutil.TimestampNow(),
	)
	require.Nil(err)
	moved, re = b.Flush(blk)
	assert.Equal(true, moved)
	assert.Equal(bCheckinValid, re)

	blk = block.NewBlockDeprecated(
		uint32(123),
		uint64(0),
		hash.Hash256{},
		testutil.TimestampNow(),
		ta.Keyinfo["producer"].PubKey,
		nil,
	)
	moved, re = b.Flush(blk)
	assert.Equal(false, moved)
	assert.Equal(bCheckinLower, re)

	blk = block.NewBlockDeprecated(
		uint32(123),
		uint64(5),
		hash.Hash256{},
		testutil.TimestampNow(),
		ta.Keyinfo["producer"].PubKey,
		nil,
	)
	moved, re = b.Flush(blk)
	assert.Equal(false, moved)
	assert.Equal(bCheckinValid, re)

	blk = block.NewBlockDeprecated(
		uint32(123),
		uint64(5),
		hash.Hash256{},
		testutil.TimestampNow(),
		ta.Keyinfo["producer"].PubKey,
		nil,
	)
	moved, re = b.Flush(blk)
	assert.Equal(false, moved)
	assert.Equal(bCheckinExisting, re)

	blk = block.NewBlockDeprecated(
		uint32(123),
		uint64(500),
		hash.Hash256{},
		testutil.TimestampNow(),
		ta.Keyinfo["producer"].PubKey,
		nil,
	)
	moved, re = b.Flush(blk)
	assert.Equal(false, moved)
	assert.Equal(bCheckinHigher, re)
}

func TestBlockBufferGetBlocksIntervalsToSync(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx := context.Background()
	cfg, err := newTestConfig()
	require.Nil(err)
	registry := protocol.Registry{}
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(registry.Register(rolldpos.ProtocolID, rp))
	chain := blockchain.NewBlockchain(
		cfg,
		blockchain.InMemStateFactoryOption(),
		blockchain.InMemDaoOption(),
		blockchain.RegistryOption(&registry),
	)
	require.NotNil(chain)
	vp := vote.NewProtocol(chain)
	require.NoError(registry.Register(vote.ProtocolID, vp))
	require.NoError(chain.Start(ctx))
	ap, err := actpool.NewActPool(chain, cfg.ActPool)
	require.NotNil(ap)
	require.Nil(err)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cs := mock_consensus.NewMockConsensus(ctrl)
	cs.EXPECT().ValidateBlockFooter(gomock.Any()).Return(nil).Times(2)
	cs.EXPECT().Calibrate(gomock.Any()).Times(1)
	defer func() {
		require.Nil(chain.Stop(ctx))
	}()

	b := blockBuffer{
		bc:           chain,
		ap:           ap,
		cs:           cs,
		blocks:       make(map[uint64]*block.Block),
		bufferSize:   16,
		intervalSize: 8,
	}

	out := b.GetBlocksIntervalsToSync(32)
	require.Equal(2, len(out))
	require.Equal(uint64(1), out[0].Start)
	require.Equal(uint64(8), out[0].End)
	require.Equal(uint64(9), out[1].Start)
	require.Equal(uint64(16), out[1].End)

	b.intervalSize = 16

	out = b.GetBlocksIntervalsToSync(32)
	require.Equal(1, len(out))
	require.Equal(uint64(1), out[0].Start)
	require.Equal(uint64(16), out[0].End)

	out = b.GetBlocksIntervalsToSync(10)
	require.Equal(1, len(out))
	require.Equal(uint64(1), out[0].Start)
	require.Equal(uint64(10), out[0].End)

	blk := block.NewBlockDeprecated(
		uint32(123),
		uint64(2),
		hash.Hash256{},
		testutil.TimestampNow(),
		ta.Keyinfo["producer"].PubKey,
		nil,
	)
	moved, result := b.Flush(blk)
	require.Equal(false, moved)
	require.Equal(bCheckinValid, result)
	blk = block.NewBlockDeprecated(
		uint32(123),
		uint64(4),
		blk.HashBlock(),
		testutil.TimestampNow(),
		ta.Keyinfo["producer"].PubKey,
		nil,
	)
	moved, result = b.Flush(blk)
	require.Equal(false, moved)
	require.Equal(bCheckinValid, result)
	blk = block.NewBlockDeprecated(
		uint32(123),
		uint64(5),
		blk.HashBlock(),
		testutil.TimestampNow(),
		ta.Keyinfo["producer"].PubKey,
		nil,
	)
	moved, result = b.Flush(blk)
	require.Equal(false, moved)
	require.Equal(bCheckinValid, result)
	blk = block.NewBlockDeprecated(
		uint32(123),
		uint64(6),
		blk.HashBlock(),
		testutil.TimestampNow(),
		ta.Keyinfo["producer"].PubKey,
		nil,
	)
	moved, result = b.Flush(blk)
	require.Equal(false, moved)
	require.Equal(bCheckinValid, result)
	blk = block.NewBlockDeprecated(
		uint32(123),
		uint64(8),
		blk.HashBlock(),
		testutil.TimestampNow(),
		ta.Keyinfo["producer"].PubKey,
		nil,
	)
	moved, result = b.Flush(blk)
	require.Equal(false, moved)
	require.Equal(bCheckinValid, result)
	blk = block.NewBlockDeprecated(
		uint32(123),
		uint64(14),
		blk.HashBlock(),
		testutil.TimestampNow(),
		ta.Keyinfo["producer"].PubKey,
		nil,
	)
	moved, result = b.Flush(blk)
	require.Equal(false, moved)
	require.Equal(bCheckinValid, result)
	blk = block.NewBlockDeprecated(
		uint32(123),
		uint64(16),
		blk.HashBlock(),
		testutil.TimestampNow(),
		ta.Keyinfo["producer"].PubKey,
		nil,
	)
	moved, result = b.Flush(blk)
	require.Equal(false, moved)
	require.Equal(bCheckinValid, result)
	assert.Len(b.GetBlocksIntervalsToSync(32), 5)
	assert.Len(b.GetBlocksIntervalsToSync(7), 3)
	assert.Len(b.GetBlocksIntervalsToSync(5), 2)
	assert.Len(b.GetBlocksIntervalsToSync(1), 1)

	blk, err = chain.MintNewBlock(
		nil,
		testutil.TimestampNow(),
	)
	require.Nil(err)
	b.Flush(blk)
	assert.Len(b.GetBlocksIntervalsToSync(0), 0)
}
