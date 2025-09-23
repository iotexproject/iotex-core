package stakingindex

import (
	"math/big"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/db"
)

type eventHandlerFactory struct {
	bucketNS string
	store    db.KVStore
	fn       CalculateUnmutedVoteWeightAtFn
}

func NewEventHandlerFactory(bucketNS string, store db.KVStore, fn CalculateUnmutedVoteWeightAtFn) EventHandlerFactory {
	return &eventHandlerFactory{
		bucketNS: bucketNS,
		store:    store,
		fn:       fn,
	}
}

func (f *eventHandlerFactory) NewEventHandler(view CandidateVotes, height uint64) (BucketStore, error) {
	return newVoteViewEventHandler(view, f.genCalculateUnmutedVoteWeight(height), f.bucketNS, f.store)
}

func (f *eventHandlerFactory) NewEventHandlerWithStore(handler BucketStore, view CandidateVotes, height uint64) (BucketStore, error) {
	return newVoteViewEventHandlerWraper(NewStoreWrapper(handler), view, f.genCalculateUnmutedVoteWeight(height))
}

func (f *eventHandlerFactory) NewEventHandlerWithHandler(handler BucketStore, view CandidateVotes, height uint64) (BucketStore, error) {
	return newVoteViewEventHandlerWraper(handler, view, f.genCalculateUnmutedVoteWeight(height))
}

func (f *eventHandlerFactory) genCalculateUnmutedVoteWeight(height uint64) CalculateUnmutedVoteWeightFn {
	return func(b *contractstaking.Bucket) *big.Int {
		return f.fn(b, height)
	}
}

type contractEventHandlerFactory struct {
	csr *contractstaking.ContractStakingStateReader
	fn  CalculateUnmutedVoteWeightAtFn
}

func NewContractEventHandlerFactory(csr *contractstaking.ContractStakingStateReader, fn CalculateUnmutedVoteWeightAtFn) EventHandlerFactory {
	return &contractEventHandlerFactory{
		csr: csr,
		fn:  fn,
	}
}

func (f *contractEventHandlerFactory) NewEventHandler(view CandidateVotes, height uint64) (BucketStore, error) {
	store := NewStoreWithContract(f.csr)
	return newVoteViewEventHandlerWraper(store, view, f.genCalculateUnmutedVoteWeight(height))
}

func (f *contractEventHandlerFactory) NewEventHandlerWithStore(handler BucketStore, view CandidateVotes, height uint64) (BucketStore, error) {
	store := NewStoreWrapper(handler)
	return newVoteViewEventHandlerWraper(store, view, f.genCalculateUnmutedVoteWeight(height))
}

func (f *contractEventHandlerFactory) NewEventHandlerWithHandler(handler BucketStore, view CandidateVotes, height uint64) (BucketStore, error) {
	return newVoteViewEventHandlerWraper(handler, view, f.genCalculateUnmutedVoteWeight(height))
}

func (f *contractEventHandlerFactory) genCalculateUnmutedVoteWeight(height uint64) CalculateUnmutedVoteWeightFn {
	return func(b *contractstaking.Bucket) *big.Int {
		return f.fn(b, height)
	}
}
