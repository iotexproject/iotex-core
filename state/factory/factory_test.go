// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"encoding/hex"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/test/mock/mock_committee"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

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
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	triePath    = "trie.test"
	stateDBPath = "stateDB.test"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func TestSnapshot(t *testing.T) {
	require := require.New(t)
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), triePath)
	testTriePath := testTrieFile.Name()

	cfg := config.Default
	cfg.DB.DbPath = testTriePath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "5"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "7"
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewBoltDB(cfg.DB)))
	require.NoError(err)
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	ctx := protocol.WithBlockCtx(
		protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{
			Genesis:  cfg.Genesis,
			Registry: registry,
		}),
		protocol.BlockCtx{},
	)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
	}()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	testSnapshot(ws, t)
	testRevert(ws, t)
}

func TestSDBSnapshot(t *testing.T) {
	require := require.New(t)
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), stateDBPath)
	testStateDBPath := testTrieFile.Name()

	cfg := config.Default
	cfg.Chain.TrieDBPath = testStateDBPath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "5"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "7"
	sdb, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(err)
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	ctx := protocol.WithBlockCtx(
		protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{
			Genesis:  cfg.Genesis,
			Registry: registry,
		}),
		protocol.BlockCtx{},
	)
	require.NoError(sdb.Start(ctx))
	ws, err := sdb.NewWorkingSet()
	require.NoError(err)
	testSnapshot(ws, t)
	testSDBRevert(ws, t)
}

func testRevert(ws WorkingSet, t *testing.T) {
	require := require.New(t)
	sHash := hash.BytesToHash160(identityset.Address(28).Bytes())

	s, err := accountutil.LoadAccount(ws, sHash)
	require.NoError(err)
	require.Equal(big.NewInt(5), s.Balance)
	s0 := ws.Snapshot()
	require.Equal(1, s0)

	s.Balance.Add(s.Balance, big.NewInt(5))
	require.Equal(big.NewInt(10), s.Balance)
	require.NoError(ws.PutState(sHash, s))

	require.NoError(ws.Revert(s0))
	require.NoError(ws.State(sHash, s))
	require.Equal(big.NewInt(5), s.Balance)
}

func testSDBRevert(ws WorkingSet, t *testing.T) {
	require := require.New(t)
	sHash := hash.BytesToHash160(identityset.Address(28).Bytes())

	s, err := accountutil.LoadAccount(ws, sHash)
	require.NoError(err)
	require.Equal(big.NewInt(5), s.Balance)
	s0 := ws.Snapshot()
	require.Equal(1, s0)

	s.Balance.Add(s.Balance, big.NewInt(5))
	require.Equal(big.NewInt(10), s.Balance)
	require.NoError(ws.PutState(sHash, s))

	require.NoError(ws.Revert(s0))
	require.NoError(ws.State(sHash, s))
	require.Equal(big.NewInt(5), s.Balance)
}

func testSnapshot(ws WorkingSet, t *testing.T) {
	require := require.New(t)
	sHash := hash.BytesToHash160(identityset.Address(28).Bytes())
	tHash := hash.BytesToHash160(identityset.Address(29).Bytes())

	s, err := accountutil.LoadAccount(ws, tHash)
	require.NoError(err)
	require.Equal(big.NewInt(7), s.Balance)
	s, err = accountutil.LoadAccount(ws, sHash)
	require.NoError(err)
	require.Equal(big.NewInt(5), s.Balance)
	s0 := ws.Snapshot()
	require.Zero(s0)
	s.Balance.Add(s.Balance, big.NewInt(5))
	require.Equal(big.NewInt(10), s.Balance)
	require.NoError(ws.PutState(sHash, s))
	s1 := ws.Snapshot()
	require.Equal(1, s1)
	s.Balance.Add(s.Balance, big.NewInt(5))
	require.Equal(big.NewInt(15), s.Balance)
	require.NoError(ws.PutState(sHash, s))

	s, err = accountutil.LoadAccount(ws, tHash)
	require.NoError(err)
	require.Equal(big.NewInt(7), s.Balance)
	s2 := ws.Snapshot()
	require.Equal(2, s2)
	require.NoError(s.AddBalance(big.NewInt(6)))
	require.Equal(big.NewInt(13), s.Balance)
	require.NoError(ws.PutState(tHash, s))

	require.NoError(ws.Revert(s2))
	require.NoError(ws.State(sHash, s))
	require.Equal(big.NewInt(15), s.Balance)
	require.NoError(ws.State(tHash, s))
	require.Equal(big.NewInt(7), s.Balance)
	require.NoError(ws.Revert(s1))
	require.NoError(ws.State(sHash, s))
	require.Equal(big.NewInt(10), s.Balance)
	require.NoError(ws.Revert(s0))
	require.NoError(ws.State(sHash, s))
	require.Equal(big.NewInt(5), s.Balance)
}

