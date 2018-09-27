// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package address

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestAddress(t *testing.T) {
	runTest := func(t *testing.T) {
		pk, _, err := crypto.EC283.NewKeyPair()
		require.NoError(t, err)

		pkHash := keypair.HashPubKey(pk)

		assertAddr := func(t *testing.T, addr *AddrV1) {
			assert.Equal(t, uint32(1024), addr.ChainID())
			assert.Equal(t, uint8(1), addr.Version())
			assert.Equal(t, pkHash[:], addr.Payload())
			assert.Equal(t, pkHash, addr.PublicKeyHash())
		}

		addr1 := V1.New(1024, pkHash)
		assertAddr(t, addr1)

		encodedAddr := addr1.Bech32()
		if isTestNet {
			require.True(t, strings.HasPrefix(encodedAddr, TestnetPrefix))
		} else {
			require.True(t, strings.HasPrefix(encodedAddr, MainnetPrefix))
		}
		addr2, err := V1.Bech32ToAddress(encodedAddr)
		require.NoError(t, err)
		assertAddr(t, addr2)

		addrBytes := addr1.Bytes()
		require.Equal(t, V1.AddressLength, len(addrBytes))
		addr3, err := V1.BytesToAddress(addrBytes)
		require.NoError(t, err)
		assertAddr(t, addr3)
	}
	t.Run("testnet", func(t *testing.T) {
		require.NoError(t, os.Setenv("IOTEX_NETWORK_TYPE", "testnet"))
		runTest(t)
	})
	t.Run("mainnet", func(t *testing.T) {
		require.NoError(t, os.Setenv("IOTEX_NETWORK_TYPE", "mainnet"))
		runTest(t)
	})
}

func TestAddressError(t *testing.T) {
	t.Parallel()

	pk, _, err := crypto.EC283.NewKeyPair()
	require.NoError(t, err)

	pkHash := keypair.HashPubKey(pk)
	addr1 := V1.New(1024, pkHash)
	require.NoError(t, err)

	encodedAddr := addr1.Bech32()
	encodedAddrBytes := []byte(encodedAddr)
	encodedAddrBytes[len(encodedAddrBytes)-1] = 'o'
	addr2, err := V1.Bech32ToAddress(string(encodedAddrBytes))
	assert.Nil(t, addr2)
	assert.Error(t, err)
}

func TestConvertFromAndToIotxAddress(t *testing.T) {
	t.Parallel()

	iotxAddr1 := testaddress.Addrinfo["producer"]
	addr, err := V1.IotxAddressToAddress(iotxAddr1.RawAddress)
	require.NoError(t, err)
	iotxAddr2 := addr.IotxAddress()
	assert.Equal(t, iotxAddr1.RawAddress, iotxAddr2)
}
