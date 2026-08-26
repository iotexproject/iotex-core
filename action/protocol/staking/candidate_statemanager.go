// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/eracow"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

type (
	// BucketSet related to setting bucket
	BucketSet interface {
		updateBucket(index uint64, bucket *VoteBucket) error
		putBucket(bucket *VoteBucket) (uint64, error)
		delBucket(index uint64) error
		putBucketAndIndex(bucket *VoteBucket) (uint64, error)
		delBucketAndIndex(owner, cand address.Address, index uint64) error
	}
	// CandidateSet related to setting candidates
	CandidateSet interface {
		requestDeactivation(address.Address) error
		deactivate(*Candidate, *VoteBucket, uint64, func(*VoteBucket, bool) *big.Int) error
		putCandidate(*Candidate) error
		delCandidate(address.Address) error
		putVoterBucketIndex(address.Address, uint64) error
		delVoterBucketIndex(address.Address, uint64) error
		putCandBucketIndex(address.Address, uint64) error
		delCandBucketIndex(address.Address, uint64) error
	}
	// CandidateStateManager is candidate state manager on top of StateManager
	CandidateStateManager interface {
		BucketSet
		NativeBucketGetByIndex
		CandidateSet
		// candidate and bucket pool related
		DirtyView() *viewData
		ContainsName(string) bool
		ContainsOwner(address.Address) bool
		ContainsOperator(address.Address) bool
		ContainsSelfStakingBucket(uint64) bool
		GetByName(string) *Candidate
		GetByOwner(address.Address) *Candidate
		GetByIdentifier(address.Address) *Candidate
		GetByOperator(address.Address) *Candidate
		// HasBLSPubKeyOtherThan reports whether any candidate except self
		// has registered the given BLS pubkey. Used to enforce one BLS
		// pubkey per delegate — a hard requirement for IIP-52's
		// FastAggregateVerify quorum-counting model.
		//
		// Deliberately a predicate and not a "who holds it" lookup: two
		// candidates may already share a pubkey from before the rule
		// existed, and naming one of them means naming whichever a Go map
		// happened to yield first, which differs per node.
		HasBLSPubKeyOtherThan(blsPubKey []byte, self address.Address) bool
		Upsert(*Candidate) error
		CreditBucketPool(*big.Int, bool) error
		DebitBucketPool(*big.Int, bool) error
		Commit(context.Context) error
		SM() protocol.StateManager
		SR() protocol.StateReader
	}

	// CandidiateStateCommon is the common interface for candidate state manager and reader
	CandidiateStateCommon interface {
		ContainsSelfStakingBucket(uint64) bool
		GetByIdentifier(address.Address) *Candidate
		SR() protocol.StateReader
		NativeBucketGetByIndex
	}

	candSM struct {
		protocol.StateManager
		candCenter *CandidateCenter
		bucketPool *BucketPool
		// cow is the IIP-59 era copy-on-write session for native bucket and
		// voter-index writes.
		cow *eracow.Session
	}
)

// NewCandidateStateManagerWithContext returns a new CandidateStateManager whose
// native bucket writes participate in the IIP-59 era copy-on-write window.
//
// ctx supplies the fork gate only. Pre-activation the session it builds is
// inert and performs no state access whatsoever, so adding the session does
// not change state access or writes until IIP-59 activates.
func NewCandidateStateManagerWithContext(ctx context.Context, sm protocol.StateManager) (CandidateStateManager, error) {
	// TODO: we can store csm in a local cache, just as how statedb store the workingset
	// b/c most time the sm is used before, no need to create another clone
	csr, err := ConstructBaseView(sm)
	if err != nil {
		return nil, err
	}

	// make a copy of candidate center and bucket pool, so they can be modified by csm
	// and won't affect base view until being committed
	view := csr.BaseView()
	return &candSM{
		StateManager: sm,
		candCenter:   view.candCenter,
		bucketPool:   view.bucketPool,
		cow:          eracow.NewSession(ctx, sm),
	}, nil
}