func TestCandidates(t *testing.T) {
	cfg := config.Default
	sf, err := NewFactory(cfg, InMemTrieOption())
	require.NoError(t, err)
	testCandidates(sf, t)
}

func TestSDBCandidates(t *testing.T) {
	cfg := config.Default
	sdb, err := NewStateDB(cfg, InMemStateDBOption())
	require.NoError(t, err)
	testCandidates(sdb, t)
}

func testCandidates(sf Factory, t *testing.T) {
	sc := state.CandidateList{
		&state.Candidate{
			Address: identityset.Address(1).String(),
			Votes:   big.NewInt(2),
		},
		&state.Candidate{
			Address: identityset.Address(2).String(),
			Votes:   big.NewInt(22),
		},
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
	defer ctrl.Finish()

	committee := mock_committee.NewMockCommittee(ctrl)
	committee.EXPECT().ResultByHeight(uint64(123456)).Return(nil, nil).AnyTimes()
	committee.EXPECT().HeightByTime(gomock.Any()).Return(uint64(123456), nil).AnyTimes()

	registry := protocol.NewRegistry()
	require.NoError(t, registry.Register("rolldpos", rolldpos.NewProtocol(36, 36, 20)))
	p, err := poll.NewGovernanceChainCommitteeProtocol(
		nil,
		nil,
		committee,
		uint64(123456),
		func(uint64) (time.Time, error) { return time.Now(), nil },
		config.Default.Genesis.NumCandidateDelegates,
		config.Default.Genesis.NumDelegates,
		config.Default.Chain.PollInitialCandidatesInterval,
		nil,
		nil,
		config.Default.Genesis.ProductivityThreshold,
		config.Default.Genesis.KickoutEpochPeriod,
		config.Default.Genesis.KickoutIntensityRate,
	)
	require.NoError(t, registry.Register("poll", p))
	require.NoError(t, err)
	gasLimit := testutil.TestGasLimit

	ctx := protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{
		Genesis:  config.Default.Genesis,
		Registry: registry,
	})
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight: 1,
		Producer:    identityset.Address(27),
		GasLimit:    gasLimit,
	})

	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions([]action.SealedEnvelope{selp}...).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)

	require.NoError(t, sf.Commit(ctx, &blk))

	candidates, err := candidatesutil.CandidatesByHeight(sf, 1)
	require.NoError(t, err)
	require.Equal(t, 2, len(candidates))
	require.Equal(t, candidates[0].Address, identityset.Address(1).String())
	require.Equal(t, candidates[0].Votes, big.NewInt(2))
	require.Equal(t, candidates[1].Address, identityset.Address(2).String())
	require.Equal(t, candidates[1].Votes, big.NewInt(22))
}

func TestState(t *testing.T) {
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), triePath)
	testTriePath := testTrieFile.Name()

	cfg := config.Default
	cfg.DB.DbPath = testTriePath
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewBoltDB(cfg.DB)))
	require.NoError(t, err)
	testState(sf, t)
}

