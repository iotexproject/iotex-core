package staking

import (
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
)

// VoteReviser is used to recalculate candidate votes.
type VoteReviser struct {
	reviseHeights []uint64
	cache         map[uint64]CandidateList
	c             genesis.VoteWeightCalConsts
}

// NewVoteReviser creates a VoteReviser.
func NewVoteReviser(c genesis.VoteWeightCalConsts, reviseHeights ...uint64) *VoteReviser {
	return &VoteReviser{
		reviseHeights: reviseHeights,
		cache:         make(map[uint64]CandidateList),
		c:             c,
	}
}

// Revise recalculate candidate votes on preset revising height.
func (vr *VoteReviser) Revise(csm CandidateStateManager, height uint64) error {
	if !vr.needRevise(height) {
		return nil
	}

	if !vr.isCacheExist(height) {
		cands, err := vr.calculateVoteWeight(csm)
		if err != nil {
			return err
		}
		vr.storeToCache(height, cands)
	}
	return vr.flush(height, csm)
}

func (vr *VoteReviser) storeToCache(height uint64, cands CandidateList) {
	vr.cache[height] = cands
}

func (vr *VoteReviser) isCacheExist(height uint64) bool {
	_, ok := vr.cache[height]
	return ok
}

func (vr *VoteReviser) needRevise(height uint64) bool {
	for _, h := range vr.reviseHeights {
		if height == h {
			return true
		}
	}
	return false
}

func (vr *VoteReviser) calculateVoteWeight(sm protocol.StateManager) (CandidateList, error) {
	cands, _, err := getAllCandidates(sm)
	if err != nil {
		return nil, err
	}
	candm := make(map[string]*Candidate)
	selfStakingBuckets := make(map[uint64]bool)
	for _, cand := range cands {
		candm[cand.Owner.String()] = cand.Clone()
		candm[cand.Owner.String()].Votes = new(big.Int)
		candm[cand.Owner.String()].SelfStake = new(big.Int)
		selfStakingBuckets[cand.SelfStakeBucketIdx] = true
	}
	buckets, _, err := getAllBuckets(sm)
	if err != nil {
		return nil, err
	}

	for _, bucket := range buckets {
		if bucket.UnstakeStartTime.Unix() != 0 && bucket.UnstakeStartTime.After(bucket.StakeStartTime) {
			continue
		}
		cand, ok := candm[bucket.Candidate.String()]
		if !ok {
			return nil, errors.Errorf("invalid candidate %s of bucket %d", bucket.Candidate.String(), bucket.Index)
		}

		cand.AddVote(calculateVoteWeight(vr.c, bucket, selfStakingBuckets[bucket.Index]))
	}

	cands = make(CandidateList, 0, len(candm))
	for _, cand := range candm {
		cands = append(cands, cand)
	}
	return cands, nil
}

func (vr *VoteReviser) flush(height uint64, csm CandidateStateManager) error {
	cands, ok := vr.cache[height]
	if !ok {
		return nil
	}
	for _, cand := range cands {
		if err := csm.Upsert(cand); err != nil {
			return err
		}
	}
	return nil
}
