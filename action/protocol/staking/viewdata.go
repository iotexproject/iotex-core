// Copyright (c) 2025 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
)

type (
	// BucketReader defines the interface to read bucket info
	BucketReader interface {
		DeductBucket(address.Address, uint64) (*contractstaking.Bucket, error)
	}

	// ContractStakeView is the interface for contract stake view
	ContractStakeView interface {
		// Wrap wraps the contract stake view
		Wrap() ContractStakeView
		// Fork forks the contract stake view, commit will not affect the original view
		Fork() ContractStakeView
		// IsDirty checks if the contract stake view is dirty
		IsDirty() bool
		// Commit commits the contract stake view
		Commit(context.Context, protocol.StateManager) error
		// CreatePreStates creates pre states for the contract stake view
		CreatePreStates(ctx context.Context) error
		// Handle handles the receipt for the contract stake view
		Handle(ctx context.Context, receipt *action.Receipt) error
		// Migrate writes the bucket types and buckets to the state manager
		Migrate(context.Context, EventHandler) error
		// Revise updates the contract stake view with the latest bucket data
		Revise(context.Context)
		// BucketsByCandidate returns the buckets by candidate address
		CandidateStakeVotes(ctx context.Context, id address.Address) *big.Int
		AddBlockReceipts(ctx context.Context, receipts []*action.Receipt) error
	}
	// viewData is the data that need to be stored in protocol's view
	viewData struct {
		candCenter     *CandidateCenter
		bucketPool     *BucketPool
		snapshots      []Snapshot
		contractsStake *contractStakeView
		// voterWeights is the per-(candidate, voter) weighted-votes aggregate
		// used by IIP-59 voter reward distribution. nil while
		// NoVoterRewardDistribution is true (pre-fork); populated by
		// CreateBaseView once on the first block after the feature activates,
		// then maintained incrementally by every staking handler that changes
		// a bucket's contribution to a candidate.
		voterWeights *VoterWeightView
	}
	Snapshot struct {
		size           int
		changes        int
		amount         *big.Int
		count          uint64
		contractsStake *contractStakeView
		// voterWeights captures the full view state at the snapshot moment so
		// Revert restores it byte-for-byte. The clone is deep — Apply is the
		// single mutation path and mutates in place, so a shallow snapshot
		// would not survive any subsequent change. nil when the IIP-59
		// feature flag is off.
		voterWeights *VoterWeightView
	}
	contractStakeView struct {
		v1 ContractStakeView
		v2 ContractStakeView
		v3 ContractStakeView
	}
)

func (v *viewData) Fork() protocol.View {
	fork := &viewData{}
	fork.candCenter = v.candCenter.Clone()
	fork.bucketPool = v.bucketPool.Clone()
	fork.snapshots = make([]Snapshot, len(v.snapshots))
	for i := range v.snapshots {
		fork.snapshots[i] = Snapshot{
			size:           v.snapshots[i].size,
			changes:        v.snapshots[i].changes,
			amount:         new(big.Int).Set(v.snapshots[i].amount),
			count:          v.snapshots[i].count,
			contractsStake: v.snapshots[i].contractsStake,
			voterWeights:   v.snapshots[i].voterWeights.Clone(),
		}
	}
	fork.contractsStake = v.contractsStake.Fork()
	fork.voterWeights = v.voterWeights.Clone()
	return fork
}

func (v *viewData) Commit(ctx context.Context, sm protocol.StateManager) error {
	if err := v.candCenter.Commit(ctx, sm); err != nil {
		return err
	}
	if err := v.bucketPool.Commit(); err != nil {
		return err
	}
	if err := v.contractsStake.Commit(ctx, sm); err != nil {
		return err
	}
	if err := v.commitVoterWeights(sm); err != nil {
		return err
	}
	v.snapshots = []Snapshot{}

	return nil
}

func (v *viewData) IsDirty() bool {
	return v.candCenter.IsDirty() ||
		v.bucketPool.IsDirty() ||
		v.contractsStake.IsDirty() ||
		v.voterWeights.IsDirty()
}

// commitVoterWeights persists the IIP-59 view's deterministic state digest
// (one Hash256 under the _voterWeights namespace tag) when the view is
// dirty. Other nodes' digests at the same block height must match
// byte-for-byte; a mismatch surfaces as a divergent block hash because
// state-trie writes feed into deltaStateDigest.
//
// nil voterWeights (feature flag off) is a no-op so pre-flag chains
// remain byte-identical to today's behavior.
func (v *viewData) commitVoterWeights(sm protocol.StateManager) error {
	if v.voterWeights == nil || !v.voterWeights.IsDirty() {
		return nil
	}
	h := v.voterWeights.Hash()
	if _, err := sm.PutState(
		&voterWeightDigest{Hash: h},
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(_voterWeightsKey),
	); err != nil {
		return errors.Wrap(err, "failed to persist voter weights digest")
	}
	v.voterWeights.MarkClean()
	return nil
}

