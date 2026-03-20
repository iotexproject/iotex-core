// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/state"
)

const (
	// HandleSetCommissionRate is the handler name
	HandleSetCommissionRate = "setCommissionRate"
)

// ActiveBucketsByCandidate returns all non-unstaked native buckets for a candidate.
// Used by the rewarding protocol (IIP-59) to distribute voter rewards.
func (p *Protocol) ActiveBucketsByCandidate(sr protocol.StateReader, candidateIdentifier address.Address) ([]*VoteBucket, error) {
	view, err := sr.ReadView(p.Name())
	if err != nil {
		return nil, nil // staking view not available
	}
	csr, ok := view.(CandidateStateReader)
	if !ok {
		return nil, nil // invalid view type
	}

	indices, _, err := csr.NativeBucketIndicesByCandidate(candidateIdentifier)
	if errors.Cause(err) == state.ErrStateNotExist || indices == nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	buckets, err := csr.NativeBucketsWithIndices(*indices)
	if err != nil {
		return nil, err
	}

	// Filter out unstaked buckets
	active := make([]*VoteBucket, 0, len(buckets))
	for _, b := range buckets {
		if !b.UnstakeStartTime.IsZero() && b.UnstakeStartTime.Unix() > 0 {
			continue // bucket has been unstaked
		}
		active = append(active, b)
	}
	return active, nil
}

// CandidateByIdentifier returns a candidate by its identifier address.
// Used by the rewarding protocol (IIP-59) to look up commission rate.
func (p *Protocol) CandidateByIdentifier(sr protocol.StateReader, id address.Address) (*Candidate, error) {
	view, err := sr.ReadView(p.Name())
	if err != nil {
		return nil, errors.Wrap(err, "failed to read staking view")
	}
	csr, ok := view.(CandidateStateReader)
	if !ok {
		return nil, errors.New("invalid staking view type")
	}

	cand, _, err := csr.CandidateByAddress(id)
	if err != nil {
		return nil, err
	}
	return cand, nil
}

// handleSetCommissionRate handles the SetCommissionRate action (IIP-59)
func (p *Protocol) handleSetCommissionRate(ctx context.Context, act *action.SetCommissionRate, csm CandidateStateManager,
) (*receiptLog, error) {
	actCtx := protocol.MustGetActionCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	log := newReceiptLog(p.addr.String(), HandleSetCommissionRate, featureCtx.NewStakingReceiptFormat)

	// Only candidate owner can set commission rate
	c := csm.GetByOwner(actCtx.Caller)
	if c == nil {
		return log, errCandNotExist
	}

	c.CommissionRate = act.Rate()

	if err := csm.Upsert(c); err != nil {
		return log, csmErrorToHandleError(c.GetIdentifier().String(), err)
	}

	log.AddTopics(c.GetIdentifier().Bytes())
	log.AddAddress(actCtx.Caller)
	return log, nil
}