func TestHistoryState(t *testing.T) {
	// using factory and enable history
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), triePath)
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTrieFile.Name()
	cfg.Chain.EnableHistoryStateDB = true
	sf, err := NewFactory(cfg, DefaultTrieOption())
	require.NoError(t, err)
	testHistoryState(sf, t, false, cfg.Chain.EnableHistoryStateDB)

	// using statedb and enable history
	testTrieFile, _ = ioutil.TempFile(os.TempDir(), triePath)
	cfg.Chain.TrieDBPath = testTrieFile.Name()
	sf, err = NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(t, err)
	testHistoryState(sf, t, true, cfg.Chain.EnableHistoryStateDB)

	// using factory and disable history
	testTrieFile, _ = ioutil.TempFile(os.TempDir(), triePath)
	cfg.Chain.TrieDBPath = testTrieFile.Name()
	cfg.Chain.EnableHistoryStateDB = false
	sf, err = NewFactory(cfg, DefaultTrieOption())
	require.NoError(t, err)
	testHistoryState(sf, t, false, cfg.Chain.EnableHistoryStateDB)

	// using statedb and disable history
	testTrieFile, _ = ioutil.TempFile(os.TempDir(), triePath)
	cfg.Chain.TrieDBPath = testTrieFile.Name()
	sf, err = NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(t, err)
	testHistoryState(sf, t, true, cfg.Chain.EnableHistoryStateDB)
}

func TestSDBState(t *testing.T) {
	testDBFile, _ := ioutil.TempFile(os.TempDir(), stateDBPath)
	testDBPath := testDBFile.Name()

	cfg := config.Default
	cfg.Chain.TrieDBPath = testDBPath
	sdb, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(t, err)
	testState(sdb, t)
}

func testState(sf Factory, t *testing.T) {
	// Create a dummy iotex address
	a := identityset.Address(28).String()
	priKeyA := identityset.PrivateKey(28)
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, acc.Register(registry))
	ge := genesis.Default
	ge.InitBalanceMap[a] = "100"
	gasLimit := uint64(1000000)
	ctx := protocol.WithBlockCtx(
		context.Background(),
		protocol.BlockCtx{
			BlockHeight: 0,
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		},
	)
	ctx = protocol.WithBlockchainCtx(
		ctx,
		protocol.BlockchainCtx{
			Genesis:  config.Default.Genesis,
			Registry: registry,
		},
	)

	require.NoError(t, sf.Start(ctx))
	defer func() {
		require.NoError(t, sf.Stop(ctx))
	}()

	tsf, err := action.NewTransfer(1, big.NewInt(10), identityset.Address(31).String(), nil, uint64(20000), big.NewInt(0))
	require.NoError(t, err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tsf).SetGasLimit(20000).Build()
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
	require.NoError(t, sf.Commit(ctx, &blk))

	//test AccountState() & State()
	var testAccount state.Account
	accountA, err := accountutil.AccountState(sf, a)
	require.NoError(t, err)
	sHash := hash.BytesToHash160(identityset.Address(28).Bytes())
	err = sf.State(sHash, &testAccount)
	require.NoError(t, err)
	require.Equal(t, accountA, &testAccount)
	require.Equal(t, big.NewInt(90), accountA.Balance)
}

