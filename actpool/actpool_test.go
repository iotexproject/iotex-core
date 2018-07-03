// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	pb "github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_state"
	"github.com/iotexproject/iotex-core/test/util"
)

const (
	pubkeyA = "2c9ccbeb9ee91271f7e5c2103753be9c9edff847e1a51227df6a6b0765f31a4b424e84027b44a663950f013a88b8fd8cdc53b1eda1d4b73f9d9dc12546c8c87d68ff1435a0f8a006"
	prikeyA = "b5affb30846a00ef5aa39b57f913d70cd8cf6badd587239863cb67feacf6b9f30c34e800"
	pubkeyB = "881504d84a0659e14dcba59f24a98e71cda55b139615342668840c64678f1514941bbd053c7492fb9b719e6050cfa972efa491b79e11a1713824dda5f638fc0d9fa1b68be3c0f905"
	prikeyB = "b89c1ec0fb5b192c8bb8f6fcf9a871e4a67ef462f40d2b8ff426da1d1eaedd9696dc9d00"
	pubkeyC = "252fc7bc9a993b68dd7b13a00213c9cf4befe80da49940c52220f93c7147771ba2d783045cf0fbf2a86b32a62848befb96c0f38c0487a5ccc806ff28bb06d9faf803b93dda107003"
	prikeyC = "3e05de562a27fb6e25ac23ff8bcaa1ada0c253fa8ff7c6d15308f65d06b6990f64ee9601"
	pubkeyD = "29aa28cc21c3ee3cc658d3a322997ceb8d5d352f45d052192d3ab57cd196d3375af558067f5a2cfe5fc65d5249cc07f991bab683468382a3acaa4c8b7af35156b46aeda00620f307"
	prikeyD = "d4b7b441382751d9a1955152b46a69f3c9f9559c6205757af928f5181ff207060d0dab00"
	pubkeyE = "64dc2d5f445a78b884527252a3dba1f72f52251c97ec213dda99868882024d4d1442f100c8f1f833d0c687871a959ee97665dea24de1a627cce6c970d9db5859da9e4295bb602e04"
	prikeyE = "53a827f7c5b4b4040b22ae9b12fcaa234e8362fa022480f50b8643981806ed67c7f77a00"
)

const (
	maxNumActPerPool = 8192
	maxNumActPerAcct = 256
)

var (
	addr1 = util.ConstructAddress(pubkeyA, prikeyA)
	addr2 = util.ConstructAddress(pubkeyB, prikeyB)
	addr3 = util.ConstructAddress(pubkeyC, prikeyC)
	addr4 = util.ConstructAddress(pubkeyD, prikeyD)
	addr5 = util.ConstructAddress(pubkeyE, prikeyE)
)

func TestActPool_validateTsf(t *testing.T) {
	assert := assert.New(t)
	l := logger.Logger().Level(zerolog.DebugLevel)
	logger.SetLogger(&l)
	sf, err := state.NewFactory(nil, state.InMemTrieOption())
	assert.NotNil(sf)
	assert.Nil(err)
	sf.CreateState(addr1.RawAddress, uint64(100))
	apConfig := config.ActPool{maxNumActPerPool, maxNumActPerAcct}
	Ap, err := NewActPool(sf, apConfig)
	assert.Nil(err)
	ap, ok := Ap.(*actPool)
	require.True(t, ok)
	// Case I: Coinbase Transfer
	coinbaseTsf := action.Transfer{IsCoinbase: true}
	err = ap.validateTsf(&coinbaseTsf)
	assert.Equal(ErrTransfer, errors.Cause(err))
	// Case II: Oversized Data
	tmpPayload := [32769]byte{}
	payload := tmpPayload[:]
	tsf := action.Transfer{Payload: payload}
	err = ap.validateTsf(&tsf)
	assert.Equal(ErrActPool, errors.Cause(err))
	// Case III: Negative Amount
	tsf = action.Transfer{Amount: big.NewInt(-100)}
	err = ap.validateTsf(&tsf)
	assert.NotNil(ErrBalance, errors.Cause(err))
	// Case IV: Signature Verification Fails
	unsignedTsf := action.NewTransfer(uint64(1), big.NewInt(1), addr1.RawAddress, addr1.RawAddress)
	err = ap.validateTsf(unsignedTsf)
	assert.Equal(action.ErrTransferError, errors.Cause(err))
	// Case V: Nonce is too low
	prevTsf, _ := signedTransfer(addr1, addr1, uint64(1), big.NewInt(50))
	ap.AddTsf(prevTsf)
	err = ap.sf.CommitStateChanges(0, []*action.Transfer{prevTsf}, nil)
	assert.Nil(err)
	ap.Reset()
	nTsf, _ := signedTransfer(addr1, addr1, uint64(1), big.NewInt(60))
	err = ap.validateTsf(nTsf)
	assert.Equal(ErrNonce, errors.Cause(err))
}

