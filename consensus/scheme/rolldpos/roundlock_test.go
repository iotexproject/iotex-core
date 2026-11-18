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

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/endorsement"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// round-2 start under the timing of roundlock_disabled_test.go; proposer = proposers[53%4] = 1
const roundLockRound2TS = rlGenesisTS + 100

func roundLockKeys() ([]crypto.PrivateKey, []string) {
	keys := make([]crypto.PrivateKey, rlNumDelegates)
	delegates := make([]string, rlNumDelegates)
	for i := 0; i < rlNumDelegates; i++ {
		keys[i] = identityset.PrivateKey(i)
		delegates[i] = identityset.Address(i).String()
	}
	return keys, delegates
}

// reProposal re-proposes the block of an earlier proposal, by proposerKey at
// ts, carrying pol as its proof of lock
func reProposal(
	t *testing.T,
	prop *iotextypes.ConsensusMessage,
	proposerKey crypto.PrivateKey,
	ts time.Time,
	pol []*endorsement.Endorsement,
) *iotextypes.ConsensusMessage {
	t.Helper()
	ecm := &EndorsedConsensusMessage{}
	require.NoError(t, ecm.LoadProto(prop, block.NewDeserializer(0)))
	bp := newBlockProposal(ecm.Document().(*blockProposal).block, pol)
	ens, err := endorsement.Endorse(bp, ts, proposerKey)
	require.NoError(t, err)
	cmsg, err := NewEndorsedConsensusMessage(51, bp, ens[0]).Proto()
	require.NoError(t, err)
	return cmsg
}

func proposalPOL(t *testing.T, keys []crypto.PrivateKey, idxs []int, blkHash []byte, ts time.Time) []*endorsement.Endorsement {
	pol := make([]*endorsement.Endorsement, 0, len(idxs))
	for _, idx := range idxs {
		pol = append(pol, rlEndorse(t, keys, idx, blkHash, PROPOSAL, ts).Endorsement())
	}
	return pol
}

func voteOf(t *testing.T, res interface{}) *ConsensusVote {
	t.Helper()
	msgs, ok := res.([]*EndorsedConsensusMessage)
	require.True(t, ok)
	require.Len(t, msgs, 1)
	return msgs[0].Document().(*ConsensusVote)
}

// A node locked on A refuses a conflicting proposal without a proof of lock,
// endorses A when the proposal phase times out, and moves its lock only on a
// polka of a later round.
func TestRoundLockRefusesConflictingProposal(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	round0, round1 := time.Unix(rlRound0TS, 0), time.Unix(rlRound1TS, 0)

	n := newRLNode(t, identityset.PrivateKey(25), delegates, keys, 1)
	prevHash := n.chain.TipHash()
	hashA, propA := mintProposal(t, n.cm, prevHash, keys[3], round0)
	hashB, propB := mintProposal(t, n.cm, prevHash, keys[0], round1)

	n.setClock(round0)
	n.prepare(51, 0)
	_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(propA))
	require.NoError(err)
	for _, idx := range []int{0, 2, 3} {
		n.feedVote(keys, idx, hashA[:], PROPOSAL, round0)
	}
	require.True(n.ctx.round.IsLocked())
	require.Equal(hashA[:], n.ctx.round.HashOfBlockInLock())
	require.Equal(uint32(0), n.ctx.round.LockRound())

	n.setClock(round1)
	n.prepare(51, 1)
	res, err := n.ctx.NewProposalEndorsement(n.deliverProposal(propB))
	require.Error(err, "a node locked on A must refuse B without a proof of lock")
	require.Nil(res)
	require.Equal(hashA[:], n.ctx.round.HashOfBlockInLock())

	// proposal phase times out: the node endorses the block in lock
	res, err = n.ctx.NewProposalEndorsement(nil)
	require.NoError(err)
	vote := voteOf(t, res)
	require.Equal(PROPOSAL, vote.Topic())
	require.Equal(hashA[:], vote.BlockHash())

	// a polka on B in round 1 (later than lock round 0) moves the lock to B
	var lockRes interface{}
	for _, idx := range []int{0, 2, 3} {
		lockRes, err = n.ctx.NewLockEndorsement(rlEndorse(t, keys, idx, hashB[:], PROPOSAL, round1))
		require.NoError(err)
	}
	require.NotNil(lockRes)
	vote = voteOf(t, lockRes)
	require.Equal(LOCK, vote.Topic())
	require.Equal(hashB[:], vote.BlockHash())
	require.Equal(hashB[:], n.ctx.round.HashOfBlockInLock())
	require.Equal(uint32(1), n.ctx.round.LockRound())
}