func testHistoryState(sf Factory, t *testing.T, statetx, archive bool) {
	// Create a dummy iotex address
	a := identityset.Address(28).String()
	b := identityset.Address(31).String()
	priKeyA := identityset.PrivateKey(28)
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, acc.Register(registry))
	ge := genesis.Default
	ge.InitBalanceMap[a] = "100"
	gasLimit := uint64(1000000)
	ctx := protocol.WithBlockCtx(
		context.Background(),
		protocol.BlockCtx{
			BlockHeight: 0,
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		},
	)
	ctx = protocol.WithBlockchainCtx(
		ctx,
		protocol.BlockchainCtx{
			Genesis:  config.Default.Genesis,
			Registry: registry,
		},
	)
	require.NoError(t, sf.Start(ctx))
	defer func() {
		require.NoError(t, sf.Stop(ctx))
	}()
	tsf, err := action.NewTransfer(1, big.NewInt(10), b, nil, uint64(20000), big.NewInt(0))
	require.NoError(t, err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tsf).SetGasLimit(20000).Build()
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
	require.NoError(t, sf.Commit(ctx, &blk))

	// check latest balance
	accountA, err := accountutil.AccountState(sf, a)
	require.NoError(t, err)
	accountB, err := accountutil.AccountState(sf, b)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(90), accountA.Balance)
	require.Equal(t, big.NewInt(10), accountB.Balance)

	// check archive data
	if statetx {
		// statetx not support archive mode
		_, err = accountutil.AccountStateAtHeight(sf, a, 0)
		require.True(t, errors.Cause(err) == ErrNotSupported)
		_, err = accountutil.AccountStateAtHeight(sf, b, 0)
		require.True(t, errors.Cause(err) == ErrNotSupported)
	} else {
		if !archive {
			_, err = accountutil.AccountStateAtHeight(sf, a, 0)
			require.True(t, errors.Cause(err) == ErrNoArchiveData)
			_, err = accountutil.AccountStateAtHeight(sf, b, 0)
			require.True(t, errors.Cause(err) == ErrNoArchiveData)
		} else {
			accountA, err = accountutil.AccountStateAtHeight(sf, a, 0)
			require.NoError(t, err)
			accountB, err = accountutil.AccountStateAtHeight(sf, b, 0)
			require.NoError(t, err)
			require.Equal(t, big.NewInt(100), accountA.Balance)
			require.Equal(t, big.NewInt(0), accountB.Balance)
		}
	}
}

func TestNonce(t *testing.T) {
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), triePath)
	testTriePath := testTrieFile.Name()

	cfg := config.Default
	cfg.DB.DbPath = testTriePath
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewBoltDB(cfg.DB)))
	require.NoError(t, err)
	testNonce(sf, t)
}
func TestSDBNonce(t *testing.T) {
	testDBFile, _ := ioutil.TempFile(os.TempDir(), stateDBPath)
	testDBPath := testDBFile.Name()

	cfg := config.Default
	cfg.Chain.TrieDBPath = testDBPath
	sdb, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(t, err)

	testNonce(sdb, t)
}

func testNonce(sf Factory, t *testing.T) {
	// Create two dummy iotex address
	a := identityset.Address(28).String()
	priKeyA := identityset.PrivateKey(28)
	b := identityset.Address(29).String()

	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, acc.Register(registry))
	ge := genesis.Default
	ge.InitBalanceMap[a] = "100"
	gasLimit := uint64(1000000)
	ctx := protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			BlockHeight: 0,
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		})
	ctx = protocol.WithBlockchainCtx(ctx,
		protocol.BlockchainCtx{
			Genesis:  config.Default.Genesis,
			Registry: registry,
		})

	require.NoError(t, sf.Start(ctx))
	defer func() {
		require.NoError(t, sf.Stop(ctx))
	}()
	ws, err := sf.NewWorkingSet()
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
	_, err = ws.RunAction(ctx, selp)
	require.NoError(t, err)
	state, err := accountutil.AccountState(sf, a)
	require.NoError(t, err)
	require.Equal(t, uint64(0), state.Nonce)

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

	require.NoError(t, sf.Commit(ctx, &blk))
	state, err = accountutil.AccountState(sf, a)
	require.NoError(t, err)
	require.Equal(t, uint64(1), state.Nonce)
}

func TestLoadStoreHeight(t *testing.T) {
	require := require.New(t)
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), triePath)
	testTriePath := testTrieFile.Name()

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	statefactory, err := NewFactory(cfg, DefaultTrieOption())
	require.NoError(err)

	testLoadStoreHeight(statefactory, t)
}

