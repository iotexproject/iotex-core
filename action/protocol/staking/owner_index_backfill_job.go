// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"context"
	"encoding/binary"
	"sort"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
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
// # Shape
//
// One record, seeded on the first block after the fork gate opens, holding the
// contract list and a cursor. Every block afterwards drains at most
// _ownerIndexBackfillPerBlock bucket reads and re-persists the cursor, the same
// bounded-job-with-a-persisted-cursor shape the era COW garbage collector uses
// (rewarding.collectEraCOWGarbage, _eraCOWGarbagePerBlock). Once the cursor
// runs off the end the record stays behind as the "already done" marker and is
// never written again.
//
// # Interaction with era boundaries
//
// See OwnerIndexBackfillComplete: an era must not be paid out while the index
// it would be drained from is still incomplete.

// _ownerIndexBackfillPerBlock is how many contract-staking bucket ids one block
// may visit. Matched to rewarding's _eraCOWGarbagePerBlock: both are bounded
// state work bolted onto CreatePreStates, and there is no reason for them to
// have different appetites. A visit is one point read plus, for an id that
// resolves to a bucket, one owner-index read-modify-write.
const _ownerIndexBackfillPerBlock = 256

// _lsdBackfillJobKey is the single key of the backfill record.
var _lsdBackfillJobKey = []byte{_lsdBackfillJob}

// ownerIndexBackfillJob is the persisted state of the activation backfill.
//
// Contracts is fixed at seed time and must not be recomputed afterwards: the
// cursor's ContractIndex is an index into it, so a list that changed shape
// between blocks would silently reposition the cursor.
type ownerIndexBackfillJob struct {
	Contracts []contractstaking.BackfillContract
	Cursor    contractstaking.OwnerIndexBackfillCursor
}

// Done reports whether the walk has finished.
func (j *ownerIndexBackfillJob) Done() bool { return j.Cursor.Done(j.Contracts) }

const (
	_backfillJobVersion  = byte(1)
	_backfillJobHeader   = 1 + 4 + 8 + 4
	_backfillContractLen = 20 + 8
)

// Serialize implements state.Serializer.
//
// Hand-rolled for the same reason eracow.Control is: a fixed-width record with
// no optional fields has exactly one byte encoding, which is what a consensus
// record wants.
func (j *ownerIndexBackfillJob) Serialize() ([]byte, error) {
	if j.Cursor.ContractIndex < 0 {
		return nil, errors.Errorf("staking: backfill cursor contract index %d is negative", j.Cursor.ContractIndex)
	}
	out := make([]byte, 0, _backfillJobHeader+len(j.Contracts)*_backfillContractLen)
	out = append(out, _backfillJobVersion)
	out = binary.BigEndian.AppendUint32(out, uint32(j.Cursor.ContractIndex))
	out = binary.BigEndian.AppendUint64(out, j.Cursor.NextBucketID)
	out = binary.BigEndian.AppendUint32(out, uint32(len(j.Contracts)))
	for _, c := range j.Contracts {
		if c.Address == nil {
			return nil, errors.New("staking: backfill contract has no address")
		}
		b := c.Address.Bytes()
		if len(b) != 20 {
			return nil, errors.Errorf("staking: backfill contract address must be 20 bytes, got %d", len(b))
		}
		out = append(out, b...)
		out = binary.BigEndian.AppendUint64(out, c.MaxBucketID)
	}
	return out, nil
}

// Deserialize implements state.Deserializer.
func (j *ownerIndexBackfillJob) Deserialize(buf []byte) error {
	if len(buf) < _backfillJobHeader {
		return errors.Errorf("staking: backfill record must be at least %d bytes, got %d", _backfillJobHeader, len(buf))
	}
	if buf[0] != _backfillJobVersion {
		return errors.Errorf("staking: unknown backfill record version %d", buf[0])
	}
	j.Cursor = contractstaking.OwnerIndexBackfillCursor{
		ContractIndex: int(binary.BigEndian.Uint32(buf[1:])),
		NextBucketID:  binary.BigEndian.Uint64(buf[5:]),
	}
	n := int(binary.BigEndian.Uint32(buf[13:]))
	rest := buf[_backfillJobHeader:]
	if want := n * _backfillContractLen; len(rest) != want {
		return errors.Errorf("staking: backfill record declares %d bytes of body but carries %d", want, len(rest))
	}
	j.Contracts = nil
	if n == 0 {
		return nil
	}
	j.Contracts = make([]contractstaking.BackfillContract, 0, n)
	for i := 0; i < n; i++ {
		off := i * _backfillContractLen
		addr, err := address.FromBytes(rest[off : off+20])
		if err != nil {
			return errors.Wrap(err, "staking: bad contract address in backfill record")
		}
		j.Contracts = append(j.Contracts, contractstaking.BackfillContract{
			Address:     addr,
			MaxBucketID: binary.BigEndian.Uint64(rest[off+20:]),
		})
	}
	return nil
}

