// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
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

	ec = ec.Cleanup(time.Now().Add(time.Second * 10 * -1))
	require.Equal(1, len(ec.endorsers))
	require.Equal(1, len(ec.endorsers[b.PublicKey().HexString()].endorsements))
	require.Equal(end, ec.endorsers[b.PublicKey().HexString()].Endorsement(PROPOSAL))
}

func TestEndorsementManager(t *testing.T) {
	require := require.New(t)
	em := newEndorsementManager(true)
	require.NotNil(em)
	//require.Equal(0, em.Size())
	//require.Equal(0, em.SizeWithBlock())

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
	err := em.Cleanup(time.Now().Add(time.Second * 10 * -1))
	require.Nil(err)
	require.NotNil(em)
	//require.Equal(1, len(em.collections))
	encoded := encodeToString(cv.BlockHash())
	require.Equal(1, len(em.collections[encoded].endorsers))

	collection := em.collections[encoded].endorsers[end.Endorser().HexString()]
	require.Equal(end, collection.endorsements[PROPOSAL])
}

func TestEndorsementManagerDB(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	now := time.Now()
	priKey1 := identityset.PrivateKey(0)
	priKey2 := identityset.PrivateKey(1)
	mockProposal := endorsement.NewEndorsement(
		now.Add(-2*time.Second),
		priKey1.PublicKey(),
		[]byte{},
	)
	mockLock := endorsement.NewEndorsement(
		now.Add(-2*time.Second),
		priKey1.PublicKey(),
		[]byte{},
	)
	mockCommit := endorsement.NewEndorsement(
		now.Add(-2*time.Second),
		priKey2.PublicKey(),
		[]byte{},
	)
	eec := newEndorserEndorsementCollection()
	require.NoError(eec.AddEndorsement(PROPOSAL, mockProposal))
	require.NoError(eec.AddEndorsement(LOCK, mockLock))

	tsf1, err := testutil.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(28), 1, big.NewInt(int64(0)), nil, genesis.Default.ActionGasLimit, big.NewInt(0))
	require.NoError(err)

	// tsf2, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(29), 2, big.NewInt(int64(0)), nil, genesis.Default.ActionGasLimit, big.NewInt(0))
	// require.NoError(err)

	// tsf3, err := testutil.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(30), 3, big.NewInt(int64(0)), nil, genesis.Default.ActionGasLimit, big.NewInt(0))
	// require.NoError(err)

	tsf4, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(28), 2, big.NewInt(int64(0)), nil, genesis.Default.ActionGasLimit, big.NewInt(0))
	require.NoError(err)

	// tsf5, err := testutil.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(29), 3, big.NewInt(int64(0)), nil, genesis.Default.ActionGasLimit, big.NewInt(0))
	// require.NoError(err)

	// tsf6, err := testutil.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(30), 4, big.NewInt(int64(0)), nil, genesis.Default.ActionGasLimit, big.NewInt(0))
	// require.NoError(err)

	execution1, err := testutil.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(28), 1, big.NewInt(1), 0, big.NewInt(0), nil)
	require.NoError(err)
	// execution2, err := testutil.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(29), 2, big.NewInt(0), 0, big.NewInt(0), nil)
	// require.NoError(err)
	// execution3, err := testutil.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(30), 3, big.NewInt(2), 0, big.NewInt(0), nil)
	// require.NoError(err)

	// create testing create deposit actions
	deposit1 := action.NewCreateDeposit(
		4,
		2,
		big.NewInt(1),
		identityset.Address(31).String(),
		testutil.TestGasLimit,
		big.NewInt(0),
	)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(4).
		SetGasLimit(testutil.TestGasLimit).
		SetAction(deposit1).Build()
	sdeposit1, err := action.Sign(elp, identityset.PrivateKey(28))
	require.NoError(err)

	deposit2 := action.NewCreateDeposit(
		5,
		2,
		big.NewInt(2),
		identityset.Address(31).String(),
		testutil.TestGasLimit,
		big.NewInt(0),
	)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(5).
		SetGasLimit(testutil.TestGasLimit).
		SetAction(deposit2).Build()
	// sdeposit2, err := action.Sign(elp, identityset.PrivateKey(29))
	// require.NoError(err)

	deposit3 := action.NewCreateDeposit(
		6,
		2,
		big.NewInt(3),
		identityset.Address(31).String(),
		testutil.TestGasLimit,
		big.NewInt(0),
	)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(6).
		SetGasLimit(testutil.TestGasLimit).
		SetAction(deposit3).Build()
	// sdeposit3, err := action.Sign(elp, identityset.PrivateKey(30))
	// require.NoError(err)

	hash1 := hash.Hash256{}
	fnv.New32().Sum(hash1[:])
	blk1, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(hash1).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1, tsf4, execution1, sdeposit1).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)
	em := newEndorsementManager(true)
	blkHash := blk1.HashBlock()
	encodedHashBlock := encodeToString(blkHash[:])
	bytes, _ := hex.DecodeString(encodedHashBlock)
	vote1 := NewConsensusVote(bytes, PROPOSAL)
	vote2 := NewConsensusVote(bytes, COMMIT)
	em.AddVoteEndorsement(vote1, mockProposal)
	em.AddVoteEndorsement(vote2, mockCommit)
	em.RegisterBlock(&blk1)

}
