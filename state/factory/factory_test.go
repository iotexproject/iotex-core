// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"math"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/test/mock/mock_committee"
	"github.com/iotexproject/iotex-election/types"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/action/protocol/poll"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/enc"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/pkg/util/fileutil"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/v2/testutil"
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

func TestGenerateConfig(t *testing.T) {
	require := require.New(t)
	cfg := GenerateConfig(blockchain.DefaultConfig, genesis.TestDefault())
	require.Equal(27, len(cfg.Genesis.InitBalanceMap))
	require.Equal(blockchain.DefaultConfig.ChainDBPath, cfg.Chain.ChainDBPath)
}

func TestSDBSnapshot(t *testing.T) {
	require := require.New(t)
	testStateDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(err)
	defer testutil.CleanupPath(testStateDBPath)

	cfg := DefaultConfig
	cfg.Chain.TrieDBPatchFile = ""
	cfg.Chain.TrieDBPath = testStateDBPath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "5"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "7"
	registry := protocol.NewRegistry()
	db2, err := db.CreateKVStoreWithCache(db.DefaultConfig, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
	require.NoError(err)
	sdb, err := NewStateDB(cfg, db2, RegistryStateDBOption(registry))
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

func TestSDBCandidates(t *testing.T) {
	cfg := DefaultConfig
	sdb, err := NewStateDB(cfg, db.NewMemKVStore(), SkipBlockValidationStateDBOption())
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
	act := action.NewPutPollResult(1, sc)
	elp := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).SetAction(act).Build()
	selp, err := action.Sign(elp, identityset.PrivateKey(27))
	require.NoError(t, err)
	require.NotNil(t, selp)

	ctrl := gomock.NewController(t)

	committee := mock_committee.NewMockCommittee(ctrl)
	committee.EXPECT().ResultByHeight(uint64(123456)).Return(result, nil).AnyTimes()
	committee.EXPECT().HeightByTime(gomock.Any()).Return(uint64(123456), nil).AnyTimes()

	require.NoError(t, sf.Register(rolldpos.NewProtocol(36, 36, 20)))
	cfg := DefaultConfig
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
		AddActions([]*action.SealedEnvelope{selp}...).
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
					ChainID:      1,
					GetBlockHash: func(uint64) (hash.Hash256, error) { return hash.ZeroHash256, nil },
					GetBlockTime: func(uint64) (time.Time, error) { return time.Now(), nil },
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

func TestHistoryState(t *testing.T) {
	r := require.New(t)
	// using factory and enable history
	cfg := DefaultConfig

	// using stateDB and enable history
	file2, err := testutil.PathOfTempFile(_triePath)
	r.NoError(err)
	cfg.Chain.TrieDBPath = file2
	db2, err := db.CreateKVStoreWithCache(db.DefaultConfig, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
	r.NoError(err)
	sf, err := NewStateDB(cfg, db2, SkipBlockValidationStateDBOption())
	r.NoError(err)
	testHistoryState(sf, t, true, cfg.Chain.EnableArchiveMode)

	// using stateDB and disable history
	file4, err := testutil.PathOfTempFile(_triePath)
	r.NoError(err)
	cfg.Chain.TrieDBPath = file4
	db2, err = db.CreateKVStoreWithCache(db.DefaultConfig, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
	r.NoError(err)
	sf, err = NewStateDB(cfg, db2, SkipBlockValidationStateDBOption())
	r.NoError(err)
	testHistoryState(sf, t, true, cfg.Chain.EnableArchiveMode)
	defer func() {
		testutil.CleanupPath(file2)
		testutil.CleanupPath(file4)
	}()
}

func TestSDBTwoBlocksSamePrevHash(t *testing.T) {
	require := require.New(t)
	testStateDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(err)
	defer testutil.CleanupPath(testStateDBPath)

	cfg := DefaultConfig
	cfg.Chain.TrieDBPath = testStateDBPath
	db1, err := db.CreateKVStoreWithCache(db.DefaultConfig, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
	require.NoError(err)
	sdb, err := NewStateDB(cfg, db1, SkipBlockValidationStateDBOption())
	require.NoError(err)

	ctx := genesis.WithGenesisContext(context.Background(), genesis.TestDefault())
	require.NoError(sdb.Start(ctx))
	defer func() {
		require.NoError(sdb.Stop(ctx))
	}()

	ctrl := gomock.NewController(t)
	ap := mock_actpool.NewMockActPool(ctrl)
	ap.EXPECT().PendingActionMap().Return(map[string][]*action.SealedEnvelope{}).Times(4)
	blk1, err := sdb.Mint(
		protocol.WithBlockchainCtx(
			protocol.WithFeatureCtx(
				protocol.WithBlockCtx(
					ctx,
					protocol.BlockCtx{
						BlockHeight: 1,
						Producer:    identityset.Address(27),
						GasLimit:    testutil.TestGasLimit,
					},
				),
			),
			protocol.BlockchainCtx{
				ChainID: 1,
			},
		),
		ap,
		identityset.PrivateKey(27),
	)
	require.NoError(err)

	blk2, err := sdb.Mint(
		protocol.WithBlockchainCtx(
			protocol.WithFeatureCtx(
				protocol.WithBlockCtx(
					ctx,
					protocol.BlockCtx{
						BlockHeight: 1,
						Producer:    identityset.Address(26),
						GasLimit:    testutil.TestGasLimit,
					},
				),
			),
			protocol.BlockchainCtx{
				ChainID: 1,
			},
		),
		ap,
		identityset.PrivateKey(26),
	)
	require.NoError(err)

	blk11, err := sdb.Mint(
		protocol.WithBlockchainCtx(
			protocol.WithFeatureCtx(
				protocol.WithBlockCtx(
					ctx,
					protocol.BlockCtx{
						BlockHeight: 2,
						Producer:    identityset.Address(25),
						GasLimit:    testutil.TestGasLimit,
					},
				),
			),
			protocol.BlockchainCtx{
				ChainID: 1,
				Tip: protocol.TipInfo{
					Height: 1,
					Hash:   blk1.HashBlock(),
				},
			},
		),
		ap,
		identityset.PrivateKey(25),
	)
	require.NoError(err)
	require.NotNil(blk11)
	require.Equal(blk1.HashBlock(), blk11.PrevHash())
	blk21, err := sdb.Mint(
		protocol.WithBlockchainCtx(
			protocol.WithFeatureCtx(
				protocol.WithBlockCtx(
					ctx,
					protocol.BlockCtx{
						BlockHeight: 2,
						Producer:    identityset.Address(25),
						GasLimit:    testutil.TestGasLimit,
					},
				),
			),
			protocol.BlockchainCtx{
				ChainID: 1,
				Tip: protocol.TipInfo{
					Height: 1,
					Hash:   blk2.HashBlock(),
				},
			},
		),
		ap,
		identityset.PrivateKey(24),
	)
	require.NoError(err)
	require.NotNil(blk21)
	require.Equal(blk2.HashBlock(), blk21.PrevHash())

	sdbReal := sdb.(*stateDB)
	ws1, exist, err := sdbReal.getFromWorkingSets(ctx, blk1.HashBlock())
	require.NoError(err)
	require.True(exist)
	require.NotNil(ws1)
	ws2, exist, err := sdbReal.getFromWorkingSets(ctx, blk2.HashBlock())
	require.NoError(err)
	require.True(exist)
	require.NotNil(ws2)
	require.NotEqual(t, ws1, ws2)
	ws11, exist, err := sdbReal.getFromWorkingSets(ctx, blk11.HashBlock())
	require.NoError(err)
	require.True(exist)
	require.NotNil(ws11)
	ws21, exist, err := sdbReal.getFromWorkingSets(ctx, blk21.HashBlock())
	require.NoError(err)
	require.True(exist)
	require.NotNil(ws21)
}

func TestSDBState(t *testing.T) {
	testDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(t, err)
	defer testutil.CleanupPath(testDBPath)

	cfg := DefaultConfig
	cfg.Chain.TrieDBPath = testDBPath
	db1, err := db.CreateKVStoreWithCache(db.DefaultConfig, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
	require.NoError(t, err)
	sdb, err := NewStateDB(cfg, db1, SkipBlockValidationStateDBOption())
	require.NoError(t, err)
	testState(sdb, t)
}

func testState(sf Factory, t *testing.T) {
	// Create a dummy iotex address
	a := identityset.Address(28)
	priKeyA := identityset.PrivateKey(28)
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, sf.Register(acc))
	ge := genesis.TestDefault()
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

	tsf := action.NewTransfer(big.NewInt(10), identityset.Address(31).String(), nil)
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
	ctx = protocol.WithFeatureCtx(ctx)
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions([]*action.SealedEnvelope{selp}...).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)
	require.NoError(t, sf.PutBlock(ctx, &blk))

	//test AccountState() & State()
	testAccount := &state.Account{}
	accountA, err := accountutil.AccountState(ctx, sf, a)
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
	ge := genesis.TestDefault()
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
	accountA, err := accountutil.AccountState(ctx, sf, a)
	require.NoError(t, err)
	accountB, err := accountutil.AccountState(ctx, sf, b)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(100), accountA.Balance)
	require.Equal(t, big.NewInt(0), accountB.Balance)
	tsf := action.NewTransfer(big.NewInt(10), b.String(), nil)
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
	ctx = protocol.WithFeatureCtx(ctx)
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions([]*action.SealedEnvelope{selp}...).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)
	require.NoError(t, sf.PutBlock(ctx, &blk))

	// check latest balance
	accountA, err = accountutil.AccountState(ctx, sf, a)
	require.NoError(t, err)
	accountB, err = accountutil.AccountState(ctx, sf, b)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(90), accountA.Balance)
	require.Equal(t, big.NewInt(10), accountB.Balance)

	// check archive data
	if statetx {
		// statetx not support archive mode yet
		_, err = sf.WorkingSetAtHeight(ctx, 0)
		require.NoError(t, err)
	} else {
		_, err = sf.WorkingSetAtHeight(ctx, 10)
		if !archive {
			require.Equal(t, ErrNoArchiveData, errors.Cause(err))
		} else {
			require.Contains(t, err.Error(), "query height 10 is higher than tip height 1")
		}
		sr, err := sf.WorkingSetAtHeight(ctx, 0)
		if !archive {
			require.Equal(t, ErrNoArchiveData, errors.Cause(err))
		} else {
			require.NoError(t, err)
			accountA, err = accountutil.AccountState(ctx, sr, a)
			require.NoError(t, err)
			accountB, err = accountutil.AccountState(ctx, sr, b)
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
	ge := genesis.TestDefault()
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
	tsf := action.NewTransfer(big.NewInt(10), b, nil)
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
	ctx = protocol.WithFeatureCtx(ctx)
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions([]*action.SealedEnvelope{selp}...).
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
	// two accounts
	require.LessOrEqual(t, 2, iter.Size())
	accounts := make([]*state.Account, 0)
	for i := 0; i < iter.Size(); i++ {
		c := &state.Account{}
		_, err = iter.Next(c)
		if err != nil {
			continue
		}
		accounts = append(accounts, c)
	}
	require.Equal(t, uint64(200), accounts[0].Balance.Uint64()+accounts[1].Balance.Uint64())

	// case III: check without cond,with AccountKVNamespace namespace,key not exists
	namespaceOpt := protocol.NamespaceOption(AccountKVNamespace)
	height, iter, err = sf.States(namespaceOpt)
	require.NoError(t, err)
	require.Equal(t, uint64(1), height)
	// two accounts
	require.LessOrEqual(t, 2, iter.Size())
	accounts = make([]*state.Account, 0)
	for i := 0; i < iter.Size(); i++ {
		c := &state.Account{}
		_, err = iter.Next(c)
		if err != nil {
			continue
		}
		accounts = append(accounts, c)
	}
	require.Equal(t, uint64(200), accounts[0].Balance.Uint64()+accounts[1].Balance.Uint64())

	// case IV: check without cond,with AccountKVNamespace namespace
	namespaceOpt = protocol.NamespaceOption(AccountKVNamespace)
	height, iter, err = sf.States(namespaceOpt)
	require.NoError(t, err)
	require.Equal(t, uint64(1), height)
	// two accounts
	require.LessOrEqual(t, 2, iter.Size())
	accounts = make([]*state.Account, 0)
	for i := 0; i < iter.Size(); i++ {
		c := &state.Account{}
		_, err = iter.Next(c)
		if err != nil {
			continue
		}
		accounts = append(accounts, c)
	}
	require.Equal(t, uint64(200), accounts[0].Balance.Uint64()+accounts[1].Balance.Uint64())

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
		c := &state.Account{}
		_, err = iter.Next(c)
		require.NoError(t, err)
		accounts = append(accounts, c)
	}
	require.Equal(t, uint64(90), accounts[0].Balance.Uint64())
}

func TestSDBNonce(t *testing.T) {
	testDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(t, err)
	defer testutil.CleanupPath(testDBPath)

	cfg := DefaultConfig
	cfg.Chain.TrieDBPath = testDBPath

	db2, err := db.CreateKVStoreWithCache(db.DefaultConfig, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
	require.NoError(t, err)

	reg := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	err = acc.Register(reg)
	require.NoError(t, err)

	sdb, err := NewStateDB(cfg, db2, SkipBlockValidationStateDBOption(), RegistryStateDBOption(reg))
	require.NoError(t, err)

	testNonce(protocol.WithRegistry(context.Background(), reg), sdb, t)
}

func testNonce(ctx context.Context, sf Factory, t *testing.T) {
	// Create two dummy iotex address
	a := identityset.Address(28)
	priKeyA := identityset.PrivateKey(28)
	b := identityset.Address(29).String()
	ge := genesis.TestDefault()
	ge.InitBalanceMap[a.String()] = "100"
	gasLimit := uint64(1000000)
	ctx = protocol.WithBlockCtx(ctx,
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

	tx := action.NewTransfer(big.NewInt(2), b, nil)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tx).SetGasLimit(20000).Build()
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
	state, err := accountutil.AccountState(ctx, sf, a)
	require.NoError(t, err)
	require.Equal(t, uint64(1), state.PendingNonce())

	tx = action.NewTransfer(big.NewInt(2), b, nil)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx).SetNonce(1).SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, priKeyA)
	require.NoError(t, err)

	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions([]*action.SealedEnvelope{selp}...).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)

	require.NoError(t, sf.PutBlock(ctx, &blk))
	state, err = accountutil.AccountState(ctx, sf, a)
	require.NoError(t, err)
	require.Equal(t, uint64(2), state.PendingNonce())
}

