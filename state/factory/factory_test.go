// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/test/mock/mock_committee"
	"github.com/iotexproject/iotex-election/types"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	_triePath    = "trie.test"
	_stateDBPath = "stateDB.test"
)

var _letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = _letterRunes[rand.Intn(len(_letterRunes))]
	}
	return string(b)
}

func TestSnapshot(t *testing.T) {
	require := require.New(t)
	testTriePath, err := testutil.PathOfTempFile(_triePath)
	require.NoError(err)

	cfg := config.Default
	cfg.DB.DbPath = testTriePath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "5"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "7"
	registry := protocol.NewRegistry()
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewBoltDB(cfg.DB)), RegistryOption(registry))
	require.NoError(err)
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	ctx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
		testutil.CleanupPath(testTriePath)
	}()
	ws, err := sf.(workingSetCreator).newWorkingSet(ctx, 1)
	require.NoError(err)
	testSnapshot(ws, t)
	testRevert(ws, t)
}

func TestSDBSnapshot(t *testing.T) {
	require := require.New(t)
	testStateDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(err)
	defer testutil.CleanupPath(testStateDBPath)

	cfg := config.Default
	cfg.Chain.TrieDBPatchFile = ""
	cfg.Chain.TrieDBPath = testStateDBPath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "5"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "7"
	registry := protocol.NewRegistry()
	sdb, err := NewStateDB(cfg, CachedStateDBOption(), RegistryStateDBOption(registry))
	require.NoError(err)
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	ctx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)
	require.NoError(sdb.Start(ctx))
	height, err := sdb.Height()
	require.NoError(err)
	ws, err := sdb.(workingSetCreator).newWorkingSet(ctx, height)
	require.NoError(err)
	testSnapshot(ws, t)
	testSDBRevert(ws, t)
}

func testRevert(ws *workingSet, t *testing.T) {
	require := require.New(t)
	sHash := hash.BytesToHash160(identityset.Address(28).Bytes())

	s, err := accountutil.LoadAccount(ws, identityset.Address(28))
	require.NoError(err)
	require.Equal(big.NewInt(5), s.Balance)
	s0 := ws.Snapshot()
	require.Equal(1, s0)

	require.NoError(s.AddBalance(big.NewInt(5)))
	require.Equal(big.NewInt(10), s.Balance)
	_, err = ws.PutState(s, protocol.LegacyKeyOption(sHash))
	require.NoError(err)

	require.NoError(ws.Revert(s0))
	_, err = ws.State(s, protocol.LegacyKeyOption(sHash))
	require.NoError(err)
	require.Equal(big.NewInt(5), s.Balance)
}

func testSDBRevert(ws *workingSet, t *testing.T) {
	require := require.New(t)
	sHash := hash.BytesToHash160(identityset.Address(28).Bytes())

	s, err := accountutil.LoadAccount(ws, identityset.Address(28))
	require.NoError(err)
	require.Equal(big.NewInt(5), s.Balance)
	s0 := ws.Snapshot()
	require.Equal(1, s0)

	require.NoError(s.AddBalance(big.NewInt(5)))
	require.Equal(big.NewInt(10), s.Balance)
	_, err = ws.PutState(s, protocol.LegacyKeyOption(sHash))
	require.NoError(err)

	require.NoError(ws.Revert(s0))
	_, err = ws.State(s, protocol.LegacyKeyOption(sHash))
	require.NoError(err)
	require.Equal(big.NewInt(5), s.Balance)
}

func testSnapshot(ws *workingSet, t *testing.T) {
	require := require.New(t)
	sHash := hash.BytesToHash160(identityset.Address(28).Bytes())
	sHashAddr := identityset.Address(28)
	tHash := hash.BytesToHash160(identityset.Address(29).Bytes())
	tHashAddr := identityset.Address(29)

	s, err := accountutil.LoadAccount(ws, tHashAddr)
	require.NoError(err)
	require.Equal(big.NewInt(7), s.Balance)
	s, err = accountutil.LoadAccount(ws, sHashAddr)
	require.NoError(err)
	require.Equal(big.NewInt(5), s.Balance)
	s0 := ws.Snapshot()
	require.Zero(s0)
	require.NoError(s.AddBalance(big.NewInt(5)))
	require.Equal(big.NewInt(10), s.Balance)
	_, err = ws.PutState(s, protocol.LegacyKeyOption(sHash))
	require.NoError(err)
	s1 := ws.Snapshot()
	require.Equal(1, s1)
	require.NoError(s.AddBalance(big.NewInt(5)))
	require.Equal(big.NewInt(15), s.Balance)
	_, err = ws.PutState(s, protocol.LegacyKeyOption(sHash))
	require.NoError(err)

	s, err = accountutil.LoadAccount(ws, tHashAddr)
	require.NoError(err)
	require.Equal(big.NewInt(7), s.Balance)
	s2 := ws.Snapshot()
	require.Equal(2, s2)
	require.NoError(s.AddBalance(big.NewInt(6)))
	require.Equal(big.NewInt(13), s.Balance)
	_, err = ws.PutState(s, protocol.LegacyKeyOption(tHash))
	require.NoError(err)

	require.NoError(ws.Revert(s2))
	_, err = ws.State(s, protocol.LegacyKeyOption(sHash))
	require.NoError(err)
	require.Equal(big.NewInt(15), s.Balance)
	_, err = ws.State(s, protocol.LegacyKeyOption(tHash))
	require.NoError(err)
	require.Equal(big.NewInt(7), s.Balance)
	require.NoError(ws.Revert(s1))
	_, err = ws.State(s, protocol.LegacyKeyOption(sHash))
	require.NoError(err)
	require.Equal(big.NewInt(10), s.Balance)
	require.NoError(ws.Revert(s0))
	_, err = ws.State(s, protocol.LegacyKeyOption(sHash))
	require.NoError(err)
	require.Equal(big.NewInt(5), s.Balance)
}

func TestCandidates(t *testing.T) {
	cfg := config.Default
	sf, err := NewFactory(cfg, InMemTrieOption(), SkipBlockValidationOption())
	require.NoError(t, err)
	testCandidates(sf, t)
}

func TestSDBCandidates(t *testing.T) {
	cfg := config.Default
	sdb, err := NewStateDB(cfg, InMemStateDBOption(), SkipBlockValidationStateDBOption())
	require.NoError(t, err)
	testCandidates(sdb, t)
}