func TestActPool_validateVote(t *testing.T) {
	assert := assert.New(t)
	l := logger.Logger().Level(zerolog.DebugLevel)
	logger.SetLogger(&l)
	sf, err := state.NewFactory(nil, state.InMemTrieOption())
	assert.NotNil(sf)
	assert.Nil(err)
	sf.CreateState(addr1.RawAddress, uint64(100))
	apConfig := config.ActPool{maxNumActPerPool, maxNumActPerAcct}
	Ap, err := NewActPool(sf, apConfig)
	assert.Nil(err)
	ap, ok := Ap.(*actPool)
	require.True(t, ok)
	// Case I: Oversized Data
	tmpSelfPubKey := [32769]byte{}
	selfPubKey := tmpSelfPubKey[:]
	vote := action.Vote{&pb.VotePb{SelfPubkey: selfPubKey}}
	err = ap.validateVote(&vote)
	assert.Equal(ErrActPool, errors.Cause(err))
	// Case II: Signature Verification Fails
	unsignedVote := action.NewVote(1, addr1.PublicKey, addr2.PublicKey)
	err = ap.validateVote(unsignedVote)
	assert.Equal(action.ErrVoteError, errors.Cause(err))
	// Case III: Nonce is too low
	prevTsf, _ := signedTransfer(addr1, addr1, uint64(1), big.NewInt(50))
	ap.AddTsf(prevTsf)
	err = ap.sf.CommitStateChanges(0, []*action.Transfer{prevTsf}, nil)
	assert.Nil(err)
	ap.Reset()
	nVote, _ := signedVote(addr1, addr1, uint64(1))
	err = ap.validateVote(nVote)
	assert.Equal(ErrNonce, errors.Cause(err))
}

