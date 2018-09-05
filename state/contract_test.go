// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
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
	addr, _ := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
	_, err = sf.LoadOrCreateState(addr.RawAddress, 0)
	require.Nil(err)
	contractHash, _ := iotxaddress.GetPubkeyHash(addr.RawAddress)
	contract := byteutil.BytesTo20B(contractHash)
	require.Nil(sf.SetCode(contract, code))
	// contract exist
	codeHash, _ := sf.GetCodeHash(contract)
	require.NotEqual(hash.ZeroHash32B, codeHash)
	v, _ := sf.GetCode(contract)
	require.Equal(code, v)
	// non-existing contract
	addr1 := byteutil.BytesTo20B(hash.Hash160b([]byte("random")))
	h, err := sf.GetCodeHash(addr1)
	require.Error(err)
	require.Equal(hash.ZeroHash32B, h)
	v, err = sf.GetCode(addr1)
	require.Error(err)
	require.Equal([]byte(nil), v)
	require.Nil(sf.CommitStateChanges(0, nil, nil, nil))
	// reload same contract
	contract1, err := sf.LoadOrCreateState(addr.RawAddress, 0)
	require.Nil(err)
	require.Equal(contract1.CodeHash, codeHash[:])
	root := sf.RootHash()
	require.Nil(sf.Stop(context.Background()))

	tr, err := trie.NewTrie(db.NewBoltDB(testTriePath, nil), trie.AccountKVNameSpace, root)
	require.Nil(err)
	sf, err = NewFactory(&cfg, PrecreatedTrieOption(tr))
	require.Nil(err)
	require.Nil(sf.Start(context.Background()))
	// reload same contract
	contract1, err = sf.LoadOrCreateState(addr.RawAddress, 0)
	require.Nil(err)
	require.Equal(contract1.CodeHash, codeHash[:])
	// contract already exist
	h, _ = sf.GetCodeHash(contract)
	require.Equal(codeHash, h)
	v, _ = sf.GetCode(contract)
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
	addr, _ := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
	_, err = sf.LoadOrCreateState(addr.RawAddress, 0)
	require.Nil(err)
	contractHash, _ := iotxaddress.GetPubkeyHash(addr.RawAddress)
	contract := byteutil.BytesTo20B(contractHash)
	require.Nil(sf.SetCode(contract, code))
	codeHash, _ := sf.GetCodeHash(contract)
	require.NotEqual(hash.ZeroHash32B, codeHash)

	v, _ := sf.GetCode(contract)
	require.Equal(code, v)
	// insert entries into storage
	k1 := byteutil.BytesTo32B(hash.Hash160b([]byte("cat")))
	v1 := byteutil.BytesTo32B(hash.Hash256b([]byte("cat")))
	k2 := byteutil.BytesTo32B(hash.Hash160b([]byte("dog")))
	v2 := byteutil.BytesTo32B(hash.Hash256b([]byte("dog")))
	require.Nil(sf.SetContractState(contract, k1, v1))
	require.Nil(sf.SetContractState(contract, k2, v2))

	code1 := []byte("2nd contract creation")
	addr1, _ := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
	_, err = sf.LoadOrCreateState(addr1.RawAddress, 0)
	require.Nil(err)
	contractHash, err = iotxaddress.GetPubkeyHash(addr1.RawAddress)
	require.Nil(err)
	contract1 := byteutil.BytesTo20B(contractHash)
	require.Nil(sf.SetCode(contract1, code1))
	codeHash1, _ := sf.GetCodeHash(contract1)
	require.NotEqual(hash.ZeroHash32B, codeHash1)
	v, _ = sf.GetCode(contract1)
	require.Equal(code1, v)
	// insert entries into storage
	k3 := byteutil.BytesTo32B(hash.Hash160b([]byte("egg")))
	v3 := byteutil.BytesTo32B(hash.Hash256b([]byte("egg")))
	k4 := byteutil.BytesTo32B(hash.Hash160b([]byte("hen")))
	v4 := byteutil.BytesTo32B(hash.Hash256b([]byte("hen")))
	require.Nil(sf.SetContractState(contract1, k3, v3))
	require.Nil(sf.SetContractState(contract1, k4, v4))

	require.Nil(sf.CommitStateChanges(0, nil, nil, nil))
	root := sf.RootHash()
	require.Nil(sf.Stop(context.Background()))

	// re-open the StateFactory
	tr, err := trie.NewTrie(db.NewBoltDB(testTriePath, nil), trie.AccountKVNameSpace, root)
	require.Nil(err)
	sf, err = NewFactory(&cfg, PrecreatedTrieOption(tr))
	require.Nil(err)
	require.Nil(sf.Start(context.Background()))
	// query first contract
	w, err := sf.GetContractState(contract, k1)
	require.Nil(err)
	require.Equal(v1, w)
	w, err = sf.GetContractState(contract, k2)
	require.Nil(err)
	require.Equal(v2, w)
	_, err = sf.GetContractState(contract, k3)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	_, err = sf.GetContractState(contract, k4)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	// query second contract
	w, err = sf.GetContractState(contract1, k3)
	require.Nil(err)
	require.Equal(v3, w)
	w, err = sf.GetContractState(contract1, k4)
	require.Nil(err)
	require.Equal(v4, w)
	_, err = sf.GetContractState(contract1, k1)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	_, err = sf.GetContractState(contract1, k2)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	require.Nil(sf.Stop(context.Background()))
}
