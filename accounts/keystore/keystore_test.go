// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package keystore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

const (
	pubkeyProducer = "336eb60a5741f585a8e81de64e071327a3b96c15af4af5723598a07b6121e8e813bbd0056ba71ae29c0d64252e913f60afaeb11059908b81ff27cbfa327fd371d35f5ec0cbc01705"
	prikeyProducer = "925f0c9e4b6f6d92f2961d01aff6204c44d73c0b9d0da188582932d4fcad0d8ee8c66600"
	pubkeyA        = "2c9ccbeb9ee91271f7e5c2103753be9c9edff847e1a51227df6a6b0765f31a4b424e84027b44a663950f013a88b8fd8cdc53b1eda1d4b73f9d9dc12546c8c87d68ff1435a0f8a006"
	prikeyA        = "b5affb30846a00ef5aa39b57f913d70cd8cf6badd587239863cb67feacf6b9f30c34e800"
)

func TestKeyStore(t *testing.T) {
	testKeyStore := func(ks KeyStore, t *testing.T) {
		require := require.New(t)

		addr1 := testaddress.Addrinfo["producer"]
		encodedAddr1 := addr1.Bech32()
		priKey1, err := keypair.DecodePrivateKey(prikeyProducer)
		require.NoError(err)
		//Test Store
		err = ks.Store("123", priKey1)
		require.Error(err)

		err = ks.Store(encodedAddr1, priKey1)
		require.NoError(err)

		err = ks.Store(encodedAddr1, priKey1)
		require.Equal(ErrExist, errors.Cause(err))

		// Test Get
		_, err = ks.Get("123")
		require.Error(err)

		_, err = ks.Get(testaddress.Addrinfo["bravo"].Bech32())
		require.Equal(ErrNotExist, errors.Cause(err))

		val, err := ks.Get(encodedAddr1)
		require.NoError(err)
		require.Equal(priKey1, val)

		// Test Has
		_, err = ks.Has("123")
		require.Error(err)

		exist, err := ks.Has(encodedAddr1)
		require.NoError(err)
		require.Equal(true, exist)

		// Test Remove
		err = ks.Remove("123")
		require.Error(err)

		err = ks.Remove(testaddress.Addrinfo["bravo"].Bech32())
		require.Equal(ErrNotExist, errors.Cause(err))

		err = ks.Remove(encodedAddr1)
		require.NoError(err)
		exist, err = ks.Has(encodedAddr1)
		require.NoError(err)
		require.Equal(false, exist)

		// Test All
		addr2 := testaddress.Addrinfo["alfa"]
		encodedAddr2 := addr2.Bech32()
		priKey2, err := keypair.DecodePrivateKey(prikeyA)
		require.NoError(err)
		err = ks.Store(encodedAddr1, priKey1)
		require.NoError(err)
		err = ks.Store(encodedAddr2, priKey2)
		require.NoError(err)
		encodedAddrs, err := ks.All()
		require.NoError(err)
		require.Equal(2, len(encodedAddrs))
	}

	t.Run("Plain KeyStore", func(t *testing.T) {
		require := require.New(t)

		ksDir := filepath.Join(os.TempDir(), "keystore")
		os.RemoveAll(ksDir)
		defer func() {
			require.NoError(os.RemoveAll(ksDir))
		}()

		ks, err := NewPlainKeyStore(ksDir)
		require.NoError(err)
		testKeyStore(ks, t)
	})

	t.Run("In-Memory KeyStore", func(t *testing.T) {
		testKeyStore(NewMemKeyStore(), t)
	})
}