// The four-delegate scenario of TestRoundLockDisabledN4 with enforcement on:
// no node endorses B, every honest node commits A.
func TestRoundLockEnforcedN4(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	const (
		h1 = 3 // round-0 proposer
		h2 = 1
		h3 = 2
		z  = 0 // round-1 proposer, faulty
	)
	producer := identityset.PrivateKey(25)
	H1 := newRLNode(t, producer, delegates, keys, h1)
	H2 := newRLNode(t, producer, delegates, keys, h2)
	H3 := newRLNode(t, producer, delegates, keys, h3)
	prevHash := H1.chain.TipHash()
	round0, round1 := time.Unix(rlRound0TS, 0), time.Unix(rlRound1TS, 0)
	hashA, propA := mintProposal(t, H1.cm, prevHash, keys[h1], round0)
	_, propB := mintProposal(t, H1.cm, prevHash, keys[z], round1)

	for _, n := range []*rlNode{H1, H2, H3} {
		n.setClock(round0)
		n.prepare(51, 0)
		_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(propA))
		require.NoError(err)
	}
	for _, n := range []*rlNode{H1, H2} {
		for _, idx := range []int{h1, h2, z} {
			n.feedVote(keys, idx, hashA[:], PROPOSAL, round0)
		}
		require.True(n.ctx.round.IsLocked())
	}
	require.False(H3.ctx.round.IsLocked())

	// H1 and H3 reach COMMIT(A); H3 had not seen the polka, but signing COMMIT
	// now locks it on A
	var h1CommitA, h3CommitA *EndorsedConsensusMessage
	for _, idx := range []int{h1, h2, z} {
		if r, err := H1.ctx.NewPreCommitEndorsement(rlEndorse(t, keys, idx, hashA[:], LOCK, round0)); err == nil && r != nil {
			h1CommitA = r.([]*EndorsedConsensusMessage)[0]
		}
		if r, err := H3.ctx.NewPreCommitEndorsement(rlEndorse(t, keys, idx, hashA[:], LOCK, round0)); err == nil && r != nil {
			h3CommitA = r.([]*EndorsedConsensusMessage)[0]
		}
	}
	require.NotNil(h1CommitA)
	require.NotNil(h3CommitA)
	// as the FSM does, H3 feeds its own COMMIT(A) back into its round
	done, err := H3.ctx.Commit(h3CommitA)
	require.NoError(err)
	require.False(done)
	require.True(H3.ctx.round.IsLocked(), "signing COMMIT(A) must lock H3 on A")
	require.Equal(hashA[:], H3.ctx.round.HashOfBlockInLock())

	for _, idx := range []int{h1, h2} {
		r, err := H2.ctx.NewPreCommitEndorsement(rlEndorse(t, keys, idx, hashA[:], LOCK, round0))
		require.NoError(err)
		require.Nil(r)
	}

	zCommitA := rlEndorse(t, keys, z, hashA[:], COMMIT, round0)
	for _, m := range []*EndorsedConsensusMessage{h1CommitA, h3CommitA, zCommitA} {
		done, err = H1.ctx.Commit(m)
		require.NoError(err)
	}
	require.True(done)
	require.Equal(hashA, H1.chain.TipHash())

	// ---- round 1: Z proposes B ----
	for _, n := range []*rlNode{H2, H3} {
		n.setClock(round1)
		n.prepare(51, 1)
	}
	// Note: PreCommitEndorsement (ReadyToCommit) does not find a node's own
	// COMMIT today, so H3 also takes the proposal path; the round lock alone
	// must keep both H2 and H3 off B.
	for _, n := range []*rlNode{H2, H3} {
		res, err := n.ctx.NewProposalEndorsement(n.deliverProposal(propB))
		require.Error(err, "a node locked on A must refuse B")
		require.Nil(res)
	}

	// only Z endorses B: no polka, no lock move
	lockRes, err := H2.ctx.NewLockEndorsement(rlEndorse(t, keys, z, hashB(t, propB), PROPOSAL, round1))
	require.NoError(err)
	require.Nil(lockRes)
	require.Equal(hashA[:], H2.ctx.round.HashOfBlockInLock())

	// liveness: the COMMIT(A) votes reach H2 and H3, both commit A
	for _, n := range []*rlNode{H2, H3} {
		done = false
		for _, m := range []*EndorsedConsensusMessage{h1CommitA, h3CommitA, zCommitA} {
			if done {
				break
			}
			done, err = n.ctx.Commit(m)
			require.NoError(err)
		}
		require.True(done)
		require.Equal(hashA, n.chain.TipHash())
	}
}