func testCandidates(sf Factory, t *testing.T) {
	sc := state.CandidateList{}
	result := types.NewElectionResultForTest(time.Now())
	for _, c := range result.Delegates() {
		oa, err := address.FromString(string(c.OperatorAddress()))
		require.NoError(t, err)
		ra, err := address.FromString(string(c.RewardAddress()))
		require.NoError(t, err)
		sc = append(sc, &state.Candidate{
			Address:       oa.String(),
			Votes:         c.Score(),
			RewardAddress: ra.String(),
			CanName:       c.Name(),
		})
	}
	act := action.NewPutPollResult(1, 1, sc)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(act).Build()
	selp, err := action.Sign(elp, identityset.PrivateKey(27))
	require.NoError(t, err)
	require.NotNil(t, selp)

	ctrl := gomock.NewController(t)

	committee := mock_committee.NewMockCommittee(ctrl)
	committee.EXPECT().ResultByHeight(uint64(123456)).Return(result, nil).AnyTimes()
	committee.EXPECT().HeightByTime(gomock.Any()).Return(uint64(123456), nil).AnyTimes()

	require.NoError(t, sf.Register(rolldpos.NewProtocol(36, 36, 20)))
	cfg := config.Default
	slasher, err := poll.NewSlasher(
		func(uint64, uint64) (map[string]uint64, error) {
			return nil, nil
		},
		nil,
		nil,
		nil,
		nil,
		cfg.Genesis.NumCandidateDelegates,
		cfg.Genesis.NumDelegates,
		cfg.Genesis.DardanellesNumSubEpochs,
		cfg.Genesis.ProductivityThreshold,
		cfg.Genesis.ProbationEpochPeriod,
		cfg.Genesis.UnproductiveDelegateMaxCacheSize,
		cfg.Genesis.ProbationIntensityRate)
	require.NoError(t, err)
	p, err := poll.NewGovernanceChainCommitteeProtocol(
		nil,
		committee,
		uint64(123456),
		func(uint64) (time.Time, error) { return time.Now(), nil },
		cfg.Chain.PollInitialCandidatesInterval,
		slasher,
	)
	require.NoError(t, err)
	require.NoError(t, sf.Register(p))
	gasLimit := testutil.TestGasLimit

	// TODO: investigate why registry cannot be added in the Blockchain Ctx
	ctx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{
			BlockHeight: 0,
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		},
	)
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
	require.NoError(t, sf.Start(ctx))
	defer require.NoError(t, sf.Stop(ctx))

	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions([]action.SealedEnvelope{selp}...).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)
	require.NoError(t, sf.PutBlock(
		protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(
			protocol.WithBlockchainCtx(
				protocol.WithBlockCtx(
					genesis.WithGenesisContext(context.Background(), cfg.Genesis),
					protocol.BlockCtx{
						BlockHeight: 1,
						Producer:    identityset.Address(27),
						GasLimit:    gasLimit,
					},
				),
				protocol.BlockchainCtx{
					ChainID: 1,
				},
			),
		),
		), &blk))

	candidates, _, err := candidatesutil.CandidatesFromDB(sf, 1, true, false)
	require.NoError(t, err)
	require.Equal(t, len(sc), len(candidates))
	for i, c := range candidates {
		require.True(t, c.Equal(sc[i]))
	}
}

func TestState(t *testing.T) {
	testTriePath, err := testutil.PathOfTempFile(_triePath)
	require.NoError(t, err)
	defer testutil.CleanupPath(testTriePath)

	cfg := config.Default
	cfg.DB.DbPath = testTriePath
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewBoltDB(cfg.DB)), SkipBlockValidationOption())
	require.NoError(t, err)
	testState(sf, t)
}

func TestHistoryState(t *testing.T) {
	r := require.New(t)
	var err error
	// using factory and enable history
	cfg := config.Default
	cfg.Chain.TrieDBPath, err = testutil.PathOfTempFile(_triePath)
	r.NoError(err)
	cfg.Chain.EnableArchiveMode = true
	sf, err := NewFactory(cfg, DefaultTrieOption(), SkipBlockValidationOption())
	r.NoError(err)
	testHistoryState(sf, t, false, cfg.Chain.EnableArchiveMode)

	// using stateDB and enable history
	cfg.Chain.TrieDBPath, err = testutil.PathOfTempFile(_triePath)
	r.NoError(err)
	sf, err = NewStateDB(cfg, CachedStateDBOption(), SkipBlockValidationStateDBOption())
	r.NoError(err)
	testHistoryState(sf, t, true, cfg.Chain.EnableArchiveMode)

	// using factory and disable history
	cfg.Chain.TrieDBPath, err = testutil.PathOfTempFile(_triePath)
	r.NoError(err)
	cfg.Chain.EnableArchiveMode = false
	sf, err = NewFactory(cfg, DefaultTrieOption(), SkipBlockValidationOption())
	r.NoError(err)
	testHistoryState(sf, t, false, cfg.Chain.EnableArchiveMode)

	// using stateDB and disable history
	cfg.Chain.TrieDBPath, err = testutil.PathOfTempFile(_triePath)
	r.NoError(err)
	sf, err = NewStateDB(cfg, CachedStateDBOption(), SkipBlockValidationStateDBOption())
	r.NoError(err)
	testHistoryState(sf, t, true, cfg.Chain.EnableArchiveMode)
	defer func() {
		testutil.CleanupPath(cfg.Chain.TrieDBPath)
	}()
}

func TestFactoryStates(t *testing.T) {
	r := require.New(t)
	var err error
	cfg := config.Default

	// using factory
	cfg.Chain.TrieDBPath, err = testutil.PathOfTempFile(_triePath)
	r.NoError(err)
	sf, err := NewFactory(cfg, DefaultTrieOption(), SkipBlockValidationOption())
	r.NoError(err)
	testFactoryStates(sf, t)

	// using stateDB
	cfg.Chain.TrieDBPath, err = testutil.PathOfTempFile(_triePath)
	r.NoError(err)
	sf, err = NewStateDB(cfg, CachedStateDBOption(), SkipBlockValidationStateDBOption())
	r.NoError(err)
	testFactoryStates(sf, t)
	defer testutil.CleanupPath(cfg.Chain.TrieDBPath)
}

func TestSDBState(t *testing.T) {
	testDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(t, err)
	defer testutil.CleanupPath(testDBPath)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testDBPath
	sdb, err := NewStateDB(cfg, CachedStateDBOption(), SkipBlockValidationStateDBOption())
	require.NoError(t, err)
	testState(sdb, t)
}

