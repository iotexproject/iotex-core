// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package keypair

import (
	"encoding/hex"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/hash"
)

const (
	secp256pubKeyLength = 65
	secp256prvKeyLength = 32
)

var (
	// ErrInvalidKey is the error that the key format is invalid
	ErrInvalidKey = errors.New("invalid key format")
)

type (
	// PublicKey represents a public key
	PublicKey interface {
		PubKeyBytes() []byte
		PubKeyHash() []byte
		Verify([]byte, []byte) bool
	}
	// PrivateKey represents a private key
	PrivateKey interface {
		PrvKeyBytes() []byte
		PubKey() PublicKey
		Sign([]byte) ([]byte, error)
	}
	// CryptoKey represents a unique key pair
	CryptoKey interface {
		PublicKey
		PrivateKey
	}
)

// GenerateKey generates a SECP256K1 PrivateKey
func GenerateKey() (PrivateKey, error) {
	return NewSecp256k1PrvKey()
}

// DecodePublicKey decodes a string to SECP256K1 PublicKey
func DecodePublicKey(pubKey string) (PublicKey, error) {
	b, err := hex.DecodeString(pubKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode public key %s", pubKey)
	}
	return BytesToPublicKey(b)
}

// DecodePrivateKey decodes a string to SECP256K1 PrivateKey
func DecodePrivateKey(prvKey string) (PrivateKey, error) {
	b, err := hex.DecodeString(prvKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode public key %s", prvKey)
	}
	return BytesToPrivateKey(b)
}

// EncodePublicKey encodes a SECP256K1 PublicKey to string
func EncodePublicKey(pubKey PublicKey) string {
	return hex.EncodeToString(pubKey.PubKeyBytes())
}

// EncodePrivateKey encodes a SECP256K1 PrivateKey to string
func EncodePrivateKey(priKey PrivateKey) string {
	return hex.EncodeToString(priKey.PrvKeyBytes())
}

// BytesToPublicKey converts a byte slice to SECP256K1 PublicKey
func BytesToPublicKey(pubKey []byte) (PublicKey, error) {
	return NewSecp256k1PubKeyFromBytes(pubKey)
}

// BytesToPrivateKey converts a byte slice to SECP256K1 PrivateKey
func BytesToPrivateKey(prvKey []byte) (PrivateKey, error) {
	return NewSecp256k1PrvKeyFromBytes(prvKey)
}

// StringToPubKeyBytes converts a string of public key to byte slice
func StringToPubKeyBytes(pubKey string) ([]byte, error) {
	pubKeyBytes, err := hex.DecodeString(pubKey)
	if err != nil {
		return nil, err
	}
	if len(pubKeyBytes) != secp256pubKeyLength {
		return nil, errors.Wrap(ErrPublicKey, "Invalid public key length")
	}
	return pubKeyBytes, nil
}

// HashPubKey returns the hash of SECP256 public key
func HashPubKey(pubKey PublicKey) hash.Hash160 {
	pkBytes := pubKey.PubKeyBytes()
	if pkBytes == nil {
		return hash.ZeroHash160
	}
	return hash.Hash160b(pkBytes[1:])
}
