package stakingindex

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

type calculateUnmutedVoteWeightFn func(*contractstaking.Bucket) *big.Int

type voteViewEventHandler struct {
	staking.EventHandler
	view CandidateVotes

	calculateUnmutedVoteWeight calculateUnmutedVoteWeightFn
}

func newVoteViewEventHandler(view CandidateVotes, fn calculateUnmutedVoteWeightFn, bucketNS string, store db.KVStore) (*voteViewEventHandler, error) {
	storeWithBuffer, err := newStoreWithBuffer(bucketNS, store)
	if err != nil {
		return nil, err
	}
	return &voteViewEventHandler{
		EventHandler:               storeWithBuffer,
		view:                       view,
		calculateUnmutedVoteWeight: fn,
	}, nil
}

func newVoteViewEventHandlerWrapper(handler staking.EventHandler, view CandidateVotes, fn calculateUnmutedVoteWeightFn) *voteViewEventHandler {
	return &voteViewEventHandler{
		EventHandler:               handler,
		view:                       view,
		calculateUnmutedVoteWeight: fn,
	}
}

func (s *voteViewEventHandler) PutBucket(addr address.Address, id uint64, bucket *contractstaking.Bucket) error {
	org, err := s.EventHandler.DeductBucket(addr, id)
	switch errors.Cause(err) {
	case nil, contractstaking.ErrBucketNotExist:
	default:
		return errors.Wrapf(err, "failed to deduct bucket")
	}

	deltaVotes, deltaAmount := s.calculateBucket(bucket)
	if org != nil {
		orgVotes, orgAmount := s.calculateBucket(org)
		deltaVotes = new(big.Int).Sub(deltaVotes, orgVotes)
		deltaAmount = new(big.Int).Sub(deltaAmount, orgAmount)
	}
	s.view.Add(addr.String(), deltaAmount, deltaVotes)

	s.EventHandler.PutBucket(addr, id, bucket)
	return nil
}

func (s *voteViewEventHandler) DeleteBucket(addr address.Address, id uint64) error {
	org, err := s.EventHandler.DeductBucket(addr, id)
	switch errors.Cause(err) {
	case nil:
		// subtract original votes
		deltaVotes, deltaAmount := s.calculateBucket(org)
		s.view.Add(addr.String(), deltaAmount.Neg(deltaAmount), deltaVotes.Neg(deltaVotes))
	case contractstaking.ErrBucketNotExist:
		// do nothing
	default:
		return errors.Wrapf(err, "failed to deduct bucket")
	}
	return s.EventHandler.DeleteBucket(addr, id)
}

func (s *voteViewEventHandler) calculateBucket(bucket *contractstaking.Bucket) (votes *big.Int, amount *big.Int) {
	if bucket.Muted {
		return big.NewInt(0), big.NewInt(0)
	}
	return s.calculateUnmutedVoteWeight(bucket), bucket.StakedAmount
}

type storeWithBuffer struct {
	ns    string
	store db.KVStore
}

func newStoreWithBuffer(ns string, store db.KVStore) (*storeWithBuffer, error) {
	return &storeWithBuffer{
		ns:    ns,
		store: store,
	}, nil
}

func (swb *storeWithBuffer) PutBucketType(addr address.Address, bt *contractstaking.BucketType) error {
	return errors.New("not supported")
}

func (swb *storeWithBuffer) DeductBucket(addr address.Address, id uint64) (*contractstaking.Bucket, error) {
	data, err := swb.store.Get(swb.ns, byteutil.Uint64ToBytesBigEndian(id))
	switch errors.Cause(err) {
	case nil:
	case db.ErrBucketNotExist, db.ErrNotExist:
		return nil, contractstaking.ErrBucketNotExist
	default:
		return nil, errors.Wrap(err, "failed to get bucket")
	}
	bucket := new(contractstaking.Bucket)
	if err := bucket.Deserialize(data); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize bucket")
	}
	return bucket, nil
}

func (swb *storeWithBuffer) PutBucket(addr address.Address, id uint64, bkt *contractstaking.Bucket) error {
	data, err := bkt.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize bucket")
	}
	return swb.store.Put(swb.ns, byteutil.Uint64ToBytesBigEndian(id), data)
}

func (swb *storeWithBuffer) DeleteBucket(addr address.Address, id uint64) error {
	return swb.store.Delete(swb.ns, byteutil.Uint64ToBytesBigEndian(id))
}
