// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import "github.com/pkg/errors"

var (
	// ErrActPool indicates the error of actpool
	ErrActPool = errors.New("invalid actpool")
	// ErrHitGasLimit is the error when hit gas limit
	ErrHitGasLimit = errors.New("Hit Gas Limit")
	// ErrInsufficientBalanceForGas is the error that the balance in executor account is lower than gas
	ErrInsufficientBalanceForGas = errors.New("Insufficient balance for gas")
	// ErrOutOfGas is the error when running out of gas
	ErrOutOfGas = errors.New("Out of gas")
	// ErrGasHigherThanLimit indicates the error of gas value
	ErrGasHigherThanLimit = errors.New("invalid gas for action")
	// ErrTransfer indicates the error of transfer
	ErrTransfer = errors.New("invalid transfer")
	// ErrNonce indicates the error of nonce
	ErrNonce = errors.New("invalid nonce")
	// ErrBalance indicates the error of balance
	ErrBalance = errors.New("invalid balance")
	// ErrVotee indicates the error of votee
	ErrVotee = errors.New("votee is not a candidate")
	// ErrHash indicates the error of action's hash
	ErrHash = errors.New("invalid hash")
)
