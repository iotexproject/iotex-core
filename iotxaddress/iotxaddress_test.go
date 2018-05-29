// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package iotxaddress

import (
	"crypto/rand"
	"strings"
	"testing"

	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/stretchr/testify/assert"
)

// TestNewAddress tests create new asset address.
func TestNewAddress(t *testing.T) {
	assert := assert.New(t)
	addr, err := NewAddress(true, []byte{0x00, 0x00, 0x00, 0x01})
	assert.Nil(err)
	assert.NotNil(addr.PrivateKey)
	assert.NotNil(addr.PublicKey)
	assert.NotEqual("", addr.RawAddress)

	t.Log("Generated address is ", addr.RawAddress)
	t.Logf("Generated public key = %x", addr.PublicKey)
	t.Logf("Generated private key = %x", addr.PrivateKey)

	p2pkh := HashPubKey(addr.PublicKey)
	if assert.Equal(p2pkh, GetPubkeyHash(addr.RawAddress)) {
		t.Logf("P2PKH = %x", p2pkh)
	}

	rmsg := make([]byte, 2048)
	rand.Read(rmsg)

	sig := cp.Sign(addr.PrivateKey, rmsg)
	assert.True(cp.Verify(addr.PublicKey, rmsg, sig))
}

// TestGetAddress tests get address for a given public key and params.
func TestGetandValidateAddress(t *testing.T) {
	assert := assert.New(t)
	pub, _, err := cp.NewKeyPair()
	assert.Nil(err)

	addr, err := GetAddress(pub, false, []byte{0x00, 0x00, 0x00, 0x01})
	assert.Nil(err)
	t.Log(addr)
	assert.True(strings.HasPrefix(addr.RawAddress, mainnetPrefix))
	assert.True(ValidateAddress(addr.RawAddress))
	addrstr := strings.Replace(addr.RawAddress, "1", "?", -1)
	assert.False(ValidateAddress(addrstr))

	addr, err = GetAddress(pub, true, []byte{0x00, 0x00, 0x00, 0x01})
	assert.Nil(err)
	t.Log(addr)
	assert.True(strings.HasPrefix(addr.RawAddress, testnetPrefix))
}