func testState(sf Factory, t *testing.T) {
	// Create a dummy iotex address
	a := identityset.Address(28)
	priKeyA := identityset.PrivateKey(28)
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, sf.Register(acc))
	ge := genesis.Default
	ge.InitBalanceMap[a.String()] = "100"
	gasLimit := uint64(1000000)
	ctx := protocol.WithBlockchainCtx(protocol.WithBlockCtx(
		context.Background(),
		protocol.BlockCtx{
			BlockHeight: 0,
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		},
	), protocol.BlockchainCtx{
		ChainID: 1,
	})
	ctx = genesis.WithGenesisContext(ctx, ge)

	require.NoError(t, sf.Start(ctx))
	defer func() {
		require.NoError(t, sf.Stop(ctx))
	}()

	tsf, err := action.NewTransfer(1, big.NewInt(10), identityset.Address(31).String(), nil, uint64(20000), big.NewInt(0))
	require.NoError(t, err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tsf).SetGasLimit(20000).SetNonce(1).Build()
	selp, err := action.Sign(elp, priKeyA)
	require.NoError(t, err)
	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight: 1,
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		},
	)
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions([]action.SealedEnvelope{selp}...).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)
	require.NoError(t, sf.PutBlock(ctx, &blk))

	//test AccountState() & State()
	testAccount := state.NewEmptyAccount()
	accountA, err := accountutil.AccountState(sf, a)
	require.NoError(t, err)
	sHash := hash.BytesToHash160(identityset.Address(28).Bytes())
	_, err = sf.State(testAccount, protocol.LegacyKeyOption(sHash))
	require.NoError(t, err)
	require.Equal(t, accountA, testAccount)
	require.Equal(t, big.NewInt(90), accountA.Balance)
}

func testHistoryState(sf Factory, t *testing.T, statetx, archive bool) {
	// Create a dummy iotex address
	a := identityset.Address(28)
	b := identityset.Address(31)
	priKeyA := identityset.PrivateKey(28)
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, sf.Register(acc))
	ge := genesis.Default
	ge.InitBalanceMap[a.String()] = "100"
	gasLimit := uint64(1000000)
	ctx := protocol.WithBlockCtx(
		context.Background(),
		protocol.BlockCtx{
			BlockHeight: 0,
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		},
	)
	ctx = genesis.WithGenesisContext(ctx, ge)

	require.NoError(t, sf.Start(ctx))
	defer func() {
		require.NoError(t, sf.Stop(ctx))
	}()
	accountA, err := accountutil.AccountState(sf, a)
	require.NoError(t, err)
	accountB, err := accountutil.AccountState(sf, b)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(100), accountA.Balance)
	require.Equal(t, big.NewInt(0), accountB.Balance)
	tsf, err := action.NewTransfer(1, big.NewInt(10), b.String(), nil, uint64(20000), big.NewInt(0))
	require.NoError(t, err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tsf).SetGasLimit(20000).SetNonce(1).Build()
	selp, err := action.Sign(elp, priKeyA)
	require.NoError(t, err)
	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight: 1,
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		},
	)
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{
		ChainID: 1,
	})
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions([]action.SealedEnvelope{selp}...).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)
	require.NoError(t, sf.PutBlock(ctx, &blk))

	// check latest balance
	accountA, err = accountutil.AccountState(sf, a)
	require.NoError(t, err)
	accountB, err = accountutil.AccountState(sf, b)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(90), accountA.Balance)
	require.Equal(t, big.NewInt(10), accountB.Balance)

	// check archive data
	if statetx {
		// statetx not support archive mode
		_, err = accountutil.AccountState(NewHistoryStateReader(sf, 0), a)
		require.Equal(t, ErrNotSupported, errors.Cause(err))
		_, err = accountutil.AccountState(NewHistoryStateReader(sf, 0), b)
		require.Equal(t, ErrNotSupported, errors.Cause(err))
	} else {
		if !archive {
			_, err = accountutil.AccountState(NewHistoryStateReader(sf, 0), a)
			require.Equal(t, ErrNoArchiveData, errors.Cause(err))
			_, err = accountutil.AccountState(NewHistoryStateReader(sf, 0), b)
			require.Equal(t, ErrNoArchiveData, errors.Cause(err))
		} else {
			accountA, err = accountutil.AccountState(NewHistoryStateReader(sf, 0), a)
			require.NoError(t, err)
			accountB, err = accountutil.AccountState(NewHistoryStateReader(sf, 0), b)
			require.NoError(t, err)
			require.Equal(t, big.NewInt(100), accountA.Balance)
			require.Equal(t, big.NewInt(0), accountB.Balance)
		}
	}
}

