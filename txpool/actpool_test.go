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
	"github.com/stretchr/testify/assert"

	trx "github.com/iotexproject/iotex-core-internal/blockchain/trx"
	"github.com/iotexproject/iotex-core-internal/common"
	"github.com/iotexproject/iotex-core-internal/iotxaddress"
	"github.com/iotexproject/iotex-core-internal/logger"
	"github.com/iotexproject/iotex-core-internal/statefactory"
	"github.com/iotexproject/iotex-core-internal/test/mock/mock_statefactory"
	"github.com/iotexproject/iotex-core-internal/trie"
)

const testTriePath = "trie.test"

var (
	isTestnet = true
	chainid   = []byte{0x00, 0x00, 0x00, 0x01}
)

func TestActPool_validateTx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	assert := assert.New(t)
	logger.UseDebugLogger()
	// Create one dummy iotex address
	pubK := "09d8c6fc6f5cb0a03df112da90486fad7cdece1501aaab658551f8afbe7f59ee"
	addr, _ := iotxaddress.GetAddress(decodeHash(pubK), isTestnet, chainid)
	defer os.Remove(testTriePath)
	trie, _ := trie.NewTrie(testTriePath)
	sf := statefactory.NewStateFactory(trie)
	sf.CreateState(addr.RawAddress, uint64(100))
	ap := &actPool{
		sf:         sf,
		accountTxs: make(map[string]TxQueue),
		allTxs:     make(map[common.Hash32B]*trx.Tx),
	}
	// Case I: Oversized Data
	payload := []byte{}
	tmpPayload := [32769]byte{}
	payload = tmpPayload[:]
	tx := trx.Tx{Payload: payload}
	err := ap.validateTx(&tx)
	assert.Equal(ErrActPool, errors.Cause(err))
	// Case II: Negative Amount
	tx = trx.Tx{Amount: big.NewInt(-100)}
	err = ap.validateTx(&tx)
	assert.NotNil(ErrBalance, errors.Cause(err))
	// Case III: Nonce is too low
	ap.sf.SetNonce(addr.RawAddress, uint64(1))
	tx = trx.Tx{Sender: addr.RawAddress, Nonce: uint64(0), Amount: big.NewInt(50)}
	err = ap.validateTx(&tx)
	assert.Equal(ErrNonce, errors.Cause(err))
}

