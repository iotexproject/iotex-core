package staking

import (
	"math/big"
	"sort"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
)

// VoteReviser is used to recalculate candidate votes.
type VoteReviser struct {
	reviseHeights      []uint64
	cache              map[uint64][2]CandidateList
	c                  genesis.VoteWeightCalConsts
	correctCandsHeight uint64
}

// NewVoteReviser creates a VoteReviser.
func NewVoteReviser(c genesis.VoteWeightCalConsts, correctCandsHeight uint64, reviseHeights ...uint64) *VoteReviser {
	return &VoteReviser{
		reviseHeights:      reviseHeights,
		cache:              make(map[uint64][2]CandidateList),
		c:                  c,
		correctCandsHeight: correctCandsHeight,
	}
}

// Revise recalculate candidate votes on preset revising height.
func (vr *VoteReviser) Revise(csm CandidateStateManager, height uint64) error {
	if !vr.isCacheExist(height) {
		var dirty CandidateList
		cands, err := vr.calculateVoteWeight(csm)
		if err != nil {
			return err
		}
		if vr.needCorrectCands(height) {
			for _, c := range csm.DirtyView().candCenter.base.nameMap {
				dirty = append(dirty, c)
			}
			for _, c := range csm.DirtyView().candCenter.base.operatorMap {
				dirty = append(dirty, c)
			}
			cands, err = vr.correctAliasCands(csm, cands)
			if err != nil {
				return err
			}
		}
		vr.storeToCache(height, dirty, cands)
	}
	return vr.flush(height, csm)
}

func (vr *VoteReviser) correctAliasCands(csm CandidateStateManager, cands CandidateList) (CandidateList, error) {
	ownerMap := map[string]*Candidate{}
	for addr, owner := range csm.DirtyView().candCenter.base.ownerMap {
		ownerMap[addr] = owner
	}
	var retval CandidateList
	for _, c := range cands {
		if owner, ok := ownerMap[c.Owner.String()]; ok {
			c.Operator = owner.Operator
			c.Reward = owner.Reward
			c.Name = owner.Name
		}
		retval = append(retval, c)
	}
	return retval, nil
}

func (vr *VoteReviser) result(height uint64) (CandidateList, bool) {
	cands, ok := vr.cache[height]
	if !ok {
		return nil, false
	}
	return cands[1], true
}

func (vr *VoteReviser) storeToCache(height uint64, dirty, cands CandidateList) {
	vr.cache[height] = [2]CandidateList{dirty, cands}
}

func (vr *VoteReviser) isCacheExist(height uint64) bool {
	_, ok := vr.cache[height]
	return ok
}

// NeedRevise returns true if height needs revise
func (vr *VoteReviser) NeedRevise(height uint64) bool {
	for _, h := range vr.reviseHeights {
		if height == h {
			return true
		}
	}
	return vr.needCorrectCands(height)
}

// NeedCorrectCands returns true if height needs to correct candidates
func (vr *VoteReviser) needCorrectCands(height uint64) bool {
	return height == vr.correctCandsHeight
}

func (vr *VoteReviser) calculateVoteWeight(csm CandidateStateManager) (CandidateList, error) {
	csr := newCandidateStateReader(csm.SM())
	cands, _, err := csr.getAllCandidates()
	switch {
	case errors.Cause(err) == state.ErrStateNotExist:
	case err != nil:
		return nil, err
	}
	candm := make(map[string]*Candidate)
	for _, cand := range cands {
		candm[cand.Owner.String()] = cand.Clone()
		candm[cand.Owner.String()].Votes = new(big.Int)
		candm[cand.Owner.String()].SelfStake = new(big.Int)
	}
	buckets, _, err := csr.getAllBuckets()
	switch {
	case errors.Cause(err) == state.ErrStateNotExist:
	case err != nil:
		return nil, err
	}

	for _, bucket := range buckets {
		if bucket.isUnstaked() {
			continue
		}
		cand, ok := candm[bucket.Candidate.String()]
		if !ok {
			log.L().Error("invalid bucket candidate", zap.Uint64("bucket index", bucket.Index), zap.String("candidate", bucket.Candidate.String()))
			continue
		}

		if cand.SelfStakeBucketIdx == bucket.Index {
			if err = cand.AddVote(calculateVoteWeight(vr.c, bucket, true)); err != nil {
				log.L().Error("failed to add vote for candidate",
					zap.Uint64("bucket index", bucket.Index),
					zap.String("candidate", bucket.Candidate.String()),
					zap.Error(err))
				continue
			}
			cand.SelfStake = bucket.StakedAmount
		} else {
			_ = cand.AddVote(calculateVoteWeight(vr.c, bucket, false))
		}
	}

	cands = make(CandidateList, 0, len(candm))
	for _, cand := range candm {
		cands = append(cands, cand)
	}
	return cands, nil
}

func (vr *VoteReviser) flush(height uint64, csm CandidateStateManager) error {
	cache, ok := vr.cache[height]
	if !ok {
		return nil
	}
	log.L().Info("committed revise action",
		zap.Uint64("height", height), zap.Int("number of cands", len(cache[1])))
	for i := 0; i < 2; i++ {
		sort.Sort(cache[i])
		for _, cand := range cache[i] {
			if err := csm.Upsert(cand); err != nil {
				return err
			}
			log.L().Info("committed revise action",
				zap.String("name", cand.Name), zap.String("votes", cand.Votes.String()))
		}
	}
	return nil
}
