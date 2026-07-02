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
		// voterDeltaSink is optional. When non-nil, PutBucket/DeleteBucket also
		// emit per-voter (cand, voter, Δweight) tuples so the staking
		// protocol's VoterWeightView stays consistent with contract staking
		// bucket transitions. Injected by voteView.Handle from context.
		voterDeltaSink staking.VoterDeltaSink
	}
)

// NewVoteViewEventHandler creates a new vote view event handler wrapper
func NewVoteViewEventHandler(store BucketStore, view CandidateVotes, fn CalculateUnmutedVoteWeightFn) (BucketStore, error) {
	return newVoteViewEventHandler(store, view, fn, nil)
}

func newVoteViewEventHandler(store BucketStore, view CandidateVotes, fn CalculateUnmutedVoteWeightFn, sink staking.VoterDeltaSink) (*voteViewEventHandler, error) {
	return &voteViewEventHandler{
		BucketStore:                store,
		view:                       view,
		calculateUnmutedVoteWeight: fn,
		voterDeltaSink:             sink,
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
	newVoterW := new(big.Int).Set(deltaVotes)
	if org != nil {
		orgVotes, orgAmount := s.calculateBucket(org)
		if org.Candidate.String() != bucket.Candidate.String() {
			s.view.Add(org.Candidate.String(), new(big.Int).Neg(orgAmount), new(big.Int).Neg(orgVotes))
			s.emitVoterDelta(org.Candidate, org.Owner, new(big.Int).Neg(orgVotes))
			s.emitVoterDelta(bucket.Candidate, bucket.Owner, newVoterW)
		} else {
			deltaVotes = new(big.Int).Sub(deltaVotes, orgVotes)
			deltaAmount = new(big.Int).Sub(deltaAmount, orgAmount)
			if org.Owner.String() != bucket.Owner.String() {
				s.emitVoterDelta(org.Candidate, org.Owner, new(big.Int).Neg(orgVotes))
				s.emitVoterDelta(bucket.Candidate, bucket.Owner, newVoterW)
			} else {
				s.emitVoterDelta(bucket.Candidate, bucket.Owner, new(big.Int).Set(deltaVotes))
			}
		}
	} else {
		s.emitVoterDelta(bucket.Candidate, bucket.Owner, newVoterW)
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
		s.emitVoterDelta(org.Candidate, org.Owner, new(big.Int).Neg(deltaVotes))
		s.view.Add(org.Candidate.String(), deltaAmount.Neg(deltaAmount), deltaVotes.Neg(deltaVotes))
	case contractstaking.ErrBucketNotExist:
		// do nothing
	default:
		return errors.Wrapf(err, "failed to deduct bucket")
	}
	return s.BucketStore.DeleteBucket(addr, id)
}

func (s *voteViewEventHandler) emitVoterDelta(cand, voter address.Address, delta *big.Int) {
	if s.voterDeltaSink == nil {
		return
	}
	if delta == nil || delta.Sign() == 0 {
		return
	}
	if cand == nil || voter == nil {
		return
	}
	s.voterDeltaSink.Apply(cand, voter, delta)
}

// voterDeltaSinkHandler wraps a BucketStore-style handler so every state
// transition also emits a per-voter weight delta to a staking.VoterDeltaSink.
// Used by voteView.Handle when a sink is attached via context, so the staking
// protocol's VoterWeightView stays incrementally consistent with contract
// staking bucket events.
type voterDeltaSinkHandler struct {
	BucketStore
	sink                       staking.VoterDeltaSink
	calculateUnmutedVoteWeight CalculateUnmutedVoteWeightFn
}

func newVoterDeltaSinkHandler(inner BucketStore, sink staking.VoterDeltaSink, fn CalculateUnmutedVoteWeightFn) *voterDeltaSinkHandler {
	return &voterDeltaSinkHandler{BucketStore: inner, sink: sink, calculateUnmutedVoteWeight: fn}
}

func (h *voterDeltaSinkHandler) PutBucket(addr address.Address, id uint64, bucket *contractstaking.Bucket) error {
	org, err := h.BucketStore.DeductBucket(addr, id)
	switch errors.Cause(err) {
	case nil, contractstaking.ErrBucketNotExist:
	default:
		return errors.Wrapf(err, "failed to deduct bucket")
	}
	newW := h.effectiveWeight(bucket)
	if err := h.BucketStore.PutBucket(addr, id, bucket); err != nil {
		return err
	}
	if org != nil {
		orgW := h.effectiveWeight(org)
		if org.Candidate.String() != bucket.Candidate.String() || org.Owner.String() != bucket.Owner.String() {
			if orgW.Sign() != 0 {
				h.sink.Apply(org.Candidate, org.Owner, new(big.Int).Neg(orgW))
			}
			if newW.Sign() != 0 {
				h.sink.Apply(bucket.Candidate, bucket.Owner, newW)
			}
			return nil
		}
		delta := new(big.Int).Sub(newW, orgW)
		if delta.Sign() != 0 {
			h.sink.Apply(bucket.Candidate, bucket.Owner, delta)
		}
		return nil
	}
	if newW.Sign() != 0 {
		h.sink.Apply(bucket.Candidate, bucket.Owner, newW)
	}
	return nil
}

func (h *voterDeltaSinkHandler) DeleteBucket(addr address.Address, id uint64) error {
	org, err := h.BucketStore.DeductBucket(addr, id)
	switch errors.Cause(err) {
	case nil:
		w := h.effectiveWeight(org)
		if w.Sign() != 0 {
			h.sink.Apply(org.Candidate, org.Owner, new(big.Int).Neg(w))
		}
	case contractstaking.ErrBucketNotExist:
		// nothing to sink
	default:
		return errors.Wrapf(err, "failed to deduct bucket")
	}
	return h.BucketStore.DeleteBucket(addr, id)
}

func (h *voterDeltaSinkHandler) effectiveWeight(bucket *contractstaking.Bucket) *big.Int {
	if bucket == nil || bucket.Muted || bucket.UnstakedAt < maxStakingNumber {
		return big.NewInt(0)
	}
	if bucket.Owner == nil || bucket.Candidate == nil {
		return big.NewInt(0)
	}
	return h.calculateUnmutedVoteWeight(bucket)
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