func TestActPool_AddActs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	assert := assert.New(t)
	l := logger.Logger().Level(zerolog.DebugLevel)
	logger.SetLogger(&l)
	sf, err := state.NewFactory(nil, state.InMemTrieOption())
	assert.NotNil(sf)
	assert.Nil(err)
	sf.CreateState(addr1.RawAddress, uint64(100))
	sf.CreateState(addr2.RawAddress, uint64(10))
	// Create actpool
	apConfig := config.ActPool{maxNumActPerPool, maxNumActPerAcct}
	Ap, err := NewActPool(sf, apConfig)
	assert.Nil(err)
	ap, ok := Ap.(*actPool)
	require.True(t, ok)
	// Test actpool status after adding a sequence of Tsfs/votes: need to check confirmed nonce, pending nonce, and pending balance
	tsf1, _ := signedTransfer(addr1, addr1, uint64(1), big.NewInt(10))
	tsf2, _ := signedTransfer(addr1, addr1, uint64(2), big.NewInt(20))
	tsf3, _ := signedTransfer(addr1, addr1, uint64(3), big.NewInt(30))
	vote4, _ := signedVote(addr1, addr1, uint64(4))
	tsf5, _ := signedTransfer(addr1, addr1, uint64(5), big.NewInt(50))
	tsf6, _ := signedTransfer(addr2, addr2, uint64(1), big.NewInt(5))
	tsf7, _ := signedTransfer(addr2, addr2, uint64(3), big.NewInt(1))
	tsf8, _ := signedTransfer(addr2, addr2, uint64(4), big.NewInt(5))

	ap.AddTsf(tsf1)
	ap.AddTsf(tsf2)
	ap.AddTsf(tsf3)
	ap.AddVote(vote4)
	ap.AddTsf(tsf5)
	ap.AddTsf(tsf6)
	ap.AddTsf(tsf7)
	ap.AddTsf(tsf8)

	pBalance1, _ := ap.getPendingBalance(addr1.RawAddress)
	assert.Equal(uint64(40), pBalance1.Uint64())
	pNonce1, _ := ap.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(5), pNonce1)

	pBalance2, _ := ap.getPendingBalance(addr2.RawAddress)
	assert.Equal(uint64(5), pBalance2.Uint64())
	pNonce2, _ := ap.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(2), pNonce2)

	tsf9, _ := signedTransfer(addr2, addr2, uint64(2), big.NewInt(3))
	ap.AddTsf(tsf9)
	pBalance2, _ = ap.getPendingBalance(addr2.RawAddress)
	assert.Equal(uint64(1), pBalance2.Uint64())
	pNonce2, _ = ap.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(4), pNonce2)
	// Error Case Handling
	// Case I: Action already exists in pool
	err = ap.AddTsf(tsf1)
	assert.Equal(fmt.Errorf("existed transfer: %x", tsf1.Hash()), err)
	err = ap.AddVote(vote4)
	assert.Equal(fmt.Errorf("existed vote: %x", vote4.Hash()), err)
	// Case II: Pool space is full
	mockSF := mock_state.NewMockFactory(ctrl)
	Ap2, err := NewActPool(mockSF, apConfig)
	assert.Nil(err)
	ap2, ok := Ap2.(*actPool)
	require.True(t, ok)
	for i := uint64(0); i < ap2.maxNumActPerPool; i++ {
		nTsf := action.Transfer{Amount: big.NewInt(int64(i))}
		nAction := &pb.ActionPb{&pb.ActionPb_Transfer{nTsf.ConvertToTransferPb()}}
		ap2.allActions[nTsf.Hash()] = nAction
	}
	mockSF.EXPECT().Nonce(gomock.Any()).Times(2).Return(uint64(0), nil)
	err = ap2.AddTsf(tsf1)
	assert.Equal(ErrActPool, errors.Cause(err))
	err = ap2.AddVote(vote4)
	assert.Equal(ErrActPool, errors.Cause(err))
	// Case III: Nonce already exists
	replaceTsf, _ := signedTransfer(addr1, addr2, uint64(1), big.NewInt(1))
	err = ap.AddTsf(replaceTsf)
	assert.Equal(ErrNonce, errors.Cause(err))
	replaceVote, _ := signedVote(addr1, addr2, uint64(4))
	err = ap.AddVote(replaceVote)
	assert.Equal(ErrNonce, errors.Cause(err))
	// Case IV: Nonce is too large
	outOfBoundsTsf, _ := signedTransfer(addr1, addr1, uint64(ap.maxNumActPerAcct+1), big.NewInt(1))
	err = ap.AddTsf(outOfBoundsTsf)
	assert.Equal(ErrNonce, errors.Cause(err))
	// Case V: Insufficient balance
	overBalTsf, _ := signedTransfer(addr2, addr2, uint64(4), big.NewInt(20))
	err = ap.AddTsf(overBalTsf)
	assert.Equal(ErrBalance, errors.Cause(err))
}

func TestActPool_PickActs(t *testing.T) {
	assert := assert.New(t)
	l := logger.Logger().Level(zerolog.DebugLevel)
	logger.SetLogger(&l)
	sf, err := state.NewFactory(nil, state.InMemTrieOption())
	assert.NotNil(sf)
	assert.Nil(err)
	sf.CreateState(addr1.RawAddress, uint64(100))
	sf.CreateState(addr2.RawAddress, uint64(10))
	// Create actpool
	apConfig := config.ActPool{maxNumActPerPool, maxNumActPerAcct}
	Ap, err := NewActPool(sf, apConfig)
	assert.Nil(err)
	ap, ok := Ap.(*actPool)
	require.True(t, ok)

	tsf1, _ := signedTransfer(addr1, addr1, uint64(1), big.NewInt(10))
	tsf2, _ := signedTransfer(addr1, addr1, uint64(2), big.NewInt(20))
	tsf3, _ := signedTransfer(addr1, addr1, uint64(3), big.NewInt(30))
	tsf4, _ := signedTransfer(addr1, addr1, uint64(4), big.NewInt(40))
	tsf5, _ := signedTransfer(addr1, addr1, uint64(5), big.NewInt(50))
	vote6, _ := signedVote(addr1, addr1, uint64(6))
	vote7, _ := signedVote(addr2, addr2, uint64(1))
	tsf8, _ := signedTransfer(addr2, addr2, uint64(3), big.NewInt(5))
	tsf9, _ := signedTransfer(addr2, addr2, uint64(4), big.NewInt(1))
	tsf10, _ := signedTransfer(addr2, addr2, uint64(5), big.NewInt(5))

	ap.AddTsf(tsf1)
	ap.AddTsf(tsf2)
	ap.AddTsf(tsf3)
	ap.AddTsf(tsf4)
	ap.AddTsf(tsf5)
	ap.AddVote(vote6)
	ap.AddVote(vote7)
	ap.AddTsf(tsf8)
	ap.AddTsf(tsf9)
	ap.AddTsf(tsf10)

	pickedTsfs, pickedVotes := ap.PickActs()
	assert.Equal([]*action.Transfer{tsf1, tsf2, tsf3, tsf4}, pickedTsfs)
	assert.Equal([]*action.Vote{vote7}, pickedVotes)
}

