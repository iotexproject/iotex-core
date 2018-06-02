// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package txpool

import (
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/common/utils"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/statefactory"
	"github.com/iotexproject/iotex-core/test/mock/mock_statefactory"
	"github.com/iotexproject/iotex-core/trie"
)

const testTriePath = "trie.test"

var (
	isTestnet = true
	chainid   = []byte{0x00, 0x00, 0x00, 0x01}
)

func TestActPool_validateTx(t *testing.T) {
	assert := assert.New(t)
	l := logger.Logger().Level(zerolog.DebugLevel)
	logger.SetLogger(&l)
	// Create one dummy iotex address
	pubK := "09d8c6fc6f5cb0a03df112da90486fad7cdece1501aaab658551f8afbe7f59ee"
	addr, _ := iotxaddress.GetAddress(decodeHash(pubK), isTestnet, chainid)
	cleanup := func() {
		if utils.FileExists(testTriePath) {
			err := os.Remove(testTriePath)
			assert.Nil(err)
		}
	}
	cleanup()
	defer cleanup()
	trie, _ := trie.NewTrie(testTriePath)
	sf := statefactory.NewStateFactory(trie)
	sf.CreateState(addr.RawAddress, uint64(100))
	ap := &actPool{
		sf:         sf,
		accountTxs: make(map[string]TxQueue),
		allTxs:     make(map[common.Hash32B]*action.Transfer),
	}
	// Case I: Oversized Data
	payload := []byte{}
	tmpPayload := [32769]byte{}
	payload = tmpPayload[:]
	tx := action.Transfer{Payload: payload}
	err := ap.validateTx(&tx)
	assert.Equal(ErrActPool, errors.Cause(err))
	// Case II: Negative Amount
	tx = action.Transfer{Amount: big.NewInt(-100)}
	err = ap.validateTx(&tx)
	assert.NotNil(ErrBalance, errors.Cause(err))
	// Case III: Nonce is too low
	prevTx := action.Transfer{Sender: addr.RawAddress, Recipient: addr.RawAddress, Nonce: uint64(1), Amount: big.NewInt(50)}
	ap.AddTx(&prevTx)
	err = ap.sf.CommitStateChanges([]*action.Transfer{&prevTx}, nil)
	assert.Nil(err)
	ap.Reset()
	tx = action.Transfer{Sender: addr.RawAddress, Recipient: addr.RawAddress, Nonce: uint64(1), Amount: big.NewInt(60)}
	err = ap.validateTx(&tx)
	assert.Equal(ErrNonce, errors.Cause(err))
}

