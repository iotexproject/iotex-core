// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos2

import (
	"github.com/facebookgo/clock"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

type rollDPoSCtx struct {
	cfg   config.RollDPoS
	id    string
	chain blockchain.Blockchain
	pool  delegate.Pool
	epoch epochCtx
	round roundCtx
	clock clock.Clock
}

// getRollingDelegates will only allows the delegates chosen for given epoch to enter the epoch
func (ctx *rollDPoSCtx) getRollingDelegates(epochNum uint64) ([]string, error) {
	// TODO: replace the pseudo roll delegates method with integrating with real delegate pool
	return ctx.pool.RollDelegates(epochNum)
}

// calcEpochNum calculates the epoch ordinal number and the epoch start height offset, which is based on the height of
// the next block to be produced
func (ctx *rollDPoSCtx) calcEpochNumAndHeight() (uint64, uint64, error) {
	height, err := ctx.chain.TipHeight()
	if err != nil {
		return 0, 0, err
	}
	numDlgs, err := ctx.pool.NumDelegatesPerEpoch()
	if err != nil {
		return 0, 0, err
	}
	subEpochNum := ctx.getNumSubEpochs()
	epochNum := height/(uint64(numDlgs)*uint64(subEpochNum)) + 1
	epochHeight := uint64(numDlgs)*uint64(subEpochNum)*(epochNum-1) + 1
	return epochNum, epochHeight, nil
}

// getNumSubEpochs returns max(configured number, 1)
func (ctx *rollDPoSCtx) getNumSubEpochs() uint {
	num := uint(1)
	if ctx.cfg.NumSubEpochs > 0 {
		num = ctx.cfg.NumSubEpochs
	}
	return num
}

// epochCtx keeps the context data for the current epoch
type epochCtx struct {
	// num is the ordinal number of an epoch
	num uint64
	// height means offset for current epochStart (i.e., the height of the first block generated in this epochStart)
	height uint64
	// numSubEpochs defines number of sub-epochs/rotations will happen in an epochStart
	numSubEpochs uint
	dkg          hash.DKGHash
	delegates    []string
}

// roundCtx keeps the context data for the current round and block.
type roundCtx struct {
	block     *blockchain.Block
	blockHash *hash.Hash32B
	prevotes  map[string]*hash.Hash32B
	votes     map[string]*hash.Hash32B
	isPr      bool
}
