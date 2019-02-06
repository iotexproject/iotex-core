// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

const testTriePath = "trie.test"

func voteForm(height uint64, cs []*state.Candidate) []string {
	r := make([]string, len(cs))
	for i := 0; i < len(cs); i++ {
		r[i] = cs[i].Address + ":" + strconv.FormatInt(cs[i].Votes.Int64(), 10)
	}
	return r
}

func TestSnapshot(t *testing.T) {
	require := require.New(t)
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	cfg := config.Default
	cfg.DB.DbPath = testTriePath
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewOnDiskDB(cfg.DB)))
	require.NoError(err)
	require.NoError(sf.Start(context.Background()))
	addr := testaddress.Addrinfo["alfa"].Bech32()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	_, err = account.LoadOrCreateAccount(ws, addr, big.NewInt(5))
	require.NoError(err)
	sHash, err := address.Bech32ToPKHash(addr)
	require.NoError(err)

	s, err := account.LoadAccount(ws, sHash)
	require.NoError(err)
	require.Equal(big.NewInt(5), s.Balance)
	s0 := ws.Snapshot()
	require.Zero(s0)
	s.Balance.Add(s.Balance, big.NewInt(5))
	require.Equal(big.NewInt(10), s.Balance)
	require.NoError(ws.PutState(sHash, s))
	s1 := ws.Snapshot()
	require.Equal(1, s1)
	s.Balance.Add(s.Balance, big.NewInt(5))
	require.Equal(big.NewInt(15), s.Balance)
	require.NoError(ws.PutState(sHash, s))
	// add another account
	addr = testaddress.Addrinfo["bravo"].Bech32()
	_, err = account.LoadOrCreateAccount(ws, addr, big.NewInt(7))
	require.NoError(err)
	tHash, err := address.Bech32ToPKHash(addr)
	require.NoError(err)
	s, err = account.LoadAccount(ws, tHash)
	require.NoError(err)
	require.Equal(big.NewInt(7), s.Balance)
	s2 := ws.Snapshot()
	require.Equal(2, s2)
	require.NoError(s.AddBalance(big.NewInt(6)))
	require.Equal(big.NewInt(13), s.Balance)
	require.NoError(ws.PutState(tHash, s))

	require.NoError(ws.Revert(s2))
	require.NoError(ws.State(sHash, s))
	require.Equal(big.NewInt(15), s.Balance)
	require.NoError(ws.State(tHash, s))
	require.Equal(big.NewInt(7), s.Balance)
	require.NoError(ws.Revert(s1))
	require.NoError(ws.State(sHash, s))
	require.Equal(big.NewInt(10), s.Balance)
	require.Equal(state.ErrStateNotExist, errors.Cause(ws.State(tHash, s)))
	require.NoError(ws.Revert(s0))
	require.NoError(ws.State(sHash, s))
	require.Equal(big.NewInt(5), s.Balance)
	require.Equal(state.ErrStateNotExist, errors.Cause(ws.State(tHash, s)))
}

