// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package address

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/address/bech32"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

const (
	// AddressLength indicates the byte length of an address
	AddressLength = 25
	// Version indicates the version number
	Version = 1
)

// Address is V1 address format to be used on IoTeX blockchain and subchains. It is composed of parts in the
// following order:
// 1. uint32: chain ID
// 2. 20 bytes: hash derived from the the public key
type Address struct {
	chainID uint32
	pkHash  hash.PKHash
}

// New constructs an address struct
func New(chainID uint32, pkHash hash.PKHash) *Address {
	return &Address{
		chainID: chainID,
		pkHash:  pkHash,
	}
}

// Bech32ToAddress decodes an encoded address string into an address struct
func Bech32ToAddress(encodedAddr string) (*Address, error) {
	payload, err := decodeBech32(encodedAddr)
	if err != nil {
		return nil, err
	}
	return BytesToAddress(payload)
}

// BytesToAddress converts a byte array into an address struct
func BytesToAddress(bytes []byte) (*Address, error) {
	if len(bytes) != AddressLength {
		return nil, errors.Wrapf(address.ErrInvalidAddr, "invalid address length in bytes: %d", len(bytes))
	}
	if version := bytes[4]; version != Version {
		return nil, errors.Wrapf(
			address.ErrInvalidAddr,
			"the address represented by the bytes is of version %d",
			version,
		)
	}
	var pkHash [hash.PKHashSize]byte
	copy(pkHash[:], bytes[5:])
	return &Address{
		chainID: enc.MachineEndian.Uint32(bytes[:4]),
		pkHash:  pkHash,
	}, nil
}

// IotxAddressToAddress converts an old address string into an address struct
// This method is used for backward compatibility
func IotxAddressToAddress(iotxRawAddr string) (*Address, error) {
	payload, err := decodeBech32(iotxRawAddr)
	if err != nil {
		return nil, err
	}
	if len(payload) != AddressLength {
		return nil, errors.Wrapf(address.ErrInvalidAddr, "invalid address length in bytes: %d", len(payload))
	}
	var pkHash [hash.PKHashSize]byte
	copy(pkHash[:], payload[5:])
	return &Address{
		chainID: enc.MachineEndian.Uint32(payload[1:5]),
		pkHash:  pkHash,
	}, nil
}

// Bech32 encodes an address struct into a a Bech32 encoded address string
// The encoded address string will start with "io" for mainnet, and with "it" for testnet
func (addr *Address) Bech32() string {
	var chainIDBytes [4]byte
	enc.MachineEndian.PutUint32(chainIDBytes[:], addr.chainID)
	payload := append(chainIDBytes[:], append([]byte{Version}, addr.pkHash[:]...)...)
	// Group the payload into 5 bit groups.
	grouped, err := bech32.ConvertBits(payload, 8, 5, true)
	if err != nil {
		logger.Error().Err(err).Msg("error when grouping the payload into 5 bit groups")
		return ""
	}
	encodedAddr, err := bech32.Encode(address.Prefix(), grouped)
	if err != nil {
		logger.Error().Err(err).Msg("error when encoding bytes into a base32 string")
		return ""
	}
	return encodedAddr
}

// Bytes converts an address struct into a byte array
func (addr *Address) Bytes() []byte {
	var chainIDBytes [4]byte
	enc.MachineEndian.PutUint32(chainIDBytes[:], addr.chainID)
	return append(chainIDBytes[:], append([]byte{1}, addr.pkHash[:]...)...)
}

// IotxAddress converts an address struct into an old address string
// This method is used for backward compatibility
func (addr *Address) IotxAddress() string {
	var chainIDBytes [4]byte
	enc.MachineEndian.PutUint32(chainIDBytes[:], addr.chainID)
	iotxAddr, err := iotxaddress.GetAddressByHash(address.IsTestNet(), chainIDBytes[:], addr.pkHash[:])
	if err != nil {
		logger.Error().Err(err).Msg("error when converting address to the iotex address")
		return ""
	}
	return iotxAddr.RawAddress
}

// ChainID returns the chain ID
func (addr *Address) ChainID() uint32 {
	return addr.chainID
}

// Version returns the address version
func (addr *Address) Version() uint8 {
	return Version
}

// Payload returns the payload, which is the public key hash
func (addr *Address) Payload() []byte {
	return addr.pkHash[:]
}

// PublicKeyHash returns the public key hash
func (addr *Address) PublicKeyHash() hash.PKHash {
	return addr.pkHash
}

func decodeBech32(encodedAddr string) ([]byte, error) {
	hrp, grouped, err := bech32.Decode(encodedAddr)
	if hrp != address.Prefix() {
		return nil, errors.Wrapf(err, "hrp %s and address prefix %s don't match", hrp, address.Prefix())
	}
	// Group the payload into 8 bit groups.
	payload, err := bech32.ConvertBits(grouped[:], 5, 8, false)
	if err != nil {
		return nil, errors.Wrapf(err, "error when converting 5 bit groups into the payload")
	}
	return payload, nil
}
