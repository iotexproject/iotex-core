// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
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
	stateDB := NewStateDBAdapter(mcm, ws, 1, hash.ZeroHash256)

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
	cfg.Explorer.Enabled = true
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
	}()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	mcm := mock_chainmanager.NewMockChainManager(ctrl)
	stateDB := NewStateDBAdapter(mcm, ws, 1, hash.ZeroHash256)
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
	cfg.Explorer.Enabled = true
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
	stateDB := NewStateDBAdapter(mcm, ws, 1, hash.ZeroHash256)
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
	stateDB := NewStateDBAdapter(mcm, ws, 1, hash.ZeroHash256)
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
	cfg.Explorer.Enabled = true
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
	stateDB := NewStateDBAdapter(mcm, ws, 1, hash.ZeroHash256)
	require.Equal(uint64(0), stateDB.GetNonce(addr))
	stateDB.SetNonce(addr, 1)
	require.Equal(uint64(1), stateDB.GetNonce(addr))
}

func TestSnapshotAndRevert(t *testing.T) {
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
	stateDB := NewStateDBAdapter(mcm, ws, 1, hash.ZeroHash256)

	code := []byte("test contract creation")
	addr1 := common.HexToAddress("02ae2a956d21e8d481c3a69e146633470cf625ec")
	cntr1 := common.HexToAddress("01fc246633470cf62ae2a956d21e8d481c3a69e1")
	cntr3 := common.HexToAddress("956d21e8d481c3a6901fc246633470cf62ae2ae1")
	cntr2 := common.HexToAddress("3470cf62ae2a956d38d481c3a69e121e01fc2466")
	cntr4 := common.HexToAddress("121e01fc24663470cf62ae2a956d38d481c3a69e")
	k1 := hash.Hash256b([]byte("cat"))
	v1 := hash.Hash256b([]byte("cat"))
	k2 := hash.Hash256b([]byte("dog"))
	v2 := hash.Hash256b([]byte("dog"))
	k3 := hash.Hash256b([]byte("hen"))
	v3 := hash.Hash256b([]byte("hen"))
	k4 := hash.Hash256b([]byte("fox"))
	v4 := hash.Hash256b([]byte("fox"))

	addAmount := big.NewInt(40000)
	stateDB.AddBalance(addr1, addAmount)
	stateDB.SetCode(cntr1, code)
	v := stateDB.GetCode(cntr1)
	require.Equal(code, v)
	require.NoError(stateDB.setContractState(hash.BytesToHash160(cntr1[:]), k1, v1))
	require.NoError(stateDB.setContractState(hash.BytesToHash160(cntr1[:]), k2, v2))
	require.NoError(stateDB.setContractState(hash.BytesToHash160(cntr3[:]), k3, v4))
	require.False(stateDB.Suicide(cntr2))
	require.False(stateDB.Exist(cntr2))
	require.False(stateDB.Suicide(cntr4))
	require.False(stateDB.Exist(cntr4))
	stateDB.AddPreimage(common.BytesToHash(v1[:]), []byte("cat"))
	stateDB.AddPreimage(common.BytesToHash(v2[:]), []byte("dog"))
	require.Equal(0, stateDB.Snapshot())

	stateDB.AddBalance(addr1, addAmount)
	require.NoError(stateDB.setContractState(hash.BytesToHash160(cntr1[:]), k1, v3))
	require.NoError(stateDB.setContractState(hash.BytesToHash160(cntr1[:]), k2, v4))
	stateDB.SetCode(cntr2, code)
	v = stateDB.GetCode(cntr2)
	require.Equal(code, v)
	require.NoError(stateDB.setContractState(hash.BytesToHash160(cntr2[:]), k3, v3))
	require.NoError(stateDB.setContractState(hash.BytesToHash160(cntr2[:]), k4, v4))
	// kill contract 1 and 3
	require.True(stateDB.Suicide(cntr1))
	require.True(stateDB.Exist(cntr1))
	require.True(stateDB.Suicide(cntr3))
	require.True(stateDB.Exist(cntr3))
	stateDB.AddPreimage(common.BytesToHash(v3[:]), []byte("hen"))
	require.Equal(1, stateDB.Snapshot())

	require.NoError(stateDB.setContractState(hash.BytesToHash160(cntr2[:]), k3, v1))
	require.NoError(stateDB.setContractState(hash.BytesToHash160(cntr2[:]), k4, v2))
	require.True(stateDB.Suicide(addr1))
	require.True(stateDB.Exist(addr1))
	stateDB.AddPreimage(common.BytesToHash(v4[:]), []byte("fox"))
	require.Equal(2, stateDB.Snapshot())

	stateDB.RevertToSnapshot(2)
	// cntr1 and 3 killed, but still exists before commit
	require.True(stateDB.HasSuicided(cntr1))
	require.True(stateDB.Exist(cntr1))
	require.True(stateDB.HasSuicided(cntr3))
	require.True(stateDB.Exist(cntr3))
	w, _ := stateDB.getContractState(hash.BytesToHash160(cntr1[:]), k1)
	require.Equal(v3, w)
	w, _ = stateDB.getContractState(hash.BytesToHash160(cntr1[:]), k2)
	require.Equal(v4, w)
	// cntr2 still normal
	require.False(stateDB.HasSuicided(cntr2))
	require.True(stateDB.Exist(cntr2))
	w, _ = stateDB.getContractState(hash.BytesToHash160(cntr2[:]), k3)
	require.Equal(v1, w)
	w, _ = stateDB.getContractState(hash.BytesToHash160(cntr2[:]), k4)
	require.Equal(v2, w)
	// addr1 also killed
	require.True(stateDB.HasSuicided(addr1))
	require.True(stateDB.Exist(addr1))
	amount := stateDB.GetBalance(addr1)
	require.Equal(0, amount.Cmp(big.NewInt(0)))
	v, _ = stateDB.preimages[common.BytesToHash(v1[:])]
	require.Equal([]byte("cat"), v)
	v, _ = stateDB.preimages[common.BytesToHash(v2[:])]
	require.Equal([]byte("dog"), v)
	v, _ = stateDB.preimages[common.BytesToHash(v3[:])]
	require.Equal([]byte("hen"), v)
	v, _ = stateDB.preimages[common.BytesToHash(v4[:])]
	require.Equal([]byte("fox"), v)

	stateDB.RevertToSnapshot(1)
	// cntr1 and 3 killed, but still exists before commit
	require.True(stateDB.HasSuicided(cntr1))
	require.True(stateDB.Exist(cntr1))
	require.True(stateDB.HasSuicided(cntr3))
	require.True(stateDB.Exist(cntr3))
	w, _ = stateDB.getContractState(hash.BytesToHash160(cntr1[:]), k1)
	require.Equal(v3, w)
	w, _ = stateDB.getContractState(hash.BytesToHash160(cntr1[:]), k2)
	require.Equal(v4, w)
	// cntr2 is normal
	require.False(stateDB.HasSuicided(cntr2))
	require.True(stateDB.Exist(cntr2))
	w, _ = stateDB.getContractState(hash.BytesToHash160(cntr2[:]), k3)
	require.Equal(v3, w)
	w, _ = stateDB.getContractState(hash.BytesToHash160(cntr2[:]), k4)
	require.Equal(v4, w)
	// addr1 has balance 80000
	require.False(stateDB.HasSuicided(addr1))
	require.True(stateDB.Exist(addr1))
	amount = stateDB.GetBalance(addr1)
	require.Equal(0, amount.Cmp(big.NewInt(80000)))
	v, _ = stateDB.preimages[common.BytesToHash(v1[:])]
	require.Equal([]byte("cat"), v)
	v, _ = stateDB.preimages[common.BytesToHash(v2[:])]
	require.Equal([]byte("dog"), v)
	v, _ = stateDB.preimages[common.BytesToHash(v3[:])]
	require.Equal([]byte("hen"), v)
	_, ok := stateDB.preimages[common.BytesToHash(v4[:])]
	require.False(ok)

	stateDB.RevertToSnapshot(0)
	// cntr1 and 3 is normal
	require.False(stateDB.HasSuicided(cntr1))
	require.True(stateDB.Exist(cntr1))
	require.False(stateDB.HasSuicided(cntr3))
	require.True(stateDB.Exist(cntr3))
	w, _ = stateDB.getContractState(hash.BytesToHash160(cntr1[:]), k1)
	require.Equal(v1, w)
	w, _ = stateDB.getContractState(hash.BytesToHash160(cntr1[:]), k2)
	require.Equal(v2, w)
	// cntr2 and 4 does not exist
	require.False(stateDB.Exist(cntr2))
	require.False(stateDB.Exist(cntr4))
	// addr1 has balance 40000
	require.False(stateDB.HasSuicided(addr1))
	require.True(stateDB.Exist(addr1))
	amount = stateDB.GetBalance(addr1)
	require.Equal(0, amount.Cmp(addAmount))
	v, _ = stateDB.preimages[common.BytesToHash(v1[:])]
	require.Equal([]byte("cat"), v)
	v, _ = stateDB.preimages[common.BytesToHash(v2[:])]
	require.Equal([]byte("dog"), v)
	_, ok = stateDB.preimages[common.BytesToHash(v3[:])]
	require.False(ok)
	_, ok = stateDB.preimages[common.BytesToHash(v4[:])]
	require.False(ok)

	require.NoError(stateDB.CommitContracts())
	stateDB.clear()
	require.True(stateDB.Exist(addr1))
	require.True(stateDB.Exist(cntr1))
	require.False(stateDB.Exist(cntr2))
	require.False(stateDB.Exist(cntr4))
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
		stateDB := NewStateDBAdapter(mcm, sm, 1, hash.ZeroHash256)
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
	stateDB := NewStateDBAdapter(mcm, ws, 1, hash.ZeroHash256)

	v1 := hash.Hash256b([]byte("cat"))
	v2 := hash.Hash256b([]byte("dog"))
	v3 := hash.Hash256b([]byte("hen"))
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