func TestActPool_AddTx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	assert := assert.New(t)
	logger.UseDebugLogger()
	// Create two dummy iotex addresses
	pubK1 := "09d8c6fc6f5cb0a03df112da90486fad7cdece1501aaab658551f8afbe7f59ee"
	addr1, _ := iotxaddress.GetAddress(decodeHash(pubK1), isTestnet, chainid)
	pubK2 := "247a92febf1daac9c9d1a0f5224bb61eb14b2f291d10e01daa0aa5cf17d3f83a"
	addr2, _ := iotxaddress.GetAddress(decodeHash(pubK2), isTestnet, chainid)
	defer os.Remove(testTriePath)
	trie, _ := trie.NewTrie(testTriePath)
	sf := statefactory.NewStateFactory(trie)
	sf.CreateState(addr1.RawAddress, uint64(100))
	sf.CreateState(addr2.RawAddress, uint64(10))
	// Create actpool
	ap := &actPool{
		sf:         sf,
		accountTxs: make(map[string]TxQueue),
		allTxs:     make(map[common.Hash32B]*trx.Tx),
	}
	// Test actpool status after adding a sequence of Txs: need to check confirmed nonce, pending nonce, and pending balance
	tx1 := trx.Tx{Sender: addr1.RawAddress, Nonce: uint64(0), Amount: big.NewInt(10)}
	tx2 := trx.Tx{Sender: addr1.RawAddress, Nonce: uint64(1), Amount: big.NewInt(20)}
	tx3 := trx.Tx{Sender: addr1.RawAddress, Nonce: uint64(2), Amount: big.NewInt(30)}
	tx4 := trx.Tx{Sender: addr1.RawAddress, Nonce: uint64(3), Amount: big.NewInt(40)}
	tx5 := trx.Tx{Sender: addr1.RawAddress, Nonce: uint64(4), Amount: big.NewInt(50)}
	tx6 := trx.Tx{Sender: addr2.RawAddress, Nonce: uint64(0), Amount: big.NewInt(5)}
	tx7 := trx.Tx{Sender: addr2.RawAddress, Nonce: uint64(2), Amount: big.NewInt(1)}
	tx8 := trx.Tx{Sender: addr2.RawAddress, Nonce: uint64(3), Amount: big.NewInt(5)}

	ap.AddTx(&tx1)
	ap.AddTx(&tx2)
	ap.AddTx(&tx3)
	ap.AddTx(&tx4)
	ap.AddTx(&tx5)
	ap.AddTx(&tx6)
	ap.AddTx(&tx7)
	ap.AddTx(&tx8)

	cNonce1, _ := ap.getConfirmedNonce(addr1.RawAddress)
	assert.Equal(uint64(4), cNonce1)
	pBalance1, _ := ap.getPendingBalance(addr1.RawAddress)
	assert.Equal(uint64(0), pBalance1.Uint64())
	pNonce1, _ := ap.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(5), pNonce1)

	cNonce2, _ := ap.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(1), cNonce2)
	pBalance2, _ := ap.getPendingBalance(addr2.RawAddress)
	assert.Equal(uint64(5), pBalance2.Uint64())
	pNonce2, _ := ap.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(1), pNonce2)

	tx9 := trx.Tx{Sender: addr2.RawAddress, Nonce: uint64(1), Amount: big.NewInt(3)}
	ap.AddTx(&tx9)
	cNonce2, _ = ap.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(3), cNonce2)
	pBalance2, _ = ap.getPendingBalance(addr2.RawAddress)
	assert.Equal(uint64(1), pBalance2.Uint64())
	pNonce2, _ = ap.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(4), pNonce2)
	// Error Case Handling
	// Case I: Tx already exists in pool
	err := ap.AddTx(&tx1)
	assert.Equal(fmt.Errorf("existed transaction: %x", tx1.Hash()), err)
	// Case II: Pool space is full
	mockSF := mock_statefactory.NewMockStateFactory(ctrl)
	ap2 := &actPool{sf: mockSF, allTxs: make(map[common.Hash32B]*trx.Tx)}
	for i := 0; i < GlobalSlots; i++ {
		nTx := trx.Tx{Amount: big.NewInt(int64(i))}
		ap2.allTxs[nTx.Hash()] = &nTx
	}
	mockSF.EXPECT().Nonce(gomock.Any()).Times(1).Return(uint64(0), nil)
	err = ap2.AddTx(&tx1)
	assert.Equal(ErrActPool, errors.Cause(err))
	// Case III: Nonce already exists
	replaceTx := trx.Tx{Sender: addr1.RawAddress, Nonce: uint64(0), Amount: big.NewInt(1)}
	err = ap.AddTx(&replaceTx)
	assert.Equal(ErrNonce, errors.Cause(err))
	// Case IV: Queue space is full
	for i := 5; i < AccountSlots; i++ {
		tx := trx.Tx{Sender: addr1.RawAddress, Nonce: uint64(i), Amount: big.NewInt(1)}
		err := ap.AddTx(&tx)
		assert.Nil(err)
	}
	outOfBoundsTx := trx.Tx{Sender: addr1.RawAddress, Nonce: uint64(AccountSlots), Amount: big.NewInt(1)}
	err = ap.AddTx(&outOfBoundsTx)
	assert.Equal(ErrActPool, errors.Cause(err))
}

