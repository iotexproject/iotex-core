package statefactory

import (
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/test/mock/mock_trie"
)

func TestEncodeDecode(t *testing.T) {
	addr, err := iotxaddress.NewAddress(true, 0x01, []byte{0xa4, 0x00, 0x00, 0x00})
	assert.Nil(t, err)

	ss := stateToBytes(&State{Address: addr})
	assert.NotEmpty(t, ss)

	state := bytesToState(ss)
	assert.Equal(t, addr.RawAddress, state.Address.RawAddress)
	assert.Equal(t, addr.PublicKey, state.Address.PublicKey)
}

func TestRootHash(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	trie := mock_trie.NewMockTrie(ctrl)
	sf := New(db.NewMemKVStore(), trie)
	trie.EXPECT().RootHash().Times(1).Return(common.ZeroHash32B)
	assert.Equal(t, common.ZeroHash32B, sf.RootHash())
}

func TestAddState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	trie := mock_trie.NewMockTrie(ctrl)
	kvdb := db.NewMemKVStore()
	kvdb.Start()
	sf := New(kvdb, trie)
	trie.EXPECT().Update(gomock.Any(), gomock.Any()).Times(1)
	addr, err := iotxaddress.NewAddress(true, 0x01, []byte{0xa4, 0x00, 0x00, 0x00})
	assert.Nil(t, err)
	state := sf.AddState(addr)
	assert.Equal(t, uint64(0x0), state.Nonce)
	assert.Equal(t, *big.NewInt(0), state.Balance)
	assert.Equal(t, addr.RawAddress, state.Address.RawAddress)
}

func TestBalance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	trie := mock_trie.NewMockTrie(ctrl)
	kvdb := db.NewMemKVStore()
	kvdb.Start()
	sf := New(kvdb, trie)

	// Add 10 so the balance should be 10
	addr, err := iotxaddress.NewAddress(true, 0x01, []byte{0xa4, 0x00, 0x00, 0x00})
	assert.Nil(t, err)
	trie.EXPECT().Update(gomock.Any(), gomock.Any()).Times(1)
	trie.EXPECT().Get(gomock.Any()).Times(1).Return(stateToBytes(&State{Address: addr}), nil)
	addr, err = iotxaddress.NewAddress(true, 0x01, []byte{0xa4, 0x00, 0x00, 0x00})
	assert.Nil(t, err)
	err = sf.AddBalance(addr, big.NewInt(10))
	assert.Nil(t, err)
}