func TestSDBLoadStoreHeight(t *testing.T) {
	testDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(t, err)
	defer testutil.CleanupPath(testDBPath)

	cfg := DefaultConfig
	cfg.Chain.TrieDBPath = testDBPath

	db2, err := db.CreateKVStoreWithCache(db.DefaultConfig, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
	require.NoError(t, err)
	db, err := NewStateDB(cfg, db2, SkipBlockValidationStateDBOption())
	require.NoError(t, err)

	testLoadStoreHeight(db, t)
}

func TestSDBLoadStoreHeightInMem(t *testing.T) {
	testDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(t, err)
	defer testutil.CleanupPath(testDBPath)
	cfg := DefaultConfig
	cfg.Chain.TrieDBPath = testDBPath
	db, err := NewStateDB(cfg, db.NewMemKVStore(), SkipBlockValidationStateDBOption())
	require.NoError(t, err)

	testLoadStoreHeight(db, t)
}

func testLoadStoreHeight(sf Factory, t *testing.T) {
	require := require.New(t)
	ctx := genesis.WithGenesisContext(context.Background(), genesis.TestDefault())
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
		ctx = protocol.WithFeatureCtx(ctx)
		blk, err := block.NewTestingBuilder().
			SetHeight(i).
			SetPrevBlockHash(lastBlockHash).
			SetTimeStamp(testutil.TimestampNow()).
			AddActions([]*action.SealedEnvelope{}...).
			SignAndBuild(identityset.PrivateKey(27))
		require.NoError(err)
		require.NoError(sf.PutBlock(ctx, &blk))

		height, err = sf.Height()
		require.NoError(err)
		require.Equal(uint64(i), height)
	}
}

