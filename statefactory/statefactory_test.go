// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package statefactory

import (
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/test/mock/mock_trie"
)

func TestEncodeDecode(t *testing.T) {
	addr, err := iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	assert.Nil(t, err)

	ss, _ := stateToBytes(&State{Address: addr, Nonce: 0x10})
	assert.NotEmpty(t, ss)

	state, _ := bytesToState(ss)
	assert.Equal(t, addr.RawAddress, state.Address.RawAddress)
	assert.Equal(t, addr.PublicKey, state.Address.PublicKey)
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
	state, _ := sf.CreateState(addr, 0)
	assert.Equal(t, uint64(0x0), state.Nonce)
	assert.Equal(t, big.NewInt(0), state.Balance)
	assert.Equal(t, addr.RawAddress, state.Address.RawAddress)
}

func TestBalance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	trie := mock_trie.NewMockTrie(ctrl)
	addr, err := iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	assert.Nil(t, err)
	state := &State{Address: addr, Balance: big.NewInt(20)}
	mstate, _ := stateToBytes(state)
	trie.EXPECT().Get(gomock.Any()).Times(0).Return(mstate, nil)
	// Add 10 to the balance
	err = state.AddBalance(big.NewInt(10))
	assert.Nil(t, err)
	// balance should == 30 now
	assert.Equal(t, 0, state.Balance.Cmp(big.NewInt(30)))
}

//func TestNonce(t *testing.T) {
//	ctrl := gomock.NewController(t)
//	defer ctrl.Finish()
//
//	trie := mock_trie.NewMockTrie(ctrl)
//	sf := NewStateFactory(trie)
//
//	// Add 10 so the balance should be 10
//	addr, err := iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
//	assert.Nil(t, err)
//	mstate, _ := stateToBytes(&State{Address: addr, Nonce: 0x10})
//	trie.EXPECT().Get(gomock.Any()).Times(1).Return(mstate, nil)
//	addr, err = iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
//	assert.Nil(t, err)
//	n, err := sf.Nonce(addr)
//	assert.Equal(t, uint64(0x10), n)
//	assert.Nil(t, err)
//
//	trie.EXPECT().Get(gomock.Any()).Times(1).Return(nil, nil)
//	_, err = sf.Nonce(addr)
//	assert.Equal(t, ErrFailedToUnmarshalState, err)
//
//	trie.EXPECT().Upsert(gomock.Any(), gomock.Any()).Times(1).Do(func(key, value []byte) error {
//		state, _ := bytesToState(value)
//		assert.Equal(t, uint64(0x11), state.Nonce)
//		return nil
//	})
//	mstate, _ = stateToBytes(&State{Address: addr, Nonce: 0x10})
//	trie.EXPECT().Get(gomock.Any()).Times(1).Return(mstate, nil)
//	err = sf.SetNonce(addr, uint64(0x11))
//}
