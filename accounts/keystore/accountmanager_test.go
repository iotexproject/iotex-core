// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package keystore

import (
	"math/big"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestAccountManager_NewAccount(t *testing.T) {
	require := require.New(t)
	m := NewMemAccountManager()
	priKey, err := m.NewAccount()
	require.NoError(err)

	addr, err := keyToAddress(priKey)
	require.NoError(err)
	val, err := m.keystore.Get(addr.Bech32())
	require.NoError(err)
	require.Equal(priKey, val)
}

func TestAccountManager_Contains(t *testing.T) {
	require := require.New(t)
	m := NewMemAccountManager()
	priKey, err := m.NewAccount()
	require.NoError(err)

	_, err = m.Contains("123")
	require.Error(err)
	addr, err := keyToAddress(priKey)
	require.NoError(err)
	exist, err := m.Contains(addr.Bech32())
	require.NoError(err)
	require.Equal(true, exist)
}

func TestAccountManager_Remove(t *testing.T) {
	require := require.New(t)
	m := NewMemAccountManager()
	_, err := m.NewAccount()
	require.NoError(err)
	priKey2, err := m.NewAccount()
	require.NoError(err)
	_, err = m.NewAccount()
	require.NoError(err)

	addr2, err := keyToAddress(priKey2)
	require.NoError(err)
	err = m.Remove(addr2.Bech32())
	require.NoError(err)

	_, err = m.keystore.Get(addr2.Bech32())
	require.Equal(ErrNotExist, errors.Cause(err))
}

func TestAccountManager_Import(t *testing.T) {
	require := require.New(t)
	m := NewMemAccountManager()

	key, err := keypair.DecodePrivateKey(prikeyProducer)
	require.NoError(err)

	require.NoError(m.Import(key[:]))

	addr, err := keyToAddress(key)
	require.NoError(err)
	val, err := m.keystore.Get(addr.Bech32())
	require.NoError(err)
	require.Equal(key, val)
}

func TestAccountManager_SignTransfer(t *testing.T) {
	require := require.New(t)
	m := NewMemAccountManager()

	tsf, err := action.NewTransfer(uint64(1), big.NewInt(1), testaddress.Addrinfo["producer"].Bech32(),
		testaddress.Addrinfo["alfa"].Bech32(), []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	bd := action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetAction(tsf).
		SetGasLimit(100000).
		SetDestinationAddress(testaddress.Addrinfo["alfa"].Bech32()).
		SetGasPrice(big.NewInt(10)).Build()
	_, err = m.SignAction(testaddress.Addrinfo["producer"].Bech32(), elp)
	require.Equal(ErrNotExist, errors.Cause(err))

	key, err := keypair.DecodePrivateKey(prikeyProducer)
	require.NoError(err)

	require.NoError(m.Import(key[:]))

	selp, err := m.SignAction(testaddress.Addrinfo["producer"].Bech32(), elp)
	require.NoError(err)
	require.NotNil(selp.Signature())
}

func TestAccountManager_SignVote(t *testing.T) {
	require := require.New(t)
	m := NewMemAccountManager()

	vote, err := action.NewVote(
		uint64(1), testaddress.Addrinfo["producer"].Bech32(), testaddress.Addrinfo["alfa"].Bech32(), uint64(100000),
		big.NewInt(10))
	require.NoError(err)

	bd := action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetAction(vote).
		SetGasLimit(100000).
		SetDestinationAddress(testaddress.Addrinfo["alfa"].Bech32()).
		SetGasPrice(big.NewInt(10)).Build()

	_, err = m.SignAction(testaddress.Addrinfo["producer"].Bech32(), elp)
	require.Equal(ErrNotExist, errors.Cause(err))

	key, err := keypair.DecodePrivateKey(prikeyProducer)
	require.NoError(err)

	require.NoError(m.Import(key[:]))

	selp, err := m.SignAction(testaddress.Addrinfo["producer"].Bech32(), elp)
	require.NoError(err)
	require.NotNil(selp.Signature())
}

func TestAccountManager_SignHash(t *testing.T) {
	require := require.New(t)
	m := NewMemAccountManager()

	pk, err := keypair.DecodePublicKey(pubkeyProducer)
	require.NoError(err)
	blk := block.NewBlockDeprecated(1, 0, hash.ZeroHash32B, testutil.TimestampNow(), pk, nil)
	hash := blk.HashBlock()

	signature, err := m.SignHash(testaddress.Addrinfo["producer"].Bech32(), hash[:])
	require.Nil(signature)
	require.Equal(ErrNotExist, errors.Cause(err))

	key, err := keypair.DecodePrivateKey(prikeyProducer)
	require.NoError(err)

	require.NoError(m.Import(key[:]))

	signature, err = m.SignHash(testaddress.Addrinfo["producer"].Bech32(), hash[:])
	require.NoError(err)
	require.NotNil(signature)
}

func keyToAddress(priKey keypair.PrivateKey) (address.Address, error) {
	pubKey, err := crypto.EC283.NewPubKey(priKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to derive public key from private key")
	}
	pkHash := keypair.HashPubKey(pubKey)
	return address.New(config.Default.Chain.ID, pkHash[:]), nil
}
