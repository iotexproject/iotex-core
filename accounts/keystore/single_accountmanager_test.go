// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package keystore

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestSingleAccountManager_SignTransfer(t *testing.T) {
	require := require.New(t)

	key, err := hex.DecodeString(prikeyProducer)
	require.NoError(err)

	accountManager := NewMemAccountManager()
	require.NoError(accountManager.Import(key))

	m, err := NewSingleAccountManager(accountManager)
	require.NoError(err)

	tsf, err := action.NewTransfer(uint64(1), big.NewInt(1), testaddress.Addrinfo["producer"].Bech32(),
		testaddress.Addrinfo["alfa"].Bech32(), []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)

	bd := action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetAction(tsf).
		SetGasLimit(100000).
		SetDestinationAddress(testaddress.Addrinfo["alfa"].Bech32()).
		SetGasPrice(big.NewInt(10)).Build()

	_, err = m.SignAction(elp)
	require.NoError(err)

	require.NoError(accountManager.Remove(testaddress.Addrinfo["producer"].Bech32()))
	_, err = m.SignAction(elp)
	require.Equal(ErrNumAccounts, errors.Cause(err))
}

func TestSingleAccountManager_SignVote(t *testing.T) {
	require := require.New(t)

	key, err := hex.DecodeString(prikeyProducer)
	require.NoError(err)

	accountManager := NewMemAccountManager()
	require.NoError(accountManager.Import(key))

	m, err := NewSingleAccountManager(accountManager)
	require.NoError(err)

	vote, err := action.NewVote(
		uint64(1), testaddress.Addrinfo["producer"].Bech32(), testaddress.Addrinfo["alfa"].Bech32(),
		uint64(100000), big.NewInt(10))
	require.NoError(err)

	bd := action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetAction(vote).
		SetGasLimit(100000).
		SetDestinationAddress(testaddress.Addrinfo["alfa"].Bech32()).
		SetGasPrice(big.NewInt(10)).Build()

	_, err = m.SignAction(elp)
	require.NoError(err)

	require.NoError(accountManager.Remove(testaddress.Addrinfo["producer"].Bech32()))
	_, err = m.SignAction(elp)
	require.Equal(ErrNumAccounts, errors.Cause(err))
}

func TestSingleAccountManager_SignHash(t *testing.T) {
	require := require.New(t)

	key, err := hex.DecodeString(prikeyProducer)
	require.NoError(err)

	accountManager := NewMemAccountManager()
	require.NoError(accountManager.Import(key))

	m, err := NewSingleAccountManager(accountManager)
	require.NoError(err)

	pk, err := keypair.DecodePublicKey(pubkeyProducer)
	require.NoError(err)
	blk := block.NewBlockDeprecated(1, 0, hash.ZeroHash32B, testutil.TimestampNow(), pk, nil)
	hash := blk.HashBlock()
	signature, err := m.SignHash(hash[:])
	require.NoError(err)
	require.NotNil(signature)

	require.NoError(accountManager.Remove(testaddress.Addrinfo["producer"].Bech32()))
	signature, err = m.SignHash(hash[:])
	require.Equal(ErrNumAccounts, errors.Cause(err))
	require.Nil(signature)
}