func TestActPool_AddTx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	assert := assert.New(t)
	l := logger.Logger().Level(zerolog.DebugLevel)
	logger.SetLogger(&l)
	// Create two dummy iotex addresses
	pubK1 := "09d8c6fc6f5cb0a03df112da90486fad7cdece1501aaab658551f8afbe7f59ee"
	addr1, _ := iotxaddress.GetAddress(decodeHash(pubK1), isTestnet, chainid)
	pubK2 := "247a92febf1daac9c9d1a0f5224bb61eb14b2f291d10e01daa0aa5cf17d3f83a"
	addr2, _ := iotxaddress.GetAddress(decodeHash(pubK2), isTestnet, chainid)
	cleanup := func() {
		if utils.FileExists(testTriePath) {
			err := os.Remove(testTriePath)
			assert.Nil(err)
		}
	}
	cleanup()
	defer cleanup()
	trie, _ := trie.NewTrie(testTriePath)
	sf := statefactory.NewStateFactory(trie)
	sf.CreateState(addr1.RawAddress, uint64(100))
	sf.CreateState(addr2.RawAddress, uint64(10))
	// Create actpool
	ap := &actPool{
		sf:         sf,
		accountTxs: make(map[string]TxQueue),
		allTxs:     make(map[common.Hash32B]*action.Transfer),
	}
	// Test actpool status after adding a sequence of Txs: need to check confirmed nonce, pending nonce, and pending balance
	tx1 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(1), Amount: big.NewInt(10)}
	tx2 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(2), Amount: big.NewInt(20)}
	tx3 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(3), Amount: big.NewInt(30)}
	tx4 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(4), Amount: big.NewInt(40)}
	tx5 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(5), Amount: big.NewInt(50)}
	tx6 := action.Transfer{Sender: addr2.RawAddress, Nonce: uint64(1), Amount: big.NewInt(5)}
	tx7 := action.Transfer{Sender: addr2.RawAddress, Nonce: uint64(3), Amount: big.NewInt(1)}
	tx8 := action.Transfer{Sender: addr2.RawAddress, Nonce: uint64(4), Amount: big.NewInt(5)}

	ap.AddTx(&tx1)
	ap.AddTx(&tx2)
	ap.AddTx(&tx3)
	ap.AddTx(&tx4)
	ap.AddTx(&tx5)
	ap.AddTx(&tx6)
	ap.AddTx(&tx7)
	ap.AddTx(&tx8)

	cNonce1, _ := ap.getConfirmedNonce(addr1.RawAddress)
	assert.Equal(uint64(5), cNonce1)
	pBalance1, _ := ap.getPendingBalance(addr1.RawAddress)
	assert.Equal(uint64(0), pBalance1.Uint64())
	pNonce1, _ := ap.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(6), pNonce1)

	cNonce2, _ := ap.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(2), cNonce2)
	pBalance2, _ := ap.getPendingBalance(addr2.RawAddress)
	assert.Equal(uint64(5), pBalance2.Uint64())
	pNonce2, _ := ap.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(2), pNonce2)

	tx9 := action.Transfer{Sender: addr2.RawAddress, Nonce: uint64(2), Amount: big.NewInt(3)}
	ap.AddTx(&tx9)
	cNonce2, _ = ap.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(4), cNonce2)
	pBalance2, _ = ap.getPendingBalance(addr2.RawAddress)
	assert.Equal(uint64(1), pBalance2.Uint64())
	pNonce2, _ = ap.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(5), pNonce2)
	// Error Case Handling
	// Case I: Tx already exists in pool
	err := ap.AddTx(&tx1)
	assert.Equal(fmt.Errorf("existed transaction: %x", tx1.Hash()), err)
	// Case II: Pool space is full
	mockSF := mock_statefactory.NewMockStateFactory(ctrl)
	ap2 := &actPool{sf: mockSF, allTxs: make(map[common.Hash32B]*action.Transfer)}
	for i := 0; i < GlobalSlots; i++ {
		nTx := action.Transfer{Amount: big.NewInt(int64(i))}
		ap2.allTxs[nTx.Hash()] = &nTx
	}
	mockSF.EXPECT().Nonce(gomock.Any()).Times(1).Return(uint64(0), nil)
	err = ap2.AddTx(&tx1)
	assert.Equal(ErrActPool, errors.Cause(err))
	// Case III: Nonce already exists
	replaceTx := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(1), Amount: big.NewInt(1)}
	err = ap.AddTx(&replaceTx)
	assert.Equal(ErrNonce, errors.Cause(err))
	// Case IV: Queue space is full
	for i := 6; i <= AccountSlots; i++ {
		tx := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(i), Amount: big.NewInt(1)}
		err := ap.AddTx(&tx)
		assert.Nil(err)
	}
	outOfBoundsTx := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(AccountSlots + 1), Amount: big.NewInt(1)}
	err = ap.AddTx(&outOfBoundsTx)
	assert.Equal(ErrActPool, errors.Cause(err))
}

func TestActPool_PickTxs(t *testing.T) {
	assert := assert.New(t)
	l := logger.Logger().Level(zerolog.DebugLevel)
	logger.SetLogger(&l)
	// Create two dummy iotex addresses
	pubK1 := "09d8c6fc6f5cb0a03df112da90486fad7cdece1501aaab658551f8afbe7f59ee"
	addr1, _ := iotxaddress.GetAddress(decodeHash(pubK1), isTestnet, chainid)
	pubK2 := "247a92febf1daac9c9d1a0f5224bb61eb14b2f291d10e01daa0aa5cf17d3f83a"
	addr2, _ := iotxaddress.GetAddress(decodeHash(pubK2), isTestnet, chainid)
	cleanup := func() {
		if utils.FileExists(testTriePath) {
			err := os.Remove(testTriePath)
			assert.Nil(err)
		}
	}
	cleanup()
	defer cleanup()
	trie, _ := trie.NewTrie(testTriePath)
	sf := statefactory.NewStateFactory(trie)
	sf.CreateState(addr1.RawAddress, uint64(100))
	sf.CreateState(addr2.RawAddress, uint64(10))
	// Create actpool
	ap := &actPool{
		sf:         sf,
		accountTxs: make(map[string]TxQueue),
		allTxs:     make(map[common.Hash32B]*action.Transfer),
	}

	tx1 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(1), Amount: big.NewInt(10)}
	tx2 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(2), Amount: big.NewInt(20)}
	tx3 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(3), Amount: big.NewInt(30)}
	tx4 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(4), Amount: big.NewInt(40)}
	tx5 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(5), Amount: big.NewInt(50)}
	tx6 := action.Transfer{Sender: addr2.RawAddress, Nonce: uint64(2), Amount: big.NewInt(5)}
	tx7 := action.Transfer{Sender: addr2.RawAddress, Nonce: uint64(3), Amount: big.NewInt(1)}
	tx8 := action.Transfer{Sender: addr2.RawAddress, Nonce: uint64(5), Amount: big.NewInt(5)}

	ap.AddTx(&tx1)
	ap.AddTx(&tx2)
	ap.AddTx(&tx3)
	ap.AddTx(&tx4)
	ap.AddTx(&tx5)
	ap.AddTx(&tx6)
	ap.AddTx(&tx7)
	ap.AddTx(&tx8)

	pending, err := ap.PickTxs()
	assert.Nil(err)
	assert.Equal([]*action.Transfer{&tx1, &tx2, &tx3, &tx4}, pending[addr1.RawAddress])
	assert.Equal([]*action.Transfer{}, pending[addr2.RawAddress])
}

