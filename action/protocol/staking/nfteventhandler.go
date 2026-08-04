package staking

import (
	"context"
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
		// ctx carries the IIP-59 fork gate down to the contract-staking state
		// manager. The EventHandler interface this type satisfies is
		// ctx-less, so the block's context is captured at construction; every
		// construction site is per-block, so it never outlives its height.
		ctx context.Context
	}
)

func newNFTBucketEventHandler(ctx context.Context, sm protocol.StateManager, calculateVoteWeight CalculateVoteWeightFunc) (*nftEventHandler, error) {
	csm, err := NewCandidateStateManagerWithContext(ctx, sm)
	if err != nil {
		return nil, err
	}
	return &nftEventHandler{
		calculateVoteWeight: calculateVoteWeight,
		cssm:                contractstaking.NewContractStakingStateManager(sm),
		csm:                 csm,
		bucketTypes:         make(map[address.Address]map[uint64]*contractstaking.BucketType),
		bucketTypesLookup:   make(map[address.Address]map[int64]map[uint64]uint64),
		ctx:                 ctx,
	}, nil
}

func newNFTBucketEventHandlerErigonOnly(ctx context.Context, sm protocol.StateManager, calculateVoteWeight CalculateVoteWeightFunc) *nftEventHandler {
	return &nftEventHandler{
		calculateVoteWeight: calculateVoteWeight,
		cssm:                contractstaking.NewContractStakingStateManager(sm, protocol.ErigonStoreOnlyOption()),
		bucketTypes:         make(map[address.Address]map[uint64]*contractstaking.BucketType),
		bucketTypesLookup:   make(map[address.Address]map[int64]map[uint64]uint64),
		ctx:                 ctx,
	}
}

// stateCtx returns the context the contract-staking state manager should see.
// A nil ctx (handlers built directly in tests) reads as pre-activation, which
// leaves the owner index untouched.
func (handler *nftEventHandler) stateCtx() context.Context {
	if handler.ctx == nil {
		return context.Background()
	}
	return handler.ctx
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
	if candidate == nil {
		return bucket, nil
	}
	weight := handler.calculateVoteWeight(bucket, height)
	// IIP-59: contract bucket weight removed from (candidate, owner).
	if err := subCandidateVotes(candidate, weight); err != nil {
		return nil, errors.Wrap(err, "failed to subtract vote")
	}
	if err := handler.csm.Upsert(candidate); err != nil {
		return nil, errors.Wrap(err, "failed to upsert candidate")
	}
	return bucket, nil
}

func (handler *nftEventHandler) PutBucket(contractAddr address.Address, id uint64, bkt *contractstaking.Bucket) error {
	if err := handler.cssm.UpsertBucket(handler.stateCtx(), contractAddr, id, bkt); err != nil {
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
	if candidate == nil {
		return nil
	}
	weight := handler.calculateVoteWeight(bkt, height)
	// IIP-59: contract bucket weight added to (candidate, owner).
	if err := addCandidateVotes(candidate, weight); err != nil {
		return errors.Wrap(err, "failed to add vote")
	}
	return handler.csm.Upsert(candidate)
}

func (handler *nftEventHandler) DeleteBucket(contractAddr address.Address, id uint64) error {
	bucket, err := handler.cssm.Bucket(contractAddr, id)
	if err != nil {
		return errors.Wrap(err, "failed to get bucket")
	}
	if err := handler.cssm.DeleteBucket(handler.stateCtx(), contractAddr, id); err != nil {
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
	if candidate == nil {
		return nil
	}
	weight := handler.calculateVoteWeight(bucket, height)
	// IIP-59: contract bucket weight removed from (candidate, owner).
	if err := subCandidateVotes(candidate, weight); err != nil {
		return errors.Wrap(err, "failed to subtract vote")
	}
	return handler.csm.Upsert(candidate)
}
