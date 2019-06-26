// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"bytes"
	"context"
	"io/ioutil"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestAddBalance(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	cfg := config.Default
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
	}()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	mcm := mock_chainmanager.NewMockChainManager(ctrl)

	addr := common.HexToAddress("02ae2a956d21e8d481c3a69e146633470cf625ec")
	stateDB := NewStateDBAdapter(mcm, ws, config.NewHeightUpgrade(cfg), 1, hash.ZeroHash256)

	addAmount := big.NewInt(40000)
	stateDB.AddBalance(addr, addAmount)
	amount := stateDB.GetBalance(addr)
	require.Equal(0, amount.Cmp(addAmount))
	stateDB.AddBalance(addr, addAmount)
	amount = stateDB.GetBalance(addr)
	require.Equal(0, amount.Cmp(big.NewInt(80000)))
}

func TestRefundAPIs(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	cfg := config.Default
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
	}()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	mcm := mock_chainmanager.NewMockChainManager(ctrl)
	stateDB := NewStateDBAdapter(mcm, ws, config.NewHeightUpgrade(cfg), 1, hash.ZeroHash256)
	require.Zero(stateDB.GetRefund())
	refund := uint64(1024)
	stateDB.AddRefund(refund)
	require.Equal(refund, stateDB.GetRefund())
}

func TestEmptyAndCode(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	cfg := config.Default
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
	}()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	mcm := mock_chainmanager.NewMockChainManager(ctrl)
	addr := common.HexToAddress("02ae2a956d21e8d481c3a69e146633470cf625ec")
	stateDB := NewStateDBAdapter(mcm, ws, config.NewHeightUpgrade(cfg), 1, hash.ZeroHash256)
	require.True(stateDB.Empty(addr))
	stateDB.CreateAccount(addr)
	require.True(stateDB.Empty(addr))
	stateDB.SetCode(addr, []byte("0123456789"))
	require.True(bytes.Equal(stateDB.GetCode(addr), []byte("0123456789")))
	require.False(stateDB.Empty(addr))
}

func TestForEachStorage(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	cfg := config.Default
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
	}()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	mcm := mock_chainmanager.NewMockChainManager(ctrl)

	addr := common.HexToAddress("02ae2a956d21e8d481c3a69e146633470cf625ec")
	stateDB := NewStateDBAdapter(mcm, ws, config.NewHeightUpgrade(cfg), 1, hash.ZeroHash256)
	stateDB.CreateAccount(addr)
	kvs := map[common.Hash]common.Hash{
		common.HexToHash("0123456701234567012345670123456701234567012345670123456701234560"): common.HexToHash("0123456701234567012345670123456701234567012345670123456701234560"),
		common.HexToHash("0123456701234567012345670123456701234567012345670123456701234561"): common.HexToHash("0123456701234567012345670123456701234567012345670123456701234561"),
		common.HexToHash("0123456701234567012345670123456701234567012345670123456701234562"): common.HexToHash("0123456701234567012345670123456701234567012345670123456701234562"),
		common.HexToHash("0123456701234567012345670123456701234567012345670123456701234563"): common.HexToHash("0123456701234567012345670123456701234567012345670123456701234563"),
	}
	for k, v := range kvs {
		stateDB.SetState(addr, k, v)
	}
	stateDB.ForEachStorage(addr, func(k common.Hash, v common.Hash) bool {
		require.Equal(k, v)
		delete(kvs, k)
		return true
	})
	require.Equal(0, len(kvs))
}

func TestNonce(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	cfg := config.Default
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
	}()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	mcm := mock_chainmanager.NewMockChainManager(ctrl)
	addr := common.HexToAddress("02ae2a956d21e8d481c3a69e146633470cf625ec")
	stateDB := NewStateDBAdapter(mcm, ws, config.NewHeightUpgrade(cfg), 1, hash.ZeroHash256)
	require.Equal(uint64(0), stateDB.GetNonce(addr))
	stateDB.SetNonce(addr, 1)
	require.Equal(uint64(1), stateDB.GetNonce(addr))
}

