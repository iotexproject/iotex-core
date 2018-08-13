// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"context"
	"math/big"
	"sort"
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/test/mock/mock_trie"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/iotexproject/iotex-core/trie"
)

var chainid = []byte{0x00, 0x00, 0x00, 0x01}

const (
	isTestnet    = true
	testTriePath = "trie.test"
)

func TestEncodeDecode(t *testing.T) {
	require := require.New(t)
	ss, _ := stateToBytes(&State{Nonce: 0x10})
	require.NotEmpty(ss)

	state, _ := bytesToState(ss)
	require.Nil(state.Balance)
	require.Equal(uint64(0x10), state.Nonce)
	require.Equal(hash.ZeroHash32B, state.Root)
	require.Nil(state.CodeHash)
}

func TestRootHash(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	accountTrie := mock_trie.NewMockTrie(ctrl)
	candidateTrie := mock_trie.NewMockTrie(ctrl)
	accountTrie.EXPECT().TrieDB().Times(1).Return(nil)
	sf, err := NewFactory(&config.Default, PrecreatedTrieOption(accountTrie, candidateTrie))
	require.Nil(err)
	accountTrie.EXPECT().RootHash().Times(1).Return(hash.ZeroHash32B)
	require.Equal(hash.ZeroHash32B, sf.RootHash())
}

func TestCreateState(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	accountTrie, err := trie.NewTrie(db.NewMemKVStore(), "account", trie.EmptyRoot)
	require.Nil(err)
	candidateTrie, err := trie.NewTrie(accountTrie.TrieDB(), "candidate", trie.EmptyRoot)
	sf, err := NewFactory(&config.Default, PrecreatedTrieOption(accountTrie, candidateTrie))
	require.Nil(err)
	require.Nil(sf.Start(context.Background()))
	addr, err := iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	require.Nil(err)
	state, _ := sf.LoadOrCreateState(addr.RawAddress, 5)
	require.Equal(uint64(0x0), state.Nonce)
	require.Equal(big.NewInt(5), state.Balance)
	ss, err := sf.State(addr.RawAddress)
	require.Nil(err)
	require.Equal(uint64(0x0), ss.Nonce)
	require.Equal(big.NewInt(5), ss.Balance)
}

func TestBalance(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	state := &State{Balance: big.NewInt(20)}
	// Add 10 to the balance
	err := state.AddBalance(big.NewInt(10))
	require.Nil(err)
	// balance should == 30 now
	require.Equal(0, state.Balance.Cmp(big.NewInt(30)))
}

func TestNonce(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	accountTrie := mock_trie.NewMockTrie(ctrl)
	candidateTrie := mock_trie.NewMockTrie(ctrl)
	accountTrie.EXPECT().TrieDB().Times(1).Return(nil)
	sf, err := NewFactory(&config.Default, PrecreatedTrieOption(accountTrie, candidateTrie))
	require.Nil(err)

	// Add 10 so the balance should be 10
	addr, err := iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	require.Nil(err)
	mstate, _ := stateToBytes(&State{Nonce: 0x10})
	accountTrie.EXPECT().Get(gomock.Any()).Times(1).Return(mstate, nil)
	addr, err = iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	require.Nil(err)
	n, err := sf.Nonce(addr.RawAddress)
	require.Equal(uint64(0x10), n)
	require.Nil(err)

	accountTrie.EXPECT().Get(gomock.Any()).Times(1).Return(nil, nil)
	_, err = sf.Nonce(addr.RawAddress)
	require.Equal(ErrFailedToUnmarshalState, err)
}

func voteForm(height uint64, cs []*Candidate) []string {
	r := make([]string, len(cs))
	for i := 0; i < len(cs); i++ {
		r[i] = (*cs[i]).Address + ":" + strconv.FormatInt((*cs[i]).Votes.Int64(), 10)
	}
	return r
}