func (v *viewData) Snapshot() int {
	snapshot := len(v.snapshots)
	wrapped := v.contractsStake.Wrap()
	v.snapshots = append(v.snapshots, Snapshot{
		size:           v.candCenter.size,
		changes:        len(v.candCenter.change.candidates),
		amount:         new(big.Int).Set(v.bucketPool.total.amount),
		count:          v.bucketPool.total.count,
		contractsStake: v.contractsStake,
		voterWeights:   v.voterWeights.Clone(),
	})
	v.contractsStake = wrapped
	return snapshot
}

func (v *viewData) Revert(snapshot int) error {
	if snapshot < 0 || snapshot >= len(v.snapshots) {
		return errors.Errorf("invalid snapshot index %d", snapshot)
	}
	s := v.snapshots[snapshot]
	change, err := listToCandChange(v.candCenter.change.items()[:s.changes])
	if err != nil {
		return err
	}
	v.candCenter.size = s.size
	v.candCenter.change = change
	v.bucketPool.total.amount.Set(s.amount)
	v.bucketPool.total.count = s.count
	v.contractsStake = s.contractsStake
	v.voterWeights = s.voterWeights
	v.snapshots = v.snapshots[:snapshot]
	return nil
}

func (csv *contractStakeView) Revise(ctx context.Context) {
	if csv.v1 != nil {
		csv.v1.Revise(ctx)
	}
	if csv.v2 != nil {
		csv.v2.Revise(ctx)
	}
	if csv.v3 != nil {
		csv.v3.Revise(ctx)
	}
}

func (csv *contractStakeView) Migrate(ctx context.Context, nftHandler EventHandler) error {
	if csv.v1 != nil {
		if err := csv.v1.Migrate(ctx, nftHandler); err != nil {
			return err
		}
	}
	if csv.v2 != nil {
		if err := csv.v2.Migrate(ctx, nftHandler); err != nil {
			return err
		}
	}
	if csv.v3 != nil {
		if err := csv.v3.Migrate(ctx, nftHandler); err != nil {
			return err
		}
	}
	return nil
}

func (csv *contractStakeView) Wrap() *contractStakeView {
	if csv == nil {
		return nil
	}
	wrapped := &contractStakeView{}
	if csv.v1 != nil {
		wrapped.v1 = csv.v1.Wrap()
	}
	if csv.v2 != nil {
		wrapped.v2 = csv.v2.Wrap()
	}
	if csv.v3 != nil {
		wrapped.v3 = csv.v3.Wrap()
	}
	return wrapped
}

func (csv *contractStakeView) Fork() *contractStakeView {
	if csv == nil {
		return nil
	}
	clone := &contractStakeView{}
	if csv.v1 != nil {
		clone.v1 = csv.v1.Fork()
	}
	if csv.v2 != nil {
		clone.v2 = csv.v2.Fork()
	}
	if csv.v3 != nil {
		clone.v3 = csv.v3.Fork()
	}
	return clone
}

func (csv *contractStakeView) CreatePreStates(ctx context.Context) error {
	if csv.v1 != nil {
		if err := csv.v1.CreatePreStates(ctx); err != nil {
			return err
		}
	}
	if csv.v2 != nil {
		if err := csv.v2.CreatePreStates(ctx); err != nil {
			return err
		}
	}
	if csv.v3 != nil {
		if err := csv.v3.CreatePreStates(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (csv *contractStakeView) IsDirty() bool {
	if csv == nil {
		return false
	}
	if csv.v1 != nil && csv.v1.IsDirty() {
		return true
	}
	if csv.v2 != nil && csv.v2.IsDirty() {
		return true
	}
	if csv.v3 != nil && csv.v3.IsDirty() {
		return true
	}
	return false
}

func (csv *contractStakeView) Commit(ctx context.Context, sm protocol.StateManager) error {
	if csv == nil {
		return nil
	}
	featureCtx, ok := protocol.GetFeatureCtx(ctx)
	if !ok || !featureCtx.StoreVoteOfNFTBucketIntoView {
		sm = nil
	}
	if csv.v1 != nil {
		if err := csv.v1.Commit(ctx, sm); err != nil {
			return err
		}
	}
	if csv.v2 != nil {
		if err := csv.v2.Commit(ctx, sm); err != nil {
			return err
		}
	}
	if csv.v3 != nil {
		if err := csv.v3.Commit(ctx, sm); err != nil {
			return err
		}
	}
	return nil
}

func (csv *contractStakeView) Handle(ctx context.Context, receipt *action.Receipt) error {
	if csv.v1 != nil {
		if err := csv.v1.Handle(ctx, receipt); err != nil {
			return err
		}
	}
	if csv.v2 != nil {
		if err := csv.v2.Handle(ctx, receipt); err != nil {
			return err
		}
	}
	if csv.v3 != nil {
		if err := csv.v3.Handle(ctx, receipt); err != nil {
			return err
		}
	}
	return nil
}
