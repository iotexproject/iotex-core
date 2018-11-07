// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"context"
	"math/big"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/iotexproject/iotex-core/trie"
)

func TestCreateContract(t *testing.T) {
	require := require.New(t)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	sf, err := NewFactory(&cfg, DefaultTrieOption())
	require.Nil(err)
	require.Nil(sf.Start(context.Background()))

	code := []byte("test contract creation")
	addr := testaddress.Addrinfo["alfa"]
	ws, err := sf.NewWorkingSet()
	require.Nil(err)
	_, err = ws.LoadOrCreateAccountState(addr.RawAddress, big.NewInt(0))
	require.Nil(err)
	contractHash, _ := iotxaddress.GetPubkeyHash(addr.RawAddress)
	contract := byteutil.BytesTo20B(contractHash)
	require.Nil(ws.SetCode(contract, code))
	// contract exist
	codeHash, _ := ws.GetCodeHash(contract)
	require.NotEqual(hash.ZeroHash32B, codeHash)
	v, _ := ws.GetCode(contract)
	require.Equal(code, v)
	// non-existing contract
	addr1 := byteutil.BytesTo20B(hash.Hash160b([]byte("random")))
	h, err := ws.GetCodeHash(addr1)
	require.Error(err)
	require.Equal(hash.ZeroHash32B, h)
	v, err = ws.GetCode(addr1)
	require.Error(err)
	require.Equal([]byte(nil), v)
	gasLimit := testutil.TestGasLimit
	ctx := Context{testaddress.Addrinfo["producer"].RawAddress, &gasLimit, testutil.EnableGasCharge}
	_, _, err = ws.RunActions(0, nil, ctx)
	require.Nil(err)
	// reload same contract
	contract1, err := ws.LoadOrCreateAccountState(addr.RawAddress, big.NewInt(0))
	require.Nil(err)
	require.Equal(contract1.CodeHash, codeHash[:])
	require.Nil(sf.Commit(ws))
	require.Nil(sf.Stop(context.Background()))

	sf, err = NewFactory(&cfg, PrecreatedTrieDBOption(db.NewBoltDB(testTriePath, nil)))
	require.Nil(err)
	require.Nil(sf.Start(context.Background()))
	// reload same contract
	ws, err = sf.NewWorkingSet()
	require.Nil(err)
	contract1, err = ws.LoadOrCreateAccountState(addr.RawAddress, big.NewInt(0))
	require.Nil(err)
	require.Equal(contract1.CodeHash, codeHash[:])
	// contract already exist
	h, _ = ws.GetCodeHash(contract)
	require.Equal(codeHash, h)
	v, _ = ws.GetCode(contract)
	require.Equal(code, v)
	require.Nil(sf.Stop(context.Background()))
}

func TestLoadStoreContract(t *testing.T) {
	require := require.New(t)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	sf, err := NewFactory(&cfg, DefaultTrieOption())
	require.Nil(err)
	require.Nil(sf.Start(context.Background()))

	code := []byte("test contract creation")
	addr := testaddress.Addrinfo["alfa"]
	ws, err := sf.NewWorkingSet()
	require.Nil(err)
	_, err = ws.LoadOrCreateAccountState(addr.RawAddress, big.NewInt(0))
	require.Nil(err)
	contractHash, _ := iotxaddress.GetPubkeyHash(addr.RawAddress)
	contract := byteutil.BytesTo20B(contractHash)
	require.Nil(ws.SetCode(contract, code))
	codeHash, _ := ws.GetCodeHash(contract)
	require.NotEqual(hash.ZeroHash32B, codeHash)

	v, _ := ws.GetCode(contract)
	require.Equal(code, v)
	// insert entries into storage
	k1 := byteutil.BytesTo32B(hash.Hash160b([]byte("cat")))
	v1 := byteutil.BytesTo32B(hash.Hash256b([]byte("cat")))
	k2 := byteutil.BytesTo32B(hash.Hash160b([]byte("dog")))
	v2 := byteutil.BytesTo32B(hash.Hash256b([]byte("dog")))
	require.Nil(ws.SetContractState(contract, k1, v1))
	require.Nil(ws.SetContractState(contract, k2, v2))

	code1 := []byte("2nd contract creation")
	addr1 := testaddress.Addrinfo["bravo"]
	_, err = ws.LoadOrCreateAccountState(addr1.RawAddress, big.NewInt(0))
	require.Nil(err)
	contractHash, err = iotxaddress.GetPubkeyHash(addr1.RawAddress)
	require.Nil(err)
	contract1 := byteutil.BytesTo20B(contractHash)
	require.Nil(ws.SetCode(contract1, code1))
	codeHash1, _ := ws.GetCodeHash(contract1)
	require.NotEqual(hash.ZeroHash32B, codeHash1)
	v, _ = ws.GetCode(contract1)
	require.Equal(code1, v)
	// insert entries into storage
	k3 := byteutil.BytesTo32B(hash.Hash160b([]byte("egg")))
	v3 := byteutil.BytesTo32B(hash.Hash256b([]byte("egg")))
	k4 := byteutil.BytesTo32B(hash.Hash160b([]byte("hen")))
	v4 := byteutil.BytesTo32B(hash.Hash256b([]byte("hen")))
	require.Nil(ws.SetContractState(contract1, k3, v3))
	require.Nil(ws.SetContractState(contract1, k4, v4))
	gasLimit := testutil.TestGasLimit
	ctx := Context{testaddress.Addrinfo["producer"].RawAddress, &gasLimit, testutil.EnableGasCharge}
	_, _, err = ws.RunActions(0, nil, ctx)
	require.Nil(err)
	require.Nil(sf.Commit(ws))
	require.Nil(sf.Stop(context.Background()))

	// re-open the StateFactory
	sf, err = NewFactory(&cfg, PrecreatedTrieDBOption(db.NewBoltDB(testTriePath, nil)))
	require.Nil(err)
	require.Nil(sf.Start(context.Background()))
	// query first contract
	ws, err = sf.NewWorkingSet()
	require.Nil(err)
	w, err := ws.GetContractState(contract, k1)
	require.Nil(err)
	require.Equal(v1, w)
	w, err = ws.GetContractState(contract, k2)
	require.Nil(err)
	require.Equal(v2, w)
	_, err = ws.GetContractState(contract, k3)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	_, err = ws.GetContractState(contract, k4)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	// query second contract
	w, err = ws.GetContractState(contract1, k3)
	require.Nil(err)
	require.Equal(v3, w)
	w, err = ws.GetContractState(contract1, k4)
	require.Nil(err)
	require.Equal(v4, w)
	_, err = ws.GetContractState(contract1, k1)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	_, err = ws.GetContractState(contract1, k2)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	require.Nil(sf.Stop(context.Background()))
}