// Test configure: candidateSize = 2, candidateBufferSize = 3
//func TestCandidatePool(t *testing.T) {
//	c1 := &Candidate{Address: "a1", Votes: big.NewInt(1), PubKey: []byte("p1")}
//	c2 := &Candidate{Address: "a2", Votes: big.NewInt(2), PubKey: []byte("p2")}
//	c3 := &Candidate{Address: "a3", Votes: big.NewInt(3), PubKey: []byte("p3")}
//	c4 := &Candidate{Address: "a4", Votes: big.NewInt(4), PubKey: []byte("p4")}
//	c5 := &Candidate{Address: "a5", Votes: big.NewInt(5), PubKey: []byte("p5")}
//	c6 := &Candidate{Address: "a6", Votes: big.NewInt(6), PubKey: []byte("p6")}
//	c7 := &Candidate{Address: "a7", Votes: big.NewInt(7), PubKey: []byte("p7")}
//	c8 := &Candidate{Address: "a8", Votes: big.NewInt(8), PubKey: []byte("p8")}
//	c9 := &Candidate{Address: "a9", Votes: big.NewInt(9), PubKey: []byte("p9")}
//	c10 := &Candidate{Address: "a10", Votes: big.NewInt(10), PubKey: []byte("p10")}
//	c11 := &Candidate{Address: "a11", Votes: big.NewInt(11), PubKey: []byte("p11")}
//	c12 := &Candidate{Address: "a12", Votes: big.NewInt(12), PubKey: []byte("p12")}
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
	a, _ := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
	b, _ := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
	c, _ := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
	d, _ := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
	e, _ := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
	f, _ := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	accountTr, _ := trie.NewTrie(db.NewBoltDB(testTriePath, nil), "account", trie.EmptyRoot)
	candidateTr, _ := trie.NewTrie(accountTr.TrieDB(), "candidate", trie.EmptyRoot)
	require.Nil(t, accountTr.Start(context.Background()))
	require.Nil(t, candidateTr.Start(context.Background()))
	sf := &factory{
		accountTrie:      accountTr,
		candidateTrie:    candidateTr,
		numCandidates:    uint(2),
		cachedCandidates: make(map[string]*Candidate),
		cachedAccount:    make(map[string]*State),
	}
	_, err := sf.LoadOrCreateState(a.RawAddress, uint64(100))
	require.NoError(t, err)
	_, err = sf.LoadOrCreateState(b.RawAddress, uint64(200))
	require.NoError(t, err)
	_, err = sf.LoadOrCreateState(c.RawAddress, uint64(300))
	require.NoError(t, err)
	_, err = sf.LoadOrCreateState(d.RawAddress, uint64(100))
	require.NoError(t, err)
	_, err = sf.LoadOrCreateState(e.RawAddress, uint64(100))
	require.NoError(t, err)
	_, err = sf.LoadOrCreateState(f.RawAddress, uint64(300))
	require.NoError(t, err)

	// a:100(0) b:200(0) c:300(0)
	tx1 := action.Transfer{Sender: a.RawAddress, Recipient: b.RawAddress, Nonce: uint64(1), Amount: big.NewInt(10)}
	tx2 := action.Transfer{Sender: a.RawAddress, Recipient: c.RawAddress, Nonce: uint64(2), Amount: big.NewInt(20)}
	err = sf.CommitStateChanges(0, []*action.Transfer{&tx1, &tx2}, []*action.Vote{})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{}))
	// a:70 b:210 c:320

	vote, err := action.NewVote(0, a.RawAddress, a.RawAddress)
	vote.GetVote().SelfPubkey = a.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":70"}))
	// a(a):70(+0=70) b:210 c:320

	vote2, err := action.NewVote(0, b.RawAddress, b.RawAddress)
	vote2.GetVote().SelfPubkey = b.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote2})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":70", b.RawAddress + ":210"}))
	// a(a):70(+0=70) b(b):210(+0=210) !c:320

	vote3, err := action.NewVote(1, a.RawAddress, b.RawAddress)
	vote3.GetVote().SelfPubkey = a.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote3})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":0", b.RawAddress + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	tx3 := action.Transfer{Sender: b.RawAddress, Recipient: a.RawAddress, Nonce: uint64(2), Amount: big.NewInt(20)}
	err = sf.CommitStateChanges(0, []*action.Transfer{&tx3}, []*action.Vote{})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":0", b.RawAddress + ":280"}))
	// a(b):90(0) b(b):190(+90=280) !c:320

	tx4 := action.Transfer{Sender: a.RawAddress, Recipient: b.RawAddress, Nonce: uint64(2), Amount: big.NewInt(20)}
	err = sf.CommitStateChanges(0, []*action.Transfer{&tx4}, []*action.Vote{})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":0", b.RawAddress + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	vote4, err := action.NewVote(1, b.RawAddress, a.RawAddress)
	vote4.GetVote().SelfPubkey = b.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote4})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":210", b.RawAddress + ":70"}))
	// a(b):70(210) b(a):210(70) !c:320

	vote5, err := action.NewVote(2, b.RawAddress, b.RawAddress)
	vote5.GetVote().SelfPubkey = b.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote5})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":0", b.RawAddress + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	vote6, err := action.NewVote(3, b.RawAddress, b.RawAddress)
	vote6.GetVote().SelfPubkey = b.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote6})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":0", b.RawAddress + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	tx5 := action.Transfer{Sender: c.RawAddress, Recipient: a.RawAddress, Nonce: uint64(2), Amount: big.NewInt(20)}
	err = sf.CommitStateChanges(0, []*action.Transfer{&tx5}, []*action.Vote{})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":0", b.RawAddress + ":300"}))
	// a(b):90(0) b(b):210(+90=300) !c:300

	vote7, err := action.NewVote(0, c.RawAddress, a.RawAddress)
	vote7.GetVote().SelfPubkey = c.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote7})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":300", b.RawAddress + ":300"}))
	// a(b):90(300) b(b):210(+90=300) !c(a):300

	vote8, err := action.NewVote(4, b.RawAddress, c.RawAddress)
	vote8.GetVote().SelfPubkey = b.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote8})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":300", b.RawAddress + ":90"}))
	// a(b):90(300) b(c):210(90) !c(a):300

	vote9, err := action.NewVote(1, c.RawAddress, c.RawAddress)
	vote9.GetVote().SelfPubkey = c.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote9})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{c.RawAddress + ":510", b.RawAddress + ":90"}))
	// a(b):90(0) b(c):210(90) c(c):300(+210=510)

	vote10, err := action.NewVote(0, d.RawAddress, e.RawAddress)
	vote10.GetVote().SelfPubkey = d.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote10})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{c.RawAddress + ":510", b.RawAddress + ":90"}))
	// a(b):90(0) b(c):210(90) c(c):300(+210=510)

	vote11, err := action.NewVote(1, d.RawAddress, d.RawAddress)
	vote11.GetVote().SelfPubkey = d.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote11})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{c.RawAddress + ":510", d.RawAddress + ":100"}))
	// a(b):90(0) b(c):210(90) c(c):300(+210=510) d(d): 100(100)

	vote12, err := action.NewVote(2, d.RawAddress, a.RawAddress)
	vote12.GetVote().SelfPubkey = d.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote12})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{c.RawAddress + ":510", a.RawAddress + ":100"}))
	// a(b):90(100) b(c):210(90) c(c):300(+210=510) d(a): 100(0)

	vote13, err := action.NewVote(2, c.RawAddress, d.RawAddress)
	vote13.GetVote().SelfPubkey = c.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote13})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{c.RawAddress + ":210", d.RawAddress + ":300"}))
	// a(b):90(100) b(c):210(90) c(d):300(210) d(a): 100(300)

	vote14, err := action.NewVote(3, c.RawAddress, c.RawAddress)
	vote14.GetVote().SelfPubkey = c.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote14})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{c.RawAddress + ":510", a.RawAddress + ":100"}))
	// a(b):90(100) b(c):210(90) c(c):300(+210=510) d(a): 100(0)

	tx6 := action.Transfer{Sender: c.RawAddress, Recipient: e.RawAddress, Nonce: uint64(1), Amount: big.NewInt(200)}
	tx7 := action.Transfer{Sender: b.RawAddress, Recipient: e.RawAddress, Nonce: uint64(2), Amount: big.NewInt(200)}
	err = sf.CommitStateChanges(0, []*action.Transfer{&tx6, &tx7}, []*action.Vote{})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{c.RawAddress + ":110", a.RawAddress + ":100"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(0) !e:500

	vote15, err := action.NewVote(0, e.RawAddress, e.RawAddress)
	vote15.GetVote().SelfPubkey = e.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote15})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{c.RawAddress + ":110", e.RawAddress + ":500"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(0) e(e):500(+0=500)

	vote16, err := action.NewVote(0, f.RawAddress, f.RawAddress)
	vote16.GetVote().SelfPubkey = f.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote16})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{f.RawAddress + ":300", e.RawAddress + ":500"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(0) e(e):500(+0=500) f(f):300(+0=300)

	vote17, err := action.NewVote(0, f.RawAddress, d.RawAddress)
	vote17.GetVote().SelfPubkey = f.PublicKey[:]
	require.NoError(t, err)
	vote18, err := action.NewVote(1, f.RawAddress, d.RawAddress)
	vote18.GetVote().SelfPubkey = f.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote17, vote18})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{d.RawAddress + ":300", e.RawAddress + ":500"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(300) e(e):500(+0=500) f(d):300(0)

	tx8 := action.Transfer{Sender: f.RawAddress, Recipient: b.RawAddress, Nonce: uint64(1), Amount: big.NewInt(200)}
	err = sf.CommitStateChanges(0, []*action.Transfer{&tx8}, []*action.Vote{})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{c.RawAddress + ":310", e.RawAddress + ":500"}))
	// a(b):90(100) b(c):210(90) c(c):100(+210=310) d(a): 100(100) e(e):500(+0=500) f(d):100(0)
	//fmt.Printf("%v \n", voteForm(sf.candidatesBuffer()))

	tx9 := action.Transfer{Sender: b.RawAddress, Recipient: a.RawAddress, Nonce: uint64(1), Amount: big.NewInt(10)}
	err = sf.CommitStateChanges(0, []*action.Transfer{&tx9}, []*action.Vote{})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{c.RawAddress + ":300", e.RawAddress + ":500"}))
	// a(b):100(100) b(c):200(100) c(c):100(+200=300) d(a): 100(100) e(e):500(+0=500) f(d):100(0)

	tx10 := action.Transfer{Sender: e.RawAddress, Recipient: d.RawAddress, Nonce: uint64(1), Amount: big.NewInt(300)}
	err = sf.CommitStateChanges(1, []*action.Transfer{&tx10}, []*action.Vote{})
	require.Nil(t, err)
	height, _ := sf.Candidates()
	require.True(t, height == 1)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{c.RawAddress + ":300", a.RawAddress + ":400"}))
	// a(b):100(400) b(c):200(100) c(c):100(+200=300) d(a): 400(100) e(e):200(+0=200) f(d):100(0)

	vote19, err := action.NewVote(0, d.RawAddress, a.RawAddress)
	vote19.GetVote().SelfPubkey = d.PublicKey[:]
	require.NoError(t, err)
	vote20, err := action.NewVote(3, d.RawAddress, b.RawAddress)
	vote20.GetVote().SelfPubkey = d.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(2, []*action.Transfer{}, []*action.Vote{vote19, vote20})
	require.Nil(t, err)
	height, _ = sf.Candidates()
	require.True(t, height == 2)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{c.RawAddress + ":300", b.RawAddress + ":500"}))
	// a(b):100(0) b(c):200(500) c(c):100(+200=300) d(b): 400(100) e(e):200(+0=200) f(d):100(0)

	vote21, err := action.NewVote(4, c.RawAddress, "")
	vote21.GetVote().SelfPubkey = c.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(3, []*action.Transfer{}, []*action.Vote{vote21})
	require.Nil(t, err)
	height, _ = sf.Candidates()
	require.True(t, height == 3)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{e.RawAddress + ":200", b.RawAddress + ":500"}))
	// a(b):100(0) b(c):200(500) [c(c):100(+200=300)] d(b): 400(100) e(e):200(+0=200) f(d):100(0)

	vote22, err := action.NewVote(4, f.RawAddress, "")
	vote22.GetVote().SelfPubkey = f.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(3, []*action.Transfer{}, []*action.Vote{vote22})
	require.Nil(t, err)
	height, _ = sf.Candidates()
	require.True(t, height == 3)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{e.RawAddress + ":200", b.RawAddress + ":500"}))
	// a(b):100(0) b(c):200(500) [c(c):100(+200=300)] d(b): 400(100) e(e):200(+0=200) f(d):100(0)
}

