// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package minifactory

import (
	"context"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state/factory"
)

// const represents operation
const (
	Try = iota + 1
	Commit
	Get
)

type (
	// stateDB implements StateFactory interface, tracks changes to account/contract and batch-commits to DB
	stateDB struct {
		*factory.StateDB
		commitBlock bool
		stopHeight  uint64
	}

	// StateDBOption sets stateDB construction parameter
	StateDBOption func(*stateDB)
)

// CommitBlockStateDBOption commits block on PutBlock
func CommitBlockStateDBOption() StateDBOption {
	return func(sdb *stateDB) {
		sdb.commitBlock = true
	}
}

// WithStopHeightStateDBOption sets stopHeight for uncommitted on PutBlock
func WithStopHeightStateDBOption(stopHeight uint64) StateDBOption {
	return func(sdb *stateDB) {
		sdb.stopHeight = stopHeight
	}
}

// NewStateDB creates a new state db
func NewStateDB(cfg factory.Config, dao db.KVStore, optsStateDB []factory.StateDBOption, opts ...StateDBOption) (factory.Factory, error) {
	sdb, err := factory.NewStateDB(cfg, dao, optsStateDB...)
	if err != nil {
		return nil, err
	}
	minisdb := stateDB{
		StateDB: sdb.(*factory.StateDB),
	}
	for _, opt := range opts {
		opt(&minisdb)
	}
	return &minisdb, nil
}

// PutBlock persists all changes in RunActions() into the DB
func (sdb *stateDB) PutBlock(ctx context.Context, blk *block.Block) error {
	ctx, ws, isExist, err := sdb.CreateWS(ctx, blk)
	if err != nil {
		return err
	}
	if err = sdb.TryBlock(ctx, blk, ws, isExist); err != nil {
		return err
	}
	if blk.Height() == sdb.stopHeight && !sdb.commitBlock {
		return nil
	}
	return sdb.CommitBlock(ctx, blk, ws)
}