func testFactoryStates(sf Factory, t *testing.T) {
	// Create a dummy iotex address
	a := identityset.Address(28).String()
	b := identityset.Address(31).String()
	priKeyA := identityset.PrivateKey(28)
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, sf.Register(acc))
	ge := genesis.Default
	ge.InitBalanceMap = make(map[string]string)
	ge.InitBalanceMap[a] = "100"
	ge.InitBalanceMap[b] = "100"
	gasLimit := uint64(1000000)
	ctx := protocol.WithBlockCtx(
		context.Background(),
		protocol.BlockCtx{
			BlockHeight: 0,
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		},
	)
	ctx = genesis.WithGenesisContext(ctx, ge)

	require.NoError(t, sf.Start(ctx))
	defer func() {
		require.NoError(t, sf.Stop(ctx))
	}()
	tsf, err := action.NewTransfer(1, big.NewInt(10), b, nil, uint64(20000), big.NewInt(0))
	require.NoError(t, err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tsf).SetGasLimit(20000).SetNonce(1).Build()
	selp, err := action.Sign(elp, priKeyA)
	require.NoError(t, err)
	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight: 1,
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		},
	)
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{
		ChainID: 1,
	})
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions([]action.SealedEnvelope{selp}...).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)
	require.NoError(t, sf.PutBlock(ctx, &blk))

	// case I: check KeyOption
	keyOpt := protocol.KeyOption([]byte(""))
	_, _, err = sf.States(keyOpt)
	require.Equal(t, ErrNotSupported, errors.Cause(err))

	// case II: check without namespace & keys
	height, iter, err := sf.States()
	require.NoError(t, err)
	require.Equal(t, uint64(1), height)
	// two accounts and one CurrentHeightKey
	require.Equal(t, 3, iter.Size())
	accounts := make([]*state.Account, 0)
	for i := 0; i < iter.Size(); i++ {
		c := state.NewEmptyAccount()
		err = iter.Next(c)
		if err != nil {
			continue
		}
		accounts = append(accounts, c)
	}
	require.Equal(t, uint64(90), accounts[0].Balance.Uint64())
	require.Equal(t, uint64(110), accounts[1].Balance.Uint64())

	// case III: check without cond,with AccountKVNamespace namespace,key not exists
	namespaceOpt := protocol.NamespaceOption(AccountKVNamespace)
	height, iter, err = sf.States(namespaceOpt)
	require.NoError(t, err)
	require.Equal(t, uint64(1), height)
	// two accounts and one CurrentHeightKey
	require.Equal(t, 3, iter.Size())
	accounts = make([]*state.Account, 0)
	for i := 0; i < iter.Size(); i++ {
		c := state.NewEmptyAccount()
		err = iter.Next(c)
		if err != nil {
			continue
		}
		accounts = append(accounts, c)
	}
	require.Equal(t, uint64(90), accounts[0].Balance.Uint64())
	require.Equal(t, uint64(110), accounts[1].Balance.Uint64())

	// case IV: check without cond,with AccountKVNamespace namespace
	namespaceOpt = protocol.NamespaceOption(AccountKVNamespace)
	height, iter, err = sf.States(namespaceOpt)
	require.NoError(t, err)
	require.Equal(t, uint64(1), height)
	// two accounts and one CurrentHeightKey
	require.Equal(t, 3, iter.Size())
	accounts = make([]*state.Account, 0)
	for i := 0; i < iter.Size(); i++ {
		c := state.NewEmptyAccount()
		err = iter.Next(c)
		if err != nil {
			continue
		}
		accounts = append(accounts, c)
	}
	require.Equal(t, uint64(90), accounts[0].Balance.Uint64())
	require.Equal(t, uint64(110), accounts[1].Balance.Uint64())

	// case V: check cond,with AccountKVNamespace namespace
	namespaceOpt = protocol.NamespaceOption(AccountKVNamespace)
	addrHash := hash.BytesToHash160(identityset.Address(28).Bytes())
	height, iter, err = sf.States(namespaceOpt, protocol.KeysOption(func() ([][]byte, error) {
		return [][]byte{addrHash[:]}, nil
	}))
	require.NoError(t, err)
	require.Equal(t, uint64(1), height)
	require.Equal(t, 1, iter.Size())
	accounts = make([]*state.Account, 0)
	for i := 0; i < iter.Size(); i++ {
		c := state.NewEmptyAccount()
		require.NoError(t, iter.Next(c))
		accounts = append(accounts, c)
	}
	require.Equal(t, uint64(90), accounts[0].Balance.Uint64())
}

func TestNonce(t *testing.T) {
	testTriePath, err := testutil.PathOfTempFile(_triePath)
	require.NoError(t, err)
	defer testutil.CleanupPath(testTriePath)

	cfg := config.Default
	cfg.DB.DbPath = testTriePath
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewBoltDB(cfg.DB)), SkipBlockValidationOption())
	require.NoError(t, err)
	testNonce(sf, t)
}

func TestSDBNonce(t *testing.T) {
	testDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(t, err)
	defer testutil.CleanupPath(testDBPath)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testDBPath
	sdb, err := NewStateDB(cfg, CachedStateDBOption(), SkipBlockValidationStateDBOption())
	require.NoError(t, err)

	testNonce(sdb, t)
}

func testNonce(sf Factory, t *testing.T) {
	// Create two dummy iotex address
	a := identityset.Address(28)
	priKeyA := identityset.PrivateKey(28)
	b := identityset.Address(29).String()

	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, sf.Register(acc))
	ge := genesis.Default
	ge.InitBalanceMap[a.String()] = "100"
	gasLimit := uint64(1000000)
	ctx := protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			BlockHeight: 0,
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		})
	ctx = protocol.WithFeatureCtx(genesis.WithGenesisContext(ctx, ge))

	require.NoError(t, sf.Start(ctx))
	defer func() {
		require.NoError(t, sf.Stop(ctx))
	}()
	ws, err := sf.(workingSetCreator).newWorkingSet(ctx, 1)
	require.NoError(t, err)

	tx, err := action.NewTransfer(0, big.NewInt(2), b, nil, uint64(20000), big.NewInt(0))
	require.NoError(t, err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tx).SetNonce(0).SetGasLimit(20000).Build()
	selp, err := action.Sign(elp, priKeyA)
	require.NoError(t, err)

	ctx = protocol.WithBlockCtx(ctx,
		protocol.BlockCtx{
			BlockHeight: 1,
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		})
	intrinsicGas, err := selp.IntrinsicGas()
	require.NoError(t, err)
	selpHash, err := selp.Hash()
	require.NoError(t, err)
	ctx = protocol.WithActionCtx(
		ctx,
		protocol.ActionCtx{
			Caller:       a,
			ActionHash:   selpHash,
			GasPrice:     selp.GasPrice(),
			Nonce:        selp.Nonce(),
			IntrinsicGas: intrinsicGas,
		},
	)
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{
		ChainID: 1,
	})
	_, err = ws.runAction(ctx, selp)
	require.NoError(t, err)
	state, err := accountutil.AccountState(sf, a)
	require.NoError(t, err)
	require.Equal(t, uint64(1), state.PendingNonce())

	tx, err = action.NewTransfer(1, big.NewInt(2), b, nil, uint64(20000), big.NewInt(0))
	require.NoError(t, err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx).SetNonce(1).SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, priKeyA)
	require.NoError(t, err)

	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions([]action.SealedEnvelope{selp}...).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)

	require.NoError(t, sf.PutBlock(ctx, &blk))
	state, err = accountutil.AccountState(sf, a)
	require.NoError(t, err)
	require.Equal(t, uint64(2), state.PendingNonce())
}

func TestLoadStoreHeight(t *testing.T) {
	testTriePath, err := testutil.PathOfTempFile(_triePath)
	require.NoError(t, err)
	defer testutil.CleanupPath(testTriePath)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	statefactory, err := NewFactory(cfg, DefaultTrieOption(), SkipBlockValidationOption())
	require.NoError(t, err)

	testLoadStoreHeight(statefactory, t)
}

func TestLoadStoreHeightInMem(t *testing.T) {
	testTriePath, err := testutil.PathOfTempFile(_triePath)
	require.NoError(t, err)
	defer testutil.CleanupPath(testTriePath)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	statefactory, err := NewFactory(cfg, InMemTrieOption(), SkipBlockValidationOption())
	require.NoError(t, err)
	testLoadStoreHeight(statefactory, t)
}

func TestSDBLoadStoreHeight(t *testing.T) {
	testDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(t, err)
	defer testutil.CleanupPath(testDBPath)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testDBPath
	db, err := NewStateDB(cfg, CachedStateDBOption(), SkipBlockValidationStateDBOption())
	require.NoError(t, err)

	testLoadStoreHeight(db, t)
}