func TestCandidatesByHeight(t *testing.T) {
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	accountTr, _ := trie.NewTrie(db.NewBoltDB(testTriePath, nil), "account", trie.EmptyRoot)
	candidateTr, _ := trie.NewTrie(accountTr.TrieDB(), "candidate", trie.EmptyRoot)
	require.Nil(t, accountTr.Start(context.Background()))
	require.Nil(t, candidateTr.Start(context.Background()))
	sf := &factory{
		accountTrie:      accountTr,
		candidateTrie:    candidateTr,
		numCandidates:    uint(2),
		cachedCandidates: make(map[string]*Candidate),
		cachedAccount:    make(map[string]*State),
	}

	cand1 := &Candidate{
		Address: "Alpha",
		Votes:   big.NewInt(1),
	}
	cand2 := &Candidate{
		Address: "Beta",
		Votes:   big.NewInt(2),
	}
	cand3 := &Candidate{
		Address: "Theta",
		Votes:   big.NewInt(3),
	}
	candidateList := make(CandidateList, 0, 3)
	candidateList = append(candidateList, cand1)
	candidateList = append(candidateList, cand2)
	sort.Sort(candidateList)

	candidatesBytes, err := Serialize(candidateList)
	require.NoError(t, err)

	sf.candidateTrie.Upsert(byteutil.Uint64ToBytes(0), candidatesBytes)
	candidates, err := sf.CandidatesByHeight(0)
	require.NoError(t, err)
	require.Equal(t, 2, len(candidates))
	require.Equal(t, "Alpha", candidates[0].Address)
	require.Equal(t, "Beta", candidates[1].Address)

	candidateList = append(candidateList, cand3)
	sort.Sort(candidateList)
	candidatesBytes, err = Serialize(candidateList)
	require.NoError(t, err)

	sf.candidateTrie.Upsert(byteutil.Uint64ToBytes(1), candidatesBytes)
	candidates, err = sf.CandidatesByHeight(1)
	require.NoError(t, err)
	require.Equal(t, 2, len(candidates))
	require.Equal(t, "Beta", candidates[0].Address)
	require.Equal(t, "Theta", candidates[1].Address)
}