func hashB(t *testing.T, prop *iotextypes.ConsensusMessage) []byte {
	ecm := &EndorsedConsensusMessage{}
	require.NoError(t, ecm.LoadProto(prop, block.NewDeserializer(0)))
	h := ecm.Document().(*blockProposal).block.HashBlock()
	return h[:]
}

// A proof of lock from a round later than the node's lock round moves the lock.
func TestRoundLockMovesOnLaterProofOfLock(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	round0, round1, round2 := time.Unix(rlRound0TS, 0), time.Unix(rlRound1TS, 0), time.Unix(roundLockRound2TS, 0)

	n := newRLNode(t, identityset.PrivateKey(25), delegates, keys, 2)
	prevHash := n.chain.TipHash()
	hashA, propA := mintProposal(t, n.cm, prevHash, keys[3], round0)
	hashB, propB := mintProposal(t, n.cm, prevHash, keys[0], round1)

	n.setClock(round0)
	n.prepare(51, 0)
	_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(propA))
	require.NoError(err)
	for _, idx := range []int{0, 1, 3} {
		n.feedVote(keys, idx, hashA[:], PROPOSAL, round0)
	}
	require.Equal(hashA[:], n.ctx.round.HashOfBlockInLock())

	n.setClock(round1)
	n.prepare(51, 1)
	n.setClock(round2)
	n.prepare(51, 2)

	// delegate 1 (round-2 proposer) re-proposes B with B's round-1 polka
	pol := proposalPOL(t, keys, []int{0, 1, 3}, hashB[:], round1)
	res, err := n.ctx.NewProposalEndorsement(n.deliverProposal(reProposal(t, propB, keys[1], round2, pol)))
	require.NoError(err)
	vote := voteOf(t, res)
	require.Equal(PROPOSAL, vote.Topic())
	require.Equal(hashB[:], vote.BlockHash())
	require.Equal(hashB[:], n.ctx.round.HashOfBlockInLock())
	require.Equal(uint32(1), n.ctx.round.LockRound())
}

// A proof of lock from a round not later than the node's lock round is refused.
func TestRoundLockRejectsStaleProofOfLock(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	round0, round1, round2 := time.Unix(rlRound0TS, 0), time.Unix(rlRound1TS, 0), time.Unix(roundLockRound2TS, 0)

	n := newRLNode(t, identityset.PrivateKey(25), delegates, keys, 2)
	prevHash := n.chain.TipHash()
	hashB, propB := mintProposal(t, n.cm, prevHash, keys[3], round0)
	hashA, propA := mintProposal(t, n.cm, prevHash, keys[0], round1)

	n.setClock(round0)
	n.prepare(51, 0)
	n.setClock(round1)
	n.prepare(51, 1)
	_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(propA))
	require.NoError(err)
	for _, idx := range []int{0, 1, 3} {
		n.feedVote(keys, idx, hashA[:], PROPOSAL, round1)
	}
	require.Equal(hashA[:], n.ctx.round.HashOfBlockInLock())
	require.Equal(uint32(1), n.ctx.round.LockRound())

	n.setClock(round2)
	n.prepare(51, 2)
	pol := proposalPOL(t, keys, []int{0, 1, 3}, hashB[:], round0)
	res, err := n.ctx.NewProposalEndorsement(n.deliverProposal(reProposal(t, propB, keys[1], round2, pol)))
	require.Error(err, "a round-0 proof of lock must not move a round-1 lock")
	require.Nil(res)
	require.Equal(hashA[:], n.ctx.round.HashOfBlockInLock())
	require.Equal(uint32(1), n.ctx.round.LockRound())
}

