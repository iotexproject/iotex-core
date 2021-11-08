// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import "github.com/pkg/errors"

var (
	// ErrAddress indicates error of address
	ErrAddress = errors.New("invalid address")
	// ErrVotee indicates the error of votee
	ErrVotee = errors.New("votee is not a candidate")
	// ErrNotFound indicates the nonexistence of action
	ErrNotFound = errors.New("action not found")
	// ErrChainID indicates the error of chainID
	ErrChainID = errors.New("invalid chainID")
	// ErrExistedInPool indicates the action already exists in the actpool
	ErrExistedInPool = errors.New("known transaction")
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
	// ErrIntrinsicGas is returned if the transaction is specified to use less gas
	// than required to start the invocation.
	ErrIntrinsicGas = errors.New("intrinsic gas too low")
	// ErrInsufficientFunds is returned if the total cost of executing a transaction
	// is higher than the balance of the user's account.
	ErrInsufficientFunds = errors.New("insufficient funds for gas * price + value")
	// ErrNonceTooHigh is returned if the nonce of a transaction is higher than the
	// next one expected based on the local chain.
	ErrNonceTooHigh = errors.New("nonce too high")
	// ErrInvalidSender is returned if the transaction contains an invalid signature.
	ErrInvalidSender = errors.New("invalid sender")
	// ErrTxPoolOverflow is returned if the transaction pool is full and can't accpet
	// another remote transaction.
	ErrTxPoolOverflow = errors.New("txpool is full")
	// ErrGasLimit is returned if a transaction's requested gas limit exceeds the
	// maximum allowance of the current block.
	ErrGasLimit = errors.New("exceeds block gas limit")
	// ErrOversizedData is returned if the input data of a transaction is greater
	// than some meaningful limit a user might use. This is not a consensus error
	// making the transaction invalid, rather a DOS protection.
	ErrOversizedData = errors.New("oversized data")
)

// LoadErrorDescription loads corresponding description related to the error
func LoadErrorDescription(err error) string {
	switch errors.Cause(err) {
	case ErrOversizedData, ErrTxPoolOverflow, ErrInvalidSender, ErrNonceTooHigh, ErrInsufficientFunds, ErrIntrinsicGas, ErrChainID, ErrNotFound, ErrVotee, ErrAddress, ErrExistedInPool, ErrReplaceUnderpriced, ErrNonceTooLow, ErrUnderpriced, ErrNonceTooHigh, ErrAddress, ErrNegativeValue:
		return err.Error()
	default:
		return "Unknown"
	}
}
