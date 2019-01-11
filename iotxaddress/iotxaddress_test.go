// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package iotxaddress

import (
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/address/bech32"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
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
	log.S().Info("Generated address ", addr.RawAddress)
	log.S().Info("Generated public key ", hex.EncodeToString(addr.PublicKey[:]))
	log.S().Info("Generated private key ", hex.EncodeToString(addr.PrivateKey[:]))
	p2pkh := keypair.HashPubKey(addr.PublicKey)
	p2pkh1, err := GetPubkeyHash(addr.RawAddress)
	require.NoError(err)
	require.Equal(p2pkh[:], p2pkh1)
	log.S().Info("P2PKH ", hex.EncodeToString(p2pkh[:]))

	p2pkhSlice, err := hex.DecodeString("36500e9520e13d02bea26a08e99b6e7145fa6c10")
	require.NoError(err)
	copy(p2pkh[:], p2pkhSlice)
	addr1, err := GetAddressByHash(true, []byte{0x00, 0x00, 0x00, 0x01}, p2pkhSlice)
	require.Nil(err)
	p2pkh1, err = GetPubkeyHash(addr1.RawAddress)
	require.NoError(err)
	require.Equal(p2pkh[:], p2pkh1)

	rmsg := make([]byte, 2048)
	_, err = rand.Read(rmsg)
	require.NoError(err)
	sig := crypto.EC283.Sign(addr.PrivateKey, rmsg)
	require.True(crypto.EC283.Verify(addr.PublicKey, rmsg, sig))
}

func TestInvalidAddress(t *testing.T) {
	require := require.New(t)
	chainid := []byte{0x00, 0x00, 0x00, 0x01}
	addr, err := NewAddress(true, chainid)
	require.Nil(err)

	pub, pri, err := crypto.EC283.NewKeyPair()
	require.Nil(err)
	require.NotEqual(keypair.ZeroPublicKey, pub)
	require.NotEqual(keypair.ZeroPrivateKey, pri)
	addr.PrivateKey = pri

	// test invalid prefix
	pkHash := keypair.HashPubKey(pub)
	payload := append([]byte{version.ProtocolVersion}, append(chainid, pkHash[:]...)...)
	grouped, err := bech32.ConvertBits(payload, 8, 5, true)
	require.Nil(err)
	wrongPrefix := "ix"
	raddr, err := bech32.Encode(wrongPrefix, grouped)
	require.NotNil(raddr)
	require.Nil(err)
	require.Nil(GetPubkeyHash(raddr))
	_, err = GetPubkeyHash(raddr)
	require.Error(err)

	// test invalid version
	payload = append([]byte{0}, append(chainid, pkHash[:]...)...)
	grouped, err = bech32.ConvertBits(payload, 8, 5, true)
	require.Nil(err)
	raddr, err = bech32.Encode(mainnetPrefix, grouped)
	require.NotNil(raddr)
	require.Nil(err)
	require.Nil(GetPubkeyHash(raddr))
	_, err = GetPubkeyHash(raddr)
	require.Error(err)
}