// The lock survives a restart, and is not restored when enforcement is off.
func TestRoundLockSurvivesRestart(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	round0, round1 := time.Unix(rlRound0TS, 0), time.Unix(rlRound1TS, 0)
	producer := identityset.PrivateKey(25)
	dbPath := filepath.Join(t.TempDir(), "consensus.db")

	n := newRLNodeWithDB(t, producer, delegates, keys, 1, dbPath)
	prevHash := n.chain.TipHash()
	hashA, propA := mintProposal(t, n.cm, prevHash, keys[3], round0)
	_, propB := mintProposal(t, n.cm, prevHash, keys[0], round1)

	n.setClock(round0)
	n.prepare(51, 0)
	_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(propA))
	require.NoError(err)
	for _, idx := range []int{0, 2, 3} {
		n.feedVote(keys, idx, hashA[:], PROPOSAL, round0)
	}
	require.True(n.ctx.round.IsLocked())
	require.NoError(n.ctx.Stop(context.Background()))

	// restart in round 1: the lock and the locked block are back
	r := newRLNodeWithDB(t, producer, delegates, keys, 1, dbPath)
	r.setClock(round1)
	r.prepare(51, 1)
	require.True(r.ctx.round.IsLocked(), "lock must survive the restart")
	require.Equal(hashA[:], r.ctx.round.HashOfBlockInLock())
	require.Equal(uint32(0), r.ctx.round.LockRound())
	require.NotNil(r.ctx.round.Block(hashA[:]))
	require.NotEmpty(r.ctx.round.ProofOfLock())
	_, err = r.ctx.NewProposalEndorsement(r.deliverProposal(propB))
	require.Error(err, "the restarted node must still refuse B")
	require.NoError(r.ctx.Stop(context.Background()))

	// with enforcement disabled the persisted lock is ignored (legacy behaviour)
	l := newRLNodeWithDB(t, producer, delegates, keys, 1, dbPath, WithRoundLockEnforcement(false))
	l.setClock(round1)
	l.prepare(51, 1)
	require.False(l.ctx.round.IsLocked())
}

func TestLockStateSerialization(t *testing.T) {
	require := require.New(t)
	keys, _ := roundLockKeys()
	blkHash := []byte("0123456789abcdef0123456789abcdef")
	pol := proposalPOL(t, keys, []int{0, 1, 2}, blkHash, time.Unix(rlRound1TS, 0))
	ls := &lockState{height: 51, round: 3, status: _locked, blkHash: blkHash, proofOfLock: pol}

	buf, err := ls.serialize()
	require.NoError(err)
	got, err := deserializeLockState(buf)
	require.NoError(err)
	require.Equal(ls.height, got.height)
	require.Equal(ls.round, got.round)
	require.Equal(ls.status, got.status)
	require.Equal(ls.blkHash, got.blkHash)
	require.Len(got.proofOfLock, len(pol))
	for i := range pol {
		require.Equal(pol[i].Signature(), got.proofOfLock[i].Signature())
		require.True(pol[i].Timestamp().Equal(got.proofOfLock[i].Timestamp()))
	}

	for _, bad := range [][]byte{nil, {0x02}, buf[:10], append([]byte{0x02}, buf[1:]...)} {
		_, err := deserializeLockState(bad)
		require.Error(err)
	}
}

func TestEndorsementManagerLockPersistence(t *testing.T) {
	require := require.New(t)
	kv := db.NewMemKVStore()
	require.NoError(kv.Start(context.Background()))
	m, err := newEndorsementManager(kv, block.NewDeserializer(0))
	require.NoError(err)
	require.Nil(m.Lock())

	ls := &lockState{height: 7, round: 1, status: _locked, blkHash: []byte("hash")}
	require.NoError(m.SetLock(ls))

	// a manager loaded from the same store sees the lock
	m2, err := newEndorsementManager(kv, block.NewDeserializer(0))
	require.NoError(err)
	require.NotNil(m2.Lock())
	require.Equal(uint64(7), m2.Lock().height)

	// a new height (zero timestamp cleanup) clears it, in memory and in the store
	require.NoError(m2.Cleanup(time.Time{}))
	require.Nil(m2.Lock())
	_, err = kv.Get(_eManagerNS, _lockStateKey)
	require.Equal(db.ErrNotExist, errors.Cause(err))

	// an undecodable lock does not block startup
	require.NoError(kv.Put(_eManagerNS, _lockStateKey, []byte{0xff}))
	m3, err := newEndorsementManager(kv, block.NewDeserializer(0))
	require.NoError(err)
	require.Nil(m3.Lock())
}

