// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
)

// compoundErrorIsChainDetermined reports whether every node can derive the
// failure from the same chain state. Such failures may safely fall back to a
// direct credit. Unknown read/write failures remain fatal because degrading a
// node-local infrastructure error could fork consensus.
func compoundErrorIsChainDetermined(err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, staking.ErrCompoundSelfStakeRoleChanged),
		errors.Is(err, staking.ErrCompoundBucketOwnerMismatch),
		errors.Is(err, action.ErrInvalidAmount):
		return true
	}
	var receiptErr staking.ReceiptError
	if !errors.As(err, &receiptErr) {
		return false
	}
	status := receiptErr.ReceiptStatus()
	return status != uint64(iotextypes.ReceiptStatus_Failure) &&
		status != uint64(iotextypes.ReceiptStatus_Success)
}

// voterChunkSettleableError marks a consensus-determined dispatcher failure
// that Handle may turn into a Failure receipt. Unmarked scan and state errors
// must fail the block so node-local failures cannot advance the cursor.
type voterChunkSettleableError struct{ error }

func settleableVoterChunkError(format string, args ...interface{}) error {
	return &voterChunkSettleableError{errors.Errorf(format, args...)}
}

func voterChunkErrorIsSettleable(err error) bool {
	var target *voterChunkSettleableError
	return errors.As(err, &target)
}
