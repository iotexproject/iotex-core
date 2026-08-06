package contractstaking

import (
	"bytes"
	"context"
	"sort"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/eracow"
	"github.com/iotexproject/iotex-core/v2/state"
)

// ContractStakingStateManager wraps a state manager to provide staking contract-specific writes.
type ContractStakingStateManager struct {
	ContractStakingStateReader
	sm protocol.StateManager
	// erigonOnly is true when this manager was built with
	// protocol.ErigonStoreOnlyOption. Such a manager is a mirror writer: the
	// authoritative trie writes for the same buckets are made later in the
	// block by the view commit path. It therefore takes no part in the IIP-59
	// era copy-on-write layer, which lives in the trie.
	erigonOnly bool
	// cow is the IIP-59 era copy-on-write session, built on first use so that a
	// manager constructed pre-activation never touches state.
	cow *eracow.Session
}

// NewContractStakingStateManager creates a new ContractStakingStateManager
func NewContractStakingStateManager(sm protocol.StateManager, opts ...protocol.StateOption) *ContractStakingStateManager {
	erigonOnly := false
	if cfg, err := protocol.CreateStateConfig(opts...); err == nil {
		erigonOnly = cfg.ErigonStoreOnly
	}
	return &ContractStakingStateManager{
		ContractStakingStateReader: *NewStateReader(sm, opts...),
		sm:                         sm,
		erigonOnly:                 erigonOnly,
	}
}

// cowSession returns the era copy-on-write session, creating it on first use.
//
// Building a session performs no state access, so this stays free
// pre-activation. The session caches the era window, which is why it must not
// outlive the block: every construction site of this manager is per-block.
func (cs *ContractStakingStateManager) cowSession(ctx context.Context) *eracow.Session {
	if cs.erigonOnly {
		return nil
	}
	if cs.cow == nil {
		cs.cow = eracow.NewSession(ctx, cs.sm)
	}
	return cs.cow
}

// snapshotBucketForEra copies a contract-staking bucket's as-of-H value into
// the era window before UpsertBucket/DeleteBucket changes it.
//
// The drain recomputes contract bucket weights from StakedAmount,
// StakedDuration, CreatedAt, UnlockedAt, UnstakedAt, IsTimestampBased and
// Muted, all of which are denormalized onto the Bucket itself — so the bucket
// record is the whole read set and the bucket type record needs no copy.
//
// Receipt processing rewrites these buckets on essentially every block, so the
// extra read is deliberately behind the window check: it happens only while an
// era drain is actually outstanding.
func (cs *ContractStakingStateManager) snapshotBucketForEra(ctx context.Context, contractAddr address.Address, bucketID uint64) error {
	s := cs.cowSession(ctx)
	active, err := s.Active()
	if err != nil || !active {
		return err
	}
	// A nil *Bucket inside a non-nil interface would be recorded as "existed at
	// H and serialized to nothing", so the absent case stays a nil interface
	// and becomes a tombstone.
	var prior state.Serializer
	switch prev, bErr := cs.Bucket(contractAddr, bucketID); {
	case bErr == nil:
		prior = prev
	case errors.Is(bErr, ErrBucketNotExist), errors.Cause(bErr) == state.ErrStateNotExist:
	default:
		return bErr
	}
	return s.SnapshotContractBucket(contractAddr.Bytes(), bucketID, prior)
}

// snapshotOwnerIndexForEra copies an owner's contract-staking bucket list into
// the era window before it changes.
//
// This is the LSD counterpart of the native _voterIndex copy: it is the list
// the drain enumerates a voter's contract buckets from, and writeOwnerIndex
// deletes the whole key once the list empties, so an owner who was owed a share
// at H can otherwise disappear from the state the drain walks.
//
// refs is the list as read immediately before the mutation; the empty list is
// never stored, so len(refs) == 0 means the key was absent and is recorded as a
// tombstone.
func (cs *ContractStakingStateManager) snapshotOwnerIndexForEra(ctx context.Context, owner address.Address, refs ContractBucketRefs) error {
	var prior state.Serializer
	if len(refs) > 0 {
		prior = &refs
	}
	return cs.cowSession(ctx).Snapshot(eracow.KindLSDVoterIndex, eracow.AddrSubkey(owner.Bytes()), prior)
}

