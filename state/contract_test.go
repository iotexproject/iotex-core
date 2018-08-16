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
	addr, err := iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	require.Nil(err)
	_, err = sf.LoadOrCreateState(addr.RawAddress, 5)
	require.Nil(err)
	root := sf.RootHash()
	require.NotEqual(root, trie.EmptyRoot)

	code := []byte("test contract creation")
	contractAddr, err := sf.CreateContract(addr.RawAddress, code)
	require.Nil(err)
	require.NotEqual("", contractAddr)
	contractHash, err := iotxaddress.GetPubkeyHash(contractAddr)
	require.Nil(err)
	contract := byteutil.BytesTo20B(contractHash)
	// contract exist
	codeHash, _ := sf.GetCodeHash(contract)
	require.NotEqual(hash.ZeroHash32B, codeHash)
	v, _ := sf.GetCode(contract)
	require.Equal(code, v)
	// re-create cause collision
	contract1, err := sf.CreateContract(addr.RawAddress, code)
	require.Equal(ErrAccountCollision, errors.Cause(err))
	require.Equal("", contract1)
	// non-existing contract
	addr1 := byteutil.BytesTo20B(hash.Hash160b([]byte("random")))
	_, err = sf.GetCodeHash(addr1)
	require.Equal(ErrAccountNotExist, errors.Cause(err))
	_, err = sf.GetCode(addr1)
	require.Equal(ErrAccountNotExist, errors.Cause(err))
	require.Nil(sf.CommitStateChanges(0, nil, nil, nil))
	root = sf.RootHash()
	require.Nil(sf.Stop(context.Background()))

	tr, err := trie.NewTrie(db.NewBoltDB(testTriePath, nil), trie.AccountKVNameSpace, root)
	require.Nil(err)
	sf, err = NewFactory(&cfg, PrecreatedTrieOption(tr, nil))
	require.Nil(err)
	require.Nil(sf.Start(context.Background()))
	// cannot re-create existing
	_, err = sf.CreateContract(addr.RawAddress, code)
	require.Equal(ErrAccountCollision, errors.Cause(err))
	// contract already exist
	h, _ := sf.GetCodeHash(contract)
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
	addr, err := iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	require.Nil(err)
	_, err = sf.LoadOrCreateState(addr.RawAddress, 5)
	require.Nil(err)
	addr1, err := iotxaddress.NewAddress(true, []byte{0xa4, 0x00, 0x00, 0x00})
	require.Nil(err)
	_, err = sf.LoadOrCreateState(addr1.RawAddress, 6)
	require.Nil(err)
	root := sf.RootHash()
	require.NotEqual(root, trie.EmptyRoot)

	code := []byte("test contract creation")
	contractAddr, err := sf.CreateContract(addr.RawAddress, code)
	require.Nil(err)
	require.NotEqual("", contractAddr)
	contractHash, err := iotxaddress.GetPubkeyHash(contractAddr)
	require.Nil(err)
	contract := byteutil.BytesTo20B(contractHash)
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
	contractAddr1, err := sf.CreateContract(addr1.RawAddress, code1)
	require.Nil(err)
	require.NotEqual("", contractAddr1)
	contractHash, err = iotxaddress.GetPubkeyHash(contractAddr1)
	require.Nil(err)
	contract1 := byteutil.BytesTo20B(contractHash)
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
	root = sf.RootHash()
	require.Nil(sf.Stop(context.Background()))

	// re-open the StateFactory
	tr, err := trie.NewTrie(db.NewBoltDB(testTriePath, nil), trie.AccountKVNameSpace, root)
	require.Nil(err)
	sf, err = NewFactory(&cfg, PrecreatedTrieOption(tr, nil))
	require.Nil(err)
	require.Nil(sf.Start(context.Background()))
	// query first contract
	w, err := sf.GetContractState(contract, k1)
	require.Nil(err)
	require.Equal(v1, w)
	w, err = sf.GetContractState(contract, k2)
	require.Nil(err)
	require.Equal(v2, w)
	w, err = sf.GetContractState(contract, k3)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	w, err = sf.GetContractState(contract, k4)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	// query second contract
	w, err = sf.GetContractState(contract1, k3)
	require.Nil(err)
	require.Equal(v3, w)
	w, err = sf.GetContractState(contract1, k4)
	require.Nil(err)
	require.Equal(v4, w)
	w, err = sf.GetContractState(contract1, k1)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	w, err = sf.GetContractState(contract1, k2)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	require.Nil(sf.Stop(context.Background()))
}
