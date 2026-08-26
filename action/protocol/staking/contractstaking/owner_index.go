// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"bytes"
	"context"
	"sort"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/eracow"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

// LSDVoterIndexPrefix is the 1-byte tag of the owner -> contract-staking
// bucket index inside state.StakingNamespace.
//
// Key shape is {LSDVoterIndexPrefix} || owner(20) = 21 bytes. That is the same
// length as the native _voterIndex / _candIndex / _candidateRewardSnapshot keys,
// which is fine because those carry a different leading tag; readers that scan
// the shared namespace discriminate on tag *and* length (see
// parseVoterWeightKey in the staking package), and no existing tag uses this
// value.
const LSDVoterIndexPrefix = byte(6)

// _lsdVoterIndexKeyLen is the length of an owner index key.
const _lsdVoterIndexKeyLen = 1 + 20

type (
	// ContractBucketRef names one contract-staking bucket. Bucket ids are only
	// unique per staking contract, so the contract address is part of the
	// identity, not context.
	ContractBucketRef struct {
		Contract address.Address
		BucketID uint64
	}

	// ContractBucketRefs is the value stored under an owner index key: every
	// contract-staking bucket an address owns, across all staking contracts.
	//
	// The slice is kept sorted ascending by (contract bytes, bucket id) at all
	// times. This is consensus state: two nodes that applied the same set of
	// bucket writes must produce byte-identical values, so the order can never
	// come from map iteration.
	ContractBucketRefs []ContractBucketRef
)

// ErrOwnerIndexNotExist is returned when an address owns no contract-staking
// bucket, i.e. the index key is absent. The empty list is never stored.
var ErrOwnerIndexNotExist = errors.New("contract-staking owner index does not exist")

// lsdVoterIndexKey returns the state key holding one owner's contract-staking
// bucket refs.
//
// Unexported on purpose: no code outside this package may address the owner
// index, because doing so would drop the reader's global options. Use
// OwnerIndexStateOpts instead.
func lsdVoterIndexKey(owner address.Address) []byte {
	key := make([]byte, _lsdVoterIndexKeyLen)
	key[0] = LSDVoterIndexPrefix
	copy(key[1:], owner.Bytes())
	return key
}

// OwnerIndexStateOpts addresses one owner's contract-staking bucket ref list.
//
// This is the single expression for that address in the repository; see
// BucketStateOpts for why frozen and live reads must share one.
func (r *ContractStakingStateReader) OwnerIndexStateOpts(owner address.Address) []protocol.StateOption {
	return r.makeOpts(
		protocol.NamespaceOption(state.StakingNamespace),
		protocol.KeyOption(lsdVoterIndexKey(owner)),
	)
}

// ParseLSDVoterIndexKey reverses lsdVoterIndexKey. ok is false for any key in
// the staking namespace that is not an owner index entry -- buckets, native
// bucket indices, endorsements, reward snapshots and voter weights all share the
// namespace, so a scan must discriminate by key rather than by whether the
// value happens to deserialize.
func ParseLSDVoterIndexKey(key []byte) (address.Address, bool) {
	if len(key) != _lsdVoterIndexKeyLen || key[0] != LSDVoterIndexPrefix {
		return nil, false
	}
	owner, err := address.FromBytes(key[1:])
	if err != nil {
		return nil, false
	}
	return owner, true
}

// compareRef orders refs by contract address bytes, then bucket id.
func compareRef(a, b ContractBucketRef) int {
	if c := bytes.Compare(a.Contract.Bytes(), b.Contract.Bytes()); c != 0 {
		return c
	}
	switch {
	case a.BucketID < b.BucketID:
		return -1
	case a.BucketID > b.BucketID:
		return 1
	default:
		return 0
	}
}

// search returns the insertion point of ref and whether it is already present.
func (refs ContractBucketRefs) search(ref ContractBucketRef) (int, bool) {
	i := sort.Search(len(refs), func(i int) bool {
		return compareRef(refs[i], ref) >= 0
	})
	return i, i < len(refs) && compareRef(refs[i], ref) == 0
}

