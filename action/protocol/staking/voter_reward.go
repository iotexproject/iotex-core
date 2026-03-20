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
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

const (
	// HandleSetCommissionRate is the handler name
	HandleSetCommissionRate = "setCommissionRate"
)

// ActiveBucketsByCandidate returns all non-unstaked buckets (native + contract staking)
// for a candidate. Used by the rewarding protocol (IIP-59) to distribute voter rewards.
func (p *Protocol) ActiveBucketsByCandidate(sm protocol.StateManager, candidateIdentifier address.Address) ([]*VoteBucket, error) {
	csm, err := NewCandidateStateManager(sm)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create candidate state manager")
	}

	// 1. Read native buckets
	var indices BucketIndices
	if _, err := csm.SM().State(&indices,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(append([]byte{_candIndex}, candidateIdentifier.Bytes()...)),
	); err != nil {
		indices = nil // no native buckets
	}

	active := make([]*VoteBucket, 0)
	for _, idx := range indices {
		bucket, err := csm.NativeBucket(idx)
		if err != nil {
			continue
		}
		if bucket.isUnstaked() {
			continue
		}
		active = append(active, bucket)
	}

	// 2. Read contract staking buckets (V1, V2, V3)
	height, _ := sm.Height()
	for _, indexer := range p.contractStakingIndexers() {
		if indexer == nil {
			continue
		}
		contractBuckets, err := indexer.BucketsByCandidate(candidateIdentifier, height)
		if err != nil {
			continue // indexer may not be ready
		}
		for _, b := range contractBuckets {
			if b.isUnstaked() {
				continue
			}
			active = append(active, b)
		}
	}

	return active, nil
}

// contractStakingIndexers returns all configured contract staking indexers
func (p *Protocol) contractStakingIndexers() []ContractStakingIndexer {
	var indexers []ContractStakingIndexer
	if p.contractStakingIndexer != nil {
		indexers = append(indexers, p.contractStakingIndexer)
	}
	if p.contractStakingIndexerV2 != nil {
		indexers = append(indexers, p.contractStakingIndexerV2)
	}
	if p.contractStakingIndexerV3 != nil {
		indexers = append(indexers, p.contractStakingIndexerV3)
	}
	return indexers
}

// CandidateByIdentifier returns a candidate by its identifier address.
// Used by the rewarding protocol (IIP-59) to look up commission rate.
func (p *Protocol) CandidateByIdentifier(sm protocol.StateManager, id address.Address) (*Candidate, error) {
	csm, err := NewCandidateStateManager(sm)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create candidate state manager")
	}

	cand := csm.GetByIdentifier(id)
	if cand == nil {
		return nil, errors.Errorf("candidate %s not found", id.String())
	}
	return cand, nil
}

// VoteWeightCalConsts returns the vote weight calculation constants from this protocol's config.
// Used by the rewarding protocol (IIP-59) to calculate voter shares consistently.
func (p *Protocol) VoteWeightCalConsts() genesis.VoteWeightCalConsts {
	return p.config.VoteWeightCalConsts
}

// handleSetCommissionRate handles the SetCommissionRate action (IIP-59)
func (p *Protocol) handleSetCommissionRate(ctx context.Context, act *action.SetCommissionRate, csm CandidateStateManager,
) (*receiptLog, error) {
	actCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	log := newReceiptLog(p.addr.String(), HandleSetCommissionRate, featureCtx.NewStakingReceiptFormat)

	// Gate behind Yosemite hard fork
	if !featureCtx.EnableVoterRewardDistribution {
		return log, errors.New("SetCommissionRate not enabled before Yosemite hard fork")
	}

	// Only candidate owner can set commission rate
	c := csm.GetByOwner(actCtx.Caller)
	if c == nil {
		return log, errCandNotExist
	}

	// Enforce cooldown: must wait CommissionRateCooldownEpochs between changes
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	currentEpoch := rp.GetEpochNum(blkCtx.BlockHeight)
	if c.CommissionRateLastEpoch > 0 && currentEpoch < c.CommissionRateLastEpoch+action.CommissionRateCooldownEpochs {
		return log, errors.Errorf(
			"commission rate cooldown: last changed at epoch %d, current epoch %d, must wait until epoch %d",
			c.CommissionRateLastEpoch, currentEpoch, c.CommissionRateLastEpoch+action.CommissionRateCooldownEpochs,
		)
	}

	c.CommissionRate = act.Rate()
	c.CommissionRateLastEpoch = currentEpoch

	if err := csm.Upsert(c); err != nil {
		return log, csmErrorToHandleError(c.GetIdentifier().String(), err)
	}

	log.AddTopics(c.GetIdentifier().Bytes())
	log.AddAddress(actCtx.Caller)
	return log, nil
}
