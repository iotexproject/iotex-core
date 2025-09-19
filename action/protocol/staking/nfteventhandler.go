package staking

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
)

type (
	// CalculateVoteWeightFunc is a function that calculates the vote weight of a bucket.
	CalculateVoteWeightFunc func(bkt *contractstaking.Bucket, height uint64) *big.Int

	nftEventHandler struct {
		calculateVoteWeight CalculateVoteWeightFunc
		cssm                *contractstaking.ContractStakingStateManager
		csm                 CandidateStateManager
		bucketTypes         map[address.Address]map[uint64]*contractstaking.BucketType
		bucketTypesLookup   map[address.Address]map[int64]map[uint64]uint64 // contract -> amount -> duration -> id
	}
)

func newNFTBucketEventHandler(sm protocol.StateManager, calculateVoteWeight CalculateVoteWeightFunc) (*nftEventHandler, error) {
	csm, err := NewCandidateStateManager(sm)
	if err != nil {
		return nil, err
	}
	return &nftEventHandler{
		calculateVoteWeight: calculateVoteWeight,
		cssm:                contractstaking.NewContractStakingStateManager(sm),
		csm:                 csm,
		bucketTypes:         make(map[address.Address]map[uint64]*contractstaking.BucketType),
		bucketTypesLookup:   make(map[address.Address]map[int64]map[uint64]uint64),
	}, nil
}

func newNFTBucketEventHandlerSecondaryOnly(sm protocol.StateManager, calculateVoteWeight CalculateVoteWeightFunc) (*nftEventHandler, error) {
	return &nftEventHandler{
		calculateVoteWeight: calculateVoteWeight,
		cssm:                contractstaking.NewContractStakingStateManager(sm, protocol.SecondaryOnlyOption()),
		csm:                 nil,
		bucketTypes:         make(map[address.Address]map[uint64]*contractstaking.BucketType),
		bucketTypesLookup:   make(map[address.Address]map[int64]map[uint64]uint64),
	}, nil
}

func (handler *nftEventHandler) matchBucketType(contractAddr address.Address, amount *big.Int, duration uint64) (uint64, error) {
	cmap, ok := handler.bucketTypesLookup[contractAddr]
	if !ok {
		tids, bucketTypes, err := handler.cssm.BucketTypes(contractAddr)
		if err != nil {
			return 0, err
		}
		cmap = make(map[int64]map[uint64]uint64)
		bts := make(map[uint64]*contractstaking.BucketType, len(tids))
		for i, bt := range bucketTypes {
			amount := bt.Amount.Int64()
			if cmap[amount] == nil {
				cmap[amount] = make(map[uint64]uint64)
			}
			cmap[amount][bt.Duration] = tids[i]
			bts[tids[i]] = bt
		}
		handler.bucketTypesLookup[contractAddr] = cmap
		handler.bucketTypes[contractAddr] = bts
	}
	amap, ok := cmap[amount.Int64()]
	if !ok {
		return uint64(len(cmap)), nil
	}
	id, ok := amap[duration]
	if !ok {
		return uint64(len(cmap)), nil
	}

	return id, nil
}

func (handler *nftEventHandler) PutBucketType(contractAddr address.Address, bt *contractstaking.BucketType) error {
	id, err := handler.matchBucketType(contractAddr, bt.Amount, bt.Duration)
	if err != nil {
		return err
	}
	if err := handler.cssm.UpsertBucketType(contractAddr, id, bt); err != nil {
		return err
	}
	handler.bucketTypes[contractAddr][id] = bt
	return nil
}

func (handler *nftEventHandler) DeductBucket(contractAddr address.Address, id uint64) (*contractstaking.Bucket, error) {
	bucket, err := handler.cssm.Bucket(contractAddr, id)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get bucket")
	}
	if handler.csm == nil {
		return bucket, nil
	}
	height, err := handler.csm.SR().Height()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get height")
	}
	candidate := handler.csm.GetByIdentifier(bucket.Candidate)
	if err := candidate.SubVote(handler.calculateVoteWeight(bucket, height)); err != nil {
		return nil, errors.Wrap(err, "failed to subtract vote")
	}
	if err := handler.csm.Upsert(candidate); err != nil {
		return nil, errors.Wrap(err, "failed to upsert candidate")
	}
	return bucket, nil
}

func (handler *nftEventHandler) PutBucket(contractAddr address.Address, id uint64, bkt *contractstaking.Bucket) error {
	if err := handler.cssm.UpsertBucket(contractAddr, id, bkt); err != nil {
		return errors.Wrap(err, "failed to put bucket")
	}
	if handler.csm == nil {
		return nil
	}
	height, err := handler.csm.SR().Height()
	if err != nil {
		return errors.Wrap(err, "failed to get height")
	}
	candidate := handler.csm.GetByIdentifier(bkt.Candidate)
	if err := candidate.AddVote(handler.calculateVoteWeight(bkt, height)); err != nil {
		return errors.Wrap(err, "failed to add vote")
	}
	return handler.csm.Upsert(candidate)
}

func (handler *nftEventHandler) DeleteBucket(contractAddr address.Address, id uint64) error {
	bucket, err := handler.cssm.Bucket(contractAddr, id)
	if err != nil {
		return errors.Wrap(err, "failed to get bucket")
	}
	if err := handler.cssm.DeleteBucket(contractAddr, id); err != nil {
		return errors.Wrap(err, "failed to delete bucket")
	}
	if handler.csm == nil {
		return nil
	}
	height, err := handler.csm.SR().Height()
	if err != nil {
		return errors.Wrap(err, "failed to get height")
	}
	candidate := handler.csm.GetByIdentifier(bucket.Candidate)
	if err := candidate.SubVote(handler.calculateVoteWeight(bucket, height)); err != nil {
		return errors.Wrap(err, "failed to subtract vote")
	}
	return handler.csm.Upsert(candidate)
}
