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
	"github.com/stretchr/testify/assert"
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

	clock := clock.NewMock()
	var prevHash hash.Hash32B
	blk := block.NewBlockDeprecated(
		1,
		8,
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
			blockchain.EXPECT().TipHeight().Return(uint64(8)).Times(3)
			blockchain.EXPECT().GetBlockByHeight(uint64(8)).Return(blk, nil).Times(1)
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

	epoch, err := ctx.epochCtxByHeight(uint64(8 + 1))
	assert.NoError(t, err)
	assert.Equal(t, uint64(3), epoch.num)
	assert.Equal(t, uint64(0), epoch.subEpochNum)
	assert.Equal(t, uint64(9), epoch.height)

	crypto.SortCandidates(candidates, epoch.num, crypto.CryptoSeed)

	assert.Equal(t, candidates, epoch.delegates)
	ctx.epoch = epoch

	subEpoch, err := ctx.calcSubEpochNum()
	require.NoError(t, err)
	assert.Equal(t, uint64(0), subEpoch)

	proposer, height, round, err := ctx.rotatedProposer(time.Duration(0))
	require.NoError(t, err)
	assert.Equal(t, candidates[1], proposer)
	assert.Equal(t, uint64(9), height)
	assert.Equal(t, uint32(0), round)

	clock.Add(time.Second)
	duration, err := ctx.calcDurationSinceLastBlock()
	require.NoError(t, err)
	assert.Equal(t, time.Second, duration)
}