// Contains reports whether the ref is in the list.
func (refs ContractBucketRefs) Contains(ref ContractBucketRef) bool {
	_, found := refs.search(ref)
	return found
}

// add inserts ref keeping the list sorted. Adding a ref that is already there
// is a no-op, so replaying the same bucket write is idempotent.
func (refs *ContractBucketRefs) add(ref ContractBucketRef) {
	i, found := refs.search(ref)
	if found {
		return
	}
	old := *refs
	old = append(old, ContractBucketRef{})
	copy(old[i+1:], old[i:])
	old[i] = ref
	*refs = old
}

// remove drops ref if present and reports whether anything was removed.
func (refs *ContractBucketRefs) remove(ref ContractBucketRef) bool {
	i, found := refs.search(ref)
	if !found {
		return false
	}
	old := *refs
	*refs = append(old[:i], old[i+1:]...)
	return true
}

// Proto converts the refs to protobuf.
func (refs *ContractBucketRefs) Proto() *stakingpb.ContractBucketRefs {
	pb := make([]*stakingpb.ContractBucketRef, 0, len(*refs))
	for _, ref := range *refs {
		pb = append(pb, &stakingpb.ContractBucketRef{
			Contract: ref.Contract.Bytes(),
			Index:    ref.BucketID,
		})
	}
	return &stakingpb.ContractBucketRefs{Refs: pb}
}

// LoadProto converts protobuf to refs.
func (refs *ContractBucketRefs) LoadProto(pb *stakingpb.ContractBucketRefs) error {
	if pb == nil {
		return errors.New("contract bucket refs protobuf cannot be nil")
	}
	out := make(ContractBucketRefs, 0, len(pb.Refs))
	for _, r := range pb.Refs {
		if r == nil {
			return errors.New("nil contract bucket ref")
		}
		contract, err := address.FromBytes(r.Contract)
		if err != nil {
			return errors.Wrap(err, "failed to convert contract bytes to address")
		}
		out = append(out, ContractBucketRef{Contract: contract, BucketID: r.Index})
	}
	*refs = out
	return nil
}

// Serialize serializes the refs into bytes. The list is sorted first so a
// caller that built one by hand cannot write a value that differs only in
// order from the one a replaying node would produce.
func (refs *ContractBucketRefs) Serialize() ([]byte, error) {
	sorted := make(ContractBucketRefs, len(*refs))
	copy(sorted, *refs)
	sort.SliceStable(sorted, func(i, j int) bool { return compareRef(sorted[i], sorted[j]) < 0 })
	return proto.Marshal(sorted.Proto())
}

// Deserialize deserializes bytes into refs.
func (refs *ContractBucketRefs) Deserialize(data []byte) error {
	pb := &stakingpb.ContractBucketRefs{}
	if err := proto.Unmarshal(data, pb); err != nil {
		return errors.Wrap(err, "failed to unmarshal contract bucket refs")
	}
	return refs.LoadProto(pb)
}

// Encode encodes the refs into a GenericValue for Erigon dual-storage.
func (refs *ContractBucketRefs) Encode() (systemcontracts.GenericValue, error) {
	data, err := refs.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, errors.Wrap(err, "failed to serialize contract bucket refs")
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

// Decode decodes the refs from a GenericValue.
func (refs *ContractBucketRefs) Decode(gv systemcontracts.GenericValue) error {
	return refs.Deserialize(gv.PrimaryData)
}

// OwnerIndexEnabled reports whether the owner -> contract-staking bucket index
// may be written to the state trie.
//
// Pre-activation it must stay out of it: nodes upgrade over days, and one that
// wrote these keys early would diverge from every node still on the old binary
// -- a split at deployment time rather than at activation. It is bound to
// protocol.FeatureCtx.NoVoterRewardDistribution, the IIP-59 fork gate.
//
// A context with no feature context at all (indexer bootstraps, tests) is
// treated as pre-activation, so nothing is written by accident.
func OwnerIndexEnabled(ctx context.Context) bool {
	fCtx, ok := protocol.GetFeatureCtx(ctx)
	return ok && !fCtx.NoVoterRewardDistribution
}

// BucketRefsByOwner returns every contract-staking bucket the address owns,
// sorted by (contract, bucket id), plus the height the state was read at.
//
// This lives in the contractstaking package rather than next to
// NativeBucketIndicesByVoter because the value type names a staking contract:
// `staking` already imports `contractstaking`, so the ref type cannot live on
// the other side without inverting the dependency.
//
// An owner with no contract-staking buckets has no key at all; the error is
// state.ErrStateNotExist, wrapped as ErrOwnerIndexNotExist.
func (r *ContractStakingStateReader) BucketRefsByOwner(owner address.Address) (ContractBucketRefs, uint64, error) {
	var refs ContractBucketRefs
	height, err := r.sr.State(
		&refs,
		r.OwnerIndexStateOpts(owner)...,
	)
	if err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return nil, height, errors.Wrapf(ErrOwnerIndexNotExist, "owner %s", owner.String())
		}
		return nil, height, err
	}
	return refs, height, nil
}