// lockedOnA returns node idx locked on A (proposed by delegate 3) by a round-0
// polka from voters, plus B minted by delegate 0 for round 1
func lockedOnA(t *testing.T, idx int, voters []int) (*rlNode, []crypto.PrivateKey, []byte, []byte, *iotextypes.ConsensusMessage) {
	keys, delegates := roundLockKeys()
	round0, round1 := time.Unix(rlRound0TS, 0), time.Unix(rlRound1TS, 0)
	n := newRLNode(t, identityset.PrivateKey(25), delegates, keys, idx)
	prevHash := n.chain.TipHash()
	hashA, propA := mintProposal(t, n.cm, prevHash, keys[3], round0)
	hashB, propB := mintProposal(t, n.cm, prevHash, keys[0], round1)
	n.setClock(round0)
	n.prepare(51, 0)
	_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(propA))
	require.NoError(t, err)
	for _, v := range voters {
		n.feedVote(keys, v, hashA[:], PROPOSAL, round0)
	}
	require.True(t, n.ctx.round.IsLocked())
	require.Equal(t, hashA[:], n.ctx.round.HashOfBlockInLock())
	return n, keys, hashA[:], hashB[:], propB
}

func TestRoundLockRejectsMultiRoundProofOfLock(t *testing.T) {
	require := require.New(t)
	n, keys, hashA, hashB, propB := lockedOnA(t, 2, []int{0, 1, 3})
	round0, round1, round2 := time.Unix(rlRound0TS, 0), time.Unix(rlRound1TS, 0), time.Unix(roundLockRound2TS, 0)
	n.setClock(round1)
	n.prepare(51, 1)
	n.setClock(round2)
	n.prepare(51, 2)
	pol := append(proposalPOL(t, keys, []int{0, 1}, hashB, round1), proposalPOL(t, keys, []int{3}, hashB, round0)...)
	_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(reProposal(t, propB, keys[1], round2, pol)))
	require.Error(err)
	require.Equal(hashA, n.ctx.round.HashOfBlockInLock())
}

func TestRoundLockAcceptsDecidedProofOfLock(t *testing.T) {
	require := require.New(t)
	n, keys, _, hashB, propB := lockedOnA(t, 2, []int{0, 1, 3})
	round1, round2 := time.Unix(rlRound1TS, 0), time.Unix(roundLockRound2TS, 0)
	n.setClock(round1)
	n.prepare(51, 1)
	n.setClock(round2)
	n.prepare(51, 2)
	var pol []*endorsement.Endorsement
	for _, idx := range []int{0, 1, 3} {
		pol = append(pol, rlEndorse(t, keys, idx, hashB, COMMIT, round1).Endorsement())
	}
	res, err := n.ctx.NewProposalEndorsement(n.deliverProposal(reProposal(t, propB, keys[1], round2, pol)))
	require.NoError(err, "2/3 COMMIT on B proves B final")
	require.Equal(hashB, voteOf(t, res).BlockHash())
	require.Equal(hashB, n.ctx.round.HashOfBlockInLock())
}

// A lock taken on 2/3 LOCK may hold a proof of lock that is not a quorum; the
// node then skips proposing instead of broadcasting a re-proposal everyone
// rejects, until the proof is completed.
func TestRoundLockWeakProofOfLockSkipsProposal(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	round0, round1, round2 := time.Unix(rlRound0TS, 0), time.Unix(rlRound1TS, 0), time.Unix(roundLockRound2TS, 0)
	n := newRLNode(t, identityset.PrivateKey(25), delegates, keys, 1) // round-2 proposer
	hashA, propA := mintProposal(t, n.cm, n.chain.TipHash(), keys[3], round0)

	n.setClock(round0)
	n.prepare(51, 0)
	_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(propA))
	require.NoError(err)
	var commit interface{}
	for _, idx := range []int{0, 2, 3} {
		commit, err = n.ctx.NewPreCommitEndorsement(rlEndorse(t, keys, idx, hashA[:], LOCK, round0))
		require.NoError(err)
	}
	require.NotNil(commit)
	require.True(n.ctx.round.IsLocked())
	require.False(n.ctx.round.hasValidProofOfLock())

	n.setClock(round1)
	n.prepare(51, 1)
	n.setClock(round2)
	n.prepare(51, 2)
	require.Equal(keys[1].PublicKey().Address().String(), n.ctx.round.Proposer())
	prop, err := n.ctx.Proposal()
	require.NoError(err)
	require.Nil(prop, "a locked proposer with a weak proof of lock must not propose")

	// COMMIT(A) votes arrive: the proof of lock becomes a quorum, the node
	// re-proposes A
	for _, idx := range []int{0, 2, 3} {
		n.feedVote(keys, idx, hashA[:], COMMIT, round0)
	}
	require.True(n.ctx.round.hasValidProofOfLock())
	prop, err = n.ctx.Proposal()
	require.NoError(err)
	require.NotNil(prop)
	bp := prop.(*EndorsedConsensusMessage).Document().(*blockProposal)
	reHash := bp.block.HashBlock()
	require.Equal(hashA[:], reHash[:])
}

