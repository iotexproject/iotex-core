// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// round-3 start under the timing of roundlock_disabled_test.go
const statelostRound3TS = rlGenesisTS + 120

// putConsensusDB writes key=val into the consensus DB at path while no node
// has it open
func putConsensusDB(t *testing.T, path string, key, val []byte) {
	t.Helper()
	cfg := db.DefaultConfig
	cfg.DbPath = path
	kv := db.NewBoltDB(cfg)
	require.NoError(t, kv.Start(context.Background()))
	require.NoError(t, kv.Put(_eManagerNS, key, val))
	require.NoError(t, kv.Stop(context.Background()))
}

// signedVotesRecord is a record of a vote signed at the previous height, as an
// intact consensus DB of a running producer holds
func signedVotesRecord(t *testing.T) []byte {
	t.Helper()
	buf, err := serializeSignedVotes(map[ConsensusVoteTopic]*signedVote{
		COMMIT: {height: 50, round: 0, blkHash: []byte("0123456789abcdef0123456789abcdef")},
	})
	require.NoError(t, err)
	return buf
}

// signAtRound0 starts node 1 on dbPath, signs PROPOSAL for A in round 0 and
// stops it
func signAtRound0(t *testing.T, dbPath string, opts ...Option) *rlNode {
	keys, delegates := roundLockKeys()
	n := newRLNodeWithDB(t, identityset.PrivateKey(25), delegates, keys, 1, dbPath, opts...)
	_, propA := mintProposal(t, n.cm, n.chain.TipHash(), keys[3], time.Unix(rlRound0TS, 0))
	n.setClock(time.Unix(rlRound0TS, 0))
	n.prepare(51, 0)
	_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(propA))
	require.NoError(t, err)
	require.NoError(t, n.ctx.Stop(context.Background()))
	return n
}

// requireAbstains checks that node n, started at round 0 of height 51, signs
// nothing up to round 2 and signs again from round 3
func requireAbstains(t *testing.T, n *rlNode) {
	require := require.New(t)
	keys, _ := roundLockKeys()
	hashA, propA := mintProposal(t, n.cm, n.chain.TipHash(), keys[3], time.Unix(rlRound0TS, 0))
	n.setClock(time.Unix(rlRound0TS, 0))
	n.prepare(51, 0)
	res, err := n.ctx.NewProposalEndorsement(n.deliverProposal(propA))
	require.Error(err, "must not sign at the height it started in")
	require.Contains(err.Error(), "before this start are unknown")
	require.Nil(res)
	require.NotNil(n.ctx.round.Block(hashA[:]), "the proposal is still tracked")
	_, err = n.ctx.NewProposalEndorsement(nil)
	require.Error(err, "no empty vote either")

	for i, ts := range []int64{rlRound1TS, roundLockRound2TS} {
		n.setClock(time.Unix(ts, 0))
		n.prepare(51, uint32(i+1))
		require.True(n.ctx.abstaining(), "round %d", i+1)
	}
	n.setClock(time.Unix(statelostRound3TS, 0))
	n.prepare(51, 3)
	require.False(n.ctx.abstaining(), "signs again after the margin")
	res, err = n.ctx.NewProposalEndorsement(nil)
	require.NoError(err)
	require.NotNil(res)

	// the next height is not affected
	n.ctx.round.height = 52
	n.ctx.round.roundNum = 0
	require.False(n.ctx.abstaining())
}

func TestStartupAbstentionNoConsensusDB(t *testing.T) {
	keys, delegates := roundLockKeys()
	n := newRLNode(t, identityset.PrivateKey(25), delegates, keys, 1, WithStartupAbstention(true))
	requireAbstains(t, n)
}

func TestStartupAbstentionFreshConsensusDB(t *testing.T) {
	keys, delegates := roundLockKeys()
	dbPath := filepath.Join(t.TempDir(), "consensus.db")
	n := newRLNodeWithDB(t, identityset.PrivateKey(25), delegates, keys, 1, dbPath, WithStartupAbstention(true))
	requireAbstains(t, n)
}