func TestSTXRunActions(t *testing.T) {
	require := require.New(t)
	testStateDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(err)

	cfg := DefaultConfig
	cfg.Chain.TrieDBPath = testStateDBPath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "200"

	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))

	db2, err := db.CreateKVStoreWithCache(db.DefaultConfig, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
	require.NoError(err)
	sdb, err := NewStateDB(cfg, db2, SkipBlockValidationStateDBOption(), RegistryStateDBOption(registry))
	require.NoError(err)

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

	tx1 := action.NewTransfer(big.NewInt(10), b, nil)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).SetAction(tx1).Build()
	selp1, err := action.Sign(elp, priKeyA)
	require.NoError(err)

	tx2 := action.NewTransfer(big.NewInt(20), a, nil)
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
		genesis.TestDefault(),
	)
	ctx = protocol.WithFeatureCtx(ctx)
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(blkHash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp1, selp2).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)

	require.NoError(factory.PutBlock(ctx, &blk))
}

func TestSTXPickAndRunActions(t *testing.T) {
	require := require.New(t)
	testStateDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(err)

	cfg := DefaultConfig
	cfg.Chain.TrieDBPath = testStateDBPath
	cfg.Genesis.InitBalanceMap[identityset.Address(0).String()] = "1000000000"
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "200"
	registry := protocol.NewRegistry()
	db2, err := db.CreateKVStoreWithCache(db.DefaultConfig, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
	require.NoError(err)
	sdb, err := NewStateDB(cfg, db2, RegistryStateDBOption(registry))
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

	accMap := make(map[string][]*action.SealedEnvelope)
	tx1 := action.NewTransfer(big.NewInt(10), b, nil)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).SetAction(tx1).Build()
	selp1, err := action.Sign(elp, priKeyA)
	require.NoError(err)
	accMap[identityset.Address(28).String()] = []*action.SealedEnvelope{selp1}

	addr0 := identityset.Address(27).String()
	tsf0, err := action.SignedTransfer(addr0, identityset.PrivateKey(0), 1, big.NewInt(90000000), nil, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(err)
	accMap[identityset.Address(0).String()] = []*action.SealedEnvelope{tsf0}

	tx2 := action.NewTransfer(big.NewInt(20), a, nil)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).SetAction(tx2).Build()
	selp2, err := action.Sign(elp, priKeyB)
	require.NoError(err)
	accMap[identityset.Address(29).String()] = []*action.SealedEnvelope{selp2}
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
		genesis.WithGenesisContext(ctx, genesis.TestDefault()),
		protocol.BlockchainCtx{},
	)
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
	blk, err := factory.Mint(ctx, ap, identityset.PrivateKey(27))
	require.NoError(err)
	require.NoError(factory.PutBlock(ctx, blk))
}

