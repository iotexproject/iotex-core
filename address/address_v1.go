// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package address

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/address/bech32"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// _v1 is a singleton and defines V1 address metadata
var _v1 = v1{
	AddressLength: 20,
}

type v1 struct {
	// AddressLength indicates the byte length of an address
	AddressLength int
}

// FromString decodes an encoded address string into an address struct
func (v *v1) FromString(encodedAddr string) (*AddrV1, error) {
	payload, err := v.decodeBech32(encodedAddr)
	if err != nil {
		return nil, err
	}
	return v.FromBytes(payload)
}

// FromBytes converts a byte array into an address struct
func (v *v1) FromBytes(bytes []byte) (*AddrV1, error) {
	if len(bytes) != v.AddressLength {
		return nil, errors.Wrapf(ErrInvalidAddr, "invalid address length in bytes: %d", len(bytes))
	}
	return &AddrV1{
		payload: hash.BytesToHash160(bytes),
	}, nil
}

func (v *v1) decodeBech32(encodedAddr string) ([]byte, error) {
	hrp, grouped, err := bech32.Decode(encodedAddr)
	if hrp != prefix() {
		return nil, errors.Wrapf(err, "hrp %s and address prefix %s don't match", hrp, prefix())
	}
	// Group the payload into 8 bit groups.
	payload, err := bech32.ConvertBits(grouped, 5, 8, false)
	if err != nil {
		return nil, errors.Wrapf(err, "error when converting 5 bit groups into the payload")
	}
	return payload, nil
}

// AddrV1 is V1 address format to be used on IoTeX blockchain and subchains. It is composed of
// 20 bytes: hash derived from the the public key:
type AddrV1 struct {
	payload hash.Hash160
}

// String encodes an address struct into a a String encoded address string
// The encoded address string will start with "io" for mainnet, and with "it" for testnet
func (addr *AddrV1) String() string {
	payload := addr.payload[:]
	// Group the payload into 5 bit groups.
	grouped, err := bech32.ConvertBits(payload, 8, 5, true)
	if err != nil {
		log.L().Panic("Error when grouping the payload into 5 bit groups.", zap.Error(err))
		return ""
	}
	encodedAddr, err := bech32.Encode(prefix(), grouped)
	if err != nil {
		log.L().Panic("Error when encoding bytes into a base32 string.", zap.Error(err))
		return ""
	}
	return encodedAddr
}

// Bytes converts an address struct into a byte array
func (addr *AddrV1) Bytes() []byte {
	return addr.payload[:]
}