func TestSDBLoadStoreHeightInMem(t *testing.T) {
	testDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(t, err)
	defer testutil.CleanupPath(testDBPath)
	cfg := config.Default
	cfg.Chain.TrieDBPath = testDBPath
	db, err := NewStateDB(cfg, InMemStateDBOption(), SkipBlockValidationStateDBOption())
	require.NoError(t, err)

	testLoadStoreHeight(db, t)
}

func testLoadStoreHeight(sf Factory, t *testing.T) {
	require := require.New(t)
	ctx := genesis.WithGenesisContext(context.Background(), genesis.Default)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
	}()
	height, err := sf.Height()
	require.NoError(err)
	require.Equal(uint64(0), height)
	lastBlockHash := hash.ZeroHash256
	for i := uint64(1); i <= 10; i++ {
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight: i,
			Producer:    identityset.Address(27),
			GasLimit:    testutil.TestGasLimit,
		})
		blk, err := block.NewTestingBuilder().
			SetHeight(i).
			SetPrevBlockHash(lastBlockHash).
			SetTimeStamp(testutil.TimestampNow()).
			AddActions([]action.SealedEnvelope{}...).
			SignAndBuild(identityset.PrivateKey(27))
		require.NoError(err)
		require.NoError(sf.PutBlock(ctx, &blk))

		height, err = sf.Height()
		require.NoError(err)
		require.Equal(uint64(i), height)
	}
}

func TestRunActions(t *testing.T) {
	require := require.New(t)
	testTriePath, err := testutil.PathOfTempFile(_triePath)
	require.NoError(err)

	cfg := config.Default
	cfg.DB.DbPath = testTriePath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "200"
	registry := protocol.NewRegistry()
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewBoltDB(cfg.DB)), RegistryOption(registry), SkipBlockValidationOption())
	require.NoError(err)

	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	ctx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
		testutil.CleanupPath(testTriePath)
	}()
	testCommit(sf, t)
}

func TestSTXRunActions(t *testing.T) {
	require := require.New(t)
	testStateDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(err)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testStateDBPath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "200"
	sdb, err := NewStateDB(cfg, CachedStateDBOption(), SkipBlockValidationStateDBOption())
	require.NoError(err)

	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	ctx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)
	require.NoError(sdb.Start(ctx))
	defer func() {
		require.NoError(sdb.Stop(ctx))
		testutil.CleanupPath(testStateDBPath)
	}()
	testCommit(sdb, t)
}

func testCommit(factory Factory, t *testing.T) {
	require := require.New(t)
	a := identityset.Address(28).String()
	priKeyA := identityset.PrivateKey(28)
	b := identityset.Address(29).String()
	priKeyB := identityset.PrivateKey(29)

	tx1, err := action.NewTransfer(uint64(1), big.NewInt(10), b, nil, uint64(100000), big.NewInt(0))
	require.NoError(err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).SetAction(tx1).Build()
	selp1, err := action.Sign(elp, priKeyA)
	require.NoError(err)

	tx2, err := action.NewTransfer(uint64(1), big.NewInt(20), a, nil, uint64(100000), big.NewInt(0))
	require.NoError(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).SetAction(tx2).Build()
	selp2, err := action.Sign(elp, priKeyB)
	require.NoError(err)

	blkHash, err := selp1.Hash()
	require.NoError(err)

	gasLimit := uint64(1000000)
	ctx := protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			BlockHeight: 1,
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		})
	ctx = genesis.WithGenesisContext(
		protocol.WithBlockchainCtx(
			ctx,
			protocol.BlockchainCtx{
				Tip: protocol.TipInfo{
					Height: 0,
					Hash:   blkHash,
				},
			}),
		genesis.Default,
	)

	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(blkHash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp1, selp2).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)

	require.NoError(factory.PutBlock(ctx, &blk))
}

func TestPickAndRunActions(t *testing.T) {
	require := require.New(t)
	testTriePath, err := testutil.PathOfTempFile(_triePath)
	require.NoError(err)

	cfg := config.Default
	cfg.DB.DbPath = testTriePath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "200"
	registry := protocol.NewRegistry()
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewBoltDB(cfg.DB)), RegistryOption(registry))
	require.NoError(err)

	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	ctx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
		testutil.CleanupPath(testTriePath)
	}()
	testNewBlockBuilder(sf, t)
}

func TestSTXPickAndRunActions(t *testing.T) {
	require := require.New(t)
	testStateDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(err)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testStateDBPath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "200"
	registry := protocol.NewRegistry()
	sdb, err := NewStateDB(cfg, CachedStateDBOption(), RegistryStateDBOption(registry))
	require.NoError(err)

	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	ctx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)
	require.NoError(sdb.Start(ctx))
	defer func() {
		require.NoError(sdb.Stop(ctx))
		testutil.CleanupPath(testStateDBPath)
	}()
	testNewBlockBuilder(sdb, t)
}

func testNewBlockBuilder(factory Factory, t *testing.T) {
	require := require.New(t)
	a := identityset.Address(28).String()
	priKeyA := identityset.PrivateKey(28)
	b := identityset.Address(29).String()
	priKeyB := identityset.PrivateKey(29)

	accMap := make(map[string][]action.SealedEnvelope)
	tx1, err := action.NewTransfer(uint64(1), big.NewInt(10), b, nil, uint64(100000), big.NewInt(0))
	require.NoError(err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).SetAction(tx1).Build()
	selp1, err := action.Sign(elp, priKeyA)
	require.NoError(err)
	accMap[identityset.Address(28).String()] = []action.SealedEnvelope{selp1}

	addr0 := identityset.Address(27).String()
	tsf0, err := action.SignedTransfer(addr0, identityset.PrivateKey(0), 1, big.NewInt(90000000), nil, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(err)
	accMap[identityset.Address(0).String()] = []action.SealedEnvelope{tsf0}

	tx2, err := action.NewTransfer(uint64(1), big.NewInt(20), a, nil, uint64(100000), big.NewInt(0))
	require.NoError(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).SetAction(tx2).Build()
	selp2, err := action.Sign(elp, priKeyB)
	require.NoError(err)
	accMap[identityset.Address(29).String()] = []action.SealedEnvelope{selp2}
	ctrl := gomock.NewController(t)
	ap := mock_actpool.NewMockActPool(ctrl)
	ap.EXPECT().PendingActionMap().Return(accMap).Times(1)
	gasLimit := uint64(1000000)
	ctx := protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			BlockHeight: 1,
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		})
	ctx = protocol.WithBlockchainCtx(
		genesis.WithGenesisContext(ctx, genesis.Default),
		protocol.BlockchainCtx{},
	)
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
	blkBuilder, err := factory.NewBlockBuilder(ctx, ap, nil)
	require.NoError(err)
	require.NotNil(blkBuilder)
	blk, err := blkBuilder.SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)
	require.NoError(factory.PutBlock(ctx, &blk))
}

