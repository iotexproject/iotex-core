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
	"github.com/iotexproject/iotex-core/blockchain/block"
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
	ends := ec.Endorsements([]VoteTopic{PROPOSAL})
	require.Equal(1, len(ends))
	require.Equal(end, ends[0])

	ec = ec.Cleanup(time.Now().Add(time.Second * 10 * -1))
	require.Equal(1, len(ec.endorsers))
	require.Equal(1, len(ec.endorsers[b.PublicKey().HexString()].endorsements))
	require.Equal(end, ec.endorsers[b.PublicKey().HexString()].Endorsement(PROPOSAL))
}

func TestEndorsementManager(t *testing.T) {
	require := require.New(t)

	em, err := newEndorsementManager(nil, block.NewDeserializer(0))
	require.NoError(err)
	require.NotNil(em)
	require.Equal(0, em.Size())
	require.Equal(0, em.SizeWithBlock())

	b := getBlock(t)

	require.NoError(em.RegisterBlock(&b))

	require.Panics(func() {
		em.AddVoteEndorsement(nil, nil)
	}, "vote is nil")
	blkHash := b.HashBlock()
	cv := NewVote(blkHash[:], PROPOSAL)
	require.NotNil(cv)

	require.Panics(func() {
		em.AddVoteEndorsement(cv, nil)
	}, "endorsement is nil")

	timestamp := time.Now()
	end := endorsement.NewEndorsement(timestamp, b.PublicKey(), []byte("123"))
	require.NoError(em.AddVoteEndorsement(cv, end))

	require.Panics(func() {
		em.Log(nil, nil)
	}, "logger is nil")
	l := em.Log(log.L(), nil)
	require.NotNil(l)
	l.Info("test output")

	cv2 := NewVote(blkHash[:], LOCK)
	require.NotNil(cv2)
	end2 := endorsement.NewEndorsement(timestamp.Add(time.Second*10), b.PublicKey(), []byte("456"))
	require.NoError(em.AddVoteEndorsement(cv2, end2))
	l.Info("test output2")

	encoded := encodeToString(cv.BlockHash())
	require.Equal(1, len(em.collections[encoded].endorsers))
	collection := em.collections[encoded].endorsers[end.Endorser().HexString()]
	require.Equal(2, len(collection.endorsements))
	require.Equal(end, collection.endorsements[PROPOSAL])
	require.Equal(end2, collection.endorsements[LOCK])

	//cleanup
	require.NoError(em.Cleanup(timestamp.Add(time.Second * 2)))
	require.NotNil(em)
	require.Equal(1, len(em.collections))
	require.Equal(1, len(em.collections[encoded].endorsers))

	collection = em.collections[encoded].endorsers[end.Endorser().HexString()] //ee
	require.Equal(1, len(collection.endorsements))
	require.Equal(end2, collection.endorsements[LOCK])

	//when the time is zero, it should generate empty eManager
	zerotime := time.Time{}
	require.Equal(zerotime.IsZero(), true)
	require.NoError(em.Cleanup(zerotime))
	require.Equal(0, len(em.collections))

	// test for cachedMintedBlock
	require.NoError(em.SetMintedBlock(&b))
	require.Equal(&b, em.CachedMintedBlock())

	blkTimestamp := b.Timestamp()
	require.NoError(em.Cleanup(blkTimestamp)) // if roundStartTime is same or ealier than blockTimestamp
	require.NotNil(em)
	require.NotNil(em.CachedMintedBlock()) // not clean up
	require.Equal(&b, em.CachedMintedBlock())

	require.NoError(em.Cleanup(blkTimestamp.Add(time.Second * 2))) // if roundStartTime is after than blockTimestamp (old block)
	require.Nil(em.CachedMintedBlock())                            // clean up

	require.NoError(em.SetMintedBlock(&b))
	require.Equal(&b, em.CachedMintedBlock())

	require.NoError(em.Cleanup(zerotime)) // if it is clean up with zero time
	require.Nil(em.CachedMintedBlock())   // clean up
}

func TestEndorsementManagerProto(t *testing.T) {
	require := require.New(t)
	em, err := newEndorsementManager(nil, block.NewDeserializer(0))
	require.NoError(err)
	require.NotNil(em)

	b := getBlock(t)

	require.NoError(em.RegisterBlock(&b))
	blkHash := b.HashBlock()
	cv := NewVote(blkHash[:], PROPOSAL)
	require.NotNil(cv)
	end := endorsement.NewEndorsement(time.Now(), b.PublicKey(), []byte("123"))
	require.NoError(em.AddVoteEndorsement(cv, end))
	require.Nil(em.cachedMintedBlk)
	require.NoError(em.SetMintedBlock(&b))

	//test converting endorsement pb
	endProto, err := end.Proto()
	require.NoError(err)
	end2 := &endorsement.Endorsement{}
	require.NoError(end2.LoadProto(endProto))
	require.Equal(end, end2)

	//test converting emanager pb
	emProto, err := em.toProto()
	require.NoError(err)
	em2, err := newEndorsementManager(nil, block.NewDeserializer(0))
	require.NoError(err)
	require.NoError(em2.fromProto(emProto, block.NewDeserializer(0)))

	require.Equal(len(em.collections), len(em2.collections))
	encoded := encodeToString(cv.BlockHash())
	require.Equal(em.collections[encoded].endorsers, em2.collections[encoded].endorsers)
	require.Equal(em.cachedMintedBlk.HashBlock(), em2.cachedMintedBlk.HashBlock())
}