func TestSTXSimulateExecution(t *testing.T) {
	require := require.New(t)
	testStateDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(err)

	cfg := DefaultConfig
	cfg.Chain.TrieDBPath = testStateDBPath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "200"
	registry := protocol.NewRegistry()
	db2, err := db.CreateKVStoreWithCache(db.DefaultConfig, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
	require.NoError(err)
	sdb, err := NewStateDB(cfg, db2, RegistryStateDBOption(registry))
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
	ex := action.NewExecution(action.EmptyAddress, big.NewInt(0), data)
	elp := (&action.EnvelopeBuilder{}).SetGasLimit(100000).SetAction(ex).Build()
	addr, err := address.FromString(address.ZeroAddress)
	require.NoError(err)

	ctx = evm.WithHelperCtx(ctx, evm.HelperContext{
		GetBlockHash: func(uint64) (hash.Hash256, error) {
			return hash.ZeroHash256, nil
		},
		GetBlockTime: func(u uint64) (time.Time, error) {
			return time.Time{}, nil
		},
		DepositGasFunc: rewarding.DepositGas,
	})
	ws, err := sf.WorkingSet(ctx)
	require.NoError(err)
	_, _, err = evm.SimulateExecution(ctx, ws, addr, elp)
	require.NoError(err)
}

func TestSTXCachedBatch(t *testing.T) {
	sdb, err := NewStateDB(DefaultConfig, db.NewMemKVStore())
	require.NoError(t, err)
	ctx := genesis.WithGenesisContext(
		protocol.WithRegistry(context.Background(), protocol.NewRegistry()),
		genesis.TestDefault(),
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
	accountA, err := state.NewAccount()
	require.NoError(err)
	require.NoError(accountA.AddBalance(big.NewInt(70)))
	_, err = ws.PutState(accountA, protocol.LegacyKeyOption(hashA))
	require.NoError(err)

	// test State()
	testAccount := &state.Account{}
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
	cfg := DefaultConfig
	dbcfg := db.DefaultConfig
	dbcfg.DbPath = testDBPath
	cfg.Chain.TrieDBPatchFile = patchFile
	trieDB := db.NewBoltDB(dbcfg)
	sdb, err := NewStateDB(cfg, trieDB, DefaultPatchOption(), SkipBlockValidationStateDBOption())
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
	ctx = genesis.WithGenesisContext(ctx, genesis.TestDefault())

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
	ctx = protocol.WithFeatureCtx(ctx)
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
	_, err = trieDB.Get(n1, ha1)
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
		acc, err := state.NewAccount()
		require.NoError(t, err)
		require.NoError(t, acc.SetPendingNonce(1))
		require.NoError(t, acc.SetPendingNonce(2))
		_, err = ws.PutState(acc, protocol.LegacyKeyOption(key))
		require.NoError(t, err)
		_, err = ws.DelState(protocol.LegacyKeyOption(key))
		require.NoError(t, err)
		_, err = ws.State(acc, protocol.LegacyKeyOption(key))
		require.Equal(t, state.ErrStateNotExist, errors.Cause(err))
		_, err = ws.State(acc, protocol.LegacyKeyOption(hash.Hash160b([]byte("other"))))
		require.Equal(t, state.ErrStateNotExist, errors.Cause(err))
	}
	ctx := genesis.WithGenesisContext(
		protocol.WithRegistry(context.Background(), protocol.NewRegistry()),
		genesis.TestDefault(),
	)
	t.Run("stateTx", func(t *testing.T) {
		sdb, err := NewStateDB(DefaultConfig, db.NewMemKVStore())
		require.NoError(t, err)
		ws, err := sdb.(workingSetCreator).newWorkingSet(ctx, 0)
		require.NoError(t, err)
		testDeleteAndPutSameKey(t, ws)
	})
}

func TestMintBlocksWithCandidateUpdate(t *testing.T) {
	require := require.New(t)
	testStateDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(err)
	defer testutil.CleanupPath(testStateDBPath)
	a := identityset.Address(28)
	b := identityset.Address(29)
	priKeyA := identityset.PrivateKey(28)
	priKeyB := identityset.PrivateKey(29)

	cfg := DefaultConfig
	cfg.Chain.TrieDBPath = testStateDBPath
	cfg.Genesis.InitBalanceMap[a.String()] = unit.ConvertIotxToRau(10000000).String()
	cfg.Genesis.InitBalanceMap[b.String()] = unit.ConvertIotxToRau(5000000).String()

	registry := protocol.NewRegistry()
	require.NoError(account.NewProtocol(rewarding.DepositGas).Register(registry))
	sp, err := staking.NewProtocol(
		staking.HelperCtx{
			DepositGas: rewarding.DepositGas,
			BlockInterval: func(u uint64) time.Duration {
				return time.Second
			},
		},
		&staking.BuilderConfig{
			Staking:                  genesis.TestDefault().Staking,
			PersistStakingPatchBlock: math.MaxUint64,
		},
		nil,
		nil,
		nil,
	)
	require.NoError(err)
	require.NoError(sp.Register(registry))

	db2, err := db.CreateKVStoreWithCache(db.DefaultConfig, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
	require.NoError(err)
	sdb, err := NewStateDB(cfg, db2, SkipBlockValidationStateDBOption(), RegistryStateDBOption(registry))
	require.NoError(err)

	ctx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	require.NoError(sdb.Start(ctx))
	defer func() {
		require.NoError(sdb.Stop(ctx))
	}()

	tsf1, err := action.NewCandidateRegister("cand1", a.String(), a.String(), a.String(), unit.ConvertIotxToRau(1200000).String(), 0, false, nil)
	require.NoError(err)
	elp1 := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasLimit(20000).SetAction(tsf1).Build()
	selp1, err := action.Sign(elp1, priKeyA)
	require.NoError(err)

	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight: 1,
			Producer:    identityset.Address(27),
			GasLimit:    testutil.TestGasLimit,
		},
	)
	ctx = protocol.WithFeatureCtx(ctx)
	mockActPool := mock_actpool.NewMockActPool(gomock.NewController(t))
	mockActPool.EXPECT().PendingActionMap().Return(map[string][]*action.SealedEnvelope{
		a.String(): {selp1},
	}).Times(1)

	blk1, err := sdb.Mint(
		protocol.WithBlockchainCtx(
			ctx,
			protocol.BlockchainCtx{
				ChainID: 1,
				Tip: protocol.TipInfo{
					Height: 0,
					Hash:   hash.ZeroHash256,
				},
			},
		),
		mockActPool,
		identityset.PrivateKey(27))
	require.NoError(err)
	require.NotNil(blk1)

	ws1, exist, err := sdb.(*stateDB).getFromWorkingSets(ctx, blk1.HashBlock())
	require.NoError(err)
	require.True(exist)
	require.NotNil(ws1)

	tsf2, err := action.NewCandidateRegister("cand2", b.String(), b.String(), b.String(), unit.ConvertIotxToRau(1200000).String(), 0, false, nil)
	require.NoError(err)
	elp2 := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasLimit(20000).SetAction(tsf2).Build()
	selp2, err := action.Sign(elp2, priKeyB)
	require.NoError(err)

	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight: 1,
			Producer:    identityset.Address(26),
			GasLimit:    testutil.TestGasLimit,
		},
	)
	ctx = protocol.WithFeatureCtx(ctx)
	mockActPool.EXPECT().PendingActionMap().Return(map[string][]*action.SealedEnvelope{
		a.String(): {selp2},
	}).Times(1)

	blk2, err := sdb.Mint(
		protocol.WithBlockchainCtx(
			ctx,
			protocol.BlockchainCtx{
				ChainID: 1,
				Tip: protocol.TipInfo{
					Height: 0,
					Hash:   hash.ZeroHash256,
				},
			},
		),
		mockActPool,
		identityset.PrivateKey(26))
	require.NoError(err)
	require.NotNil(blk2)

	ws2, exist, err := sdb.(*stateDB).getFromWorkingSets(ctx, blk2.HashBlock())
	require.NoError(err)
	require.True(exist)
	require.NotNil(ws2)

	csr, err := staking.ConstructBaseView(sdb)
	require.NoError(err)
	csr1, err := staking.ConstructBaseView(ws1)
	require.NoError(err)
	csr2, err := staking.ConstructBaseView(ws2)
	require.NoError(err)
	require.NotNil(csr1.GetCandidateByName("cand1"))
	require.Nil(csr.GetCandidateByName("cand1"))
	require.Nil(csr2.GetCandidateByName("cand1"))
	require.NotNil(csr2.GetCandidateByName("cand2"))
	require.Nil(csr.GetCandidateByName("cand2"))
	require.Nil(csr1.GetCandidateByName("cand2"))

	require.NoError(sdb.PutBlock(ctx, blk1))
	csr, err = staking.ConstructBaseView(sdb)
	require.NoError(err)
	require.NotNil(csr.GetCandidateByName("cand1"))
	require.Nil(csr2.GetCandidateByName("cand1"))
	require.NotNil(csr2.GetCandidateByName("cand2"))
	require.Nil(csr.GetCandidateByName("cand2"))
}