func TestActPool_removeConfirmedActs(t *testing.T) {
	assert := assert.New(t)
	l := logger.Logger().Level(zerolog.DebugLevel)
	logger.SetLogger(&l)
	sf, err := state.NewFactory(nil, state.InMemTrieOption())
	assert.NotNil(sf)
	assert.Nil(err)
	sf.CreateState(addr1.RawAddress, uint64(100))
	// Create actpool
	apConfig := config.ActPool{maxNumActPerPool, maxNumActPerAcct}
	Ap, err := NewActPool(sf, apConfig)
	assert.Nil(err)
	ap, ok := Ap.(*actPool)
	require.True(t, ok)

	tsf1, _ := signedTransfer(addr1, addr1, uint64(1), big.NewInt(10))
	tsf2, _ := signedTransfer(addr1, addr1, uint64(2), big.NewInt(20))
	tsf3, _ := signedTransfer(addr1, addr1, uint64(3), big.NewInt(30))
	vote4, _ := signedVote(addr1, addr1, uint64(4))

	ap.AddTsf(tsf1)
	ap.AddTsf(tsf2)
	ap.AddTsf(tsf3)
	ap.AddVote(vote4)

	assert.Equal(4, len(ap.allActions))
	assert.NotNil(ap.accountActs[addr1.RawAddress])
	err = ap.sf.CommitStateChanges(0, []*action.Transfer{tsf1, tsf2, tsf3}, []*action.Vote{vote4})
	assert.Nil(err)
	ap.removeConfirmedActs()
	assert.Equal(0, len(ap.allActions))
	assert.Nil(ap.accountActs[addr1.RawAddress])
}

