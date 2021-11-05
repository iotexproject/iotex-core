// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import "github.com/pkg/errors"

var (
	// ErrAction indicates error for an action
	ErrAction = errors.New("invalid action")
	// ErrAddress indicates error of address
	ErrAddress = errors.New("invalid address")
	// ErrActPool indicates the error of actpool
	ErrActPool = errors.New("invalid actpool")
	// ErrHitGasLimit is the error when hit gas limit
	ErrHitGasLimit = errors.New("Hit gas limit")
	// ErrInsufficientBalanceForGas is the error that the balance in executor account is lower than gas
	ErrInsufficientBalanceForGas = errors.New("Insufficient balance for gas")
	// ErrOutOfGas is the error when running out of gas
	ErrOutOfGas = errors.New("Out of gas")
	// ErrTransfer indicates the error of transfer
	ErrTransfer = errors.New("invalid transfer")
	// ErrNonce indicates the error of nonce
	ErrNonce = errors.New("invalid nonce")
	// ErrBalance indicates the error of balance
	ErrBalance = errors.New("invalid balance")
	// ErrGasPrice indicates the error of gas price
	ErrGasPrice = errors.New("invalid gas price")
	// ErrVotee indicates the error of votee
	ErrVotee = errors.New("votee is not a candidate")
	// ErrNotFound indicates the nonexistence of action
	ErrNotFound = errors.New("action not found")
	// ErrChainID indicates the error of chainID
	ErrChainID = errors.New("invalid chainID")
	// ErrGasTooExpensive indicates there is no insufficient gas space for action
	ErrGasTooExpensive = errors.New("insufficient gas space")
	// ErrExistedInPool indicates the action already exists in the actpool
	ErrExistedInPool = errors.New("already exist in the actpool")
	// ErrReplaceUnderpriced is returned if a transaction is attempted to be replaced
	// with a different one without the required price bump.
	ErrReplaceUnderpriced = errors.New("replacement transaction underpriced")
	// ErrNonceTooLow indicates if the nonce of a transaction is lower than the
	// one present in the local chain.
	ErrNonceTooLow = errors.New("nonce too low")
	// ErrUnderpriced is returned if a transaction's gas price is below the minimum
	// configured for the transaction pool.
	ErrUnderpriced = errors.New("transaction underpriced")
	// ErrNegativeValue is a sanity error to ensure no one is able to specify a
	// transaction with a negative value.
	ErrNegativeValue = errors.New("negative value")
)