func TestLoadStoreHeightInMem(t *testing.T) {
	require := require.New(t)

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), triePath)
	testTriePath := testTrieFile.Name()
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	statefactory, err := NewFactory(cfg, InMemTrieOption())
	require.NoError(err)
	testLoadStoreHeight(statefactory, t)
}

func TestSDBLoadStoreHeight(t *testing.T) {
	require := require.New(t)
	testDBFile, _ := ioutil.TempFile(os.TempDir(), stateDBPath)
	testDBPath := testDBFile.Name()

	cfg := config.Default
	cfg.Chain.TrieDBPath = testDBPath
	db, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(err)

	testLoadStoreHeight(db, t)
}

func TestSDBLoadStoreHeightInMem(t *testing.T) {
	require := require.New(t)

	testDBFile, _ := ioutil.TempFile(os.TempDir(), stateDBPath)
	testDBPath := testDBFile.Name()
	cfg := config.Default
	cfg.Chain.TrieDBPath = testDBPath
	db, err := NewStateDB(cfg, InMemStateDBOption())
	require.NoError(err)

	testLoadStoreHeight(db, t)
}

func testLoadStoreHeight(sf Factory, t *testing.T) {
	require := require.New(t)
	ctx := protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{})
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
	}()
	height, err := sf.Height()
	require.NoError(err)
	require.Equal(uint64(0), height)

	height, err = sf.Height()
	require.NoError(err)
	require.Equal(uint64(1), height)
}

func TestRunActions(t *testing.T) {
	require := require.New(t)
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), triePath)
	testTriePath := testTrieFile.Name()

	cfg := config.Default
	cfg.DB.DbPath = testTriePath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "200"
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewBoltDB(cfg.DB)))
	require.NoError(err)

	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	ctx := protocol.WithBlockCtx(
		protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{
			Genesis:  cfg.Genesis,
			Registry: registry,
		}),
		protocol.BlockCtx{},
	)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
	}()
	testCommit(sf, registry, t)
}

func TestSTXRunActions(t *testing.T) {
	require := require.New(t)
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), stateDBPath)
	testStateDBPath := testTrieFile.Name()

	cfg := config.Default
	cfg.Chain.TrieDBPath = testStateDBPath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "200"
	sdb, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(err)

	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	ctx := protocol.WithBlockCtx(
		protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{
			Genesis:  cfg.Genesis,
			Registry: registry,
		}),
		protocol.BlockCtx{},
	)
	require.NoError(sdb.Start(ctx))
	defer func() {
		require.NoError(sdb.Stop(ctx))
	}()
	testCommit(sdb, registry, t)
}

func testCommit(factory Factory, registry *protocol.Registry, t *testing.T) {
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

	blkHash := selp1.Hash()

	gasLimit := uint64(1000000)
	ctx := protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			BlockHeight: 1,
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		})
	ctx = protocol.WithBlockchainCtx(ctx,
		protocol.BlockchainCtx{
			Genesis:  config.Default.Genesis,
			Registry: registry,
			Tip: protocol.TipInfo{
				Height: 0,
				Hash:   blkHash,
			},
		})

	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(blkHash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp1, selp2).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)

	require.NoError(factory.Commit(ctx, &blk))
}

func TestPickAndRunActions(t *testing.T) {
	require := require.New(t)
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), triePath)
	testTriePath := testTrieFile.Name()

	cfg := config.Default
	cfg.DB.DbPath = testTriePath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "200"
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewBoltDB(cfg.DB)))
	require.NoError(err)

	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	ctx := protocol.WithBlockCtx(
		protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{
			Genesis:  cfg.Genesis,
			Registry: registry,
		}),
		protocol.BlockCtx{},
	)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
	}()
	testNewBlockBuilder(sf, registry, t)
}