// FrozenBucketRefs reads an owner's contract-staking bucket list as of the era
// freeze height. An absent list is returned as nil without an error.
func (r *ContractStakingStateReader) FrozenBucketRefs(window eracow.Window, owner address.Address) (ContractBucketRefs, error) {
	if !window.Open() {
		return nil, errors.New("contractstaking: no era window open")
	}
	var refs ContractBucketRefs
	err := eracow.Resolve(
		r.sr, window.FreezeHeight,
		eracow.KindLSDVoterIndex, eracow.AddrSubkey(owner.Bytes()),
		&refs,
		r.OwnerIndexStateOpts(owner)...,
	)
	switch {
	case err == nil:
		return refs, nil
	case errors.Is(err, eracow.ErrNotFrozen), errors.Cause(err) == state.ErrStateNotExist:
		return nil, nil
	default:
		return nil, err
	}
}

// readOwnerIndex loads an owner's refs, treating "no key" as the empty list.
func (cs *ContractStakingStateManager) readOwnerIndex(owner address.Address) (ContractBucketRefs, error) {
	var refs ContractBucketRefs
	if _, err := cs.sm.State(
		&refs,
		cs.OwnerIndexStateOpts(owner)...,
	); err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return nil, err
	}
	return refs, nil
}

// writeOwnerIndex persists an owner's refs, deleting the key when the list is
// empty. An empty BucketIndices-style value would be indistinguishable from a
// stale entry on a later scan and would keep a trie node alive forever.
func (cs *ContractStakingStateManager) writeOwnerIndex(owner address.Address, refs ContractBucketRefs) error {
	opts := cs.OwnerIndexStateOpts(owner)
	if len(refs) == 0 {
		_, err := cs.sm.DelState(append(opts, protocol.ObjectOption(&ContractBucketRefs{}))...)
		if err != nil && errors.Cause(err) == state.ErrStateNotExist {
			return nil
		}
		return err
	}
	_, err := cs.sm.PutState(&refs, opts...)
	return err
}

// addOwnerRef records that owner owns (contract, bucketID).
func (cs *ContractStakingStateManager) addOwnerRef(ctx context.Context, owner address.Address, ref ContractBucketRef) error {
	refs, err := cs.readOwnerIndex(owner)
	if err != nil {
		return errors.Wrapf(err, "failed to read owner index for %s", owner.String())
	}
	if _, found := refs.search(ref); found {
		// already indexed, avoid a pointless trie write
		return nil
	}
	// IIP-59: copy the list aside before it changes, while it still holds the
	// value the era drain must see. Checked for membership first so a no-op
	// write does not produce a copy either.
	if err := cs.snapshotOwnerIndexForEra(ctx, owner, refs); err != nil {
		return err
	}
	refs.add(ref)
	return errors.Wrapf(cs.writeOwnerIndex(owner, refs), "failed to write owner index for %s", owner.String())
}

