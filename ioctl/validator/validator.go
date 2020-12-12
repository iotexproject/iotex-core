// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package validator

import (
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"
)

// Errors
var (
	// ErrInvalidAddr indicates error for an invalid address
	ErrInvalidAddr = errors.New("invalid IoTeX address")
	// ErrLongAlias indicates error for a long alias more than 40 characters
	ErrLongAlias = errors.New("invalid long alias that is more than 40 characters")
	// ErrNonPositiveNumber indicates error for a non-positive number
	ErrNonPositiveNumber = errors.New("invalid number that is not positive")
	// ErrInvalidStakeDuration indicates error for invalid stake duration
	ErrInvalidStakeDuration = errors.New("stake duration must be within 0 and 1050 and in multiples of 7")
	// ErrInvalidCandidateName indicates error for invalid candidate name
	ErrLongCandidateName = errors.New("invalid length of candidate name that is more than 12 ")
	// ErrInvalidCandidateName indicates error for invalid candidate name (for ioctl stake2 command)
	ErrStake2CandidateName = errors.New("the candidate name string is not valid")
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

// ValidateStakeDuration validates stake duration for native staking
func ValidateStakeDuration(stakeDuration *big.Int) error {
	stakeDurationInt := stakeDuration.Int64()
	if stakeDurationInt%7 != 0 || stakeDurationInt < 0 || stakeDurationInt > 1050 {
		return ErrInvalidStakeDuration
	}

	return nil
}

// ValidateCandidateNameForStake2 validates candidate name for native staking 2
func ValidateCandidateNameForStake2(candidateName string) error {
	if len(candidateName) == 0 || len(candidateName) > 12 {
		return ErrStake2CandidateName
	}
	for _, c := range candidateName {
		if !(('a' <= c && c <= 'z') || ('0' <= c && c <= '9')) {
			return ErrStake2CandidateName
		}
	}
	return nil
}