// UpsertBucketType inserts or updates a bucket type for a given contract and bucket ID.
func (cs *ContractStakingStateManager) UpsertBucketType(contractAddr address.Address, bucketID uint64, bucketType *BucketType) error {
	_, err := cs.sm.PutState(
		bucketType,
		cs.makeOpts(
			bucketTypeNamespaceOption(contractAddr),
			bucketIDKeyOption(bucketID),
		)...,
	)

	return err
}

// DeleteBucket removes a bucket for a given contract and bucket ID.
//
// ctx carries the fork gate for the owner index only; the bucket write itself
// is unconditional and byte-for-byte what it was before the index existed.
func (cs *ContractStakingStateManager) DeleteBucket(ctx context.Context, contractAddr address.Address, bucketID uint64) error {
	// IIP-59: a bucket that counted towards the era being drained must keep
	// its as-of-H value even after it is burned.
	if err := cs.snapshotBucketForEra(ctx, contractAddr, bucketID); err != nil {
		return err
	}
	// Read the owner before the delete, while the bucket is still there.
	if OwnerIndexEnabled(ctx) {
		if err := cs.indexDelete(ctx, contractAddr, bucketID); err != nil {
			return err
		}
	}
	_, err := cs.sm.DelState(
		append(cs.BucketStateOpts(contractAddr, bucketID), protocol.ObjectOption(&Bucket{}))...,
	)

	return err
}

// UpsertBucket inserts or updates a bucket for a given contract and bid.
//
// This and DeleteBucket are the only writers of contract-staking bucket state,
// which is what makes them the single choke point for the owner index.
func (cs *ContractStakingStateManager) UpsertBucket(ctx context.Context, contractAddr address.Address, bid uint64, bucket *Bucket) error {
	// IIP-59: capture the value this write is about to replace, so the era
	// drain keeps seeing the bucket as it stood at the boundary.
	if err := cs.snapshotBucketForEra(ctx, contractAddr, bid); err != nil {
		return err
	}
	// Read the prior bucket before overwriting it, so an owner change can be
	// detected and the ref moved off the old owner's list.
	if OwnerIndexEnabled(ctx) {
		if err := cs.indexUpsert(ctx, contractAddr, bid, bucket); err != nil {
			return err
		}
		// IIP-59: every contract whose buckets can count towards a voter's
		// weight must have a high-water mark to freeze, or the era window has
		// no id bound for it and rejects all of its buckets.
		if err := cs.RaiseNumOfBuckets(contractAddr, bid); err != nil {
			return err
		}
	}
	_, err := cs.sm.PutState(
		bucket,
		cs.BucketStateOpts(contractAddr, bid)...,
	)

	return err
}

// RaiseNumOfBuckets raises the contract's bucket high-water mark to cover
// bucketID, and does nothing if it already does.
//
// # Why this exists
//
// StakingContract.NumOfBuckets is the max bucket id ever seen for a contract
// (see BackfillContract for why that is not a count). It is the only bound the
// IIP-59 era window has on contract bucket ids -- eracow.Window.
// ContractBucketExisted answers "did this id exist at H" purely from it -- and
// a contract with no record at all is rejected outright.
//
// Before IIP-59 exactly one code path maintained it:
// blockindex/contractstaking/cache.go Commit, i.e. the V1 indexer only. The V2
// and V3 indexers in systemcontractindex/stakingindex never wrote it, and
// neither did staking.nftEventHandler, the shared trie-write path all three
// indexers funnel their bucket writes through. So V2/V3 buckets had no frozen
// mark and were silently dropped from every frozen weight, no matter what the
// owner index said about them.
//
// Hooking the raise here rather than in each indexer covers all three at once,
// because UpsertBucket is the single writer of contract bucket state.
//
// # Raise-only, and gated
//
// Raise-only because the mark's whole meaning is "no id above this existed
// before now": lowering it would let a post-freeze bucket into a frozen era.
// It is also what makes this safe to run alongside the V1 indexer's own
// unconditional write of the same key -- V1's value is monotone too, so the two
// agree and the extra write is a no-op.
//
// Gated behind the IIP-59 fork by its only caller, because writing a meta
// record for a contract that has none today changes the state root.
func (cs *ContractStakingStateManager) RaiseNumOfBuckets(contractAddr address.Address, bucketID uint64) error {
	switch mark, err := cs.NumOfBuckets(contractAddr); {
	case err == nil:
		if mark >= bucketID {
			return nil
		}
	case errors.Cause(err) == state.ErrStateNotExist:
		// First bucket of a contract with no meta record yet.
	default:
		return errors.Wrapf(err, "failed to read bucket high-water mark of %s", contractAddr.String())
	}
	return cs.UpdateNumOfBuckets(contractAddr, bucketID)
}