func newCandidateStateManager(sm protocol.StateManager) CandidateStateManager {
	return &candSM{
		StateManager: sm,
	}
}

func (csm *candSM) SM() protocol.StateManager {
	return csm.StateManager
}

func (csm *candSM) SR() protocol.StateReader {
	return csm.StateManager
}

// DirtyView is csm's current state, which reflects base view + applying delta saved in csm's dock
func (csm *candSM) DirtyView() *viewData {
	v, err := csm.StateManager.ReadView(_protocolID)
	if err != nil {
		log.S().Panic("failed to read view", zap.Error(err))
	}
	vd, ok := v.(*viewData)
	if !ok {
		log.S().Panicf("unexpected view type %T", v)
	}
	return &viewData{
		candCenter:     csm.candCenter,
		bucketPool:     csm.bucketPool,
		contractsStake: vd.contractsStake,
	}
}

func (csm *candSM) ContainsName(name string) bool {
	return csm.candCenter.ContainsName(name)
}

func (csm *candSM) ContainsOwner(addr address.Address) bool {
	return csm.candCenter.ContainsOwner(addr)
}

func (csm *candSM) ContainsOperator(addr address.Address) bool {
	return csm.candCenter.ContainsOperator(addr)
}

func (csm *candSM) HasBLSPubKeyOtherThan(blsPubKey []byte, self address.Address) bool {
	return csm.candCenter.HasBLSPubKeyOtherThan(blsPubKey, self)
}

func (csm *candSM) ContainsSelfStakingBucket(index uint64) bool {
	return csm.candCenter.ContainsSelfStakingBucket(index)
}

func (csm *candSM) GetByName(name string) *Candidate {
	return csm.candCenter.GetByName(name)
}

func (csm *candSM) GetByOwner(addr address.Address) *Candidate {
	return csm.candCenter.GetByOwner(addr)
}

func (csm *candSM) GetByIdentifier(addr address.Address) *Candidate {
	return csm.candCenter.GetByIdentifier(addr)
}

func (csm *candSM) GetByOperator(addr address.Address) *Candidate {
	return csm.candCenter.GetByOperator(addr)
}

// Upsert writes the candidate into state manager and cand center
func (csm *candSM) Upsert(d *Candidate) error {
	return csm.upsert(d)
}

func (csm *candSM) upsert(d *Candidate) error {
	if err := csm.candCenter.Upsert(d); err != nil {
		return err
	}

	return csm.putCandidate(d)
}

func (csm *candSM) CreditBucketPool(amount *big.Int, deleteBucket bool) error {
	return csm.bucketPool.CreditPool(csm.StateManager, amount, deleteBucket)
}

func (csm *candSM) DebitBucketPool(amount *big.Int, newBucket bool) error {
	return csm.bucketPool.DebitPool(csm, amount, newBucket)
}

func (csm *candSM) Commit(ctx context.Context) error {
	view := csm.DirtyView()
	if err := view.Commit(ctx, csm); err != nil {
		return err
	}

	// write updated view back to state factory
	return csm.WriteView(_protocolID, view)
}

func (csm *candSM) NativeBucket(index uint64) (*VoteBucket, error) {
	return newCandidateStateReader(csm).NativeBucket(index)
}

func (csm *candSM) requestDeactivation(owner address.Address) error {
	cand := csm.candCenter.GetByOwner(owner)
	if cand == nil {
		return errors.Wrapf(errCandNotExist, "failed to get candidate with owner %s", owner)
	}
	if cand.DeactivatedAt != 0 {
		return ErrExitAlreadyRequested
	}
	cand.DeactivatedAt = candidateExitRequested

	return csm.upsert(cand)
}