// Encode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (j *ownerIndexBackfillJob) Encode() (systemcontracts.GenericValue, error) {
	data, err := j.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

// Decode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (j *ownerIndexBackfillJob) Decode(v systemcontracts.GenericValue) error {
	return j.Deserialize(v.PrimaryData)
}

// readOwnerIndexBackfillJob returns the backfill record, or nil if there is
// none yet.
func readOwnerIndexBackfillJob(sr protocol.StateReader) (*ownerIndexBackfillJob, error) {
	j := &ownerIndexBackfillJob{}
	if _, err := sr.State(j,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(_lsdBackfillJobKey),
	); err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return nil, nil
		}
		return nil, errors.Wrap(err, "staking: failed to read owner index backfill record")
	}
	return j, nil
}

func writeOwnerIndexBackfillJob(sm protocol.StateManager, j *ownerIndexBackfillJob) error {
	_, err := sm.PutState(j,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(_lsdBackfillJobKey),
	)
	return errors.Wrap(err, "staking: failed to write owner index backfill record")
}

// OwnerIndexBackfillComplete reports whether the IIP-59 owner-index activation
// backfill has finished.
//
// # What the rewarding protocol must do with this
//
// An era boundary freezes a voter set and then pays it out over the following
// blocks. Freezing one while the index is still being built freezes a
// *knowingly incomplete* voter set: every LSD voter the backfill has not
// reached yet is absent from the shard walk, is never visited by the drain, and
// their share becomes residual. The loss is permanent — the era is sealed and
// the payout is not revisited.
//
// So the boundary must not be taken. Rolling the pool forward instead is both
// cheap and already implemented: pendingBlockRewardPool accumulates per
// delegate and is only drained when isEraBoundary is true, so a boundary that
// does not fire leaves the accumulated rewards in place for the next one. The
// alternatives are worse — freezing an incomplete set silently underpays, and
// halting the block halts the chain over a condition that resolves itself
// within a few hundred blocks.
//
// It also cannot loop: the backfill advances every block regardless of era
// boundaries, so it completes in ceil(totalBuckets/_ownerIndexBackfillPerBlock)
// blocks whatever the reward protocol does, and the boundary after that fires
// normally.
//
// A missing record reads as incomplete. Pre-activation that is the state of the
// world and the caller's own fork gate is false anyway; post-activation it can
// only be seen in the single block between the gate opening and
// CreatePreStates seeding the record, when the index genuinely is not built.
func OwnerIndexBackfillComplete(sr protocol.StateReader) (bool, error) {
	j, err := readOwnerIndexBackfillJob(sr)
	if err != nil {
		return false, err
	}
	if j == nil {
		return false, nil
	}
	return j.Done(), nil
}

// runOwnerIndexBackfill advances the activation backfill by at most one batch.
//
// Returns the number of bucket ids visited, for the caller's metrics; 0 means
// either "not activated", "already finished", or "nothing left in this batch".
//
// Gated by contractstaking.OwnerIndexEnabled, the identical predicate that
// gates the live index maintenance in UpsertBucket/DeleteBucket. It has to be
// the same one: this writes the same keys, and a backfill that ran a block
// earlier or later than the maintenance would be a state-root divergence.
func runOwnerIndexBackfill(ctx context.Context, sm protocol.StateManager) (int, error) {
	if !contractstaking.OwnerIndexEnabled(ctx) {
		return 0, nil
	}
	job, err := readOwnerIndexBackfillJob(sm)
	if err != nil {
		return 0, err
	}
	if job == nil {
		if job, err = seedOwnerIndexBackfillJob(ctx, sm); err != nil {
			return 0, err
		}
	} else if job.Done() {
		// Steady state for the rest of the chain's life: one small point read
		// per block and no write. Keeping the record rather than deleting it is
		// what makes "already done" distinguishable from "not seeded yet".
		return 0, nil
	}
	before := job.Cursor
	cursor, err := contractstaking.BackfillOwnerIndex(
		ctx,
		contractstaking.NewContractStakingStateManager(sm),
		job.Contracts,
		job.Cursor,
		_ownerIndexBackfillPerBlock,
	)
	if err != nil {
		return 0, errors.Wrap(err, "staking: owner index backfill failed")
	}
	job.Cursor = cursor
	if err := writeOwnerIndexBackfillJob(sm, job); err != nil {
		return 0, err
	}
	if job.Done() {
		log.L().Info("IIP-59: owner index activation backfill complete",
			zap.Int("contracts", len(job.Contracts)))
	}
	return backfillProgress(job.Contracts, before, cursor), nil
}

