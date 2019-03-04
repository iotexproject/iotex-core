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

	"github.com/iotexproject/iotex-core/pkg/keypair"
)

func TestAddress(t *testing.T) {
	runTest := func(t *testing.T) {
		sk, err := keypair.GenerateKey()
		require.NoError(t, err)

		pkHash := sk.PublicKey().Hash()
		addr1, err := _v1.FromBytes(pkHash)
		require.NoError(t, err)
		assert.Equal(t, pkHash, addr1.Bytes())

		encodedAddr := addr1.String()
		if isTestNet {
			require.True(t, strings.HasPrefix(encodedAddr, TestnetPrefix))
		} else {
			require.True(t, strings.HasPrefix(encodedAddr, MainnetPrefix))
		}
		addr2, err := _v1.FromString(encodedAddr)
		require.NoError(t, err)
		assert.Equal(t, pkHash[:], addr2.Bytes())

		addrBytes := addr1.Bytes()
		require.Equal(t, _v1.AddressLength, len(addrBytes))
		addr3, err := _v1.FromBytes(addrBytes)
		require.NoError(t, err)
		assert.Equal(t, pkHash[:], addr3.Bytes())
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

	sk, err := keypair.GenerateKey()
	require.NoError(t, err)

	addr1, err := _v1.FromBytes(sk.PublicKey().Hash())
	require.NoError(t, err)

	encodedAddr := addr1.String()
	encodedAddrBytes := []byte(encodedAddr)
	encodedAddrBytes[len(encodedAddrBytes)-1] = 'o'
	addr2, err := _v1.FromString(string(encodedAddrBytes))
	assert.Nil(t, addr2)
	assert.Error(t, err)
}
