// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
)

var (
	ErrNotSychronized = errors.New("indexers are not synchronized")
)

// SyncIndexers is a special index that includes a master and multiple indexes,
// which stay in sync when blocks are added.
type SyncIndexers struct {
	master   blockdao.BlockIndexer
	indexers []blockdao.BlockIndexer
}

// NewSyncIndexers creates a new SyncIndexers
// each indexer will PutBlock one by one in the order of the indexers
func NewSyncIndexers(master blockdao.BlockIndexer, indexers ...blockdao.BlockIndexer) *SyncIndexers {
	return &SyncIndexers{
		master:   master,
		indexers: indexers}
}

// Start starts the indexer group
func (ig *SyncIndexers) Start(ctx context.Context) error {
	if err := ig.master.Start(ctx); err != nil {
		return err
	}
	for _, indexer := range ig.indexers {
		if err := indexer.Start(ctx); err != nil {
			return err
		}
	}
	return ig.checkSync()
}

// Stop stops the indexer group
func (ig *SyncIndexers) Stop(ctx context.Context) error {
	if err := ig.master.Stop(ctx); err != nil {
		return err
	}
	for _, indexer := range ig.indexers {
		if err := indexer.Stop(ctx); err != nil {
			return err
		}
	}
	return nil
}

// PutBlock puts a block into the indexers in the group
func (ig *SyncIndexers) PutBlock(ctx context.Context, blk *block.Block) error {
	for _, indexer := range ig.indexers {
		height, err := indexer.Height()
		if err != nil {
			return err
		}
		if blk.Height() <= height {
			// if the block is lower than the indexer's height, do nothing
			continue
		}
		// put block
		if err := indexer.PutBlock(ctx, blk); err != nil {
			return err
		}
	}
	return nil
}

// Height returns the height of the indexers in the group
// which must be same as master's
func (ig *SyncIndexers) Height() (uint64, error) {
	return ig.master.Height()
}

func (ig *SyncIndexers) checkSync() error {
	masterHeight, err := ig.master.Height()
	if err != nil {
		return err
	}
	// all other indexers must have same height as master to be in-sync
	for i, indexer := range ig.indexers {
		if start, ok := indexer.(interface{ IsActive() bool }); ok && !start.IsActive() {
			continue
		}
		tipHeight, err := indexer.Height()
		if err != nil {
			return err
		}
		if tipHeight != masterHeight {
			return errors.Wrapf(ErrNotSychronized, "indexer %d, expecting height = %d, actual height = %d", i, masterHeight, tipHeight)
		}
	}
	return nil
}
