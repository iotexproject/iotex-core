// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package iotxaddress

import (
	"crypto/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress/bech32"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/version"
)

// TestNewAddress tests create new asset address.
func TestNewAddress(t *testing.T) {
	require := require.New(t)
	addr, err := NewAddress(true, []byte{0x00, 0x00, 0x00, 0x01})
	require.Nil(err)
	require.NotNil(addr.PrivateKey)
	require.NotNil(addr.PublicKey)
	require.NotEqual("", addr.RawAddress)

	t.Log("Generated address is ", addr.RawAddress)
	t.Logf("Generated public key = %x", addr.PublicKey)
	t.Logf("Generated private key = %x", addr.PrivateKey)

	p2pkh := HashPubKey(addr.PublicKey)
	require.Equal(p2pkh, GetPubkeyHash(addr.RawAddress))
	t.Logf("P2PKH = %x", p2pkh)

	rmsg := make([]byte, 2048)
	_, err = rand.Read(rmsg)
	require.NoError(err)

	sig := cp.Sign(addr.PrivateKey, rmsg)
	require.True(cp.Verify(addr.PublicKey, rmsg, sig))
}

// TestGetAddress tests get address for a given public key and params.
func TestGetandValidateAddress(t *testing.T) {
	require := require.New(t)
	pub, _, err := cp.NewKeyPair()
	require.Nil(err)

	addr, err := GetAddress(pub, false, []byte{0x00, 0x00, 0x00, 0x01})
	require.Nil(err)
	t.Log(addr)
	require.True(strings.HasPrefix(addr.RawAddress, mainnetPrefix))
	require.True(ValidateAddress(addr.RawAddress))
	addrstr := strings.Replace(addr.RawAddress, "1", "?", -1)
	require.False(ValidateAddress(addrstr))

	addr, err = GetAddress(pub, true, []byte{0x00, 0x00, 0x00, 0x01})
	require.Nil(err)
	t.Log(addr)
	require.True(strings.HasPrefix(addr.RawAddress, testnetPrefix))
}

const wrongPrefix = "ix"

func TestInvalidAddress(t *testing.T) {
	require := require.New(t)
	chainid := []byte{0x00, 0x00, 0x00, 0x01}
	addr, err := NewAddress(true, chainid)

	pub, pri, err := cp.NewKeyPair()
	require.NotEqual(keypair.ZeroPublicKey, pub)
	require.NotEqual(keypair.ZeroPrivateKey, pri)
	require.Nil(err)
	addr.PrivateKey = pri

	// test invalid prefix
	payload := append([]byte{version.ProtocolVersion}, append(chainid, HashPubKey(pub)...)...)
	grouped, err := bech32.ConvertBits(payload, 8, 5, true)
	require.Nil(err)
	raddr, err := bech32.Encode(wrongPrefix, grouped)
	require.NotNil(raddr)
	require.Nil(err)
	require.Nil(GetPubkeyHash(raddr))
	require.Equal(false, ValidateAddress(raddr))

	// test invalid version
	payload = append([]byte{0}, append(chainid, HashPubKey(pub)...)...)
	grouped, err = bech32.ConvertBits(payload, 8, 5, true)
	require.Nil(err)
	raddr, err = bech32.Encode(mainnetPrefix, grouped)
	require.NotNil(raddr)
	require.Nil(err)
	require.Nil(GetPubkeyHash(raddr))
	require.Equal(false, ValidateAddress(raddr))
}