// Two polkas in one round need equivocation; the lock does not move and no
// LOCK or COMMIT is signed for the second block in that round.
func TestRoundLockIgnoresSameRoundConflictingPolka(t *testing.T) {
	require := require.New(t)
	n, keys, hashA, _, _ := lockedOnA(t, 1, []int{0, 2, 3})
	round0 := time.Unix(rlRound0TS, 0)
	// the round-0 proposer (delegate 3) equivocates with a second block
	hashC, propC := mintProposal(t, n.cm, n.chain.TipHash(), keys[3], round0.Add(time.Second))
	_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(propC))
	require.Error(err)

	for _, idx := range []int{0, 2, 3} {
		res, err := n.ctx.NewLockEndorsement(rlEndorse(t, keys, idx, hashC[:], PROPOSAL, round0))
		if err == nil {
			require.Nil(res, "no LOCK for a second block in the round of the lock")
		}
	}
	require.Equal(hashA, n.ctx.round.HashOfBlockInLock())
	require.Equal(uint32(0), n.ctx.round.LockRound())

	for _, idx := range []int{0, 2, 3} {
		res, err := n.ctx.NewPreCommitEndorsement(rlEndorse(t, keys, idx, hashC[:], LOCK, round0))
		if err == nil {
			require.Nil(res, "no COMMIT for a second block in the round of the lock")
		}
	}
	require.Equal(hashA, n.ctx.round.HashOfBlockInLock())
}

func TestRoundLockRestoreCapsFutureRound(t *testing.T) {
	require := require.New(t)
	n, _, hashA, _, _ := lockedOnA(t, 1, []int{0, 2, 3})
	r := *n.ctx.round
	r.status, r.blockInLock, r.lockRound, r.proofOfLock = _open, nil, 0, nil
	require.True(r.restoreLock(&lockState{height: 51, round: 5, status: _locked, blkHash: hashA}))
	require.True(r.IsLocked())
	require.Equal(uint32(0), r.LockRound(), "a lock ahead of the clock is kept, as of the current round")
	require.False(r.restoreLock(&lockState{height: 52, round: 0, status: _locked, blkHash: hashA}), "other height")
	require.False(r.restoreLock(&lockState{height: 51, round: 0, status: _locked, blkHash: []byte("unknown")}), "unknown block")
}

// Every round of this proof lies inside (lockRound, current), so only the
// single-round requirement can reject it.
func TestRoundLockRejectsMultiRoundProofOfLockInWindow(t *testing.T) {
	require := require.New(t)
	n, keys, hashA, hashB, propB := lockedOnA(t, 2, []int{0, 1, 3})
	round1, round2, round3 := time.Unix(rlRound1TS, 0), time.Unix(roundLockRound2TS, 0), time.Unix(roundLockRound2TS+20, 0)
	n.setClock(round1)
	n.prepare(51, 1)
	n.setClock(round2)
	n.prepare(51, 2)
	n.setClock(round3)
	n.prepare(51, 3)
	pol := append(proposalPOL(t, keys, []int{0, 1}, hashB, round1), proposalPOL(t, keys, []int{3}, hashB, round2)...)
	_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(reProposal(t, propB, keys[2], round3, pol)))
	require.Error(err, "a proof of lock assembled from two rounds must not move the lock")
	require.Equal(hashA, n.ctx.round.HashOfBlockInLock())
}