// Test configure: candidateSize = 2, candidateBufferSize = 3
//func TestCandidatePool(t *testing.T) {
//	c1 := &Candidate{Address: "a1", Votes: big.NewInt(1), PublicKey: []byte("p1")}
//	c2 := &Candidate{Address: "a2", Votes: big.NewInt(2), PublicKey: []byte("p2")}
//	c3 := &Candidate{Address: "a3", Votes: big.NewInt(3), PublicKey: []byte("p3")}
//	c4 := &Candidate{Address: "a4", Votes: big.NewInt(4), PublicKey: []byte("p4")}
//	c5 := &Candidate{Address: "a5", Votes: big.NewInt(5), PublicKey: []byte("p5")}
//	c6 := &Candidate{Address: "a6", Votes: big.NewInt(6), PublicKey: []byte("p6")}
//	c7 := &Candidate{Address: "a7", Votes: big.NewInt(7), PublicKey: []byte("p7")}
//	c8 := &Candidate{Address: "a8", Votes: big.NewInt(8), PublicKey: []byte("p8")}
//	c9 := &Candidate{Address: "a9", Votes: big.NewInt(9), PublicKey: []byte("p9")}
//	c10 := &Candidate{Address: "a10", Votes: big.NewInt(10), PublicKey: []byte("p10")}
//	c11 := &Candidate{Address: "a11", Votes: big.NewInt(11), PublicKey: []byte("p11")}
//	c12 := &Candidate{Address: "a12", Votes: big.NewInt(12), PublicKey: []byte("p12")}
//	tr, _ := trie.NewTrie("trie.test", false)
//	sf := &stateFactory{
//		trie:                   tr,
//		candidateHeap:          CandidateMinPQ{candidateSize, make([]*Candidate, 0)},
//		candidateBufferMinHeap: CandidateMinPQ{candidateBufferSize, make([]*Candidate, 0)},
//		candidateBufferMaxHeap: CandidateMaxPQ{candidateBufferSize, make([]*Candidate, 0)},
//	}
//
//	sf.updateVotes(c1, big.NewInt(1))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a1:1"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{}))
//
//	sf.updateVotes(c1, big.NewInt(2))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a1:2"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{}))
//
//	sf.updateVotes(c2, big.NewInt(2))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a1:2", "a2:2"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{}))
//
//	sf.updateVotes(c3, big.NewInt(3))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a2:2", "a3:3"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{"a1:2"}))
//
//	sf.updateVotes(c4, big.NewInt(4))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a3:3", "a4:4"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{"a1:2", "a2:2"}))
//
//	sf.updateVotes(c2, big.NewInt(1))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a3:3", "a4:4"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{"a1:2", "a2:1"}))
//
//	sf.updateVotes(c5, big.NewInt(5))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a4:4", "a5:5"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{"a1:2", "a2:1", "a3:3"}))
//
//	sf.updateVotes(c2, big.NewInt(9))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a2:9", "a5:5"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{"a1:2", "a3:3", "a4:4"}))
//
//	sf.updateVotes(c6, big.NewInt(6))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a2:9", "a6:6"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{"a3:3", "a4:4", "a5:5"}))
//
//	sf.updateVotes(c1, big.NewInt(10))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a1:10", "a2:9"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{"a4:4", "a5:5", "a6:6"}))
//
//	sf.updateVotes(c7, big.NewInt(7))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a1:10", "a2:9"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{"a5:5", "a6:6", "a7:7"}))
//
//	sf.updateVotes(c3, big.NewInt(8))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a1:10", "a2:9"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{"a3:8", "a6:6", "a7:7"}))
//
//	sf.updateVotes(c8, big.NewInt(12))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a1:10", "a8:12"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{"a2:9", "a3:8", "a7:7"}))
//
//	sf.updateVotes(c4, big.NewInt(8))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a1:10", "a8:12"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{"a2:9", "a3:8", "a4:8"}))
//
//	sf.updateVotes(c6, big.NewInt(7))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a1:10", "a8:12"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{"a2:9", "a3:8", "a4:8"}))
//
//	sf.updateVotes(c1, big.NewInt(1))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a2:9", "a8:12"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{"a3:8", "a4:8", "a1:1"}))
//
//	sf.updateVotes(c9, big.NewInt(2))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a2:9", "a8:12"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{"a3:8", "a4:8", "a9:2"}))
//
//	sf.updateVotes(c10, big.NewInt(8))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a2:9", "a8:12"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{"a10:8", "a3:8", "a4:8"}))
//
//	sf.updateVotes(c11, big.NewInt(3))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a2:9", "a8:12"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{"a10:8", "a3:8", "a4:8"}))
//
//	sf.updateVotes(c12, big.NewInt(1))
//	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a2:9", "a8:12"}))
//	assert.True(t, compareStrings(voteForm(sf.candidatesBuffer()), []string{"a10:8", "a3:8", "a4:8"}))
//}