func TestSimulateExecution(t *testing.T) {
	require := require.New(t)
	testTriePath, err := testutil.PathOfTempFile(_triePath)
	require.NoError(err)

	cfg := config.Default
	cfg.DB.DbPath = testTriePath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "200"
	registry := protocol.NewRegistry()
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewBoltDB(cfg.DB)), RegistryOption(registry))
	require.NoError(err)

	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	ctx := protocol.WithBlockchainCtx(
		protocol.WithBlockCtx(
			genesis.WithGenesisContext(context.Background(), cfg.Genesis),
			protocol.BlockCtx{},
		),
		protocol.BlockchainCtx{},
	)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
		testutil.CleanupPath(testTriePath)
	}()
	testSimulateExecution(ctx, sf, t)
}

func TestSTXSimulateExecution(t *testing.T) {
	require := require.New(t)
	testStateDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(err)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testStateDBPath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "200"
	registry := protocol.NewRegistry()
	sdb, err := NewStateDB(cfg, CachedStateDBOption(), RegistryStateDBOption(registry))
	require.NoError(err)

	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	ctx := protocol.WithBlockchainCtx(
		protocol.WithBlockCtx(
			genesis.WithGenesisContext(context.Background(), cfg.Genesis),
			protocol.BlockCtx{},
		),
		protocol.BlockchainCtx{},
	)
	require.NoError(sdb.Start(ctx))
	defer func() {
		require.NoError(sdb.Stop(ctx))
		testutil.CleanupPath(testStateDBPath)
	}()
	testSimulateExecution(ctx, sdb, t)
}

func testSimulateExecution(ctx context.Context, sf Factory, t *testing.T) {
	require := require.New(t)

	data, _ := hex.DecodeString("608060405234801561001057600080fd5b5060df8061001f6000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a7230582002faabbefbbda99b20217cf33cb8ab8100caf1542bf1f48117d72e2c59139aea0029")
	ex, err := action.NewExecution(action.EmptyAddress, 1, big.NewInt(0), uint64(100000), big.NewInt(0), data)
	require.NoError(err)
	addr, err := address.FromString(address.ZeroAddress)
	require.NoError(err)

	_, _, err = sf.SimulateExecution(ctx, addr, ex, func(uint64) (hash.Hash256, error) {
		return hash.ZeroHash256, nil
	})
	require.NoError(err)
}

func TestCachedBatch(t *testing.T) {
	sf, err := NewFactory(config.Default, InMemTrieOption())
	require.NoError(t, err)
	ctx := genesis.WithGenesisContext(
		protocol.WithRegistry(context.Background(), protocol.NewRegistry()),
		genesis.Default,
	)
	require.NoError(t, sf.Start(ctx))
	ws, err := sf.(workingSetCreator).newWorkingSet(ctx, 1)
	require.NoError(t, err)
	testCachedBatch(ws, t)
}

func TestSTXCachedBatch(t *testing.T) {
	sdb, err := NewStateDB(config.Default, InMemStateDBOption())
	require.NoError(t, err)
	ctx := genesis.WithGenesisContext(
		protocol.WithRegistry(context.Background(), protocol.NewRegistry()),
		genesis.Default,
	)
	require.NoError(t, sdb.Start(ctx))
	ws, err := sdb.(workingSetCreator).newWorkingSet(ctx, 1)
	require.NoError(t, err)
	testCachedBatch(ws, t)
}

func testCachedBatch(ws *workingSet, t *testing.T) {
	require := require.New(t)

	// test PutState()
	hashA := hash.BytesToHash160(identityset.Address(28).Bytes())
	accountA := state.NewEmptyAccount()
	require.NoError(accountA.AddBalance(big.NewInt(70)))
	_, err := ws.PutState(accountA, protocol.LegacyKeyOption(hashA))
	require.NoError(err)

	// test State()
	testAccount := state.NewEmptyAccount()
	_, err = ws.State(testAccount, protocol.LegacyKeyOption(hashA))
	require.NoError(err)
	require.Equal(accountA, testAccount)

	// test DelState()
	_, err = ws.DelState(protocol.LegacyKeyOption(hashA))
	require.NoError(err)

	// can't state account "alfa" anymore
	_, err = ws.State(testAccount, protocol.LegacyKeyOption(hashA))
	require.Error(err)
}

