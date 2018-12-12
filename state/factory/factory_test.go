// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"math/big"
	"strconv"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

var cfg = config.Default

const testTriePath = "trie.test"

func voteForm(height uint64, cs []*state.Candidate) []string {
	r := make([]string, len(cs))
	for i := 0; i < len(cs); i++ {
		r[i] = (*cs[i]).Address + ":" + strconv.FormatInt((*cs[i]).Votes.Int64(), 10)
	}
	return r
}

func TestSnapshot(t *testing.T) {
	require := require.New(t)
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	cfg.DB.DbPath = testTriePath
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewOnDiskDB(cfg.DB)))
	require.NoError(err)
	require.NoError(sf.Start(context.Background()))
	addr, err := iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	require.Nil(err)
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	_, err = account.LoadOrCreateAccount(ws, addr.RawAddress, big.NewInt(5))
	require.NoError(err)
	sHash, err := iotxaddress.AddressToPKHash(addr.RawAddress)
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
	addr, err = iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	require.Nil(err)
	_, err = account.LoadOrCreateAccount(ws, addr.RawAddress, big.NewInt(7))
	require.NoError(err)
	tHash, err := iotxaddress.AddressToPKHash(addr.RawAddress)
	require.NoError(err)
	s, err = account.LoadAccount(ws, tHash)
	require.NoError(err)
	require.Equal(big.NewInt(7), s.Balance)
	s2 := ws.Snapshot()
	require.Equal(2, s2)

	require.NoError(ws.Revert(s1))
	require.NoError(ws.State(sHash, s))
	require.Equal(big.NewInt(10), s.Balance)
	require.Equal(state.ErrStateNotExist, errors.Cause(ws.State(tHash, s)))
	require.NoError(ws.Revert(s0))
	require.NoError(ws.State(sHash, s))
	require.Equal(big.NewInt(5), s.Balance)
	require.Equal(state.ErrStateNotExist, errors.Cause(ws.State(tHash, s)))
	require.NoError(ws.Revert(s2))
	require.NoError(ws.State(sHash, s))
	require.Equal(big.NewInt(15), s.Balance)
	require.NoError(ws.State(tHash, s))
	require.Equal(big.NewInt(7), s.Balance)
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
	a := testaddress.Addrinfo["alfa"]
	b := testaddress.Addrinfo["bravo"]
	c := testaddress.Addrinfo["charlie"]
	d := testaddress.Addrinfo["delta"]
	e := testaddress.Addrinfo["echo"]
	f := testaddress.Addrinfo["foxtrot"]
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	cfg.Chain.NumCandidates = 2
	cfg.DB.DbPath = testTriePath
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewOnDiskDB(cfg.DB)))
	require.NoError(t, err)
	sf.AddActionHandlers(account.NewProtocol(), vote.NewProtocol(nil))
	require.NoError(t, sf.Start(context.Background()))
	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	_, err = account.LoadOrCreateAccount(ws, a.RawAddress, big.NewInt(100))
	require.NoError(t, err)
	_, err = account.LoadOrCreateAccount(ws, b.RawAddress, big.NewInt(200))
	require.NoError(t, err)
	_, err = account.LoadOrCreateAccount(ws, c.RawAddress, big.NewInt(300))
	require.NoError(t, err)
	_, err = account.LoadOrCreateAccount(ws, d.RawAddress, big.NewInt(100))
	require.NoError(t, err)
	_, err = account.LoadOrCreateAccount(ws, e.RawAddress, big.NewInt(100))
	require.NoError(t, err)
	_, err = account.LoadOrCreateAccount(ws, f.RawAddress, big.NewInt(300))
	require.NoError(t, err)

	// a:100(0) b:200(0) c:300(0)
	tx1, err := action.NewTransfer(uint64(1), big.NewInt(10), a.RawAddress, b.RawAddress, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)
	tx2, err := action.NewTransfer(uint64(2), big.NewInt(20), a.RawAddress, c.RawAddress, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			ProducerAddr:    testaddress.Addrinfo["producer"].RawAddress,
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	newRoot, _, err := ws.RunActions(ctx, 0, []action.Action{tx1, tx2})
	require.Nil(t, err)
	root := newRoot
	require.NotEqual(t, hash.ZeroHash32B, root)
	require.Nil(t, sf.Commit(ws))
	balanceB, err := sf.Balance(b.RawAddress)
	require.Nil(t, err)
	require.Equal(t, balanceB, big.NewInt(210))
	balanceC, err := sf.Balance(c.RawAddress)
	require.Nil(t, err)
	require.Equal(t, balanceC, big.NewInt(320))
	h, _ := sf.Height()
	cand, _ := sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{}))
	// a:70 b:210 c:320

	//vote, err := action.NewVote(0, a.RawAddress, a.RawAddress, uint64(100000), big.NewInt(10))
	vote, err := action.NewVote(0, a.RawAddress, a.RawAddress, uint64(100000), big.NewInt(0))
	vote.SetVoterPublicKey(a.PublicKey)
	require.NoError(t, err)
	zeroGasLimit := uint64(0)
	zctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			ProducerAddr:    testaddress.Addrinfo["producer"].RawAddress,
			GasLimit:        &zeroGasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(zctx, 0, []action.Action{vote})
	require.NotNil(t, err)
	newRoot, _, err = ws.RunActions(ctx, 0, []action.Action{vote})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a.RawAddress + ":70"}))
	// a(a):70(+0=70) b:210 c:320

	vote2, err := action.NewVote(0, b.RawAddress, b.RawAddress, uint64(100000), big.NewInt(0))
	vote2.SetVoterPublicKey(b.PublicKey)
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 1, []action.Action{vote2})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a.RawAddress + ":70", b.RawAddress + ":210"}))
	// a(a):70(+0=70) b(b):210(+0=210) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote3, err := action.NewVote(1, a.RawAddress, b.RawAddress, uint64(100000), big.NewInt(0))
	vote3.SetVoterPublicKey(a.PublicKey)
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 2, []action.Action{vote3})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a.RawAddress + ":0", b.RawAddress + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx3, err := action.NewTransfer(uint64(2), big.NewInt(20), b.RawAddress, a.RawAddress, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 3, []action.Action{tx3})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a.RawAddress + ":0", b.RawAddress + ":280"}))
	// a(b):90(0) b(b):190(+90=280) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx4, err := action.NewTransfer(uint64(2), big.NewInt(20), a.RawAddress, b.RawAddress, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 4, []action.Action{tx4})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a.RawAddress + ":0", b.RawAddress + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote4, err := action.NewVote(1, b.RawAddress, a.RawAddress, uint64(100000), big.NewInt(0))
	vote4.SetVoterPublicKey(b.PublicKey)
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 5, []action.Action{vote4})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a.RawAddress + ":210", b.RawAddress + ":70"}))
	// a(b):70(210) b(a):210(70) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote5, err := action.NewVote(2, b.RawAddress, b.RawAddress, uint64(100000), big.NewInt(0))
	vote5.SetVoterPublicKey(b.PublicKey)
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 6, []action.Action{vote5})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a.RawAddress + ":0", b.RawAddress + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote6, err := action.NewVote(3, b.RawAddress, b.RawAddress, uint64(100000), big.NewInt(0))
	vote6.SetVoterPublicKey(b.PublicKey)
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 7, []action.Action{vote6})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a.RawAddress + ":0", b.RawAddress + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx5, err := action.NewTransfer(uint64(2), big.NewInt(20), c.RawAddress, a.RawAddress, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 8, []action.Action{tx5})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a.RawAddress + ":0", b.RawAddress + ":300"}))
	// a(b):90(0) b(b):210(+90=300) !c:300

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote7, err := action.NewVote(0, c.RawAddress, a.RawAddress, uint64(100000), big.NewInt(0))
	vote7.SetVoterPublicKey(c.PublicKey)
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 9, []action.Action{vote7})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a.RawAddress + ":300", b.RawAddress + ":300"}))
	// a(b):90(300) b(b):210(+90=300) !c(a):300

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote8, err := action.NewVote(4, b.RawAddress, c.RawAddress, uint64(100000), big.NewInt(0))
	vote8.SetVoterPublicKey(b.PublicKey)
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 10, []action.Action{vote8})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a.RawAddress + ":300", b.RawAddress + ":90"}))
	// a(b):90(300) b(c):210(90) !c(a):300

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote9, err := action.NewVote(1, c.RawAddress, c.RawAddress, uint64(100000), big.NewInt(0))
	vote9.SetVoterPublicKey(c.PublicKey)
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 11, []action.Action{vote9})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c.RawAddress + ":510", b.RawAddress + ":90"}))
	// a(b):90(0) b(c):210(90) c(c):300(+210=510)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote10, err := action.NewVote(0, d.RawAddress, e.RawAddress, uint64(100000), big.NewInt(0))
	vote10.SetVoterPublicKey(d.PublicKey)
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 12, []action.Action{vote10})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c.RawAddress + ":510", b.RawAddress + ":90"}))
	// a(b):90(0) b(c):210(90) c(c):300(+210=510)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote11, err := action.NewVote(1, d.RawAddress, d.RawAddress, uint64(100000), big.NewInt(0))
	vote11.SetVoterPublicKey(d.PublicKey)
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 13, []action.Action{vote11})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c.RawAddress + ":510", d.RawAddress + ":100"}))
	// a(b):90(0) b(c):210(90) c(c):300(+210=510) d(d): 100(100)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote12, err := action.NewVote(2, d.RawAddress, a.RawAddress, uint64(100000), big.NewInt(0))
	vote12.SetVoterPublicKey(d.PublicKey)
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 14, []action.Action{vote12})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c.RawAddress + ":510", a.RawAddress + ":100"}))
	// a(b):90(100) b(c):210(90) c(c):300(+210=510) d(a): 100(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote13, err := action.NewVote(2, c.RawAddress, d.RawAddress, uint64(100000), big.NewInt(0))
	vote13.SetVoterPublicKey(c.PublicKey)
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 15, []action.Action{vote13})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c.RawAddress + ":210", d.RawAddress + ":300"}))
	// a(b):90(100) b(c):210(90) c(d):300(210) d(a): 100(300)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote14, err := action.NewVote(3, c.RawAddress, c.RawAddress, uint64(100000), big.NewInt(0))
	vote14.SetVoterPublicKey(c.PublicKey)
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 16, []action.Action{vote14})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c.RawAddress + ":510", a.RawAddress + ":100"}))
	// a(b):90(100) b(c):210(90) c(c):300(+210=510) d(a): 100(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx6, err := action.NewTransfer(uint64(1), big.NewInt(200), c.RawAddress, e.RawAddress, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)
	tx7, err := action.NewTransfer(uint64(2), big.NewInt(200), b.RawAddress, e.RawAddress, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 17, []action.Action{tx6, tx7})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c.RawAddress + ":110", a.RawAddress + ":100"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(0) !e:500

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote15, err := action.NewVote(0, e.RawAddress, e.RawAddress, uint64(100000), big.NewInt(0))
	vote15.SetVoterPublicKey(e.PublicKey)
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 18, []action.Action{vote15})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c.RawAddress + ":110", e.RawAddress + ":500"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(0) e(e):500(+0=500)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote16, err := action.NewVote(0, f.RawAddress, f.RawAddress, uint64(100000), big.NewInt(0))
	vote16.SetVoterPublicKey(f.PublicKey)
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 19, []action.Action{vote16})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{f.RawAddress + ":300", e.RawAddress + ":500"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(0) e(e):500(+0=500) f(f):300(+0=300)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote17, err := action.NewVote(0, f.RawAddress, d.RawAddress, uint64(100000), big.NewInt(0))
	vote17.SetVoterPublicKey(f.PublicKey)
	require.NoError(t, err)
	vote18, err := action.NewVote(1, f.RawAddress, d.RawAddress, uint64(100000), big.NewInt(0))
	vote18.SetVoterPublicKey(f.PublicKey)
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 20, []action.Action{vote17, vote18})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{d.RawAddress + ":300", e.RawAddress + ":500"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(300) e(e):500(+0=500) f(d):300(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx8, err := action.NewTransfer(uint64(1), big.NewInt(200), f.RawAddress, b.RawAddress, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 21, []action.Action{tx8})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c.RawAddress + ":310", e.RawAddress + ":500"}))
	// a(b):90(100) b(c):210(90) c(c):100(+210=310) d(a): 100(100) e(e):500(+0=500) f(d):100(0)
	//fmt.Printf("%v \n", voteForm(sf.candidatesBuffer()))

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx9, err := action.NewTransfer(uint64(1), big.NewInt(10), b.RawAddress, a.RawAddress, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 22, []action.Action{tx9})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c.RawAddress + ":300", e.RawAddress + ":500"}))
	// a(b):100(100) b(c):200(100) c(c):100(+200=300) d(a): 100(100) e(e):500(+0=500) f(d):100(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx10, err := action.NewTransfer(uint64(1), big.NewInt(300), e.RawAddress, d.RawAddress, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 23, []action.Action{tx10})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, err = sf.Height()
	require.Equal(t, uint64(23), h)
	require.NoError(t, err)
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c.RawAddress + ":300", a.RawAddress + ":400"}))
	// a(b):100(400) b(c):200(100) c(c):100(+200=300) d(a): 400(100) e(e):200(+0=200) f(d):100(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote19, err := action.NewVote(0, d.RawAddress, a.RawAddress, uint64(100000), big.NewInt(0))
	vote19.SetVoterPublicKey(d.PublicKey)
	require.NoError(t, err)
	vote20, err := action.NewVote(3, d.RawAddress, b.RawAddress, uint64(100000), big.NewInt(0))
	vote20.SetVoterPublicKey(d.PublicKey)
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 24, []action.Action{vote19, vote20})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, err = sf.Height()
	require.Equal(t, uint64(24), h)
	require.NoError(t, err)
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c.RawAddress + ":300", b.RawAddress + ":500"}))
	// a(b):100(0) b(c):200(500) c(c):100(+200=300) d(b): 400(100) e(e):200(+0=200) f(d):100(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote21, err := action.NewVote(4, c.RawAddress, "", uint64(100000), big.NewInt(0))
	vote21.SetVoterPublicKey(c.PublicKey)
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 25, []action.Action{vote21})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	root = newRoot
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	require.Equal(t, uint64(25), h)
	require.NoError(t, err)
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{e.RawAddress + ":200", b.RawAddress + ":500"}))
	// a(b):100(0) b(c):200(500) [c(c):100(+200=300)] d(b): 400(100) e(e):200(+0=200) f(d):100(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote22, err := action.NewVote(4, f.RawAddress, "", uint64(100000), big.NewInt(0))
	vote22.SetVoterPublicKey(f.PublicKey)
	require.NoError(t, err)
	newRoot, _, err = ws.RunActions(ctx, 26, []action.Action{vote22})
	require.Nil(t, err)
	require.NotEqual(t, newRoot, root)
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	require.Equal(t, uint64(26), h)
	require.NoError(t, err)
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{e.RawAddress + ":200", b.RawAddress + ":500"}))
	// a(b):100(0) b(c):200(500) [c(c):100(+200=300)] d(b): 400(100) e(e):200(+0=200) f(d):100(0)
	stateA, err := account.LoadOrCreateAccount(ws, a.RawAddress, big.NewInt(0))
	require.Nil(t, err)
	require.Equal(t, stateA.Balance, big.NewInt(100))
}

