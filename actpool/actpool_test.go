// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"bytes"
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/actpool/actioniterator"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/test/mock/mock_sealed_envelope_validator"
)

const (
	_maxNumActsPerPool  = 8192
	_maxGasLimitPerPool = 81920000
	_maxNumActsPerAcct  = 256
)

var (
	_addr1   = identityset.Address(28).String()
	_priKey1 = identityset.PrivateKey(28)
	_addr2   = identityset.Address(29).String()
	_priKey2 = identityset.PrivateKey(29)
	_addr3   = identityset.Address(30).String()
	_priKey3 = identityset.PrivateKey(30)
	_addr4   = identityset.Address(31).String()
	_priKey4 = identityset.PrivateKey(31)
	_addr5   = identityset.Address(32).String()
	_priKey5 = identityset.PrivateKey(32)
	_addr6   = identityset.Address(33).String()
	_priKey6 = identityset.PrivateKey(33)
)

func TestActPool_NewActPool(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)

	//error caused by nil blockchain
	_, err := NewActPool(genesis.Default, nil, DefaultConfig, nil)
	require.Error(err)

	// all good
	require.Panics(func() { blockchain.NewBlockchain(blockchain.DefaultConfig, genesis.Default, nil, nil, nil) }, "option is nil")
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	act, err := NewActPool(genesis.Default, sf, DefaultConfig)
	require.NoError(err)
	require.NotNil(act)

	// panic caused by option is nil
	require.Panics(func() { NewActPool(genesis.Default, sf, DefaultConfig, nil) }, "option is nil")

	// error caused by option
	opt2 := func(pool *actPool) error {
		return errors.New("test error")
	}
	_, err = NewActPool(genesis.Default, sf, DefaultConfig, opt2)
	require.Error(err)

	// test AddAction nil
	require.NotPanics(func() { act.AddActionEnvelopeValidators(nil) }, "option is nil")
}

func TestValidate(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	g := genesis.Default
	g.InitBalanceMap[_addr1] = "100"
	re := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(re))
	ctx := genesis.WithGenesisContext(
		protocol.WithRegistry(context.Background(), re),
		g,
	)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	sev := mock_sealed_envelope_validator.NewMockSealedEnvelopeValidator(ctrl)
	mockError := errors.New("mock error")
	sev.EXPECT().Validate(gomock.Any(), gomock.Any()).Return(mockError).Times(1)
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(g, sf, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(sev)
	// Case 0: Blacklist
	tsfFromBL, err := action.SignedTransfer(_addr6, _priKey6, 1, big.NewInt(1), nil, 0, big.NewInt(0))
	require.NoError(err)
	require.Equal(action.ErrAddress, errors.Cause(ap.Validate(ctx, tsfFromBL)))
	// Case I: failed by sealed envelope validator
	tsf, err := action.SignedTransfer(_addr1, _priKey1, 1, big.NewInt(1), nil, 0, big.NewInt(0))
	require.NoError(err)
	require.Equal(mockError, errors.Cause(ap.Validate(ctx, tsf)))
}