func TestUnvote(t *testing.T) {
	// Create three dummy iotex addresses
	a, _ := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
	b, _ := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	accountTr, _ := trie.NewTrie(db.NewBoltDB(testTriePath, nil), "account", trie.EmptyRoot)
	candidateTr, _ := trie.NewTrie(accountTr.TrieDB(), "candidate", trie.EmptyRoot)
	require.Nil(t, accountTr.Start(context.Background()))
	require.Nil(t, candidateTr.Start(context.Background()))
	sf := &factory{
		accountTrie:      accountTr,
		candidateTrie:    candidateTr,
		numCandidates:    uint(2),
		cachedCandidates: make(map[string]*Candidate),
		cachedAccount:    make(map[string]*State),
	}
	_, err := sf.LoadOrCreateState(a.RawAddress, uint64(100))
	require.NoError(t, err)
	_, err = sf.LoadOrCreateState(b.RawAddress, uint64(200))
	require.NoError(t, err)

	vote1, err := action.NewVote(0, a.RawAddress, "")
	vote1.GetVote().SelfPubkey = a.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote1})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{}))

	vote2, err := action.NewVote(0, a.RawAddress, a.RawAddress)
	vote2.GetVote().SelfPubkey = a.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote2})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":100"}))

	vote3, err := action.NewVote(0, a.RawAddress, "")
	vote3.GetVote().SelfPubkey = a.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote3})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{}))

	vote4, err := action.NewVote(0, b.RawAddress, b.RawAddress)
	vote4.GetVote().SelfPubkey = b.PublicKey[:]
	require.NoError(t, err)
	vote5, err := action.NewVote(0, a.RawAddress, b.RawAddress)
	vote5.GetVote().SelfPubkey = a.PublicKey[:]
	require.NoError(t, err)
	vote6, err := action.NewVote(0, a.RawAddress, "")
	vote6.GetVote().SelfPubkey = a.PublicKey[:]
	require.NoError(t, err)
	err = sf.CommitStateChanges(0, []*action.Transfer{}, []*action.Vote{vote4, vote5, vote6})
	require.Nil(t, err)
	require.True(t, compareStrings(voteForm(sf.Candidates()), []string{b.RawAddress + ":200"}))
}

func TestCreateContract(t *testing.T) {
	require := require.New(t)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	sf, err := NewFactory(&cfg, DefaultTrieOption())
	require.Nil(err)
	require.NoError(sf.Start(context.Background()))
	addr, err := iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	require.Nil(err)
	_, err = sf.LoadOrCreateState(addr.RawAddress, 5)
	require.Nil(err)

	code := []byte("test contract creation")
	contract, err := sf.CreateContract(addr.RawAddress, code)
	require.Nil(err)
	require.NotEqual("", contract)
	// contract exist
	codeHash := sf.GetCodeHash(contract)
	require.NotEqual(hash.ZeroHash32B, codeHash)
	code1 := sf.GetCode(contract)
	require.Equal(code, code1)
	// re-create cause collision
	contract, err = sf.CreateContract(addr.RawAddress, code)
	require.Equal(ErrAccountCollision, errors.Cause(err))
	require.Equal("", contract)
	// non-existing contract
	addr1, _ := iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	codeHash = sf.GetCodeHash(addr1.RawAddress)
	require.Equal(hash.ZeroHash32B, codeHash)
	code1 = sf.GetCode(addr1.RawAddress)
	require.Equal([]byte(nil), code1)
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
