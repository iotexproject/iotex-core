package stakingindex

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
)

type (
	CalculateUnmutedVoteWeightFn   func(*contractstaking.Bucket) *big.Int
	CalculateUnmutedVoteWeightAtFn func(*contractstaking.Bucket, uint64) *big.Int
	BucketReader                   = staking.BucketReader

	voteViewEventHandler struct {
		BucketStore
		view CandidateVotes

		calculateUnmutedVoteWeight CalculateUnmutedVoteWeightFn
	}
)

// NewVoteViewEventHandler creates a new vote view event handler wrapper
func NewVoteViewEventHandler(store BucketStore, view CandidateVotes, fn CalculateUnmutedVoteWeightFn) (BucketStore, error) {
	return newVoteViewEventHandler(store, view, fn)
}

func newVoteViewEventHandler(store BucketStore, view CandidateVotes, fn CalculateUnmutedVoteWeightFn) (*voteViewEventHandler, error) {
	return &voteViewEventHandler{
		BucketStore:                store,
		view:                       view,
		calculateUnmutedVoteWeight: fn,
	}, nil
}

func (s *voteViewEventHandler) PutBucket(addr address.Address, id uint64, bucket *contractstaking.Bucket) error {
	org, err := s.BucketStore.DeductBucket(addr, id)
	switch errors.Cause(err) {
	case nil, contractstaking.ErrBucketNotExist:
	default:
		return errors.Wrapf(err, "failed to deduct bucket")
	}

	deltaVotes, deltaAmount := s.calculateBucket(bucket)
	if org != nil {
		orgVotes, orgAmount := s.calculateBucket(org)
		if org.Candidate.String() != bucket.Candidate.String() {
			s.view.Add(org.Candidate.String(), new(big.Int).Neg(orgAmount), new(big.Int).Neg(orgVotes))
		} else {
			deltaVotes = new(big.Int).Sub(deltaVotes, orgVotes)
			deltaAmount = new(big.Int).Sub(deltaAmount, orgAmount)
		}
	}
	s.view.Add(bucket.Candidate.String(), deltaAmount, deltaVotes)

	s.BucketStore.PutBucket(addr, id, bucket)
	return nil
}

func (s *voteViewEventHandler) DeleteBucket(addr address.Address, id uint64) error {
	org, err := s.BucketStore.DeductBucket(addr, id)
	switch errors.Cause(err) {
	case nil:
		// subtract original votes
		deltaVotes, deltaAmount := s.calculateBucket(org)
		s.view.Add(org.Candidate.String(), deltaAmount.Neg(deltaAmount), deltaVotes.Neg(deltaVotes))
	case contractstaking.ErrBucketNotExist:
		// do nothing
	default:
		return errors.Wrapf(err, "failed to deduct bucket")
	}
	return s.BucketStore.DeleteBucket(addr, id)
}

func (s *voteViewEventHandler) calculateBucket(bucket *contractstaking.Bucket) (votes *big.Int, amount *big.Int) {
	if bucket.Muted || bucket.UnstakedAt < maxStakingNumber {
		return big.NewInt(0), big.NewInt(0)
	}
	return s.calculateUnmutedVoteWeight(bucket), bucket.StakedAmount
}

type bucketStore struct {
	store BucketReader
	dirty map[string]map[uint64]*Bucket
}

func newBucketStore(store BucketReader) *bucketStore {
	return &bucketStore{
		store: store,
		dirty: make(map[string]map[uint64]*Bucket),
	}
}

func (swb *bucketStore) PutBucketType(addr address.Address, bt *contractstaking.BucketType) error {
	return nil
}

func (swb *bucketStore) DeductBucket(addr address.Address, id uint64) (*contractstaking.Bucket, error) {
	dirty, ok := swb.dirtyBucket(addr, id)
	if ok {
		if dirty == nil {
			return nil, errors.Wrap(contractstaking.ErrBucketNotExist, "bucket not exist")
		}
		return dirty.Clone(), nil
	}
	bucket, err := swb.store.DeductBucket(addr, id)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get bucket")
	}
	return bucket, nil
}

func (swb *bucketStore) PutBucket(addr address.Address, id uint64, bkt *contractstaking.Bucket) error {
	if _, ok := swb.dirty[addr.String()]; !ok {
		swb.dirty[addr.String()] = make(map[uint64]*Bucket)
	}
	swb.dirty[addr.String()][id] = bkt.Clone()
	return nil
}

func (swb *bucketStore) DeleteBucket(addr address.Address, id uint64) error {
	if _, ok := swb.dirty[addr.String()]; !ok {
		swb.dirty[addr.String()] = make(map[uint64]*Bucket)
	}
	swb.dirty[addr.String()][id] = nil
	return nil
}

func (swb *bucketStore) dirtyBucket(addr address.Address, id uint64) (*Bucket, bool) {
	if buckets, ok := swb.dirty[addr.String()]; ok {
		if bkt, ok := buckets[id]; ok {
			return bkt, true
		}
	}
	return nil, false
}
