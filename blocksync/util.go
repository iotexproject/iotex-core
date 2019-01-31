// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"time"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
)

func commitBlock(bc blockchain.Blockchain, ap actpool.ActPool, cs consensus.Consensus, blk *block.Block) error {
	if err := cs.ValidateBlockFooter(blk); err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	cs.Calibrate(blk.Height())
	// remove transfers in this block from ActPool and reset ActPool state
	ap.Reset()
	return nil
}

// syncTaskInterval returns the recurring sync task interval, or 0 if this config should not need to run sync task
func syncTaskInterval(cfg config.Config) time.Duration {
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
