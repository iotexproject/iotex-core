// Copyright (c) 2025 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/pkg/errors"
)

type (
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
		Migrate(EventHandler) error
		// BucketsByCandidate returns the buckets by candidate address
		BucketsByCandidate(ownerAddr address.Address) ([]*VoteBucket, error)
		AddBlockReceipts(ctx context.Context, receipts []*action.Receipt) error
	}
	// viewData is the data that need to be stored in protocol's view
	viewData struct {
		candCenter     *CandidateCenter
		bucketPool     *BucketPool
		snapshots      []Snapshot
		contractsStake *contractStakeView
	}
	Snapshot struct {
		size           int
		changes        int
		amount         *big.Int
		count          uint64
		contractsStake *contractStakeView
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
		}
	}
	fork.contractsStake = v.contractsStake.Fork()
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
	v.snapshots = []Snapshot{}

	return nil
}

func (v *viewData) IsDirty() bool {
	return v.candCenter.IsDirty() || v.bucketPool.IsDirty() || v.contractsStake.IsDirty()
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
	v.snapshots = v.snapshots[:snapshot]
	return nil
}

func (csv *contractStakeView) Migrate(nftHandler EventHandler) error {
	if csv.v1 != nil {
		if err := csv.v1.Migrate(nftHandler); err != nil {
			return err
		}
	}
	if csv.v2 != nil {
		if err := csv.v2.Migrate(nftHandler); err != nil {
			return err
		}
	}
	if csv.v3 != nil {
		if err := csv.v3.Migrate(nftHandler); err != nil {
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
