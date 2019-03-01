// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package keypair

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/hash"
)

type (
	// secp256k1PrvKey implements the SECP256K1 private key
	secp256k1PrvKey struct {
		*ecdsa.PrivateKey
	}
	// secp256k1PubKey implements the SECP256K1 public key
	secp256k1PubKey struct {
		*ecdsa.PublicKey
	}
)

//======================================
// PrivateKey function
//======================================

// NewSecp256k1PrvKey generates a new SECP256K1 private key
func NewSecp256k1PrvKey() (PrivateKey, error) {
	sk, err := crypto.GenerateKey()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create secp256k1 private key")
	}
	return &secp256k1PrvKey{
		PrivateKey: sk,
	}, nil
}

// NewSecp256k1PrvKeyFromBytes converts bytes format to PrivateKey
func NewSecp256k1PrvKeyFromBytes(b []byte) (CryptoKey, error) {
	sk, err := crypto.ToECDSA(b)
	if err != nil {
		return nil, err
	}
	return &secp256k1PrvKey{
		PrivateKey: sk,
	}, nil
}

// PrvKeyBytes returns the private key in bytes representation
func (k *secp256k1PrvKey) PrvKeyBytes() []byte {
	return crypto.FromECDSA(k.PrivateKey)
}

// PubKey returns the public key corresponding to private key
func (k *secp256k1PrvKey) PubKey() PublicKey {
	return &secp256k1PubKey{
		PublicKey: &k.PublicKey,
	}
}

// Sign signs the message/hash
func (k *secp256k1PrvKey) Sign(hash []byte) ([]byte, error) {
	return crypto.Sign(hash, k.PrivateKey)
}

// PubKeyBytes returns the public key in bytes representation
func (k *secp256k1PrvKey) PubKeyBytes() []byte {
	return crypto.FromECDSAPub(&k.PublicKey)
}

// PubKeyHash is the hash of the public key
func (k *secp256k1PrvKey) PubKeyHash() []byte {
	h := hash.Hash160b(k.PubKeyBytes())
	return h[:]
}

// Verify verifies the signature
func (k *secp256k1PrvKey) Verify(hash, sig []byte) bool {
	return crypto.VerifySignature(k.PubKeyBytes(), hash, sig[:len(sig)-1])
}

//======================================
// PublicKey function
//======================================

// NewSecp256k1PubKeyFromBytes converts bytes format to PublicKey
func NewSecp256k1PubKeyFromBytes(b []byte) (PublicKey, error) {
	pk, err := crypto.UnmarshalPubkey(b)
	if err != nil {
		return nil, err
	}
	return &secp256k1PubKey{
		PublicKey: pk,
	}, nil
}

// PubKeyBytes returns the public key in bytes representation
func (k *secp256k1PubKey) PubKeyBytes() []byte {
	return crypto.FromECDSAPub(k.PublicKey)
}

// PubKeyHash is the hash of the public key
func (k *secp256k1PubKey) PubKeyHash() []byte {
	h := hash.Hash160b(k.PubKeyBytes())
	return h[:]
}

// Verify verifies the signature
func (k *secp256k1PubKey) Verify(hash, sig []byte) bool {
	return crypto.VerifySignature(k.PubKeyBytes(), hash, sig[:len(sig)-1])
}