func TestActPool_Reset(t *testing.T) {
	assert := assert.New(t)
	l := logger.Logger().Level(zerolog.DebugLevel)
	logger.SetLogger(&l)

	sf, err := state.NewFactory(nil, state.InMemTrieOption())
	assert.NotNil(sf)
	assert.Nil(err)
	sf.CreateState(addr1.RawAddress, uint64(100))
	sf.CreateState(addr2.RawAddress, uint64(200))
	sf.CreateState(addr3.RawAddress, uint64(300))

	apConfig := config.ActPool{maxNumActPerPool, maxNumActPerAcct}
	Ap1, err := NewActPool(sf, apConfig)
	assert.Nil(err)
	ap1, ok := Ap1.(*actPool)
	require.True(t, ok)
	Ap2, err := NewActPool(sf, apConfig)
	assert.Nil(err)
	ap2, ok := Ap2.(*actPool)
	require.True(t, ok)

	// Tsfs to be added to ap1
	tsf1, _ := signedTransfer(addr1, addr2, uint64(1), big.NewInt(50))
	tsf2, _ := signedTransfer(addr1, addr3, uint64(2), big.NewInt(30))
	tsf3, _ := signedTransfer(addr1, addr2, uint64(3), big.NewInt(60))
	tsf4, _ := signedTransfer(addr2, addr1, uint64(1), big.NewInt(100))
	tsf5, _ := signedTransfer(addr2, addr3, uint64(2), big.NewInt(50))
	tsf6, _ := signedTransfer(addr2, addr1, uint64(3), big.NewInt(60))
	tsf7, _ := signedTransfer(addr3, addr1, uint64(1), big.NewInt(100))
	tsf8, _ := signedTransfer(addr3, addr2, uint64(2), big.NewInt(100))
	tsf9, _ := signedTransfer(addr3, addr1, uint64(4), big.NewInt(100))

	ap1.AddTsf(tsf1)
	ap1.AddTsf(tsf2)
	ap1.AddTsf(tsf3)
	ap1.AddTsf(tsf4)
	ap1.AddTsf(tsf5)
	ap1.AddTsf(tsf6)
	ap1.AddTsf(tsf7)
	ap1.AddTsf(tsf8)
	ap1.AddTsf(tsf9)
	// Tsfs to be added to ap2 only
	tsf10, _ := signedTransfer(addr1, addr2, uint64(3), big.NewInt(20))
	tsf11, _ := signedTransfer(addr1, addr3, uint64(4), big.NewInt(10))
	tsf12, _ := signedTransfer(addr2, addr3, uint64(2), big.NewInt(70))
	tsf13, _ := signedTransfer(addr3, addr1, uint64(1), big.NewInt(200))
	tsf14, _ := signedTransfer(addr3, addr2, uint64(2), big.NewInt(50))

	ap2.AddTsf(tsf1)
	ap2.AddTsf(tsf2)
	ap2.AddTsf(tsf10)
	ap2.AddTsf(tsf11)
	ap2.AddTsf(tsf4)
	ap2.AddTsf(tsf12)
	ap2.AddTsf(tsf13)
	ap2.AddTsf(tsf14)
	ap2.AddTsf(tsf9)
	// Check confirmed nonce, pending nonce, and pending balance after adding Tsfs above for each account
	// ap1
	// Addr1
	ap1PNonce1, _ := ap1.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(3), ap1PNonce1)
	ap1PBalance1, _ := ap1.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(20).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1PNonce2, _ := ap1.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(3), ap1PNonce2)
	ap1PBalance2, _ := ap1.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(50).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1PNonce3, _ := ap1.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(3), ap1PNonce3)
	ap1PBalance3, _ := ap1.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(100).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2PNonce1, _ := ap2.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(4), ap2PNonce1)
	ap2PBalance1, _ := ap2.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(0).Uint64(), ap2PBalance1.Uint64())
	// Addr2
	ap2PNonce2, _ := ap2.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(3), ap2PNonce2)
	ap2PBalance2, _ := ap2.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(30).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2PNonce3, _ := ap2.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(3), ap2PNonce3)
	ap2PBalance3, _ := ap2.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(50).Uint64(), ap2PBalance3.Uint64())
	// Let ap1 be BP's actpool
	pickedTsfs, pickedVotes := ap1.PickActs()
	// ap1 commits update of accounts to trie
	err = ap1.sf.CommitStateChanges(0, pickedTsfs, pickedVotes)
	assert.Nil(err)
	//Reset
	ap1.Reset()
	ap2.Reset()
	// Check confirmed nonce, pending nonce, and pending balance after resetting actpool for each account
	// ap1
	// Addr1
	ap1PNonce1, _ = ap1.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(3), ap1PNonce1)
	ap1PBalance1, _ = ap1.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(220).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1PNonce2, _ = ap1.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(3), ap1PNonce2)
	ap1PBalance2, _ = ap1.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(200).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1PNonce3, _ = ap1.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(3), ap1PNonce3)
	ap1PBalance3, _ = ap1.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(180).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2PNonce1, _ = ap2.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(4), ap2PNonce1)
	ap2PBalance1, _ = ap2.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(200).Uint64(), ap2PBalance1.Uint64())
	// Addr2
	ap2PNonce2, _ = ap2.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(3), ap2PNonce2)
	ap2PBalance2, _ = ap2.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(200).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2PNonce3, _ = ap2.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(3), ap2PNonce3)
	ap2PBalance3, _ = ap2.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(180).Uint64(), ap2PBalance3.Uint64())
	// Add more Tsfs after resetting
	// Tsfs To be added to ap1 only
	tsf15, _ := signedTransfer(addr3, addr2, uint64(3), big.NewInt(80))
	// Tsfs To be added to ap2 only
	tsf16, _ := signedTransfer(addr1, addr2, uint64(4), big.NewInt(150))
	tsf17, _ := signedTransfer(addr2, addr1, uint64(3), big.NewInt(90))
	tsf18, _ := signedTransfer(addr2, addr3, uint64(4), big.NewInt(100))
	tsf19, _ := signedTransfer(addr2, addr1, uint64(5), big.NewInt(50))
	tsf20, _ := signedTransfer(addr3, addr2, uint64(3), big.NewInt(200))

	ap1.AddTsf(tsf15)
	ap2.AddTsf(tsf16)
	ap2.AddTsf(tsf17)
	ap2.AddTsf(tsf18)
	ap2.AddTsf(tsf19)
	ap2.AddTsf(tsf20)
	// Check confirmed nonce, pending nonce, and pending balance after adding Tsfs above for each account
	// ap1
	// Addr1
	ap1PNonce1, _ = ap1.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(3), ap1PNonce1)
	ap1PBalance1, _ = ap1.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(220).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1PNonce2, _ = ap1.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(3), ap1PNonce2)
	ap1PBalance2, _ = ap1.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(200).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1PNonce3, _ = ap1.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(5), ap1PNonce3)
	ap1PBalance3, _ = ap1.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(0).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2PNonce1, _ = ap2.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(5), ap2PNonce1)
	ap2PBalance1, _ = ap2.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(50).Uint64(), ap2PBalance1.Uint64())
	// Addr2
	ap2PNonce2, _ = ap2.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(5), ap2PNonce2)
	ap2PBalance2, _ = ap2.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(10).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2PNonce3, _ = ap2.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(3), ap2PNonce3)
	ap2PBalance3, _ = ap2.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(180).Uint64(), ap2PBalance3.Uint64())
	// Let ap2 be BP's actpool
	pickedTsfs, pickedVotes = ap2.PickActs()
	// ap2 commits update of accounts to trie
	err = ap2.sf.CommitStateChanges(0, pickedTsfs, pickedVotes)
	assert.Nil(err)
	//Reset
	ap1.Reset()
	ap2.Reset()
	// Check confirmed nonce, pending nonce, and pending balance after resetting actpool for each account
	// ap1
	// Addr1
	ap1PNonce1, _ = ap1.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(5), ap1PNonce1)
	ap1PBalance1, _ = ap1.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(140).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1PNonce2, _ = ap1.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(5), ap1PNonce2)
	ap1PBalance2, _ = ap1.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(180).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1PNonce3, _ = ap1.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(5), ap1PNonce3)
	ap1PBalance3, _ = ap1.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(100).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2PNonce1, _ = ap2.getPendingNonce(addr1.RawAddress)
	assert.Equal(uint64(5), ap2PNonce1)
	ap2PBalance1, _ = ap2.getPendingBalance(addr1.RawAddress)
	assert.Equal(big.NewInt(140).Uint64(), ap2PBalance1.Uint64())
	// Addr2
	ap2PNonce2, _ = ap2.getPendingNonce(addr2.RawAddress)
	assert.Equal(uint64(5), ap2PNonce2)
	ap2PBalance2, _ = ap2.getPendingBalance(addr2.RawAddress)
	assert.Equal(big.NewInt(180).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2PNonce3, _ = ap2.getPendingNonce(addr3.RawAddress)
	assert.Equal(uint64(3), ap2PNonce3)
	ap2PBalance3, _ = ap2.getPendingBalance(addr3.RawAddress)
	assert.Equal(big.NewInt(280).Uint64(), ap2PBalance3.Uint64())

	// Add two more players
	sf.CreateState(addr4.RawAddress, uint64(10))
	sf.CreateState(addr5.RawAddress, uint64(20))
	tsf21, _ := signedTransfer(addr4, addr5, uint64(1), big.NewInt(10))
	vote22, _ := signedVote(addr4, addr4, uint64(2))
	vote23, _ := signedVote(addr4, addr5, uint64(3))
	vote24, _ := signedVote(addr5, addr4, uint64(1))
	tsf25, _ := signedTransfer(addr5, addr4, uint64(2), big.NewInt(10))
	vote26, _ := signedVote(addr5, addr5, uint64(3))

	ap1.AddTsf(tsf21)
	ap1.AddVote(vote22)
	ap1.AddVote(vote23)
	ap1.AddVote(vote24)
	ap1.AddTsf(tsf25)
	ap1.AddVote(vote26)
	// Check confirmed nonce, pending nonce, and pending balance after adding actions above for account4 and account5
	// ap1
	// Addr4
	ap1PNonce4, _ := ap1.getPendingNonce(addr4.RawAddress)
	assert.Equal(uint64(4), ap1PNonce4)
	ap1PBalance4, _ := ap1.getPendingBalance(addr4.RawAddress)
	assert.Equal(big.NewInt(0).Uint64(), ap1PBalance4.Uint64())
	// Addr5
	ap1PNonce5, _ := ap1.getPendingNonce(addr5.RawAddress)
	assert.Equal(uint64(4), ap1PNonce5)
	ap1PBalance5, _ := ap1.getPendingBalance(addr5.RawAddress)
	assert.Equal(big.NewInt(10).Uint64(), ap1PBalance5.Uint64())
	// Let ap1 be BP's actpool
	pickedTsfs, pickedVotes = ap1.PickActs()
	// ap1 commits update of accounts to trie
	err = ap1.sf.CommitStateChanges(0, pickedTsfs, pickedVotes)
	assert.Nil(err)
	//Reset
	ap1.Reset()
	// Check confirmed nonce, pending nonce, and pending balance after resetting actpool for each account
	// ap1
	// Addr4
	ap1PNonce4, _ = ap1.getPendingNonce(addr4.RawAddress)
	assert.Equal(uint64(4), ap1PNonce4)
	ap1PBalance4, _ = ap1.getPendingBalance(addr4.RawAddress)
	assert.Equal(big.NewInt(10).Uint64(), ap1PBalance4.Uint64())
	// Addr5
	ap1PNonce5, _ = ap1.getPendingNonce(addr5.RawAddress)
	assert.Equal(uint64(4), ap1PNonce5)
	ap1PBalance5, _ = ap1.getPendingBalance(addr5.RawAddress)
	assert.Equal(big.NewInt(20).Uint64(), ap1PBalance5.Uint64())
}