func TestStateDBPatch(t *testing.T) {
	require := require.New(t)
	n1 := "n1"
	n2 := "n2"
	a1 := "a1"
	a2 := "a2"
	b11 := "bb11"
	b12 := "bb12"
	b21 := "bb21"
	b22 := "bb22"
	ha1, err := hex.DecodeString(a1)
	require.NoError(err)
	ha2, err := hex.DecodeString(a2)
	require.NoError(err)
	hb11, err := hex.DecodeString(b11)
	require.NoError(err)
	hb12, err := hex.DecodeString(b12)
	require.NoError(err)
	hb21, err := hex.DecodeString(b21)
	require.NoError(err)
	hb22, err := hex.DecodeString(b22)
	require.NoError(err)
	patchTest := [][]string{
		{"1", "PUT", n1, a1, b11},
		{"1", "PUT", n1, a2, b12},
		{"1", "PUT", n2, a1, b21},
		{"2", "DELETE", n1, a1},
		{"2", "PUT", n2, a2, b22},
	}
	patchFile, err := testutil.PathOfTempFile(_triePath + ".patch")
	require.NoError(err)
	f, err := os.Create(patchFile)
	require.NoError(err)
	require.NoError(csv.NewWriter(f).WriteAll(patchTest))
	require.NoError(f.Close())

	testDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(err)
	cfg := config.Default
	cfg.DB.DbPath = testDBPath
	cfg.Chain.TrieDBPatchFile = patchFile
	trieDB := db.NewBoltDB(cfg.DB)
	sdb, err := NewStateDB(cfg, PrecreatedStateDBOption(trieDB), DefaultPatchOption(), SkipBlockValidationStateDBOption())
	require.NoError(err)
	gasLimit := testutil.TestGasLimit

	ctx := protocol.WithBlockCtx(
		context.Background(),
		protocol.BlockCtx{
			BlockHeight: 0,
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		},
	)
	ctx = genesis.WithGenesisContext(ctx, genesis.Default)

	require.NoError(sdb.Start(ctx))
	defer func() {
		require.NoError(sdb.Stop(ctx))
		testutil.CleanupPath(patchFile)
		testutil.CleanupPath(testDBPath)
	}()
	ctx = protocol.WithBlockchainCtx(protocol.WithBlockCtx(ctx,
		protocol.BlockCtx{
			BlockHeight: 1,
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		}), protocol.BlockchainCtx{
		ChainID: 1,
	})
	_, err = trieDB.Get(n1, ha1)
	require.EqualError(errors.Cause(err), db.ErrNotExist.Error())
	_, err = trieDB.Get(n1, ha2)
	require.EqualError(errors.Cause(err), db.ErrNotExist.Error())
	_, err = trieDB.Get(n2, ha1)
	require.EqualError(errors.Cause(err), db.ErrNotExist.Error())
	_, err = trieDB.Get(n2, ha2)
	require.EqualError(errors.Cause(err), db.ErrNotExist.Error())
	blk1, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)
	require.NoError(sdb.PutBlock(ctx, &blk1))
	v11, err := trieDB.Get(n1, ha1)
	require.NoError(err)
	require.Equal(v11, hb11)
	v12, err := trieDB.Get(n1, ha2)
	require.NoError(err)
	require.Equal(v12, hb12)
	v21, err := trieDB.Get(n2, ha1)
	require.NoError(err)
	require.Equal(v21, hb21)
	_, err = trieDB.Get(n2, ha2)
	require.EqualError(errors.Cause(err), db.ErrNotExist.Error())
	ctx = protocol.WithBlockchainCtx(protocol.WithBlockCtx(ctx,
		protocol.BlockCtx{
			BlockHeight: 2,
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		}), protocol.BlockchainCtx{
		ChainID: 1,
	})
	blk2, err := block.NewTestingBuilder().
		SetHeight(2).
		SetPrevBlockHash(blk1.HashBlock()).
		SetTimeStamp(testutil.TimestampNow()).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)
	require.NoError(sdb.PutBlock(ctx, &blk2))
	v11, err = trieDB.Get(n1, ha1)
	require.EqualError(errors.Cause(err), db.ErrNotExist.Error())
	v12, err = trieDB.Get(n1, ha2)
	require.NoError(err)
	require.Equal(v12, hb12)
	v21, err = trieDB.Get(n2, ha1)
	require.NoError(err)
	require.Equal(v21, hb21)
	v22, err := trieDB.Get(n2, ha2)
	require.NoError(err)
	require.Equal(v22, hb22)

	require.NoError(os.RemoveAll(patchFile))
}

func TestDeleteAndPutSameKey(t *testing.T) {
	testDeleteAndPutSameKey := func(t *testing.T, ws *workingSet) {
		key := hash.Hash160b([]byte("test"))
		acc := state.NewEmptyAccount()
		require.NoError(t, acc.SetNonce(1))
		_, err := ws.PutState(acc, protocol.LegacyKeyOption(key))
		require.NoError(t, err)
		_, err = ws.DelState(protocol.LegacyKeyOption(key))
		require.NoError(t, err)
		_, err = ws.State(&acc, protocol.LegacyKeyOption(key))
		require.Equal(t, state.ErrStateNotExist, errors.Cause(err))
		_, err = ws.State(&acc, protocol.LegacyKeyOption(hash.Hash160b([]byte("other"))))
		require.Equal(t, state.ErrStateNotExist, errors.Cause(err))
	}
	ctx := genesis.WithGenesisContext(
		protocol.WithRegistry(context.Background(), protocol.NewRegistry()),
		genesis.Default,
	)
	t.Run("workingSet", func(t *testing.T) {
		sf, err := NewFactory(config.Default, InMemTrieOption())
		require.NoError(t, err)
		ws, err := sf.(workingSetCreator).newWorkingSet(ctx, 0)
		require.NoError(t, err)
		testDeleteAndPutSameKey(t, ws)
	})
	t.Run("stateTx", func(t *testing.T) {
		sdb, err := NewStateDB(config.Default, InMemStateDBOption())
		ws, err := sdb.(workingSetCreator).newWorkingSet(ctx, 0)
		require.NoError(t, err)
		testDeleteAndPutSameKey(t, ws)
	})
}

func BenchmarkInMemRunAction(b *testing.B) {
	cfg := config.Default
	sf, err := NewFactory(cfg, InMemTrieOption(), SkipBlockValidationOption())
	if err != nil {
		b.Fatal(err)
	}
	benchRunAction(sf, b)
}

func BenchmarkDBRunAction(b *testing.B) {
	tp := filepath.Join(os.TempDir(), _triePath)
	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}

	cfg := config.Default
	cfg.DB.DbPath = tp
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewBoltDB(cfg.DB)), SkipBlockValidationOption())
	if err != nil {
		b.Fatal(err)
	}
	benchRunAction(sf, b)

	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}
}

func BenchmarkSDBInMemRunAction(b *testing.B) {
	cfg := config.Default
	sdb, err := NewStateDB(cfg, InMemStateDBOption(), SkipBlockValidationStateDBOption())
	if err != nil {
		b.Fatal(err)
	}
	benchRunAction(sdb, b)
}

func BenchmarkSDBRunAction(b *testing.B) {
	tp := filepath.Join(os.TempDir(), _stateDBPath)
	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}
	cfg := config.Default
	cfg.Chain.TrieDBPath = tp
	sdb, err := NewStateDB(cfg, DefaultStateDBOption(), SkipBlockValidationStateDBOption())
	if err != nil {
		b.Fatal(err)
	}
	benchRunAction(sdb, b)
	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}
}

func BenchmarkCachedSDBRunAction(b *testing.B) {
	tp := filepath.Join(os.TempDir(), _stateDBPath)
	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}
	cfg := config.Default
	cfg.Chain.TrieDBPath = tp
	sdb, err := NewStateDB(cfg, CachedStateDBOption(), SkipBlockValidationStateDBOption())
	if err != nil {
		b.Fatal(err)
	}
	benchRunAction(sdb, b)
	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}
}