func TestActPool_PickTxs(t *testing.T) {
	assert := assert.New(t)
	logger.UseDebugLogger()
	// Create two dummy iotex addresses
	pubK1 := "09d8c6fc6f5cb0a03df112da90486fad7cdece1501aaab658551f8afbe7f59ee"
	addr1, _ := iotxaddress.GetAddress(decodeHash(pubK1), isTestnet, chainid)
	pubK2 := "247a92febf1daac9c9d1a0f5224bb61eb14b2f291d10e01daa0aa5cf17d3f83a"
	addr2, _ := iotxaddress.GetAddress(decodeHash(pubK2), isTestnet, chainid)
	defer os.Remove(testTriePath)
	trie, _ := trie.NewTrie(testTriePath)
	sf := statefactory.NewStateFactory(trie)
	sf.CreateState(addr1.RawAddress, uint64(100))
	sf.CreateState(addr2.RawAddress, uint64(10))
	// Create actpool
	ap := &actPool{
		sf:         sf,
		accountTxs: make(map[string]TxQueue),
		allTxs:     make(map[common.Hash32B]*trx.Tx),
	}

	tx1 := trx.Tx{Sender: addr1.RawAddress, Nonce: uint64(0), Amount: big.NewInt(10)}
	tx2 := trx.Tx{Sender: addr1.RawAddress, Nonce: uint64(1), Amount: big.NewInt(20)}
	tx3 := trx.Tx{Sender: addr1.RawAddress, Nonce: uint64(2), Amount: big.NewInt(30)}
	tx4 := trx.Tx{Sender: addr1.RawAddress, Nonce: uint64(3), Amount: big.NewInt(40)}
	tx5 := trx.Tx{Sender: addr1.RawAddress, Nonce: uint64(4), Amount: big.NewInt(50)}
	tx6 := trx.Tx{Sender: addr2.RawAddress, Nonce: uint64(1), Amount: big.NewInt(5)}
	tx7 := trx.Tx{Sender: addr2.RawAddress, Nonce: uint64(2), Amount: big.NewInt(1)}
	tx8 := trx.Tx{Sender: addr2.RawAddress, Nonce: uint64(4), Amount: big.NewInt(5)}

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
	assert.Equal([]*trx.Tx{&tx1, &tx2, &tx3, &tx4}, pending[addr1.RawAddress])
	assert.Equal([]*trx.Tx{}, pending[addr2.RawAddress])
}

func TestActPool_removeCommittedTxs(t *testing.T) {
	assert := assert.New(t)
	logger.UseDebugLogger()
	// Create one dummy iotex address
	pubK := "09d8c6fc6f5cb0a03df112da90486fad7cdece1501aaab658551f8afbe7f59ee"
	addr, _ := iotxaddress.GetAddress(decodeHash(pubK), isTestnet, chainid)
	defer os.Remove(testTriePath)
	trie, _ := trie.NewTrie(testTriePath)
	sf := statefactory.NewStateFactory(trie)
	sf.CreateState(addr.RawAddress, uint64(100))
	// Create actpool
	ap := &actPool{
		sf:         sf,
		accountTxs: make(map[string]TxQueue),
		allTxs:     make(map[common.Hash32B]*trx.Tx),
	}

	tx1 := trx.Tx{Sender: addr.RawAddress, Nonce: uint64(0), Amount: big.NewInt(10)}
	tx2 := trx.Tx{Sender: addr.RawAddress, Nonce: uint64(1), Amount: big.NewInt(20)}
	tx3 := trx.Tx{Sender: addr.RawAddress, Nonce: uint64(2), Amount: big.NewInt(30)}
	tx4 := trx.Tx{Sender: addr.RawAddress, Nonce: uint64(3), Amount: big.NewInt(40)}

	ap.AddTx(&tx1)
	ap.AddTx(&tx2)
	ap.AddTx(&tx3)
	ap.AddTx(&tx4)

	assert.Equal(4, len(ap.allTxs))
	assert.NotNil(ap.accountTxs[addr.RawAddress])
	err := ap.sf.SetNonce(addr.RawAddress, ap.accountTxs[addr.RawAddress].ConfirmedNonce())
	assert.Nil(err)
	ap.removeCommittedTxs()
	assert.Equal(0, len(ap.allTxs))
	assert.Nil(ap.accountTxs[addr.RawAddress])
}

