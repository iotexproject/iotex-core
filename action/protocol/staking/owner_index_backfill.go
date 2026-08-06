// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"context"
	"sort"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

// This file is the activation path for the IIP-59 owner -> contract-staking
// bucket index.
//
// # Why an activation backfill is needed at all
//
// contractstaking.ContractStakingStateManager maintains the owner index from
// the moment IIP-59 activates, on every bucket upsert and delete. That covers
// buckets created after activation and nothing else. An LSD voter whose buckets
// all predate activation has no entry, and the era drain walks voters
// shard-major over the index — so that voter is never visited, never paid, and
// their share silently falls into the residual.
//
// It does not self-heal either. When such a bucket is next written (a lock, an
// unlock, an expansion, a restake — anything that is not an owner change),
// ContractStakingStateManager.indexUpsert finds the prior owner equal to the
// new owner and returns early on the assumption that "the ref is already
// there". For a pre-activation bucket that assumption is exactly false. Only a
// transfer heals the entry, and only for the transferee.
//
// # Shape: one block, no persisted progress
//
// The whole index is built in the single block at g.ToBeEnabledBlockHeight,
// from CreatePreStates. There is no job record, no cursor, and no "complete"
// marker, because there is no intermediate state for anything to observe.
//
// The cost is bounded and already paid: this walks each contract's bucket
// namespace once with ContractStakingStateReader.Buckets, and writes one
// owner-index entry per distinct owner. protocol.go already performs a strictly
// heavier one-block migration at g.XinguBlockHeight, where
// contractsStake.Migrate flushes every contract bucket into state.
//
// # Why no era window can be open while this runs
//
// Writing the index while an era copy-on-write window was open would record the
// backfill as an in-era mutation and copy the pre-backfill (empty) list aside,
// which is precisely the value the drain must not see. It cannot happen:
//
//   - beginEraCOWWindow is only reached from FreezePollSnapshot, which is only
//     reached from poll.setCandidates, whose only caller is the PutPollResult
//     action handler. Actions run after CreatePreStates, so the backfill has
//     always finished before the first window of the chain can open.
//   - freezeIIP59PollSnapshot returns early while FeatureCtx.NoVoterRewardDistribution
//     is set, and that flag is !g.IsToBeEnabled(height) — so no window exists at
//     any height below the one this runs at, either.
//
// This is also why the index cannot be half-built at activation:
// contractstaking.OwnerIndexEnabled is bound to the same flag, so no live
// upsert has maintained it before this block.

// backfillOwnerIndex builds the owner -> contract-staking bucket index for
// every bucket that predates IIP-59 activation.
//
// Called exactly once in the life of a chain, from CreatePreStates at the
// activation height.
func backfillOwnerIndex(ctx context.Context, sm protocol.StateManager) error {
	contracts, marks, err := backfillContracts(ctx, sm)
	if err != nil {
		return err
	}

	var (
		csr     = contractstaking.NewStateReader(sm)
		csm     = contractstaking.NewContractStakingStateManager(sm)
		byOwner = make(map[string][]contractstaking.ContractBucketRef)
		owners  = make([]address.Address, 0, 64)
		scanned int
	)
	for _, contract := range contracts {
		ids, buckets, err := csr.Buckets(contract)
		if err != nil {
			return errors.Wrapf(err, "staking: failed to scan buckets of contract %s for the IIP-59 owner index", contract.String())
		}
		var (
			maxID uint64
			found bool
		)
		for i, id := range ids {
			if !found || id > maxID {
				maxID, found = id, true
			}
			bucket := buckets[i]
			if bucket == nil || bucket.Owner == nil {
				// A bucket with no owner cannot be indexed and cannot be paid.
				// Skipping it is what the live path does too (indexUpsert
				// rejects it), and failing the activation block over one
				// malformed record would halt the chain.
				log.L().Warn("IIP-59: skipping contract-staking bucket with no owner during owner index backfill",
					zap.String("contract", contract.String()), zap.Uint64("bucket", id))
				continue
			}
			scanned++
			key := string(bucket.Owner.Bytes())
			if _, seen := byOwner[key]; !seen {
				owners = append(owners, bucket.Owner)
			}
			byOwner[key] = append(byOwner[key], contractstaking.ContractBucketRef{
				Contract: contract,
				BucketID: id,
			})
		}
		// The recorded mark can be above the highest id still in state, because
		// the top bucket may since have been burned. Keep the larger: the mark
		// is the era layer's "this bucket existed at the freeze height" bound,
		// and lowering it would exclude buckets that do exist.
		if mark, ok := marks[string(contract.Bytes())]; ok {
			if !found || mark > maxID {
				maxID, found = mark, true
			}
		}
		if !found {
			// No buckets and no mark: nothing to backfill. Any bucket this
			// contract gets from here on is created post-activation, so
			// UpsertBucket indexes it and RaiseNumOfBuckets creates the mark.
			continue
		}
		if err := csm.RaiseNumOfBuckets(contract, maxID); err != nil {
			return err
		}
	}

	// Sorted by raw address bytes. The write order is consensus-visible through
	// the trie, so map iteration order would make two nodes disagree on
	// identical state.
	sort.Slice(owners, func(i, j int) bool {
		return bytes.Compare(owners[i].Bytes(), owners[j].Bytes()) < 0
	})
	for _, owner := range owners {
		if err := csm.AddOwnerRefs(ctx, owner, byOwner[string(owner.Bytes())]); err != nil {
			return errors.Wrapf(err, "staking: failed to backfill the IIP-59 owner index for %s", owner.String())
		}
	}

	// The only place the real size of this block's work is visible. Read it off
	// a testnet run before choosing the mainnet activation height.
	log.L().Info("IIP-59: built the contract-staking owner index",
		zap.Int("contracts", len(contracts)),
		zap.Int("buckets", scanned),
		zap.Int("owners", len(owners)))
	return nil
}

// backfillContracts returns the staking contracts to walk, sorted by raw
// address bytes, together with their recorded bucket high-water marks.
//
// The set is the genesis system staking contracts unioned with every contract
// that already has a record in the meta namespace. Genesis alone is not enough
// (a contract can have a mark without being named in genesis) and the meta
// namespace alone is not enough (only the V1 indexer ever maintained the mark).
func backfillContracts(ctx context.Context, sm protocol.StateManager) ([]address.Address, map[string]uint64, error) {
	g := genesis.MustExtractGenesisContext(ctx)
	byAddr := make(map[string]address.Address, 4)
	for _, s := range []string{
		g.SystemStakingContractAddress,
		g.SystemStakingContractV2Address,
		g.SystemStakingContractV3Address,
	} {
		if s == "" {
			continue
		}
		addr, err := address.FromString(s)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "staking: bad system staking contract address %q in genesis", s)
		}
		byAddr[string(addr.Bytes())] = addr
	}
	records, err := contractstaking.BucketHighWaterMarks(sm)
	if err != nil {
		return nil, nil, err
	}
	marks := make(map[string]uint64, len(records))
	for _, m := range records {
		addr, err := address.FromBytes(m.Contract)
		if err != nil {
			return nil, nil, errors.Wrap(err, "staking: bad contract address in the staking contract meta namespace")
		}
		byAddr[string(m.Contract)] = addr
		marks[string(m.Contract)] = m.NumOfBuckets
	}

	out := make([]address.Address, 0, len(byAddr))
	for _, addr := range byAddr {
		out = append(out, addr)
	}
	sort.Slice(out, func(i, j int) bool {
		return bytes.Compare(out[i].Bytes(), out[j].Bytes()) < 0
	})
	return out, marks, nil
}