func TestUnvote(t *testing.T) {
	// Create three dummy iotex addresses
	a := testaddress.Addrinfo["alfa"]
	b := testaddress.Addrinfo["bravo"]

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

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
	_, err = account.LoadOrCreateAccount(ws, a.RawAddress, big.NewInt(100))
	require.NoError(t, err)
	_, err = account.LoadOrCreateAccount(ws, b.RawAddress, big.NewInt(200))
	require.NoError(t, err)

	vote1, err := action.NewVote(0, a.RawAddress, "", uint64(100000), big.NewInt(0))
	vote1.SetVoterPublicKey(a.PublicKey)
	require.NoError(t, err)
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			ProducerAddr:    testaddress.Addrinfo["producer"].RawAddress,
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 0, []action.Action{vote1})
	require.Nil(t, err)
	require.Nil(t, sf.Commit(ws))
	h, _ := sf.Height()
	cand, _ := sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{}))

	vote2, err := action.NewVote(0, a.RawAddress, a.RawAddress, uint64(100000), big.NewInt(0))
	vote2.SetVoterPublicKey(a.PublicKey)
	require.NoError(t, err)
	_, _, err = ws.RunActions(ctx, 0, []action.Action{vote2})
	require.Nil(t, err)
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a.RawAddress + ":100"}))

	vote3, err := action.NewVote(0, a.RawAddress, "", uint64(100000), big.NewInt(0))
	vote3.SetVoterPublicKey(a.PublicKey)
	require.NoError(t, err)
	_, _, err = ws.RunActions(ctx, 0, []action.Action{vote3})
	require.Nil(t, err)
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{}))

	vote4, err := action.NewVote(0, b.RawAddress, b.RawAddress, uint64(100000), big.NewInt(0))
	vote4.SetVoterPublicKey(b.PublicKey)
	require.NoError(t, err)
	vote5, err := action.NewVote(0, a.RawAddress, b.RawAddress, uint64(100000), big.NewInt(0))
	vote5.SetVoterPublicKey(a.PublicKey)
	require.NoError(t, err)
	vote6, err := action.NewVote(0, a.RawAddress, "", uint64(100000), big.NewInt(0))
	vote6.SetVoterPublicKey(a.PublicKey)
	require.NoError(t, err)
	_, _, err = ws.RunActions(ctx, 0, []action.Action{vote4, vote5, vote6})
	require.Nil(t, err)
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{b.RawAddress + ":200"}))
}

func TestLoadStoreHeight(t *testing.T) {
	require := require.New(t)

	cfg.Chain.TrieDBPath = testTriePath

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
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
