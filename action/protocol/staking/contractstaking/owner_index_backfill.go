// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"context"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
)

// BackfillContract names one staking contract to be indexed, together with the
// exclusive upper bound on its bucket ids.
//
// The bound has to be supplied rather than discovered: enumerating a
// per-contract namespace is a full scan, and the whole point of the backfill is
// that no single call is allowed to do unbounded work. Callers already have the
// number cheaply -- Indexer.TotalBucketCount(height) in memory, or
// ContractStakingStateReader.NumOfBuckets from the meta namespace where the V1/
// V2 indexers maintain it. Bucket ids are minted from 0 upwards and never
// reused, so "ever minted" is a safe bound even though burnt ids are gaps.
type BackfillContract struct {
	Address     address.Address
	NumOfBucket uint64
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
// It is deliberately NOT wired into any activation path. It exists so an
// activation path can later drain it across several blocks; doing every LSD
// bucket in one block is not acceptable.
//
// Callers must gate this the same way the live maintenance is gated: it writes
// consensus state and must not run before IIP-59 activates.
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
	for cursor.ContractIndex < len(contracts) {
		c := contracts[cursor.ContractIndex]
		if c.Address == nil {
			return cursor, errors.Errorf("nil contract address at index %d", cursor.ContractIndex)
		}
		for cursor.NextBucketID < c.NumOfBucket {
			if budget == 0 {
				return cursor, nil
			}
			id := cursor.NextBucketID
			cursor.NextBucketID++
			budget--

			bkt, err := cs.Bucket(c.Address, id)
			if err != nil {
				if errors.Is(err, ErrBucketNotExist) {
					// burnt or never minted; ids are not dense
					continue
				}
				return cursor, errors.Wrapf(err, "failed to read bucket %d of %s", id, c.Address.String())
			}
			if bkt.Owner == nil {
				return cursor, errors.Errorf("contract-staking bucket %d of %s has no owner", id, c.Address.String())
			}
			if err := cs.addOwnerRef(ctx, bkt.Owner, ContractBucketRef{Contract: c.Address, BucketID: id}); err != nil {
				return cursor, err
			}
		}
		cursor.ContractIndex++
		cursor.NextBucketID = 0
	}
	return cursor, nil
}