func TestSTXPickAndRunActions(t *testing.T) {
	require := require.New(t)
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), stateDBPath)
	testStateDBPath := testTrieFile.Name()

	cfg := config.Default
	cfg.Chain.TrieDBPath = testStateDBPath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "200"
	sdb, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(err)

	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	ctx := protocol.WithBlockCtx(
		protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{
			Genesis:  cfg.Genesis,
			Registry: registry,
		}),
		protocol.BlockCtx{},
	)
	require.NoError(sdb.Start(ctx))
	defer func() {
		require.NoError(sdb.Stop(ctx))
	}()
	testNewBlockBuilder(sdb, registry, t)
}

func testNewBlockBuilder(factory Factory, registry *protocol.Registry, t *testing.T) {
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

	addr0 := identityset.Address(27).String()
	tsf0, err := testutil.SignedTransfer(addr0, identityset.PrivateKey(0), 1, big.NewInt(90000000), nil, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(err)
	accMap := make(map[string][]action.SealedEnvelope)
	accMap[identityset.Address(0).String()] = []action.SealedEnvelope{tsf0}

	tx2, err := action.NewTransfer(uint64(1), big.NewInt(20), a, nil, uint64(100000), big.NewInt(0))
	require.NoError(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).SetAction(tx2).Build()
	selp2, err := action.Sign(elp, priKeyB)
	require.NoError(err)

	gasLimit := uint64(1000000)
	ctx := protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			BlockHeight: 1,
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		})
	ctx = protocol.WithBlockchainCtx(ctx,
		protocol.BlockchainCtx{
			Genesis:  config.Default.Genesis,
			Registry: registry,
		})

	minter, ok := factory.(Minter)
	require.True(ok)
	blkBuilder, err := minter.NewBlockBuilder(ctx, accMap, []action.SealedEnvelope{selp1, selp2})
	require.NoError(err)
	require.NotNil(blkBuilder)
	blk, err := blkBuilder.SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)
	require.NoError(factory.Commit(ctx, &blk))
}

func TestSimulateExecution(t *testing.T) {
	require := require.New(t)
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), triePath)
	testTriePath := testTrieFile.Name()

	cfg := config.Default
	cfg.DB.DbPath = testTriePath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "200"
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewBoltDB(cfg.DB)))
	require.NoError(err)

	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	ctx := protocol.WithBlockCtx(
		protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{
			Genesis:  cfg.Genesis,
			Registry: registry,
		}),
		protocol.BlockCtx{},
	)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
	}()
	testSimulateExecution(ctx, sf, t)
}

func TestSTXSimulateExecution(t *testing.T) {
	require := require.New(t)
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), stateDBPath)
	testStateDBPath := testTrieFile.Name()

	cfg := config.Default
	cfg.Chain.TrieDBPath = testStateDBPath
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100"
	cfg.Genesis.InitBalanceMap[identityset.Address(29).String()] = "200"
	sdb, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(err)

	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	ctx := protocol.WithBlockCtx(
		protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{
			Genesis:  cfg.Genesis,
			Registry: registry,
		}),
		protocol.BlockCtx{},
	)
	require.NoError(sdb.Start(ctx))
	defer func() {
		require.NoError(sdb.Stop(ctx))
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
	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	testCachedBatch(ws, t)
}

func TestSTXCachedBatch(t *testing.T) {
	sdb, err := NewStateDB(config.Default, InMemStateDBOption())
	require.NoError(t, err)
	ws, _ := sdb.NewWorkingSet()
	testCachedBatch(ws, t)
}

func testCachedBatch(ws WorkingSet, t *testing.T) {
	require := require.New(t)

	// test PutState()
	hashA := hash.BytesToHash160(identityset.Address(28).Bytes())
	accountA := state.EmptyAccount()
	accountA.Balance = big.NewInt(70)
	err := ws.PutState(hashA, accountA)
	require.NoError(err)

	// test State()
	testAccount := state.EmptyAccount()
	require.NoError(ws.State(hashA, &testAccount))
	require.Equal(accountA, testAccount)

	// test DelState()
	err = ws.DelState(hashA)
	require.NoError(err)

	// can't state account "alfa" anymore
	require.Error(ws.State(hashA, &testAccount))
}