func benchRunAction(sf Factory, b *testing.B) {
	// set up
	accounts := []string{
		identityset.Address(28).String(),
		identityset.Address(29).String(),
		identityset.Address(30).String(),
		identityset.Address(31).String(),
		identityset.Address(32).String(),
		identityset.Address(33).String(),
	}
	pubKeys := []crypto.PublicKey{
		identityset.PrivateKey(28).PublicKey(),
		identityset.PrivateKey(29).PublicKey(),
		identityset.PrivateKey(30).PublicKey(),
		identityset.PrivateKey(31).PublicKey(),
		identityset.PrivateKey(32).PublicKey(),
		identityset.PrivateKey(33).PublicKey(),
	}
	nonces := make([]uint64, len(accounts))
	ge := genesis.Default
	prevHash := ge.Hash()
	for _, acc := range accounts {
		ge.InitBalanceMap[acc] = big.NewInt(int64(b.N * 100)).String()
	}
	acc := account.NewProtocol(rewarding.DepositGas)
	if err := sf.Register(acc); err != nil {
		b.Fatal(err)
	}
	ctx := genesis.WithGenesisContext(context.Background(), ge)

	if err := sf.Start(ctx); err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := sf.Stop(ctx); err != nil {
			b.Fatal(err)
		}
	}()

	gasLimit := testutil.TestGasLimit * 100000

	for n := 1; n < b.N; n++ {
		// put 500 actions together to run
		b.StopTimer()
		total := 500
		acts := make([]action.SealedEnvelope, 0, total)
		for numActs := 0; numActs < total; numActs++ {
			senderIdx := rand.Int() % len(accounts)

			var chainIDBytes [4]byte
			enc.MachineEndian.PutUint32(chainIDBytes[:], 1)
			payload := []byte(randStringRunes(20))
			receiverAddr, err := address.FromBytes(payload)
			if err != nil {
				b.Fatal(err)
			}
			receiver := receiverAddr.String()
			nonces[senderIdx] += nonces[senderIdx]
			tx, err := action.NewTransfer(nonces[senderIdx], big.NewInt(1), receiver, nil, uint64(0), big.NewInt(0))
			if err != nil {
				b.Fatal(err)
			}
			bd := &action.EnvelopeBuilder{}
			elp := bd.SetNonce(nonces[senderIdx]).SetAction(tx).Build()
			selp := action.FakeSeal(elp, pubKeys[senderIdx])
			acts = append(acts, selp)
		}
		b.StartTimer()
		zctx := protocol.WithBlockCtx(context.Background(),
			protocol.BlockCtx{
				BlockHeight: uint64(n),
				Producer:    identityset.Address(27),
				GasLimit:    gasLimit,
			})
		zctx = genesis.WithGenesisContext(zctx, genesis.Default)

		blk, err := block.NewTestingBuilder().
			SetHeight(uint64(n)).
			SetPrevBlockHash(prevHash).
			SetTimeStamp(testutil.TimestampNow()).
			AddActions(acts...).
			SignAndBuild(identityset.PrivateKey(27))
		if err != nil {
			b.Fatal(err)
		}

		if err := sf.PutBlock(zctx, &blk); err != nil {
			b.Fatal(err)
		}
		prevHash = blk.HashBlock()
	}
}

func BenchmarkSDBState(b *testing.B) {
	tp := filepath.Join(os.TempDir(), _stateDBPath)
	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}
	cfg := config.Default
	cfg.Chain.TrieDBPath = tp
	sdb, err := NewStateDB(cfg, DefaultStateDBOption(), SkipBlockValidationStateDBOption())
	if err != nil {
		b.Fatal(err)
	}
	benchState(sdb, b)
	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}
}

func BenchmarkCachedSDBState(b *testing.B) {
	tp := filepath.Join(os.TempDir(), _stateDBPath)
	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}
	cfg := config.Default
	cfg.Chain.TrieDBPath = tp
	sdb, err := NewStateDB(cfg, CachedStateDBOption(), SkipBlockValidationStateDBOption())
	if err != nil {
		b.Fatal(err)
	}
	benchState(sdb, b)
	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}
}

func benchState(sf Factory, b *testing.B) {
	b.StopTimer()
	// set up
	accounts := []string{
		identityset.Address(28).String(),
		identityset.Address(29).String(),
		identityset.Address(30).String(),
		identityset.Address(31).String(),
		identityset.Address(32).String(),
		identityset.Address(33).String(),
	}
	pubKeys := []crypto.PublicKey{
		identityset.PrivateKey(28).PublicKey(),
		identityset.PrivateKey(29).PublicKey(),
		identityset.PrivateKey(30).PublicKey(),
		identityset.PrivateKey(31).PublicKey(),
		identityset.PrivateKey(32).PublicKey(),
		identityset.PrivateKey(33).PublicKey(),
	}
	nonces := make([]uint64, len(accounts))
	ge := genesis.Default
	prevHash := ge.Hash()
	for _, acc := range accounts {
		ge.InitBalanceMap[acc] = big.NewInt(int64(1000)).String()
	}
	acc := account.NewProtocol(rewarding.DepositGas)
	if err := sf.Register(acc); err != nil {
		b.Fatal(err)
	}
	ctx := genesis.WithGenesisContext(context.Background(), ge)

	if err := sf.Start(ctx); err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := sf.Stop(ctx); err != nil {
			b.Fatal(err)
		}
	}()

	gasLimit := testutil.TestGasLimit * 100000

	total := 500
	acts := make([]action.SealedEnvelope, 0, total)
	for numActs := 0; numActs < total; numActs++ {
		senderIdx := rand.Int() % len(accounts)

		var chainIDBytes [4]byte
		enc.MachineEndian.PutUint32(chainIDBytes[:], 1)
		payload := []byte(randStringRunes(20))
		receiverAddr, err := address.FromBytes(payload)
		if err != nil {
			b.Fatal(err)
		}
		receiver := receiverAddr.String()
		nonces[senderIdx] += nonces[senderIdx]
		tx, err := action.NewTransfer(nonces[senderIdx], big.NewInt(1), receiver, nil, uint64(0), big.NewInt(0))
		if err != nil {
			b.Fatal(err)
		}
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetNonce(nonces[senderIdx]).SetAction(tx).Build()
		selp := action.FakeSeal(elp, pubKeys[senderIdx])
		acts = append(acts, selp)
	}
	zctx := protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			BlockHeight: uint64(1),
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		})
	zctx = genesis.WithGenesisContext(zctx, genesis.Default)

	blk, err := block.NewTestingBuilder().
		SetHeight(uint64(1)).
		SetPrevBlockHash(prevHash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(acts...).
		SignAndBuild(identityset.PrivateKey(27))
	if err != nil {
		b.Fatal(err)
	}
	if err := sf.PutBlock(zctx, &blk); err != nil {
		b.Fatal(err)
	}

	// measure state read time
	for n := 1; n < b.N; n++ {
		b.StartTimer()
		idx := rand.Int() % len(accounts)
		addr, err := address.FromString(accounts[idx])
		if err != nil {
			b.Fatal(err)
		}
		_, err = accountutil.AccountState(sf, addr)
		if err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
	}
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
