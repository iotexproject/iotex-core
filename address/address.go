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

	"github.com/iotexproject/iotex-core/pkg/hash"
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
var ErrInvalidAddr = errors.New("invalid address")

var isTestNet bool

// Address defines the interface of the blockchain address
type Address interface {
	// String encodes the whole address into an address string encoded in String format
	String() string
	// Bytes serializes the whole address struct into a byte slice, which is composed of the payload to identify an
	// address within one blockchain
	Bytes() []byte
}

// New constructs an address instance
func New(payload []byte) Address {
	var pkHash hash.Hash160
	copy(pkHash[:], payload)
	return V1.New(pkHash)
}

// StringToAddress decodes an encoded address string into an address struct
func StringToAddress(encodedAddr string) (Address, error) { return V1.StringToAddress(encodedAddr) }

// BytesToAddress converts a byte array into an address struct
func BytesToAddress(bytes []byte) (Address, error) { return V1.BytesToAddress(bytes) }

// prefix returns the current prefix
func prefix() string {
	prefix := MainnetPrefix
	if isTestNet {
		prefix = TestnetPrefix
	}
	return prefix
}