func (csm *candSM) deactivate(cand *Candidate, bucket *VoteBucket, height uint64, calcVote func(bucket *VoteBucket, selfStake bool) *big.Int) error {
	if cand == nil {
		return errors.Wrapf(errCandNotExist, "invalid candidate")
	}
	if bucket == nil {
		return errors.Wrapf(ErrNoSelfStakeBucket, "invalid bucket")
	}
	if cand.SelfStakeBucketIdx != bucket.Index {
		return errors.New("self-stake bucket index mismatch")
	}

	switch {
	case cand.DeactivatedAt == 0:
		return ErrExitNotRequested
	case cand.DeactivatedAt == candidateExitRequested:
		return ErrExitNotScheduled
	case cand.DeactivatedAt > height:
		return ErrExitNotReady
	}
	prevWeight := calcVote(bucket, true)
	newWeight := calcVote(bucket, false)
	if err := cand.SubVote(prevWeight); err != nil {
		return err
	}
	if err := cand.AddVote(newWeight); err != nil {
		return err
	}
	cand.SelfStake = big.NewInt(0)
	cand.SelfStakeBucketIdx = candidateNoSelfStakeBucketIndex
	// Clear the exit-queue marker so a subsequent re-stake / activate flow
	// starts from a clean "no exit in flight" state. Without this the
	// candidateDeactivation view keeps reporting the previous schedule, and
	// frontends (iotex-hub) jump straight to "Confirm Exit" again.
	cand.DeactivatedAt = 0
	if err := csm.candCenter.Upsert(cand); err != nil {
		return err
	}

	return csm.upsert(cand)
}

func (csm *candSM) updateBucket(index uint64, bucket *VoteBucket) error {
	prior, err := csm.NativeBucket(index)
	if err != nil {
		return err
	}
	// IIP-59: the era drain recomputes weights from buckets several blocks
	// after the boundary, and it mutates them itself (compound deposits grow
	// StakedAmount). The value this write is about to overwrite is the one the
	// drain must keep seeing. The read above was already there, so the copy
	// costs nothing extra.
	if err := csm.cow.SnapshotNativeBucket(index, prior); err != nil {
		return err
	}

	_, err = csm.PutState(bucket, nativeBucketStateOpts(index)...)
	return err
}

func (csm *candSM) putBucket(bucket *VoteBucket) (uint64, error) {
	var tc totalBucketCount
	if _, err := csm.State(
		&tc,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(TotalBucketKey)); err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return 0, err
	}

	index := tc.Count()
	// IIP-59: no copy-on-write here, on purpose, for either of the two keys
	// this function touches.
	//
	// The bucket at `index` is brand new, and `index` is the current count,
	// which only ever grows — so `index` is necessarily >= the count frozen at
	// the era boundary, and the drain's high-water-mark check already rejects
	// it as "did not exist at H". A tombstone would be redundant.
	//
	// TotalBucketKey itself is frozen as a scalar into the era window at Begin
	// rather than copied on write. That is strictly stronger: the frozen scalar
	// still rejects a post-H bucket even if that bucket's own copy were missed,
	// whereas a copied counter would only be as good as the copy.
	// Add index inside bucket
	bucket.Index = index
	if _, err := csm.PutState(bucket, nativeBucketStateOpts(index)...); err != nil {
		return 0, err
	}
	tc.count++
	_, err := csm.PutState(
		&tc,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(TotalBucketKey))
	return index, err
}

func (csm *candSM) delBucket(index uint64) error {
	// IIP-59: a withdrawn bucket still counted towards the era being drained,
	// so its as-of-H value has to survive the delete. The read is behind the
	// window check, so pre-activation and outside a drain this costs nothing.
	if active, err := csm.cow.Active(); err != nil {
		return err
	} else if active {
		// A nil *VoteBucket in a non-nil interface would look like "it existed
		// and serialized to nothing", so the absent case is kept as a genuinely
		// nil interface.
		var prior state.Serializer
		switch b, err := csm.NativeBucket(index); {
		case err == nil:
			prior = b
		case errors.Cause(err) == state.ErrStateNotExist:
		default:
			return err
		}
		if err := csm.cow.SnapshotNativeBucket(index, prior); err != nil {
			return err
		}
	}
	_, err := csm.DelState(
		append(nativeBucketStateOpts(index), protocol.ObjectOption(&VoteBucket{}))...,
	)
	return err
}

