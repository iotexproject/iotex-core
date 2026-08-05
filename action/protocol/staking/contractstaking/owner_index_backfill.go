// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"context"
	"math"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/state"
)

// BackfillContract names one staking contract to be indexed, together with the
// highest bucket id the backfill has to visit.
//
// # MaxBucketID is INCLUSIVE, and it is a max-seen id, not a count
//
// The name is deliberate. The number the node actually tracks per contract --
// StakingContract.NumOfBuckets in state.StakingContractMetaNamespace, written
// by ContractStakingStateManager.UpdateNumOfBuckets -- is maintained as
//
//	if id > totalBucketCount { totalBucketCount = id }
//
// (blockindex/contractstaking/cache.go). That is the largest bucket id ever
// observed. It is *not* a cardinality, despite the field being commented as
// "total number of buckets including burned buckets": burning never lowers it,
// and gaps in the id space never lower it either.
//
// All three deployed IIP-13 contracts mint bucket ids from a pre-incremented
// counter, so the first id they ever mint is 1, not 0:
//
//   - V1  `__currTokenId = unsafeInc(__currTokenId)`
//   - V2/V3 `bucketId = __currBucketId = unsafeInc(__currBucketId)`
//
// verified end-to-end by e2etest/contract_staking_test.go (V1, first stake
// asserts Index==1) and e2etest/contract_staking_v2_test.go (V2 and V3, first
// stake asserts Index==1). With 1-based ids the max-seen id numerically
// coincides with the ever-minted count, which is why the two readings were easy
// to confuse -- but only for the *value*, never for the *bound*. Treating the
// number as an exclusive bound drops the top bucket of every contract, and for
// a contract with exactly one bucket (MaxBucketID==1, bucket id 1) it drops the
// only bucket there is.
//
// The bound has to be supplied rather than discovered: enumerating a
// per-contract namespace is a full scan, and the whole point of the backfill is
// that no single call is allowed to do unbounded work. Callers already have the
// number cheaply -- Indexer.TotalBucketCount(height) in memory, or
// ContractStakingStateReader.NumOfBuckets from the meta namespace. Ids are
// never reused, so "highest ever minted" is a safe bound even though burnt ids
// leave gaps; the walk simply skips ids with no bucket record.
//
// Zero is a legal value and means "visit only id 0". Id 0 is unreachable for
// the deployed contracts, so a contract that has never minted must instead be
// left out of the contract list entirely (see
// staking.ownerIndexBackfillJob, which does exactly that).
type BackfillContract struct {
	Address address.Address
	// MaxBucketID is the highest bucket id to visit, inclusive. See the type
	// doc: this is a max-seen id, not a count.
	MaxBucketID uint64
}

// OwnerIndexBackfillCursor is the resume point of a batched backfill.
//
// The natural cursor is (position in the contract list, next bucket id within
// that contract): both components are monotonically increasing, the contract
// list is caller-ordered and fixed for the duration of the run, and bucket ids
// are dense-ish and ascending. Restarting from a cursor visits exactly the ids
// a single-shot run would have visited after that point, in the same order.
//
// The zero value starts at the beginning.
type OwnerIndexBackfillCursor struct {
	// ContractIndex is the index into the contracts slice being processed.
	ContractIndex int
	// NextBucketID is the first bucket id of that contract not yet processed.
	NextBucketID uint64
}

// Done reports whether the cursor has run past the end of the contract list.
func (c OwnerIndexBackfillCursor) Done(contracts []BackfillContract) bool {
	return c.ContractIndex >= len(contracts)
}

// BackfillOwnerIndex builds the owner -> contract-staking bucket index for
// buckets that already exist in state, doing at most `limit` bucket reads per
// call and returning the cursor to resume from.
//
// Callers must gate this the same way the live maintenance is gated: it writes
// consensus state and must not run before IIP-59 activates. The activation
// driver is staking.ownerIndexBackfillJob, reached from
// staking.Protocol.CreatePreStates.
//
// Buckets are visited in (contract order, ascending bucket id) order and the
// refs are inserted into a sorted list, so the resulting state does not depend
// on where the batches happened to be cut.
func BackfillOwnerIndex(
	ctx context.Context,
	cs *ContractStakingStateManager,
	contracts []BackfillContract,
	cursor OwnerIndexBackfillCursor,
	limit int,
) (OwnerIndexBackfillCursor, error) {
	if limit <= 0 {
		return cursor, errors.New("backfill batch limit must be positive")
	}
	if cursor.ContractIndex < 0 {
		return cursor, errors.Errorf("invalid backfill cursor contract index %d", cursor.ContractIndex)
	}
	budget := limit
contractLoop:
	for cursor.ContractIndex < len(contracts) {
		c := contracts[cursor.ContractIndex]
		if c.Address == nil {
			return cursor, errors.Errorf("nil contract address at index %d", cursor.ContractIndex)
		}
		// Inclusive bound: MaxBucketID is a bucket that exists, not one past
		// the end. See the BackfillContract doc.
		for cursor.NextBucketID <= c.MaxBucketID {
			if budget == 0 {
				return cursor, nil
			}
			id := cursor.NextBucketID
			budget--
			// math.MaxUint64 is not reachable from the deployed contracts, but
			// an inclusive bound makes "advance past the last id" unrepresentable
			// at the top of the id space: incrementing would wrap the cursor to
			// 0 and re-walk the contract forever. Move to the next contract
			// instead.
			atTop := id == math.MaxUint64
			if !atTop {
				cursor.NextBucketID++
			}
			if err := backfillBucket(ctx, cs, c.Address, id); err != nil {
				return cursor, err
			}
			if atTop {
				cursor.ContractIndex++
				cursor.NextBucketID = 0
				continue contractLoop
			}
		}
		cursor.ContractIndex++
		cursor.NextBucketID = 0
	}
	return cursor, nil
}

// backfillBucket adds the owner ref for one bucket, treating "no such bucket"
// as success: ids are not dense, burnt ids leave holes, and the bound is a
// high-water mark rather than a membership list.
func backfillBucket(ctx context.Context, cs *ContractStakingStateManager, contract address.Address, id uint64) error {
	bkt, err := cs.Bucket(contract, id)
	if err != nil {
		if errors.Is(err, ErrBucketNotExist) || errors.Cause(err) == state.ErrStateNotExist {
			return nil
		}
		return errors.Wrapf(err, "failed to read bucket %d of %s", id, contract.String())
	}
	if bkt.Owner == nil {
		return errors.Errorf("contract-staking bucket %d of %s has no owner", id, contract.String())
	}
	return cs.addOwnerRef(ctx, bkt.Owner, ContractBucketRef{Contract: contract, BucketID: id})
}
