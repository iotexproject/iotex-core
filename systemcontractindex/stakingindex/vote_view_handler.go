package stakingindex

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

type CalculateUnmutedVoteWeightFn func(*contractstaking.Bucket) *big.Int
type CalculateUnmutedVoteWeightAtFn func(*contractstaking.Bucket, uint64) *big.Int

type voteViewEventHandler struct {
	BucketStore
	view CandidateVotes

	calculateUnmutedVoteWeight CalculateUnmutedVoteWeightFn
}

func newVoteViewEventHandler(view CandidateVotes, fn CalculateUnmutedVoteWeightFn, bucketNS string, store db.KVStore) (*voteViewEventHandler, error) {
	storeWithBuffer, err := newStoreWithBuffer(bucketNS, store)
	if err != nil {
		return nil, err
	}
	return &voteViewEventHandler{
		BucketStore:                storeWithBuffer,
		view:                       view,
		calculateUnmutedVoteWeight: fn,
	}, nil
}

func NewVoteViewEventHandlerWraper(store BucketStore, view CandidateVotes, fn CalculateUnmutedVoteWeightFn) (BucketStore, error) {
	return newVoteViewEventHandlerWraper(store, view, fn)
}

func newVoteViewEventHandlerWraper(store BucketStore, view CandidateVotes, fn CalculateUnmutedVoteWeightFn) (*voteViewEventHandler, error) {
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

func (swb *storeWithBuffer) KVStore() db.KVStore {
	return swb.store
}

type storeWithContract struct {
	csr   *contractstaking.ContractStakingStateReader
	dirty map[string]map[uint64]*Bucket
}

func NewStoreWithContract(csr *contractstaking.ContractStakingStateReader) *storeWithContract {
	return &storeWithContract{
		csr:   csr,
		dirty: make(map[string]map[uint64]*Bucket),
	}
}

func (swb *storeWithContract) PutBucketType(addr address.Address, bt *contractstaking.BucketType) error {
	return nil
}

func (swb *storeWithContract) DeductBucket(addr address.Address, id uint64) (*contractstaking.Bucket, error) {
	dirty, ok := swb.dirtyBucket(addr, id)
	if ok {
		if dirty == nil {
			return nil, errors.Wrap(contractstaking.ErrBucketNotExist, "bucket not exist")
		}
		return dirty, nil
	}
	bucket, err := swb.csr.Bucket(addr, id)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get bucket")
	}
	return bucket, nil
}

func (swb *storeWithContract) PutBucket(addr address.Address, id uint64, bkt *contractstaking.Bucket) error {
	if _, ok := swb.dirty[addr.String()]; !ok {
		swb.dirty[addr.String()] = make(map[uint64]*Bucket)
	}
	swb.dirty[addr.String()][id] = bkt
	return nil
}

func (swb *storeWithContract) DeleteBucket(addr address.Address, id uint64) error {
	if _, ok := swb.dirty[addr.String()]; !ok {
		swb.dirty[addr.String()] = make(map[uint64]*Bucket)
	}
	swb.dirty[addr.String()][id] = nil
	return nil
}

func (swb *storeWithContract) dirtyBucket(addr address.Address, id uint64) (*Bucket, bool) {
	if buckets, ok := swb.dirty[addr.String()]; ok {
		if bkt, ok := buckets[id]; ok {
			return bkt, true
		}
	}
	return nil, false
}

type contractBucketCache struct {
	contractAddr address.Address
	csr          *contractstaking.ContractStakingStateReader
}

// NewContractBucketCache creates a new instance of BucketCache
func NewContractBucketCache(contractAddr address.Address, csr *contractstaking.ContractStakingStateReader) BucketCache {
	return &contractBucketCache{
		contractAddr: contractAddr,
		csr:          csr,
	}
}

func (cbc *contractBucketCache) ContractStakingBuckets() (uint64, map[uint64]*Bucket, error) {
	height, err := cbc.csr.Height(cbc.contractAddr)
	if err != nil {
		return 0, nil, errors.Wrap(err, "failed to get height")
	}
	buckets := make(map[uint64]*Bucket)
	ids, bkts, err := cbc.csr.Buckets(cbc.contractAddr)
	if err != nil {
		return 0, nil, errors.Wrap(err, "failed to get buckets")
	}
	for i := range ids {
		buckets[ids[i]] = bkts[i]
	}
	return height, buckets, nil
}

type storeWrapper struct {
	store BucketStore
	dirty map[string]map[uint64]*Bucket
}

func NewStoreWrapper(store BucketStore) *storeWrapper {
	return &storeWrapper{
		store: store,
		dirty: make(map[string]map[uint64]*Bucket),
	}
}

func (swb *storeWrapper) PutBucketType(addr address.Address, bt *contractstaking.BucketType) error {
	return nil
}

func (swb *storeWrapper) DeductBucket(addr address.Address, id uint64) (*contractstaking.Bucket, error) {
	dirty, ok := swb.dirtyBucket(addr, id)
	if ok {
		if dirty == nil {
			return nil, errors.Wrap(contractstaking.ErrBucketNotExist, "bucket not exist")
		}
		return dirty, nil
	}
	bucket, err := swb.store.DeductBucket(addr, id)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get bucket")
	}
	return bucket, nil
}

func (swb *storeWrapper) PutBucket(addr address.Address, id uint64, bkt *contractstaking.Bucket) error {
	if _, ok := swb.dirty[addr.String()]; !ok {
		swb.dirty[addr.String()] = make(map[uint64]*Bucket)
	}
	swb.dirty[addr.String()][id] = bkt
	return nil
}

func (swb *storeWrapper) DeleteBucket(addr address.Address, id uint64) error {
	if _, ok := swb.dirty[addr.String()]; !ok {
		swb.dirty[addr.String()] = make(map[uint64]*Bucket)
	}
	swb.dirty[addr.String()][id] = nil
	return nil
}

func (swb *storeWrapper) dirtyBucket(addr address.Address, id uint64) (*Bucket, bool) {
	if buckets, ok := swb.dirty[addr.String()]; ok {
		if bkt, ok := buckets[id]; ok {
			return bkt, true
		}
	}
	return nil, false
}