func TestCandidates(t *testing.T) {
	// Create three dummy iotex addresses
	a := testaddress.Addrinfo["alfa"].Bech32()
	priKeyA := testaddress.Keyinfo["alfa"].PriKey
	b := testaddress.Addrinfo["bravo"].Bech32()
	priKeyB := testaddress.Keyinfo["bravo"].PriKey
	c := testaddress.Addrinfo["charlie"].Bech32()
	priKeyC := testaddress.Keyinfo["charlie"].PriKey
	d := testaddress.Addrinfo["delta"].Bech32()
	priKeyD := testaddress.Keyinfo["delta"].PriKey
	e := testaddress.Addrinfo["echo"].Bech32()
	priKeyE := testaddress.Keyinfo["echo"].PriKey
	f := testaddress.Addrinfo["foxtrot"].Bech32()
	priKeyF := testaddress.Keyinfo["foxtrot"].PriKey
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	cfg := config.Default
	cfg.Chain.NumCandidates = 2
	cfg.DB.DbPath = testTriePath
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewOnDiskDB(cfg.DB)))
	require.NoError(t, err)
	sf.AddActionHandlers(account.NewProtocol(), vote.NewProtocol(nil))
	require.NoError(t, sf.Start(context.Background()))
	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	_, err = account.LoadOrCreateAccount(ws, a, big.NewInt(100))
	require.NoError(t, err)
	_, err = account.LoadOrCreateAccount(ws, b, big.NewInt(200))
	require.NoError(t, err)
	_, err = account.LoadOrCreateAccount(ws, c, big.NewInt(300))
	require.NoError(t, err)
	_, err = account.LoadOrCreateAccount(ws, d, big.NewInt(100))
	require.NoError(t, err)
	_, err = account.LoadOrCreateAccount(ws, e, big.NewInt(100))
	require.NoError(t, err)
	_, err = account.LoadOrCreateAccount(ws, f, big.NewInt(300))
	require.NoError(t, err)

	// a:100(0) b:200(0) c:300(0)
	tx1, err := action.NewTransfer(uint64(1), big.NewInt(10), a, b, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).SetDestinationAddress(b).SetAction(tx1).Build()
	selp1, err := action.Sign(elp, a, priKeyA)
	require.NoError(t, err)

	tx2, err := action.NewTransfer(uint64(2), big.NewInt(20), a, c, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(2).SetDestinationAddress(c).SetAction(tx2).Build()
	selp2, err := action.Sign(elp, a, priKeyA)
	require.NoError(t, err)

	gasLimit := uint64(1000000)
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer:        testaddress.Addrinfo["producer"],
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	newRoot, _, err := ws.RunActions(ctx, 0, []action.SealedEnvelope{selp1, selp2})
	require.Nil(t, err)
	root := newRoot
	require.NotEqual(t, hash.ZeroHash32B, root)
	require.Nil(t, sf.Commit(ws))
	balanceB, err := sf.Balance(b)
	require.Nil(t, err)
	require.Equal(t, balanceB, big.NewInt(210))
	balanceC, err := sf.Balance(c)
	require.Nil(t, err)
	require.Equal(t, balanceC, big.NewInt(320))
	h, _ := sf.Height()
	cand, _ := sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{}))
	// a:70 b:210 c:320

	vote, err := action.NewVote(0, a, a, uint64(20000), big.NewInt(0))
	require.NoError(t, err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote).
		SetDestinationAddress(a).SetGasLimit(20000).Build()
	selp, err := action.Sign(elp, a, priKeyA)
	require.NoError(t, err)
	zeroGasLimit := uint64(0)
	zctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer:        testaddress.Addrinfo["producer"],
			GasLimit:        &zeroGasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(zctx, 0, []action.SealedEnvelope{selp})
	require.NotNil(t, err)
	newRoot, _, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":70"}))
	// a(a):70(+0=70) b:210 c:320

	vote2, err := action.NewVote(0, b, b, uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote2).
		SetDestinationAddress(b).SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, b, priKeyB)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 1, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":70", b + ":210"}))
	// a(a):70(+0=70) b(b):210(+0=210) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote3, err := action.NewVote(1, a, b, uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote3).SetNonce(1).
		SetDestinationAddress(b).SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, a, priKeyA)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 2, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":0", b + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx3, err := action.NewTransfer(uint64(2), big.NewInt(20), b, a, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx3).SetNonce(2).
		SetDestinationAddress(a).Build()
	selp, err = action.Sign(elp, b, priKeyB)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 3, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":0", b + ":280"}))
	// a(b):90(0) b(b):190(+90=280) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx4, err := action.NewTransfer(uint64(2), big.NewInt(20), a, b, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx4).SetNonce(2).
		SetDestinationAddress(b).Build()
	selp, err = action.Sign(elp, a, priKeyA)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 4, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":0", b + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote4, err := action.NewVote(1, b, a, uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote4).SetNonce(1).
		SetDestinationAddress(a).SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, b, priKeyB)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 5, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":210", b + ":70"}))
	// a(b):70(210) b(a):210(70) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote5, err := action.NewVote(2, b, b, uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote5).SetNonce(2).
		SetDestinationAddress(b).SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, b, priKeyB)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 6, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":0", b + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote6, err := action.NewVote(3, b, b, uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote6).SetNonce(3).
		SetDestinationAddress(b).SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, b, priKeyB)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 7, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":0", b + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx5, err := action.NewTransfer(uint64(2), big.NewInt(20), c, a, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx5).SetNonce(2).
		SetDestinationAddress(a).Build()
	selp, err = action.Sign(elp, c, priKeyC)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 8, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":0", b + ":300"}))
	// a(b):90(0) b(b):210(+90=300) !c:300

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote7, err := action.NewVote(0, c, a, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote7).SetNonce(0).
		SetDestinationAddress(a).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, c, priKeyC)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 9, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":300", b + ":300"}))
	// a(b):90(300) b(b):210(+90=300) !c(a):300

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote8, err := action.NewVote(4, b, c, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote8).SetNonce(4).
		SetDestinationAddress(c).Build()
	selp, err = action.Sign(elp, b, priKeyB)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 10, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":300", b + ":90"}))
	// a(b):90(300) b(c):210(90) !c(a):300

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote9, err := action.NewVote(1, c, c, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote9).SetNonce(1).
		SetDestinationAddress(c).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, c, priKeyC)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 11, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":510", b + ":90"}))
	// a(b):90(0) b(c):210(90) c(c):300(+210=510)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote10, err := action.NewVote(0, d, e, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote10).SetNonce(0).
		SetDestinationAddress(e).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, d, priKeyD)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 12, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":510", b + ":90"}))
	// a(b):90(0) b(c):210(90) c(c):300(+210=510)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote11, err := action.NewVote(1, d, d, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote11).SetNonce(1).
		SetDestinationAddress(d).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, d, priKeyD)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 13, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":510", d + ":100"}))
	// a(b):90(0) b(c):210(90) c(c):300(+210=510) d(d): 100(100)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote12, err := action.NewVote(2, d, a, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote12).SetNonce(2).
		SetDestinationAddress(a).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, d, priKeyD)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 14, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":510", a + ":100"}))
	// a(b):90(100) b(c):210(90) c(c):300(+210=510) d(a): 100(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote13, err := action.NewVote(2, c, d, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote13).SetNonce(2).
		SetDestinationAddress(d).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, c, priKeyC)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 15, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":210", d + ":300"}))
	// a(b):90(100) b(c):210(90) c(d):300(210) d(a): 100(300)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote14, err := action.NewVote(3, c, c, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote14).SetNonce(3).
		SetDestinationAddress(c).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, c, priKeyC)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 16, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":510", a + ":100"}))
	// a(b):90(100) b(c):210(90) c(c):300(+210=510) d(a): 100(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx6, err := action.NewTransfer(uint64(1), big.NewInt(200), c, e, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx6).SetNonce(1).
		SetDestinationAddress(e).Build()
	selp1, err = action.Sign(elp, c, priKeyC)
	require.NoError(t, err)

	tx7, err := action.NewTransfer(uint64(2), big.NewInt(200), b, e, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx7).SetNonce(2).
		SetDestinationAddress(e).Build()
	selp2, err = action.Sign(elp, b, priKeyB)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 17, []action.SealedEnvelope{selp1, selp2})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":110", a + ":100"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(0) !e:500

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote15, err := action.NewVote(0, e, e, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote15).SetNonce(0).
		SetDestinationAddress(e).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, e, priKeyE)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 18, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":110", e + ":500"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(0) e(e):500(+0=500)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote16, err := action.NewVote(0, f, f, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote16).SetNonce(0).
		SetDestinationAddress(f).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, f, priKeyF)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 19, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{f + ":300", e + ":500"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(0) e(e):500(+0=500) f(f):300(+0=300)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote17, err := action.NewVote(0, f, d, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote17).SetNonce(0).
		SetDestinationAddress(d).SetGasLimit(100000).Build()
	selp1, err = action.Sign(elp, f, priKeyF)
	require.NoError(t, err)

	vote18, err := action.NewVote(1, f, d, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote18).SetNonce(1).
		SetDestinationAddress(d).SetGasLimit(100000).Build()
	selp2, err = action.Sign(elp, f, priKeyF)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 20, []action.SealedEnvelope{selp1, selp2})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{d + ":300", e + ":500"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(300) e(e):500(+0=500) f(d):300(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx8, err := action.NewTransfer(uint64(1), big.NewInt(200), f, b, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx8).SetNonce(1).
		SetDestinationAddress(b).Build()
	selp, err = action.Sign(elp, f, priKeyF)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 21, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":310", e + ":500"}))
	// a(b):90(100) b(c):210(90) c(c):100(+210=310) d(a): 100(100) e(e):500(+0=500) f(d):100(0)
	//fmt.Printf("%v \n", voteForm(sf.candidatesBuffer()))

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx9, err := action.NewTransfer(uint64(1), big.NewInt(10), b, a, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx9).SetNonce(1).
		SetDestinationAddress(a).Build()
	selp, err = action.Sign(elp, b, priKeyB)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 22, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":300", e + ":500"}))
	// a(b):100(100) b(c):200(100) c(c):100(+200=300) d(a): 100(100) e(e):500(+0=500) f(d):100(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx10, err := action.NewTransfer(uint64(1), big.NewInt(300), e, d, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx10).SetNonce(1).
		SetDestinationAddress(d).Build()
	selp, err = action.Sign(elp, e, priKeyE)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 23, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, err = sf.Height()
	require.Equal(t, uint64(23), h)
	require.NoError(t, err)
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":300", a + ":400"}))
	// a(b):100(400) b(c):200(100) c(c):100(+200=300) d(a): 400(100) e(e):200(+0=200) f(d):100(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote19, err := action.NewVote(0, d, a, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote19).SetNonce(0).
		SetDestinationAddress(a).SetGasLimit(100000).Build()
	selp1, err = action.Sign(elp, d, priKeyD)
	require.NoError(t, err)

	vote20, err := action.NewVote(3, d, b, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote20).SetNonce(3).
		SetDestinationAddress(b).SetGasLimit(100000).Build()
	selp2, err = action.Sign(elp, d, priKeyD)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 24, []action.SealedEnvelope{selp1, selp2})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, err = sf.Height()
	require.Equal(t, uint64(24), h)
	require.NoError(t, err)
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":300", b + ":500"}))
	// a(b):100(0) b(c):200(500) c(c):100(+200=300) d(b): 400(100) e(e):200(+0=200) f(d):100(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote21, err := action.NewVote(4, c, "", uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote21).SetNonce(4).
		SetDestinationAddress("").SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, c, priKeyC)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 25, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	require.Equal(t, uint64(25), h)
	require.NoError(t, err)
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{e + ":200", b + ":500"}))
	// a(b):100(0) b(c):200(500) [c(c):100(+200=300)] d(b): 400(100) e(e):200(+0=200) f(d):100(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote22, err := action.NewVote(4, f, "", uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote22).SetNonce(4).
		SetDestinationAddress("").SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, f, priKeyF)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 26, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	require.Equal(t, uint64(26), h)
	require.NoError(t, err)
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{e + ":200", b + ":500"}))
	// a(b):100(0) b(c):200(500) [c(c):100(+200=300)] d(b): 400(100) e(e):200(+0=200) f(d):100(0)
	stateA, err := account.LoadOrCreateAccount(ws, a, big.NewInt(0))
	require.Nil(t, err)
	require.Equal(t, stateA.Balance, big.NewInt(100))
}

