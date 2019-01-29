// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"testing"
	"time"

	"github.com/facebookgo/clock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestRollDPoSCtx(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	candidates := make([]string, 4)
	for i := 0; i < len(candidates); i++ {
		candidates[i] = testAddrs[i].encodedAddr
	}

	blockHeight := uint64(8)
	clock := clock.NewMock()
	var prevHash hash.Hash32B
	blk := block.NewBlockDeprecated(
		1,
		blockHeight,
		prevHash,
		testutil.TimestampNowFromClock(clock),
		testAddrs[0].pubKey,
		make([]action.SealedEnvelope, 0),
	)
	ctx := makeTestRollDPoSCtx(
		testAddrs[0],
		ctrl,
		config.RollDPoS{
			NumSubEpochs: 1,
			NumDelegates: 4,
		},
		func(blockchain *mock_blockchain.MockBlockchain) {
			blockchain.EXPECT().GetBlockByHeight(blockHeight).Return(blk, nil).Times(4)
			blockchain.EXPECT().CandidatesByHeight(gomock.Any()).Return([]*state.Candidate{
				{Address: candidates[0]},
				{Address: candidates[1]},
				{Address: candidates[2]},
				{Address: candidates[3]},
			}, nil).Times(1)
		},
		func(_ *mock_actpool.MockActPool) {},
		nil,
		clock,
	)
	ctx.round = &roundCtx{height: blockHeight + 1}
	ctx.cfg.DelegateInterval = 10 * time.Second
	ctx.cfg.FSM.AcceptBlockTTL = 4 * time.Second
	ctx.cfg.FSM.AcceptProposalEndorsementTTL = 2 * time.Second
	ctx.cfg.FSM.AcceptLockEndorsementTTL = 2 * time.Second
	ctx.cfg.ToleratedOvertime = 2 * time.Second

	epoch, err := ctx.epochCtxByHeight(blockHeight + 1)
	require.NoError(t, err)
	require.Equal(t, uint64(3), epoch.num)
	require.Equal(t, uint64(0), epoch.subEpochNum)
	require.Equal(t, uint64(9), epoch.height)

	crypto.SortCandidates(candidates, epoch.num, crypto.CryptoSeed)

	require.Equal(t, candidates, epoch.delegates)
	ctx.epoch = epoch

	require.NoError(t, ctx.updateSubEpochNum(blockHeight+1))
	require.Equal(t, uint64(0), ctx.epoch.subEpochNum)

	clock.Add(9 * time.Second)
	err = ctx.updateRound(blockHeight + 1)
	require.NoError(t, err)
	require.Equal(t, uint32(0), ctx.round.number)
	clock.Add(1 * time.Second)
	require.NoError(t, ctx.updateRound(blockHeight+1))
	require.Equal(t, uint32(0), ctx.round.number)
	clock.Add(1 * time.Second)
	require.NoError(t, ctx.updateRound(blockHeight+1))
	require.Equal(t, uint32(0), ctx.round.number)
	clock.Add(12 * time.Second)
	require.NoError(t, ctx.updateRound(blockHeight+1))
	require.Equal(t, uint32(2), ctx.round.number)
	require.Equal(t, candidates[1], ctx.round.proposer)
	require.Equal(t, clock.Now().Add(7*time.Second), ctx.round.timestamp)
	require.Equal(t, uint64(9), ctx.round.height)
}
