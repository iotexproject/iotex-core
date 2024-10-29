// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
)

// SyncIndexers is a special index that includes multiple indexes,
// which stay in sync when blocks are added.
type SyncIndexers struct {
	indexers []blockdao.BlockIndexer
}

// NewSyncIndexers creates a new SyncIndexers
// each indexer will PutBlock one by one in the order of the indexers
func NewSyncIndexers(indexers ...blockdao.BlockIndexer) *SyncIndexers {
	return &SyncIndexers{indexers: indexers}
}

// Start starts the indexer group
func (ig *SyncIndexers) Start(ctx context.Context) error {
	for _, indexer := range ig.indexers {
		if err := indexer.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Stop stops the indexer group
func (ig *SyncIndexers) Stop(ctx context.Context) error {
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
		// check if the block is higher than the indexer's height
		height, err := indexer.Height()
		if err != nil {
			return err
		}
		if blk.Height() <= height {
			continue
		}
		// put block
		if err := indexer.PutBlock(ctx, blk); err != nil {
			return err
		}
	}
	return nil
}

// DeleteTipBlock deletes the tip block from the indexers in the group
func (ig *SyncIndexers) DeleteTipBlock(ctx context.Context, blk *block.Block) error {
	for _, indexer := range ig.indexers {
		if err := indexer.DeleteTipBlock(ctx, blk); err != nil {
			return err
		}
	}
	return nil
}

// Height returns the minimum height of the indexers in the group
func (ig *SyncIndexers) Height() (uint64, error) {
	var height uint64
	for i, indexer := range ig.indexers {
		h, err := indexer.Height()
		if err != nil {
			return 0, err
		}
		if i == 0 || h < height {
			height = h
		}
	}
	return height, nil
}
