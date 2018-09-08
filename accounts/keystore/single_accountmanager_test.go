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

	"github.com/facebookgo/clock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

func TestSingleAccountManager_SignTransfer(t *testing.T) {
	require := require.New(t)

	key := &Key{PublicKey: pubKey1, PrivateKey: priKey1, RawAddress: rawAddr1}
	keyBytes, err := json.Marshal(key)
	require.NoError(err)

	accountManager := NewMemAccountManager()
	accountManager.Import(keyBytes)

	m, err := NewSingleAccountManager(accountManager)
	require.NoError(err)

	rawTransfer, err := action.NewTransfer(uint64(1), big.NewInt(1), rawAddr1, rawAddr2, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	signedTransfer, err := m.SignTransfer(rawTransfer)
	require.NoError(err)
	require.NotNil(signedTransfer)

	require.NoError(accountManager.Remove(rawAddr1))
	signedTransfer, err = m.SignTransfer(rawTransfer)
	require.Equal(ErrNumAccounts, errors.Cause(err))
	require.Nil(signedTransfer)
}

func TestSingleAccountManager_SignVote(t *testing.T) {
	require := require.New(t)

	key := &Key{PublicKey: pubKey1, PrivateKey: priKey1, RawAddress: rawAddr1}
	keyBytes, err := json.Marshal(key)
	require.NoError(err)

	accountManager := NewMemAccountManager()
	accountManager.Import(keyBytes)

	m, err := NewSingleAccountManager(accountManager)
	require.NoError(err)

	selfPubKey, err := keypair.DecodePublicKey(pubKey1)
	require.NoError(err)
	votePubKey, err := keypair.DecodePublicKey(pubKey2)
	require.NoError(err)
	voterAddress, err := iotxaddress.GetAddressByPubkey(iotxaddress.IsTestnet, iotxaddress.ChainID, selfPubKey)
	require.NoError(err)
	voteeAddress, err := iotxaddress.GetAddressByPubkey(iotxaddress.IsTestnet, iotxaddress.ChainID, votePubKey)
	require.NoError(err)
	rawVote, err := action.NewVote(uint64(1), voterAddress.RawAddress, voteeAddress.RawAddress, uint64(100000), big.NewInt(10))
	require.NoError(err)
	signedVote, err := m.SignVote(rawVote)
	require.NoError(err)
	require.NotNil(signedVote)

	require.NoError(accountManager.Remove(rawAddr1))
	signedVote, err = m.SignVote(rawVote)
	require.Equal(ErrNumAccounts, errors.Cause(err))
	require.Nil(signedVote)
}

func TestSingleAccountManager_SignHash(t *testing.T) {
	require := require.New(t)

	key := &Key{PublicKey: pubKey1, PrivateKey: priKey1, RawAddress: rawAddr1}
	keyBytes, err := json.Marshal(key)
	require.NoError(err)

	accountManager := NewMemAccountManager()
	accountManager.Import(keyBytes)

	m, err := NewSingleAccountManager(accountManager)
	require.NoError(err)

	blk := blockchain.NewBlock(1, 0, hash.ZeroHash32B, clock.New(), nil, nil, nil)
	hash := blk.HashBlock()
	signature, err := m.SignHash(hash[:])
	require.NoError(err)
	require.NotNil(signature)

	require.NoError(accountManager.Remove(rawAddr1))
	signature, err = m.SignHash(hash[:])
	require.Equal(ErrNumAccounts, errors.Cause(err))
	require.Nil(signature)
}