// backfillProgress counts the bucket ids visited between two cursors, for
// metrics only. It is not used for control flow, so an over- or under-count
// here cannot affect state — hence the unconditional clamp rather than careful
// overflow handling on bounds no deployed contract can reach.
func backfillProgress(contracts []contractstaking.BackfillContract, from, to contractstaking.OwnerIndexBackfillCursor) int {
	n := uint64(0)
	if from.ContractIndex == to.ContractIndex {
		n = to.NextBucketID - from.NextBucketID
	} else {
		if from.ContractIndex < len(contracts) {
			n += contracts[from.ContractIndex].MaxBucketID + 1 - from.NextBucketID
		}
		for i := from.ContractIndex + 1; i < to.ContractIndex && i < len(contracts); i++ {
			n += contracts[i].MaxBucketID + 1
		}
		if to.ContractIndex < len(contracts) {
			n += to.NextBucketID
		}
	}
	if n > _ownerIndexBackfillPerBlock {
		return _ownerIndexBackfillPerBlock
	}
	return int(n)
}

// seedOwnerIndexBackfillJob builds the contract list, records each contract's
// bucket high-water mark, and persists the fresh record.
//
// # Where the contract list comes from
//
// The union of the three genesis staking contract addresses and every contract
// that already has a record in the contract meta namespace. Genesis is used
// rather than p.contractStakingIndexer*.ContractAddress() deliberately: which
// indexers a node has wired is node configuration, and this writes consensus
// state, so the list must come from something every node agrees on by
// construction. The meta namespace is folded in so a contract registered by
// some path other than genesis still gets walked.
//
// # Where each contract's bound comes from
//
// max(existing meta record, highest bucket id actually in state). The scan is
// what makes this work for V2 and V3, which never had a meta record: only the
// V1 indexer ever wrote one (see
// ContractStakingStateManager.RaiseNumOfBuckets). Contracts with neither are
// dropped from the list — they have no buckets, so there is nothing to index,
// and writing a mark of 0 for them would be a lie that says "bucket 0 existed".
//
// The resolved mark is written back through RaiseNumOfBuckets, which is the
// other half of the fix: without it the era window has no bound for V2/V3 and
// eracow.Window.ContractBucketExisted rejects every one of their buckets.
func seedOwnerIndexBackfillJob(ctx context.Context, sm protocol.StateManager) (*ownerIndexBackfillJob, error) {
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
			return nil, errors.Wrapf(err, "staking: bad system staking contract address %q in genesis", s)
		}
		byAddr[string(addr.Bytes())] = addr
	}
	marks, err := contractstaking.BucketHighWaterMarks(sm)
	if err != nil {
		return nil, err
	}
	known := make(map[string]uint64, len(marks))
	for _, m := range marks {
		addr, err := address.FromBytes(m.Contract)
		if err != nil {
			return nil, errors.Wrap(err, "staking: bad contract address in the staking contract meta namespace")
		}
		byAddr[string(m.Contract)] = addr
		known[string(m.Contract)] = m.NumOfBuckets
	}

	keys := make([]string, 0, len(byAddr))
	for k := range byAddr {
		keys = append(keys, k)
	}
	// Sorted by raw address bytes: the order is baked into the cursor and into
	// the resulting state, so map iteration order would make two nodes disagree.
	sort.Slice(keys, func(i, j int) bool { return bytes.Compare([]byte(keys[i]), []byte(keys[j])) < 0 })

	csr := contractstaking.NewStateReader(sm)
	csm := contractstaking.NewContractStakingStateManager(sm)
	job := &ownerIndexBackfillJob{Contracts: make([]contractstaking.BackfillContract, 0, len(keys))}
	for _, k := range keys {
		addr := byAddr[k]
		maxID, found, err := csr.MaxBucketIDInState(addr)
		if err != nil {
			return nil, err
		}
		if mark, ok := known[k]; ok {
			if !found || mark > maxID {
				maxID = mark
			}
			found = true
		}
		if !found {
			// No buckets and no mark: nothing to backfill. Any bucket it gets
			// from here on is created post-activation, so UpsertBucket indexes
			// it and RaiseNumOfBuckets creates the mark.
			continue
		}
		if err := csm.RaiseNumOfBuckets(addr, maxID); err != nil {
			return nil, err
		}
		job.Contracts = append(job.Contracts, contractstaking.BackfillContract{Address: addr, MaxBucketID: maxID})
	}
	log.L().Info("IIP-59: seeding owner index activation backfill",
		zap.Int("contracts", len(job.Contracts)))
	if err := writeOwnerIndexBackfillJob(sm, job); err != nil {
		return nil, err
	}
	return job, nil
}