func TestActPool_AddActs(t *testing.T) {
	ctrl := gomock.NewController(t)

	require := require.New(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := account.(*state.Account)
		require.True(ok)
		cfg := &protocol.StateConfig{}
		for _, opt := range opts {
			opt(cfg)
		}
		if bytes.Equal(cfg.Key, identityset.Address(28).Bytes()) {
			require.NoError(acct.AddBalance(big.NewInt(100)))
		} else {
			require.NoError(acct.AddBalance(big.NewInt(10)))
		}
		return 0, nil
	}).AnyTimes()
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(genesis.Default, sf, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	// Test actpool status after adding a sequence of Tsfs/votes: need to check confirmed nonce, pending nonce, and pending balance
	tsf1, err := action.SignedTransfer(_addr1, _priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(_addr1, _priKey1, uint64(2), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(_addr1, _priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf4, err := action.SignedTransfer(_addr1, _priKey1, uint64(4), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf5, err := action.SignedTransfer(_addr1, _priKey1, uint64(5), big.NewInt(50), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf6, err := action.SignedTransfer(_addr2, _priKey2, uint64(1), big.NewInt(5), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf7, err := action.SignedTransfer(_addr2, _priKey2, uint64(3), big.NewInt(1), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf8, err := action.SignedTransfer(_addr2, _priKey2, uint64(4), big.NewInt(5), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)

	ctx := genesis.WithGenesisContext(context.Background(), genesis.Default)
	require.NoError(ap.Add(ctx, tsf1))
	require.NoError(ap.Add(ctx, tsf2))
	require.NoError(ap.Add(ctx, tsf3))
	require.NoError(ap.Add(ctx, tsf4))
	require.Equal(action.ErrInsufficientFunds, errors.Cause(ap.Add(ctx, tsf5)))
	require.NoError(ap.Add(ctx, tsf6))
	require.NoError(ap.Add(ctx, tsf7))
	require.NoError(ap.Add(ctx, tsf8))

	pBalance1, _ := getPendingBalance(ap, _addr1)
	require.Equal(uint64(10), pBalance1.Uint64())
	pNonce1, _ := ap.GetPendingNonce(_addr1)
	require.Equal(uint64(5), pNonce1)

	pBalance2, _ := getPendingBalance(ap, _addr2)
	require.Equal(uint64(5), pBalance2.Uint64())
	pNonce2, _ := ap.GetPendingNonce(_addr2)
	require.Equal(uint64(2), pNonce2)

	tsf9, err := action.SignedTransfer(_addr2, _priKey2, uint64(2), big.NewInt(3), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	require.NoError(ap.Add(ctx, tsf9))
	pBalance2, _ = getPendingBalance(ap, _addr2)
	require.Equal(uint64(1), pBalance2.Uint64())
	pNonce2, _ = ap.GetPendingNonce(_addr2)
	require.Equal(uint64(4), pNonce2)
	// Error Case Handling
	// Case I: Action source address is blacklisted
	bannedTsf, err := action.SignedTransfer(_addr6, _priKey6, uint64(1), big.NewInt(0), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = ap.Add(ctx, bannedTsf)
	require.Contains(err.Error(), "action source address is blacklisted")
	// Case II: Action already exists in pool
	require.Error(ap.Add(ctx, tsf1))
	require.Error(ap.Add(ctx, tsf4))
	// Case III: Pool space/gas space is full
	Ap2, err := NewActPool(genesis.Default, sf, apConfig)
	require.NoError(err)
	ap2, ok := Ap2.(*actPool)
	require.True(ok)
	for i := uint64(0); i < ap2.cfg.MaxNumActsPerPool; i++ {
		nTsf, err := action.SignedTransfer(_addr2, _priKey2, i, big.NewInt(50), nil, uint64(0), big.NewInt(0))
		require.NoError(err)
		nTsfHash, err := nTsf.Hash()
		require.NoError(err)
		ap2.allActions.Set(nTsfHash, nTsf)
	}
	require.Equal(uint64(ap2.allActions.Count()), apConfig.MaxNumActsPerPool)
	// Tx Pool is full, but replacement happens
	require.Error(action.ErrTxPoolOverflow, ap2.Add(ctx, tsf1))
	require.Equal(uint64(ap2.allActions.Count()), apConfig.MaxNumActsPerPool)
	require.NoError(ap2.Add(ctx, tsf4))
	require.Equal(uint64(ap2.allActions.Count()), apConfig.MaxNumActsPerPool)

	Ap3, err := NewActPool(genesis.Default, sf, apConfig)
	require.NoError(err)
	ap3, ok := Ap3.(*actPool)
	require.True(ok)
	for i := uint64(1); i < apConfig.MaxGasLimitPerPool/10000; i++ {
		nTsf, err := action.SignedTransfer(_addr2, _priKey2, i, big.NewInt(50), nil, uint64(10000), big.NewInt(0))
		require.NoError(err)
		nTsfHash, err := nTsf.Hash()
		require.NoError(err)
		ap3.allActions.Set(nTsfHash, nTsf)
		intrinsicGas, err := nTsf.IntrinsicGas()
		require.NoError(err)
		ap3.gasInPool += intrinsicGas
	}
	tsf10, err := action.SignedTransfer(_addr2, _priKey2, uint64(apConfig.MaxGasLimitPerPool/10000), big.NewInt(50), []byte{1, 2, 3}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	require.Equal(uint64(ap2.allActions.Count()), apConfig.MaxNumActsPerPool)
	require.Error(action.ErrNonceTooHigh, ap3.Add(ctx, tsf10))
	require.Equal(uint64(ap2.allActions.Count()), apConfig.MaxNumActsPerPool)

	// Case IV: Nonce already exists
	replaceTsf, err := action.SignedTransfer(_addr2, _priKey1, uint64(1), big.NewInt(1), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = ap.Add(ctx, replaceTsf)
	require.Equal(action.ErrReplaceUnderpriced, errors.Cause(err))
	replaceTransfer, err := action.NewTransfer(uint64(4), big.NewInt(1), _addr2, []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)

	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(4).
		SetAction(replaceTransfer).
		SetGasLimit(100000).Build()
	selp, err := action.Sign(elp, _priKey1)

	require.NoError(err)

	err = ap.Add(ctx, selp)
	require.Equal(action.ErrReplaceUnderpriced, errors.Cause(err))
	// Case V: Nonce is too large
	outOfBoundsTsf, err := action.SignedTransfer(_addr1, _priKey1, ap.cfg.MaxNumActsPerAcct+1, big.NewInt(1), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = ap.Add(ctx, outOfBoundsTsf)
	require.Equal(action.ErrNonceTooHigh, errors.Cause(err))
	// Case VI: Insufficient balance
	overBalTsf, err := action.SignedTransfer(_addr2, _priKey2, uint64(4), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = ap.Add(ctx, overBalTsf)
	require.Equal(action.ErrInsufficientFunds, errors.Cause(err))
	// Case VII: insufficient gas
	tmpData := [1234]byte{}
	creationExecution, err := action.NewExecution(
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
	selp, err = action.Sign(elp, _priKey1)
	require.NoError(err)

	err = ap.Add(ctx, selp)
	require.Equal(action.ErrIntrinsicGas, errors.Cause(err))

	// test reject system action
	elp = bd.SetAction(&action.GrantReward{}).Build()
	err = ap.Add(ctx, action.FakeSeal(elp, _priKey1.PublicKey()))
	require.Equal(action.ErrInvalidAct, errors.Cause(err))
	elp = bd.SetAction(&action.PutPollResult{}).Build()
	err = ap.Add(ctx, action.FakeSeal(elp, _priKey1.PublicKey()))
	require.Equal(action.ErrInvalidAct, errors.Cause(err))
}

func TestActPool_PickActs(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	ctx := genesis.WithGenesisContext(context.Background(), genesis.Default)
	createActPool := func(cfg Config) (*actPool, []*action.SealedEnvelope, []*action.SealedEnvelope, []*action.SealedEnvelope) {
		// Create actpool
		Ap, err := NewActPool(genesis.Default, sf, cfg)
		require.NoError(err)
		ap, ok := Ap.(*actPool)
		require.True(ok)
		ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

		tsf1, err := action.SignedTransfer(_addr1, _priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf2, err := action.SignedTransfer(_addr1, _priKey1, uint64(2), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf3, err := action.SignedTransfer(_addr1, _priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf4, err := action.SignedTransfer(_addr1, _priKey1, uint64(4), big.NewInt(40), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf5, err := action.SignedTransfer(_addr1, _priKey1, uint64(5), big.NewInt(50), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf6, err := action.SignedTransfer(_addr1, _priKey1, uint64(7), big.NewInt(50), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf7, err := action.SignedTransfer(_addr2, _priKey2, uint64(1), big.NewInt(50), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf8, err := action.SignedTransfer(_addr2, _priKey2, uint64(3), big.NewInt(5), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf9, err := action.SignedTransfer(_addr2, _priKey2, uint64(4), big.NewInt(1), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf10, err := action.SignedTransfer(_addr2, _priKey2, uint64(5), big.NewInt(5), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)

		sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
			acct, ok := account.(*state.Account)
			require.True(ok)
			cfg := &protocol.StateConfig{}
			for _, opt := range opts {
				opt(cfg)
			}
			if bytes.Equal(cfg.Key, identityset.Address(28).Bytes()) {
				require.NoError(acct.AddBalance(big.NewInt(100)))
			} else {
				require.NoError(acct.AddBalance(big.NewInt(10)))
			}
			return 0, nil
		}).AnyTimes()
		sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
		require.NoError(ap.Add(ctx, tsf1))
		require.NoError(ap.Add(ctx, tsf2))
		require.NoError(ap.Add(ctx, tsf3))
		require.NoError(ap.Add(ctx, tsf4))
		require.Equal(action.ErrInsufficientFunds, errors.Cause(ap.Add(ctx, tsf5)))
		require.Error(ap.Add(ctx, tsf6))

		require.Error(ap.Add(ctx, tsf7))
		require.NoError(ap.Add(ctx, tsf8))
		require.NoError(ap.Add(ctx, tsf9))
		require.NoError(ap.Add(ctx, tsf10))

		return ap, []*action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4}, []*action.SealedEnvelope{}, []*action.SealedEnvelope{}
	}

	t.Run("no-expiry", func(t *testing.T) {
		apConfig := getActPoolCfg()
		ap, transfers, _, executions := createActPool(apConfig)
		pickedActs := ap.PendingActionMap()
		require.Equal(len(transfers)+len(executions), lenPendingActionMap(pickedActs))
		time.Sleep(2 * time.Second)
		pickedActs = ap.PendingActionMap()
		require.Equal(len(transfers)+len(executions), lenPendingActionMap(pickedActs))
	})
}

func TestActPool_removeConfirmedActs(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(genesis.Default, sf, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

	tsf1, err := action.SignedTransfer(_addr1, _priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(_addr1, _priKey1, uint64(2), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(_addr1, _priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf4, err := action.SignedTransfer(_addr1, _priKey1, uint64(4), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)

	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := account.(*state.Account)
		require.True(ok)
		require.NoError(acct.AddBalance(big.NewInt(100000000000000000)))

		return 0, nil
	}).Times(5)
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
	ctx := genesis.WithGenesisContext(context.Background(), genesis.Default)
	require.NoError(ap.Add(ctx, tsf1))
	require.NoError(ap.Add(ctx, tsf2))
	require.NoError(ap.Add(ctx, tsf3))
	require.NoError(ap.Add(ctx, tsf4))

	require.Equal(4, ap.allActions.Count())
	addr, err := address.FromString(_addr1)
	require.NoError(err)
	require.NotNil(ap.worker[ap.allocatedWorker(addr)].accountActs.Account(addr.String()))
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := account.(*state.Account)
		require.True(ok)
		for i := uint64(1); i <= 4; i++ {
			require.NoError(acct.SetPendingNonce(i + 1))
		}
		require.NoError(acct.AddBalance(big.NewInt(100000000000000000)))

		return 0, nil
	}).Times(1)
	ap.Reset()
	require.Equal(0, ap.allActions.Count())
	require.True(ap.worker[ap.allocatedWorker(addr)].accountActs.Account(addr.String()).Empty())
}

func TestActPool_Reset(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)

	balances := []*big.Int{
		big.NewInt(100),
		big.NewInt(200),
		big.NewInt(300),
		big.NewInt(10),
		big.NewInt(20),
	}
	nonces := []uint64{0, 0, 0, 0, 0}
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := account.(*state.Account)
		require.True(ok)
		cfg := &protocol.StateConfig{}
		for _, opt := range opts {
			opt(cfg)
		}
		switch {
		case bytes.Equal(cfg.Key, identityset.Address(28).Bytes()):
			require.NoError(acct.AddBalance(new(big.Int).Set(balances[0])))
			for i := uint64(1); i <= nonces[0]; i++ {
				require.NoError(acct.SetPendingNonce(i + 1))
			}
		case bytes.Equal(cfg.Key, identityset.Address(29).Bytes()):
			require.NoError(acct.AddBalance(new(big.Int).Set(balances[1])))
			for i := uint64(1); i <= nonces[1]; i++ {
				require.NoError(acct.SetPendingNonce(i + 1))
			}
		case bytes.Equal(cfg.Key, identityset.Address(30).Bytes()):
			require.NoError(acct.AddBalance(new(big.Int).Set(balances[2])))
			for i := uint64(1); i <= nonces[2]; i++ {
				require.NoError(acct.SetPendingNonce(i + 1))
			}
		case bytes.Equal(cfg.Key, identityset.Address(31).Bytes()):
			require.NoError(acct.AddBalance(new(big.Int).Set(balances[3])))
			for i := uint64(1); i <= nonces[3]; i++ {
				require.NoError(acct.SetPendingNonce(i + 1))
			}
		case bytes.Equal(cfg.Key, identityset.Address(32).Bytes()):
			require.NoError(acct.AddBalance(new(big.Int).Set(balances[4])))
			for i := uint64(1); i <= nonces[4]; i++ {
				require.NoError(acct.SetPendingNonce(i + 1))
			}
		}
		return 0, nil
	}).AnyTimes()
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()

	apConfig := getActPoolCfg()
	Ap1, err := NewActPool(genesis.Default, sf, apConfig)
	require.NoError(err)
	ap1, ok := Ap1.(*actPool)
	require.True(ok)
	ap1.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	Ap2, err := NewActPool(genesis.Default, sf, apConfig)
	require.NoError(err)
	ap2, ok := Ap2.(*actPool)
	require.True(ok)
	ap2.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

	// Tsfs to be added to ap1
	tsf1, err := action.SignedTransfer(_addr2, _priKey1, uint64(1), big.NewInt(50), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(_addr3, _priKey1, uint64(2), big.NewInt(30), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(_addr2, _priKey1, uint64(3), big.NewInt(60), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf4, err := action.SignedTransfer(_addr1, _priKey2, uint64(1), big.NewInt(100), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf5, err := action.SignedTransfer(_addr3, _priKey2, uint64(2), big.NewInt(50), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf6, err := action.SignedTransfer(_addr1, _priKey2, uint64(3), big.NewInt(60), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf7, err := action.SignedTransfer(_addr1, _priKey3, uint64(1), big.NewInt(100), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf8, err := action.SignedTransfer(_addr2, _priKey3, uint64(2), big.NewInt(100), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf9, err := action.SignedTransfer(_addr1, _priKey3, uint64(4), big.NewInt(100), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)

	ctx := genesis.WithGenesisContext(context.Background(), genesis.Default)
	require.NoError(ap1.Add(ctx, tsf1))
	require.NoError(ap1.Add(ctx, tsf2))
	err = ap1.Add(ctx, tsf3)
	require.Equal(action.ErrInsufficientFunds, errors.Cause(err))
	require.NoError(ap1.Add(ctx, tsf4))
	require.NoError(ap1.Add(ctx, tsf5))
	err = ap1.Add(ctx, tsf6)
	require.Equal(action.ErrInsufficientFunds, errors.Cause(err))
	require.NoError(ap1.Add(ctx, tsf7))
	require.NoError(ap1.Add(ctx, tsf8))
	require.NoError(ap1.Add(ctx, tsf9))
	// Tsfs to be added to ap2 only
	tsf10, err := action.SignedTransfer(_addr2, _priKey1, uint64(3), big.NewInt(20), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf11, err := action.SignedTransfer(_addr3, _priKey1, uint64(4), big.NewInt(10), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf12, err := action.SignedTransfer(_addr3, _priKey2, uint64(2), big.NewInt(70), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf13, err := action.SignedTransfer(_addr1, _priKey3, uint64(1), big.NewInt(200), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf14, err := action.SignedTransfer(_addr2, _priKey3, uint64(2), big.NewInt(50), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)

	require.NoError(ap2.Add(ctx, tsf1))
	require.NoError(ap2.Add(ctx, tsf2))
	require.NoError(ap2.Add(ctx, tsf10))
	err = ap2.Add(ctx, tsf11)
	require.Equal(action.ErrInsufficientFunds, errors.Cause(err))
	require.NoError(ap2.Add(ctx, tsf4))
	require.NoError(ap2.Add(ctx, tsf12))
	require.NoError(ap2.Add(ctx, tsf13))
	require.NoError(ap2.Add(ctx, tsf14))
	err = ap2.Add(ctx, tsf9)
	require.Equal(action.ErrInsufficientFunds, errors.Cause(err))
	// Check confirmed nonce, pending nonce, and pending balance after adding Tsfs above for each account
	// ap1
	// Addr1
	ap1PNonce1, _ := ap1.GetPendingNonce(_addr1)
	require.Equal(uint64(3), ap1PNonce1)
	ap1PBalance1, _ := getPendingBalance(ap1, _addr1)
	require.Equal(big.NewInt(20).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1PNonce2, _ := ap1.GetPendingNonce(_addr2)
	require.Equal(uint64(3), ap1PNonce2)
	ap1PBalance2, _ := getPendingBalance(ap1, _addr2)
	require.Equal(big.NewInt(50).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1PNonce3, _ := ap1.GetPendingNonce(_addr3)
	require.Equal(uint64(3), ap1PNonce3)
	ap1PBalance3, _ := getPendingBalance(ap1, _addr3)
	require.Equal(big.NewInt(100).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2PNonce1, _ := ap2.GetPendingNonce(_addr1)
	require.Equal(uint64(4), ap2PNonce1)
	ap2PBalance1, _ := getPendingBalance(ap2, _addr1)
	require.Equal(big.NewInt(0).Uint64(), ap2PBalance1.Uint64())
	// Addr2
	ap2PNonce2, _ := ap2.GetPendingNonce(_addr2)
	require.Equal(uint64(3), ap2PNonce2)
	ap2PBalance2, _ := getPendingBalance(ap2, _addr2)
	require.Equal(big.NewInt(30).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2PNonce3, _ := ap2.GetPendingNonce(_addr3)
	require.Equal(uint64(3), ap2PNonce3)
	ap2PBalance3, _ := getPendingBalance(ap2, _addr3)
	require.Equal(big.NewInt(50).Uint64(), ap2PBalance3.Uint64())
	// Let ap1 be BP's actpool
	balances[0] = big.NewInt(220)
	nonces[0] = 2
	balances[1] = big.NewInt(200)
	nonces[1] = 2
	balances[2] = big.NewInt(180)
	nonces[2] = 2
	//Reset
	ap1.Reset()
	ap2.Reset()
	// Check confirmed nonce, pending nonce, and pending balance after resetting actpool for each account
	// ap1
	// Addr1
	ap1PNonce1, _ = ap1.GetPendingNonce(_addr1)
	require.Equal(uint64(3), ap1PNonce1)
	ap1PBalance1, _ = getPendingBalance(ap1, _addr1)
	require.Equal(big.NewInt(220).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1PNonce2, _ = ap1.GetPendingNonce(_addr2)
	require.Equal(uint64(3), ap1PNonce2)
	ap1PBalance2, _ = getPendingBalance(ap1, _addr2)
	require.Equal(big.NewInt(200).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1PNonce3, _ = ap1.GetPendingNonce(_addr3)
	require.Equal(uint64(3), ap1PNonce3)
	ap1PBalance3, _ = getPendingBalance(ap1, _addr3)
	require.Equal(big.NewInt(180).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2PNonce1, _ = ap2.GetPendingNonce(_addr1)
	require.Equal(uint64(4), ap2PNonce1)
	ap2PBalance1, _ = getPendingBalance(ap2, _addr1)
	require.Equal(big.NewInt(200).Uint64(), ap2PBalance1.Uint64())
	// Addr2
	ap2PNonce2, _ = ap2.GetPendingNonce(_addr2)
	require.Equal(uint64(3), ap2PNonce2)
	ap2PBalance2, _ = getPendingBalance(ap2, _addr2)
	require.Equal(big.NewInt(200).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2PNonce3, _ = ap2.GetPendingNonce(_addr3)
	require.Equal(uint64(3), ap2PNonce3)
	ap2PBalance3, _ = getPendingBalance(ap2, _addr3)
	require.Equal(big.NewInt(180).Uint64(), ap2PBalance3.Uint64())
	// Add more Tsfs after resetting
	// Tsfs To be added to ap1 only
	tsf15, err := action.SignedTransfer(_addr2, _priKey3, uint64(3), big.NewInt(80), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	// Tsfs To be added to ap2 only
	tsf16, err := action.SignedTransfer(_addr2, _priKey1, uint64(4), big.NewInt(150), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf17, err := action.SignedTransfer(_addr1, _priKey2, uint64(3), big.NewInt(90), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf18, err := action.SignedTransfer(_addr3, _priKey2, uint64(4), big.NewInt(100), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf19, err := action.SignedTransfer(_addr1, _priKey2, uint64(5), big.NewInt(50), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf20, err := action.SignedTransfer(_addr2, _priKey3, uint64(3), big.NewInt(200), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)

	require.NoError(ap1.Add(ctx, tsf15))
	require.NoError(ap2.Add(ctx, tsf16))
	require.NoError(ap2.Add(ctx, tsf17))
	require.NoError(ap2.Add(ctx, tsf18))
	err = ap2.Add(ctx, tsf19)
	require.Equal(action.ErrInsufficientFunds, errors.Cause(err))
	err = ap2.Add(ctx, tsf20)
	require.Equal(action.ErrInsufficientFunds, errors.Cause(err))
	// Check confirmed nonce, pending nonce, and pending balance after adding Tsfs above for each account
	// ap1
	// Addr1
	ap1PNonce1, _ = ap1.GetPendingNonce(_addr1)
	require.Equal(uint64(3), ap1PNonce1)
	ap1PBalance1, _ = getPendingBalance(ap1, _addr1)
	require.Equal(big.NewInt(220).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1PNonce2, _ = ap1.GetPendingNonce(_addr2)
	require.Equal(uint64(3), ap1PNonce2)
	ap1PBalance2, _ = getPendingBalance(ap1, _addr2)
	require.Equal(big.NewInt(200).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1PNonce3, _ = ap1.GetPendingNonce(_addr3)
	require.Equal(uint64(5), ap1PNonce3)
	ap1PBalance3, _ = getPendingBalance(ap1, _addr3)
	require.Equal(big.NewInt(0).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2PNonce1, _ = ap2.GetPendingNonce(_addr1)
	require.Equal(uint64(5), ap2PNonce1)
	ap2PBalance2, _ = getPendingBalance(ap2, _addr2)
	require.Equal(big.NewInt(10).Uint64(), ap2PBalance2.Uint64())
	// Addr2
	ap2PNonce2, _ = ap2.GetPendingNonce(_addr2)
	require.Equal(uint64(5), ap2PNonce2)
	ap2PBalance2, _ = getPendingBalance(ap2, _addr2)
	require.Equal(big.NewInt(10).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2PNonce3, _ = ap2.GetPendingNonce(_addr3)
	require.Equal(uint64(3), ap2PNonce3)
	ap2PBalance3, _ = getPendingBalance(ap2, _addr3)
	require.Equal(big.NewInt(180).Uint64(), ap2PBalance3.Uint64())
	// Let ap2 be BP's actpool
	balances[0] = big.NewInt(140)
	nonces[0] = 4
	balances[1] = big.NewInt(180)
	nonces[1] = 4
	balances[2] = big.NewInt(280)
	nonces[2] = 2

	//Reset
	ap1.Reset()
	ap2.Reset()
	// Check confirmed nonce, pending nonce, and pending balance after resetting actpool for each account
	// ap1
	// Addr1
	ap1PNonce1, _ = ap1.GetPendingNonce(_addr1)
	require.Equal(uint64(5), ap1PNonce1)
	ap1PBalance1, _ = getPendingBalance(ap1, _addr1)
	require.Equal(big.NewInt(140).Uint64(), ap1PBalance1.Uint64())
	// Addr2
	ap1PNonce2, _ = ap1.GetPendingNonce(_addr2)
	require.Equal(uint64(5), ap1PNonce2)
	ap1PBalance2, _ = getPendingBalance(ap1, _addr2)
	require.Equal(big.NewInt(180).Uint64(), ap1PBalance2.Uint64())
	// Addr3
	ap1PNonce3, _ = ap1.GetPendingNonce(_addr3)
	require.Equal(uint64(5), ap1PNonce3)
	ap1PBalance3, _ = getPendingBalance(ap1, _addr3)
	require.Equal(big.NewInt(100).Uint64(), ap1PBalance3.Uint64())
	// ap2
	// Addr1
	ap2PNonce1, _ = ap2.GetPendingNonce(_addr1)
	require.Equal(uint64(5), ap2PNonce1)
	ap2PBalance1, _ = getPendingBalance(ap2, _addr1)
	require.Equal(big.NewInt(140).Uint64(), ap2PBalance1.Uint64())
	// Addr2
	ap2PNonce2, _ = ap2.GetPendingNonce(_addr2)
	require.Equal(uint64(5), ap2PNonce2)
	ap2PBalance2, _ = getPendingBalance(ap2, _addr2)
	require.Equal(big.NewInt(180).Uint64(), ap2PBalance2.Uint64())
	// Addr3
	ap2PNonce3, _ = ap2.GetPendingNonce(_addr3)
	require.Equal(uint64(3), ap2PNonce3)
	ap2PBalance3, _ = getPendingBalance(ap2, _addr3)
	require.Equal(big.NewInt(280).Uint64(), ap2PBalance3.Uint64())

	// Add two more players
	tsf21, err := action.SignedTransfer(_addr5, _priKey4, uint64(1), big.NewInt(10), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf22, err := action.SignedTransfer(_addr5, _priKey4, uint64(2), big.NewInt(10), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf23, err := action.NewTransfer(uint64(3), big.NewInt(1), "", []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)

	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(3).
		SetGasLimit(20000).
		SetAction(tsf23).Build()
	selp23, err := action.Sign(elp, _priKey4)
	require.NoError(err)

	tsf24, err := action.SignedTransfer(_addr5, _priKey5, uint64(1), big.NewInt(10), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf25, err := action.SignedTransfer(_addr4, _priKey5, uint64(2), big.NewInt(10), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf26, err := action.NewTransfer(uint64(3), big.NewInt(1), _addr4, []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(3).
		SetGasLimit(20000).
		SetAction(tsf26).Build()
	selp26, err := action.Sign(elp, _priKey5)
	require.NoError(err)

	require.NoError(ap1.Add(ctx, tsf21))
	require.Error(ap1.Add(ctx, tsf22))
	require.Error(ap1.Add(ctx, selp23))
	require.NoError(ap1.Add(ctx, tsf24))
	require.NoError(ap1.Add(ctx, tsf25))
	require.Error(ap1.Add(ctx, selp26))
	// Check confirmed nonce, pending nonce, and pending balance after adding actions above for account4 and account5
	// ap1
	// Addr4
	ap1PNonce4, _ := ap1.GetPendingNonce(_addr4)
	require.Equal(uint64(2), ap1PNonce4)
	ap1PBalance4, _ := getPendingBalance(ap1, _addr4)
	require.Equal(big.NewInt(0).Uint64(), ap1PBalance4.Uint64())
	// Addr5
	ap1PNonce5, _ := ap1.GetPendingNonce(_addr5)
	require.Equal(uint64(3), ap1PNonce5)
	ap1PBalance5, _ := getPendingBalance(ap1, _addr5)
	require.Equal(big.NewInt(0).Uint64(), ap1PBalance5.Uint64())
	// Let ap1 be BP's actpool
	balances[3] = big.NewInt(10)
	nonces[3] = 1
	balances[4] = big.NewInt(20)
	nonces[4] = 2
	//Reset
	ap1.Reset()
	// Check confirmed nonce, pending nonce, and pending balance after resetting actpool for each account
	// ap1
	// Addr4
	ap1PNonce4, _ = ap1.GetPendingNonce(_addr4)
	require.Equal(uint64(2), ap1PNonce4)
	ap1PBalance4, _ = getPendingBalance(ap1, _addr4)
	require.Equal(big.NewInt(10).Uint64(), ap1PBalance4.Uint64())
	// Addr5
	ap1PNonce5, _ = ap1.GetPendingNonce(_addr5)
	require.Equal(uint64(3), ap1PNonce5)
	ap1PBalance5, _ = getPendingBalance(ap1, _addr5)
	require.Equal(big.NewInt(20).Uint64(), ap1PBalance5.Uint64())
}

func TestActPool_removeInvalidActs(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(genesis.Default, sf, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

	tsf1, err := action.SignedTransfer(_addr1, _priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(_addr1, _priKey1, uint64(2), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(_addr1, _priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf4, err := action.SignedTransfer(_addr1, _priKey1, uint64(4), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := account.(*state.Account)
		require.True(ok)
		require.NoError(acct.AddBalance(big.NewInt(100000000000000000)))

		return 0, nil
	}).Times(5)
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
	ctx := genesis.WithGenesisContext(context.Background(), genesis.Default)
	require.NoError(ap.Add(ctx, tsf1))
	require.NoError(ap.Add(ctx, tsf2))
	require.NoError(ap.Add(ctx, tsf3))
	require.NoError(ap.Add(ctx, tsf4))

	hash1, err := tsf1.Hash()
	require.NoError(err)
	hash2, err := tsf4.Hash()
	require.NoError(err)
	acts := []*action.SealedEnvelope{tsf1, tsf4}
	_, exist1 := ap.allActions.Get(hash1)
	require.True(exist1)
	_, exist2 := ap.allActions.Get(hash2)
	require.True(exist2)
	ap.removeInvalidActs(acts)
	_, exist1 = ap.allActions.Get(hash1)
	require.False(exist1)
	_, exist2 = ap.allActions.Get(hash2)
	require.False(exist2)
}

func TestActPool_GetPendingNonce(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(genesis.Default, sf, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

	tsf1, err := action.SignedTransfer(_addr1, _priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(_addr1, _priKey1, uint64(2), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(_addr1, _priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf4, err := action.SignedTransfer(_addr1, _priKey1, uint64(4), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := account.(*state.Account)
		require.True(ok)
		require.NoError(acct.AddBalance(big.NewInt(100000000000000000)))

		return 0, nil
	}).Times(6)
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()

	ctx := genesis.WithGenesisContext(context.Background(), genesis.Default)
	require.NoError(ap.Add(ctx, tsf1))
	require.NoError(ap.Add(ctx, tsf3))
	require.NoError(ap.Add(ctx, tsf4))

	nonce, err := ap.GetPendingNonce(_addr2)
	require.NoError(err)
	require.Equal(uint64(1), nonce)

	nonce, err = ap.GetPendingNonce(_addr1)
	require.NoError(err)
	require.Equal(uint64(2), nonce)

	require.NoError(ap.Add(ctx, tsf2))
	nonce, err = ap.GetPendingNonce(_addr1)
	require.NoError(err)
	require.Equal(uint64(5), nonce)
}

func TestActPool_GetUnconfirmedActs(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(genesis.Default, sf, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

	tsf1, err := action.SignedTransfer(_addr1, _priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(_addr1, _priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf4, err := action.SignedTransfer(_addr1, _priKey1, uint64(4), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf5, err := action.SignedTransfer(_addr1, _priKey2, uint64(1), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)

	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := account.(*state.Account)
		require.True(ok)
		require.NoError(acct.AddBalance(big.NewInt(100000000000000000)))

		return 0, nil
	}).Times(6)
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
	ctx := genesis.WithGenesisContext(context.Background(), genesis.Default)
	require.NoError(ap.Add(ctx, tsf1))
	require.NoError(ap.Add(ctx, tsf3))
	require.NoError(ap.Add(ctx, tsf4))
	require.NoError(ap.Add(ctx, tsf5))

	acts := ap.GetUnconfirmedActs(_addr3)
	require.Equal([]*action.SealedEnvelope(nil), acts)

	acts = ap.GetUnconfirmedActs(_addr1)
	validated := []*action.SealedEnvelope{tsf1, tsf3, tsf4, tsf5}
	require.Equal(len(acts), len(validated))
	for i := 0; i < len(acts); i++ {
		hashVal1, hashErr1 := validated[i].Hash()
		require.NoError(hashErr1)
		hashVal2, hashErr2 := acts[i].Hash()
		require.NoError(hashErr2)
		require.Equal(hashVal1, hashVal2)
	}
}

func TestActPool_GetActionByHash(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)

	sf := mock_chainmanager.NewMockStateReader(ctrl)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(genesis.Default, sf, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)

	tsf1, err := action.SignedTransfer(_addr1, _priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	hash1, err := tsf1.Hash()
	require.NoError(err)
	tsf2, err := action.SignedTransfer(_addr1, _priKey1, uint64(2), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	hash2, err := tsf2.Hash()
	require.NoError(err)

	ap.allActions.Set(hash1, tsf1)
	act, err := ap.GetActionByHash(hash1)
	require.NoError(err)
	require.Equal(tsf1, act)
	act, err = ap.GetActionByHash(hash2)
	require.Equal(action.ErrNotFound, errors.Cause(err))
	require.Nil(act)

	ap.allActions.Set(hash2, tsf2)
	act, err = ap.GetActionByHash(hash2)
	require.NoError(err)
	require.Equal(tsf2, act)
}

func TestActPool_GetCapacity(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(genesis.Default, sf, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	require.Equal(uint64(_maxNumActsPerPool), ap.GetCapacity())
	require.Equal(uint64(_maxGasLimitPerPool), ap.GetGasCapacity())
}

func TestActPool_GetSize(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(genesis.Default, sf, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	require.Zero(ap.GetSize())
	require.Zero(ap.GetGasSize())

	tsf1, err := action.SignedTransfer(_addr1, _priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(_addr1, _priKey1, uint64(2), big.NewInt(20), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(_addr1, _priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf4, err := action.SignedTransfer(_addr1, _priKey1, uint64(4), big.NewInt(30), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)

	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := account.(*state.Account)
		require.True(ok)
		require.NoError(acct.AddBalance(big.NewInt(100000000000000000)))

		return 0, nil
	}).Times(5)
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
	ctx := genesis.WithGenesisContext(context.Background(), genesis.Default)
	require.NoError(ap.Add(ctx, tsf1))
	require.NoError(ap.Add(ctx, tsf2))
	require.NoError(ap.Add(ctx, tsf3))
	require.NoError(ap.Add(ctx, tsf4))
	require.Equal(uint64(4), ap.GetSize())
	require.Equal(uint64(40000), ap.GetGasSize())
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := account.(*state.Account)
		require.True(ok)
		for i := uint64(1); i <= 4; i++ {
			require.NoError(acct.SetPendingNonce(i + 1))
		}
		require.NoError(acct.AddBalance(big.NewInt(100000000000000000)))

		return 0, nil
	}).Times(1)
	ap.Reset()
	require.Equal(uint64(0), ap.GetSize())
	require.Equal(uint64(0), ap.GetGasSize())
}

func TestActPool_AddActionNotEnoughGasPrice(t *testing.T) {
	ctrl := gomock.NewController(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	sf.EXPECT().Height().Return(uint64(0), nil).Times(2)

	apConfig := DefaultConfig
	ap, err := NewActPool(genesis.Default, sf, apConfig)
	require.NoError(t, err)
	tsf, err := action.SignedTransfer(
		identityset.Address(0).String(),
		identityset.PrivateKey(1),
		uint64(1),
		big.NewInt(10),
		[]byte{},
		uint64(20000),
		big.NewInt(0),
	)
	require.NoError(t, err)

	ctx := protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{})
	ctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(
		genesis.WithGenesisContext(ctx, genesis.Default), protocol.BlockCtx{
			BlockHeight: 1,
		}))
	require.Error(t, ap.Add(ctx, tsf))
}

func TestActPool_SpeedUpAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := account.(*state.Account)
		require.True(ok)
		cfg := &protocol.StateConfig{}
		for _, opt := range opts {
			opt(cfg)
		}
		require.NoError(acct.AddBalance(big.NewInt(10000000)))
		return 0, nil
	}).AnyTimes()
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()

	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(genesis.Default, sf, apConfig)
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

	tsf1, err := action.SignedTransfer(_addr1, _priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(_addr2, _priKey2, uint64(1), big.NewInt(5), []byte{}, uint64(100000), big.NewInt(1))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(_addr1, _priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(3))
	require.NoError(err)

	// A send action tsf1 with nonce 1, B send action tsf2 with nonce 1
	ctx := genesis.WithGenesisContext(context.Background(), genesis.Default)
	require.NoError(ap.Add(ctx, tsf1))
	require.NoError(ap.Add(ctx, tsf2))

	// check account and actpool status
	pBalance1, _ := getPendingBalance(ap, _addr1)
	require.Equal(uint64(10000000-10), pBalance1.Uint64())
	pNonce1, _ := ap.GetPendingNonce(_addr1)
	require.Equal(uint64(2), pNonce1)
	pBalance2, _ := getPendingBalance(ap, _addr2)
	require.Equal(uint64(10000000-5-10000), pBalance2.Uint64())
	pNonce2, _ := ap.GetPendingNonce(_addr2)
	require.Equal(uint64(2), pNonce2)

	// A send action tsf3 with nonce 1 and higher gas price
	require.NoError(ap.Add(ctx, tsf3))

	// check account and actpool status again after new action is inserted
	pNonce3, _ := ap.GetPendingNonce(_addr1)
	require.Equal(uint64(2), pNonce3)

	ai := actioniterator.NewActionIterator(ap.PendingActionMap())
	appliedActionList := make([]*action.SealedEnvelope, 0)
	for {
		bestAction, ok := ai.Next()
		if !ok {
			break
		}
		appliedActionList = append(appliedActionList, bestAction)
	}
	// tsf1 is replaced by tsf3 with higher gas price
	validated := []*action.SealedEnvelope{tsf3, tsf2}
	require.Equal(len(appliedActionList), len(validated))
	for i := 0; i < len(appliedActionList); i++ {
		hashVal1, hashErr1 := validated[i].Hash()
		require.NoError(hashErr1)
		hashVal2, hashErr2 := appliedActionList[i].Hash()
		require.NoError(hashErr2)
		require.Equal(hashVal1, hashVal2)
	}
}

// Helper function to return the correct pending balance just in case of empty queue
func getPendingBalance(ap *actPool, addrStr string) (*big.Int, error) {
	addr, err := address.FromString(addrStr)
	if err != nil {
		return nil, err
	}
	if queue := ap.worker[ap.allocatedWorker(addr)].accountActs.Account(addr.String()); queue != nil {
		q := queue.(*actQueue)
		return q.getPendingBalanceAtNonce(q.pendingNonce), nil
	}
	state, err := accountutil.AccountState(genesis.WithGenesisContext(context.Background(), genesis.Default), ap.sf, addr)
	if err != nil {
		return nil, err
	}
	return state.Balance, nil
}

func getActPoolCfg() Config {
	return Config{
		MaxNumActsPerPool:  _maxNumActsPerPool,
		MaxGasLimitPerPool: _maxGasLimitPerPool,
		MaxNumActsPerAcct:  _maxNumActsPerAcct,
		MinGasPriceStr:     "0",
		BlackList:          []string{_addr6},
	}
}

func lenPendingActionMap(acts map[string][]*action.SealedEnvelope) int {
	l := 0
	for _, part := range acts {
		l += len(part)
	}
	return l
}

func TestValidateMinGasPrice(t *testing.T) {
	ap := Config{MinGasPriceStr: DefaultConfig.MinGasPriceStr}
	mgp := ap.MinGasPrice()
	require.IsType(t, &big.Int{}, mgp)
}