func TestActPool_removeInvalidActs(t *testing.T) {
	assert := assert.New(t)
	l := logger.Logger().Level(zerolog.DebugLevel)
	logger.SetLogger(&l)
	sf, err := state.NewFactory(nil, state.InMemTrieOption())
	assert.NotNil(sf)
	assert.Nil(err)
	sf.CreateState(addr1.RawAddress, uint64(100))
	// Create actpool
	apConfig := config.ActPool{maxNumActPerPool, maxNumActPerAcct}
	Ap, err := NewActPool(sf, apConfig)
	assert.Nil(err)
	ap, ok := Ap.(*actPool)
	require.True(t, ok)

	tsf1, _ := signedTransfer(addr1, addr1, uint64(1), big.NewInt(10))
	tsf2, _ := signedTransfer(addr1, addr1, uint64(2), big.NewInt(20))
	tsf3, _ := signedTransfer(addr1, addr1, uint64(3), big.NewInt(30))
	vote4, _ := signedVote(addr1, addr1, uint64(4))

	ap.AddTsf(tsf1)
	ap.AddTsf(tsf2)
	ap.AddTsf(tsf3)
	ap.AddVote(vote4)

	hash1 := tsf1.Hash()
	action1 := &pb.ActionPb{&pb.ActionPb_Transfer{tsf1.ConvertToTransferPb()}}
	hash2 := vote4.Hash()
	action2 := &pb.ActionPb{&pb.ActionPb_Vote{vote4.ConvertToVotePb()}}
	acts := []*pb.ActionPb{action1, action2}
	assert.NotNil(ap.allActions[hash1])
	assert.NotNil(ap.allActions[hash2])
	ap.removeInvalidActs(acts)
	assert.Nil(ap.allActions[hash1])
	assert.Nil(ap.allActions[hash2])
}

