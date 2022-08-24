// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensus

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/endorsement"
)

func TestRoundCtx(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	now := time.Now()
	blockHeight := uint64(8)
	round := &roundCtx{
		height:               blockHeight + 1,
		roundNum:             3,
		proposer:             "delegateC",
		roundStartTime:       now.Add(-2 * time.Second),
		nextRoundStartTime:   now.Add(2 * time.Second),
		epochNum:             uint64(1),
		epochStartHeight:     uint64(1),
		nextEpochStartHeight: uint64(33),
		delegates: []string{
			"delegate1", "delegate2", "delegate3", "delegate4",
			"delegate5", "delegate6", "delegate7", "delegate8",
			"delegate9", "delegate0", "delegateA", "delegateB",
			"delegateC", "delegateD", "delegateE", "delegateF",
		},
	}

	require.Equal(uint64(1), round.EpochNum())
	require.Equal(16, len(round.Delegates()))
	require.Equal(uint64(1), round.EpochStartHeight())
	require.Equal(uint64(33), round.NextEpochStartHeight())
	require.Equal(now.Add(-2*time.Second), round.StartTime())
	require.Equal(now.Add(2*time.Second), round.NextRoundStartTime())
	require.Equal(blockHeight+1, round.Height())
	require.Equal(uint32(3), round.Number())
	require.Equal("delegateC", round.Proposer())
	require.False(round.IsDelegate("non-delegate"))
	for _, d := range round.Delegates() {
		require.True(round.IsDelegate(d))
	}
	t.Run("is-stale", func(t *testing.T) {
		blkHash := []byte("Some block hash")
		require.True(round.IsStale(blockHeight, 2, nil))
		require.True(round.IsStale(blockHeight, 3, nil))
		require.True(round.IsStale(blockHeight, 4, nil))
		require.True(round.IsStale(blockHeight+1, 2, nil))
		require.True(round.IsStale(blockHeight+1, 2, NewEndorsedConsensusMessage(
			blockHeight+1,
			&blockProposal{},
			&endorsement.Endorsement{},
		)))
		require.True(round.IsStale(blockHeight+1, 2, NewEndorsedConsensusMessage(
			blockHeight+1,
			NewConsensusVote(
				blkHash,
				PROPOSAL,
			),
			&endorsement.Endorsement{},
		)))
		require.False(round.IsStale(blockHeight+1, 2, NewEndorsedConsensusMessage(
			blockHeight+1,
			NewConsensusVote(
				blkHash,
				COMMIT,
			),
			&endorsement.Endorsement{},
		)))
		require.False(round.IsStale(blockHeight+1, 3, nil))
		require.False(round.IsStale(blockHeight+1, 4, nil))
		require.False(round.IsStale(blockHeight+2, 2, nil))
	})
	t.Run("is-future", func(t *testing.T) {
		require.False(round.IsFuture(blockHeight, 4))
		require.False(round.IsFuture(blockHeight+1, 2))
		require.False(round.IsFuture(blockHeight+1, 3))
		require.True(round.IsFuture(blockHeight+1, 4))
		require.True(round.IsFuture(blockHeight+2, 2))
	})
	// TODO: add more unit tests
}
