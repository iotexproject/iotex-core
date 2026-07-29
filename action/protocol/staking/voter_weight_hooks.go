// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
)

type contractBucketVoterWeightObserver struct {
	csm                 CandidateStateManager
	calculateVoteWeight CalculateVoteWeightFunc
	height              uint64
}

func newContractBucketVoterWeightObserver(
	sm protocol.StateManager,
	calculateVoteWeight CalculateVoteWeightFunc,
	height uint64,
) (*contractBucketVoterWeightObserver, error) {
	csm, err := NewCandidateStateManager(sm)
	if err != nil {
		return nil, err
	}
	return &contractBucketVoterWeightObserver{
		csm: csm, calculateVoteWeight: calculateVoteWeight, height: height,
	}, nil
}

func (o *contractBucketVoterWeightObserver) PutContractBucket(previous, current *contractstaking.Bucket) {
	o.apply(previous, true)
	o.apply(current, false)
}

func (o *contractBucketVoterWeightObserver) DeleteContractBucket(previous *contractstaking.Bucket) {
	o.apply(previous, true)
}

func (o *contractBucketVoterWeightObserver) ReviseContractBucket(bucket *contractstaking.Bucket) {
	if o == nil || bucket == nil || o.csm == nil || o.calculateVoteWeight == nil || o.height == 0 {
		return
	}
	previous := o.calculateVoteWeight(bucket, o.height-1)
	current := o.calculateVoteWeight(bucket, o.height)
	o.applyWeight(bucket, new(big.Int).Sub(current, previous))
}

func (o *contractBucketVoterWeightObserver) apply(bucket *contractstaking.Bucket, subtract bool) {
	if o == nil || bucket == nil || o.csm == nil || o.calculateVoteWeight == nil {
		return
	}
	weight := o.calculateVoteWeight(bucket, o.height)
	if subtract {
		weight.Neg(weight)
	}
	o.applyWeight(bucket, weight)
}

func (o *contractBucketVoterWeightObserver) applyWeight(bucket *contractstaking.Bucket, weight *big.Int) {
	if weight == nil || weight.Sign() == 0 {
		return
	}
	candidate := o.csm.GetByIdentifier(bucket.Candidate)
	if candidate == nil {
		return
	}
	applyVoterWeightDelta(o.csm, candidate.GetIdentifier(), bucket.Owner, weight)
}

// applyVoterWeightDelta is the single entry point every staking handler uses
// to keep the IIP-59 VoterWeightView in sync with on-chain bucket changes.
// No-op when the view has not been installed (pre-fork / test setups that skip
// Protocol.Start) and when delta is zero, so callers can wire it next to any
// existing candidate.AddVote / candidate.SubVote site without first checking
// the fork flag.
//
// candIdentifier must be the candidate's identifier address (not operator) —
// same key the view uses internally. voter is the bucket owner.
func applyVoterWeightDelta(csm CandidateStateManager, candIdentifier address.Address, voter address.Address, delta *big.Int) {
	if delta == nil || delta.Sign() == 0 {
		return
	}
	if csm == nil || candIdentifier == nil || voter == nil {
		return
	}
	view := csm.DirtyView()
	if view == nil || view.voterWeights == nil {
		return
	}
	view.voterWeights.Apply(hash.BytesToHash160(candIdentifier.Bytes()), voter, delta)
}
