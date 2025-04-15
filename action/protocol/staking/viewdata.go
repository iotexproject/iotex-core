// Copyright (c) 2025 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/pkg/errors"
)

type (
	// ViewData is the data that need to be stored in protocol's view
	ViewData struct {
		candCenter *CandidateCenter
		bucketPool *BucketPool
		snapshots  []Snapshot
	}
	Snapshot struct {
		size    int
		changes int
		amount  *big.Int
		count   uint64
	}
)

func (v *ViewData) Clone() protocol.View {
	clone := &ViewData{}
	clone.candCenter = v.candCenter.Clone()
	clone.bucketPool = v.bucketPool.Clone()
	clone.snapshots = make([]Snapshot, len(v.snapshots))
	for i := range v.snapshots {
		clone.snapshots[i] = Snapshot{
			size:    v.snapshots[i].size,
			changes: v.snapshots[i].changes,
			amount:  new(big.Int).Set(v.snapshots[i].amount),
			count:   v.snapshots[i].count,
		}
	}

	return clone
}

func (v *ViewData) Commit(ctx context.Context, sr protocol.StateReader) error {
	height, err := sr.Height()
	if err != nil {
		return err
	}
	if featureWithHeightCtx, ok := protocol.GetFeatureWithHeightCtx(ctx); ok && featureWithHeightCtx.CandCenterHasAlias(height) {
		if err := v.candCenter.LegacyCommit(); err != nil {
			return err
		}
	} else {
		if err := v.candCenter.Commit(); err != nil {
			return err
		}
	}
	if err := v.bucketPool.Commit(sr); err != nil {
		return err
	}
	v.snapshots = []Snapshot{}

	return nil
}

func (v *ViewData) IsDirty() bool {
	return v.candCenter.IsDirty() || v.bucketPool.IsDirty()
}

func (v *ViewData) Snapshot() int {
	snapshot := len(v.snapshots)
	v.snapshots = append(v.snapshots, Snapshot{
		size:    v.candCenter.size,
		changes: v.candCenter.change.size(),
		amount:  new(big.Int).Set(v.bucketPool.total.amount),
		count:   v.bucketPool.total.count,
	})
	return snapshot
}

func (v *ViewData) Revert(snapshot int) error {
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
	v.snapshots = v.snapshots[:snapshot]
	return nil
}