func TestUnvote(t *testing.T) {
	// Create three dummy iotex addresses
	a := testaddress.Addrinfo["alfa"].Bech32()
	priKeyA := testaddress.Keyinfo["alfa"].PriKey
	b := testaddress.Addrinfo["bravo"].Bech32()
	priKeyB := testaddress.Keyinfo["bravo"].PriKey

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	cfg := config.Default
	cfg.Chain.NumCandidates = 2
	cfg.DB.DbPath = testTriePath
	f, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewOnDiskDB(cfg.DB)))
	require.NoError(t, err)
	sf, ok := f.(*factory)
	require.True(t, ok)
	sf.AddActionHandlers(vote.NewProtocol(nil))
	require.NoError(t, sf.Start(context.Background()))

	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	_, err = account.LoadOrCreateAccount(ws, a, big.NewInt(100))
	require.NoError(t, err)
	_, err = account.LoadOrCreateAccount(ws, b, big.NewInt(200))
	require.NoError(t, err)

	vote1, err := action.NewVote(0, a, "", uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(vote1).SetNonce(0).
		SetDestinationAddress("").SetGasLimit(100000).Build()
	selp, err := action.Sign(elp, a, priKeyA)
	require.NoError(t, err)

	gasLimit := uint64(10000000)
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer:        testaddress.Addrinfo["producer"],
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.Nil(t, sf.Commit(ws))
	h, _ := sf.Height()
	cand, _ := sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{}))

	vote2, err := action.NewVote(0, a, a, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote2).SetNonce(0).
		SetDestinationAddress(a).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, a, priKeyA)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":100"}))

	vote3, err := action.NewVote(0, a, "", uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote3).SetNonce(0).
		SetDestinationAddress("").SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, a, priKeyA)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{}))

	vote4, err := action.NewVote(0, b, b, uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote4).SetNonce(0).
		SetDestinationAddress(b).SetGasLimit(20000).Build()
	selp1, err := action.Sign(elp, b, priKeyB)
	require.NoError(t, err)

	vote5, err := action.NewVote(0, a, b, uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote5).SetNonce(0).
		SetDestinationAddress(b).SetGasLimit(20000).Build()
	selp2, err := action.Sign(elp, a, priKeyA)
	require.NoError(t, err)

	vote6, err := action.NewVote(0, a, "", uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote6).SetNonce(0).
		SetDestinationAddress("").SetGasLimit(20000).Build()
	selp3, err := action.Sign(elp, a, priKeyA)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{selp1, selp2, selp3})
	require.Nil(t, err)
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{b + ":200"}))
}

