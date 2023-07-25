// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
)

// Index group is a special index that includes multiple indexes,
// which stay in sync when blocks are added.
type IndexerGroup struct {
	indexers []blockdao.BlockIndexer
}

// NewIndexerGroup creates a new indexer group
func NewIndexerGroup(indexers ...blockdao.BlockIndexer) *IndexerGroup {
	return &IndexerGroup{indexers: indexers}
}

// Start starts the indexer group
func (ig *IndexerGroup) Start(ctx context.Context) error {
	for _, indexer := range ig.indexers {
		if err := indexer.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Stop stops the indexer group
func (ig *IndexerGroup) Stop(ctx context.Context) error {
	for _, indexer := range ig.indexers {
		if err := indexer.Stop(ctx); err != nil {
			return err
		}
	}
	return nil
}

// PutBlock puts a block into the indexers in the group
func (ig *IndexerGroup) PutBlock(ctx context.Context, blk *block.Block) error {
	for _, indexer := range ig.indexers {
		// check if the indexer is a BlockIndexerWithStart
		if indexerWithStart, ok := indexer.(blockdao.BlockIndexerWithStart); ok {
			startHeight, err := indexerWithStart.StartHeight()
			if err != nil {
				return err
			}
			if blk.Height() < startHeight {
				continue
			}
		}
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
func (ig *IndexerGroup) DeleteTipBlock(ctx context.Context, blk *block.Block) error {
	for _, indexer := range ig.indexers {
		if err := indexer.DeleteTipBlock(ctx, blk); err != nil {
			return err
		}
	}
	return nil
}

// StartHeight returns the minimum start height of the indexers in the group
func (ig *IndexerGroup) StartHeight() (uint64, error) {
	var result uint64
	for i, indexer := range ig.indexers {
		tipHeight, err := indexer.Height()
		if err != nil {
			return 0, err
		}
		indexStartHeight := tipHeight + 1
		if indexerWithStart, ok := indexer.(blockdao.BlockIndexerWithStart); ok {
			startHeight, err := indexerWithStart.StartHeight()
			if err != nil {
				return 0, err
			}
			if startHeight > indexStartHeight {
				indexStartHeight = startHeight
			}
		}
		if i == 0 || indexStartHeight < result {
			result = indexStartHeight
		}
	}
	return result, nil
}

// Height returns the minimum height of the indexers in the group
func (ig *IndexerGroup) Height() (uint64, error) {
	var height uint64
	for _, indexer := range ig.indexers {
		h, err := indexer.Height()
		if err != nil {
			return 0, err
		}
		if height == 0 || h < height {
			height = h
		}
	}
	return height, nil
}
