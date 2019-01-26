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

	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

const (
	pubkeyProducer = "04755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf33"
	prikeyProducer = "cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1"
	prikeyA        = "d3e7252d95ecef433bf152e9878f15e1c5072867399d18226fe7f8668618492c"
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