func TestMintBlocksWithTransfers(t *testing.T) {
	require := require.New(t)
	testStateDBPath, err := testutil.PathOfTempFile(_stateDBPath)
	require.NoError(err)
	defer testutil.CleanupPath(testStateDBPath)

	cfg := DefaultConfig
	cfg.Chain.TrieDBPath = testStateDBPath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "50"
	cfg.Genesis.InitBalanceMap[identityset.Address(30).String()] = "0"

	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))

	db2, err := db.CreateKVStoreWithCache(db.DefaultConfig, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
	require.NoError(err)
	sdb, err := NewStateDB(cfg, db2, SkipBlockValidationStateDBOption(), RegistryStateDBOption(registry))
	require.NoError(err)

	ctx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)
	require.NoError(sdb.Start(ctx))
	defer func() {
		require.NoError(sdb.Stop(ctx))
	}()

	// Mint the first block with a transfer from A to B
	a := identityset.Address(28)
	b := identityset.Address(29)
	priKeyA := identityset.PrivateKey(28)

	tsf1 := action.NewTransfer(big.NewInt(20), b.String(), nil)
	elp1 := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasLimit(20000).SetAction(tsf1).Build()
	selp1, err := action.Sign(elp1, priKeyA)
	require.NoError(err)

	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight: 1,
			Producer:    identityset.Address(27),
			GasLimit:    testutil.TestGasLimit,
		},
	)
	ctx = protocol.WithFeatureCtx(ctx)
	mockActPool := mock_actpool.NewMockActPool(gomock.NewController(t))
	mockActPool.EXPECT().PendingActionMap().Return(map[string][]*action.SealedEnvelope{
		a.String(): {selp1},
	}).Times(1)

	blk1, err := sdb.Mint(
		protocol.WithBlockchainCtx(
			ctx,
			protocol.BlockchainCtx{
				ChainID: 1,
				Tip: protocol.TipInfo{
					Height: 0,
					Hash:   hash.ZeroHash256,
				},
			},
		),
		mockActPool,
		identityset.PrivateKey(27))
	require.NoError(err)
	require.NotNil(blk1)

	ws1, exist, err := sdb.(*stateDB).getFromWorkingSets(ctx, blk1.HashBlock())
	require.NoError(err)
	require.True(exist)
	require.NotNil(ws1)
	// Check balances after the first block
	accountA, err := accountutil.AccountState(ctx, ws1, a)
	require.NoError(err)
	accountB, err := accountutil.AccountState(ctx, ws1, b)
	require.NoError(err)
	require.Equal(big.NewInt(80), accountA.Balance)
	require.Equal(big.NewInt(70), accountB.Balance)

	// Mint the second block with a transfer from B to C
	c := identityset.Address(30)
	priKeyB := identityset.PrivateKey(29)

	tsf2 := action.NewTransfer(big.NewInt(30), c.String(), nil)
	elp2 := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasLimit(20000).SetAction(tsf2).Build()
	selp2, err := action.Sign(elp2, priKeyB)
	require.NoError(err)

	mockActPool.EXPECT().PendingActionMap().Return(map[string][]*action.SealedEnvelope{
		b.String(): {selp2},
	}).Times(1)

	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight: 2,
			Producer:    identityset.Address(27),
			GasLimit:    testutil.TestGasLimit,
		},
	)
	ctx = protocol.WithFeatureCtx(ctx)
	blk2, err := sdb.Mint(
		protocol.WithBlockchainCtx(
			ctx,
			protocol.BlockchainCtx{
				ChainID: 1,
				Tip: protocol.TipInfo{
					Height: 1,
					Hash:   blk1.HashBlock(),
				},
			},
		),
		mockActPool,
		identityset.PrivateKey(27),
	)
	require.NoError(err)
	require.NotNil(blk2)

	ws2, exist, err := sdb.(*stateDB).getFromWorkingSets(ctx, blk2.HashBlock())
	require.NoError(err)
	require.True(exist)
	require.NotNil(ws2)

	// Check balances after the second block
	accountA, err = accountutil.AccountState(ctx, ws2, a)
	require.NoError(err)
	accountB, err = accountutil.AccountState(ctx, ws2, b)
	require.NoError(err)
	accountC, err := accountutil.AccountState(ctx, ws2, c)
	require.NoError(err)
	require.Equal(big.NewInt(80), accountA.Balance)
	require.Equal(big.NewInt(40), accountB.Balance)
	require.Equal(big.NewInt(30), accountC.Balance)
	// Check balances in ws1
	accountA, err = accountutil.AccountState(ctx, ws1, a)
	require.NoError(err)
	accountB, err = accountutil.AccountState(ctx, ws1, b)
	require.NoError(err)
	require.Equal(big.NewInt(80), accountA.Balance)
	require.Equal(big.NewInt(70), accountB.Balance)
	// Check balances in sdb
	accountA, err = accountutil.AccountState(ctx, sdb, a)
	require.NoError(err)
	accountB, err = accountutil.AccountState(ctx, sdb, b)
	require.NoError(err)
	accountC, err = accountutil.AccountState(ctx, sdb, c)
	require.NoError(err)
	require.Equal(big.NewInt(100), accountA.Balance)
	require.Equal(big.NewInt(50), accountB.Balance)
	require.Equal(big.NewInt(0), accountC.Balance)
	// Put blk1 into sdb
	require.NoError(sdb.PutBlock(ctx, blk1))

	// Check balances after blk1 is committed
	accountA, err = accountutil.AccountState(ctx, sdb, a)
	require.NoError(err)
	accountB, err = accountutil.AccountState(ctx, sdb, b)
	require.NoError(err)
	require.Equal(big.NewInt(80), accountA.Balance)
	require.Equal(big.NewInt(70), accountB.Balance)
	// Check balances in ws2
	accountA, err = accountutil.AccountState(ctx, ws2, a)
	require.NoError(err)
	accountB, err = accountutil.AccountState(ctx, ws2, b)
	require.NoError(err)
	accountC, err = accountutil.AccountState(ctx, ws2, c)
	require.NoError(err)
	require.Equal(big.NewInt(80), accountA.Balance)
	require.Equal(big.NewInt(40), accountB.Balance)
	require.Equal(big.NewInt(30), accountC.Balance)
}