func TestGetDB(t *testing.T) {
	require := require.New(t)
	sf, err := NewFactory(config.Default, InMemTrieOption())
	require.NoError(err)
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	require.Equal(uint64(1), ws.Version())
	kvstore := ws.GetDB()
	_, ok := kvstore.(db.KVStoreWithBuffer)
	require.True(ok)
}

func TestSTXGetDB(t *testing.T) {
	require := require.New(t)
	sdb, err := NewStateDB(config.Default, InMemStateDBOption())
	require.NoError(err)
	ws, err := sdb.NewWorkingSet()
	require.NoError(err)
	require.Equal(uint64(1), ws.Version())
	kvstore := ws.GetDB()
	_, ok := kvstore.(db.KVStoreWithBuffer)
	require.True(ok)
}

func TestDeleteAndPutSameKey(t *testing.T) {
	testDeleteAndPutSameKey := func(t *testing.T, ws WorkingSet) {
		key := hash.Hash160b([]byte("test"))
		acc := state.Account{
			Nonce: 1,
		}
		require.NoError(t, ws.PutState(key, acc))
		require.NoError(t, ws.DelState(key))
		require.Equal(t, state.ErrStateNotExist, errors.Cause(ws.State(key, &acc)))
		require.Equal(t, state.ErrStateNotExist, errors.Cause(ws.State(hash.Hash160b([]byte("other")), &acc)))
	}
	t.Run("workingSet", func(t *testing.T) {
		sf, err := NewFactory(config.Default, InMemTrieOption())
		require.NoError(t, err)
		ws, err := sf.NewWorkingSet()
		require.NoError(t, err)
		testDeleteAndPutSameKey(t, ws)
	})
	t.Run("stateTx", func(t *testing.T) {
		ws, err := newStateTX(0, db.NewMemKVStore())
		require.NoError(t, err)
		testDeleteAndPutSameKey(t, ws)
	})
}

func BenchmarkInMemRunAction(b *testing.B) {
	cfg := config.Default
	sf, err := NewFactory(cfg, InMemTrieOption())
	if err != nil {
		b.Fatal(err)
	}
	benchRunAction(sf, b)
}

func BenchmarkDBRunAction(b *testing.B) {
	tp := filepath.Join(os.TempDir(), triePath)
	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}

	cfg := config.Default
	cfg.DB.DbPath = tp
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewBoltDB(cfg.DB)))
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
	sdb, err := NewStateDB(cfg, InMemStateDBOption())
	if err != nil {
		b.Fatal(err)
	}
	benchRunAction(sdb, b)
}

func BenchmarkSDBRunAction(b *testing.B) {
	tp := filepath.Join(os.TempDir(), stateDBPath)
	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}
	cfg := config.Default
	cfg.Chain.TrieDBPath = tp
	sdb, err := NewStateDB(cfg, DefaultStateDBOption())
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
	for _, acc := range accounts {
		ge.InitBalanceMap[acc] = big.NewInt(int64(b.N * 100)).String()
	}
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	if err := acc.Register(registry); err != nil {
		b.Fatal(err)
	}
	ctx := protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{
		Genesis:  ge,
		Registry: registry,
	})
	if err := sf.Start(ctx); err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := sf.Stop(ctx); err != nil {
			b.Fatal(err)
		}
	}()

	gasLimit := testutil.TestGasLimit * 100000

	for n := 0; n < b.N; n++ {

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
		zctx = protocol.WithBlockchainCtx(zctx,
			protocol.BlockchainCtx{
				Genesis:  config.Default.Genesis,
				Registry: registry,
			})

		blk, err := block.NewTestingBuilder().
			SetHeight(uint64(n)).
			SetPrevBlockHash(hash.ZeroHash256).
			SetTimeStamp(testutil.TimestampNow()).
			AddActions(acts...).
			SignAndBuild(identityset.PrivateKey(27))
		b.Fatal(err)

		if err := sf.Commit(zctx, &blk); err != nil {
			b.Fatal(err)
		}
	}
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
