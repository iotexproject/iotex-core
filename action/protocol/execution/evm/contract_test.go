// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	testTriePath = "trie.test"
)

func TestCreateContract(t *testing.T) {
	require := require.New(t)
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.Nil(err)
	require.Nil(sf.Start(context.Background()))

	code := []byte("test contract creation")
	addr := testaddress.Addrinfo["alfa"]
	ws, err := sf.NewWorkingSet()
	require.Nil(err)
	_, err = account.LoadOrCreateAccount(ws, addr.String(), big.NewInt(0))
	require.Nil(err)
	stateDB := StateDBAdapter{
		sm:             ws,
		cachedContract: make(map[hash.Hash160]Contract),
		dao:            ws.GetDB(),
		cb:             ws.GetCachedBatch(),
	}
	contract := addr.Bytes()
	var evmContract common.Address
	copy(evmContract[:], contract[:])
	stateDB.SetCode(evmContract, code)
	// contract exist
	codeHash := stateDB.GetCodeHash(evmContract)
	var emptyEVMHash common.Hash
	require.NotEqual(emptyEVMHash, codeHash)
	v := stateDB.GetCode(evmContract)
	require.Equal(code, v)
	// non-existing contract
	addr1 := byteutil.BytesTo20B(hash.Hash160b([]byte("random")))
	var evmAddr1 common.Address
	copy(evmAddr1[:], addr1[:])
	h := stateDB.GetCodeHash(evmAddr1)
	require.Equal(emptyEVMHash, h)
	require.Nil(stateDB.GetCode(evmAddr1))
	require.NoError(stateDB.commitContracts())
	stateDB.clear()
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer:        testaddress.Addrinfo["producer"],
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 0, nil)
	require.Nil(err)

	// reload same contract
	contract1, err := account.LoadOrCreateAccount(ws, addr.String(), big.NewInt(0))
	require.Nil(err)
	require.Equal(codeHash[:], contract1.CodeHash)
	require.Nil(sf.Commit(ws))
	require.Nil(sf.Stop(context.Background()))

	cfg.DB.DbPath = testTriePath
	sf, err = factory.NewFactory(cfg, factory.PrecreatedTrieDBOption(db.NewOnDiskDB(cfg.DB)))
	require.Nil(err)
	require.Nil(sf.Start(context.Background()))
	// reload same contract
	ws, err = sf.NewWorkingSet()
	require.Nil(err)
	contract1, err = account.LoadOrCreateAccount(ws, addr.String(), big.NewInt(0))
	require.Nil(err)
	require.Equal(codeHash[:], contract1.CodeHash)
	stateDB = StateDBAdapter{
		sm:             ws,
		cachedContract: make(map[hash.Hash160]Contract),
		dao:            ws.GetDB(),
		cb:             ws.GetCachedBatch(),
	}
	// contract already exist
	h = stateDB.GetCodeHash(evmContract)
	require.Equal(codeHash, h)
	v = stateDB.GetCode(evmContract)
	require.Equal(code, v)
	require.Nil(sf.Stop(context.Background()))
}

