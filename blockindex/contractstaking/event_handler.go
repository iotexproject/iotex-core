package contractstaking

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/systemcontractindex/stakingindex"
)

type eventHandlerFactory struct {
	store                   db.KVStore
	calculateVoteWeightFunc stakingindex.CalculateUnmutedVoteWeightAtFn
}

func newEventHandlerFactory(store db.KVStore, fn stakingindex.CalculateUnmutedVoteWeightAtFn) *eventHandlerFactory {
	return &eventHandlerFactory{
		store:                   store,
		calculateVoteWeightFunc: fn,
	}
}

func (f *eventHandlerFactory) NewEventHandler(v stakingindex.CandidateVotes, height uint64) (stakingindex.BucketStore, error) {
	store := newStoreWithBuffer(newStoreEventReader(f.store))
	handler, err := stakingindex.NewVoteViewEventHandlerWraper(store, v, f.genCalculateUnmutedVoteWeight(height))
	if err != nil {
		return nil, err
	}
	return handler, nil
}

func (f *eventHandlerFactory) NewEventHandlerWithStore(store stakingindex.BucketStore, v stakingindex.CandidateVotes, height uint64) (stakingindex.BucketStore, error) {
	handler, err := stakingindex.NewVoteViewEventHandlerWraper(store, v, f.genCalculateUnmutedVoteWeight(height))
	if err != nil {
		return nil, err
	}
	return handler, nil
}

func (f *eventHandlerFactory) NewEventHandlerWithHandler(handler stakingindex.BucketStore, v stakingindex.CandidateVotes, height uint64) (stakingindex.BucketStore, error) {
	return stakingindex.NewVoteViewEventHandlerWraper(handler, v, f.genCalculateUnmutedVoteWeight(height))
}

func (f *eventHandlerFactory) genCalculateUnmutedVoteWeight(height uint64) stakingindex.CalculateUnmutedVoteWeightFn {
	return func(b *contractstaking.Bucket) *big.Int {
		return f.calculateVoteWeightFunc(b, height)
	}
}

type readOnlyEventHandler interface {
	DeductBucket(address.Address, uint64) (*contractstaking.Bucket, error)
}

type storeWithBuffer struct {
	underlying readOnlyEventHandler
	buffer     map[string]map[uint64]*contractstaking.Bucket
}

func newStoreWithBuffer(underlying readOnlyEventHandler) *storeWithBuffer {
	return &storeWithBuffer{
		underlying: underlying,
		buffer:     make(map[string]map[uint64]*contractstaking.Bucket),
	}
}

func (e *storeWithBuffer) PutBucketType(address.Address, *contractstaking.BucketType) error {
	// nothing to do
	return nil
}

func (e *storeWithBuffer) DeductBucket(addr address.Address, id uint64) (*contractstaking.Bucket, error) {
	if bm, ok := e.buffer[addr.String()]; ok {
		if bkt, ok := bm[id]; ok {
			if bkt != nil {
				return bkt, nil
			} else {
				return nil, errors.Wrap(contractstaking.ErrBucketNotExist, "bucket has been deleted")
			}
		}
	}
	return e.underlying.DeductBucket(addr, id)
}
func (e *storeWithBuffer) PutBucket(addr address.Address, id uint64, bucket *contractstaking.Bucket) error {
	if bm, ok := e.buffer[addr.String()]; ok {
		bm[id] = bucket
	} else {
		e.buffer[addr.String()] = map[uint64]*contractstaking.Bucket{id: bucket}
	}
	return nil
}
func (e *storeWithBuffer) DeleteBucket(addr address.Address, id uint64) error {
	if bm, ok := e.buffer[addr.String()]; ok {
		bm[id] = nil
	} else {
		e.buffer[addr.String()] = map[uint64]*contractstaking.Bucket{id: nil}
	}
	return nil
}

type storeEventReader struct {
	store db.KVStore
}

func newStoreEventReader(store db.KVStore) *storeEventReader {
	return &storeEventReader{store: store}
}

func (e *storeEventReader) DeductBucket(addr address.Address, id uint64) (*contractstaking.Bucket, error) {
	// load bucket info
	value, err := e.store.Get(_StakingBucketInfoNS, byteutil.Uint64ToBytesBigEndian(id))
	switch errors.Cause(err) {
	case nil:
	case db.ErrNotExist, db.ErrBucketNotExist:
		return nil, errors.Wrap(contractstaking.ErrBucketNotExist, "bucket doesn't exist")
	default:
		return nil, errors.Wrap(err, "failed to get bucket from db")
	}
	var bi bucketInfo
	if err := bi.Deserialize(value); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize bucket info")
	}

	// load bucket type
	value, err = e.store.Get(_StakingBucketTypeNS, byteutil.Uint64ToBytesBigEndian(bi.TypeIndex))
	switch errors.Cause(err) {
	case nil:
	case db.ErrNotExist, db.ErrBucketNotExist:
		// should not happen
		return nil, errors.Wrap(err, "bucket type doesn't exist but bucket exists, SHOULD NOT HAPPEN")
	default:
		return nil, errors.Wrap(err, "failed to get bucket type from db")
	}
	var bt BucketType
	if err := bt.Deserialize(value); err != nil {
		return nil, err
	}
	return assembleContractBucket(&bi, &bt), nil
}
