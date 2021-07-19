// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"bytes"
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/test/mock/mock_sealed_envelope_validator"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	maxNumActsPerPool  = 8192
	maxGasLimitPerPool = 81920000
	maxNumActsPerAcct  = 256
)

var (
	addr1   = identityset.Address(28).String()
	pubKey1 = identityset.PrivateKey(28).PublicKey()
	priKey1 = identityset.PrivateKey(28)
	addr2   = identityset.Address(29).String()
	priKey2 = identityset.PrivateKey(29)
	addr3   = identityset.Address(30).String()
	priKey3 = identityset.PrivateKey(30)
	addr4   = identityset.Address(31).String()
	priKey4 = identityset.PrivateKey(31)
	addr5   = identityset.Address(32).String()
	priKey5 = identityset.PrivateKey(32)
	addr6   = identityset.Address(33).String()
	priKey6 = identityset.PrivateKey(33)
)

func TestActPool_NewActPool(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)
	cfg := config.Default

	//error caused by nil blockchain
	_, err := NewActPool(nil, cfg.ActPool, nil)
	require.Error(err)

	// all good
	opt := EnableExperimentalActions()
	require.Panics(func() { blockchain.NewBlockchain(cfg, nil, nil, nil) }, "option is nil")
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	act, err := NewActPool(sf, cfg.ActPool, opt)
	require.NoError(err)
	require.NotNil(act)

	// panic caused by option is nil
	require.Panics(func() { NewActPool(sf, cfg.ActPool, nil) }, "option is nil")

	// error caused by option
	opt2 := func(pool *actPool) error {
		return errors.New("test error")
	}
	_, err = NewActPool(sf, cfg.ActPool, opt2)
	require.Error(err)

	// test AddAction nil
	require.NotPanics(func() { act.AddActionEnvelopeValidators(nil) }, "option is nil")
}

