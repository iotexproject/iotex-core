// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package statefactory

import (
	"encoding/hex"
	"math/big"
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/test/mock/mock_trie"
	"github.com/iotexproject/iotex-core/test/util"
	"github.com/iotexproject/iotex-core/trie"
)

var chainid = []byte{0x00, 0x00, 0x00, 0x01}

const (
	isTestnet    = true
	testTriePath = "trie.test"
)

func TestEncodeDecode(t *testing.T) {
	addr, err := iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	assert.Nil(t, err)

	ss, _ := stateToBytes(&State{Address: addr.RawAddress, Nonce: 0x10})
	assert.NotEmpty(t, ss)

	state, _ := bytesToState(ss)
	assert.Equal(t, addr.RawAddress, state.Address)
	assert.Equal(t, uint64(0x10), state.Nonce)
}

func TestRootHash(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	trie := mock_trie.NewMockTrie(ctrl)
	sf := NewStateFactory(trie)
	trie.EXPECT().RootHash().Times(1).Return(common.ZeroHash32B)
	assert.Equal(t, common.ZeroHash32B, sf.RootHash())
}

func TestCreateState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	trie := mock_trie.NewMockTrie(ctrl)
	sf := NewStateFactory(trie)
	trie.EXPECT().Upsert(gomock.Any(), gomock.Any()).Times(1)
	addr, err := iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	assert.Nil(t, err)
	state, _ := sf.CreateState(addr.RawAddress, 0)
	assert.Equal(t, uint64(0x0), state.Nonce)
	assert.Equal(t, big.NewInt(0), state.Balance)
	assert.Equal(t, addr.RawAddress, state.Address)
}

func TestBalance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	trie := mock_trie.NewMockTrie(ctrl)
	addr, err := iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	assert.Nil(t, err)
	state := &State{Address: addr.RawAddress, Balance: big.NewInt(20)}
	mstate, _ := stateToBytes(state)
	trie.EXPECT().Get(gomock.Any()).Times(0).Return(mstate, nil)
	// Add 10 to the balance
	err = state.AddBalance(big.NewInt(10))
	assert.Nil(t, err)
	// balance should == 30 now
	assert.Equal(t, 0, state.Balance.Cmp(big.NewInt(30)))
}

func TestNonce(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	trie := mock_trie.NewMockTrie(ctrl)
	sf := NewStateFactory(trie)

	// Add 10 so the balance should be 10
	addr, err := iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	assert.Nil(t, err)
	mstate, _ := stateToBytes(&State{Address: addr.RawAddress, Nonce: 0x10})
	trie.EXPECT().Get(gomock.Any()).Times(1).Return(mstate, nil)
	addr, err = iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	assert.Nil(t, err)
	n, err := sf.Nonce(addr.RawAddress)
	assert.Equal(t, uint64(0x10), n)
	assert.Nil(t, err)

	trie.EXPECT().Get(gomock.Any()).Times(1).Return(nil, nil)
	_, err = sf.Nonce(addr.RawAddress)
	assert.Equal(t, ErrFailedToUnmarshalState, err)

	trie.EXPECT().Upsert(gomock.Any(), gomock.Any()).Times(1).Do(func(key, value []byte) error {
		state, _ := bytesToState(value)
		assert.Equal(t, uint64(0x11), state.Nonce)
		return nil
	})
	mstate, _ = stateToBytes(&State{Address: addr.RawAddress, Nonce: 0x10})
	trie.EXPECT().Get(gomock.Any()).Times(1).Return(mstate, nil)
	err = sf.SetNonce(addr.RawAddress, uint64(0x11))
}

func voteForm(cs []*Candidate) []string {
	r := make([]string, len(cs))
	for i := 0; i < len(cs); i++ {
		r[i] = (*cs[i]).Address + ":" + strconv.FormatInt((*cs[i]).Votes.Int64(), 10)
	}
	return r
}

