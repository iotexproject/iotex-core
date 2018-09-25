// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package address

import (
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/address/bech32"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

// init reads IOTEX_NETWORK_TYPE environment variable. If it exists and the value is equal to "testnet" with case
// ignored,  the global variable isTestNet is set to true for the whole runtime
func init() {
	isTestNet = strings.EqualFold(os.Getenv("IOTEX_NETWORK_TYPE"), "testnet")
}

// AddressLength indicates the byte length of an address
const AddressLength = 25

var (
	isTestNet     bool
	mainnetPrefix = "io"
	testnetPrefix = "it"

	// ErrInvalidAddr indicates the invalid address error
	ErrInvalidAddr = errors.New("Invalid address")
)

// Address is the new address format to be used on IoTeX blockchain and subchains. It is composed of parts in the
// following order:
// 1. uint32: chain ID
// 2. uint8: version of address
// 3. 20 bytes: hash derived from the the public key
type Address struct {
	chainID uint32
	version uint8
	pkHash  [hash.PKHashSize]byte
}

// New instantiates an address struct
func New(chainID uint32, version uint8, pkHash [hash.PKHashSize]byte) Address {
	return Address{
		chainID: chainID,
		version: version,
		pkHash:  pkHash,
	}
}

// Bech32ToAddress decodes an encoded address string into an address struct
func Bech32ToAddress(encodedAddr string) (Address, error) {
	payload, err := decodeBech32(encodedAddr)
	if err != nil {
		return Address{}, err
	}
	return BytesToAddress(payload)
}

// BytesToAddress converts a byte array into an address struct
func BytesToAddress(bytes [AddressLength]byte) (Address, error) {
	var pkHash [hash.PKHashSize]byte
	copy(pkHash[:], bytes[5:])
	return Address{
		chainID: enc.MachineEndian.Uint32(bytes[:4]),
		version: uint8(bytes[4]),
		pkHash:  pkHash,
	}, nil
}

// IotxAddressToAddress converts an old address string into an address struct
// This method is used for backward compatibility
func IotxAddressToAddress(iotxRawAddr string) (Address, error) {
	payload, err := decodeBech32(iotxRawAddr)
	if err != nil {
		return Address{}, err
	}
	var pkHash [hash.PKHashSize]byte
	copy(pkHash[:], payload[5:])
	return Address{
		chainID: enc.MachineEndian.Uint32(payload[1:5]),
		version: uint8(payload[0]),
		pkHash:  pkHash,
	}, nil
}

// Bech32 encodes an address struct into a a Bech32 encoded address string
// The encoded address string will start with "io" for mainnet, and with "it" for testnet
func (addr *Address) Bech32() string {
	var chainIDBytes [4]byte
	enc.MachineEndian.PutUint32(chainIDBytes[:], addr.chainID)
	payload := append(chainIDBytes[:], append([]byte{addr.version}, addr.pkHash[:]...)...)
	// Group the payload into 5 bit groups.
	grouped, err := bech32.ConvertBits(payload, 8, 5, true)
	if err != nil {
		logger.Error().Err(err).Msg("error when grouping the payload into 5 bit groups")
		return ""
	}
	encodedAddr, err := bech32.Encode(addrPrefix(), grouped)
	if err != nil {
		logger.Error().Err(err).Msg("error when encoding bytes into a base32 string")
		return ""
	}
	return encodedAddr
}

// Bytes converts an address struct into a byte array
func (addr *Address) Bytes() [AddressLength]byte {
	var chainIDBytes [4]byte
	enc.MachineEndian.PutUint32(chainIDBytes[:], addr.chainID)
	addrBytesSlice := append(chainIDBytes[:], append([]byte{addr.version}, addr.pkHash[:]...)...)
	var addrBytes [AddressLength]byte
	copy(addrBytes[:], addrBytesSlice)
	return addrBytes
}

// IotxAddress converts an address struct into an old address string
// This method is used for backward compatibility
func (addr *Address) IotxAddress() string {
	var chainIDBytes [4]byte
	enc.MachineEndian.PutUint32(chainIDBytes[:], addr.chainID)
	iotxAddr, err := iotxaddress.GetAddressByHash(isTestNet, chainIDBytes[:], addr.pkHash[:])
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
	return addr.version
}

// PublicKeyHash returns the public key hash
func (addr *Address) PublicKeyHash() [hash.PKHashSize]byte {
	return addr.pkHash
}

func decodeBech32(encodedAddr string) ([AddressLength]byte, error) {
	var addrBytes [AddressLength]byte
	hrp, grouped, err := bech32.Decode(encodedAddr)
	if hrp != addrPrefix() {
		return addrBytes, errors.Wrapf(err, "hrp %s and address prefix %s don't match", hrp, addrPrefix())
	}
	// Group the payload into 8 bit groups.
	payload, err := bech32.ConvertBits(grouped[:], 5, 8, false)
	if err != nil {
		return addrBytes, errors.Wrapf(err, "error when converting 5 bit groups into the payload")
	}
	if len(payload) != AddressLength {
		return addrBytes, errors.Wrapf(ErrInvalidAddr, "invalid address length in bytes: %d", len(payload))
	}
	copy(addrBytes[:], payload)
	return addrBytes, nil
}

func addrPrefix() string {
	prefix := mainnetPrefix
	if isTestNet {
		prefix = testnetPrefix
	}
	return prefix
}