func TestValidate(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	cfg := config.Default
	cfg.Genesis.InitBalanceMap[addr1] = "100"
	re := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(re))
	ctx := genesis.WithGenesisContext(
		protocol.WithRegistry(context.Background(), re),
		cfg.Genesis,
	)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	sev := mock_sealed_envelope_validator.NewMockSealedEnvelopeValidator(ctrl)
	mockError := errors.New("mock error")
	sev.EXPECT().Validate(gomock.Any(), gomock.Any()).Return(mockError).Times(1)
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(sf, apConfig, EnableExperimentalActions())
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(sev)
	// Case 0: Blacklist
	tsfFromBL, err := action.SignedTransfer(addr6, priKey6, 1, big.NewInt(1), nil, 0, big.NewInt(0))
	require.NoError(err)
	require.Equal(action.ErrAddress, errors.Cause(ap.Validate(ctx, tsfFromBL)))
	// Case I: failed by sealed envelope validator
	tsf, err := action.SignedTransfer(addr1, priKey1, 1, big.NewInt(1), nil, 0, big.NewInt(0))
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
		acct.Nonce = 0
		cfg := &protocol.StateConfig{}
		for _, opt := range opts {
			opt(cfg)
		}
		if bytes.Equal(cfg.Key, identityset.Address(28).Bytes()) {
			acct.Balance = big.NewInt(100)
		} else {
			acct.Balance = big.NewInt(10)
		}
		return 0, nil
	}).AnyTimes()
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(sf, apConfig, EnableExperimentalActions())
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	// Test actpool status after adding a sequence of Tsfs/votes: need to check confirmed nonce, pending nonce, and pending balance
	tsf1, err := action.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(addr1, priKey1, uint64(2), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(addr1, priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf4, err := action.SignedTransfer(addr1, priKey1, uint64(4), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf5, err := action.SignedTransfer(addr1, priKey1, uint64(5), big.NewInt(50), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf6, err := action.SignedTransfer(addr2, priKey2, uint64(1), big.NewInt(5), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf7, err := action.SignedTransfer(addr2, priKey2, uint64(3), big.NewInt(1), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf8, err := action.SignedTransfer(addr2, priKey2, uint64(4), big.NewInt(5), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)

	ctx := context.Background()
	require.NoError(ap.Add(ctx, tsf1))
	require.NoError(ap.Add(ctx, tsf2))
	require.NoError(ap.Add(ctx, tsf3))
	require.NoError(ap.Add(ctx, tsf4))
	require.Equal(action.ErrBalance, errors.Cause(ap.Add(ctx, tsf5)))
	require.NoError(ap.Add(ctx, tsf6))
	require.NoError(ap.Add(ctx, tsf7))
	require.NoError(ap.Add(ctx, tsf8))

	pBalance1, _ := ap.getPendingBalance(addr1)
	require.Equal(uint64(10), pBalance1.Uint64())
	pNonce1, _ := ap.getPendingNonce(addr1)
	require.Equal(uint64(5), pNonce1)

	pBalance2, _ := ap.getPendingBalance(addr2)
	require.Equal(uint64(5), pBalance2.Uint64())
	pNonce2, _ := ap.getPendingNonce(addr2)
	require.Equal(uint64(2), pNonce2)

	tsf9, err := action.SignedTransfer(addr2, priKey2, uint64(2), big.NewInt(3), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	require.NoError(ap.Add(ctx, tsf9))
	pBalance2, _ = ap.getPendingBalance(addr2)
	require.Equal(uint64(1), pBalance2.Uint64())
	pNonce2, _ = ap.getPendingNonce(addr2)
	require.Equal(uint64(4), pNonce2)
	// Error Case Handling
	// Case I: Action source address is blacklisted
	bannedTsf, err := action.SignedTransfer(addr6, priKey6, uint64(1), big.NewInt(0), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = ap.Add(ctx, bannedTsf)
	require.True(strings.Contains(err.Error(), "action source address is blacklisted"))
	// Case II: Action already exists in pool
	require.Error(ap.Add(ctx, tsf1))
	require.Error(ap.Add(ctx, tsf4))
	// Case III: Pool space/gas space is full
	Ap2, err := NewActPool(sf, apConfig, EnableExperimentalActions())
	require.NoError(err)
	ap2, ok := Ap2.(*actPool)
	require.True(ok)
	for i := uint64(0); i < ap2.cfg.MaxNumActsPerPool; i++ {
		nTsf, err := action.SignedTransfer(addr2, priKey2, i, big.NewInt(50), nil, uint64(0), big.NewInt(0))
		require.NoError(err)

		ap2.allActions[nTsf.Hash()] = nTsf
	}
	err = ap2.Add(ctx, tsf1)
	require.Equal(action.ErrActPool, errors.Cause(err))
	err = ap2.Add(ctx, tsf4)
	require.Equal(action.ErrActPool, errors.Cause(err))

	Ap3, err := NewActPool(sf, apConfig)
	require.NoError(err)
	ap3, ok := Ap3.(*actPool)
	require.True(ok)
	for i := uint64(1); i < apConfig.MaxGasLimitPerPool/10000; i++ {
		nTsf, err := action.SignedTransfer(addr2, priKey2, i, big.NewInt(50), nil, uint64(10000), big.NewInt(0))
		require.NoError(err)
		ap3.allActions[nTsf.Hash()] = nTsf
		intrinsicGas, err := nTsf.IntrinsicGas()
		require.NoError(err)
		ap3.gasInPool += intrinsicGas
	}
	tsf10, err := action.SignedTransfer(addr2, priKey2, uint64(apConfig.MaxGasLimitPerPool/10000), big.NewInt(50), []byte{1, 2, 3}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	err = ap3.Add(ctx, tsf10)
	require.True(strings.Contains(err.Error(), "insufficient gas space for action"))

	// Case IV: Nonce already exists
	replaceTsf, err := action.SignedTransfer(addr2, priKey1, uint64(1), big.NewInt(1), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = ap.Add(ctx, replaceTsf)
	require.Equal(action.ErrNonce, errors.Cause(err))
	replaceTransfer, err := action.NewTransfer(uint64(4), big.NewInt(1), addr2, []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)

	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(4).
		SetAction(replaceTransfer).
		SetGasLimit(100000).Build()
	selp, err := action.Sign(elp, priKey1)

	require.NoError(err)

	err = ap.Add(ctx, selp)
	require.Equal(action.ErrNonce, errors.Cause(err))
	// Case V: Nonce is too large
	outOfBoundsTsf, err := action.SignedTransfer(addr1, priKey1, ap.cfg.MaxNumActsPerAcct+1, big.NewInt(1), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = ap.Add(ctx, outOfBoundsTsf)
	require.Equal(action.ErrNonce, errors.Cause(err))
	// Case VI: Insufficient balance
	overBalTsf, err := action.SignedTransfer(addr2, priKey2, uint64(4), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = ap.Add(ctx, overBalTsf)
	require.Equal(action.ErrBalance, errors.Cause(err))
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
	selp, err = action.Sign(elp, priKey1)
	require.NoError(err)

	err = ap.Add(ctx, selp)
	require.Equal(action.ErrInsufficientBalanceForGas, errors.Cause(err))
}

func TestActPool_PickActs(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	createActPool := func(cfg config.ActPool) (*actPool, []action.SealedEnvelope, []action.SealedEnvelope, []action.SealedEnvelope) {
		// Create actpool
		Ap, err := NewActPool(sf, cfg, EnableExperimentalActions())
		require.NoError(err)
		ap, ok := Ap.(*actPool)
		require.True(ok)
		ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

		tsf1, err := action.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf2, err := action.SignedTransfer(addr1, priKey1, uint64(2), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf3, err := action.SignedTransfer(addr1, priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf4, err := action.SignedTransfer(addr1, priKey1, uint64(4), big.NewInt(40), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf5, err := action.SignedTransfer(addr1, priKey1, uint64(5), big.NewInt(50), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf6, err := action.SignedTransfer(addr1, priKey1, uint64(6), big.NewInt(50), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf7, err := action.SignedTransfer(addr2, priKey2, uint64(1), big.NewInt(50), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf8, err := action.SignedTransfer(addr2, priKey2, uint64(3), big.NewInt(5), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf9, err := action.SignedTransfer(addr2, priKey2, uint64(4), big.NewInt(1), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsf10, err := action.SignedTransfer(addr2, priKey2, uint64(5), big.NewInt(5), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)

		sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
			acct, ok := account.(*state.Account)
			require.True(ok)
			acct.Nonce = 0
			cfg := &protocol.StateConfig{}
			for _, opt := range opts {
				opt(cfg)
			}
			if bytes.Equal(cfg.Key, identityset.Address(28).Bytes()) {
				acct.Balance = big.NewInt(100)
			} else {
				acct.Balance = big.NewInt(10)
			}
			return 0, nil
		}).AnyTimes()
		require.NoError(ap.Add(context.Background(), tsf1))
		require.NoError(ap.Add(context.Background(), tsf2))
		require.NoError(ap.Add(context.Background(), tsf3))
		require.NoError(ap.Add(context.Background(), tsf4))
		require.Equal(action.ErrBalance, errors.Cause(ap.Add(context.Background(), tsf5)))
		require.Error(ap.Add(context.Background(), tsf6))

		require.Error(ap.Add(context.Background(), tsf7))
		require.NoError(ap.Add(context.Background(), tsf8))
		require.NoError(ap.Add(context.Background(), tsf9))
		require.NoError(ap.Add(context.Background(), tsf10))

		return ap, []action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4}, []action.SealedEnvelope{}, []action.SealedEnvelope{}
	}

	t.Run("no-expiry", func(t *testing.T) {
		apConfig := getActPoolCfg()
		ap, transfers, _, executions := createActPool(apConfig)
		pickedActs := ap.PendingActionMap()
		require.Equal(len(transfers)+len(executions), lenPendingActionMap(pickedActs))
	})
	t.Run("expiry", func(t *testing.T) {
		apConfig := getActPoolCfg()
		apConfig.ActionExpiry = time.Second
		ap, _, _, _ := createActPool(apConfig)
		require.NoError(testutil.WaitUntil(100*time.Millisecond, 10*time.Second, func() (bool, error) {
			pickedActs := ap.PendingActionMap()
			return lenPendingActionMap(pickedActs) == 0, nil
		}))
	})
}

func TestActPool_removeConfirmedActs(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(sf, apConfig, EnableExperimentalActions())
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

	tsf1, err := action.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(addr1, priKey1, uint64(2), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(addr1, priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf4, err := action.SignedTransfer(addr1, priKey1, uint64(4), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)

	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := account.(*state.Account)
		require.True(ok)
		acct.Nonce = 0
		acct.Balance = big.NewInt(100000000000000000)

		return 0, nil
	}).Times(9)
	require.NoError(ap.Add(context.Background(), tsf1))
	require.NoError(ap.Add(context.Background(), tsf2))
	require.NoError(ap.Add(context.Background(), tsf3))
	require.NoError(ap.Add(context.Background(), tsf4))

	require.Equal(4, len(ap.allActions))
	require.NotNil(ap.accountActs[addr1])
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := account.(*state.Account)
		require.True(ok)
		acct.Nonce = 4
		acct.Balance = big.NewInt(100000000000000000)

		return 0, nil
	}).Times(1)
	ap.removeConfirmedActs()
	require.Equal(0, len(ap.allActions))
	require.Nil(ap.accountActs[addr1])
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
			acct.Balance = new(big.Int).Set(balances[0])
			acct.Nonce = nonces[0]
		case bytes.Equal(cfg.Key, identityset.Address(29).Bytes()):
			acct.Balance = new(big.Int).Set(balances[1])
			acct.Nonce = nonces[1]
		case bytes.Equal(cfg.Key, identityset.Address(30).Bytes()):
			acct.Balance = new(big.Int).Set(balances[2])
			acct.Nonce = nonces[2]
		case bytes.Equal(cfg.Key, identityset.Address(31).Bytes()):
			acct.Balance = new(big.Int).Set(balances[3])
			acct.Nonce = nonces[3]
		case bytes.Equal(cfg.Key, identityset.Address(32).Bytes()):
			acct.Balance = new(big.Int).Set(balances[4])
			acct.Nonce = nonces[4]
		}
		return 0, nil
	}).AnyTimes()

	apConfig := getActPoolCfg()
	Ap1, err := NewActPool(sf, apConfig, EnableExperimentalActions())
	require.NoError(err)
	ap1, ok := Ap1.(*actPool)
	require.True(ok)
	ap1.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	Ap2, err := NewActPool(sf, apConfig, EnableExperimentalActions())
	require.NoError(err)
	ap2, ok := Ap2.(*actPool)
	require.True(ok)
	ap2.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

	// Tsfs to be added to ap1
	tsf1, err := action.SignedTransfer(addr2, priKey1, uint64(1), big.NewInt(50), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(addr3, priKey1, uint64(2), big.NewInt(30), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(addr2, priKey1, uint64(3), big.NewInt(60), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf4, err := action.SignedTransfer(addr1, priKey2, uint64(1), big.NewInt(100), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf5, err := action.SignedTransfer(addr3, priKey2, uint64(2), big.NewInt(50), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf6, err := action.SignedTransfer(addr1, priKey2, uint64(3), big.NewInt(60), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf7, err := action.SignedTransfer(addr1, priKey3, uint64(1), big.NewInt(100), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf8, err := action.SignedTransfer(addr2, priKey3, uint64(2), big.NewInt(100), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf9, err := action.SignedTransfer(addr1, priKey3, uint64(4), big.NewInt(100), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)

	ctx := context.Background()
	require.NoError(ap1.Add(ctx, tsf1))
	require.NoError(ap1.Add(ctx, tsf2))
	err = ap1.Add(ctx, tsf3)
	require.Equal(action.ErrBalance, errors.Cause(err))
	require.NoError(ap1.Add(ctx, tsf4))
	require.NoError(ap1.Add(ctx, tsf5))
	err = ap1.Add(ctx, tsf6)
	require.Equal(action.ErrBalance, errors.Cause(err))
	require.NoError(ap1.Add(ctx, tsf7))
	require.NoError(ap1.Add(ctx, tsf8))
	require.NoError(ap1.Add(ctx, tsf9))
	// Tsfs to be added to ap2 only
	tsf10, err := action.SignedTransfer(addr2, priKey1, uint64(3), big.NewInt(20), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf11, err := action.SignedTransfer(addr3, priKey1, uint64(4), big.NewInt(10), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf12, err := action.SignedTransfer(addr3, priKey2, uint64(2), big.NewInt(70), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf13, err := action.SignedTransfer(addr1, priKey3, uint64(1), big.NewInt(200), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf14, err := action.SignedTransfer(addr2, priKey3, uint64(2), big.NewInt(50), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)

	require.NoError(ap2.Add(ctx, tsf1))
	require.NoError(ap2.Add(ctx, tsf2))
	require.NoError(ap2.Add(ctx, tsf10))
	err = ap2.Add(ctx, tsf11)
	require.Equal(action.ErrBalance, errors.Cause(err))
	require.NoError(ap2.Add(ctx, tsf4))
	require.NoError(ap2.Add(ctx, tsf12))
	require.NoError(ap2.Add(ctx, tsf13))
	require.NoError(ap2.Add(ctx, tsf14))
	err = ap2.Add(ctx, tsf9)
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
	tsf15, err := action.SignedTransfer(addr2, priKey3, uint64(3), big.NewInt(80), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	// Tsfs To be added to ap2 only
	tsf16, err := action.SignedTransfer(addr2, priKey1, uint64(4), big.NewInt(150), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf17, err := action.SignedTransfer(addr1, priKey2, uint64(3), big.NewInt(90), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf18, err := action.SignedTransfer(addr3, priKey2, uint64(4), big.NewInt(100), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf19, err := action.SignedTransfer(addr1, priKey2, uint64(5), big.NewInt(50), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf20, err := action.SignedTransfer(addr2, priKey3, uint64(3), big.NewInt(200), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)

	require.NoError(ap1.Add(ctx, tsf15))
	require.NoError(ap2.Add(ctx, tsf16))
	require.NoError(ap2.Add(ctx, tsf17))
	require.NoError(ap2.Add(ctx, tsf18))
	err = ap2.Add(ctx, tsf19)
	require.Equal(action.ErrBalance, errors.Cause(err))
	err = ap2.Add(ctx, tsf20)
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
	tsf21, err := action.SignedTransfer(addr5, priKey4, uint64(1), big.NewInt(10), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf22, err := action.SignedTransfer(addr5, priKey4, uint64(2), big.NewInt(10), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf23, err := action.NewTransfer(uint64(3), big.NewInt(1), "", []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)

	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(3).
		SetGasLimit(20000).
		SetAction(tsf23).Build()
	selp23, err := action.Sign(elp, priKey4)
	require.NoError(err)

	tsf24, err := action.SignedTransfer(addr5, priKey5, uint64(1), big.NewInt(10), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf25, err := action.SignedTransfer(addr4, priKey5, uint64(2), big.NewInt(10), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf26, err := action.NewTransfer(uint64(3), big.NewInt(1), addr4, []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(3).
		SetGasLimit(20000).
		SetAction(tsf26).Build()
	selp26, err := action.Sign(elp, priKey5)
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
	ap1PNonce4, _ := ap1.getPendingNonce(addr4)
	require.Equal(uint64(2), ap1PNonce4)
	ap1PBalance4, _ := ap1.getPendingBalance(addr4)
	require.Equal(big.NewInt(0).Uint64(), ap1PBalance4.Uint64())
	// Addr5
	ap1PNonce5, _ := ap1.getPendingNonce(addr5)
	require.Equal(uint64(3), ap1PNonce5)
	ap1PBalance5, _ := ap1.getPendingBalance(addr5)
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
	ap1PNonce4, _ = ap1.getPendingNonce(addr4)
	require.Equal(uint64(2), ap1PNonce4)
	ap1PBalance4, _ = ap1.getPendingBalance(addr4)
	require.Equal(big.NewInt(10).Uint64(), ap1PBalance4.Uint64())
	// Addr5
	ap1PNonce5, _ = ap1.getPendingNonce(addr5)
	require.Equal(uint64(3), ap1PNonce5)
	ap1PBalance5, _ = ap1.getPendingBalance(addr5)
	require.Equal(big.NewInt(20).Uint64(), ap1PBalance5.Uint64())
}

func TestActPool_removeInvalidActs(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(sf, apConfig, EnableExperimentalActions())
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

	tsf1, err := action.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(addr1, priKey1, uint64(2), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(addr1, priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf4, err := action.SignedTransfer(addr1, priKey1, uint64(4), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := account.(*state.Account)
		require.True(ok)
		acct.Nonce = 0
		acct.Balance = big.NewInt(100000000000000000)

		return 0, nil
	}).Times(9)
	require.NoError(ap.Add(context.Background(), tsf1))
	require.NoError(ap.Add(context.Background(), tsf2))
	require.NoError(ap.Add(context.Background(), tsf3))
	require.NoError(ap.Add(context.Background(), tsf4))

	hash1 := tsf1.Hash()
	hash2 := tsf4.Hash()
	acts := []action.SealedEnvelope{tsf1, tsf4}
	require.NotNil(ap.allActions[hash1])
	require.NotNil(ap.allActions[hash2])
	ap.removeInvalidActs(acts)
	require.Equal(action.SealedEnvelope{}, ap.allActions[hash1])
	require.Equal(action.SealedEnvelope{}, ap.allActions[hash2])
}

func TestActPool_GetPendingNonce(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(sf, apConfig, EnableExperimentalActions())
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

	tsf1, err := action.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(addr1, priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf4, err := action.SignedTransfer(addr1, priKey1, uint64(4), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := account.(*state.Account)
		require.True(ok)
		acct.Nonce = 0
		acct.Balance = big.NewInt(100000000000000000)

		return 0, nil
	}).Times(8)

	require.NoError(ap.Add(context.Background(), tsf1))
	require.NoError(ap.Add(context.Background(), tsf3))
	require.NoError(ap.Add(context.Background(), tsf4))

	nonce, err := ap.GetPendingNonce(addr2)
	require.NoError(err)
	require.Equal(uint64(1), nonce)

	nonce, err = ap.GetPendingNonce(addr1)
	require.NoError(err)
	require.Equal(uint64(2), nonce)
}

func TestActPool_GetUnconfirmedActs(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(sf, apConfig, EnableExperimentalActions())
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

	tsf1, err := action.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(addr1, priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf4, err := action.SignedTransfer(addr1, priKey1, uint64(4), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf5, err := action.SignedTransfer(addr1, priKey2, uint64(1), big.NewInt(30), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)

	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := account.(*state.Account)
		require.True(ok)
		acct.Nonce = 0
		acct.Balance = big.NewInt(100000000000000000)

		return 0, nil
	}).Times(10)
	require.NoError(ap.Add(context.Background(), tsf1))
	require.NoError(ap.Add(context.Background(), tsf3))
	require.NoError(ap.Add(context.Background(), tsf4))
	require.NoError(ap.Add(context.Background(), tsf5))

	acts := ap.GetUnconfirmedActs(addr3)
	require.Equal([]action.SealedEnvelope(nil), acts)

	acts = ap.GetUnconfirmedActs(addr1)
	require.Equal([]action.SealedEnvelope{tsf1, tsf3, tsf4, tsf5}, acts)
}

func TestActPool_GetActionByHash(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)

	sf := mock_chainmanager.NewMockStateReader(ctrl)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(sf, apConfig, EnableExperimentalActions())
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)

	tsf1, err := action.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	hash1 := tsf1.Hash()
	tsf2, err := action.SignedTransfer(addr1, priKey1, uint64(2), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	hash2 := tsf2.Hash()

	ap.allActions[hash1] = tsf1
	act, err := ap.GetActionByHash(hash1)
	require.NoError(err)
	require.Equal(tsf1, act)
	act, err = ap.GetActionByHash(hash2)
	require.Equal(action.ErrNotFound, errors.Cause(err))
	require.Equal(action.SealedEnvelope{}, act)

	ap.allActions[hash2] = tsf2
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
	Ap, err := NewActPool(sf, apConfig, EnableExperimentalActions())
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	require.Equal(uint64(maxNumActsPerPool), ap.GetCapacity())
	require.Equal(uint64(maxGasLimitPerPool), ap.GetGasCapacity())
}

func TestActPool_GetSize(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	// Create actpool
	apConfig := getActPoolCfg()
	Ap, err := NewActPool(sf, apConfig, EnableExperimentalActions())
	require.NoError(err)
	ap, ok := Ap.(*actPool)
	require.True(ok)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	require.Zero(ap.GetSize())
	require.Zero(ap.GetGasSize())

	tsf1, err := action.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(addr1, priKey1, uint64(2), big.NewInt(20), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(addr1, priKey1, uint64(3), big.NewInt(30), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)
	tsf4, err := action.SignedTransfer(addr1, priKey1, uint64(4), big.NewInt(30), []byte{}, uint64(20000), big.NewInt(0))
	require.NoError(err)

	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := account.(*state.Account)
		require.True(ok)
		acct.Nonce = 0
		acct.Balance = big.NewInt(100000000000000000)

		return 0, nil
	}).Times(9)
	require.NoError(ap.Add(context.Background(), tsf1))
	require.NoError(ap.Add(context.Background(), tsf2))
	require.NoError(ap.Add(context.Background(), tsf3))
	require.NoError(ap.Add(context.Background(), tsf4))
	require.Equal(uint64(4), ap.GetSize())
	require.Equal(uint64(40000), ap.GetGasSize())
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := account.(*state.Account)
		require.True(ok)
		acct.Nonce = 4
		acct.Balance = big.NewInt(100000000000000000)

		return 0, nil
	}).Times(1)
	ap.removeConfirmedActs()
	require.Equal(uint64(0), ap.GetSize())
	require.Equal(uint64(0), ap.GetGasSize())
}

func TestActPool_AddActionNotEnoughGasPrice(t *testing.T) {
	ctrl := gomock.NewController(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)

	apConfig := config.Default.ActPool
	ap, err := NewActPool(sf, apConfig, EnableExperimentalActions())
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
	require.Error(t, ap.Add(ctx, tsf))
}

// Helper function to return the correct pending nonce just in case of empty queue
func (ap *actPool) getPendingNonce(addr string) (uint64, error) {
	if queue, ok := ap.accountActs[addr]; ok {
		return queue.PendingNonce(), nil
	}
	committedState, err := accountutil.AccountState(ap.sf, addr)
	return committedState.Nonce + 1, err
}

// Helper function to return the correct pending balance just in case of empty queue
func (ap *actPool) getPendingBalance(addr string) (*big.Int, error) {
	if queue, ok := ap.accountActs[addr]; ok {
		return queue.PendingBalance(), nil
	}
	state, err := accountutil.AccountState(ap.sf, addr)
	if err != nil {
		return nil, err
	}

	return state.Balance, nil
}

func getActPoolCfg() config.ActPool {
	return config.ActPool{
		MaxNumActsPerPool:  maxNumActsPerPool,
		MaxGasLimitPerPool: maxGasLimitPerPool,
		MaxNumActsPerAcct:  maxNumActsPerAcct,
		MinGasPriceStr:     "0",
		BlackList:          []string{addr6},
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
