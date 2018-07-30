// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package keystore

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestAccountManager_NewAccount(t *testing.T) {
	require := require.New(t)
	m := NewMemAccountManager()
	addr, err := m.NewAccount()
	require.NoError(err)

	val, err := m.keystore.Get(addr.RawAddress)
	require.NoError(err)
	require.Equal(addr, val)
}

func TestAccountManager_Contains(t *testing.T) {
	require := require.New(t)
	m := NewMemAccountManager()
	addr, err := m.NewAccount()
	require.NoError(err)

	_, err = m.Contains("123")
	require.Equal(ErrAddr, errors.Cause(err))
	exist, err := m.Contains(addr.RawAddress)
	require.NoError(err)
	require.Equal(true, exist)
}

func TestAccountManager_Remove(t *testing.T) {
	require := require.New(t)
	m := NewMemAccountManager()
	_, err := m.NewAccount()
	require.NoError(err)
	addr2, err := m.NewAccount()
	require.NoError(err)
	_, err = m.NewAccount()
	require.NoError(err)

	err = m.Remove(addr2.RawAddress)
	require.NoError(err)

	_, err = m.keystore.Get(addr2.RawAddress)
	require.Equal(ErrNotExist, errors.Cause(err))
}

func TestAccountManager_Import(t *testing.T) {
	require := require.New(t)
	m := NewMemAccountManager()

	key := &Key{PublicKey: pubKey1, PrivateKey: priKey1, RawAddress: rawAddr1}
	keyBytes, err := json.Marshal(key)
	require.NoError(err)

	err = m.Import(keyBytes)
	require.NoError(err)

	addr := testutil.ConstructAddress(pubKey1, priKey1)
	val, err := m.keystore.Get(rawAddr1)
	require.NoError(err)
	require.Equal(addr, val)
}

func TestAccountManager_SignTransfer(t *testing.T) {
	require := require.New(t)
	m := NewMemAccountManager()

	rawTransfer := action.NewTransfer(uint64(1), big.NewInt(1), rawAddr1, rawAddr2)
	signedTransfer, err := m.SignTransfer(rawAddr1, rawTransfer)
	require.Nil(signedTransfer)
	require.Equal(ErrNotExist, errors.Cause(err))

	key := &Key{PublicKey: pubKey1, PrivateKey: priKey1, RawAddress: rawAddr1}
	keyBytes, err := json.Marshal(key)
	require.NoError(err)

	err = m.Import(keyBytes)
	require.NoError(err)

	signedTransfer, err = m.SignTransfer(rawAddr1, rawTransfer)
	require.NoError(err)
	require.NotNil(signedTransfer.Signature)
}

func TestAccountManager_SignVote(t *testing.T) {
	require := require.New(t)
	m := NewMemAccountManager()

	selfPubKey, err := keypair.DecodePublicKey(pubKey1)
	require.NoError(err)
	votePubKey, err := keypair.DecodePublicKey(pubKey2)
	require.NoError(err)
	rawVote := action.NewVote(uint64(1), selfPubKey, votePubKey)
	signedVote, err := m.SignVote(rawAddr1, rawVote)
	require.Nil(signedVote)
	require.Equal(ErrNotExist, errors.Cause(err))

	key := &Key{PublicKey: pubKey1, PrivateKey: priKey1, RawAddress: rawAddr1}
	keyBytes, err := json.Marshal(key)
	require.NoError(err)

	err = m.Import(keyBytes)
	require.NoError(err)

	signedVote, err = m.SignVote(rawAddr1, rawVote)
	require.NoError(err)
	require.NotNil(signedVote.Signature)
}

func TestAccountManager_SignHash(t *testing.T) {
	require := require.New(t)
	m := NewMemAccountManager()

	blk := blockchain.NewBlock(1, 0, hash.ZeroHash32B, nil, nil)
	hash := blk.HashBlock()

	signature, err := m.SignHash(rawAddr1, hash[:])
	require.Nil(signature)
	require.Equal(ErrNotExist, errors.Cause(err))

	key := &Key{PublicKey: pubKey1, PrivateKey: priKey1, RawAddress: rawAddr1}
	keyBytes, err := json.Marshal(key)
	require.NoError(err)

	err = m.Import(keyBytes)
	require.NoError(err)

	signature, err = m.SignHash(rawAddr1, hash[:])
	require.NoError(err)
	require.NotNil(signature)
}
