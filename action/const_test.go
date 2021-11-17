// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadErrorDescription(t *testing.T) {
	request := require.New(t)
	tests := []struct {
		err error
		ret string
	}{
		{ErrOversizedData, ErrOversizedData.Error()},
		{ErrTxPoolOverflow, ErrTxPoolOverflow.Error()},
		{ErrInvalidSender, ErrInvalidSender.Error()},
		{ErrNonceTooHigh, ErrNonceTooHigh.Error()},
		{ErrInsufficientFunds, ErrInsufficientFunds.Error()},
		{ErrIntrinsicGas, ErrIntrinsicGas.Error()},
		{ErrChainID, ErrChainID.Error()},
		{ErrNotFound, ErrNotFound.Error()},
		{ErrVotee, ErrVotee.Error()},
		{ErrAddress, ErrAddress.Error()},
		{ErrExistedInPool, ErrExistedInPool.Error()},
		{ErrReplaceUnderpriced, ErrReplaceUnderpriced.Error()},
		{ErrNonceTooLow, ErrNonceTooLow.Error()},
		{ErrUnderpriced, ErrUnderpriced.Error()},
		{ErrNonceTooHigh, ErrNonceTooHigh.Error()},
		{ErrAddress, ErrAddress.Error()},
		{ErrNegativeValue, ErrNegativeValue.Error()},
		{ErrNilAction, "Unknown"},
		{ErrEmptyActionPool, "Unknown"},
	}
	for _, e := range tests {
		request.Equal(e.ret, LoadErrorDescription(e.err))
	}
}