// Helper function to return the correct pending nonce just in case of empty queue
func (ap *actPool) getPendingNonce(addr string) (uint64, error) {
	if queue, ok := ap.accountActs[addr]; ok {
		return queue.PendingNonce(), nil
	}
	committedNonce, err := ap.sf.Nonce(addr)
	pendingNonce := committedNonce + 1
	return pendingNonce, err
}

// Helper function to return the correct pending balance just in case of empty queue
func (ap *actPool) getPendingBalance(addr string) (*big.Int, error) {
	if queue, ok := ap.accountActs[addr]; ok {
		return queue.PendingBalance(), nil
	}
	return ap.sf.Balance(addr)
}

// Helper function to return a signed transfer
func signedTransfer(sender *iotxaddress.Address, recipient *iotxaddress.Address, nonce uint64, amount *big.Int) (*action.Transfer, error) {
	transfer := action.NewTransfer(nonce, amount, sender.RawAddress, recipient.RawAddress)
	return transfer.Sign(sender)
}

// Helper function to return a signed vote
func signedVote(voter *iotxaddress.Address, votee *iotxaddress.Address, nonce uint64) (*action.Vote, error) {
	vote := action.NewVote(nonce, voter.PublicKey, votee.PublicKey)
	return vote.Sign(voter)
}
