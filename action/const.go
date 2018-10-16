// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import "github.com/pkg/errors"

const (
	// VersionSizeInBytes defines the size of version in byte units
	VersionSizeInBytes = 4
	// NonceSizeInBytes defines the size of nonce in byte units
	NonceSizeInBytes = 8
	// TimestampSizeInBytes defines the size of 8-byte timestamp
	TimestampSizeInBytes = 8
	// BooleanSizeInBytes defines the size of booleans
	BooleanSizeInBytes = 1
	// GasSizeInBytes defines the size of gas in byte uints
	GasSizeInBytes = 8
	// GasLimit is the total gas limit to be consumed in a block
	GasLimit = uint64(1000000000)
)

var (
	// ErrHitGasLimit is the error when hit gas limit
	ErrHitGasLimit = errors.New("Hit Gas Limit")
	// ErrInsufficientBalanceForGas is the error that the balance in executor account is lower than gas
	ErrInsufficientBalanceForGas = errors.New("Insufficient balance for gas")
	// ErrOutOfGas is the error when running out of gas
	ErrOutOfGas = errors.New("Out of gas")
)
