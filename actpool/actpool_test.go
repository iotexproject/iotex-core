// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/test/identityset"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	maxNumActsPerPool = 8192
	maxNumActsPerAcct = 256
)

var (
	chainID = config.Default.Chain.ID
	addr1   = testaddress.Addrinfo["alfa"].String()
	pubKey1 = testaddress.Keyinfo["alfa"].PubKey
	priKey1 = testaddress.Keyinfo["alfa"].PriKey
	addr2   = testaddress.Addrinfo["bravo"].String()
	priKey2 = testaddress.Keyinfo["bravo"].PriKey
	addr3   = testaddress.Addrinfo["charlie"].String()
	priKey3 = testaddress.Keyinfo["charlie"].PriKey
	addr4   = testaddress.Addrinfo["delta"].String()
	priKey4 = testaddress.Keyinfo["delta"].PriKey
	addr5   = testaddress.Addrinfo["echo"].String()
	priKey5 = testaddress.Keyinfo["echo"].PriKey
)

func TestActPool_validateGenericAction(t *testing.T) {
	require := require.New(t)

	bc := blockchain.NewBlockchain(
		config.Default,
		blockchain.InMemStateFactoryOption(),
		blockchain.InMemDaoOption(),
	)
	bc.GetFactory().AddActionHandlers(account.NewProtocol())
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1, big.NewInt(100))
	require.NoError(err)
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	validator := ap.actionEnvelopeValidators[0]
	// Case I: Over-gassed transfer
	tsf, err := testutil.SignedTransfer(addr1, priKey1, 1, big.NewInt(1), nil, genesis.Default.ActionGasLimit+1, big.NewInt(0))
	require.NoError(err)

	ctx := protocol.WithValidateActionsCtx(context.Background(), protocol.ValidateActionsCtx{})
	err = validator.Validate(ctx, tsf)
	require.Equal(action.ErrGasHigherThanLimit, errors.Cause(err))
	// Case II: Insufficient gas
	tsf, err = testutil.SignedTransfer(addr1, priKey1, 1, big.NewInt(1), nil, 0, big.NewInt(0))
	require.NoError(err)
	err = validator.Validate(ctx, tsf)
	require.Equal(action.ErrInsufficientBalanceForGas, errors.Cause(err))
	// Case III: Signature verification fails
	unsignedTsf, err := action.NewTransfer(uint64(1), big.NewInt(1), addr1, []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)

	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetAction(unsignedTsf).
		SetGasLimit(100000).Build()
	selp := action.FakeSeal(elp, pubKey1)
	err = validator.Validate(ctx, selp)
	require.True(strings.Contains(err.Error(), "incorrect length of signature"))
	// Case IV: Nonce is too low
	prevTsf, err := testutil.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(50), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = ap.Add(prevTsf)
	require.NoError(err)
	sf := bc.GetFactory()
	require.NotNil(sf)
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	gasLimit := testutil.TestGasLimit
	ctx = protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			GasLimit: gasLimit,
		})
	_, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{prevTsf})
	require.NoError(err)
	require.Nil(sf.Commit(ws))
	ap.Reset()
	nTsf, err := testutil.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(60), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	ctx = protocol.WithValidateActionsCtx(context.Background(), protocol.ValidateActionsCtx{
		Caller: testaddress.Addrinfo["alfa"],
	})
	err = validator.Validate(ctx, nTsf)
	require.Equal(action.ErrNonce, errors.Cause(err))
}