func TestLoadStoreHeight(t *testing.T) {
	require := require.New(t)
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	statefactory, err := NewFactory(cfg, DefaultTrieOption())
	require.Nil(err)
	require.Nil(statefactory.Start(context.Background()))

	sf := statefactory.(*factory)

	require.Nil(sf.dao.Put(AccountKVNameSpace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(0)))
	height, err := sf.Height()
	require.NoError(err)
	require.Equal(uint64(0), height)

	require.Nil(sf.dao.Put(AccountKVNameSpace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(10)))
	height, err = sf.Height()
	require.NoError(err)
	require.Equal(uint64(10), height)
}

func TestLoadStoreHeightInMem(t *testing.T) {
	require := require.New(t)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	statefactory, err := NewFactory(cfg, InMemTrieOption())
	require.Nil(err)
	require.Nil(statefactory.Start(context.Background()))

	sf := statefactory.(*factory)

	require.Nil(sf.dao.Put(AccountKVNameSpace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(0)))
	height, err := sf.Height()
	require.NoError(err)
	require.Equal(uint64(0), height)

	require.Nil(sf.dao.Put(AccountKVNameSpace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(10)))
	height, err = sf.Height()
	require.NoError(err)
	require.Equal(uint64(10), height)
}

func TestFactory_RootHashByHeight(t *testing.T) {
	cfg := config.Default
	ctx := context.Background()
	sf, err := NewFactory(cfg, InMemTrieOption())
	require.NoError(t, err)
	require.NoError(t, sf.Start(ctx))
	defer require.NoError(t, sf.Stop(ctx))

	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	_, _, err = ws.RunActions(context.Background(), 1, nil)
	require.NoError(t, err)
	require.NoError(t, sf.Commit(ws))

	rootHash, err := sf.RootHashByHeight(1)
	require.NoError(t, err)
	require.NotEqual(t, hash.ZeroHash32B, rootHash)
}

