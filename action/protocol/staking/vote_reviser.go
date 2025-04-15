package staking

import (
	"math/big"
	"slices"
	"sort"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

// VoteReviser is used to recalculate candidate votes.
type (
	VoteReviser struct {
		cache map[uint64]CandidateList
		cfg   ReviseConfig
	}

	ReviseConfig struct {
		VoteWeight                  genesis.VoteWeightCalConsts
		ReviseHeights               []uint64
		CorrectCandsHeight          uint64
		SelfStakeBucketReviseHeight uint64
		CorrectCandSelfStakeHeight  uint64
	}
)

// NewVoteReviser creates a VoteReviser.
func NewVoteReviser(cfg ReviseConfig) *VoteReviser {
	// TODO: return error if cfg.CorrectSelfStakeBucketHeights is before hardfork height
	return &VoteReviser{
		cfg:   cfg,
		cache: make(map[uint64]CandidateList),
	}
}

// Revise recalculate candidate votes on preset revising height.
func (vr *VoteReviser) Revise(ctx protocol.FeatureCtx, csm CandidateStateManager, height uint64) error {
	if !vr.isCacheExist(height) {
		cands, _, err := newCandidateStateReader(csm.SM()).getAllCandidates()
		switch {
		case errors.Cause(err) == state.ErrStateNotExist:
		case err != nil:
			return err
		}
		if vr.shouldCorrectCandSelfStake(height) {
			cands, err = vr.correctCandSelfStake(ctx, csm, height, cands)
			if err != nil {
				return err
			}
		}
		if vr.shouldReviseSelfStakeBuckets(height) {
			cands, err = vr.reviseSelfStakeBuckets(ctx, csm, height, cands)
			if err != nil {
				return err
			}
		}
		cands, err = vr.calculateVoteWeight(csm, cands)
		if err != nil {
			return err
		}
		sort.Sort(cands)
		if vr.shouldReviseAlias(height) {
			cands, err = vr.correctAliasCands(csm, cands)
			if err != nil {
				return err
			}
		}
		vr.storeToCache(height, cands)
	}
	return vr.flush(height, csm)
}

func (vr *VoteReviser) correctAliasCands(csm CandidateStateManager, cands CandidateList) (CandidateList, error) {
	var retval CandidateList
	view := csm.DirtyView()
	for _, c := range view.candCenter.base.nameMap {
		retval = append(retval, c)
	}
	for _, c := range view.candCenter.base.operatorMap {
		retval = append(retval, c)
	}
	sort.Sort(retval)
	ownerMap := map[string]*Candidate{}
	for _, cand := range view.candCenter.base.owners {
		ownerMap[cand.Owner.String()] = cand
	}
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

func (vr *VoteReviser) reviseSelfStakeBuckets(ctx protocol.FeatureCtx, csm CandidateStateManager, height uint64, cands CandidateList) (CandidateList, error) {
	// revise endorsements
	esm := NewEndorsementStateManager(csm.SM())
	for _, cand := range cands {
		endorsement, err := esm.Get(cand.SelfStakeBucketIdx)
		switch errors.Cause(err) {
		case state.ErrStateNotExist:
			continue
		case nil:
			if endorsement.LegacyStatus(height) == EndorseExpired {
				if err := esm.Delete(cand.SelfStakeBucketIdx); err != nil {
					return nil, errors.Wrapf(err, "failed to delete endorsement with bucket index %d", cand.SelfStakeBucketIdx)
				}
				cand.SelfStakeBucketIdx = candidateNoSelfStakeBucketIndex
			}
		default:
			return nil, errors.Wrapf(err, "failed to get endorsement with bucket index %d", cand.SelfStakeBucketIdx)
		}
	}
	return cands, nil
}

func (vr *VoteReviser) correctCandSelfStake(ctx protocol.FeatureCtx, csm CandidateStateManager, height uint64, cands CandidateList) (CandidateList, error) {
	// revise selfstake
	for _, cand := range cands {
		if cand.SelfStakeBucketIdx == candidateNoSelfStakeBucketIndex {
			cand.SelfStake = big.NewInt(0)
			continue
		}
		sb, err := csm.getBucket(cand.SelfStakeBucketIdx)
		switch errors.Cause(err) {
		case state.ErrStateNotExist:
			// bucket has been withdrawn
			cand.SelfStakeBucketIdx = candidateNoSelfStakeBucketIndex
			cand.SelfStake = big.NewInt(0)
		case nil:
			if sb.isUnstaked() {
				cand.SelfStakeBucketIdx = candidateNoSelfStakeBucketIndex
				cand.SelfStake = big.NewInt(0)
			}
		default:
			return nil, errors.Wrapf(err, "failed to get bucket with index %d", cand.SelfStakeBucketIdx)
		}
	}
	return cands, nil
}

func (vr *VoteReviser) result(height uint64) (CandidateList, bool) {
	cands, ok := vr.cache[height]
	if !ok {
		return nil, false
	}
	return cands, true
}

func (vr *VoteReviser) storeToCache(height uint64, cands CandidateList) {
	vr.cache[height] = cands
}

func (vr *VoteReviser) isCacheExist(height uint64) bool {
	_, ok := vr.cache[height]
	return ok
}

// NeedRevise returns true if height needs revise
func (vr *VoteReviser) NeedRevise(height uint64) bool {
	return slices.Contains(vr.cfg.ReviseHeights, height) ||
		vr.shouldReviseSelfStakeBuckets(height) ||
		vr.shouldReviseAlias(height) ||
		vr.shouldCorrectCandSelfStake(height)
}

// shouldReviseAlias returns true if height needs to revise candidate aliases
func (vr *VoteReviser) shouldReviseAlias(height uint64) bool {
	return height == vr.cfg.CorrectCandsHeight
}

func (vr *VoteReviser) shouldReviseSelfStakeBuckets(height uint64) bool {
	return vr.cfg.SelfStakeBucketReviseHeight == height
}

func (vr *VoteReviser) shouldCorrectCandSelfStake(height uint64) bool {
	return vr.cfg.CorrectCandSelfStakeHeight == height
}

func (vr *VoteReviser) calculateVoteWeight(csm CandidateStateManager, cands CandidateList) (CandidateList, error) {
	csr := newCandidateStateReader(csm.SM())
	candm := make(map[string]*Candidate)
	for _, cand := range cands {
		candm[cand.GetIdentifier().String()] = cand.Clone()
		candm[cand.GetIdentifier().String()].Votes = new(big.Int)
		candm[cand.GetIdentifier().String()].SelfStake = new(big.Int)
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
			if err = cand.AddVote(CalculateVoteWeight(vr.cfg.VoteWeight, bucket, true)); err != nil {
				log.L().Error("failed to add vote for candidate",
					zap.Uint64("bucket index", bucket.Index),
					zap.String("candidate", bucket.Candidate.String()),
					zap.Error(err))
				continue
			}
			cand.SelfStake = bucket.StakedAmount
		} else {
			_ = cand.AddVote(CalculateVoteWeight(vr.cfg.VoteWeight, bucket, false))
		}
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
	log.L().Info("committed revise action",
		zap.Uint64("height", height), zap.Int("number of cands", len(cands)))
	for _, cand := range cands {
		if err := csm.Upsert(cand); err != nil {
			return err
		}
		log.L().Info("committed revise action",
			zap.String("name", cand.Name), zap.String("votes", cand.Votes.String()))
	}
	return nil
}
