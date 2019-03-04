// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package keypair

import (
	"crypto/ecdsa"
	"encoding/hex"

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

// newSecp256k1PrvKey generates a new SECP256K1 private key
func newSecp256k1PrvKey() (PrivateKey, error) {
	sk, err := crypto.GenerateKey()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create secp256k1 private key")
	}
	return &secp256k1PrvKey{
		PrivateKey: sk,
	}, nil
}

// newSecp256k1PrvKeyFromBytes converts bytes format to PrivateKey
func newSecp256k1PrvKeyFromBytes(b []byte) (PrivateKey, error) {
	sk, err := crypto.ToECDSA(b)
	if err != nil {
		return nil, err
	}
	return &secp256k1PrvKey{
		PrivateKey: sk,
	}, nil
}

// Bytes returns the private key in bytes representation
func (k *secp256k1PrvKey) Bytes() []byte {
	return crypto.FromECDSA(k.PrivateKey)
}

// HexString returns the private key in hex string
func (k *secp256k1PrvKey) HexString() string {
	return hex.EncodeToString(k.Bytes())
}

// EcdsaPrivateKey returns the embedded ecdsa private key
func (k *secp256k1PrvKey) EcdsaPrivateKey() *ecdsa.PrivateKey {
	return k.PrivateKey
}

// PublicKey returns the public key corresponding to private key
func (k *secp256k1PrvKey) PublicKey() PublicKey {
	return &secp256k1PubKey{
		PublicKey: &k.PrivateKey.PublicKey,
	}
}

// Sign signs the message/hash
func (k *secp256k1PrvKey) Sign(hash []byte) ([]byte, error) {
	return crypto.Sign(hash, k.PrivateKey)
}

// Zero zeroes the private key data
func (k *secp256k1PrvKey) Zero() {
	b := k.D.Bits()
	for i := range b {
		b[i] = 0
	}
}

//======================================
// PublicKey function
//======================================

// newSecp256k1PubKeyFromBytes converts bytes format to PublicKey
func newSecp256k1PubKeyFromBytes(b []byte) (PublicKey, error) {
	pk, err := crypto.UnmarshalPubkey(b)
	if err != nil {
		return nil, err
	}
	return &secp256k1PubKey{
		PublicKey: pk,
	}, nil
}

// Bytes returns the public key in bytes representation
func (k *secp256k1PubKey) Bytes() []byte {
	return crypto.FromECDSAPub(k.PublicKey)
}

// HexString returns the public key in hex string
func (k *secp256k1PubKey) HexString() string {
	return hex.EncodeToString(k.Bytes())
}

// EcdsaPublicKey returns the embedded ecdsa publick key
func (k *secp256k1PubKey) EcdsaPublicKey() *ecdsa.PublicKey {
	return k.PublicKey
}

// Hash is the last 20-byte of keccak hash of public key bytes, same as Ethereum address generation
func (k *secp256k1PubKey) Hash() []byte {
	h := hash.Hash160b(k.Bytes()[1:])
	return h[:]
}

// Verify verifies the signature
func (k *secp256k1PubKey) Verify(hash, sig []byte) bool {
	if len(sig) != secp256pubKeyLength {
		return false
	}
	// signature must be in the [R || S || V] format where V is 0 or 1
	v := sig[secp256pubKeyLength-1]
	if !(v == 0 || v == 1) {
		return false
	}
	return crypto.VerifySignature(k.Bytes(), hash, sig[:len(sig)-1])
}
