// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/testutil"
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
	maxNumActsPerPool = 8192
	maxNumActsPerAcct = 256
)

var (
	addr1 = testutil.ConstructAddress(pubkeyA, prikeyA)
	addr2 = testutil.ConstructAddress(pubkeyB, prikeyB)
	addr3 = testutil.ConstructAddress(pubkeyC, prikeyC)
	addr4 = testutil.ConstructAddress(pubkeyD, prikeyD)
	addr5 = testutil.ConstructAddress(pubkeyE, prikeyE)
)

func TestActPool_validateTsf(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(&config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1.RawAddress, uint64(100))
	require.NoError(err)
	_, err = bc.GetFactory().RunActions(0, nil, nil, nil)
	require.NoError(err)
	require.Nil(bc.GetFactory().Commit())
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	// Case I: Coinbase transfer
	coinbaseTsf := action.NewCoinBaseTransfer(big.NewInt(1), "1")
	err = ap.validateTsf(coinbaseTsf)
	require.Equal(ErrTransfer, errors.Cause(err))
	// Case II: Oversized data
	tmpPayload := [32769]byte{}
	payload := tmpPayload[:]
	tsf, err := action.NewTransfer(uint64(1), big.NewInt(1), "1", "2", payload, uint64(0), big.NewInt(0))
	require.NoError(err)
	err = ap.validateTsf(tsf)
	require.Equal(ErrActPool, errors.Cause(err))
	// Case III: Negative amount
	tsf, err = action.NewTransfer(uint64(1), big.NewInt(-100), "1", "2", nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	err = ap.validateTsf(tsf)
	require.Equal(ErrBalance, errors.Cause(err))
	// Case IV: Invalid address
	tsf, err = action.NewTransfer(
		1,
		big.NewInt(1),
		addr1.RawAddress,
		"io1qyqsyqcyq5narhapakcsrhksfajfcpl24us3xp38zwvsep",
		nil, uint64(0),
		big.NewInt(0),
	)
	require.NoError(err)
	err = ap.validateTsf(tsf)
	require.Error(err)
	require.True(strings.Contains(err.Error(), "error when validating recipient's address"))
	// Case V: Signature verification fails
	unsignedTsf, err := action.NewTransfer(uint64(1), big.NewInt(1), addr1.RawAddress, addr1.RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	err = ap.validateTsf(unsignedTsf)
	require.Equal(action.ErrAction, errors.Cause(err))
	// Case VI: Nonce is too low
	prevTsf, _ := signedTransfer(addr1, addr1, uint64(1), big.NewInt(50), []byte{}, uint64(100000), big.NewInt(10))
	err = ap.AddTsf(prevTsf)
	require.NoError(err)
	_, err = bc.GetFactory().RunActions(0, []*action.Transfer{prevTsf}, nil, nil)
	require.NoError(err)
	require.Nil(bc.GetFactory().Commit())
	ap.Reset()
	nTsf, _ := signedTransfer(addr1, addr1, uint64(1), big.NewInt(60), []byte{}, uint64(100000), big.NewInt(10))
	err = ap.validateTsf(nTsf)
	require.Equal(ErrNonce, errors.Cause(err))
}

func TestActPool_validateVote(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(&config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1.RawAddress, uint64(100))
	require.NoError(err)
	_, err = bc.CreateState(addr2.RawAddress, uint64(100))
	require.NoError(err)
	_, err = bc.GetFactory().RunActions(0, nil, nil, nil)
	require.NoError(err)
	require.Nil(bc.GetFactory().Commit())
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	// Case I: Invalid address
	vote, err := action.NewVote(1, addr1.RawAddress, "123", 0, big.NewInt(0))
	require.NoError(err)
	vote.SetVoterPublicKey(addr1.PublicKey)
	err = ap.validateVote(vote)
	require.Error(err)
	require.True(strings.Contains(err.Error(), "error when validating votee's address"))
	// Case II: Signature verification fails
	unsignedVote, err := action.NewVote(1, addr1.RawAddress, addr2.RawAddress, uint64(100000), big.NewInt(10))
	require.NoError(err)
	unsignedVote.SetVoterPublicKey(addr1.PublicKey)
	require.NoError(err)
	err = ap.validateVote(unsignedVote)
	require.Equal(action.ErrAction, errors.Cause(err))
	// Case III: Nonce is too low
	prevTsf, _ := signedTransfer(addr1, addr1, uint64(1), big.NewInt(50), []byte{}, uint64(100000), big.NewInt(10))
	err = ap.AddTsf(prevTsf)
	require.NoError(err)
	_, err = bc.GetFactory().RunActions(0, []*action.Transfer{prevTsf}, nil, nil)
	require.NoError(err)
	require.Nil(bc.GetFactory().Commit())
	ap.Reset()
	nVote, _ := signedVote(addr1, addr1, uint64(1), uint64(100000), big.NewInt(10))
	err = ap.validateVote(nVote)
	require.Equal(ErrNonce, errors.Cause(err))
	// Case IV: Votee is not a candidate
	vote2, _ := signedVote(addr1, addr2, uint64(2), uint64(100000), big.NewInt(10))
	err = ap.validateVote(vote2)
	require.Equal(ErrVotee, errors.Cause(err))
}

func TestActPool_AddActs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	require := require.New(t)
	bc := blockchain.NewBlockchain(&config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1.RawAddress, uint64(100))
	require.NoError(err)
	_, err = bc.CreateState(addr2.RawAddress, uint64(10))
	require.NoError(err)
	_, err = bc.GetFactory().RunActions(0, nil, nil, nil)
	require.NoError(err)
	require.Nil(bc.GetFactory().Commit())
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	// Test actpool status after adding a sequence of Tsfs/votes: need to check confirmed nonce, pending nonce, and pending balance
	tsf1, _ := signedTransfer(addr1, addr1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(10))
	tsf2, _ := signedTransfer(addr1, addr1, uint64(2), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(10))
	tsf3, _ := signedTransfer(addr1, addr1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(10))
	vote4, _ := signedVote(addr1, addr1, uint64(4), uint64(100000), big.NewInt(10))
	tsf5, _ := signedTransfer(addr1, addr1, uint64(5), big.NewInt(50), []byte{}, uint64(100000), big.NewInt(10))
	tsf6, _ := signedTransfer(addr2, addr2, uint64(1), big.NewInt(5), []byte{}, uint64(100000), big.NewInt(10))
	tsf7, _ := signedTransfer(addr2, addr2, uint64(3), big.NewInt(1), []byte{}, uint64(100000), big.NewInt(10))
	tsf8, _ := signedTransfer(addr2, addr2, uint64(4), big.NewInt(5), []byte{}, uint64(100000), big.NewInt(10))

	err = ap.AddTsf(tsf1)
	require.NoError(err)
	err = ap.AddTsf(tsf2)
	require.NoError(err)
	err = ap.AddTsf(tsf3)
	require.NoError(err)
	err = ap.AddVote(vote4)
	require.NoError(err)
	err = ap.AddTsf(tsf5)
	require.Equal(ErrBalance, errors.Cause(err))
	err = ap.AddTsf(tsf6)
	require.NoError(err)
	err = ap.AddTsf(tsf7)
	require.NoError(err)
	err = ap.AddTsf(tsf8)
	require.NoError(err)

	pBalance1, _ := ap.getPendingBalance(addr1.RawAddress)
	require.Equal(uint64(40), pBalance1.Uint64())
	pNonce1, _ := ap.getPendingNonce(addr1.RawAddress)
	require.Equal(uint64(5), pNonce1)

	pBalance2, _ := ap.getPendingBalance(addr2.RawAddress)
	require.Equal(uint64(5), pBalance2.Uint64())
	pNonce2, _ := ap.getPendingNonce(addr2.RawAddress)
	require.Equal(uint64(2), pNonce2)

	tsf9, _ := signedTransfer(addr2, addr2, uint64(2), big.NewInt(3), []byte{}, uint64(100000), big.NewInt(10))
	err = ap.AddTsf(tsf9)
	require.NoError(err)
	pBalance2, _ = ap.getPendingBalance(addr2.RawAddress)
	require.Equal(uint64(1), pBalance2.Uint64())
	pNonce2, _ = ap.getPendingNonce(addr2.RawAddress)
	require.Equal(uint64(4), pNonce2)
	// Error Case Handling
	// Case I: Action already exists in pool
	err = ap.AddTsf(tsf1)
	require.Equal(fmt.Errorf("existed transfer: %x", tsf1.Hash()), err)
	err = ap.AddVote(vote4)
	require.Equal(fmt.Errorf("existed vote: %x", vote4.Hash()), err)
	// Case II: Pool space is full
	mockBC := mock_blockchain.NewMockBlockchain(ctrl)
	Ap2, err := NewActPool(mockBC, apConfig)
	require.NoError(err)
	ap2, ok := Ap2.(*actPool)
	require.True(ok)
	for i := uint64(0); i < ap2.cfg.MaxNumActsPerPool; i++ {
		nTsf, err := action.NewTransfer(
			i, big.NewInt(int64(i)), "1", "2", nil, uint64(0), big.NewInt(0))
		require.NoError(err)
		nAction := nTsf.ConvertToActionPb()
		ap2.allActions[nTsf.Hash()] = nAction
	}
	mockBC.EXPECT().Nonce(gomock.Any()).Times(2).Return(uint64(0), nil)
	mockBC.EXPECT().StateByAddr(gomock.Any()).Times(1).Return(nil, nil)
	err = ap2.AddTsf(tsf1)
	require.Equal(ErrActPool, errors.Cause(err))
	err = ap2.AddVote(vote4)
	require.Equal(ErrActPool, errors.Cause(err))
	// Case III: Nonce already exists
	replaceTsf, _ := signedTransfer(addr1, addr2, uint64(1), big.NewInt(1), []byte{}, uint64(100000), big.NewInt(10))
	err = ap.AddTsf(replaceTsf)
	require.Equal(ErrNonce, errors.Cause(err))
	replaceVote, err := action.NewVote(4, addr1.RawAddress, "", uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NoError(action.Sign(replaceVote, addr1))
	err = ap.AddVote(replaceVote)
	require.Equal(ErrNonce, errors.Cause(err))
	// Case IV: Nonce is too large
	outOfBoundsTsf, _ := signedTransfer(addr1, addr1, ap.cfg.MaxNumActsPerAcct+1, big.NewInt(1), []byte{}, uint64(100000), big.NewInt(10))
	err = ap.AddTsf(outOfBoundsTsf)
	require.Equal(ErrNonce, errors.Cause(err))
	// Case V: Insufficient balance
	overBalTsf, _ := signedTransfer(addr2, addr2, uint64(4), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(10))
	err = ap.AddTsf(overBalTsf)
	require.Equal(ErrBalance, errors.Cause(err))
	// Case VI: over gas limit
	creationExecution, err := action.NewExecution(addr1.RawAddress, action.EmptyAddress, uint64(5), big.NewInt(int64(0)), action.GasLimit+100, big.NewInt(10), []byte{})
	require.NoError(err)
	err = ap.AddExecution(creationExecution)
	require.Equal(ErrGasHigherThanLimit, errors.Cause(err))
	// Case VII: insufficient gas
	tmpData := [1234]byte{}
	creationExecution, err = action.NewExecution(
		addr1.RawAddress,
		action.EmptyAddress,
		uint64(5),
		big.NewInt(int64(0)),
		10,
		big.NewInt(10),
		tmpData[:],
	)
	require.NoError(err)
	err = ap.AddExecution(creationExecution)
	require.Equal(ErrInsufficientGas, errors.Cause(err))
}

func TestActPool_PickActs(t *testing.T) {
	createActPool := func(cfg config.ActPool) (*actPool, []*action.Transfer, []*action.Vote, []*action.Execution) {
		require := require.New(t)
		bc := blockchain.NewBlockchain(&config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
		require.NoError(bc.Start(context.Background()))
		_, err := bc.CreateState(addr1.RawAddress, uint64(100))
		require.NoError(err)
		_, err = bc.CreateState(addr2.RawAddress, uint64(10))
		require.NoError(err)
		_, err = bc.GetFactory().RunActions(0, nil, nil, nil)
		require.NoError(err)
		require.Nil(bc.GetFactory().Commit())
		// Create actpool
		Ap, err := NewActPool(bc, cfg)
		require.NoError(err)
		ap, ok := Ap.(*actPool)
		require.True(ok)

		tsf1, _ := signedTransfer(addr1, addr1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(10))
		tsf2, _ := signedTransfer(addr1, addr1, uint64(2), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(10))
		tsf3, _ := signedTransfer(addr1, addr1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(10))
		tsf4, _ := signedTransfer(addr1, addr1, uint64(4), big.NewInt(40), []byte{}, uint64(100000), big.NewInt(10))
		tsf5, _ := signedTransfer(addr1, addr1, uint64(5), big.NewInt(50), []byte{}, uint64(100000), big.NewInt(10))
		vote6, _ := signedVote(addr1, addr1, uint64(6), uint64(100000), big.NewInt(10))
		vote7, _ := signedVote(addr2, addr2, uint64(1), uint64(100000), big.NewInt(10))
		tsf8, _ := signedTransfer(addr2, addr2, uint64(3), big.NewInt(5), []byte{}, uint64(100000), big.NewInt(10))
		tsf9, _ := signedTransfer(addr2, addr2, uint64(4), big.NewInt(1), []byte{}, uint64(100000), big.NewInt(10))
		tsf10, _ := signedTransfer(addr2, addr2, uint64(5), big.NewInt(5), []byte{}, uint64(100000), big.NewInt(10))

		err = ap.AddTsf(tsf1)
		require.NoError(err)
		err = ap.AddTsf(tsf2)
		require.NoError(err)
		err = ap.AddTsf(tsf3)
		require.NoError(err)
		err = ap.AddTsf(tsf4)
		require.NoError(err)
		err = ap.AddTsf(tsf5)
		require.Equal(ErrBalance, errors.Cause(err))
		err = ap.AddVote(vote6)
		require.NoError(err)
		err = ap.AddVote(vote7)
		require.NoError(err)
		err = ap.AddTsf(tsf8)
		require.NoError(err)
		err = ap.AddTsf(tsf9)
		require.NoError(err)
		err = ap.AddTsf(tsf10)
		require.NoError(err)
		return ap, []*action.Transfer{tsf1, tsf2, tsf3, tsf4}, []*action.Vote{vote7}, []*action.Execution{}
	}

	t.Run("no-limit", func(t *testing.T) {
		apConfig := getActPoolCfg()
		ap, transfers, votes, executions := createActPool(apConfig)
		pickedTsfs, pickedVotes, pickedExecutions := ap.PickActs()
		require.Equal(t, transfers, pickedTsfs)
		require.Equal(t, votes, pickedVotes)
		require.Equal(t, executions, pickedExecutions)
	})
	t.Run("enough-limit", func(t *testing.T) {
		apConfig := getActPoolCfg()
		apConfig.MaxNumActsToPick = 10
		ap, transfers, votes, executions := createActPool(apConfig)
		pickedTsfs, pickedVotes, pickedExecutions := ap.PickActs()
		require.Equal(t, transfers, pickedTsfs)
		require.Equal(t, votes, pickedVotes)
		require.Equal(t, executions, pickedExecutions)
	})
	t.Run("low-limit", func(t *testing.T) {
		apConfig := getActPoolCfg()
		apConfig.MaxNumActsToPick = 3
		ap, _, _, _ := createActPool(apConfig)
		pickedTsfs, pickedVotes, pickedExecutions := ap.PickActs()
		require.Equal(t, 3, len(pickedTsfs)+len(pickedVotes)+len(pickedExecutions))
	})
}

func TestActPool_removeConfirmedActs(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(&config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1.RawAddress, uint64(100))
	require.NoError(err)
	_, err = bc.GetFactory().RunActions(0, nil, nil, nil)
	require.NoError(err)
	require.Nil(bc.GetFactory().Commit())
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)

	tsf1, _ := signedTransfer(addr1, addr1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(10))
	tsf2, _ := signedTransfer(addr1, addr1, uint64(2), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(10))
	tsf3, _ := signedTransfer(addr1, addr1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(10))
	vote4, _ := signedVote(addr1, addr1, uint64(4), uint64(100000), big.NewInt(10))

	err = ap.AddTsf(tsf1)
	require.NoError(err)
	err = ap.AddTsf(tsf2)
	require.NoError(err)
	err = ap.AddTsf(tsf3)
	require.NoError(err)
	err = ap.AddVote(vote4)
	require.NoError(err)

	require.Equal(4, len(ap.allActions))
	require.NotNil(ap.accountActs[addr1.RawAddress])
	_, err = bc.GetFactory().RunActions(0, []*action.Transfer{tsf1, tsf2, tsf3}, []*action.Vote{vote4}, []*action.Execution{})
	require.NoError(err)
	require.Nil(bc.GetFactory().Commit())
	ap.removeConfirmedActs()
	require.Equal(0, len(ap.allActions))
	require.Nil(ap.accountActs[addr1.RawAddress])
}

func TestActPool_Reset(t *testing.T) {
	require := require.New(t)

	bc := blockchain.NewBlockchain(&config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1.RawAddress, uint64(100))
	require.NoError(err)
	_, err = bc.CreateState(addr2.RawAddress, uint64(200))
	require.NoError(err)
	_, err = bc.CreateState(addr3.RawAddress, uint64(300))
	require.NoError(err)
	_, err = bc.GetFactory().RunActions(0, nil, nil, nil)
	require.NoError(err)
	require.Nil(bc.GetFactory().Commit())

	apConfig := getActPoolCfg()
	Ap1, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap1, ok := Ap1.(*actPool)
	require.True(ok)
	Ap2, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap2, ok := Ap2.(*actPool)
	require.True(ok)

	// Tsfs to be added to ap1
	tsf1, _ := signedTransfer(addr1, addr2, uint64(1), big.NewInt(50), []byte{}, uint64(100000), big.NewInt(10))
	tsf2, _ := signedTransfer(addr1, addr3, uint64(2), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(10))
	tsf3, _ := signedTransfer(addr1, addr2, uint64(3), big.NewInt(60), []byte{}, uint64(100000), big.NewInt(10))
	tsf4, _ := signedTransfer(addr2, addr1, uint64(1), big.NewInt(100), []byte{}, uint64(100000), big.NewInt(10))
	tsf5, _ := signedTransfer(addr2, addr3, uint64(2), big.NewInt(50), []byte{}, uint64(100000), big.NewInt(10))
	tsf6, _ := signedTransfer(addr2, addr1, uint64(3), big.NewInt(60), []byte{}, uint64(100000), big.NewInt(10))
	tsf7, _ := signedTransfer(addr3, addr1, uint64(1), big.NewInt(100), []byte{}, uint64(100000), big.NewInt(10))
	tsf8, _ := signedTransfer(addr3, addr2, uint64(2), big.NewInt(100), []byte{}, uint64(100000), big.NewInt(10))
	tsf9, _ := signedTransfer(addr3, addr1, uint64(4), big.NewInt(100), []byte{}, uint64(100000), big.NewInt(10))

	err = ap1.AddTsf(tsf1)
	require.NoError(err)
	err = ap1.AddTsf(tsf2)
	require.NoError(err)
	err = ap1.AddTsf(tsf3)
	require.Equal(ErrBalance, errors.Cause(err))
	err = ap1.AddTsf(tsf4)
	require.NoError(err)
	err = ap1.AddTsf(tsf5)
	require.NoError(err)
	err = ap1.AddTsf(tsf6)
	require.Equal(ErrBalance, errors.Cause(err))
	err = ap1.AddTsf(tsf7)
	require.NoError(err)
	err = ap1.AddTsf(tsf8)
	require.NoError(err)
	err = ap1.AddTsf(tsf9)
	require.NoError(err)
	// Tsfs to be added to ap2 only
	tsf10, _ := signedTransfer(addr1, addr2, uint64(3), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(10))
	tsf11, _ := signedTransfer(addr1, addr3, uint64(4), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(10))
	tsf12, _ := signedTransfer(addr2, addr3, uint64(2), big.NewInt(70), []byte{}, uint64(100000), big.NewInt(10))
	tsf13, _ := signedTransfer(addr3, addr1, uint64(1), big.NewInt(200), []byte{}, uint64(100000), big.NewInt(10))
	tsf14, _ := signedTransfer(addr3, addr2, uint64(2), big.NewInt(50), []byte{}, uint64(100000), big.NewInt(10))

	err = ap2.AddTsf(tsf1)
	require.NoError(err)
	err = ap2.AddTsf(tsf2)
	require.NoError(err)
	err = ap2.AddTsf(tsf10)
	require.NoError(err)
	err = ap2.AddTsf(tsf11)
	require.Equal(ErrBalance, errors.Cause(err))
	err = ap2.AddTsf(tsf4)
	require.NoError(err)
	err = ap2.AddTsf(tsf12)
	require.NoError(err)
	err = ap2.AddTsf(tsf13)
	require.NoError(err)
	err = ap2.AddTsf(tsf14)
	require.NoError(err)
	err = ap2.AddTsf(tsf9)
	require.Equal(ErrBalance, errors.Cause(err))
	// Check confirmed nonce, pending nonce, and pending balance after adding Tsfs above for each account
	// ap1
	// Addr1
	ap1PNonce1, _ := ap1.getPendingNonce(addr1.RawAddress)
	require.Equal(uint64(3), ap1PNonce1)
	ap1PBalance1, _ := ap1.getPendingBalance(addr1.RawAddress)
	require.Equal(big.NewInt(20).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1PNonce2, _ := ap1.getPendingNonce(addr2.RawAddress)
	require.Equal(uint64(3), ap1PNonce2)
	ap1PBalance2, _ := ap1.getPendingBalance(addr2.RawAddress)
	require.Equal(big.NewInt(50).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1PNonce3, _ := ap1.getPendingNonce(addr3.RawAddress)
	require.Equal(uint64(3), ap1PNonce3)
	ap1PBalance3, _ := ap1.getPendingBalance(addr3.RawAddress)
	require.Equal(big.NewInt(100).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2PNonce1, _ := ap2.getPendingNonce(addr1.RawAddress)
	require.Equal(uint64(4), ap2PNonce1)
	ap2PBalance1, _ := ap2.getPendingBalance(addr1.RawAddress)
	require.Equal(big.NewInt(0).Uint64(), ap2PBalance1.Uint64())
	// Addr2
	ap2PNonce2, _ := ap2.getPendingNonce(addr2.RawAddress)
	require.Equal(uint64(3), ap2PNonce2)
	ap2PBalance2, _ := ap2.getPendingBalance(addr2.RawAddress)
	require.Equal(big.NewInt(30).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2PNonce3, _ := ap2.getPendingNonce(addr3.RawAddress)
	require.Equal(uint64(3), ap2PNonce3)
	ap2PBalance3, _ := ap2.getPendingBalance(addr3.RawAddress)
	require.Equal(big.NewInt(50).Uint64(), ap2PBalance3.Uint64())
	// Let ap1 be BP's actpool
	pickedTsfs, pickedVotes, pickedExecutions := ap1.PickActs()
	// ap1 commits update of accounts to trie
	_, err = bc.GetFactory().RunActions(0, pickedTsfs, pickedVotes, pickedExecutions)
	require.NoError(err)
	require.Nil(bc.GetFactory().Commit())
	//Reset
	ap1.Reset()
	ap2.Reset()
	// Check confirmed nonce, pending nonce, and pending balance after resetting actpool for each account
	// ap1
	// Addr1
	ap1PNonce1, _ = ap1.getPendingNonce(addr1.RawAddress)
	require.Equal(uint64(3), ap1PNonce1)
	ap1PBalance1, _ = ap1.getPendingBalance(addr1.RawAddress)
	require.Equal(big.NewInt(220).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1PNonce2, _ = ap1.getPendingNonce(addr2.RawAddress)
	require.Equal(uint64(3), ap1PNonce2)
	ap1PBalance2, _ = ap1.getPendingBalance(addr2.RawAddress)
	require.Equal(big.NewInt(200).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1PNonce3, _ = ap1.getPendingNonce(addr3.RawAddress)
	require.Equal(uint64(3), ap1PNonce3)
	ap1PBalance3, _ = ap1.getPendingBalance(addr3.RawAddress)
	require.Equal(big.NewInt(180).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2PNonce1, _ = ap2.getPendingNonce(addr1.RawAddress)
	require.Equal(uint64(4), ap2PNonce1)
	ap2PBalance1, _ = ap2.getPendingBalance(addr1.RawAddress)
	require.Equal(big.NewInt(200).Uint64(), ap2PBalance1.Uint64())
	// Addr2
	ap2PNonce2, _ = ap2.getPendingNonce(addr2.RawAddress)
	require.Equal(uint64(3), ap2PNonce2)
	ap2PBalance2, _ = ap2.getPendingBalance(addr2.RawAddress)
	require.Equal(big.NewInt(200).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2PNonce3, _ = ap2.getPendingNonce(addr3.RawAddress)
	require.Equal(uint64(3), ap2PNonce3)
	ap2PBalance3, _ = ap2.getPendingBalance(addr3.RawAddress)
	require.Equal(big.NewInt(180).Uint64(), ap2PBalance3.Uint64())
	// Add more Tsfs after resetting
	// Tsfs To be added to ap1 only
	tsf15, _ := signedTransfer(addr3, addr2, uint64(3), big.NewInt(80), []byte{}, uint64(100000), big.NewInt(10))
	// Tsfs To be added to ap2 only
	tsf16, _ := signedTransfer(addr1, addr2, uint64(4), big.NewInt(150), []byte{}, uint64(100000), big.NewInt(10))
	tsf17, _ := signedTransfer(addr2, addr1, uint64(3), big.NewInt(90), []byte{}, uint64(100000), big.NewInt(10))
	tsf18, _ := signedTransfer(addr2, addr3, uint64(4), big.NewInt(100), []byte{}, uint64(100000), big.NewInt(10))
	tsf19, _ := signedTransfer(addr2, addr1, uint64(5), big.NewInt(50), []byte{}, uint64(100000), big.NewInt(10))
	tsf20, _ := signedTransfer(addr3, addr2, uint64(3), big.NewInt(200), []byte{}, uint64(100000), big.NewInt(10))

	err = ap1.AddTsf(tsf15)
	require.NoError(err)
	err = ap2.AddTsf(tsf16)
	require.NoError(err)
	err = ap2.AddTsf(tsf17)
	require.NoError(err)
	err = ap2.AddTsf(tsf18)
	require.NoError(err)
	err = ap2.AddTsf(tsf19)
	require.Equal(ErrBalance, errors.Cause(err))
	err = ap2.AddTsf(tsf20)
	require.Equal(ErrBalance, errors.Cause(err))
	// Check confirmed nonce, pending nonce, and pending balance after adding Tsfs above for each account
	// ap1
	// Addr1
	ap1PNonce1, _ = ap1.getPendingNonce(addr1.RawAddress)
	require.Equal(uint64(3), ap1PNonce1)
	ap1PBalance1, _ = ap1.getPendingBalance(addr1.RawAddress)
	require.Equal(big.NewInt(220).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1PNonce2, _ = ap1.getPendingNonce(addr2.RawAddress)
	require.Equal(uint64(3), ap1PNonce2)
	ap1PBalance2, _ = ap1.getPendingBalance(addr2.RawAddress)
	require.Equal(big.NewInt(200).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1PNonce3, _ = ap1.getPendingNonce(addr3.RawAddress)
	require.Equal(uint64(5), ap1PNonce3)
	ap1PBalance3, _ = ap1.getPendingBalance(addr3.RawAddress)
	require.Equal(big.NewInt(0).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2PNonce1, _ = ap2.getPendingNonce(addr1.RawAddress)
	require.Equal(uint64(5), ap2PNonce1)
	ap2PBalance1, _ = ap2.getPendingBalance(addr1.RawAddress)
	require.Equal(big.NewInt(50).Uint64(), ap2PBalance1.Uint64())
	// Addr2
	ap2PNonce2, _ = ap2.getPendingNonce(addr2.RawAddress)
	require.Equal(uint64(5), ap2PNonce2)
	ap2PBalance2, _ = ap2.getPendingBalance(addr2.RawAddress)
	require.Equal(big.NewInt(10).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2PNonce3, _ = ap2.getPendingNonce(addr3.RawAddress)
	require.Equal(uint64(3), ap2PNonce3)
	ap2PBalance3, _ = ap2.getPendingBalance(addr3.RawAddress)
	require.Equal(big.NewInt(180).Uint64(), ap2PBalance3.Uint64())
	// Let ap2 be BP's actpool
	pickedTsfs, pickedVotes, pickedExecutions = ap2.PickActs()
	// ap2 commits update of accounts to trie
	_, err = bc.GetFactory().RunActions(0, pickedTsfs, pickedVotes, pickedExecutions)
	require.NoError(err)
	require.Nil(bc.GetFactory().Commit())
	//Reset
	ap1.Reset()
	ap2.Reset()
	// Check confirmed nonce, pending nonce, and pending balance after resetting actpool for each account
	// ap1
	// Addr1
	ap1PNonce1, _ = ap1.getPendingNonce(addr1.RawAddress)
	require.Equal(uint64(5), ap1PNonce1)
	ap1PBalance1, _ = ap1.getPendingBalance(addr1.RawAddress)
	require.Equal(big.NewInt(140).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1PNonce2, _ = ap1.getPendingNonce(addr2.RawAddress)
	require.Equal(uint64(5), ap1PNonce2)
	ap1PBalance2, _ = ap1.getPendingBalance(addr2.RawAddress)
	require.Equal(big.NewInt(180).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1PNonce3, _ = ap1.getPendingNonce(addr3.RawAddress)
	require.Equal(uint64(5), ap1PNonce3)
	ap1PBalance3, _ = ap1.getPendingBalance(addr3.RawAddress)
	require.Equal(big.NewInt(100).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2PNonce1, _ = ap2.getPendingNonce(addr1.RawAddress)
	require.Equal(uint64(5), ap2PNonce1)
	ap2PBalance1, _ = ap2.getPendingBalance(addr1.RawAddress)
	require.Equal(big.NewInt(140).Uint64(), ap2PBalance1.Uint64())
	// Addr2
	ap2PNonce2, _ = ap2.getPendingNonce(addr2.RawAddress)
	require.Equal(uint64(5), ap2PNonce2)
	ap2PBalance2, _ = ap2.getPendingBalance(addr2.RawAddress)
	require.Equal(big.NewInt(180).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2PNonce3, _ = ap2.getPendingNonce(addr3.RawAddress)
	require.Equal(uint64(3), ap2PNonce3)
	ap2PBalance3, _ = ap2.getPendingBalance(addr3.RawAddress)
	require.Equal(big.NewInt(280).Uint64(), ap2PBalance3.Uint64())

	// Add two more players
	_, err = bc.CreateState(addr4.RawAddress, uint64(10))
	require.NoError(err)
	_, err = bc.CreateState(addr5.RawAddress, uint64(20))
	require.NoError(err)
	_, err = bc.GetFactory().RunActions(1, nil, nil, nil)
	require.NoError(err)
	require.Nil(bc.GetFactory().Commit())
	tsf21, _ := signedTransfer(addr4, addr5, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(10))
	vote22, _ := signedVote(addr4, addr4, uint64(2), uint64(100000), big.NewInt(10))
	vote23, _ := action.NewVote(3, addr4.RawAddress, "", uint64(100000), big.NewInt(10))
	_ = action.Sign(vote23, addr4)
	vote24, _ := signedVote(addr5, addr5, uint64(1), uint64(100000), big.NewInt(10))
	tsf25, _ := signedTransfer(addr5, addr4, uint64(2), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(10))
	vote26, _ := action.NewVote(3, addr5.RawAddress, "", uint64(100000), big.NewInt(10))
	_ = action.Sign(vote26, addr5)

	err = ap1.AddTsf(tsf21)
	require.NoError(err)
	err = ap1.AddVote(vote22)
	require.NoError(err)
	err = ap1.AddVote(vote23)
	require.NoError(err)
	err = ap1.AddVote(vote24)
	require.NoError(err)
	err = ap1.AddTsf(tsf25)
	require.NoError(err)
	err = ap1.AddVote(vote26)
	require.NoError(err)
	// Check confirmed nonce, pending nonce, and pending balance after adding actions above for account4 and account5
	// ap1
	// Addr4
	ap1PNonce4, _ := ap1.getPendingNonce(addr4.RawAddress)
	require.Equal(uint64(4), ap1PNonce4)
	ap1PBalance4, _ := ap1.getPendingBalance(addr4.RawAddress)
	require.Equal(big.NewInt(0).Uint64(), ap1PBalance4.Uint64())
	// Addr5
	ap1PNonce5, _ := ap1.getPendingNonce(addr5.RawAddress)
	require.Equal(uint64(4), ap1PNonce5)
	ap1PBalance5, _ := ap1.getPendingBalance(addr5.RawAddress)
	require.Equal(big.NewInt(10).Uint64(), ap1PBalance5.Uint64())
	// Let ap1 be BP's actpool
	pickedTsfs, pickedVotes, pickedExecutions = ap1.PickActs()
	// ap1 commits update of accounts to trie
	_, err = bc.GetFactory().RunActions(0, pickedTsfs, pickedVotes, pickedExecutions)
	require.NoError(err)
	require.Nil(bc.GetFactory().Commit())
	//Reset
	ap1.Reset()
	// Check confirmed nonce, pending nonce, and pending balance after resetting actpool for each account
	// ap1
	// Addr4
	ap1PNonce4, _ = ap1.getPendingNonce(addr4.RawAddress)
	require.Equal(uint64(4), ap1PNonce4)
	ap1PBalance4, _ = ap1.getPendingBalance(addr4.RawAddress)
	require.Equal(big.NewInt(10).Uint64(), ap1PBalance4.Uint64())
	// Addr5
	ap1PNonce5, _ = ap1.getPendingNonce(addr5.RawAddress)
	require.Equal(uint64(4), ap1PNonce5)
	ap1PBalance5, _ = ap1.getPendingBalance(addr5.RawAddress)
	require.Equal(big.NewInt(20).Uint64(), ap1PBalance5.Uint64())
}

func TestActPool_removeInvalidActs(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(&config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1.RawAddress, uint64(100))
	require.NoError(err)
	_, err = bc.GetFactory().RunActions(0, nil, nil, nil)
	require.NoError(err)
	require.Nil(bc.GetFactory().Commit())
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)

	tsf1, _ := signedTransfer(addr1, addr1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(10))
	tsf2, _ := signedTransfer(addr1, addr1, uint64(2), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(10))
	tsf3, _ := signedTransfer(addr1, addr1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(10))
	vote4, _ := signedVote(addr1, addr1, uint64(4), uint64(100000), big.NewInt(10))

	err = ap.AddTsf(tsf1)
	require.NoError(err)
	err = ap.AddTsf(tsf2)
	require.NoError(err)
	err = ap.AddTsf(tsf3)
	require.NoError(err)
	err = ap.AddVote(vote4)
	require.NoError(err)

	hash1 := tsf1.Hash()
	action1 := tsf1.ConvertToActionPb()
	hash2 := vote4.Hash()
	action2 := vote4.ConvertToActionPb()
	acts := []*iproto.ActionPb{action1, action2}
	require.NotNil(ap.allActions[hash1])
	require.NotNil(ap.allActions[hash2])
	ap.removeInvalidActs(acts)
	require.Nil(ap.allActions[hash1])
	require.Nil(ap.allActions[hash2])
}

func TestActPool_GetPendingNonce(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(&config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1.RawAddress, uint64(100))
	require.NoError(err)
	_, err = bc.CreateState(addr2.RawAddress, uint64(100))
	require.NoError(err)
	_, err = bc.GetFactory().RunActions(0, nil, nil, nil)
	require.NoError(err)
	require.Nil(bc.GetFactory().Commit())
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)

	tsf1, _ := signedTransfer(addr1, addr1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(10))
	tsf3, _ := signedTransfer(addr1, addr1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(10))
	vote4, _ := signedVote(addr1, addr1, uint64(4), uint64(100000), big.NewInt(10))

	err = ap.AddTsf(tsf1)
	require.NoError(err)
	err = ap.AddTsf(tsf3)
	require.NoError(err)
	err = ap.AddVote(vote4)
	require.NoError(err)

	nonce, err := ap.GetPendingNonce(addr2.RawAddress)
	require.NoError(err)
	require.Equal(uint64(1), nonce)

	nonce, err = ap.GetPendingNonce(addr1.RawAddress)
	require.NoError(err)
	require.Equal(uint64(2), nonce)
}

func TestActPool_GetUnconfirmedActs(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(&config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1.RawAddress, uint64(100))
	require.NoError(err)
	_, err = bc.CreateState(addr2.RawAddress, uint64(100))
	require.NoError(err)
	_, err = bc.GetFactory().RunActions(0, nil, nil, nil)
	require.NoError(err)
	require.Nil(bc.GetFactory().Commit())
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)

	tsf1, _ := signedTransfer(addr1, addr1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(10))
	act1 := tsf1.ConvertToActionPb()
	tsf3, _ := signedTransfer(addr1, addr1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(10))
	act3 := tsf3.ConvertToActionPb()
	vote4, _ := signedVote(addr1, addr1, uint64(4), uint64(100000), big.NewInt(10))
	act4 := vote4.ConvertToActionPb()

	err = ap.AddTsf(tsf1)
	require.NoError(err)
	err = ap.AddTsf(tsf3)
	require.NoError(err)
	err = ap.AddVote(vote4)
	require.NoError(err)

	acts := ap.GetUnconfirmedActs(addr2.RawAddress)
	require.Equal([]*iproto.ActionPb{}, acts)

	acts = ap.GetUnconfirmedActs(addr1.RawAddress)
	require.Equal([]*iproto.ActionPb{act1, act3, act4}, acts)
}

func TestActPool_GetActionByHash(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(&config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1.RawAddress, uint64(100))
	require.NoError(err)
	_, err = bc.CreateState(addr2.RawAddress, uint64(100))
	require.NoError(err)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)

	tsf1, _ := signedTransfer(addr1, addr1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(10))
	hash1 := tsf1.Hash()
	act1 := tsf1.ConvertToActionPb()
	vote2, _ := signedVote(addr1, addr1, uint64(2), uint64(100000), big.NewInt(10))
	hash2 := vote2.Hash()
	act2 := vote2.ConvertToActionPb()

	ap.allActions[hash1] = act1
	act, err := ap.GetActionByHash(hash1)
	require.NoError(err)
	require.Equal(act1, act)
	act, err = ap.GetActionByHash(hash2)
	require.Equal(ErrHash, errors.Cause(err))
	require.Nil(act)

	ap.allActions[hash2] = act2
	act, err = ap.GetActionByHash(hash2)
	require.NoError(err)
	require.Equal(act2, act)
}

func TestActPool_GetCapacity(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(&config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	require.Equal(uint64(maxNumActsPerPool), ap.GetCapacity())
}

func TestActPool_GetSize(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(&config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1.RawAddress, uint64(100))
	require.NoError(err)
	_, err = bc.GetFactory().RunActions(0, nil, nil, nil)
	require.NoError(err)
	require.Nil(bc.GetFactory().Commit())
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	require.Zero(ap.GetSize())

	tsf1, err := signedTransfer(addr1, addr1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	tsf2, err := signedTransfer(addr1, addr1, uint64(2), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	tsf3, err := signedTransfer(addr1, addr1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	vote4, err := signedVote(addr1, addr1, uint64(4), uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NoError(ap.AddTsf(tsf1))
	require.NoError(ap.AddTsf(tsf2))
	require.NoError(ap.AddTsf(tsf3))
	require.NoError(ap.AddVote(vote4))
	require.Equal(uint64(4), ap.GetSize())
	_, err = bc.GetFactory().RunActions(0, []*action.Transfer{tsf1, tsf2, tsf3}, []*action.Vote{vote4}, nil)
	require.NoError(err)
	require.Nil(bc.GetFactory().Commit())
	ap.removeConfirmedActs()
	require.Equal(uint64(0), ap.GetSize())
}

// Helper function to return the correct pending nonce just in case of empty queue
func (ap *actPool) getPendingNonce(addr string) (uint64, error) {
	if queue, ok := ap.accountActs[addr]; ok {
		return queue.PendingNonce(), nil
	}
	committedNonce, err := ap.bc.Nonce(addr)
	pendingNonce := committedNonce + 1
	return pendingNonce, err
}

// Helper function to return the correct pending balance just in case of empty queue
func (ap *actPool) getPendingBalance(addr string) (*big.Int, error) {
	if queue, ok := ap.accountActs[addr]; ok {
		return queue.PendingBalance(), nil
	}
	return ap.bc.Balance(addr)
}

// Helper function to return a signed transfer
func signedTransfer(sender *iotxaddress.Address, recipient *iotxaddress.Address, nonce uint64, amount *big.Int, payload []byte, gasLimit uint64, gasPrice *big.Int) (*action.Transfer, error) {
	transfer, err := action.NewTransfer(nonce, amount, sender.RawAddress, recipient.RawAddress, payload, gasLimit, gasPrice)
	if err != nil {
		return nil, err
	}
	if err := action.Sign(transfer, sender); err != nil {
		return nil, err
	}
	return transfer, nil
}

// Helper function to return a signed vote
func signedVote(voter *iotxaddress.Address, votee *iotxaddress.Address, nonce uint64, gasLimit uint64, gasPrice *big.Int) (*action.Vote, error) {
	vote, err := action.NewVote(nonce, voter.RawAddress, votee.RawAddress, gasLimit, gasPrice)
	if err != nil {
		return nil, err
	}
	if err := action.Sign(vote, voter); err != nil {
		return nil, err
	}
	return vote, nil
}

func getActPoolCfg() config.ActPool {
	return config.ActPool{
		MaxNumActsPerPool: maxNumActsPerPool,
		MaxNumActsPerAcct: maxNumActsPerAcct,
	}
}