// AddOwnerRefs records that owner owns every ref in refs.
//
// Semantically identical to calling addOwnerRef once per ref, including its
// idempotence and its "a no-op write produces no era copy either" rule. The
// difference is cost: the per-ref form is a read-modify-write of the same trie
// key for each ref, which is what made the IIP-59 activation backfill expensive
// for owners holding many buckets. This reads once, copies aside at most once,
// and writes once.
//
// Exported because the backfill lives in the staking package (see
// staking.backfillOwnerIndex); nothing else needs it — the live paths add one
// ref at a time.
func (cs *ContractStakingStateManager) AddOwnerRefs(ctx context.Context, owner address.Address, refs []ContractBucketRef) error {
	if len(refs) == 0 {
		return nil
	}
	cur, err := cs.readOwnerIndex(owner)
	if err != nil {
		return errors.Wrapf(err, "failed to read owner index for %s", owner.String())
	}
	fresh := make([]ContractBucketRef, 0, len(refs))
	for _, ref := range refs {
		if _, found := cur.search(ref); !found {
			fresh = append(fresh, ref)
		}
	}
	if len(fresh) == 0 {
		return nil
	}
	// IIP-59: copy the list aside before it changes, exactly as addOwnerRef
	// does. On the backfill path no era window can be open — it runs in
	// CreatePreStates, and the window only opens from the PutPollResult action
	// later in the same block — so this is a no-op there. It stays because the
	// method is a general one and its correctness should not rest on where its
	// only caller happens to sit today.
	if err := cs.snapshotOwnerIndexForEra(ctx, owner, cur); err != nil {
		return err
	}
	for _, ref := range fresh {
		// add keeps the list sorted, so the result is independent of the order
		// refs arrives in.
		cur.add(ref)
	}
	return errors.Wrapf(cs.writeOwnerIndex(owner, cur), "failed to write owner index for %s", owner.String())
}

// delOwnerRef drops (contract, bucketID) from owner's list.
func (cs *ContractStakingStateManager) delOwnerRef(ctx context.Context, owner address.Address, ref ContractBucketRef) error {
	refs, err := cs.readOwnerIndex(owner)
	if err != nil {
		return errors.Wrapf(err, "failed to read owner index for %s", owner.String())
	}
	if _, found := refs.search(ref); !found {
		return nil
	}
	// IIP-59: see addOwnerRef. This is the path that empties a list and drops
	// the key entirely, so it is the one that would otherwise lose a voter.
	if err := cs.snapshotOwnerIndexForEra(ctx, owner, refs); err != nil {
		return err
	}
	refs.remove(ref)
	return errors.Wrapf(cs.writeOwnerIndex(owner, refs), "failed to write owner index for %s", owner.String())
}

// indexUpsert keeps the owner index in step with a bucket write: a new bucket
// is added to its owner's list, and a bucket whose owner changed is moved from
// the old list to the new one.
func (cs *ContractStakingStateManager) indexUpsert(ctx context.Context, contractAddr address.Address, bucketID uint64, bucket, prior *Bucket) error {
	if bucket == nil || bucket.Owner == nil {
		return errors.Errorf("contract-staking bucket %d of %s has no owner", bucketID, contractAddr.String())
	}
	ref := ContractBucketRef{Contract: contractAddr, BucketID: bucketID}
	if prior != nil && prior.Owner != nil {
		if bytes.Equal(prior.Owner.Bytes(), bucket.Owner.Bytes()) {
			// same owner: the ref is already there, and add() is a no-op
			// anyway. Skip both reads.
			return nil
		}
		if err := cs.delOwnerRef(ctx, prior.Owner, ref); err != nil {
			return err
		}
	}
	return cs.addOwnerRef(ctx, bucket.Owner, ref)
}

// indexDelete removes a bucket's ref from its owner's list.
func (cs *ContractStakingStateManager) indexDelete(ctx context.Context, contractAddr address.Address, bucketID uint64, prior *Bucket) error {
	if prior == nil || prior.Owner == nil {
		// deleting a bucket that is not in state; nothing is indexed either.
		return nil
	}
	return cs.delOwnerRef(ctx, prior.Owner, ContractBucketRef{Contract: contractAddr, BucketID: bucketID})
}
