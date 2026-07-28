// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

func (p *Protocol) handleSetVoterRewardOptIn(
	ctx context.Context, act *action.SetVoterRewardOptIn, csm CandidateStateManager,
) (*receiptLog, []*action.TransactionLog, error) {
	actCtx := protocol.MustGetActionCtx(ctx)
	id, err := address.FromBytes(act.CandidateIdentifier())
	if err != nil {
		return nil, nil, &handleError{
			err: errors.Wrap(err, "invalid candidate identifier"), failureStatus: iotextypes.ReceiptStatus_ErrCandidateNotExist,
		}
	}
	cand := csm.GetByIdentifier(id)
	if cand == nil {
		return nil, nil, errCandNotExist
	}
	if !address.Equal(actCtx.Caller, cand.Owner) {
		return nil, nil, &handleError{
			err: errors.New("caller is not the candidate owner"), failureStatus: iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		}
	}
	cand.VoterRewardOnchainOptIn = true
	if err := csm.Upsert(cand); err != nil {
		return nil, nil, csmErrorToHandleError(cand.GetIdentifier().String(), err)
	}
	topics, eventData, err := action.PackVoterRewardOptInSetEvent(cand.GetIdentifier().Bytes(), true)
	if err != nil {
		return nil, nil, err
	}
	rLog := newReceiptLog(p.addr.String())
	rLog.AddEvent(topics, eventData)
	return rLog, nil, nil
}