func TestActPool_Reset(t *testing.T) {
	assert := assert.New(t)
	logger.UseDebugLogger()
	// Create three dummy iotex addresses
	pubK1 := "09d8c6fc6f5cb0a03df112da90486fad7cdece1501aaab658551f8afbe7f59ee"
	addr1, _ := iotxaddress.GetAddress(decodeHash(pubK1), isTestnet, chainid)
	pubK2 := "247a92febf1daac9c9d1a0f5224bb61eb14b2f291d10e01daa0aa5cf17d3f83a"
	addr2, _ := iotxaddress.GetAddress(decodeHash(pubK2), isTestnet, chainid)
	pubK3 := "335664daa5d054e02004c71c28ed4ab4aaec650098fb29e518e23e25274a7273"
	addr3, _ := iotxaddress.GetAddress(decodeHash(pubK3), isTestnet, chainid)

	defer os.Remove(testTriePath)
	trie, _ := trie.NewTrie(testTriePath)
	sf := statefactory.NewStateFactory(trie)
	sf.CreateState(addr1.RawAddress, uint64(100))
	sf.CreateState(addr2.RawAddress, uint64(200))
	sf.CreateState(addr3.RawAddress, uint64(300))

	ap1 := &actPool{
		sf:         sf,
		accountTxs: make(map[string]TxQueue),
		allTxs:     make(map[common.Hash32B]*trx.Tx),
	}

	ap2 := &actPool{
		sf:         sf,
		accountTxs: make(map[string]TxQueue),
		allTxs:     make(map[common.Hash32B]*trx.Tx),
	}

	// Txs to be added to ap1
	tx1 := trx.Tx{Sender: addr1.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(0), Amount: big.NewInt(50)}
	tx2 := trx.Tx{Sender: addr1.RawAddress, Recipient: addr3.RawAddress, Nonce: uint64(1), Amount: big.NewInt(30)}
	tx3 := trx.Tx{Sender: addr1.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(2), Amount: big.NewInt(60)}
	tx4 := trx.Tx{Sender: addr2.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(0), Amount: big.NewInt(100)}
	tx5 := trx.Tx{Sender: addr2.RawAddress, Recipient: addr3.RawAddress, Nonce: uint64(1), Amount: big.NewInt(50)}
	tx6 := trx.Tx{Sender: addr2.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(2), Amount: big.NewInt(60)}
	tx7 := trx.Tx{Sender: addr3.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(0), Amount: big.NewInt(100)}
	tx8 := trx.Tx{Sender: addr3.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(1), Amount: big.NewInt(100)}
	tx9 := trx.Tx{Sender: addr3.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(3), Amount: big.NewInt(100)}

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
	tx10 := trx.Tx{Sender: addr1.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(2), Amount: big.NewInt(20)}
	tx11 := trx.Tx{Sender: addr1.RawAddress, Recipient: addr3.RawAddress, Nonce: uint64(3), Amount: big.NewInt(10)}
	tx12 := trx.Tx{Sender: addr2.RawAddress, Recipient: addr3.RawAddress, Nonce: uint64(1), Amount: big.NewInt(70)}
	tx13 := trx.Tx{Sender: addr3.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(0), Amount: big.NewInt(200)}
	tx14 := trx.Tx{Sender: addr3.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(1), Amount: big.NewInt(50)}

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
	ap1_cNonce1, _ := ap1.getConfirmedNonce(addr1.RawAddress)
	assert.Equal(uint64(2), ap1_cNonce1)
	ap1_pNonce1, _ := ap1.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(3), ap1_pNonce1)
	ap1_pBalance1, _ := ap1.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(20).Uint64(), ap1_pBalance1.Uint64())
	// Addr2
	ap1_cNonce2, _ := ap1.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(2), ap1_cNonce2)
	ap1_pNonce2, _ := ap1.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(3), ap1_pNonce2)
	ap1_pBalance2, _ := ap1.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(50).Uint64(), ap1_pBalance2.Uint64())
	// Addr3
	ap1_cNonce3, _ := ap1.getConfirmedNonce(addr3.RawAddress)
	assert.Equal(uint64(2), ap1_cNonce3)
	ap1_pNonce3, _ := ap1.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(2), ap1_pNonce3)
	ap1_pBalance3, _ := ap1.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(100).Uint64(), ap1_pBalance3.Uint64())
	// ap2
	// Addr1
	ap2_cNonce1, _ := ap2.getConfirmedNonce(addr1.RawAddress)
	assert.Equal(uint64(3), ap2_cNonce1)
	ap2_pNonce1, _ := ap2.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(4), ap2_pNonce1)
	ap2_pBalance1, _ := ap2.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(0).Uint64(), ap2_pBalance1.Uint64())
	// Addr2
	ap2_cNonce2, _ := ap2.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(2), ap2_cNonce2)
	ap2_pNonce2, _ := ap2.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(2), ap2_pNonce2)
	ap2_pBalance2, _ := ap2.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(30).Uint64(), ap2_pBalance2.Uint64())
	// Addr3
	ap2_cNonce3, _ := ap2.getConfirmedNonce(addr3.RawAddress)
	assert.Equal(uint64(2), ap2_cNonce3)
	ap2_pNonce3, _ := ap2.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(2), ap2_pNonce3)
	ap2_pBalance3, _ := ap2.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(50).Uint64(), ap2_pBalance3.Uint64())
	// Let ap1 be BP's actpool
	pickedTxs, _ := ap1.PickTxs()
	// ap1 commits update of balance to trie
	for _, txs := range pickedTxs {
		err := ap1.sf.UpdateStatesWithTransfer(txs)
		assert.Nil(err)
	}
	// ap1 commits update of nonce to trie
	ap1.sf.SetNonce(addr1.RawAddress, ap1_cNonce1)
	ap1.sf.SetNonce(addr2.RawAddress, ap1_cNonce2)
	ap1.sf.SetNonce(addr3.RawAddress, ap1_cNonce3)
	//Reset
	ap1.Reset()
	ap2.Reset()
	// Check confirmed nonce, pending nonce, and pending balance after resetting actpool for each account
	// ap1
	// Addr1
	ap1_cNonce1, _ = ap1.getConfirmedNonce(addr1.RawAddress)
	assert.Equal(uint64(3), ap1_cNonce1)
	ap1_pNonce1, _ = ap1.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(3), ap1_pNonce1)
	ap1_pBalance1, _ = ap1.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(160).Uint64(), ap1_pBalance1.Uint64())
	// Addr2
	ap1_cNonce2, _ = ap1.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(3), ap1_cNonce2)
	ap1_pNonce2, _ = ap1.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(3), ap1_pNonce2)
	ap1_pBalance2, _ = ap1.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(140).Uint64(), ap1_pBalance2.Uint64())
	// Addr3
	ap1_cNonce3, _ = ap1.getConfirmedNonce(addr3.RawAddress)
	assert.Equal(uint64(2), ap1_cNonce3)
	ap1_pNonce3, _ = ap1.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(2), ap1_pNonce3)
	ap1_pBalance3, _ = ap1.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(180).Uint64(), ap1_pBalance3.Uint64())
	// ap2
	// Addr1
	ap2_cNonce1, _ = ap2.getConfirmedNonce(addr1.RawAddress)
	assert.Equal(uint64(4), ap2_cNonce1)
	ap2_pNonce1, _ = ap2.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(4), ap2_pNonce1)
	ap2_pBalance1, _ = ap2.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(190).Uint64(), ap2_pBalance1.Uint64())
	// Addr2
	ap2_cNonce2, _ = ap2.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(2), ap2_cNonce2)
	ap2_pNonce2, _ = ap2.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(2), ap2_pNonce2)
	ap2_pBalance2, _ = ap2.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(200).Uint64(), ap2_pBalance2.Uint64())
	// Addr3
	ap2_cNonce3, _ = ap2.getConfirmedNonce(addr3.RawAddress)
	assert.Equal(uint64(2), ap2_cNonce3)
	ap2_pNonce3, _ = ap2.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(2), ap2_pNonce3)
	ap2_pBalance3, _ = ap2.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(180).Uint64(), ap2_pBalance3.Uint64())
	// Add more Txs after resetting
	// Txs To be added to ap1 only
	tx15 := trx.Tx{Sender: addr3.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(2), Amount: big.NewInt(80)}
	// Txs To be added to ap2 only
	tx16 := trx.Tx{Sender: addr1.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(4), Amount: big.NewInt(150)}
	tx17 := trx.Tx{Sender: addr2.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(2), Amount: big.NewInt(90)}
	tx18 := trx.Tx{Sender: addr2.RawAddress, Recipient: addr3.RawAddress, Nonce: uint64(3), Amount: big.NewInt(100)}
	tx19 := trx.Tx{Sender: addr2.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(4), Amount: big.NewInt(50)}
	tx20 := trx.Tx{Sender: addr3.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(2), Amount: big.NewInt(200)}

	ap1.AddTx(&tx15)
	ap2.AddTx(&tx16)
	ap2.AddTx(&tx17)
	ap2.AddTx(&tx18)
	ap2.AddTx(&tx19)
	ap2.AddTx(&tx20)
	// Check confirmed nonce, pending nonce, and pending balance after adding Txs above for each account
	// ap1
	// Addr1
	ap1_cNonce1, _ = ap1.getConfirmedNonce(addr1.RawAddress)
	assert.Equal(uint64(3), ap1_cNonce1)
	ap1_pNonce1, _ = ap1.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(3), ap1_pNonce1)
	ap1_pBalance1, _ = ap1.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(160).Uint64(), ap1_pBalance1.Uint64())
	// Addr2
	ap1_cNonce2, _ = ap1.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(3), ap1_cNonce2)
	ap1_pNonce2, _ = ap1.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(3), ap1_pNonce2)
	ap1_pBalance2, _ = ap1.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(140).Uint64(), ap1_pBalance2.Uint64())
	// Addr3
	ap1_cNonce3, _ = ap1.getConfirmedNonce(addr3.RawAddress)
	assert.Equal(uint64(4), ap1_cNonce3)
	ap1_pNonce3, _ = ap1.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(4), ap1_pNonce3)
	ap1_pBalance3, _ = ap1.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(0).Uint64(), ap1_pBalance3.Uint64())
	// ap2
	// Addr1
	ap2_cNonce1, _ = ap2.getConfirmedNonce(addr1.RawAddress)
	assert.Equal(uint64(5), ap2_cNonce1)
	ap2_pNonce1, _ = ap2.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(5), ap2_pNonce1)
	ap2_pBalance1, _ = ap2.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(40).Uint64(), ap2_pBalance1.Uint64())
	// Addr2
	ap2_cNonce2, _ = ap2.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(4), ap2_cNonce2)
	ap2_pNonce2, _ = ap2.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(5), ap2_pNonce2)
	ap2_pBalance2, _ = ap2.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(10).Uint64(), ap2_pBalance2.Uint64())
	// Addr3
	ap2_cNonce3, _ = ap2.getConfirmedNonce(addr3.RawAddress)
	assert.Equal(uint64(2), ap2_cNonce3)
	ap2_pNonce3, _ = ap2.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(4), ap2_pNonce3)
	ap2_pBalance3, _ = ap2.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(180).Uint64(), ap2_pBalance3.Uint64())
	// Let ap2 be BP's actpool
	pickedTxs, _ = ap2.PickTxs()
	// ap2 commits update of balance to trie
	for _, txs := range pickedTxs {
		err := ap2.sf.UpdateStatesWithTransfer(txs)
		assert.Nil(err)
	}
	// ap2 commits update of nonce to trie
	ap2.sf.SetNonce(addr1.RawAddress, ap2_cNonce1)
	ap2.sf.SetNonce(addr2.RawAddress, ap2_cNonce2)
	ap2.sf.SetNonce(addr3.RawAddress, ap2_cNonce3)
	//Reset
	ap1.Reset()
	ap2.Reset()
	// Check confirmed nonce, pending nonce, and pending balance after resetting actpool for each account
	// ap1
	// Addr1
	ap1_cNonce1, _ = ap1.getConfirmedNonce(addr1.RawAddress)
	assert.Equal(uint64(5), ap1_cNonce1)
	ap1_pNonce1, _ = ap1.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(5), ap1_pNonce1)
	ap1_pBalance1, _ = ap1.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(130).Uint64(), ap1_pBalance1.Uint64())
	// Addr2
	ap1_cNonce2, _ = ap1.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(4), ap1_cNonce2)
	ap1_pNonce2, _ = ap1.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(4), ap1_pNonce2)
	ap1_pBalance2, _ = ap1.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(180).Uint64(), ap1_pBalance2.Uint64())
	// Addr3
	ap1_cNonce3, _ = ap1.getConfirmedNonce(addr3.RawAddress)
	assert.Equal(uint64(4), ap1_cNonce3)
	ap1_pNonce3, _ = ap1.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(4), ap1_pNonce3)
	ap1_pBalance3, _ = ap1.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(110).Uint64(), ap1_pBalance3.Uint64())
	// ap2
	// Addr1
	ap2_cNonce1, _ = ap2.getConfirmedNonce(addr1.RawAddress)
	assert.Equal(uint64(5), ap2_cNonce1)
	ap2_pNonce1, _ = ap2.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(5), ap2_pNonce1)
	ap2_pBalance1, _ = ap2.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(130).Uint64(), ap2_pBalance1.Uint64())
	// Addr2
	ap2_cNonce2, _ = ap2.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(5), ap2_cNonce2)
	ap2_pNonce2, _ = ap2.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(5), ap2_pNonce2)
	ap2_pBalance2, _ = ap2.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(130).Uint64(), ap2_pBalance2.Uint64())
	// Addr3
	ap2_cNonce3, _ = ap2.getConfirmedNonce(addr3.RawAddress)
	assert.Equal(uint64(3), ap2_cNonce3)
	ap2_pNonce3, _ = ap2.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(4), ap2_pNonce3)
	ap2_pBalance3, _ = ap2.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(90).Uint64(), ap2_pBalance3.Uint64())
}

// Helper function to return the correct confirmed nonce just in case of empty queue
func (ap *actPool) getConfirmedNonce(addr string) (uint64, error) {
	if queue, ok := ap.accountTxs[addr]; ok {
		return queue.ConfirmedNonce(), nil
	}
	return ap.sf.Nonce(addr)
}

// Helper function to return the correct pending nonce just in case of empty queue
func (ap *actPool) getPendingNonce(addr string) (uint64, error) {
	if queue, ok := ap.accountTxs[addr]; ok {
		return queue.PendingNonce(), nil
	}
	return ap.sf.Nonce(addr)
}

// Helper function to return the correct pending balance just in case of empty queue
func (ap *actPool) getPendingBalance(addr string) (*big.Int, error) {
	if queue, ok := ap.accountTxs[addr]; ok {
		return queue.PendingBalance(), nil
	}
	return ap.sf.Balance(addr)
}
