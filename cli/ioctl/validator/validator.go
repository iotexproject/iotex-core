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
	// ErrLongName indicates error for a long name more than 40 characters
	ErrLongName = errors.New("invalid long name that is more than 40 characters")
	// ErrMinusAmount indicates error for an monus amount
	ErrMinusAmount = errors.New("invalid amount that is minus")
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

// ValidateName validates name for account
func ValidateName(name string) error {
	if len(name) > 40 {
		return ErrLongName
	}
	return nil
}

// ValidateAmount validates amount for action
func ValidateAmount(amount int64) error {
	if amount < 0 {
		return ErrMinusAmount
	}
	return nil
}
