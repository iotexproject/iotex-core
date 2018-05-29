// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

/*
Address format to be used on IoTeX blockchains is composed of:
-- A prefix indicating the network on which this address is valid, i.e., "io" for mainnet, "it" for testnet and regtest
-- A separator, always `1`
-- A base32 encoded payload indicating the destination of the address and containing a checksum:
---- 1 byte:  version byte, starting with 0x01; The most significant bit is reserved and must be 0
---- 4 bytes: chain identifier: 0x00000001 for the root chain and the remaining for subchains
---- 160-bit hash, derived from the the public key
---- checksum
*/

package iotxaddress

import (
	"errors"

	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/common"
	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress/bech32"
)

var (
	// ErrInvalidVersion is returned when invalid version has been detected.
	ErrInvalidVersion = errors.New("invalid version")
	// ErrInvalidChainID is returned when invalid chain ID has been detected.
	ErrInvalidChainID = errors.New("invalid chain ID")
)

const (
	mainnetPrefix = "io"
	testnetPrefix = "it"
)

// Address contains a pair of key and a string address
type Address struct {
	PrivateKey []byte
	PublicKey  []byte
	RawAddress string
}

// NewAddress returns a newly created public/private key pair together with the address derived.
func NewAddress(isTestnet bool, chainid []byte) (*Address, error) {
	pub, pri, err := cp.NewKeyPair()
	if err != nil {
		return nil, err
	}

	addr, err := GetAddress(pub, isTestnet, chainid)
	if err != nil {
		return nil, err
	}

	addr.PrivateKey = pri
	return addr, nil
}

// GetAddress returns the address given a public key and necessary params.
func GetAddress(pub []byte, isTestnet bool, chainid []byte) (*Address, error) {
	if !isValidChainID(chainid) {
		return nil, ErrInvalidChainID
	}

	hrp := mainnetPrefix
	if isTestnet {
		hrp = testnetPrefix
	}

	payload := append([]byte{common.ProtocolVersion}, append(chainid, HashPubKey(pub)...)...)
	// Group the payload into 5 bit groups.
	grouped, err := bech32.ConvertBits(payload, 8, 5, true)
	if err != nil {
		return nil, err
	}
	raddr, err := bech32.Encode(hrp, grouped)
	if err != nil {
		return nil, err
	}
	return &Address{PublicKey: pub, RawAddress: raddr}, nil
}

// GetPubkeyHash extracts public key hash from address
func GetPubkeyHash(address string) []byte {
	hrp, grouped, err := bech32.Decode(address)
	if err != nil {
		return nil
	}

	// Exclude the separator, version and chainID
	payload, err := bech32.ConvertBits(grouped[:], 5, 8, false)
	if err != nil {
		return nil
	}

	if hrp != mainnetPrefix && hrp != testnetPrefix {
		return nil
	}
	if !isValidVersion(payload[0]) {
		return nil
	}
	if !isValidChainID(payload[1:5]) {
		return nil
	}

	return payload[5:25]
}

// ValidateAddress check if address if valid.
func ValidateAddress(address string) bool {
	hrp, grouped, err := bech32.Decode(address)
	if err != nil {
		return false
	}

	// Exclude the separator, version and chainID
	payload, err := bech32.ConvertBits(grouped[:], 5, 8, false)
	if err != nil {
		return false
	}

	if hrp != mainnetPrefix && hrp != testnetPrefix {
		return false
	}
	if !isValidVersion(payload[0]) {
		return false
	}
	if !isValidChainID(payload[1:5]) {
		return false
	}
	return true
}

// HashPubKey returns the hash of public key
func HashPubKey(pubKey []byte) []byte {
	// use Blake2b algorithm
	digest := blake2b.Sum256(pubKey)
	return digest[7:27]
}

func isValidVersion(version byte) bool {
	if version >= 0x01 {
		return true
	}
	return false
}

func isValidChainID(chainid []byte) bool {
	if len(chainid) != 4 {
		return false
	}
	return true
}