func TestActPool_removeCommittedTxs(t *testing.T) {
	assert := assert.New(t)
	l := logger.Logger().Level(zerolog.DebugLevel)
	logger.SetLogger(&l)
	// Create one dummy iotex address
	pubK := "09d8c6fc6f5cb0a03df112da90486fad7cdece1501aaab658551f8afbe7f59ee"
	addr, _ := iotxaddress.GetAddress(decodeHash(pubK), isTestnet, chainid)
	cleanup := func() {
		if utils.FileExists(testTriePath) {
			err := os.Remove(testTriePath)
			assert.Nil(err)
		}
	}
	cleanup()
	defer cleanup()
	trie, _ := trie.NewTrie(testTriePath)
	sf := statefactory.NewStateFactory(trie)
	sf.CreateState(addr.RawAddress, uint64(100))
	// Create actpool
	ap := &actPool{
		sf:         sf,
		accountTxs: make(map[string]TxQueue),
		allTxs:     make(map[common.Hash32B]*action.Transfer),
	}

	tx1 := action.Transfer{Sender: addr.RawAddress, Recipient: addr.RawAddress, Nonce: uint64(1), Amount: big.NewInt(10)}
	tx2 := action.Transfer{Sender: addr.RawAddress, Recipient: addr.RawAddress, Nonce: uint64(2), Amount: big.NewInt(20)}
	tx3 := action.Transfer{Sender: addr.RawAddress, Recipient: addr.RawAddress, Nonce: uint64(3), Amount: big.NewInt(30)}
	tx4 := action.Transfer{Sender: addr.RawAddress, Recipient: addr.RawAddress, Nonce: uint64(4), Amount: big.NewInt(40)}

	ap.AddTx(&tx1)
	ap.AddTx(&tx2)
	ap.AddTx(&tx3)
	ap.AddTx(&tx4)

	assert.Equal(4, len(ap.allTxs))
	assert.NotNil(ap.accountTxs[addr.RawAddress])
	err := ap.sf.CommitStateChanges([]*action.Transfer{&tx1, &tx2, &tx3, &tx4}, nil)
	assert.Nil(err)
	ap.removeCommittedTxs()
	assert.Equal(0, len(ap.allTxs))
	assert.Nil(ap.accountTxs[addr.RawAddress])
}

