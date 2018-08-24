// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
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
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress/bech32"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/version"
)

var (
	// ErrInvalidVersion is returned when invalid version has been detected.
	ErrInvalidVersion = errors.New("invalid version")
	// ErrInvalidChainID is returned when invalid chain ID has been detected.
	ErrInvalidChainID = errors.New("invalid chain ID")
	// ErrInvalidHRP is returned when invalid human readable prefix has been detected.
	ErrInvalidHRP = errors.New("invalid human readable prefix")
	// IsTestnet is used to get address
	IsTestnet = false
	// ChainID is used to get address
	ChainID = []byte{0x01, 0x02, 0x03, 0x04}
)

const (
	mainnetPrefix = "io"
	testnetPrefix = "it"
)

// Address contains a pair of key and a string address
type Address struct {
	PrivateKey keypair.PrivateKey
	PublicKey  keypair.PublicKey
	RawAddress string
}

// DKGAddress contains a pair of DKGkey and a DKGID
type DKGAddress struct {
	PrivateKey []uint32
	PublicKey  []byte
	ID         []uint8
}

// NewAddress returns a newly created public/private key pair together with the address derived.
func NewAddress(isTestnet bool, chainID []byte) (*Address, error) {
	pub, pri, err := crypto.EC283.NewKeyPair()
	if err != nil {
		return nil, err
	}

	addr, err := GetAddressByPubkey(isTestnet, chainID, pub)
	if err != nil {
		return nil, err
	}

	addr.PrivateKey = pri
	return addr, nil
}

// GetAddressByHash returns the address given a 20-byte hash
func GetAddressByHash(isTestnet bool, chainID, hash []byte) (*Address, error) {
	raddr, err := getRawAddress(isTestnet, chainID, hash)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting raw address")
	}
	return &Address{RawAddress: raddr}, nil
}

// CreateContractAddress returns the contract address given owner address and nonce
func CreateContractAddress(ownerAddr string, nonce uint64) (string, error) {
	temp := make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, nonce)
	// generate contract address from owner addr and nonce
	// nonce guarantees a different contract addr even if the same code is deployed the second time
	contractHash := hash.Hash160b(append([]byte(ownerAddr), temp...))
	return getRawAddress(IsTestnet, ChainID, contractHash)
}

// GetAddressByPubkey returns the address given a public key and necessary params.
func GetAddressByPubkey(isTestnet bool, chainID []byte, pub keypair.PublicKey) (*Address, error) {
	raddr, err := getRawAddress(isTestnet, chainID, keypair.HashPubKey(pub))
	if err != nil {
		return nil, errors.Wrap(err, "error when getting raw address")
	}
	return &Address{PublicKey: pub, RawAddress: raddr}, nil
}

// GetPubkeyHash extracts public key hash from address
func GetPubkeyHash(address string) ([]byte, error) {
	hrp, grouped, err := bech32.Decode(address)
	if err != nil {
		return nil, errors.Wrap(err, "error when decoding the address in the form of base32 string")
	}
	if !isValidHrp(hrp) {
		return nil, errors.Wrapf(ErrInvalidHRP, "invalid human readable prefix %s", hrp)
	}

	// Exclude the separator, version and chainID
	payload, err := bech32.ConvertBits(grouped[:], 5, 8, false)
	if err != nil {
		return nil, errors.Wrapf(err, "error when converting 5 bit groups into the payload")
	}
	if !isValidVersion(payload[0]) {
		return nil, errors.Wrapf(ErrInvalidVersion, "invalid address version %d", payload[0])
	}
	return payload[5:25], nil
}

func getRawAddress(isTestnet bool, chainID, hash []byte) (string, error) {
	hrp := mainnetPrefix
	if isTestnet {
		hrp = testnetPrefix
	}
	payload := append([]byte{version.ProtocolVersion}, append(chainID, hash...)...)
	// Group the payload into 5 bit groups.
	grouped, err := bech32.ConvertBits(payload, 8, 5, true)
	if err != nil {
		return "", errors.Wrap(err, "error when grouping the payload into 5 bit groups")
	}
	raddr, err := bech32.Encode(hrp, grouped)
	if err != nil {
		return "", errors.Wrap(err, "error when encoding bytes into a base32 string")
	}
	return raddr, nil
}

func isValidHrp(hrp string) bool { return hrp == mainnetPrefix || hrp == testnetPrefix }

func isValidVersion(version byte) bool { return version >= 0x01 }