func TestCandidatePool(t *testing.T) {
	c1 := &Candidate{Address: "a1", Votes: big.NewInt(1), PubKey: []byte("p1")}
	c2 := &Candidate{Address: "a2", Votes: big.NewInt(2), PubKey: []byte("p2")}
	c3 := &Candidate{Address: "a3", Votes: big.NewInt(3), PubKey: []byte("p3")}
	c4 := &Candidate{Address: "a4", Votes: big.NewInt(4), PubKey: []byte("p4")}
	c5 := &Candidate{Address: "a5", Votes: big.NewInt(5), PubKey: []byte("p5")}
	c6 := &Candidate{Address: "a6", Votes: big.NewInt(6), PubKey: []byte("p6")}
	c7 := &Candidate{Address: "a7", Votes: big.NewInt(7), PubKey: []byte("p7")}
	c8 := &Candidate{Address: "a8", Votes: big.NewInt(8), PubKey: []byte("p8")}
	c9 := &Candidate{Address: "a9", Votes: big.NewInt(9), PubKey: []byte("p9")}
	c10 := &Candidate{Address: "a10", Votes: big.NewInt(10), PubKey: []byte("p10")}
	c11 := &Candidate{Address: "a11", Votes: big.NewInt(11), PubKey: []byte("p11")}
	c12 := &Candidate{Address: "a12", Votes: big.NewInt(12), PubKey: []byte("p12")}
	tr, _ := trie.NewTrie("trie.test", false)
	sf := &stateFactory{
		trie:                   tr,
		candidateHeap:          CandidateMinPQ{candidateSize, make([]*Candidate, 0)},
		candidateBufferMinHeap: CandidateMinPQ{candidateBufferSize, make([]*Candidate, 0)},
		candidateBufferMaxHeap: CandidateMaxPQ{candidateBufferSize, make([]*Candidate, 0)},
	}

	sf.updateVotes(c1, big.NewInt(1))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a1:1"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{}))

	sf.updateVotes(c1, big.NewInt(2))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a1:2"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{}))

	sf.updateVotes(c2, big.NewInt(2))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a1:2", "a2:2"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{}))

	sf.updateVotes(c3, big.NewInt(3))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a2:2", "a3:3"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{"a1:2"}))

	sf.updateVotes(c4, big.NewInt(4))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a3:3", "a4:4"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{"a1:2", "a2:2"}))

	sf.updateVotes(c2, big.NewInt(1))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a3:3", "a4:4"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{"a1:2", "a2:1"}))

	sf.updateVotes(c5, big.NewInt(5))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a4:4", "a5:5"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{"a1:2", "a2:1", "a3:3"}))

	sf.updateVotes(c2, big.NewInt(9))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a2:9", "a5:5"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{"a1:2", "a3:3", "a4:4"}))

	sf.updateVotes(c6, big.NewInt(6))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a2:9", "a6:6"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{"a3:3", "a4:4", "a5:5"}))

	sf.updateVotes(c1, big.NewInt(10))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a1:10", "a2:9"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{"a4:4", "a5:5", "a6:6"}))

	sf.updateVotes(c7, big.NewInt(7))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a1:10", "a2:9"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{"a5:5", "a6:6", "a7:7"}))

	sf.updateVotes(c3, big.NewInt(8))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a1:10", "a2:9"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{"a3:8", "a6:6", "a7:7"}))

	sf.updateVotes(c8, big.NewInt(12))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a1:10", "a8:12"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{"a2:9", "a3:8", "a7:7"}))

	sf.updateVotes(c4, big.NewInt(8))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a1:10", "a8:12"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{"a2:9", "a3:8", "a4:8"}))

	sf.updateVotes(c6, big.NewInt(7))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a1:10", "a8:12"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{"a2:9", "a3:8", "a4:8"}))

	sf.updateVotes(c1, big.NewInt(1))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a2:9", "a8:12"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{"a3:8", "a4:8", "a1:1"}))

	sf.updateVotes(c9, big.NewInt(2))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a2:9", "a8:12"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{"a3:8", "a4:8", "a9:2"}))

	sf.updateVotes(c10, big.NewInt(8))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a2:9", "a8:12"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{"a10:8", "a3:8", "a4:8"}))

	sf.updateVotes(c11, big.NewInt(3))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a2:9", "a8:12"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{"a10:8", "a3:8", "a4:8"}))

	sf.updateVotes(c12, big.NewInt(1))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{"a2:9", "a8:12"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{"a10:8", "a3:8", "a4:8"}))
}

func decodeHash(in string) []byte {
	hash, _ := hex.DecodeString(in)
	return hash
}