// A restart on the intact DB signs at once; the same restart after the DB is
// lost does not.
func TestStartupAbstentionIntactAndLostDB(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	dbPath := filepath.Join(t.TempDir(), "consensus.db")
	signAtRound0(t, dbPath)

	r := newRLNodeWithDB(t, identityset.PrivateKey(25), delegates, keys, 1, dbPath, WithStartupAbstention(true))
	hashA, propA := mintProposal(t, r.cm, r.chain.TipHash(), keys[3], time.Unix(rlRound0TS, 0))
	r.setClock(time.Unix(rlRound0TS, 0))
	r.prepare(51, 0)
	require.False(r.ctx.abstaining())
	res, err := r.ctx.NewProposalEndorsement(r.deliverProposal(propA))
	require.NoError(err)
	require.Equal(hashA[:], voteOf(t, res).BlockHash())
	require.NoError(r.ctx.Stop(context.Background()))

	require.NoError(os.Remove(dbPath))
	l := newRLNodeWithDB(t, identityset.PrivateKey(25), delegates, keys, 1, dbPath, WithStartupAbstention(true))
	requireAbstains(t, l)
}

// An unreadable signed votes record or round lock means the state is unknown.
func TestStartupAbstentionUnreadableRecords(t *testing.T) {
	keys, delegates := roundLockKeys()
	for _, c := range []struct {
		name string
		key  []byte
	}{
		{"signedVotes", _signedVotesKey},
		{"lockState", _lockStateKey},
	} {
		t.Run(c.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "consensus.db")
			putConsensusDB(t, dbPath, _signedVotesKey, signedVotesRecord(t))
			putConsensusDB(t, dbPath, c.key, []byte{0xff})
			n := newRLNodeWithDB(t, identityset.PrivateKey(25), delegates, keys, 1, dbPath, WithStartupAbstention(true))
			requireAbstains(t, n)
		})
	}
}

// A persisted lock that cannot be restored makes the node sit out that height.
func TestStartupAbstentionLockNotRestored(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	dbPath := filepath.Join(t.TempDir(), "consensus.db")
	putConsensusDB(t, dbPath, _signedVotesKey, signedVotesRecord(t))
	ls := &lockState{height: 51, round: 0, status: _locked, blkHash: []byte("0123456789abcdef0123456789abcdef")}
	buf, err := ls.serialize()
	require.NoError(err)
	putConsensusDB(t, dbPath, _lockStateKey, buf)

	n := newRLNodeWithDB(t, identityset.PrivateKey(25), delegates, keys, 1, dbPath, WithStartupAbstention(true))
	require.Zero(n.ctx.abstainHeight, "the signed votes are known")
	requireAbstains(t, n)
	require.False(n.ctx.round.IsLocked())
}

func TestStartupAbstentionNotApplied(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	round0 := time.Unix(rlRound0TS, 0)
	for _, c := range []struct {
		name string
		opts []Option
		prep func(*rlNode)
	}{
		{"option off", nil, nil},
		{"enforcement off", []Option{WithStartupAbstention(true), WithRoundLockEnforcement(false)}, nil},
		{"others lack a quorum", []Option{WithStartupAbstention(true)}, func(n *rlNode) {
			// this node holds 2 of the 4 delegate keys
			n.ctx.priKeys = append(n.ctx.priKeys, keys[2])
			n.ctx.encodedAddrs = append(n.ctx.encodedAddrs, delegates[2])
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			n := newRLNode(t, identityset.PrivateKey(25), delegates, keys, 1, c.opts...)
			if c.prep != nil {
				c.prep(n)
			}
			_, propA := mintProposal(t, n.cm, n.chain.TipHash(), keys[3], round0)
			n.setClock(round0)
			n.prepare(51, 0)
			require.False(n.ctx.abstaining())
			res, err := n.ctx.NewProposalEndorsement(n.deliverProposal(propA))
			require.NoError(err)
			require.Len(res, len(n.ctx.priKeys), "one vote per key")
		})
	}
}