// A proof of lock from the lock round itself does not move the lock.
func TestRoundLockRejectsProofOfLockFromLockRound(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	round0, round1, round2 := time.Unix(rlRound0TS, 0), time.Unix(rlRound1TS, 0), time.Unix(roundLockRound2TS, 0)
	n := newRLNode(t, identityset.PrivateKey(25), delegates, keys, 2)
	prevHash := n.chain.TipHash()
	hashB, propB := mintProposal(t, n.cm, prevHash, keys[3], round0)
	hashA, propA := mintProposal(t, n.cm, prevHash, keys[0], round1)

	n.setClock(round0)
	n.prepare(51, 0)
	n.setClock(round1)
	n.prepare(51, 1)
	_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(propA))
	require.NoError(err)
	for _, idx := range []int{0, 1, 3} {
		n.feedVote(keys, idx, hashA[:], PROPOSAL, round1)
	}
	require.Equal(uint32(1), n.ctx.round.LockRound())

	n.setClock(round2)
	n.prepare(51, 2)
	pol := proposalPOL(t, keys, []int{0, 1, 3}, hashB[:], round1)
	_, err = n.ctx.NewProposalEndorsement(n.deliverProposal(reProposal(t, propB, keys[1], round2, pol)))
	require.Error(err, "a proof of lock from the lock round must not move the lock")
	require.Equal(hashA[:], n.ctx.round.HashOfBlockInLock())
	require.Equal(uint32(1), n.ctx.round.LockRound())
}

// A proof of lock must come from a round before the current one; a polka of
// the current round moves the lock through the votes themselves.
func TestRoundLockRejectsProofOfLockFromCurrentRound(t *testing.T) {
	require := require.New(t)
	n, keys, hashA, hashB, propB := lockedOnA(t, 2, []int{0, 1, 3})
	round1, round2 := time.Unix(rlRound1TS, 0), time.Unix(roundLockRound2TS, 0)
	n.setClock(round1)
	n.prepare(51, 1)
	n.setClock(round2)
	n.prepare(51, 2)
	pol := proposalPOL(t, keys, []int{0, 1, 3}, hashB, round2)
	_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(reProposal(t, propB, keys[1], round2, pol)))
	require.Error(err, "a proof of lock from the current round is not accepted on a proposal")
	require.Equal(hashA, n.ctx.round.HashOfBlockInLock())
}

// A locked node keeps at most one refused block per round: further
// conflicting proposals of the round are neither validated nor stored.
func TestRoundLockBoundsRefusedProposals(t *testing.T) {
	require := require.New(t)
	n, keys, hashA, _, _ := lockedOnA(t, 1, []int{0, 2, 3})
	round1, round2 := time.Unix(rlRound1TS, 0), time.Unix(roundLockRound2TS, 0)
	prevHash := n.chain.TipHash()

	n.setClock(round1)
	n.prepare(51, 1)
	before := n.ctx.round.eManager.SizeWithBlock()
	var first []byte
	for k := 0; k < 10; k++ {
		// distinct valid blocks of the round-1 proposer (delegate 0)
		h, prop := mintProposal(t, n.cm, prevHash, keys[0], round1.Add(time.Duration(k)*time.Second))
		if k == 0 {
			first = h[:]
		}
		if k == 5 {
			// the same round rebuilt (e.g. a timeout within tolerated overtime)
			n.prepare(51, 1)
		}
		res, err := n.ctx.NewProposalEndorsement(n.deliverProposal(prop))
		require.Error(err)
		require.Nil(res)
	}
	require.Equal(before+1, n.ctx.round.eManager.SizeWithBlock(), "only the first refused block of the round is kept")
	require.NotNil(n.ctx.round.Block(first), "the first refused block stays observable")
	require.Equal(hashA, n.ctx.round.HashOfBlockInLock())

	// the bound is per round
	n.setClock(round2)
	n.prepare(51, 2)
	_, prop := mintProposal(t, n.cm, prevHash, keys[1], round2)
	_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(prop))
	require.Error(err)
	require.Equal(before+2, n.ctx.round.eManager.SizeWithBlock())
}

