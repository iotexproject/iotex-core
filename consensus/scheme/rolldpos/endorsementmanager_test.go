// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestEndorserEndorsementCollection(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	now := time.Now()
	priKey := identityset.PrivateKey(0)
	mockProposal := endorsement.NewEndorsement(
		now.Add(-2*time.Second),
		priKey.PublicKey(),
		[]byte{},
	)
	mockLock := endorsement.NewEndorsement(
		now.Add(-2*time.Second),
		priKey.PublicKey(),
		[]byte{},
	)
	mockCommit := endorsement.NewEndorsement(
		now.Add(-2*time.Second),
		priKey.PublicKey(),
		[]byte{},
	)
	eec := newEndorserEndorsementCollection()
	require.NoError(eec.AddEndorsement(PROPOSAL, mockProposal))
	require.NoError(eec.AddEndorsement(LOCK, mockLock))
	require.NoError(eec.AddEndorsement(COMMIT, mockCommit))
	t.Run("read", func(t *testing.T) {
		require.Equal(mockProposal, eec.Endorsement(PROPOSAL))
		require.Equal(mockLock, eec.Endorsement(LOCK))
		require.Equal(mockCommit, eec.Endorsement(COMMIT))
	})
	t.Run("cleanup", func(t *testing.T) {
		cleaned := eec.Cleanup(now)
		require.Nil(cleaned.Endorsement(PROPOSAL))
		require.Nil(cleaned.Endorsement(LOCK))
		require.NotNil(cleaned.Endorsement(COMMIT))
	})
	t.Run("failure-to-replace", func(t *testing.T) {
		mockProposal2 := endorsement.NewEndorsement(
			now.Add(-3*time.Second),
			priKey.PublicKey(),
			[]byte{},
		)
		require.Error(eec.AddEndorsement(PROPOSAL, mockProposal2))
	})
	t.Run("success-to-replace", func(t *testing.T) {
		mockProposal2 := endorsement.NewEndorsement(
			now.Add(-2*time.Second),
			priKey.PublicKey(),
			[]byte{},
		)
		require.NoError(eec.AddEndorsement(PROPOSAL, mockProposal2))
	})
}

func TestBlockEndorsementCollection(t *testing.T) {
	require := require.New(t)
	b := getBlock(t)
	ec := newBlockEndorsementCollection(&b)
	require.NotNil(ec)
	require.NoError(ec.SetBlock(&b))
	require.Equal(&b, ec.Block())
	end := endorsement.NewEndorsement(time.Now(), b.PublicKey(), []byte("123"))

	require.NoError(ec.AddEndorsement(PROPOSAL, end))
	require.Equal(end, ec.Endorsement(b.PublicKey().HexString(), PROPOSAL))
	ends := ec.Endorsements([]ConsensusVoteTopic{PROPOSAL})
	require.Equal(1, len(ends))
	require.Equal(end, ends[0])

	cleaned := ec.Cleanup(time.Now().Add(time.Second * 10 * -1))
	require.Equal(1, len(cleaned.endorsers))
	require.Equal(1, len(cleaned.endorsers[b.PublicKey().HexString()].endorsements))
	require.Equal(end, cleaned.endorsers[b.PublicKey().HexString()].Endorsement(PROPOSAL))
}

func TestEndorsementManager(t *testing.T) {
	require := require.New(t)
	em := newEndorsementManager()
	require.NotNil(em)
	require.Equal(0, em.Size())
	require.Equal(0, em.SizeWithBlock())

	b := getBlock(t)
	require.NoError(em.RegisterBlock(&b))

	require.Panics(func() {
		em.AddVoteEndorsement(nil, nil)
	}, "vote is nil")
	blkHash := b.HashBlock()
	cv := NewConsensusVote(blkHash[:], PROPOSAL)
	require.NotNil(cv)

	require.Panics(func() {
		em.AddVoteEndorsement(cv, nil)
	}, "endorsement is nil")

	end := endorsement.NewEndorsement(time.Now(), b.PublicKey(), []byte("123"))
	require.NoError(em.AddVoteEndorsement(cv, end))

	require.Panics(func() {
		em.Log(nil, nil)
	}, "logger is nil")
	l := em.Log(log.L(), nil)
	require.NotNil(l)
	l.Info("test output")
	cleaned := em.Cleanup(time.Now().Add(time.Second * 10 * -1))
	require.NotNil(cleaned)
	require.Equal(1, len(cleaned.collections))
	encoded := encodeToString(cv.BlockHash())
	require.Equal(1, len(cleaned.collections[encoded].endorsers))

	collection := cleaned.collections[encoded].endorsers[end.Endorser().HexString()]
	require.Equal(end, collection.endorsements[PROPOSAL])
}