func BenchmarkSDBInMemRunAction(b *testing.B) {
	cfg := DefaultConfig
	sdb, err := NewStateDB(cfg, db.NewMemKVStore(), SkipBlockValidationStateDBOption())
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
	cfg := DefaultConfig
	cfg.Chain.TrieDBPath = tp
	db1, err := db.CreateKVStore(db.DefaultConfig, cfg.Chain.TrieDBPath)
	require.NoError(b, err)
	sdb, err := NewStateDB(cfg, db1, SkipBlockValidationStateDBOption())
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
	cfg := DefaultConfig
	cfg.Chain.TrieDBPath = tp
	db2, err := db.CreateKVStoreWithCache(db.DefaultConfig, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
	require.NoError(b, err)
	sdb, err := NewStateDB(cfg, db2, SkipBlockValidationStateDBOption())
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
	ge := genesis.TestDefault()
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
		acts := make([]*action.SealedEnvelope, 0, total)
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
			tx := action.NewTransfer(big.NewInt(1), receiver, nil)
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
		zctx = genesis.WithGenesisContext(zctx, genesis.TestDefault())

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
	cfg := DefaultConfig
	cfg.Chain.TrieDBPath = tp
	db1, err := db.CreateKVStore(db.DefaultConfig, cfg.Chain.TrieDBPath)
	require.NoError(b, err)
	sdb, err := NewStateDB(cfg, db1, SkipBlockValidationStateDBOption())
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
	cfg := DefaultConfig
	cfg.Chain.TrieDBPath = tp
	db2, err := db.CreateKVStoreWithCache(db.DefaultConfig, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
	require.NoError(b, err)
	sdb, err := NewStateDB(cfg, db2, SkipBlockValidationStateDBOption())
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
	ge := genesis.TestDefault()
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
	acts := make([]*action.SealedEnvelope, 0, total)
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
		tx := action.NewTransfer(big.NewInt(1), receiver, nil)
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
	zctx = genesis.WithGenesisContext(zctx, genesis.TestDefault())

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
		_, err = accountutil.AccountState(zctx, sf, addr)
		if err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
	}
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
