// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

// handleSetCommissionRate handles IIP-59's SetCommissionRate action.
//
// Semantics:
//   - Only the candidate's owner may set the rate.
//   - The new rate is stored on the staking candidate immediately. It only
//     takes effect at distribution time after the next PutPollResult
//     snapshots it into the per-epoch state.Candidate, which gives voters
//     ~1.5 epochs of reaction time. No separate cooldown field is needed —
//     IIP-59 doesn't prescribe one.
//   - The rate-range check happens at Validate (validateSetCommissionRate)
//     so out-of-range values never reach the handler.
//   - A non-owner caller returns a handleError so the tx receipt is marked
//     failed; block production continues.
func (p *Protocol) handleSetCommissionRate(
	ctx context.Context,
	act *action.SetCommissionRate,
	csm CandidateStateManager,
) (*receiptLog, error) {
	actCtx := protocol.MustGetActionCtx(ctx)
	rLog := newReceiptLog(p.addr.String())

	cand := csm.GetByOwner(actCtx.Caller)
	if cand == nil {
		return rLog, errCandNotExist
	}

	cand.CommissionRate = act.Rate()
	if err := csm.Upsert(cand); err != nil {
		return rLog, csmErrorToHandleError(cand.GetIdentifier().String(), err)
	}

	topics, eventData, err := action.PackCommissionRateSetEvent(cand.GetIdentifier(), act.Rate())
	if err != nil {
		return rLog, err
	}
	rLog.AddEvent(topics, eventData)
	return rLog, nil
}

