// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package txpool

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/statefactory"
	"github.com/iotexproject/iotex-core/test/mock/mock_statefactory"
	"github.com/iotexproject/iotex-core/trie"
)

var (
	isTestnet = true
	chainid   = []byte{0x00, 0x00, 0x00, 0x01}
)

func TestActPool_validateTsf(t *testing.T) {
	assert := assert.New(t)
	l := logger.Logger().Level(zerolog.DebugLevel)
	logger.SetLogger(&l)
	// Create one dummy iotex address
	pubK := "09d8c6fc6f5cb0a03df112da90486fad7cdece1501aaab658551f8afbe7f59ee"
	addr, _ := iotxaddress.GetAddress(decodeHash(pubK), isTestnet, chainid)
	tr, _ := trie.NewTrie("", true)
	assert.NotNil(tr)
	sf := statefactory.NewStateFactory(tr)
	assert.NotNil(sf)
	sf.CreateState(addr.RawAddress, uint64(100))
	ap := newActPool(sf)
	assert.NotNil(ap)
	// Case I: Oversized Data
	payload := []byte{}
	tmpPayload := [32769]byte{}
	payload = tmpPayload[:]
	tsf := action.Transfer{Payload: payload}
	err := ap.validateTsf(&tsf)
	assert.Equal(ErrActPool, errors.Cause(err))
	// Case II: Negative Amount
	tsf = action.Transfer{Amount: big.NewInt(-100)}
	err = ap.validateTsf(&tsf)
	assert.NotNil(ErrBalance, errors.Cause(err))
	// Case III: Nonce is too low
	prevTsf := action.Transfer{Sender: addr.RawAddress, Recipient: addr.RawAddress, Nonce: uint64(1), Amount: big.NewInt(50)}
	ap.AddTsf(&prevTsf)
	err = ap.sf.CommitStateChanges(0, []*action.Transfer{&prevTsf}, nil)
	assert.Nil(err)
	ap.Reset()
	tsf = action.Transfer{Sender: addr.RawAddress, Recipient: addr.RawAddress, Nonce: uint64(1), Amount: big.NewInt(60)}
	err = ap.validateTsf(&tsf)
	assert.Equal(ErrNonce, errors.Cause(err))
}

func TestActPool_AddTsf(t *testing.T) {
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
	tr, _ := trie.NewTrie("", true)
	assert.NotNil(tr)
	sf := statefactory.NewStateFactory(tr)
	assert.NotNil(sf)
	sf.CreateState(addr1.RawAddress, uint64(100))
	sf.CreateState(addr2.RawAddress, uint64(10))
	// Create actpool
	ap := newActPool(sf)
	assert.NotNil(ap)
	// Test actpool status after adding a sequence of Tsfs: need to check confirmed nonce, pending nonce, and pending balance
	tsf1 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(1), Amount: big.NewInt(10)}
	tsf2 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(2), Amount: big.NewInt(20)}
	tsf3 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(3), Amount: big.NewInt(30)}
	tsf4 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(4), Amount: big.NewInt(40)}
	tsf5 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(5), Amount: big.NewInt(50)}
	tsf6 := action.Transfer{Sender: addr2.RawAddress, Nonce: uint64(1), Amount: big.NewInt(5)}
	tsf7 := action.Transfer{Sender: addr2.RawAddress, Nonce: uint64(3), Amount: big.NewInt(1)}
	tsf8 := action.Transfer{Sender: addr2.RawAddress, Nonce: uint64(4), Amount: big.NewInt(5)}

	ap.AddTsf(&tsf1)
	ap.AddTsf(&tsf2)
	ap.AddTsf(&tsf3)
	ap.AddTsf(&tsf4)
	ap.AddTsf(&tsf5)
	ap.AddTsf(&tsf6)
	ap.AddTsf(&tsf7)
	ap.AddTsf(&tsf8)

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

	tsf9 := action.Transfer{Sender: addr2.RawAddress, Nonce: uint64(2), Amount: big.NewInt(3)}
	ap.AddTsf(&tsf9)
	cNonce2, _ = ap.getConfirmedNonce(addr2.RawAddress)
	assert.Equal(uint64(4), cNonce2)
	pBalance2, _ = ap.getPendingBalance(addr2.RawAddress)
	assert.Equal(uint64(1), pBalance2.Uint64())
	pNonce2, _ = ap.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(5), pNonce2)
	// Error Case Handling
	// Case I: Tsf already exists in pool
	err := ap.AddTsf(&tsf1)
	assert.Equal(fmt.Errorf("existed transfer: %x", tsf1.Hash()), err)
	// Case II: Pool space is full
	mockSF := mock_statefactory.NewMockStateFactory(ctrl)
	ap2 := newActPool(mockSF)
	assert.NotNil(ap2)
	for i := 0; i < GlobalSlots; i++ {
		nTsf := action.Transfer{Amount: big.NewInt(int64(i))}
		ap2.allTsfs[nTsf.Hash()] = &nTsf
	}
	mockSF.EXPECT().Nonce(gomock.Any()).Times(1).Return(uint64(0), nil)
	err = ap2.AddTsf(&tsf1)
	assert.Equal(ErrActPool, errors.Cause(err))
	// Case III: Nonce already exists
	replaceTsf := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(1), Amount: big.NewInt(1)}
	err = ap.AddTsf(&replaceTsf)
	assert.Equal(ErrNonce, errors.Cause(err))
	// Case IV: Queue space is full
	for i := 6; i <= AccountSlots; i++ {
		tsf := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(i), Amount: big.NewInt(1)}
		err := ap.AddTsf(&tsf)
		assert.Nil(err)
	}
	outOfBoundsTsf := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(AccountSlots + 1), Amount: big.NewInt(1)}
	err = ap.AddTsf(&outOfBoundsTsf)
	assert.Equal(ErrActPool, errors.Cause(err))
}

