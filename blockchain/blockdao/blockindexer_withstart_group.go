// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"errors"

	"github.com/iotexproject/iotex-core/blockchain/block"
)

type (
	// BlockIndexerWithStart defines an interface to accept block to build index from a start height
	BlockIndexerWithStart interface {
		BlockIndexer
		// StartHeight returns the start height of the indexer
		StartHeight() uint64
	}

	// BlockIndexerWithStartGroup defines a group of block indexers with start height
	BlockIndexerWithStartGroup struct {
		indexers      []BlockIndexerWithStart
		tipHeightFunc func() (uint64, error)
	}
)

// NewBlockIndexerWithStartGroup creates a new block indexer group
func NewBlockIndexerWithStartGroup(indexers ...BlockIndexerWithStart) *BlockIndexerWithStartGroup {
	return &BlockIndexerWithStartGroup{
		indexers: indexers,
	}
}

// SetTipHeightFunc sets the max valid height getter
// it must be set before Start
func (g *BlockIndexerWithStartGroup) SetTipHeightFunc(f func() (uint64, error)) {
	g.tipHeightFunc = f
}

// Start starts the block indexer group
func (g *BlockIndexerWithStartGroup) Start(ctx context.Context) error {
	if g.tipHeightFunc == nil {
		return errors.New("tipHeightFunc is not set")
	}
	for _, indexer := range g.indexers {
		if err := indexer.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Stop stops the block indexer group
func (g *BlockIndexerWithStartGroup) Stop(ctx context.Context) error {
	for _, indexer := range g.indexers {
		if err := indexer.Stop(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Height returns the height of the block indexer group
// first, for every indexer, use max of Height and StartHeight
// then, use the min of all indexers
// finally, use the min of the max and maxValidHeight
func (g *BlockIndexerWithStartGroup) Height() (uint64, error) {
	tipHeight, err := g.tipHeightFunc()
	if err != nil {
		return 0, err
	}

	for _, indexer := range g.indexers {
		indexerHeight, err := indexer.Height()
		if err != nil {
			return 0, err
		}
		if indexerHeight > tipHeight {
			return 0, errors.New("indexer height is greater than tip height")
		}
		indexerStartHeight := indexer.StartHeight()
		receivedHeight := indexerStartHeight - 1
		// use max of Height and StartHeight
		if receivedHeight > indexerHeight {
			indexerHeight = receivedHeight
		}
		if indexerHeight < tipHeight {
			tipHeight = indexerHeight
		}
	}

	return tipHeight, nil
}

// PutBlock puts a block into the block indexer group
// for every indexer, if StartHeight <= block.Height && Height < block.Height, put the block
func (g *BlockIndexerWithStartGroup) PutBlock(ctx context.Context, blk *block.Block) error {
	for _, indexer := range g.indexers {
		indexerHeight, err := indexer.Height()
		if err != nil {
			return err
		}
		if indexer.StartHeight() > blk.Height() || indexerHeight >= blk.Height() {
			continue
		}
		if err := indexer.PutBlock(ctx, blk); err != nil {
			return err
		}
	}
	return nil
}

// DeleteTipBlock deletes the tip block from the block indexer group
func (g *BlockIndexerWithStartGroup) DeleteTipBlock(context.Context, *block.Block) error {
	return errors.New("not implemented")
}
