// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package validator

import (
	"errors"

	"github.com/iotexproject/iotex-core/address"
)

// Errors
var (
	// ErrInvalidAddr indicates error for an invalid address
	ErrInvalidAddr = errors.New("invalid IoTeX address")
	// ErrLongAlias indicates error for a long alias more than 40 characters
	ErrLongAlias = errors.New("invalid long alias that is more than 40 characters")
	// ErrNonPositiveNumber indicates error for a non-positive number
	ErrNonPositiveNumber = errors.New("invalid number that is not positive")
)

const (
	// IoAddrLen defines length of IoTeX address
	IoAddrLen = 41
)

// ValidateAddress validates IoTeX address
func ValidateAddress(addr string) error {
	if _, err := address.FromString(addr); err != nil {
		return ErrInvalidAddr
	}
	return nil
}

// ValidateAlias validates alias for account
func ValidateAlias(alias string) error {
	if len(alias) > 40 {
		return ErrLongAlias
	}
	return nil
}

// ValidatePositiveNumber validates positive Number for action
func ValidatePositiveNumber(number int64) error {
	if number <= 0 {
		return ErrNonPositiveNumber
	}
	return nil
}
