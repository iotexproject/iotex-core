// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/endorsement"
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
		delegates: []*Delegate{
			{Address: "delegate1"}, {Address: "delegate2"}, {Address: "delegate3"}, {Address: "delegate4"},
			{Address: "delegate5"}, {Address: "delegate6"}, {Address: "delegate7"}, {Address: "delegate8"},
			{Address: "delegate9"}, {Address: "delegate0"}, {Address: "delegateA"}, {Address: "delegateB"},
			{Address: "delegateC"}, {Address: "delegateD"}, {Address: "delegateE"}, {Address: "delegateF"},
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
		require.True(round.IsDelegate(d.Address))
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

func TestRoundCtx_IsProducer(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	mkBLS := func(seed string) *crypto.BLS12381PublicKey {
		h := sha256.Sum256([]byte(seed))
		sk, err := crypto.GenerateBLS12381PrivateKey(h[:])
		require.NoError(err)
		return sk.PublicKey()
	}

	pk1 := mkBLS("d1")
	pk2 := mkBLS("d2")
	otherPK := mkBLS("not-in-set")

	round := &roundCtx{
		delegates: []*Delegate{
			{Address: "d1", BLSPubKey: pk1},
			{Address: "d2", BLSPubKey: pk2},
			{Address: "d3-no-bls", BLSPubKey: nil}, // pre-fork or unregistered
		},
	}

	require.True(round.IsProducer(pk1.Bytes()),
		"a registered delegate's BLS pubkey matches")
	require.True(round.IsProducer(pk2.Bytes()),
		"another registered delegate's BLS pubkey matches")
	require.False(round.IsProducer(otherPK.Bytes()),
		"an unregistered BLS pubkey does not match")
	require.False(round.IsProducer(nil),
		"a nil pubkey never matches — header must carry one")
	require.False(round.IsProducer([]byte{}),
		"an empty pubkey never matches")

	// Truncating one byte off a valid pubkey must not match. Guards
	// against silent prefix/suffix-matching bugs in a hypothetical
	// future implementation.
	truncated := pk1.Bytes()[:crypto.BLSPubkeyLength-1]
	require.False(round.IsProducer(truncated),
		"a truncated pubkey does not match")
}
