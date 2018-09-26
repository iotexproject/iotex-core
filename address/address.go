// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package address

import (
	"errors"
	"os"
	"strings"
)

// init reads IOTEX_NETWORK_TYPE environment variable. If it exists and the value is equal to "testnet" with case
// ignored,  the global variable isTestNet is set to true for the whole runtime
func init() {
	isTestNet = strings.EqualFold(os.Getenv("IOTEX_NETWORK_TYPE"), "testnet")
}

const (
	// MainnetPrefix is the prefix added to the human readable address of mainnet
	MainnetPrefix = "io"
	// TestnetPrefix is the prefix added to the human readable address of testnet
	TestnetPrefix = "it"
)

// ErrInvalidAddr indicates the invalid address error
var ErrInvalidAddr = errors.New("Invalid address")

var isTestNet bool

// Address defines the interface of the blockchain address
type Address interface {
	// ChainID returns the version
	ChainID() uint32
	// Version returns the version
	Version() uint8
	// Payload returns the payload
	Payload() []byte
	// Bech32 encodes the whole address into an address string encoded in Bech32 format
	Bech32() string
	// Bytes serializes the whole address struct into a byte slice, which is composed of the following parts in order:
	// 1. uint32: chain ID
	// 2. uint8: version of address
	// 3. byte slice: the payload to identify an address within one blockchain
	Bytes() []byte
}

// IsTestNet returns if the current runtime is a testnet or not
func IsTestNet() bool {
	return isTestNet
}

// Prefix returns the current prefix
func Prefix() string {
	prefix := MainnetPrefix
	if isTestNet {
		prefix = TestnetPrefix
	}
	return prefix
}
