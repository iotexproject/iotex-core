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
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/testaddress"
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
	chainID = config.Default.Chain.ID
	addr1   = testaddress.ConstructAddress(chainID, pubkeyA, prikeyA)
	addr2   = testaddress.ConstructAddress(chainID, pubkeyB, prikeyB)
	addr3   = testaddress.ConstructAddress(chainID, pubkeyC, prikeyC)
	addr4   = testaddress.ConstructAddress(chainID, pubkeyD, prikeyD)
	addr5   = testaddress.ConstructAddress(chainID, pubkeyE, prikeyE)
)

func TestActPool_validateGenericAction(t *testing.T) {
	require := require.New(t)

	bc := blockchain.NewBlockchain(config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	bc.GetFactory().AddActionHandlers(account.NewProtocol())
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1.RawAddress, big.NewInt(100))
	require.NoError(err)
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	validator := ap.actionEnvelopeValidators[0]
	// Case I: Over-gassed transfer
	tsf, err := testutil.SignedTransfer(addr1, addr1, 1, big.NewInt(1), nil, blockchain.GasLimit+1, big.NewInt(0))
	require.NoError(err)

	err = validator.Validate(context.Background(), tsf)
	require.Equal(action.ErrGasHigherThanLimit, errors.Cause(err))
	// Case II: Insufficient gas
	tsf, err = testutil.SignedTransfer(addr1, addr1, 1, big.NewInt(1), nil, 0, big.NewInt(0))
	require.NoError(err)
	err = validator.Validate(context.Background(), tsf)
	require.Equal(action.ErrInsufficientBalanceForGas, errors.Cause(err))
	// Case III: Signature verification fails
	unsignedTsf, err := action.NewTransfer(uint64(1), big.NewInt(1), addr1.RawAddress, addr1.RawAddress, []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)

	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetAction(unsignedTsf).
		SetGasLimit(100000).
		SetDestinationAddress(addr1.RawAddress).Build()
	selp := action.FakeSeal(elp, addr1.RawAddress, addr1.PublicKey)
	err = validator.Validate(context.Background(), selp)
	require.Equal(action.ErrAction, errors.Cause(err))
	// Case IV: Nonce is too low
	prevTsf, err := testutil.SignedTransfer(addr1, addr1, uint64(1), big.NewInt(50),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = ap.Add(prevTsf)
	require.NoError(err)
	sf := bc.GetFactory()
	require.NotNil(sf)
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			ProducerAddr:    testaddress.IotxAddrinfo["producer"].RawAddress,
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{prevTsf})
	require.NoError(err)
	require.Nil(sf.Commit(ws))
	ap.Reset()
	nTsf, err := testutil.SignedTransfer(addr1, addr1, uint64(1), big.NewInt(60),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = validator.Validate(context.Background(), nTsf)
	require.Equal(action.ErrNonce, errors.Cause(err))
}

func TestActPool_AddActs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	require := require.New(t)
	bc := blockchain.NewBlockchain(config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1.RawAddress, big.NewInt(100))
	require.NoError(err)
	_, err = bc.CreateState(addr2.RawAddress, big.NewInt(10))
	require.NoError(err)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	ap.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc),
		execution.NewProtocol(bc))
	// Test actpool status after adding a sequence of Tsfs/votes: need to check confirmed nonce, pending nonce, and pending balance
	tsf1, err := testutil.SignedTransfer(addr1, addr1, uint64(1), big.NewInt(10),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := testutil.SignedTransfer(addr1, addr1, uint64(2), big.NewInt(20),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(addr1, addr1, uint64(3), big.NewInt(30),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	vote4, err := testutil.SignedVote(addr1, addr1, uint64(4), uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf5, err := testutil.SignedTransfer(addr1, addr1, uint64(5), big.NewInt(50),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf6, err := testutil.SignedTransfer(addr2, addr2, uint64(1), big.NewInt(5),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf7, err := testutil.SignedTransfer(addr2, addr2, uint64(3), big.NewInt(1),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf8, err := testutil.SignedTransfer(addr2, addr2, uint64(4), big.NewInt(5),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)

	err = ap.Add(tsf1)
	require.NoError(err)
	err = ap.Add(tsf2)
	require.NoError(err)
	err = ap.Add(tsf3)
	require.NoError(err)
	err = ap.Add(vote4)
	require.NoError(err)
	err = ap.Add(tsf5)
	require.Equal(action.ErrBalance, errors.Cause(err))
	err = ap.Add(tsf6)
	require.NoError(err)
	err = ap.Add(tsf7)
	require.NoError(err)
	err = ap.Add(tsf8)
	require.NoError(err)

	pBalance1, _ := ap.getPendingBalance(addr1.RawAddress)
	require.Equal(uint64(40), pBalance1.Uint64())
	pNonce1, _ := ap.getPendingNonce(addr1.RawAddress)
	require.Equal(uint64(5), pNonce1)

	pBalance2, _ := ap.getPendingBalance(addr2.RawAddress)
	require.Equal(uint64(5), pBalance2.Uint64())
	pNonce2, _ := ap.getPendingNonce(addr2.RawAddress)
	require.Equal(uint64(2), pNonce2)

	tsf9, err := testutil.SignedTransfer(addr2, addr2, uint64(2), big.NewInt(3),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = ap.Add(tsf9)
	require.NoError(err)
	pBalance2, _ = ap.getPendingBalance(addr2.RawAddress)
	require.Equal(uint64(1), pBalance2.Uint64())
	pNonce2, _ = ap.getPendingNonce(addr2.RawAddress)
	require.Equal(uint64(4), pNonce2)
	// Error Case Handling
	// Case I: Action already exists in pool
	err = ap.Add(tsf1)
	require.Equal(fmt.Errorf("reject existed action: %x", tsf1.Hash()), err)
	err = ap.Add(vote4)
	require.Equal(fmt.Errorf("reject existed action: %x", vote4.Hash()), err)
	// Case II: Pool space is full
	mockBC := mock_blockchain.NewMockBlockchain(ctrl)
	Ap2, err := NewActPool(mockBC, apConfig)
	require.NoError(err)
	ap2, ok := Ap2.(*actPool)
	require.True(ok)
	for i := uint64(0); i < ap2.cfg.MaxNumActsPerPool; i++ {
		nTsf, err := testutil.SignedTransfer(addr1, addr2, uint64(i), big.NewInt(50), nil, uint64(0), big.NewInt(0))
		require.NoError(err)

		ap2.allActions[nTsf.Hash()] = nTsf
	}
	err = ap2.Add(tsf1)
	require.Equal(action.ErrActPool, errors.Cause(err))
	err = ap2.Add(vote4)
	require.Equal(action.ErrActPool, errors.Cause(err))
	// Case III: Nonce already exists
	replaceTsf, err := testutil.SignedTransfer(addr1, addr2, uint64(1), big.NewInt(1),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = ap.Add(replaceTsf)
	require.Equal(action.ErrNonce, errors.Cause(err))
	replaceVote, err := action.NewVote(4, addr1.RawAddress, "", uint64(100000), big.NewInt(0))
	require.NoError(err)

	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(4).
		SetAction(replaceVote).
		SetGasLimit(100000).
		SetDestinationAddress("").Build()
	selp, err := action.Sign(elp, addr1.RawAddress, addr1.PrivateKey)

	require.NoError(err)

	err = ap.Add(selp)
	require.Equal(action.ErrNonce, errors.Cause(err))
	// Case IV: Nonce is too large
	outOfBoundsTsf, err := testutil.SignedTransfer(addr1, addr1, ap.cfg.MaxNumActsPerAcct+1, big.NewInt(1),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = ap.Add(outOfBoundsTsf)
	require.Equal(action.ErrNonce, errors.Cause(err))
	// Case V: Insufficient balance
	overBalTsf, err := testutil.SignedTransfer(addr2, addr2, uint64(4), big.NewInt(20),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = ap.Add(overBalTsf)
	require.Equal(action.ErrBalance, errors.Cause(err))
	// Case VI: over gas limit
	creationExecution, err := action.NewExecution(addr1.RawAddress, action.EmptyAddress, uint64(5), big.NewInt(int64(0)), blockchain.GasLimit+100, big.NewInt(10), []byte{})
	require.NoError(err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(5).
		SetGasPrice(big.NewInt(10)).
		SetGasLimit(blockchain.GasLimit + 100).
		SetAction(creationExecution).
		SetDestinationAddress(action.EmptyAddress).Build()
	selp, err = action.Sign(elp, addr1.RawAddress, addr1.PrivateKey)
	require.NoError(err)

	err = ap.Add(selp)
	require.Equal(action.ErrGasHigherThanLimit, errors.Cause(err))
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

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(5).
		SetGasPrice(big.NewInt(10)).
		SetGasLimit(10).
		SetAction(creationExecution).
		SetDestinationAddress(action.EmptyAddress).Build()
	selp, err = action.Sign(elp, addr1.RawAddress, addr1.PrivateKey)
	require.NoError(err)

	err = ap.Add(selp)
	require.Equal(action.ErrInsufficientBalanceForGas, errors.Cause(err))
}

func TestActPool_PickActs(t *testing.T) {
	createActPool := func(cfg config.ActPool) (*actPool, []action.SealedEnvelope, []action.SealedEnvelope, []action.SealedEnvelope) {
		require := require.New(t)
		bc := blockchain.NewBlockchain(config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
		require.NoError(bc.Start(context.Background()))
		_, err := bc.CreateState(addr1.RawAddress, big.NewInt(100))
		require.NoError(err)
		_, err = bc.CreateState(addr2.RawAddress, big.NewInt(10))
		require.NoError(err)
		// Create actpool
		Ap, err := NewActPool(bc, cfg)
		require.NoError(err)
		ap, ok := Ap.(*actPool)
		require.True(ok)
		ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
		ap.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))

		tsf1, err := testutil.SignedTransfer(addr1, addr1, uint64(1), big.NewInt(10),
			[]byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf2, err := testutil.SignedTransfer(addr1, addr1, uint64(2), big.NewInt(20),
			[]byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf3, err := testutil.SignedTransfer(addr1, addr1, uint64(3), big.NewInt(30),
			[]byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf4, err := testutil.SignedTransfer(addr1, addr1, uint64(4), big.NewInt(40),
			[]byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf5, err := testutil.SignedTransfer(addr1, addr1, uint64(5), big.NewInt(50),
			[]byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		vote6, err := testutil.SignedVote(addr1, addr1, uint64(6), uint64(100000), big.NewInt(0))
		require.NoError(err)
		vote7, err := testutil.SignedVote(addr2, addr2, uint64(1), uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf8, err := testutil.SignedTransfer(addr2, addr2, uint64(3), big.NewInt(5),
			[]byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf9, err := testutil.SignedTransfer(addr2, addr2, uint64(4), big.NewInt(1),
			[]byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf10, err := testutil.SignedTransfer(addr2, addr2, uint64(5), big.NewInt(5),
			[]byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)

		err = ap.Add(tsf1)
		require.NoError(err)
		err = ap.Add(tsf2)
		require.NoError(err)
		err = ap.Add(tsf3)
		require.NoError(err)
		err = ap.Add(tsf4)
		require.NoError(err)
		err = ap.Add(tsf5)
		require.Equal(action.ErrBalance, errors.Cause(err))
		err = ap.Add(vote6)
		require.NoError(err)
		err = ap.Add(vote7)
		require.NoError(err)
		err = ap.Add(tsf8)
		require.NoError(err)
		err = ap.Add(tsf9)
		require.NoError(err)
		err = ap.Add(tsf10)
		require.NoError(err)
		return ap, []action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4}, []action.SealedEnvelope{vote7}, []action.SealedEnvelope{}
	}

	t.Run("no-limit", func(t *testing.T) {
		apConfig := getActPoolCfg()
		ap, transfers, votes, executions := createActPool(apConfig)
		pickedActs := ap.PickActs()
		require.Equal(t, len(transfers)+len(votes)+len(executions), len(pickedActs))
	})
	t.Run("enough-limit", func(t *testing.T) {
		apConfig := getActPoolCfg()
		apConfig.MaxNumActsToPick = 10
		ap, transfers, votes, executions := createActPool(apConfig)
		pickedActs := ap.PickActs()
		require.Equal(t, len(transfers)+len(votes)+len(executions), len(pickedActs))
	})
	t.Run("low-limit", func(t *testing.T) {
		apConfig := getActPoolCfg()
		apConfig.MaxNumActsToPick = 3
		ap, _, _, _ := createActPool(apConfig)
		pickedActs := ap.PickActs()
		require.Equal(t, 3, len(pickedActs))
	})
}

func TestActPool_removeConfirmedActs(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	bc.GetFactory().AddActionHandlers(account.NewProtocol(), vote.NewProtocol(bc))
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1.RawAddress, big.NewInt(100))
	require.NoError(err)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	ap.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))

	tsf1, err := testutil.SignedTransfer(addr1, addr1, uint64(1), big.NewInt(10),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := testutil.SignedTransfer(addr1, addr1, uint64(2), big.NewInt(20),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(addr1, addr1, uint64(3), big.NewInt(30),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	vote4, err := testutil.SignedVote(addr1, addr1, uint64(4), uint64(100000), big.NewInt(0))
	require.NoError(err)

	err = ap.Add(tsf1)
	require.NoError(err)
	err = ap.Add(tsf2)
	require.NoError(err)
	err = ap.Add(tsf3)
	require.NoError(err)
	err = ap.Add(vote4)
	require.NoError(err)

	require.Equal(4, len(ap.allActions))
	require.NotNil(ap.accountActs[addr1.RawAddress])
	sf := bc.GetFactory()
	require.NotNil(sf)
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			ProducerAddr:    testaddress.IotxAddrinfo["producer"].RawAddress,
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{tsf1, tsf2, tsf3, vote4})
	require.NoError(err)
	require.Nil(sf.Commit(ws))
	ap.removeConfirmedActs()
	require.Equal(0, len(ap.allActions))
	require.Nil(ap.accountActs[addr1.RawAddress])
}

func TestActPool_Reset(t *testing.T) {
	require := require.New(t)

	bc := blockchain.NewBlockchain(config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	bc.GetFactory().AddActionHandlers(account.NewProtocol(), vote.NewProtocol(bc))
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1.RawAddress, big.NewInt(100))
	require.NoError(err)
	_, err = bc.CreateState(addr2.RawAddress, big.NewInt(200))
	require.NoError(err)
	_, err = bc.CreateState(addr3.RawAddress, big.NewInt(300))
	require.NoError(err)

	apConfig := getActPoolCfg()
	Ap1, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap1, ok := Ap1.(*actPool)
	require.True(ok)
	ap1.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	ap1.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc),
		execution.NewProtocol(bc))
	Ap2, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap2, ok := Ap2.(*actPool)
	require.True(ok)
	ap2.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	ap2.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc),
		execution.NewProtocol(bc))

	// Tsfs to be added to ap1
	tsf1, err := testutil.SignedTransfer(addr1, addr2, uint64(1), big.NewInt(50),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := testutil.SignedTransfer(addr1, addr3, uint64(2), big.NewInt(30),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(addr1, addr2, uint64(3), big.NewInt(60),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf4, err := testutil.SignedTransfer(addr2, addr1, uint64(1), big.NewInt(100),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf5, err := testutil.SignedTransfer(addr2, addr3, uint64(2), big.NewInt(50),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf6, err := testutil.SignedTransfer(addr2, addr1, uint64(3), big.NewInt(60),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf7, err := testutil.SignedTransfer(addr3, addr1, uint64(1), big.NewInt(100),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf8, err := testutil.SignedTransfer(addr3, addr2, uint64(2), big.NewInt(100),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf9, err := testutil.SignedTransfer(addr3, addr1, uint64(4), big.NewInt(100),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)

	err = ap1.Add(tsf1)
	require.NoError(err)
	err = ap1.Add(tsf2)
	require.NoError(err)
	err = ap1.Add(tsf3)
	require.Equal(action.ErrBalance, errors.Cause(err))
	err = ap1.Add(tsf4)
	require.NoError(err)
	err = ap1.Add(tsf5)
	require.NoError(err)
	err = ap1.Add(tsf6)
	require.Equal(action.ErrBalance, errors.Cause(err))
	err = ap1.Add(tsf7)
	require.NoError(err)
	err = ap1.Add(tsf8)
	require.NoError(err)
	err = ap1.Add(tsf9)
	require.NoError(err)
	// Tsfs to be added to ap2 only
	tsf10, err := testutil.SignedTransfer(addr1, addr2, uint64(3), big.NewInt(20),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf11, err := testutil.SignedTransfer(addr1, addr3, uint64(4), big.NewInt(10),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf12, err := testutil.SignedTransfer(addr2, addr3, uint64(2), big.NewInt(70),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf13, err := testutil.SignedTransfer(addr3, addr1, uint64(1), big.NewInt(200),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf14, err := testutil.SignedTransfer(addr3, addr2, uint64(2), big.NewInt(50),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)

	err = ap2.Add(tsf1)
	require.NoError(err)
	err = ap2.Add(tsf2)
	require.NoError(err)
	err = ap2.Add(tsf10)
	require.NoError(err)
	err = ap2.Add(tsf11)
	require.Equal(action.ErrBalance, errors.Cause(err))
	err = ap2.Add(tsf4)
	require.NoError(err)
	err = ap2.Add(tsf12)
	require.NoError(err)
	err = ap2.Add(tsf13)
	require.NoError(err)
	err = ap2.Add(tsf14)
	require.NoError(err)
	err = ap2.Add(tsf9)
	require.Equal(action.ErrBalance, errors.Cause(err))
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
	pickedActs := ap1.PickActs()
	// ap1 commits update of accounts to trie
	sf := bc.GetFactory()
	require.NotNil(sf)
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			ProducerAddr:    testaddress.IotxAddrinfo["producer"].RawAddress,
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 0, pickedActs)
	require.NoError(err)
	require.Nil(sf.Commit(ws))
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
	tsf15, err := testutil.SignedTransfer(addr3, addr2, uint64(3), big.NewInt(80),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	// Tsfs To be added to ap2 only
	tsf16, err := testutil.SignedTransfer(addr1, addr2, uint64(4), big.NewInt(150),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf17, err := testutil.SignedTransfer(addr2, addr1, uint64(3), big.NewInt(90),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf18, err := testutil.SignedTransfer(addr2, addr3, uint64(4), big.NewInt(100),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf19, err := testutil.SignedTransfer(addr2, addr1, uint64(5), big.NewInt(50),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf20, err := testutil.SignedTransfer(addr3, addr2, uint64(3), big.NewInt(200),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)

	err = ap1.Add(tsf15)
	require.NoError(err)
	err = ap2.Add(tsf16)
	require.NoError(err)
	err = ap2.Add(tsf17)
	require.NoError(err)
	err = ap2.Add(tsf18)
	require.NoError(err)
	err = ap2.Add(tsf19)
	require.Equal(action.ErrBalance, errors.Cause(err))
	err = ap2.Add(tsf20)
	require.Equal(action.ErrBalance, errors.Cause(err))
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
	pickedActs = ap2.PickActs()
	// ap2 commits update of accounts to trie
	ws, err = sf.NewWorkingSet()
	require.NoError(err)
	ctx = protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			ProducerAddr:    testaddress.IotxAddrinfo["producer"].RawAddress,
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 0, pickedActs)
	require.NoError(err)
	require.Nil(sf.Commit(ws))
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
	_, err = bc.CreateState(addr4.RawAddress, big.NewInt(10))
	require.NoError(err)
	_, err = bc.CreateState(addr5.RawAddress, big.NewInt(20))
	require.NoError(err)
	tsf21, err := testutil.SignedTransfer(addr4, addr5, uint64(1), big.NewInt(10),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	vote22, err := testutil.SignedVote(addr4, addr4, uint64(2), uint64(100000), big.NewInt(0))
	require.NoError(err)
	vote23, err := action.NewVote(3, addr4.RawAddress, "",
		uint64(100000), big.NewInt(0))
	require.NoError(err)

	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(3).
		SetGasLimit(100000).
		SetAction(vote23).
		SetDestinationAddress("").Build()
	selp23, err := action.Sign(elp, addr4.RawAddress, addr4.PrivateKey)
	require.NoError(err)

	vote24, err := testutil.SignedVote(addr5, addr5, uint64(1), uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf25, err := testutil.SignedTransfer(addr5, addr4, uint64(2), big.NewInt(10),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	vote26, err := action.NewVote(3, addr5.RawAddress, "", uint64(100000), big.NewInt(0))
	require.NoError(err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(3).
		SetGasLimit(100000).
		SetAction(vote26).
		SetDestinationAddress("").Build()
	selp26, err := action.Sign(elp, addr5.RawAddress, addr5.PrivateKey)
	require.NoError(err)

	err = ap1.Add(tsf21)
	require.NoError(err)
	err = ap1.Add(vote22)
	require.NoError(err)
	err = ap1.Add(selp23)
	require.NoError(err)
	err = ap1.Add(vote24)
	require.NoError(err)
	err = ap1.Add(tsf25)
	require.NoError(err)
	err = ap1.Add(selp26)
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
	pickedActs = ap1.PickActs()
	// ap1 commits update of accounts to trie
	ws, err = sf.NewWorkingSet()
	require.NoError(err)

	ctx = protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			ProducerAddr:    testaddress.IotxAddrinfo["producer"].RawAddress,
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 0, pickedActs)
	require.NoError(err)
	require.Nil(sf.Commit(ws))
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
	bc := blockchain.NewBlockchain(config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1.RawAddress, big.NewInt(100))
	require.NoError(err)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	ap.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))

	tsf1, err := testutil.SignedTransfer(addr1, addr1, uint64(1), big.NewInt(10),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := testutil.SignedTransfer(addr1, addr1, uint64(2), big.NewInt(20),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(addr1, addr1, uint64(3), big.NewInt(30),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	vote4, err := testutil.SignedVote(addr1, addr1, uint64(4), uint64(100000), big.NewInt(0))
	require.NoError(err)

	err = ap.Add(tsf1)
	require.NoError(err)
	err = ap.Add(tsf2)
	require.NoError(err)
	err = ap.Add(tsf3)
	require.NoError(err)
	err = ap.Add(vote4)
	require.NoError(err)

	hash1 := tsf1.Hash()
	hash2 := vote4.Hash()
	acts := []action.SealedEnvelope{tsf1, vote4}
	require.NotNil(ap.allActions[hash1])
	require.NotNil(ap.allActions[hash2])
	ap.removeInvalidActs(acts)
	require.Equal(action.SealedEnvelope{}, ap.allActions[hash1])
	require.Equal(action.SealedEnvelope{}, ap.allActions[hash2])
}

func TestActPool_GetPendingNonce(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1.RawAddress, big.NewInt(100))
	require.NoError(err)
	_, err = bc.CreateState(addr2.RawAddress, big.NewInt(100))
	require.NoError(err)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	ap.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))

	tsf1, err := testutil.SignedTransfer(addr1, addr1, uint64(1), big.NewInt(10),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(addr1, addr1, uint64(3), big.NewInt(30),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	vote4, err := testutil.SignedVote(addr1, addr1, uint64(4), uint64(100000), big.NewInt(0))
	require.NoError(err)

	err = ap.Add(tsf1)
	require.NoError(err)
	err = ap.Add(tsf3)
	require.NoError(err)
	err = ap.Add(vote4)
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
	bc := blockchain.NewBlockchain(config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1.RawAddress, big.NewInt(100))
	require.NoError(err)
	_, err = bc.CreateState(addr2.RawAddress, big.NewInt(100))
	require.NoError(err)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	ap.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))

	tsf1, err := testutil.SignedTransfer(addr1, addr1, uint64(1), big.NewInt(10),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(addr1, addr1, uint64(3), big.NewInt(30),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	vote4, err := testutil.SignedVote(addr1, addr1, uint64(4), uint64(100000), big.NewInt(0))
	require.NoError(err)

	err = ap.Add(tsf1)
	require.NoError(err)
	err = ap.Add(tsf3)
	require.NoError(err)
	err = ap.Add(vote4)
	require.NoError(err)

	acts := ap.GetUnconfirmedActs(addr2.RawAddress)
	require.Equal([]action.SealedEnvelope{}, acts)

	acts = ap.GetUnconfirmedActs(addr1.RawAddress)
	require.Equal([]action.SealedEnvelope{tsf1, tsf3, vote4}, acts)
}

func TestActPool_GetActionByHash(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1.RawAddress, big.NewInt(100))
	require.NoError(err)
	_, err = bc.CreateState(addr2.RawAddress, big.NewInt(100))
	require.NoError(err)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)

	tsf1, err := testutil.SignedTransfer(addr1, addr1, uint64(1), big.NewInt(10),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	hash1 := tsf1.Hash()
	vote2, err := testutil.SignedVote(addr1, addr1, uint64(2), uint64(100000), big.NewInt(0))
	require.NoError(err)
	hash2 := vote2.Hash()

	ap.allActions[hash1] = tsf1
	act, err := ap.GetActionByHash(hash1)
	require.NoError(err)
	require.Equal(tsf1, act)
	act, err = ap.GetActionByHash(hash2)
	require.Equal(action.ErrHash, errors.Cause(err))
	require.Equal(action.SealedEnvelope{}, act)

	ap.allActions[hash2] = vote2
	act, err = ap.GetActionByHash(hash2)
	require.NoError(err)
	require.Equal(vote2, act)
}

func TestActPool_GetCapacity(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
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
	bc := blockchain.NewBlockchain(config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	bc.GetFactory().AddActionHandlers(account.NewProtocol(), vote.NewProtocol(bc))
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1.RawAddress, big.NewInt(100))
	require.NoError(err)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	ap.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))
	require.Zero(ap.GetSize())

	tsf1, err := testutil.SignedTransfer(addr1, addr1, uint64(1), big.NewInt(10),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := testutil.SignedTransfer(addr1, addr1, uint64(2), big.NewInt(20),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(addr1, addr1, uint64(3), big.NewInt(30),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	vote4, err := testutil.SignedVote(addr1, addr1, uint64(4), uint64(100000), big.NewInt(0))
	require.NoError(err)
	require.NoError(ap.Add(tsf1))
	require.NoError(ap.Add(tsf2))
	require.NoError(ap.Add(tsf3))
	require.NoError(ap.Add(vote4))
	require.Equal(uint64(4), ap.GetSize())
	sf := bc.GetFactory()
	require.NotNil(sf)
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	gasLimit := testutil.TestGasLimit

	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			ProducerAddr:    testaddress.IotxAddrinfo["producer"].RawAddress,
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{tsf1, tsf2, tsf3, vote4})
	require.NoError(err)
	require.Nil(sf.Commit(ws))
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

func getActPoolCfg() config.ActPool {
	return config.ActPool{
		MaxNumActsPerPool: maxNumActsPerPool,
		MaxNumActsPerAcct: maxNumActsPerAcct,
	}
}
