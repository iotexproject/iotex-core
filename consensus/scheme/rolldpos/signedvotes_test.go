// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// A node restarting within a round does not sign a second block in it.
func TestSignedVotesSurviveRestart(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	round0, round1 := time.Unix(rlRound0TS, 0), time.Unix(rlRound1TS, 0)
	producer := identityset.PrivateKey(25)
	dbPath := filepath.Join(t.TempDir(), "consensus.db")

	n := newRLNodeWithDB(t, producer, delegates, keys, 1, dbPath)
	prevHash := n.chain.TipHash()
	hashA, propA := mintProposal(t, n.cm, prevHash, keys[3], round0)
	// the round-0 proposer (delegate 3) equivocates with a second block
	_, propC := mintProposal(t, n.cm, prevHash, keys[3], round0.Add(time.Second))
	hashB, propB := mintProposal(t, n.cm, prevHash, keys[0], round1)

	n.setClock(round0)
	n.prepare(51, 0)
	res, err := n.ctx.NewProposalEndorsement(n.deliverProposal(propA))
	require.NoError(err)
	require.Equal(hashA[:], voteOf(t, res).BlockHash())
	require.False(n.ctx.round.IsLocked(), "no polka seen: the round lock alone would not stop C")
	require.NoError(n.ctx.Stop(context.Background()))

	// restart within round 0
	r := newRLNodeWithDB(t, producer, delegates, keys, 1, dbPath)
	r.setClock(round0)
	r.prepare(51, 0)
	before := r.ctx.round.eManager.SizeWithBlock()
	for i := 0; i < 3; i++ {
		res, err = r.ctx.NewProposalEndorsement(r.deliverProposal(propC))
		require.Error(err, "must not sign PROPOSAL for a second block in the round")
		require.Contains(err.Error(), "already signed")
		require.Nil(res)
	}
	require.Equal(before, r.ctx.round.eManager.SizeWithBlock(), "a refused second proposal is neither validated nor stored")

	// re-signing the block already signed is fine
	res, err = r.ctx.NewProposalEndorsement(r.deliverProposal(propA))
	require.NoError(err)
	require.Equal(hashA[:], voteOf(t, res).BlockHash())

	// a later round is a new slot
	r.setClock(round1)
	r.prepare(51, 1)
	res, err = r.ctx.NewProposalEndorsement(r.deliverProposal(propB))
	require.NoError(err)
	require.Equal(hashB[:], voteOf(t, res).BlockHash())
}

func TestSignedVotesSlots(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	round0, round1 := time.Unix(rlRound0TS, 0), time.Unix(rlRound1TS, 0)
	n := newRLNode(t, identityset.PrivateKey(25), delegates, keys, 1)
	n.setClock(round0)
	n.prepare(51, 0)
	a, b := []byte("block-a"), []byte("block-b")

	require.NoError(n.ctx.round.recordSigning(LOCK, a))
	require.Error(n.ctx.round.recordSigning(LOCK, b), "a different block for the same (height, round, topic)")
	require.NoError(n.ctx.round.recordSigning(LOCK, a), "the same block again")
	require.NoError(n.ctx.round.recordSigning(COMMIT, b), "another topic is another slot")
	require.NoError(n.ctx.round.recordSigning(PROPOSAL, nil), "an empty-block PROPOSAL is a vote too")
	require.Error(n.ctx.round.recordSigning(PROPOSAL, a))

	n.setClock(round1)
	n.prepare(51, 1)
	require.NoError(n.ctx.round.recordSigning(LOCK, b), "a later round is another slot")
}

// With enforcement disabled nothing is recorded (legacy behaviour).
func TestSignedVotesDisabled(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	round0 := time.Unix(rlRound0TS, 0)
	dbPath := filepath.Join(t.TempDir(), "consensus.db")
	n := newRLNodeWithDB(t, identityset.PrivateKey(25), delegates, keys, 1, dbPath, WithRoundLockEnforcement(false))
	prevHash := n.chain.TipHash()
	_, propA := mintProposal(t, n.cm, prevHash, keys[3], round0)
	n.setClock(round0)
	n.prepare(51, 0)
	_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(propA))
	require.NoError(err)
	require.Nil(n.ctx.round.eManager.SignedVote(PROPOSAL))
	_, err = n.ctx.eManagerDB.Get(_eManagerNS, _signedVotesKey)
	require.Equal(db.ErrNotExist, errors.Cause(err))
}

func TestSignedVotesSerialization(t *testing.T) {
	require := require.New(t)
	votes := map[ConsensusVoteTopic]*signedVote{
		PROPOSAL: {height: 51, round: 2, blkHash: nil},
		LOCK:     {height: 51, round: 2, blkHash: []byte("0123456789abcdef0123456789abcdef")},
		COMMIT:   {height: 50, round: 0, blkHash: []byte("x")},
	}
	buf, err := serializeSignedVotes(votes)
	require.NoError(err)
	got, err := deserializeSignedVotes(buf)
	require.NoError(err)
	require.Len(got, 3)
	for topic, v := range votes {
		require.Equal(v.height, got[topic].height)
		require.Equal(v.round, got[topic].round)
		require.Equal(len(v.blkHash), len(got[topic].blkHash))
		require.Equal(string(v.blkHash), string(got[topic].blkHash))
	}
	for _, bad := range [][]byte{nil, {0x02, 0x00}, buf[:5], append(append([]byte{}, buf...), 0x00)} {
		_, err := deserializeSignedVotes(bad)
		require.Error(err)
	}

	// an undecodable record does not block startup
	kv := db.NewMemKVStore()
	require.NoError(kv.Start(context.Background()))
	require.NoError(kv.Put(_eManagerNS, _signedVotesKey, []byte{0xff}))
	m, err := newEndorsementManager(kv, block.NewDeserializer(0))
	require.NoError(err)
	require.Nil(m.SignedVote(PROPOSAL))
}