func (csm *candSM) putBucketAndIndex(bucket *VoteBucket) (uint64, error) {
	index, err := csm.putBucket(bucket)
	if err != nil {
		return 0, errors.Wrap(err, "failed to put bucket")
	}

	if err := csm.putVoterBucketIndex(bucket.Owner, index); err != nil {
		return 0, errors.Wrap(err, "failed to put bucket index")
	}

	if err := csm.putCandBucketIndex(bucket.Candidate, index); err != nil {
		return 0, errors.Wrap(err, "failed to put candidate index")
	}
	return index, nil
}

func (csm *candSM) delBucketAndIndex(owner, cand address.Address, index uint64) error {
	if err := csm.delBucket(index); err != nil {
		return errors.Wrap(err, "failed to delete bucket")
	}

	if err := csm.delVoterBucketIndex(owner, index); err != nil {
		return errors.Wrap(err, "failed to delete bucket index")
	}

	if err := csm.delCandBucketIndex(cand, index); err != nil {
		return errors.Wrap(err, "failed to delete candidate index")
	}
	return nil
}

func (csm *candSM) putBucketIndex(addr address.Address, prefix byte, index uint64) error {
	var (
		bis  BucketIndices
		opts = nativeBucketIndexStateOpts(addr, prefix)
	)
	existed := true
	if _, err := csm.State(&bis, opts...); err != nil {
		if errors.Cause(err) != state.ErrStateNotExist {
			return err
		}
		existed = false
	}
	if prefix == _voterIndex {
		var prior state.Serializer
		if existed {
			prior = &bis
		}
		if err := csm.cow.SnapshotNativeVoterIndex(addr.Bytes(), prior); err != nil {
			return err
		}
	}
	bis.addBucketIndex(index)
	_, err := csm.PutState(&bis, opts...)
	return err
}

func (csm *candSM) putVoterBucketIndex(addr address.Address, index uint64) error {
	return csm.putBucketIndex(addr, _voterIndex, index)
}

func (csm *candSM) delBucketIndex(addr address.Address, prefix byte, index uint64) error {
	var (
		bis  BucketIndices
		opts = nativeBucketIndexStateOpts(addr, prefix)
	)
	if _, err := csm.State(&bis, opts...); err != nil {
		return err
	}
	if prefix == _voterIndex {
		if err := csm.cow.SnapshotNativeVoterIndex(addr.Bytes(), &bis); err != nil {
			return err
		}
	}
	bis.deleteBucketIndex(index)

	var err error
	if len(bis) == 0 {
		_, err = csm.DelState(append(opts, protocol.ObjectOption(&BucketIndices{}))...)
	} else {
		_, err = csm.PutState(&bis, opts...)
	}
	return err
}

func (csm *candSM) delVoterBucketIndex(addr address.Address, index uint64) error {
	return csm.delBucketIndex(addr, _voterIndex, index)
}

func (csm *candSM) putCandidate(d *Candidate) error {
	_, err := csm.PutState(d, protocol.NamespaceOption(_candidateNameSpace), protocol.KeyOption(d.GetIdentifier().Bytes()))
	return err
}

func (csm *candSM) putCandBucketIndex(addr address.Address, index uint64) error {
	return csm.putBucketIndex(addr, _candIndex, index)
}

func (csm *candSM) delCandidate(name address.Address) error {
	_, err := csm.DelState(protocol.NamespaceOption(_candidateNameSpace), protocol.KeyOption(name.Bytes()), protocol.ObjectOption(&Candidate{}))
	return err
}

func (csm *candSM) delCandBucketIndex(addr address.Address, index uint64) error {
	return csm.delBucketIndex(addr, _candIndex, index)
}
