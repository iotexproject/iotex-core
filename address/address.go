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

var (
	isTestNet bool
	// ErrInvalidAddr indicates the invalid address error
	ErrInvalidAddr = errors.New("Invalid address")
)

// Address defines the interface of the blockchain address
type Address interface {
	ChainID() uint32
	Version() uint8
	Payload() []byte
	Bech32() string
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
