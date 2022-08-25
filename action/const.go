// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import "github.com/pkg/errors"

// vars
var (
	ErrAddress            = errors.New("invalid address")
	ErrVotee              = errors.New("votee is not a candidate")
	ErrNotFound           = errors.New("action not found")
	ErrChainID            = errors.New("invalid chainID")
	ErrExistedInPool      = errors.New("known transaction")
	ErrReplaceUnderpriced = errors.New("replacement transaction underpriced")
	ErrSystemActionNonce  = errors.New("invalid system action nonce")
	ErrNonceTooLow        = errors.New("nonce too low")
	ErrUnderpriced        = errors.New("transaction underpriced")
	ErrNegativeValue      = errors.New("negative value")
	ErrIntrinsicGas       = errors.New("intrinsic gas too low")
	ErrInsufficientFunds  = errors.New("insufficient funds for gas * price + value")
	ErrNonceTooHigh       = errors.New("nonce too high")
	ErrInvalidSender      = errors.New("invalid sender")
	ErrTxPoolOverflow     = errors.New("txpool is full")
	ErrGasLimit           = errors.New("exceeds block gas limit")
	ErrOversizedData      = errors.New("oversized data")
	ErrNilProto           = errors.New("empty action proto to load")
	ErrNilAction          = errors.New("nil action to load proto")
	ErrInvalidAct         = errors.New("invalid action type")
	ErrInvalidABI         = errors.New("invalid abi binary data")
)

// LoadErrorDescription loads corresponding description related to the error
func LoadErrorDescription(err error) string {
	switch errors.Cause(err) {
	case ErrOversizedData, ErrTxPoolOverflow, ErrInvalidSender, ErrNonceTooHigh, ErrInsufficientFunds, ErrIntrinsicGas, ErrChainID, ErrNotFound, ErrVotee, ErrAddress, ErrExistedInPool, ErrReplaceUnderpriced, ErrNonceTooLow, ErrUnderpriced, ErrNegativeValue:
		return err.Error()
	default:
		return "Unknown"
	}
}