// The first round after the start may be late in the height; the node sits
// out the rounds up to it plus the margin.
func TestStartupAbstentionLateRound(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	n := newRLNode(t, identityset.PrivateKey(25), delegates, keys, 1, WithStartupAbstention(true))
	n.setClock(time.Unix(roundLockRound2TS, 0))
	n.prepare(51, 2)
	require.Equal(uint32(2+_abstainRounds), n.ctx.abstainRound)
	require.True(n.ctx.abstaining())
	n.setClock(time.Unix(statelostRound3TS, 0))
	n.prepare(51, 3)
	require.True(n.ctx.abstaining())
}

// A proposer that sits out does not propose either.
func TestStartupAbstentionSkipsProposal(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	// round 0 of height 51 is proposed by delegate 3
	n := newRLNode(t, identityset.PrivateKey(25), delegates, keys, 3, WithStartupAbstention(true))
	n.setClock(time.Unix(rlRound0TS, 0))
	n.prepare(51, 0)
	require.Equal(delegates[3], n.ctx.round.Proposer())
	p, err := n.ctx.Proposal()
	require.NoError(err)
	require.Nil(p)

	c := newRLNode(t, identityset.PrivateKey(25), delegates, keys, 3)
	c.setClock(time.Unix(rlRound0TS, 0))
	c.prepare(51, 0)
	p, err = c.ctx.Proposal()
	require.NoError(err)
	require.NotNil(p)
}

// Another restart at the same height still finds the lock it could not
// restore (or could not read), and sits out again.
func TestStartupAbstentionSurvivesAnotherRestart(t *testing.T) {
	keys, delegates := roundLockKeys()
	unrestorable := func(t *testing.T) []byte {
		ls := &lockState{height: 51, round: 0, status: _locked, blkHash: []byte("0123456789abcdef0123456789abcdef")}
		buf, err := ls.serialize()
		require.NoError(t, err)
		return buf
	}
	for _, c := range []struct {
		name string
		lock func(*testing.T) []byte
	}{
		{"lock not restorable", unrestorable},
		{"lock unreadable", func(*testing.T) []byte { return []byte{0xff} }},
	} {
		t.Run(c.name, func(t *testing.T) {
			require := require.New(t)
			dbPath := filepath.Join(t.TempDir(), "consensus.db")
			putConsensusDB(t, dbPath, _signedVotesKey, signedVotesRecord(t))
			putConsensusDB(t, dbPath, _lockStateKey, c.lock(t))

			n := newRLNodeWithDB(t, identityset.PrivateKey(25), delegates, keys, 1, dbPath, WithStartupAbstention(true))
			n.setClock(time.Unix(rlRound0TS, 0))
			n.prepare(51, 0)
			require.True(n.ctx.abstaining())
			require.NoError(n.ctx.Stop(context.Background()))

			r := newRLNodeWithDB(t, identityset.PrivateKey(25), delegates, keys, 1, dbPath, WithStartupAbstention(true))
			r.setClock(time.Unix(roundLockRound2TS, 0))
			r.prepare(51, 2)
			require.True(r.ctx.abstaining(), "the second restart must not end the window early")
			_, err := r.ctx.NewProposalEndorsement(nil)
			require.Error(err)
			r.setClock(time.Unix(rlGenesisTS+180, 0))
			r.prepare(51, 6)
			require.False(r.ctx.abstaining(), "the window of the second restart ends too")
		})
	}
}

// The same key configured twice is still one delegate.
func TestStartupAbstentionDuplicateKey(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	n := newRLNode(t, identityset.PrivateKey(25), delegates, keys, 1, WithStartupAbstention(true))
	n.ctx.priKeys = append(n.ctx.priKeys, keys[1])
	n.ctx.encodedAddrs = append(n.ctx.encodedAddrs, delegates[1])
	n.setClock(time.Unix(rlRound0TS, 0))
	n.prepare(51, 0)
	require.True(n.ctx.abstaining())
}