func TestCandidate(t *testing.T) {
	// Create three dummy iotex addresses
	a, _ := iotxaddress.NewAddress(isTestnet, chainid)
	b, _ := iotxaddress.NewAddress(isTestnet, chainid)
	c, _ := iotxaddress.NewAddress(isTestnet, chainid)
	util.CleanupPath(t, testTriePath)
	defer util.CleanupPath(t, testTriePath)
	tr, _ := trie.NewTrie(testTriePath, false)
	sf := &stateFactory{
		trie:                   tr,
		candidateHeap:          CandidateMinPQ{candidateSize, make([]*Candidate, 0)},
		candidateBufferMinHeap: CandidateMinPQ{candidateBufferSize, make([]*Candidate, 0)},
		candidateBufferMaxHeap: CandidateMaxPQ{candidateBufferSize, make([]*Candidate, 0)},
	}
	sf.CreateState(a.RawAddress, uint64(100))
	sf.CreateState(b.RawAddress, uint64(200))
	sf.CreateState(c.RawAddress, uint64(300))

	// a:100 b:200 c:300
	tx1 := action.Transfer{Sender: a.RawAddress, Recipient: b.RawAddress, Nonce: uint64(1), Amount: big.NewInt(10)}
	tx2 := action.Transfer{Sender: a.RawAddress, Recipient: c.RawAddress, Nonce: uint64(2), Amount: big.NewInt(20)}
	vote := action.NewVote(0, a.PublicKey, a.PublicKey)
	err := sf.CommitStateChanges([]*action.Transfer{&tx1, &tx2}, []*action.Vote{vote})
	assert.Nil(t, err)
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":70"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{}))
	// a(a):70 b:210 c:320

	tx3 := action.Transfer{Sender: a.RawAddress, Recipient: c.RawAddress, Nonce: uint64(2), Amount: big.NewInt(20)}
	err = sf.CommitStateChanges([]*action.Transfer{&tx3}, nil)
	assert.Nil(t, err)
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":50"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{}))
	// a(a):50 b:210 c:340

	tx4 := action.Transfer{Sender: c.RawAddress, Recipient: a.RawAddress, Nonce: uint64(2), Amount: big.NewInt(100)}
	err = sf.CommitStateChanges([]*action.Transfer{&tx4}, nil)
	assert.Nil(t, err)
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":150"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{}))
	// a(a):150 b:210 c:240

	vote2 := action.NewVote(0, a.PublicKey, c.PublicKey)
	err = sf.CommitStateChanges(nil, []*action.Vote{vote2})
	assert.Nil(t, err)
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":0", c.RawAddress + ":390"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{}))
	// a(c):150(0) b:210 c:240(390)

	vote3 := action.NewVote(0, a.PublicKey, b.PublicKey)
	err = sf.CommitStateChanges(nil, []*action.Vote{vote3})
	assert.Nil(t, err)
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{b.RawAddress + ":360", c.RawAddress + ":240"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{a.RawAddress + ":0"}))
	// a(b):150(0) b:210(360) c:240(240)

	vote4 := action.NewVote(0, b.PublicKey, a.PublicKey)
	err = sf.CommitStateChanges(nil, []*action.Vote{vote4})
	assert.Nil(t, err)
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":210", c.RawAddress + ":240"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{b.RawAddress + ":150"}))
	// a(b):150(210) b(a):210(150) c:240(240)

	vote5 := action.NewVote(0, c.PublicKey, b.PublicKey)
	err = sf.CommitStateChanges(nil, []*action.Vote{vote5})
	assert.Nil(t, err)
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":210", b.RawAddress + ":390"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{c.RawAddress + ":0"}))
	// a(b):150(210) b(a):210(390) c(b):240(0)

	vote6 := action.NewVote(0, a.PublicKey, a.PublicKey)
	err = sf.CommitStateChanges(nil, []*action.Vote{vote6})
	assert.Nil(t, err)
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":360", b.RawAddress + ":240"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{c.RawAddress + ":0"}))
	// a():150(360) b(a):210(240) c(b):240(0)

	tx5 := action.Transfer{Sender: b.RawAddress, Recipient: c.RawAddress, Nonce: uint64(2), Amount: big.NewInt(100)}
	err = sf.CommitStateChanges([]*action.Transfer{&tx5}, []*action.Vote{vote})
	assert.Nil(t, err)
	//fmt.Printf("delegate: %v candidate: %v buffer: %v", voteForm(sf.Delegates()), voteForm(sf.Candidates()), voteForm(sf.Buffer()))
	assert.True(t, compareStrings(voteForm(sf.Candidates()), []string{a.RawAddress + ":260", b.RawAddress + ":340"}))
	assert.True(t, compareStrings(voteForm(sf.CandidatesBuffer()), []string{c.RawAddress + ":0"}))
	// a():150(260) b(a):110(340) c(b):340(0)
}

func compareStrings(actual []string, expected []string) bool {
	act := make(map[string]bool, 0)
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
