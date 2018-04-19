// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package crypto

import (
	"crypto/rand"

	"golang.org/x/crypto/ed25519"
)

// NewKeyPair wraps ed25519.GenerateKey() for now.
func NewKeyPair() ([]byte, []byte, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return pub, priv, nil
}

// Sign wraps ed25519.Sign() for now.
func Sign(priv []byte, msg []byte) []byte {
	p := ed25519.PrivateKey(priv)
	return ed25519.Sign(p, msg)
}

// Verify wraps ed25519.Verify(0 for now.
func Verify(pub []byte, msg, sig []byte) bool {
	p := ed25519.PublicKey(pub)
	return ed25519.Verify(p, msg, sig)
}