func TestActPool_Reset(t *testing.T) {
	assert := assert.New(t)
	l := logger.Logger().Level(zerolog.DebugLevel)
	logger.SetLogger(&l)
	// Create three dummy iotex addresses
	pubK1 := "09d8c6fc6f5cb0a03df112da90486fad7cdece1501aaab658551f8afbe7f59ee"
	addr1, _ := iotxaddress.GetAddress(decodeHash(pubK1), isTestnet, chainid)
	pubK2 := "247a92febf1daac9c9d1a0f5224bb61eb14b2f291d10e01daa0aa5cf17d3f83a"
	addr2, _ := iotxaddress.GetAddress(decodeHash(pubK2), isTestnet, chainid)
	pubK3 := "335664daa5d054e02004c71c28ed4ab4aaec650098fb29e518e23e25274a7273"
	addr3, _ := iotxaddress.GetAddress(decodeHash(pubK3), isTestnet, chainid)

	cleanup := func() {
		if utils.FileExists(testTriePath) {
			err := os.Remove(testTriePath)
			assert.Nil(err)
		}
	}
	cleanup()
	defer cleanup()
	trie, _ := trie.NewTrie(testTriePath)
	sf := statefactory.NewStateFactory(trie)
	sf.CreateState(addr1.RawAddress, uint64(100))
	sf.CreateState(addr2.RawAddress, uint64(200))
	sf.CreateState(addr3.RawAddress, uint64(300))

	ap1 := &actPool{
		sf:         sf,
		accountTxs: make(map[string]TxQueue),
		allTxs:     make(map[common.Hash32B]*action.Transfer),
	}

	ap2 := &actPool{
		sf:         sf,
		accountTxs: make(map[string]TxQueue),
		allTxs:     make(map[common.Hash32B]*action.Transfer),
	}

	// Txs to be added to ap1
	tx1 := action.Transfer{Sender: addr1.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(1), Amount: big.NewInt(50)}
	tx2 := action.Transfer{Sender: addr1.RawAddress, Recipient: addr3.RawAddress, Nonce: uint64(2), Amount: big.NewInt(30)}
	tx3 := action.Transfer{Sender: addr1.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(3), Amount: big.NewInt(60)}
	tx4 := action.Transfer{Sender: addr2.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(1), Amount: big.NewInt(100)}
	tx5 := action.Transfer{Sender: addr2.RawAddress, Recipient: addr3.RawAddress, Nonce: uint64(2), Amount: big.NewInt(50)}
	tx6 := action.Transfer{Sender: addr2.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(3), Amount: big.NewInt(60)}
	tx7 := action.Transfer{Sender: addr3.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(1), Amount: big.NewInt(100)}
	tx8 := action.Transfer{Sender: addr3.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(2), Amount: big.NewInt(100)}
	tx9 := action.Transfer{Sender: addr3.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(4), Amount: big.NewInt(100)}

	ap1.AddTx(&tx1)
	ap1.AddTx(&tx2)
	ap1.AddTx(&tx3)
	ap1.AddTx(&tx4)
	ap1.AddTx(&tx5)
	ap1.AddTx(&tx6)
	ap1.AddTx(&tx7)
	ap1.AddTx(&tx8)
	ap1.AddTx(&tx9)
	// Txs to be added to ap2 only
	tx10 := action.Transfer{Sender: addr1.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(3), Amount: big.NewInt(20)}
	tx11 := action.Transfer{Sender: addr1.RawAddress, Recipient: addr3.RawAddress, Nonce: uint64(4), Amount: big.NewInt(10)}
	tx12 := action.Transfer{Sender: addr2.RawAddress, Recipient: addr3.RawAddress, Nonce: uint64(2), Amount: big.NewInt(70)}
	tx13 := action.Transfer{Sender: addr3.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(1), Amount: big.NewInt(200)}
	tx14 := action.Transfer{Sender: addr3.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(2), Amount: big.NewInt(50)}

	ap2.AddTx(&tx1)
	ap2.AddTx(&tx2)
	ap2.AddTx(&tx10)
	ap2.AddTx(&tx11)
	ap2.AddTx(&tx4)
	ap2.AddTx(&tx12)
	ap2.AddTx(&tx13)
	ap2.AddTx(&tx14)
	ap2.AddTx(&tx9)
	// Check confirmed nonce, pending nonce, and pending balance after adding Txs above for each account
	// ap1
	// Addr1
	ap1CNonce1, _ := ap1.getConfirmedNonce(addr1.RawAddress)
	assert.Equal(uint64(3), ap1CNonce1)
	ap1PNonce1, _ := ap1.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(4), ap1PNonce1)
	ap1PBalance1, _ := ap1.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(20).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1CNonce2, _ := ap1.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(3), ap1CNonce2)
	ap1PNonce2, _ := ap1.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(4), ap1PNonce2)
	ap1PBalance2, _ := ap1.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(50).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1CNonce3, _ := ap1.getConfirmedNonce(addr3.RawAddress)
	assert.Equal(uint64(3), ap1CNonce3)
	ap1PNonce3, _ := ap1.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(3), ap1PNonce3)
	ap1PBalance3, _ := ap1.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(100).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2CNonce1, _ := ap2.getConfirmedNonce(addr1.RawAddress)
	assert.Equal(uint64(4), ap2CNonce1)
	ap2PNonce1, _ := ap2.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(5), ap2PNonce1)
	ap2PBalance1, _ := ap2.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(0).Uint64(), ap2PBalance1.Uint64())
	// Addr2
	ap2CNonce2, _ := ap2.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(3), ap2CNonce2)
	ap2PNonce2, _ := ap2.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(3), ap2PNonce2)
	ap2PBalance2, _ := ap2.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(30).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2CNonce3, _ := ap2.getConfirmedNonce(addr3.RawAddress)
	assert.Equal(uint64(3), ap2CNonce3)
	ap2PNonce3, _ := ap2.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(3), ap2PNonce3)
	ap2PBalance3, _ := ap2.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(50).Uint64(), ap2PBalance3.Uint64())
	// Let ap1 be BP's actpool
	pickedTxs, _ := ap1.PickTxs()
	// ap1 commits update of balance and nonce to trie
	for _, txs := range pickedTxs {
		err := ap1.sf.CommitStateChanges(txs, nil)
		assert.Nil(err)
	}
	//Reset
	ap1.Reset()
	ap2.Reset()
	// Check confirmed nonce, pending nonce, and pending balance after resetting actpool for each account
	// ap1
	// Addr1
	ap1CNonce1, _ = ap1.getConfirmedNonce(addr1.RawAddress)
	assert.Equal(uint64(4), ap1CNonce1)
	ap1PNonce1, _ = ap1.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(4), ap1PNonce1)
	ap1PBalance1, _ = ap1.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(160).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1CNonce2, _ = ap1.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(4), ap1CNonce2)
	ap1PNonce2, _ = ap1.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(4), ap1PNonce2)
	ap1PBalance2, _ = ap1.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(140).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1CNonce3, _ = ap1.getConfirmedNonce(addr3.RawAddress)
	assert.Equal(uint64(3), ap1CNonce3)
	ap1PNonce3, _ = ap1.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(3), ap1PNonce3)
	ap1PBalance3, _ = ap1.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(180).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2CNonce1, _ = ap2.getConfirmedNonce(addr1.RawAddress)
	assert.Equal(uint64(5), ap2CNonce1)
	ap2PNonce1, _ = ap2.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(5), ap2PNonce1)
	ap2PBalance1, _ = ap2.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(190).Uint64(), ap2PBalance1.Uint64())
	// Addr2
	ap2CNonce2, _ = ap2.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(3), ap2CNonce2)
	ap2PNonce2, _ = ap2.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(3), ap2PNonce2)
	ap2PBalance2, _ = ap2.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(200).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2CNonce3, _ = ap2.getConfirmedNonce(addr3.RawAddress)
	assert.Equal(uint64(3), ap2CNonce3)
	ap2PNonce3, _ = ap2.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(3), ap2PNonce3)
	ap2PBalance3, _ = ap2.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(180).Uint64(), ap2PBalance3.Uint64())
	// Add more Txs after resetting
	// Txs To be added to ap1 only
	tx15 := action.Transfer{Sender: addr3.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(3), Amount: big.NewInt(80)}
	// Txs To be added to ap2 only
	tx16 := action.Transfer{Sender: addr1.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(5), Amount: big.NewInt(150)}
	tx17 := action.Transfer{Sender: addr2.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(3), Amount: big.NewInt(90)}
	tx18 := action.Transfer{Sender: addr2.RawAddress, Recipient: addr3.RawAddress, Nonce: uint64(4), Amount: big.NewInt(100)}
	tx19 := action.Transfer{Sender: addr2.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(5), Amount: big.NewInt(50)}
	tx20 := action.Transfer{Sender: addr3.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(3), Amount: big.NewInt(200)}

	ap1.AddTx(&tx15)
	ap2.AddTx(&tx16)
	ap2.AddTx(&tx17)
	ap2.AddTx(&tx18)
	ap2.AddTx(&tx19)
	ap2.AddTx(&tx20)
	// Check confirmed nonce, pending nonce, and pending balance after adding Txs above for each account
	// ap1
	// Addr1
	ap1CNonce1, _ = ap1.getConfirmedNonce(addr1.RawAddress)
	assert.Equal(uint64(4), ap1CNonce1)
	ap1PNonce1, _ = ap1.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(4), ap1PNonce1)
	ap1PBalance1, _ = ap1.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(160).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1CNonce2, _ = ap1.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(4), ap1CNonce2)
	ap1PNonce2, _ = ap1.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(4), ap1PNonce2)
	ap1PBalance2, _ = ap1.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(140).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1CNonce3, _ = ap1.getConfirmedNonce(addr3.RawAddress)
	assert.Equal(uint64(5), ap1CNonce3)
	ap1PNonce3, _ = ap1.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(5), ap1PNonce3)
	ap1PBalance3, _ = ap1.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(0).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2CNonce1, _ = ap2.getConfirmedNonce(addr1.RawAddress)
	assert.Equal(uint64(6), ap2CNonce1)
	ap2PNonce1, _ = ap2.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(6), ap2PNonce1)
	ap2PBalance1, _ = ap2.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(40).Uint64(), ap2PBalance1.Uint64())
	// Addr2
	ap2CNonce2, _ = ap2.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(5), ap2CNonce2)
	ap2PNonce2, _ = ap2.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(6), ap2PNonce2)
	ap2PBalance2, _ = ap2.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(10).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2CNonce3, _ = ap2.getConfirmedNonce(addr3.RawAddress)
	assert.Equal(uint64(3), ap2CNonce3)
	ap2PNonce3, _ = ap2.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(5), ap2PNonce3)
	ap2PBalance3, _ = ap2.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(180).Uint64(), ap2PBalance3.Uint64())
	// Let ap2 be BP's actpool
	pickedTxs, _ = ap2.PickTxs()
	// ap2 commits update of balance and nonce to trie
	for _, txs := range pickedTxs {
		err := ap2.sf.CommitStateChanges(txs, nil)
		assert.Nil(err)
	}
	//Reset
	ap1.Reset()
	ap2.Reset()
	// Check confirmed nonce, pending nonce, and pending balance after resetting actpool for each account
	// ap1
	// Addr1
	ap1CNonce1, _ = ap1.getConfirmedNonce(addr1.RawAddress)
	assert.Equal(uint64(6), ap1CNonce1)
	ap1PNonce1, _ = ap1.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(6), ap1PNonce1)
	ap1PBalance1, _ = ap1.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(130).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1CNonce2, _ = ap1.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(5), ap1CNonce2)
	ap1PNonce2, _ = ap1.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(5), ap1PNonce2)
	ap1PBalance2, _ = ap1.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(180).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1CNonce3, _ = ap1.getConfirmedNonce(addr3.RawAddress)
	assert.Equal(uint64(5), ap1CNonce3)
	ap1PNonce3, _ = ap1.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(5), ap1PNonce3)
	ap1PBalance3, _ = ap1.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(110).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2CNonce1, _ = ap2.getConfirmedNonce(addr1.RawAddress)
	assert.Equal(uint64(6), ap2CNonce1)
	ap2PNonce1, _ = ap2.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(6), ap2PNonce1)
	ap2PBalance1, _ = ap2.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(130).Uint64(), ap2PBalance1.Uint64())
	// Addr2
	ap2CNonce2, _ = ap2.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(6), ap2CNonce2)
	ap2PNonce2, _ = ap2.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(6), ap2PNonce2)
	ap2PBalance2, _ = ap2.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(130).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2CNonce3, _ = ap2.getConfirmedNonce(addr3.RawAddress)
	assert.Equal(uint64(4), ap2CNonce3)
	ap2PNonce3, _ = ap2.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(5), ap2PNonce3)
	ap2PBalance3, _ = ap2.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(90).Uint64(), ap2PBalance3.Uint64())
}

// Helper function to return the correct confirmed nonce just in case of empty queue
func (ap *actPool) getConfirmedNonce(addr string) (uint64, error) {
	if queue, ok := ap.accountTxs[addr]; ok {
		return queue.ConfirmedNonce(), nil
	}
	committedNonce, err := ap.sf.Nonce(addr)
	confirmedNonce := committedNonce + 1
	return confirmedNonce, err
}

// Helper function to return the correct pending nonce just in case of empty queue
func (ap *actPool) getPendingNonce(addr string) (uint64, error) {
	if queue, ok := ap.accountTxs[addr]; ok {
		return queue.PendingNonce(), nil
	}
	committedNonce, err := ap.sf.Nonce(addr)
	pendingNonce := committedNonce + 1
	return pendingNonce, err
}

// Helper function to return the correct pending balance just in case of empty queue
func (ap *actPool) getPendingBalance(addr string) (*big.Int, error) {
	if queue, ok := ap.accountTxs[addr]; ok {
		return queue.PendingBalance(), nil
	}
	return ap.sf.Balance(addr)
}