func TestSnapshotRevertAndCommit(t *testing.T) {
	testSnapshotAndRevert := func(cfg config.Config, t *testing.T) {
		require := require.New(t)
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		var sf factory.Factory
		if cfg.Chain.EnableTrielessStateDB {
			sf, _ = factory.NewStateDB(cfg, factory.DefaultStateDBOption())
		} else {
			sf, _ = factory.NewFactory(cfg, factory.DefaultTrieOption())
		}
		require.NoError(sf.Start(ctx))
		ws, err := sf.NewWorkingSet()
		require.NoError(err)
		mcm := mock_chainmanager.NewMockChainManager(ctrl)
		mcm.EXPECT().ChainID().AnyTimes().Return(uint32(1))
		stateDB := NewStateDBAdapter(mcm, ws, config.NewHeightUpgrade(cfg), 1, hash.ZeroHash256)

		tests := []stateDBTest{
			{
				[]bal{
					{addr1, big.NewInt(40000)},
				},
				[]code{
					{c1, bytecode},
				},
				[]evmSet{
					{c1, k1, v1},
					{c1, k2, v2},
					{c3, k3, v4},
				},
				[]sui{
					{c2, false, false},
					{C4, false, false},
				},
				[]image{
					{common.BytesToHash(v1[:]), []byte("cat")},
					{common.BytesToHash(v2[:]), []byte("dog")},
				},
			},
			{
				[]bal{
					{addr1, big.NewInt(40000)},
				},
				[]code{
					{c2, bytecode},
				},
				[]evmSet{
					{c1, k1, v3},
					{c1, k2, v4},
					{c2, k3, v3},
					{c2, k4, v4},
				},
				[]sui{
					{c1, true, true},
					{c3, true, true},
				},
				[]image{
					{common.BytesToHash(v3[:]), []byte("hen")},
				},
			},
			{
				nil,
				nil,
				[]evmSet{
					{c2, k3, v1},
					{c2, k4, v2},
				},
				[]sui{
					{addr1, true, true},
				},
				[]image{
					{common.BytesToHash(v4[:]), []byte("fox")},
				},
			},
		}

		for i, test := range tests {
			// add balance
			for _, e := range test.balance {
				stateDB.AddBalance(e.addr, e.v)
			}
			// set code
			for _, e := range test.codes {
				stateDB.SetCode(e.addr, e.v)
				v := stateDB.GetCode(e.addr)
				require.Equal(e.v, v)
			}
			// set states
			for _, e := range test.states {
				stateDB.SetState(e.addr, e.k, e.v)
			}
			// set suicide
			for _, e := range test.suicide {
				require.Equal(e.suicide, stateDB.Suicide(e.addr))
				require.Equal(e.exist, stateDB.Exist(e.addr))
			}
			// set preimage
			for _, e := range test.preimage {
				stateDB.AddPreimage(e.hash, e.v)
			}
			require.Equal(i, stateDB.Snapshot())
		}

		reverts := []stateDBTest{
			{
				[]bal{
					{addr1, big.NewInt(0)},
				},
				[]code{},
				[]evmSet{
					{c1, k1, v3},
					{c1, k2, v4},
					{c2, k3, v1},
					{c2, k4, v2},
				},
				[]sui{
					{c1, true, true},
					{c3, true, true},
					{c2, false, true},
					{C4, false, false},
					{addr1, true, true},
				},
				[]image{
					{common.BytesToHash(v1[:]), []byte("cat")},
					{common.BytesToHash(v2[:]), []byte("dog")},
					{common.BytesToHash(v3[:]), []byte("hen")},
					{common.BytesToHash(v4[:]), []byte("fox")},
				},
			},
			{
				[]bal{
					{addr1, big.NewInt(80000)},
				},
				[]code{},
				tests[1].states,
				[]sui{
					{c1, true, true},
					{c3, true, true},
					{c2, false, true},
					{C4, false, false},
					{addr1, false, true},
				},
				[]image{
					{common.BytesToHash(v1[:]), []byte("cat")},
					{common.BytesToHash(v2[:]), []byte("dog")},
					{common.BytesToHash(v3[:]), []byte("hen")},
					{common.BytesToHash(v4[:]), []byte(nil)},
				},
			},
			{
				[]bal{
					{addr1, big.NewInt(40000)},
				},
				[]code{},
				[]evmSet{
					{c1, k1, v1},
					{c1, k2, v2},
					{c3, k3, v4},
				},
				[]sui{
					{c1, false, true},
					{c3, false, true},
					{c2, false, false},
					{C4, false, false},
					{addr1, false, true},
				},
				[]image{
					{common.BytesToHash(v1[:]), []byte("cat")},
					{common.BytesToHash(v2[:]), []byte("dog")},
					{common.BytesToHash(v3[:]), []byte(nil)},
					{common.BytesToHash(v4[:]), []byte(nil)},
				},
			},
		}

		// test revert
		for i, test := range reverts {
			stateDB.RevertToSnapshot(len(reverts) - 1 - i)

			// test balance
			for _, e := range test.balance {
				amount := stateDB.GetBalance(e.addr)
				require.Equal(e.v, amount)
			}
			// test states
			for _, e := range test.states {
				require.Equal(e.v, stateDB.GetState(e.addr, e.k))
			}
			// test suicide/exist
			for _, e := range test.suicide {
				require.Equal(e.suicide, stateDB.HasSuicided(e.addr))
				require.Equal(e.exist, stateDB.Exist(e.addr))
			}
			// test preimage
			for _, e := range test.preimage {
				v, _ := stateDB.preimages[e.hash]
				require.Equal(e.v, v)
			}
		}

		// commit snapshot 0's state
		require.NoError(stateDB.CommitContracts())
		stateDB.clear()
		gasLimit := testutil.TestGasLimit
		ctx = protocol.WithRunActionsCtx(ctx,
			protocol.RunActionsCtx{
				Producer: identityset.Address(27),
				GasLimit: gasLimit,
			})
		_, err = ws.RunActions(ctx, 0, nil)
		require.NoError(err)
		require.NoError(sf.Commit(ws))
		require.NoError(sf.Stop(ctx))

		// re-open the StateFactory
		cfg.DB.DbPath = cfg.Chain.TrieDBPath
		if cfg.Chain.EnableTrielessStateDB {
			sf, err = factory.NewStateDB(cfg, factory.PrecreatedStateDBOption(db.NewOnDiskDB(cfg.DB)))
		} else {
			sf, err = factory.NewFactory(cfg, factory.PrecreatedTrieDBOption(db.NewOnDiskDB(cfg.DB)))
		}
		require.NoError(err)
		require.NoError(sf.Start(ctx))
		defer func() {
			require.NoError(sf.Stop(ctx))
		}()

		ws, err = sf.NewWorkingSet()
		require.NoError(err)
		stateDB = NewStateDBAdapter(mcm, ws, config.NewHeightUpgrade(cfg), 1, hash.ZeroHash256)

		// state factory should have snapshot 0's state
		snapshot0 := reverts[len(reverts)-1]
		// test balance
		for _, e := range snapshot0.balance {
			amount := stateDB.GetBalance(e.addr)
			require.Equal(e.v, amount)
		}
		// test states
		for _, e := range snapshot0.states {
			require.Equal(e.v, stateDB.GetState(e.addr, e.k))
		}
		// test suicide/exist
		for _, e := range snapshot0.suicide {
			require.Equal(e.suicide, stateDB.HasSuicided(e.addr))
			require.Equal(e.exist, stateDB.Exist(e.addr))
		}
	}

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	defer func() {
		testutil.CleanupPath(t, testTriePath)
	}()
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	t.Run("contract snapshot/revert/commit with stateDB", func(t *testing.T) {
		testSnapshotAndRevert(cfg, t)
	})

	testTrieFile, _ = ioutil.TempFile(os.TempDir(), "trie")
	testTriePath2 := testTrieFile.Name()
	defer func() {
		testutil.CleanupPath(t, testTriePath2)
	}()
	cfg.Chain.EnableTrielessStateDB = false
	cfg.Chain.TrieDBPath = testTriePath2
	t.Run("contract snapshot/revert/commit with trie", func(t *testing.T) {
		testSnapshotAndRevert(cfg, t)
	})
}

func TestGetBalanceOnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sm := mock_chainmanager.NewMockStateManager(ctrl)
	sm.EXPECT().GetDB().Return(nil).AnyTimes()
	sm.EXPECT().GetCachedBatch().Return(nil).AnyTimes()
	mcm := mock_chainmanager.NewMockChainManager(ctrl)
	mcm.EXPECT().ChainID().Return(uint32(1)).AnyTimes()

	errs := []error{
		state.ErrStateNotExist,
		errors.New("other error"),
	}
	for _, err := range errs {
		sm.EXPECT().State(gomock.Any(), gomock.Any()).Return(err).Times(1)
		addr := common.HexToAddress("test address")
		stateDB := NewStateDBAdapter(mcm, sm, config.NewHeightUpgrade(config.Default), 1, hash.ZeroHash256)
		amount := stateDB.GetBalance(addr)
		assert.Equal(t, big.NewInt(0), amount)
	}
}

func TestPreimage(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	cfg := config.Default
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
	}()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	mcm := mock_chainmanager.NewMockChainManager(ctrl)
	mcm.EXPECT().ChainID().AnyTimes().Return(uint32(1))
	stateDB := NewStateDBAdapter(mcm, ws, config.NewHeightUpgrade(cfg), 1, hash.ZeroHash256)

	stateDB.AddPreimage(common.BytesToHash(v1[:]), []byte("cat"))
	stateDB.AddPreimage(common.BytesToHash(v2[:]), []byte("dog"))
	stateDB.AddPreimage(common.BytesToHash(v3[:]), []byte("hen"))
	// this won't overwrite preimage of v1
	stateDB.AddPreimage(common.BytesToHash(v1[:]), []byte("fox"))
	require.NoError(stateDB.CommitContracts())
	stateDB.clear()
	k, _ := stateDB.cb.Get(PreimageKVNameSpace, v1[:])
	require.Equal([]byte("cat"), k)
	k, _ = stateDB.cb.Get(PreimageKVNameSpace, v2[:])
	require.Equal([]byte("dog"), k)
	k, _ = stateDB.cb.Get(PreimageKVNameSpace, v3[:])
	require.Equal([]byte("hen"), k)

	require.NoError(stateDB.dao.Commit(stateDB.cb))
	k, _ = stateDB.dao.Get(PreimageKVNameSpace, v1[:])
	require.Equal([]byte("cat"), k)
	k, _ = stateDB.dao.Get(PreimageKVNameSpace, v2[:])
	require.Equal([]byte("dog"), k)
	k, _ = stateDB.dao.Get(PreimageKVNameSpace, v3[:])
	require.Equal([]byte("hen"), k)
}