func compareStrings(actual []string, expected []string) bool {
	act := make(map[string]bool)
	for i := 0; i < len(actual); i++ {
		act[actual[i]] = true
	}

	for i := 0; i < len(expected); i++ {
		if _, ok := act[expected[i]]; ok {
			delete(act, expected[i])
		} else {
			return false
		}
	}
	return len(act) == 0
}

func BenchmarkInMemRunAction(b *testing.B) {
	benchRunAction(db.NewMemKVStore(), b)
}

func BenchmarkDBRunAction(b *testing.B) {
	tp := filepath.Join(os.TempDir(), testTriePath)
	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}

	cfg := config.Default
	cfg.DB.DbPath = tp

	benchRunAction(db.NewOnDiskDB(cfg.DB), b)

	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}
}

func benchRunAction(db db.KVStore, b *testing.B) {
	// set up
	accounts := []string{
		testaddress.Addrinfo["alfa"].Bech32(),
		testaddress.Addrinfo["bravo"].Bech32(),
		testaddress.Addrinfo["charlie"].Bech32(),
		testaddress.Addrinfo["delta"].Bech32(),
		testaddress.Addrinfo["echo"].Bech32(),
		testaddress.Addrinfo["foxtrot"].Bech32(),
	}
	pubKeys := []keypair.PublicKey{
		testaddress.Keyinfo["alfa"].PubKey,
		testaddress.Keyinfo["bravo"].PubKey,
		testaddress.Keyinfo["charlie"].PubKey,
		testaddress.Keyinfo["delta"].PubKey,
		testaddress.Keyinfo["echo"].PubKey,
		testaddress.Keyinfo["foxtrot"].PubKey,
	}
	nonces := make([]uint64, len(accounts))

	cfg := config.Default
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db))
	if err != nil {
		b.Fatal(err)
	}
	sf.AddActionHandlers(account.NewProtocol())
	if err := sf.Start(context.Background()); err != nil {
		b.Fatal(err)
	}
	ws, err := sf.NewWorkingSet()
	if err != nil {
		b.Fatal(err)
	}
	for _, acc := range accounts {
		_, err = account.LoadOrCreateAccount(ws, acc, big.NewInt(int64(b.N*100)))
		if err != nil {
			b.Fatal(err)
		}
	}
	if err := sf.Commit(ws); err != nil {
		b.Fatal(err)
	}
	gasLimit := testutil.TestGasLimit

	for n := 0; n < b.N; n++ {
		ws, err := sf.NewWorkingSet()
		if err != nil {
			b.Fatal(err)
		}

		// put 500 actions together to run
		b.StopTimer()
		total := 500
		acts := make([]action.SealedEnvelope, 0, total)
		for numActs := 0; numActs < total; numActs++ {
			senderIdx := rand.Int() % len(accounts)

			var chainIDBytes [4]byte
			enc.MachineEndian.PutUint32(chainIDBytes[:], 1)
			payload := []byte(randStringRunes(20))
			receiverAddr, err := address.BytesToAddress(payload)
			if err != nil {
				b.Fatal(err)
			}
			receiver := receiverAddr.Bech32()
			nonces[senderIdx] += nonces[senderIdx]
			tx, err := action.NewTransfer(nonces[senderIdx], big.NewInt(1), accounts[senderIdx], receiver, nil, uint64(0), big.NewInt(0))
			if err != nil {
				b.Fatal(err)
			}
			bd := &action.EnvelopeBuilder{}
			elp := bd.SetNonce(nonces[senderIdx]).SetDestinationAddress(receiver).SetAction(tx).Build()
			selp := action.FakeSeal(elp, accounts[senderIdx], pubKeys[senderIdx])
			acts = append(acts, selp)
		}
		b.StartTimer()
		zctx := protocol.WithRunActionsCtx(context.Background(),
			protocol.RunActionsCtx{
				Producer:        testaddress.Addrinfo["producer"],
				GasLimit:        &gasLimit,
				EnableGasCharge: false,
			})
		_, _, err = ws.RunActions(zctx, uint64(n), acts)
		if err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		if err := sf.Commit(ws); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
	}
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