func TestActPool_PickTsfs(t *testing.T) {
	assert := assert.New(t)
	l := logger.Logger().Level(zerolog.DebugLevel)
	logger.SetLogger(&l)
	// Create two dummy iotex addresses
	pubK1 := "09d8c6fc6f5cb0a03df112da90486fad7cdece1501aaab658551f8afbe7f59ee"
	addr1, _ := iotxaddress.GetAddress(decodeHash(pubK1), isTestnet, chainid)
	pubK2 := "247a92febf1daac9c9d1a0f5224bb61eb14b2f291d10e01daa0aa5cf17d3f83a"
	addr2, _ := iotxaddress.GetAddress(decodeHash(pubK2), isTestnet, chainid)
	tr, _ := trie.NewTrie("", true)
	assert.NotNil(tr)
	sf := statefactory.NewStateFactory(tr)
	assert.NotNil(sf)
	sf.CreateState(addr1.RawAddress, uint64(100))
	sf.CreateState(addr2.RawAddress, uint64(10))
	// Create actpool
	ap := NewActPool(sf, nil)
	assert.NotNil(ap)

	tsf1 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(1), Amount: big.NewInt(10)}
	tsf2 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(2), Amount: big.NewInt(20)}
	tsf3 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(3), Amount: big.NewInt(30)}
	tsf4 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(4), Amount: big.NewInt(40)}
	tsf5 := action.Transfer{Sender: addr1.RawAddress, Nonce: uint64(5), Amount: big.NewInt(50)}
	tsf6 := action.Transfer{Sender: addr2.RawAddress, Nonce: uint64(2), Amount: big.NewInt(5)}
	tsf7 := action.Transfer{Sender: addr2.RawAddress, Nonce: uint64(3), Amount: big.NewInt(1)}
	tsf8 := action.Transfer{Sender: addr2.RawAddress, Nonce: uint64(5), Amount: big.NewInt(5)}

	ap.AddTsf(&tsf1)
	ap.AddTsf(&tsf2)
	ap.AddTsf(&tsf3)
	ap.AddTsf(&tsf4)
	ap.AddTsf(&tsf5)
	ap.AddTsf(&tsf6)
	ap.AddTsf(&tsf7)
	ap.AddTsf(&tsf8)

	pending := ap.PickTsfs()
	assert.Equal([]*action.Transfer{&tsf1, &tsf2, &tsf3, &tsf4}, pending)
}

