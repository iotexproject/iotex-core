// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package statefactory

import (
	"math/big"
	"reflect"
	"sort"
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/test/mock/mock_trie"
	"github.com/iotexproject/iotex-core/trie"
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
	sort.Strings(r)
	return r
}

func TestDelegates(t *testing.T) {
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
		trie:             tr,
		delegateHeap:     CandidateMinPQ{delegateSize, make([]*Candidate, 0)},
		candidateMinHeap: CandidateMinPQ{candidateSize, make([]*Candidate, 0)},
		candidateMaxHeap: CandidateMaxPQ{candidateSize, make([]*Candidate, 0)},
		minBuffer:        CandidateMinPQ{bufferSize, make([]*Candidate, 0)},
		maxBuffer:        CandidateMaxPQ{bufferSize, make([]*Candidate, 0)}}

	sf.updateVotes(c1, big.NewInt(1))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a1:1"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{}))

	sf.updateVotes(c1, big.NewInt(2))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a1:2"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{}))

	sf.updateVotes(c2, big.NewInt(2))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a1:2", "a2:2"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{}))

	sf.updateVotes(c3, big.NewInt(3))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a2:2", "a3:3"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{"a1:2"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{}))

	sf.updateVotes(c4, big.NewInt(4))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a3:3", "a4:4"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{"a1:2", "a2:2"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{}))

	sf.updateVotes(c2, big.NewInt(1))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a3:3", "a4:4"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{"a1:2", "a2:1"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{}))

	sf.updateVotes(c5, big.NewInt(5))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a4:4", "a5:5"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{"a1:2", "a2:1", "a3:3"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{}))

	sf.updateVotes(c2, big.NewInt(9))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a2:9", "a5:5"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{"a1:2", "a3:3", "a4:4"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{}))

	sf.updateVotes(c6, big.NewInt(6))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a2:9", "a6:6"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{"a3:3", "a4:4", "a5:5"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{"a1:2"}))

	sf.updateVotes(c1, big.NewInt(10))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a1:10", "a2:9"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{"a4:4", "a5:5", "a6:6"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{"a3:3"}))

	sf.updateVotes(c7, big.NewInt(7))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a1:10", "a2:9"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{"a5:5", "a6:6", "a7:7"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{"a3:3", "a4:4"}))

	sf.updateVotes(c3, big.NewInt(8))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a1:10", "a2:9"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{"a3:8", "a6:6", "a7:7"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{"a4:4", "a5:5"}))

	sf.updateVotes(c8, big.NewInt(12))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a1:10", "a8:12"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{"a2:9", "a3:8", "a7:7"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{"a4:4", "a5:5", "a6:6"}))

	sf.updateVotes(c4, big.NewInt(8))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a1:10", "a8:12"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{"a2:9", "a3:8", "a4:8"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{"a5:5", "a6:6", "a7:7"}))

	sf.updateVotes(c6, big.NewInt(7))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a1:10", "a8:12"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{"a2:9", "a3:8", "a4:8"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{"a5:5", "a6:7", "a7:7"}))

	sf.updateVotes(c1, big.NewInt(1))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a2:9", "a8:12"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{"a3:8", "a4:8", "a7:7"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{"a1:1", "a5:5", "a6:7"}))

	sf.updateVotes(c9, big.NewInt(2))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a2:9", "a8:12"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{"a3:8", "a4:8", "a7:7"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{"a1:1", "a5:5", "a6:7", "a9:2"}))

	sf.updateVotes(c10, big.NewInt(8))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a2:9", "a8:12"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{"a10:8", "a3:8", "a4:8"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{"a5:5", "a6:7", "a7:7", "a9:2"}))

	sf.updateVotes(c11, big.NewInt(3))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a2:9", "a8:12"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{"a10:8", "a3:8", "a4:8"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{"a11:3", "a5:5", "a6:7", "a7:7"}))

	sf.updateVotes(c12, big.NewInt(1))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Delegates()), []string{"a2:9", "a8:12"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Candidates()), []string{"a10:8", "a3:8", "a4:8"}))
	assert.True(t, reflect.DeepEqual(voteForm(sf.Buffer()), []string{"a11:3", "a5:5", "a6:7", "a7:7"}))
}