// UpdateNumOfBuckets updates the number of buckets.
//
// This is an unconditional write, used by the V1 indexer which tracks the mark
// itself. Prefer RaiseNumOfBuckets anywhere the value is not already known to
// be monotone.
func (cs *ContractStakingStateManager) UpdateNumOfBuckets(contractAddr address.Address, numOfBuckets uint64) error {
	_, err := cs.sm.PutState(
		&StakingContract{
			NumOfBuckets: uint64(numOfBuckets),
		},
		cs.makeOpts(
			metaNamespaceOption(),
			contractKeyOption(contractAddr),
		)...,
	)

	return err
}

// BucketHighWaterMarks returns every staking contract's bucket high-water mark,
// sorted ascending by contract address, for freezing into an IIP-59 era window.
//
// The values come from the contract meta namespace, which holds exactly one
// small record per registered staking contract and nothing else, so this is a
// bounded scan rather than a state walk. The number it reads is the max bucket
// id ever seen for the contract: it is only ever raised, never lowered, and
// contract bucket ids are minted from a strictly monotonic counter that burning
// does not touch. A bucket id above the frozen mark therefore cannot have
// existed at the freeze height. Note this is an *inclusive* bound -- the mark
// is an id that exists, not one past the end.
//
// Post-IIP-59 the mark is maintained for every contract by
// ContractStakingStateManager.RaiseNumOfBuckets, hooked into UpsertBucket. Do
// not assume the indexers maintain it: only the V1 indexer ever did, which is
// exactly why RaiseNumOfBuckets exists.
//
// A contract that is missing from the result has never had a bucket written
// through UpsertBucket post-activation. eracow.Window.ContractBucketExisted
// rejects every bucket of such a contract, so a contract silently going missing
// here costs its owners their share -- staking.FrozenContractBucket logs that
// case rather than letting it pass unnoticed.
//
// Sorted output matters: this ends up in a consensus record, and map iteration
// order would make two nodes disagree byte-for-byte on identical state.
func BucketHighWaterMarks(sr protocol.StateReader) ([]eracow.ContractBucketCount, error) {
	_, iter, err := sr.States(protocol.NamespaceOption(state.StakingContractMetaNamespace))
	if err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to enumerate staking contracts")
	}
	out := make([]eracow.ContractBucketCount, 0, iter.Size())
	for i := 0; i < iter.Size(); i++ {
		var sc StakingContract
		key, err := iter.Next(&sc)
		if err != nil {
			if errors.Is(err, state.ErrNilValue) {
				continue
			}
			return nil, errors.Wrap(err, "failed to read staking contract record")
		}
		if len(key) != 20 {
			// Not a contract record; the namespace is single-purpose today, but
			// a stray key must not become a 20-byte-address-shaped lie.
			continue
		}
		out = append(out, eracow.ContractBucketCount{
			Contract:     append([]byte{}, key...),
			NumOfBuckets: sc.NumOfBuckets,
		})
	}
	sort.Slice(out, func(i, j int) bool { return bytes.Compare(out[i].Contract, out[j].Contract) < 0 })
	return out, nil
}
