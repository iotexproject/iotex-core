// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package keypair

import (
	"crypto/ecdsa"
	"encoding/hex"

	"github.com/CoderZhi/go-ethereum/crypto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/hash"
)

const (
	secp256pubKeyLength  = 65
	secp256privKeyLength = 32
)

// DecodePublicKey decodes a string to SECP256K1 PublicKey
func DecodePublicKey(pubKey string) (*ecdsa.PublicKey, error) {
	pkBytes, err := hex.DecodeString(pubKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode public key %s", pubKey)
	}
	pk, err := BytesToPublicKey(pkBytes)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal public key %s", pubKey)
	}
	return pk, nil
}

// DecodePrivateKey decodes a string to SECP256K1 PrivateKey
func DecodePrivateKey(priKey string) (*ecdsa.PrivateKey, error) {
	return crypto.HexToECDSA(priKey)
}

// EncodePublicKey encodes a SECP256K1 PublicKey to string
func EncodePublicKey(pubKey *ecdsa.PublicKey) string {
	return hex.EncodeToString(PublicKeyToBytes(pubKey))
}

// EncodePrivateKey encodes a SECP256K1 PrivateKey to string
func EncodePrivateKey(priKey *ecdsa.PrivateKey) string {
	return hex.EncodeToString(PrivateKeyToBytes(priKey))
}

// BytesToPublicKey converts a byte slice to SECP256K1 PublicKey
func BytesToPublicKey(pubKey []byte) (*ecdsa.PublicKey, error) {
	return crypto.UnmarshalPubkey(pubKey)
}

// BytesToPrivateKey converts a byte slice to SECP256K1 PrivateKey
func BytesToPrivateKey(priKey []byte) (*ecdsa.PrivateKey, error) {
	return crypto.ToECDSA(priKey)
}

// PublicKeyToBytes converts a SECP256K1 PublicKey to byte slice
func PublicKeyToBytes(pubKey *ecdsa.PublicKey) []byte {
	return crypto.FromECDSAPub(pubKey)
}

// PrivateKeyToBytes converts a SECP256K1 PrivateKey to byte slice
func PrivateKeyToBytes(priKey *ecdsa.PrivateKey) []byte {
	return crypto.FromECDSA(priKey)
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
func HashPubKey(pubKey *ecdsa.PublicKey) hash.PKHash {
	var pkHash hash.PKHash
	copy(pkHash[:], hash.Hash160b(PublicKeyToBytes(pubKey)))
	return pkHash
}
