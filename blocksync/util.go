// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"time"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
)

func commitBlock(bc blockchain.Blockchain, ap actpool.ActPool, blk *blockchain.Block) error {
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	// remove transfers in this block from ActPool and reset ActPool state
	ap.Reset()
	return nil
}

// findSyncStartHeight needs to find a reasonable start point to sync
// 1. current height + 1 if current height is not dummy
// 2. current height remove all dummy on top + 1
// 3. FIXME this node may still has issue, if it was following the wrong chain, this is actually a general version of 2, but in 3, we need to rollback blockchain first
func findSyncStartHeight(bc blockchain.Blockchain) (uint64, error) {
	var next uint64
	h := bc.TipHeight()
	for ; ; h-- {
		blk, err := bc.GetBlockByHeight(h)
		if err != nil {
			return next, err
		}
		if !blk.IsDummyBlock() {
			next = h + 1
			break
		}
		if h == 0 { // 0 height is dummy
			return next, errors.New("0 height block is dummy block")
		}
	}
	return next, nil
}

// syncTaskInterval returns the recurring sync task interval, or 0 if this config should not need to run sync task
func syncTaskInterval(cfg *config.Config) time.Duration {
	if cfg.IsLightweight() {
		return time.Duration(0)
	}

	interval := cfg.BlockSync.Interval

	if cfg.IsFullnode() {
		// fullnode has less stringent requirement of staying in sync so can check less frequently
		interval <<= 2
	}
	return interval
}