func TestActPool_removeCommittedTsfs(t *testing.T) {
	assert := assert.New(t)
	l := logger.Logger().Level(zerolog.DebugLevel)
	logger.SetLogger(&l)
	// Create one dummy iotex address
	pubK := "09d8c6fc6f5cb0a03df112da90486fad7cdece1501aaab658551f8afbe7f59ee"
	addr, _ := iotxaddress.GetAddress(decodeHash(pubK), isTestnet, chainid)
	tr, _ := trie.NewTrie("", true)
	assert.NotNil(tr)
	sf := statefactory.NewStateFactory(tr)
	assert.NotNil(sf)
	sf.CreateState(addr.RawAddress, uint64(100))
	// Create actpool
	ap := newActPool(sf)
	assert.NotNil(ap)

	tsf1 := action.Transfer{Sender: addr.RawAddress, Recipient: addr.RawAddress, Nonce: uint64(1), Amount: big.NewInt(10)}
	tsf2 := action.Transfer{Sender: addr.RawAddress, Recipient: addr.RawAddress, Nonce: uint64(2), Amount: big.NewInt(20)}
	tsf3 := action.Transfer{Sender: addr.RawAddress, Recipient: addr.RawAddress, Nonce: uint64(3), Amount: big.NewInt(30)}
	tsf4 := action.Transfer{Sender: addr.RawAddress, Recipient: addr.RawAddress, Nonce: uint64(4), Amount: big.NewInt(40)}

	ap.AddTsf(&tsf1)
	ap.AddTsf(&tsf2)
	ap.AddTsf(&tsf3)
	ap.AddTsf(&tsf4)

	assert.Equal(4, len(ap.allTsfs))
	assert.NotNil(ap.accountTsfs[addr.RawAddress])
	err := ap.sf.CommitStateChanges(0, []*action.Transfer{&tsf1, &tsf2, &tsf3, &tsf4}, nil)
	assert.Nil(err)
	ap.removeCommittedTsfs()
	assert.Equal(0, len(ap.allTsfs))
	assert.Nil(ap.accountTsfs[addr.RawAddress])
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

	tr, _ := trie.NewTrie("", true)
	assert.NotNil(tr)
	sf := statefactory.NewStateFactory(tr)
	assert.NotNil(sf)
	sf.CreateState(addr1.RawAddress, uint64(100))
	sf.CreateState(addr2.RawAddress, uint64(200))
	sf.CreateState(addr3.RawAddress, uint64(300))

	ap1 := newActPool(sf)
	assert.NotNil(ap1)
	ap2 := newActPool(sf)
	assert.NotNil(ap2)

	// Tsfs to be added to ap1
	tsf1 := action.Transfer{Sender: addr1.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(1), Amount: big.NewInt(50)}
	tsf2 := action.Transfer{Sender: addr1.RawAddress, Recipient: addr3.RawAddress, Nonce: uint64(2), Amount: big.NewInt(30)}
	tsf3 := action.Transfer{Sender: addr1.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(3), Amount: big.NewInt(60)}
	tsf4 := action.Transfer{Sender: addr2.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(1), Amount: big.NewInt(100)}
	tsf5 := action.Transfer{Sender: addr2.RawAddress, Recipient: addr3.RawAddress, Nonce: uint64(2), Amount: big.NewInt(50)}
	tsf6 := action.Transfer{Sender: addr2.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(3), Amount: big.NewInt(60)}
	tsf7 := action.Transfer{Sender: addr3.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(1), Amount: big.NewInt(100)}
	tsf8 := action.Transfer{Sender: addr3.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(2), Amount: big.NewInt(100)}
	tsf9 := action.Transfer{Sender: addr3.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(4), Amount: big.NewInt(100)}

	ap1.AddTsf(&tsf1)
	ap1.AddTsf(&tsf2)
	ap1.AddTsf(&tsf3)
	ap1.AddTsf(&tsf4)
	ap1.AddTsf(&tsf5)
	ap1.AddTsf(&tsf6)
	ap1.AddTsf(&tsf7)
	ap1.AddTsf(&tsf8)
	ap1.AddTsf(&tsf9)
	// Tsfs to be added to ap2 only
	tsf10 := action.Transfer{Sender: addr1.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(3), Amount: big.NewInt(20)}
	tsf11 := action.Transfer{Sender: addr1.RawAddress, Recipient: addr3.RawAddress, Nonce: uint64(4), Amount: big.NewInt(10)}
	tsf12 := action.Transfer{Sender: addr2.RawAddress, Recipient: addr3.RawAddress, Nonce: uint64(2), Amount: big.NewInt(70)}
	tsf13 := action.Transfer{Sender: addr3.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(1), Amount: big.NewInt(200)}
	tsf14 := action.Transfer{Sender: addr3.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(2), Amount: big.NewInt(50)}

	ap2.AddTsf(&tsf1)
	ap2.AddTsf(&tsf2)
	ap2.AddTsf(&tsf10)
	ap2.AddTsf(&tsf11)
	ap2.AddTsf(&tsf4)
	ap2.AddTsf(&tsf12)
	ap2.AddTsf(&tsf13)
	ap2.AddTsf(&tsf14)
	ap2.AddTsf(&tsf9)
	// Check confirmed nonce, pending nonce, and pending balance after adding Tsfs above for each account
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
	pickedTsfs := ap1.PickTsfs()
	// ap1 commits update of balance and nonce to trie
	err := ap1.sf.CommitStateChanges(0, pickedTsfs, nil)
	assert.Nil(err)
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
	// Add more Tsfs after resetting
	// Tsfs To be added to ap1 only
	tsf15 := action.Transfer{Sender: addr3.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(3), Amount: big.NewInt(80)}
	// Tsfs To be added to ap2 only
	tsf16 := action.Transfer{Sender: addr1.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(5), Amount: big.NewInt(150)}
	tsf17 := action.Transfer{Sender: addr2.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(3), Amount: big.NewInt(90)}
	tsf18 := action.Transfer{Sender: addr2.RawAddress, Recipient: addr3.RawAddress, Nonce: uint64(4), Amount: big.NewInt(100)}
	tsf19 := action.Transfer{Sender: addr2.RawAddress, Recipient: addr1.RawAddress, Nonce: uint64(5), Amount: big.NewInt(50)}
	tsf20 := action.Transfer{Sender: addr3.RawAddress, Recipient: addr2.RawAddress, Nonce: uint64(3), Amount: big.NewInt(200)}

	ap1.AddTsf(&tsf15)
	ap2.AddTsf(&tsf16)
	ap2.AddTsf(&tsf17)
	ap2.AddTsf(&tsf18)
	ap2.AddTsf(&tsf19)
	ap2.AddTsf(&tsf20)
	// Check confirmed nonce, pending nonce, and pending balance after adding Tsfs above for each account
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
	pickedTsfs = ap2.PickTsfs()
	// ap2 commits update of balance and nonce to trie
	err = ap2.sf.CommitStateChanges(0, pickedTsfs, nil)
	assert.Nil(err)
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

// Helper function to create an instance of actPool
func newActPool(sf statefactory.StateFactory) *actPool {
	return &actPool{
		sf:          sf,
		accountTsfs: make(map[string]TsfQueue),
		allTsfs:     make(map[common.Hash32B]*action.Transfer),
	}
}

// Helper function to return the correct confirmed nonce just in case of empty queue
func (ap *actPool) getConfirmedNonce(addr string) (uint64, error) {
	if queue, ok := ap.accountTsfs[addr]; ok {
		return queue.ConfirmedNonce(), nil
	}
	committedNonce, err := ap.sf.Nonce(addr)
	confirmedNonce := committedNonce + 1
	return confirmedNonce, err
}

// Helper function to return the correct pending nonce just in case of empty queue
func (ap *actPool) getPendingNonce(addr string) (uint64, error) {
	if queue, ok := ap.accountTsfs[addr]; ok {
		return queue.PendingNonce(), nil
	}
	committedNonce, err := ap.sf.Nonce(addr)
	pendingNonce := committedNonce + 1
	return pendingNonce, err
}

// Helper function to return the correct pending balance just in case of empty queue
func (ap *actPool) getPendingBalance(addr string) (*big.Int, error) {
	if queue, ok := ap.accountTsfs[addr]; ok {
		return queue.PendingBalance(), nil
	}
	return ap.sf.Balance(addr)
}
