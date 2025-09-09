package stakingindex

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/db"
)

type eventHandlerFactory struct {
	bucketNS string
	store    db.KVStore
	fn       CalculateUnmutedVoteWeightFn
}

func NewEventHandlerFactory(bucketNS string, store db.KVStore) EventHandlerFactory {
	return &eventHandlerFactory{
		bucketNS: bucketNS,
		store:    store,
	}
}

func (f *eventHandlerFactory) NewEventHandler(view CandidateVotes) (BucketStore, error) {
	return newVoteViewEventHandler(view, f.fn, f.bucketNS, f.store)
}

func (f *eventHandlerFactory) NewEventHandlerWithStore(handler BucketStore, view CandidateVotes) (BucketStore, error) {
	veh, ok := handler.(*voteViewEventHandler)
	if !ok {
		return nil, errors.Errorf("handler %T is not voteViewEventHandler", handler)
	}
	store, ok := veh.BucketStore.(*storeWithBuffer)
	if !ok {
		return nil, errors.Errorf("handler %T is not storeWithBuffer", handler)
	}
	return newVoteViewEventHandler(view, f.fn, f.bucketNS, store.KVStore())
}

func (f *eventHandlerFactory) NewEventHandlerWithHandler(handler BucketStore, view CandidateVotes) (BucketStore, error) {
	return newVoteViewEventHandlerWraper(handler, view, f.fn)
}

type contractEventHandlerFactory struct {
	csr *contractstaking.ContractStakingStateReader
	fn  CalculateUnmutedVoteWeightFn
}

func NewContractEventHandlerFactory(csr *contractstaking.ContractStakingStateReader, fn CalculateUnmutedVoteWeightFn) EventHandlerFactory {
	return &contractEventHandlerFactory{
		csr: csr,
		fn:  fn,
	}
}

func (f *contractEventHandlerFactory) NewEventHandler(view CandidateVotes) (BucketStore, error) {
	store := NewStoreWithContract(f.csr)
	return newVoteViewEventHandlerWraper(store, view, f.fn)
}

func (f *contractEventHandlerFactory) NewEventHandlerWithStore(handler BucketStore, view CandidateVotes) (BucketStore, error) {
	store := NewStoreWrapper(handler)
	return newVoteViewEventHandlerWraper(store, view, f.fn)
}

func (f *contractEventHandlerFactory) NewEventHandlerWithHandler(handler BucketStore, view CandidateVotes) (BucketStore, error) {
	return newVoteViewEventHandlerWraper(handler, view, f.fn)
}