// A fresh proposal carrying a nil polka (proof of unlock) of a round in
// (lockRound, current) releases the lock and is endorsed.
func TestRoundLockAcceptsLaterProofOfUnlock(t *testing.T) {
	require := require.New(t)
	n, keys, _, _, _ := lockedOnA(t, 2, []int{0, 1, 3})
	round1, round2 := time.Unix(rlRound1TS, 0), time.Unix(roundLockRound2TS, 0)
	_, propX := mintProposal(t, n.cm, n.chain.TipHash(), keys[1], round2)
	n.setClock(round1)
	n.prepare(51, 1)
	n.setClock(round2)
	n.prepare(51, 2)

	pou := proposalPOL(t, keys, []int{0, 1, 3}, nil, round1)
	res, err := n.ctx.NewProposalEndorsement(n.deliverProposal(reProposal(t, propX, keys[1], round2, pou)))
	require.NoError(err, "a later nil polka unlocks the node")
	hashX := hashB(t, propX)
	require.Equal(hashX, voteOf(t, res).BlockHash())
	require.True(n.ctx.round.IsUnlocked())
	require.Empty(n.ctx.round.HashOfBlockInLock())
}

// A nil polka from the lock round or earlier does not release the lock, and
// the refusal says it was a proof of unlock.
func TestRoundLockRejectsStaleProofOfUnlock(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	round0, round1, round2 := time.Unix(rlRound0TS, 0), time.Unix(rlRound1TS, 0), time.Unix(roundLockRound2TS, 0)
	n := newRLNode(t, identityset.PrivateKey(25), delegates, keys, 2)
	prevHash := n.chain.TipHash()
	hashA, propA := mintProposal(t, n.cm, prevHash, keys[0], round1)
	_, propX := mintProposal(t, n.cm, prevHash, keys[1], round2)

	n.setClock(round0)
	n.prepare(51, 0)
	n.setClock(round1)
	n.prepare(51, 1)
	_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(propA))
	require.NoError(err)
	for _, idx := range []int{0, 1, 3} {
		n.feedVote(keys, idx, hashA[:], PROPOSAL, round1)
	}
	require.Equal(uint32(1), n.ctx.round.LockRound())

	n.setClock(round2)
	n.prepare(51, 2)
	for _, r := range []time.Time{round0, round1} {
		pou := proposalPOL(t, keys, []int{0, 1, 3}, nil, r)
		_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(reProposal(t, propX, keys[1], round2, pou)))
		require.Error(err)
		require.Contains(err.Error(), "nil polka")
		require.Equal(hashA[:], n.ctx.round.HashOfBlockInLock())
	}
}

// A nil polka assembled from two rounds inside the window is refused.
func TestRoundLockRejectsMultiRoundProofOfUnlock(t *testing.T) {
	require := require.New(t)
	n, keys, hashA, _, _ := lockedOnA(t, 2, []int{0, 1, 3})
	round1, round2, round3 := time.Unix(rlRound1TS, 0), time.Unix(roundLockRound2TS, 0), time.Unix(roundLockRound2TS+20, 0)
	_, propX := mintProposal(t, n.cm, n.chain.TipHash(), keys[2], round3)
	n.setClock(round1)
	n.prepare(51, 1)
	n.setClock(round2)
	n.prepare(51, 2)
	n.setClock(round3)
	n.prepare(51, 3)
	pou := append(proposalPOL(t, keys, []int{0, 1}, nil, round1), proposalPOL(t, keys, []int{3}, nil, round2)...)
	_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(reProposal(t, propX, keys[2], round3, pou)))
	require.Error(err)
	require.Equal(hashA, n.ctx.round.HashOfBlockInLock())
}

// A nil "polka" below quorum (here one PROPOSAL(nil)) does not unlock.
func TestRoundLockRejectsInsufficientProofOfUnlock(t *testing.T) {
	require := require.New(t)
	n, keys, hashA, _, _ := lockedOnA(t, 2, []int{0, 1, 3})
	round1, round2 := time.Unix(rlRound1TS, 0), time.Unix(roundLockRound2TS, 0)
	_, propX := mintProposal(t, n.cm, n.chain.TipHash(), keys[1], round2)
	n.setClock(round1)
	n.prepare(51, 1)
	n.setClock(round2)
	n.prepare(51, 2)
	pou := proposalPOL(t, keys, []int{1}, nil, round1) // one PROPOSAL(nil), not a quorum
	_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(reProposal(t, propX, keys[1], round2, pou)))
	require.Error(err, "a single nil endorsement must not unlock")
	require.Equal(hashA, n.ctx.round.HashOfBlockInLock())
}