func TestLoadStoreContract(t *testing.T) {
	require := require.New(t)
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.Nil(err)
	require.Nil(sf.Start(context.Background()))

	code := []byte("test contract creation")
	addr := testaddress.Addrinfo["alfa"]
	ws, err := sf.NewWorkingSet()
	require.Nil(err)
	_, err = account.LoadOrCreateAccount(ws, addr.String(), big.NewInt(0))
	require.Nil(err)
	stateDB := StateDBAdapter{
		sm:             ws,
		cachedContract: make(map[hash.Hash160]Contract),
		dao:            ws.GetDB(),
		cb:             ws.GetCachedBatch(),
	}
	contract := addr.Bytes()
	var evmContract common.Address
	copy(evmContract[:], contract[:])
	stateDB.SetCode(evmContract, code)
	codeHash := stateDB.GetCodeHash(evmContract)
	var emptyEVMHash common.Hash
	require.NotEqual(emptyEVMHash, codeHash)

	v := stateDB.GetCode(evmContract)
	require.Equal(code, v)
	// insert entries into storage
	k1 := byteutil.BytesTo32B(hash.Hash160b([]byte("cat")))
	v1 := byteutil.BytesTo32B(hash.Hash256b([]byte("cat")))
	k2 := byteutil.BytesTo32B(hash.Hash160b([]byte("dog")))
	v2 := byteutil.BytesTo32B(hash.Hash256b([]byte("dog")))
	require.Nil(stateDB.setContractState(byteutil.BytesTo20B(contract), k1, v1))
	require.Nil(stateDB.setContractState(byteutil.BytesTo20B(contract), k2, v2))

	code1 := []byte("2nd contract creation")
	addr1 := testaddress.Addrinfo["bravo"]
	_, err = account.LoadOrCreateAccount(ws, addr1.String(), big.NewInt(0))
	require.Nil(err)
	contract1 := addr1.Bytes()
	var evmContract1 common.Address
	copy(evmContract1[:], contract1[:])
	stateDB.SetCode(evmContract1, code1)
	codeHash1 := stateDB.GetCodeHash(evmContract1)
	require.NotEqual(emptyEVMHash, codeHash1)
	v = stateDB.GetCode(evmContract1)
	require.Equal(code1, v)
	// insert entries into storage
	k3 := byteutil.BytesTo32B(hash.Hash160b([]byte("egg")))
	v3 := byteutil.BytesTo32B(hash.Hash256b([]byte("egg")))
	k4 := byteutil.BytesTo32B(hash.Hash160b([]byte("hen")))
	v4 := byteutil.BytesTo32B(hash.Hash256b([]byte("hen")))
	require.Nil(stateDB.setContractState(byteutil.BytesTo20B(contract1), k3, v3))
	require.Nil(stateDB.setContractState(byteutil.BytesTo20B(contract1), k4, v4))
	require.NoError(stateDB.commitContracts())
	stateDB.clear()

	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer:        testaddress.Addrinfo["producer"],
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 0, nil)
	require.Nil(err)
	require.Nil(sf.Commit(ws))
	require.Nil(sf.Stop(context.Background()))

	// re-open the StateFactory
	cfg.DB.DbPath = testTriePath
	sf, err = factory.NewFactory(cfg, factory.PrecreatedTrieDBOption(db.NewOnDiskDB(cfg.DB)))
	require.Nil(err)
	require.Nil(sf.Start(context.Background()))
	// query first contract
	ws, err = sf.NewWorkingSet()
	require.Nil(err)
	stateDB = StateDBAdapter{
		sm:             ws,
		cachedContract: make(map[hash.Hash160]Contract),
		dao:            ws.GetDB(),
		cb:             ws.GetCachedBatch(),
	}

	w, err := stateDB.getContractState(byteutil.BytesTo20B(contract), k1)
	require.Nil(err)
	require.Equal(v1, w)
	w, err = stateDB.getContractState(byteutil.BytesTo20B(contract), k2)
	require.Nil(err)
	require.Equal(v2, w)
	_, err = stateDB.getContractState(byteutil.BytesTo20B(contract), k3)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	_, err = stateDB.getContractState(byteutil.BytesTo20B(contract), k4)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	// query second contract
	w, err = stateDB.getContractState(byteutil.BytesTo20B(contract1), k3)
	require.Nil(err)
	require.Equal(v3, w)
	w, err = stateDB.getContractState(byteutil.BytesTo20B(contract1), k4)
	require.Nil(err)
	require.Equal(v4, w)
	_, err = stateDB.getContractState(byteutil.BytesTo20B(contract1), k1)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	_, err = stateDB.getContractState(byteutil.BytesTo20B(contract1), k2)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	require.Nil(sf.Stop(context.Background()))
}

func TestSnapshot(t *testing.T) {
	require := require.New(t)

	s := &state.Account{
		Balance:      big.NewInt(5),
		VotingWeight: big.NewInt(0),
	}
	k1 := byteutil.BytesTo32B(hash.Hash160b([]byte("cat")))
	v1 := byteutil.BytesTo32B(hash.Hash256b([]byte("cat")))
	k2 := byteutil.BytesTo32B(hash.Hash160b([]byte("dog")))
	v2 := byteutil.BytesTo32B(hash.Hash256b([]byte("dog")))

	c1, err := newContract(
		byteutil.BytesTo20B(testaddress.Addrinfo["alfa"].Bytes()),
		s,
		db.NewMemKVStore(),
		db.NewCachedBatch(),
	)
	require.NoError(err)
	require.NoError(c1.SetState(k2, v2[:]))
	c2 := c1.Snapshot()
	require.NoError(c1.SelfState().AddBalance(big.NewInt(7)))
	require.NoError(c1.SetState(k1, v1[:]))
	require.Equal(big.NewInt(12), c1.SelfState().Balance)
	require.Equal(big.NewInt(5), c2.SelfState().Balance)
	require.NotEqual(c1.RootHash(), c2.RootHash())
}