// Starting the same context again decides from what that start reads back.
func TestStartupAbstentionRestartSameContext(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	dbPath := filepath.Join(t.TempDir(), "consensus.db")
	n := newRLNodeWithDB(t, identityset.PrivateKey(25), delegates, keys, 1, dbPath, WithStartupAbstention(true))
	require.Equal(uint64(51), n.ctx.abstainHeight)
	require.NoError(n.ctx.Stop(context.Background()))

	putConsensusDB(t, dbPath, _signedVotesKey, signedVotesRecord(t))
	require.NoError(n.ctx.Start(context.Background()))
	require.Zero(n.ctx.abstainHeight)
	n.setClock(time.Unix(rlRound0TS, 0))
	n.prepare(51, 0)
	require.False(n.ctx.abstaining())
}

// Keeping an unreadable lock record must not keep the endorsements and blocks
// recorded before the start: the first round still starts from a clean slate.
func TestStartupAbstentionUnreadableLockKeepsNoEndorsements(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	dbPath := filepath.Join(t.TempDir(), "consensus.db")
	round0 := time.Unix(rlRound0TS, 0)

	n := newRLNodeWithDB(t, identityset.PrivateKey(25), delegates, keys, 1, dbPath)
	hashA, propA := mintProposal(t, n.cm, n.chain.TipHash(), keys[3], round0)
	n.setClock(round0)
	n.prepare(51, 0)
	_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(propA))
	require.NoError(err)
	for _, idx := range []int{0, 2, 3} {
		n.feedVote(keys, idx, hashA[:], COMMIT, round0)
	}
	require.NotNil(n.ctx.round.Block(hashA[:]))
	require.NoError(n.ctx.Stop(context.Background()))
	putConsensusDB(t, dbPath, _lockStateKey, []byte{0xff})

	r := newRLNodeWithDB(t, identityset.PrivateKey(25), delegates, keys, 1, dbPath, WithStartupAbstention(true))
	require.NotNil(r.ctx.round.eManager.collections[encodeToString(hashA[:])], "the status record held A")
	r.setClock(round0)
	r.prepare(51, 0)
	require.True(r.ctx.abstaining())
	require.Nil(r.ctx.round.Block(hashA[:]), "blocks from before the start are dropped")
	require.Empty(r.ctx.round.Endorsements(hashA[:], []ConsensusVoteTopic{COMMIT}))
	_, err = r.ctx.eManagerDB.Get(_eManagerNS, _lockStateKey)
	require.NoError(err, "the unreadable lock record is kept for this height")
}

// failingStatusKV fails writes of the endorsement status while fail is set
type failingStatusKV struct {
	db.KVStore
	fail bool
}

func (kv *failingStatusKV) Put(ns string, key, value []byte) error {
	if kv.fail && string(key) == string(_statusKey) {
		return errors.New("injected write failure")
	}
	return kv.KVStore.Put(ns, key, value)
}

// A Prepare retried at the first height after a failed write still keeps the
// unreadable lock record.
func TestStartupAbstentionUnreadableLockSurvivesRetriedPrepare(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	dbPath := filepath.Join(t.TempDir(), "consensus.db")
	putConsensusDB(t, dbPath, _signedVotesKey, signedVotesRecord(t))
	putConsensusDB(t, dbPath, _lockStateKey, []byte{0xff})

	n := newRLNodeWithDB(t, identityset.PrivateKey(25), delegates, keys, 1, dbPath, WithStartupAbstention(true))
	kv := &failingStatusKV{KVStore: n.ctx.round.eManager.eManagerDB, fail: true}
	n.ctx.round.eManager.eManagerDB = kv
	n.setClock(time.Unix(rlRound0TS, 0))
	require.Error(n.ctx.Prepare())
	require.Zero(n.ctx.round.Height(), "the failed Prepare did not move on")
	kv.fail = false
	n.prepare(51, 0)
	require.True(n.ctx.abstaining())
	require.NoError(n.ctx.Stop(context.Background()))

	r := newRLNodeWithDB(t, identityset.PrivateKey(25), delegates, keys, 1, dbPath, WithStartupAbstention(true))
	r.setClock(time.Unix(rlRound1TS, 0))
	r.prepare(51, 1)
	require.True(r.ctx.abstaining(), "the record survived the retried Prepare")
}

