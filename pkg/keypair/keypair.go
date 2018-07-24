// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package keypair

import (
	"encoding/hex"

	"github.com/pkg/errors"
)

const (
	pubKeyLength  = 72
	privKeyLength = 36
)

var (
	// ZeroPublicKey is an instance of PublicKey consisting of all zeros
	ZeroPublicKey PublicKey
	// ZeroPrivateKey is an instance of PrivateKey consisting of all zeros
	ZeroPrivateKey PrivateKey
	// ErrPublicKey indicates the error of public key
	ErrPublicKey = errors.New("invalid public key")
	// ErrPrivateKey indicates the error of private key
	ErrPrivateKey = errors.New("invalid private key")
)

type (
	// PublicKey indicates the type of 72 byte public key
	PublicKey [pubKeyLength]byte
	// PrivateKey indicates the type 36 byte public key
	PrivateKey [privKeyLength]byte
)

// DecodePublicKey decodes a string to PublicKey
func DecodePublicKey(pubKey string) (PublicKey, error) {
	pubKeyBytes, err := hex.DecodeString(pubKey)
	if err != nil {
		return ZeroPublicKey, err
	}
	return BytesToPublicKey(pubKeyBytes)
}

// DecodePrivateKey decodes a string to PrivateKey
func DecodePrivateKey(priKey string) (PrivateKey, error) {
	priKeyBytes, err := hex.DecodeString(priKey)
	if err != nil {
		return ZeroPrivateKey, err
	}
	return BytesToPrivateKey(priKeyBytes)
}

// EncodePublicKey encodes a PublicKey to string
func EncodePublicKey(pubKey PublicKey) string {
	return hex.EncodeToString(pubKey[:])
}

// EncodePrivateKey encodes a PrivateKey to string
func EncodePrivateKey(priKey PrivateKey) string {
	return hex.EncodeToString(priKey[:])
}

// BytesToPublicKey converts a byte slice to PublicKey
func BytesToPublicKey(pubKey []byte) (PublicKey, error) {
	if len(pubKey) != pubKeyLength {
		return ZeroPublicKey, errors.Wrap(ErrPublicKey, "Invalid public key length")
	}
	var publicKey PublicKey
	copy(publicKey[:], pubKey)
	return publicKey, nil
}

// BytesToPrivateKey converts a byte slice to PrivateKey
func BytesToPrivateKey(priKey []byte) (PrivateKey, error) {
	if len(priKey) != privKeyLength {
		return ZeroPrivateKey, errors.Wrap(ErrPrivateKey, "Invalid private key length")
	}
	var privateKey PrivateKey
	copy(privateKey[:], priKey)
	return privateKey, nil
}

// StringToPubKeyBytes converts a string of public key to byte slice
func StringToPubKeyBytes(pubKey string) ([]byte, error) {
	pubKeyBytes, err := hex.DecodeString(pubKey)
	if err != nil {
		return nil, err
	}
	if len(pubKeyBytes) != pubKeyLength {
		return nil, errors.Wrap(ErrPublicKey, "Invalid public key length")
	}
	return pubKeyBytes, nil
}

// StringToPriKeyBytes converts a string of private key to byte slice
func StringToPriKeyBytes(priKey string) ([]byte, error) {
	priKeyBytes, err := hex.DecodeString(priKey)
	if err != nil {
		return nil, err
	}
	if len(priKeyBytes) != privKeyLength {
		return nil, errors.Wrap(ErrPrivateKey, "Invalid private key length")
	}
	return priKeyBytes, nil
}

// BytesToPubKeyString converts a byte slice of public key to string
func BytesToPubKeyString(pubKey []byte) (string, error) {
	if len(pubKey) != pubKeyLength {
		return "", errors.Wrap(ErrPublicKey, "Invalid public key length")
	}
	return hex.EncodeToString(pubKey), nil
}

// BytesToPriKeyString converts a byte slice of private key to string
func BytesToPriKeyString(priKey []byte) (string, error) {
	if len(priKey) != privKeyLength {
		return "", errors.Wrap(ErrPrivateKey, "Invalid private key length")
	}
	return hex.EncodeToString(priKey), nil
}
