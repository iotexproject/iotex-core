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

func (p *Protocol) handleSetVoterRewardOptIn(
	ctx context.Context, _ *action.SetVoterRewardOptIn, csm CandidateStateManager,
) (*receiptLog, []*action.TransactionLog, error) {
	actCtx := protocol.MustGetActionCtx(ctx)
	cand := csm.GetByOwner(actCtx.Caller)
	if cand == nil {
		return nil, nil, errCandNotExist
	}
	cand.VoterRewardOnchainOptIn = true
	if err := csm.Upsert(cand); err != nil {
		return nil, nil, csmErrorToHandleError(cand.GetIdentifier().String(), err)
	}
	rLog := newReceiptLog(p.addr.String())
	rLog.AddEvent(action.VoterRewardOptInSetEvent(cand.GetIdentifier().Bytes()), nil)
	return rLog, nil, nil
}