// The unreadable lock record is cleared once the height moves on.
func TestStartupAbstentionUnreadableLockClearedAtNextHeight(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	dbPath := filepath.Join(t.TempDir(), "consensus.db")
	putConsensusDB(t, dbPath, _signedVotesKey, signedVotesRecord(t))
	putConsensusDB(t, dbPath, _lockStateKey, []byte{0xff})

	n := newRLNodeWithDB(t, identityset.PrivateKey(25), delegates, keys, 1, dbPath, WithStartupAbstention(true))
	n.setClock(time.Unix(rlRound0TS, 0))
	n.prepare(51, 0)
	_, err := n.ctx.eManagerDB.Get(_eManagerNS, _lockStateKey)
	require.NoError(err, "kept at the first height")

	// block 51 is committed (e.g. by block sync); the node moves on to 52
	blk, err := n.chain.MintNewBlock(time.Unix(rlRound0TS, 0))
	require.NoError(err)
	require.NoError(blk.Finalize(nil, time.Unix(rlRound0TS, 0)))
	require.NoError(n.chain.CommitBlock(blk))
	n.setClock(time.Unix(rlRound0TS+20, 0))
	require.NoError(n.ctx.Prepare())
	require.Equal(uint64(52), n.ctx.round.Height())
	require.False(n.ctx.abstaining())
	_, err = n.ctx.eManagerDB.Get(_eManagerNS, _lockStateKey)
	require.Equal(db.ErrNotExist, errors.Cause(err), "cleared at the next height")
}

// While sitting out, a node takes in one proposal per round: an equivocating
// proposer cannot make it validate and store block after block.
func TestStartupAbstentionBoundsProposals(t *testing.T) {
	require := require.New(t)
	keys, delegates := roundLockKeys()
	round0, round1 := time.Unix(rlRound0TS, 0), time.Unix(rlRound1TS, 0)
	n := newRLNode(t, identityset.PrivateKey(25), delegates, keys, 1, WithStartupAbstention(true))
	prevHash := n.chain.TipHash()
	hashA, propA := mintProposal(t, n.cm, prevHash, keys[3], round0)
	hashC, propC := mintProposal(t, n.cm, prevHash, keys[3], round0.Add(time.Second))
	hashB, propB := mintProposal(t, n.cm, prevHash, keys[0], round1)

	n.setClock(round0)
	n.prepare(51, 0)
	_, err := n.ctx.NewProposalEndorsement(n.deliverProposal(propA))
	require.Error(err)
	require.NotNil(n.ctx.round.Block(hashA[:]), "the first proposal of the round is taken in")
	size := n.ctx.round.eManager.SizeWithBlock()
	for i := 0; i < 3; i++ {
		if i == 1 {
			// the same round rebuilt (e.g. a timeout within tolerated overtime)
			n.prepare(51, 0)
		}
		_, err = n.ctx.NewProposalEndorsement(n.deliverProposal(propC))
		require.Error(err)
	}
	require.Nil(n.ctx.round.Block(hashC[:]), "a second proposal of the round is not")
	require.Equal(size, n.ctx.round.eManager.SizeWithBlock())

	// the next round takes in one again
	n.setClock(round1)
	n.prepare(51, 1)
	_, err = n.ctx.NewProposalEndorsement(n.deliverProposal(propB))
	require.Error(err)
	require.NotNil(n.ctx.round.Block(hashB[:]))
}