func TestActPool_AddActs(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()
	require := require.New(t)
	bc := blockchain.NewBlockchain(
		config.Default,
		blockchain.InMemStateFactoryOption(),
		blockchain.InMemDaoOption(),
	)
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1, big.NewInt(100))
	require.NoError(err)
	_, err = bc.CreateState(addr2, big.NewInt(10))
	require.NoError(err)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	ap.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc),
		execution.NewProtocol(bc))
	// Test actpool status after adding a sequence of Tsfs/votes: need to check confirmed nonce, pending nonce, and pending balance
	tsf1, err := testutil.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := testutil.SignedTransfer(addr1, priKey1, uint64(2), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(addr1, priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	vote4, err := testutil.SignedVote(addr1, priKey1, uint64(4), uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf5, err := testutil.SignedTransfer(addr1, priKey1, uint64(5), big.NewInt(50), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf6, err := testutil.SignedTransfer(addr2, priKey2, uint64(1), big.NewInt(5), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf7, err := testutil.SignedTransfer(addr2, priKey2, uint64(3), big.NewInt(1), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf8, err := testutil.SignedTransfer(addr2, priKey2, uint64(4), big.NewInt(5), []byte{}, uint64(100000), big.NewInt(0))
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

	pBalance1, _ := ap.getPendingBalance(addr1)
	require.Equal(uint64(40), pBalance1.Uint64())
	pNonce1, _ := ap.getPendingNonce(addr1)
	require.Equal(uint64(5), pNonce1)

	pBalance2, _ := ap.getPendingBalance(addr2)
	require.Equal(uint64(5), pBalance2.Uint64())
	pNonce2, _ := ap.getPendingNonce(addr2)
	require.Equal(uint64(2), pNonce2)

	tsf9, err := testutil.SignedTransfer(addr2, priKey2, uint64(2), big.NewInt(3), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = ap.Add(tsf9)
	require.NoError(err)
	pBalance2, _ = ap.getPendingBalance(addr2)
	require.Equal(uint64(1), pBalance2.Uint64())
	pNonce2, _ = ap.getPendingNonce(addr2)
	require.Equal(uint64(4), pNonce2)
	// Error Case Handling
	// Case I: Action already exists in pool
	err = ap.Add(tsf1)
	require.Error(err)
	err = ap.Add(vote4)
	require.Error(err)
	// Case II: Pool space is full
	mockBC := mock_blockchain.NewMockBlockchain(ctrl)
	Ap2, err := NewActPool(mockBC, apConfig)
	require.NoError(err)
	ap2, ok := Ap2.(*actPool)
	require.True(ok)
	for i := uint64(0); i < ap2.cfg.MaxNumActsPerPool; i++ {
		nTsf, err := testutil.SignedTransfer(addr2, priKey2, i, big.NewInt(50), nil, uint64(0), big.NewInt(0))
		require.NoError(err)

		ap2.allActions[nTsf.Hash()] = nTsf
	}
	err = ap2.Add(tsf1)
	require.Equal(action.ErrActPool, errors.Cause(err))
	err = ap2.Add(vote4)
	require.Equal(action.ErrActPool, errors.Cause(err))
	// Case III: Nonce already exists
	replaceTsf, err := testutil.SignedTransfer(addr2, priKey1, uint64(1), big.NewInt(1), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = ap.Add(replaceTsf)
	require.Equal(action.ErrNonce, errors.Cause(err))
	replaceVote, err := action.NewVote(4, "", uint64(100000), big.NewInt(0))
	require.NoError(err)

	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(4).
		SetAction(replaceVote).
		SetGasLimit(100000).Build()
	selp, err := action.Sign(elp, priKey1)

	require.NoError(err)

	err = ap.Add(selp)
	require.Equal(action.ErrNonce, errors.Cause(err))
	// Case IV: Nonce is too large
	outOfBoundsTsf, err := testutil.SignedTransfer(addr1, priKey1, ap.cfg.MaxNumActsPerAcct+1, big.NewInt(1), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = ap.Add(outOfBoundsTsf)
	require.Equal(action.ErrNonce, errors.Cause(err))
	// Case V: Insufficient balance
	overBalTsf, err := testutil.SignedTransfer(addr2, priKey2, uint64(4), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = ap.Add(overBalTsf)
	require.Equal(action.ErrBalance, errors.Cause(err))
	// Case VI: over gas limit
	creationExecution, err := action.NewExecution(
		action.EmptyAddress,
		uint64(5),
		big.NewInt(int64(0)),
		genesis.Default.ActionGasLimit,
		big.NewInt(10),
		[]byte{},
	)
	require.NoError(err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(5).
		SetGasPrice(big.NewInt(10)).
		SetGasLimit(genesis.Default.ActionGasLimit + 1).
		SetAction(creationExecution).Build()
	selp, err = action.Sign(elp, priKey1)
	require.NoError(err)

	err = ap.Add(selp)
	require.Equal(action.ErrGasHigherThanLimit, errors.Cause(err))
	// Case VII: insufficient gas
	tmpData := [1234]byte{}
	creationExecution, err = action.NewExecution(
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
		SetAction(creationExecution).Build()
	selp, err = action.Sign(elp, priKey1)
	require.NoError(err)

	err = ap.Add(selp)
	require.Equal(action.ErrInsufficientBalanceForGas, errors.Cause(err))
}

func TestActPool_PickActs(t *testing.T) {
	createActPool := func(cfg config.ActPool) (*actPool, []action.SealedEnvelope, []action.SealedEnvelope, []action.SealedEnvelope) {
		require := require.New(t)
		bc := blockchain.NewBlockchain(
			config.Default,
			blockchain.InMemStateFactoryOption(),
			blockchain.InMemDaoOption(),
		)
		require.NoError(bc.Start(context.Background()))
		_, err := bc.CreateState(addr1, big.NewInt(100))
		require.NoError(err)
		_, err = bc.CreateState(addr2, big.NewInt(10))
		require.NoError(err)
		// Create actpool
		Ap, err := NewActPool(bc, cfg)
		require.NoError(err)
		ap, ok := Ap.(*actPool)
		require.True(ok)
		ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
		ap.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))

		tsf1, err := testutil.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf2, err := testutil.SignedTransfer(addr1, priKey1, uint64(2), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf3, err := testutil.SignedTransfer(addr1, priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf4, err := testutil.SignedTransfer(addr1, priKey1, uint64(4), big.NewInt(40), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf5, err := testutil.SignedTransfer(addr1, priKey1, uint64(5), big.NewInt(50), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		vote6, err := testutil.SignedVote(addr1, priKey1, uint64(6), uint64(100000), big.NewInt(0))
		require.NoError(err)
		vote7, err := testutil.SignedVote(addr2, priKey2, uint64(1), uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf8, err := testutil.SignedTransfer(addr2, priKey2, uint64(3), big.NewInt(5), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf9, err := testutil.SignedTransfer(addr2, priKey2, uint64(4), big.NewInt(1), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf10, err := testutil.SignedTransfer(addr2, priKey2, uint64(5), big.NewInt(5), []byte{}, uint64(100000), big.NewInt(0))
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

	t.Run("no-expiry", func(t *testing.T) {
		apConfig := getActPoolCfg()
		ap, transfers, votes, executions := createActPool(apConfig)
		pickedActs := ap.PendingActionMap()
		require.Equal(t, len(transfers)+len(votes)+len(executions), lenPendingActionMap(pickedActs))
	})

	t.Run("expiry", func(t *testing.T) {
		apConfig := getActPoolCfg()
		apConfig.ActionExpiry = time.Second
		ap, _, _, _ := createActPool(apConfig)
		require.NoError(t, testutil.WaitUntil(100*time.Millisecond, 10*time.Second, func() (bool, error) {
			pickedActs := ap.PendingActionMap()
			return lenPendingActionMap(pickedActs) == 0, nil
		}))
	})
}

func TestActPool_removeConfirmedActs(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(
		config.Default,
		blockchain.InMemStateFactoryOption(),
		blockchain.InMemDaoOption(),
	)
	bc.GetFactory().AddActionHandlers(account.NewProtocol(), vote.NewProtocol(bc))
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1, big.NewInt(100))
	require.NoError(err)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	ap.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))

	tsf1, err := testutil.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := testutil.SignedTransfer(addr1, priKey1, uint64(2), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(addr1, priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	vote4, err := testutil.SignedVote(addr1, priKey1, uint64(4), uint64(100000), big.NewInt(0))
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
	require.NotNil(ap.accountActs[addr1])
	sf := bc.GetFactory()
	require.NotNil(sf)
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	gasLimit := uint64(1000000)
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			GasLimit: gasLimit,
		})
	_, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{tsf1, tsf2, tsf3, vote4})
	require.NoError(err)
	require.Nil(sf.Commit(ws))
	ap.removeConfirmedActs()
	require.Equal(0, len(ap.allActions))
	require.Nil(ap.accountActs[addr1])
}

func TestActPool_Reset(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(
		config.Default,
		blockchain.InMemStateFactoryOption(),
		blockchain.InMemDaoOption(),
	)
	bc.GetFactory().AddActionHandlers(account.NewProtocol(), vote.NewProtocol(bc))
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1, big.NewInt(100))
	require.NoError(err)
	_, err = bc.CreateState(addr2, big.NewInt(200))
	require.NoError(err)
	_, err = bc.CreateState(addr3, big.NewInt(300))
	require.NoError(err)

	apConfig := getActPoolCfg()
	Ap1, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap1, ok := Ap1.(*actPool)
	require.True(ok)
	ap1.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	ap1.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc),
		execution.NewProtocol(bc))
	Ap2, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap2, ok := Ap2.(*actPool)
	require.True(ok)
	ap2.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	ap2.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc),
		execution.NewProtocol(bc))

	// Tsfs to be added to ap1
	tsf1, err := testutil.SignedTransfer(addr2, priKey1, uint64(1), big.NewInt(50), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := testutil.SignedTransfer(addr3, priKey1, uint64(2), big.NewInt(30), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(addr2, priKey1, uint64(3), big.NewInt(60), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf4, err := testutil.SignedTransfer(addr1, priKey2, uint64(1), big.NewInt(100), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf5, err := testutil.SignedTransfer(addr3, priKey2, uint64(2), big.NewInt(50), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf6, err := testutil.SignedTransfer(addr1, priKey2, uint64(3), big.NewInt(60), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf7, err := testutil.SignedTransfer(addr1, priKey3, uint64(1), big.NewInt(100), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf8, err := testutil.SignedTransfer(addr2, priKey3, uint64(2), big.NewInt(100), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf9, err := testutil.SignedTransfer(addr1, priKey3, uint64(4), big.NewInt(100), []byte{}, uint64(20000), big.NewInt(0))
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
	tsf10, err := testutil.SignedTransfer(addr2, priKey1, uint64(3), big.NewInt(20), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf11, err := testutil.SignedTransfer(addr3, priKey1, uint64(4), big.NewInt(10), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf12, err := testutil.SignedTransfer(addr3, priKey2, uint64(2), big.NewInt(70), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf13, err := testutil.SignedTransfer(addr1, priKey3, uint64(1), big.NewInt(200), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf14, err := testutil.SignedTransfer(addr2, priKey3, uint64(2), big.NewInt(50), []byte{}, uint64(20000), big.NewInt(0))
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
	ap1PNonce1, _ := ap1.getPendingNonce(addr1)
	require.Equal(uint64(3), ap1PNonce1)
	ap1PBalance1, _ := ap1.getPendingBalance(addr1)
	require.Equal(big.NewInt(20).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1PNonce2, _ := ap1.getPendingNonce(addr2)
	require.Equal(uint64(3), ap1PNonce2)
	ap1PBalance2, _ := ap1.getPendingBalance(addr2)
	require.Equal(big.NewInt(50).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1PNonce3, _ := ap1.getPendingNonce(addr3)
	require.Equal(uint64(3), ap1PNonce3)
	ap1PBalance3, _ := ap1.getPendingBalance(addr3)
	require.Equal(big.NewInt(100).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2PNonce1, _ := ap2.getPendingNonce(addr1)
	require.Equal(uint64(4), ap2PNonce1)
	ap2PBalance1, _ := ap2.getPendingBalance(addr1)
	require.Equal(big.NewInt(0).Uint64(), ap2PBalance1.Uint64())
	// Addr2
	ap2PNonce2, _ := ap2.getPendingNonce(addr2)
	require.Equal(uint64(3), ap2PNonce2)
	ap2PBalance2, _ := ap2.getPendingBalance(addr2)
	require.Equal(big.NewInt(30).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2PNonce3, _ := ap2.getPendingNonce(addr3)
	require.Equal(uint64(3), ap2PNonce3)
	ap2PBalance3, _ := ap2.getPendingBalance(addr3)
	require.Equal(big.NewInt(50).Uint64(), ap2PBalance3.Uint64())
	// Let ap1 be BP's actpool
	pickedActs := ap1.PendingActionMap()
	// ap1 commits update of accounts to trie
	sf := bc.GetFactory()
	require.NotNil(sf)
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	gasLimit := uint64(1000000)
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			GasLimit: gasLimit,
		})
	_, err = ws.RunActions(ctx, 0, actionMap2Slice(pickedActs))
	require.NoError(err)
	require.Nil(sf.Commit(ws))
	//Reset
	ap1.Reset()
	ap2.Reset()
	// Check confirmed nonce, pending nonce, and pending balance after resetting actpool for each account
	// ap1
	// Addr1
	ap1PNonce1, _ = ap1.getPendingNonce(addr1)
	require.Equal(uint64(3), ap1PNonce1)
	ap1PBalance1, _ = ap1.getPendingBalance(addr1)
	require.Equal(big.NewInt(220).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1PNonce2, _ = ap1.getPendingNonce(addr2)
	require.Equal(uint64(3), ap1PNonce2)
	ap1PBalance2, _ = ap1.getPendingBalance(addr2)
	require.Equal(big.NewInt(200).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1PNonce3, _ = ap1.getPendingNonce(addr3)
	require.Equal(uint64(3), ap1PNonce3)
	ap1PBalance3, _ = ap1.getPendingBalance(addr3)
	require.Equal(big.NewInt(180).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2PNonce1, _ = ap2.getPendingNonce(addr1)
	require.Equal(uint64(4), ap2PNonce1)
	ap2PBalance1, _ = ap2.getPendingBalance(addr1)
	require.Equal(big.NewInt(200).Uint64(), ap2PBalance1.Uint64())
	// Addr2
	ap2PNonce2, _ = ap2.getPendingNonce(addr2)
	require.Equal(uint64(3), ap2PNonce2)
	ap2PBalance2, _ = ap2.getPendingBalance(addr2)
	require.Equal(big.NewInt(200).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2PNonce3, _ = ap2.getPendingNonce(addr3)
	require.Equal(uint64(3), ap2PNonce3)
	ap2PBalance3, _ = ap2.getPendingBalance(addr3)
	require.Equal(big.NewInt(180).Uint64(), ap2PBalance3.Uint64())
	// Add more Tsfs after resetting
	// Tsfs To be added to ap1 only
	tsf15, err := testutil.SignedTransfer(addr2, priKey3, uint64(3), big.NewInt(80), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	// Tsfs To be added to ap2 only
	tsf16, err := testutil.SignedTransfer(addr2, priKey1, uint64(4), big.NewInt(150), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf17, err := testutil.SignedTransfer(addr1, priKey2, uint64(3), big.NewInt(90), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf18, err := testutil.SignedTransfer(addr3, priKey2, uint64(4), big.NewInt(100), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf19, err := testutil.SignedTransfer(addr1, priKey2, uint64(5), big.NewInt(50), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf20, err := testutil.SignedTransfer(addr2, priKey3, uint64(3), big.NewInt(200), []byte{}, uint64(20000), big.NewInt(0))
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
	ap1PNonce1, _ = ap1.getPendingNonce(addr1)
	require.Equal(uint64(3), ap1PNonce1)
	ap1PBalance1, _ = ap1.getPendingBalance(addr1)
	require.Equal(big.NewInt(220).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1PNonce2, _ = ap1.getPendingNonce(addr2)
	require.Equal(uint64(3), ap1PNonce2)
	ap1PBalance2, _ = ap1.getPendingBalance(addr2)
	require.Equal(big.NewInt(200).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1PNonce3, _ = ap1.getPendingNonce(addr3)
	require.Equal(uint64(5), ap1PNonce3)
	ap1PBalance3, _ = ap1.getPendingBalance(addr3)
	require.Equal(big.NewInt(0).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2PNonce1, _ = ap2.getPendingNonce(addr1)
	require.Equal(uint64(5), ap2PNonce1)
	ap2PBalance1, _ = ap2.getPendingBalance(addr1)
	require.Equal(big.NewInt(50).Uint64(), ap2PBalance1.Uint64())
	// Addr2
	ap2PNonce2, _ = ap2.getPendingNonce(addr2)
	require.Equal(uint64(5), ap2PNonce2)
	ap2PBalance2, _ = ap2.getPendingBalance(addr2)
	require.Equal(big.NewInt(10).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2PNonce3, _ = ap2.getPendingNonce(addr3)
	require.Equal(uint64(3), ap2PNonce3)
	ap2PBalance3, _ = ap2.getPendingBalance(addr3)
	require.Equal(big.NewInt(180).Uint64(), ap2PBalance3.Uint64())
	// Let ap2 be BP's actpool
	pickedActs = ap2.PendingActionMap()
	// ap2 commits update of accounts to trie
	ws, err = sf.NewWorkingSet()
	require.NoError(err)
	ctx = protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			GasLimit: gasLimit,
		})
	_, err = ws.RunActions(ctx, 0, actionMap2Slice(pickedActs))
	require.NoError(err)
	require.Nil(sf.Commit(ws))
	//Reset
	ap1.Reset()
	ap2.Reset()
	// Check confirmed nonce, pending nonce, and pending balance after resetting actpool for each account
	// ap1
	// Addr1
	ap1PNonce1, _ = ap1.getPendingNonce(addr1)
	require.Equal(uint64(5), ap1PNonce1)
	ap1PBalance1, _ = ap1.getPendingBalance(addr1)
	require.Equal(big.NewInt(140).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1PNonce2, _ = ap1.getPendingNonce(addr2)
	require.Equal(uint64(5), ap1PNonce2)
	ap1PBalance2, _ = ap1.getPendingBalance(addr2)
	require.Equal(big.NewInt(180).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1PNonce3, _ = ap1.getPendingNonce(addr3)
	require.Equal(uint64(5), ap1PNonce3)
	ap1PBalance3, _ = ap1.getPendingBalance(addr3)
	require.Equal(big.NewInt(100).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2PNonce1, _ = ap2.getPendingNonce(addr1)
	require.Equal(uint64(5), ap2PNonce1)
	ap2PBalance1, _ = ap2.getPendingBalance(addr1)
	require.Equal(big.NewInt(140).Uint64(), ap2PBalance1.Uint64())
	// Addr2
	ap2PNonce2, _ = ap2.getPendingNonce(addr2)
	require.Equal(uint64(5), ap2PNonce2)
	ap2PBalance2, _ = ap2.getPendingBalance(addr2)
	require.Equal(big.NewInt(180).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2PNonce3, _ = ap2.getPendingNonce(addr3)
	require.Equal(uint64(3), ap2PNonce3)
	ap2PBalance3, _ = ap2.getPendingBalance(addr3)
	require.Equal(big.NewInt(280).Uint64(), ap2PBalance3.Uint64())

	// Add two more players
	_, err = bc.CreateState(addr4, big.NewInt(10))
	require.NoError(err)
	_, err = bc.CreateState(addr5, big.NewInt(20))
	require.NoError(err)
	tsf21, err := testutil.SignedTransfer(addr5, priKey4, uint64(1), big.NewInt(10), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	vote22, err := testutil.SignedVote(addr4, priKey4, uint64(2), uint64(20000), big.NewInt(0))
	require.NoError(err)
	vote23, err := action.NewVote(3, "", uint64(20000), big.NewInt(0))
	require.NoError(err)

	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(3).
		SetGasLimit(20000).
		SetAction(vote23).Build()
	selp23, err := action.Sign(elp, priKey4)
	require.NoError(err)

	vote24, err := testutil.SignedVote(addr5, priKey5, uint64(1), uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf25, err := testutil.SignedTransfer(addr4, priKey5, uint64(2), big.NewInt(10), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	vote26, err := action.NewVote(3, "", uint64(20000), big.NewInt(0))
	require.NoError(err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(3).
		SetGasLimit(20000).
		SetAction(vote26).Build()
	selp26, err := action.Sign(elp, priKey5)
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
	ap1PNonce4, _ := ap1.getPendingNonce(addr4)
	require.Equal(uint64(4), ap1PNonce4)
	ap1PBalance4, _ := ap1.getPendingBalance(addr4)
	require.Equal(big.NewInt(0).Uint64(), ap1PBalance4.Uint64())
	// Addr5
	ap1PNonce5, _ := ap1.getPendingNonce(addr5)
	require.Equal(uint64(4), ap1PNonce5)
	ap1PBalance5, _ := ap1.getPendingBalance(addr5)
	require.Equal(big.NewInt(10).Uint64(), ap1PBalance5.Uint64())
	// Let ap1 be BP's actpool
	pickedActs = ap1.PendingActionMap()
	// ap1 commits update of accounts to trie
	ws, err = sf.NewWorkingSet()
	require.NoError(err)

	ctx = protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			GasLimit: gasLimit,
		})
	_, err = ws.RunActions(ctx, 0, actionMap2Slice(pickedActs))
	require.NoError(err)
	require.Nil(sf.Commit(ws))
	//Reset
	ap1.Reset()
	// Check confirmed nonce, pending nonce, and pending balance after resetting actpool for each account
	// ap1
	// Addr4
	ap1PNonce4, _ = ap1.getPendingNonce(addr4)
	require.Equal(uint64(4), ap1PNonce4)
	ap1PBalance4, _ = ap1.getPendingBalance(addr4)
	require.Equal(big.NewInt(10).Uint64(), ap1PBalance4.Uint64())
	// Addr5
	ap1PNonce5, _ = ap1.getPendingNonce(addr5)
	require.Equal(uint64(4), ap1PNonce5)
	ap1PBalance5, _ = ap1.getPendingBalance(addr5)
	require.Equal(big.NewInt(20).Uint64(), ap1PBalance5.Uint64())
}

func TestActPool_removeInvalidActs(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(
		config.Default,
		blockchain.InMemStateFactoryOption(),
		blockchain.InMemDaoOption(),
	)
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1, big.NewInt(100))
	require.NoError(err)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	ap.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))

	tsf1, err := testutil.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := testutil.SignedTransfer(addr1, priKey1, uint64(2), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(addr1, priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	vote4, err := testutil.SignedVote(addr1, priKey1, uint64(4), uint64(100000), big.NewInt(0))
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
	bc := blockchain.NewBlockchain(
		config.Default,
		blockchain.InMemStateFactoryOption(),
		blockchain.InMemDaoOption(),
	)
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1, big.NewInt(100))
	require.NoError(err)
	_, err = bc.CreateState(addr2, big.NewInt(100))
	require.NoError(err)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	ap.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))

	tsf1, err := testutil.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(addr1, priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	vote4, err := testutil.SignedVote(addr1, priKey1, uint64(4), uint64(100000), big.NewInt(0))
	require.NoError(err)

	err = ap.Add(tsf1)
	require.NoError(err)
	err = ap.Add(tsf3)
	require.NoError(err)
	err = ap.Add(vote4)
	require.NoError(err)

	nonce, err := ap.GetPendingNonce(addr2)
	require.NoError(err)
	require.Equal(uint64(1), nonce)

	nonce, err = ap.GetPendingNonce(addr1)
	require.NoError(err)
	require.Equal(uint64(2), nonce)
}

func TestActPool_GetUnconfirmedActs(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(
		config.Default,
		blockchain.InMemStateFactoryOption(),
		blockchain.InMemDaoOption(),
	)
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1, big.NewInt(100))
	require.NoError(err)
	_, err = bc.CreateState(addr2, big.NewInt(100))
	require.NoError(err)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	ap.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))

	tsf1, err := testutil.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(addr1, priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	vote4, err := testutil.SignedVote(addr1, priKey1, uint64(4), uint64(100000), big.NewInt(0))
	require.NoError(err)

	err = ap.Add(tsf1)
	require.NoError(err)
	err = ap.Add(tsf3)
	require.NoError(err)
	err = ap.Add(vote4)
	require.NoError(err)

	acts := ap.GetUnconfirmedActs(addr2)
	require.Equal([]action.SealedEnvelope{}, acts)

	acts = ap.GetUnconfirmedActs(addr1)
	require.Equal([]action.SealedEnvelope{tsf1, tsf3, vote4}, acts)
}

func TestActPool_GetActionByHash(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(
		config.Default,
		blockchain.InMemStateFactoryOption(),
		blockchain.InMemDaoOption(),
	)
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1, big.NewInt(100))
	require.NoError(err)
	_, err = bc.CreateState(addr2, big.NewInt(100))
	require.NoError(err)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)

	tsf1, err := testutil.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	hash1 := tsf1.Hash()
	vote2, err := testutil.SignedVote(addr1, priKey1, uint64(2), uint64(100000), big.NewInt(0))
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
	bc := blockchain.NewBlockchain(
		config.Default,
		blockchain.InMemStateFactoryOption(),
		blockchain.InMemDaoOption(),
	)
	bc.GetFactory().AddActionHandlers(account.NewProtocol(), vote.NewProtocol(bc))
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(addr1, big.NewInt(100))
	require.NoError(err)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(bc, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	ap.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))
	require.Zero(ap.GetSize())

	tsf1, err := testutil.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := testutil.SignedTransfer(addr1, priKey1, uint64(2), big.NewInt(20), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(addr1, priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	vote4, err := testutil.SignedVote(addr1, priKey1, uint64(4), uint64(20000), big.NewInt(0))
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
	gasLimit := uint64(1000000)

	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			GasLimit: gasLimit,
		})
	_, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{tsf1, tsf2, tsf3, vote4})
	require.NoError(err)
	require.Nil(sf.Commit(ws))
	ap.removeConfirmedActs()
	require.Equal(uint64(0), ap.GetSize())
}

func TestActPool_AddActionNotEnoughGasPride(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	bc := blockchain.NewBlockchain(
		config.Default,
		blockchain.InMemStateFactoryOption(),
		blockchain.InMemDaoOption(),
	)
	require.NoError(t, bc.Start(context.Background()))
	defer func() {
		require.NoError(t, bc.Stop(context.Background()))
	}()

	cfg := config.Default.ActPool
	ap, err := NewActPool(bc, cfg)
	require.NoError(t, err)
	tsf, err := testutil.SignedTransfer(
		identityset.Address(0).String(),
		identityset.PrivateKey(1),
		uint64(1),
		big.NewInt(10),
		[]byte{},
		uint64(20000),
		big.NewInt(0),
	)
	require.Error(t, ap.Add(tsf))
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
		MinGasPriceStr:    "0",
	}
}

func actionMap2Slice(actMap map[string][]action.SealedEnvelope) []action.SealedEnvelope {
	acts := make([]action.SealedEnvelope, 0)
	for _, parts := range actMap {
		acts = append(acts, parts...)
	}
	return acts
}

func lenPendingActionMap(acts map[string][]action.SealedEnvelope) int {
	l := 0
	for _, part := range acts {
		l += len(part)
	}
	return l
}
