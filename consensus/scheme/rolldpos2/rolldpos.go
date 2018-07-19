// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos2

import (
	"time"

	"github.com/facebookgo/clock"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

type rollDPoSCtx struct {
	cfg     config.RollDPoS
	addr    *iotxaddress.Address
	chain   blockchain.Blockchain
	actPool actpool.ActPool
	pool    delegate.Pool
	p2p     network.Overlay
	epoch   epochCtx
	round   roundCtx
	clock   clock.Clock
}

// rollingDelegates will only allows the delegates chosen for given epoch to enter the epoch
func (ctx *rollDPoSCtx) rollingDelegates(epochNum uint64) ([]string, error) {
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

// generateDKG generates a pseudo DKG bytes
func (ctx *rollDPoSCtx) generateDKG() (hash.DKGHash, error) {
	var dkg hash.DKGHash
	// TODO: fill the logic to generate DKG
	return dkg, nil
}

// getNumSubEpochs returns max(configured number, 1)
func (ctx *rollDPoSCtx) getNumSubEpochs() uint {
	num := uint(1)
	if ctx.cfg.NumSubEpochs > 0 {
		num = ctx.cfg.NumSubEpochs
	}
	return num
}

// rotatedProposer will rotate among the delegates to choose the proposer. It is pseudo order based on the position
// in the delegate list and the block height
func (ctx *rollDPoSCtx) rotatedProposer() (string, uint64, error) {
	height, err := ctx.chain.TipHeight()
	if err != nil {
		return "", 0, err
	}
	// Next block height
	height++
	numDelegates := len(ctx.epoch.delegates)
	if numDelegates == 0 {
		return "", 0, delegate.ErrZeroDelegate
	}
	return ctx.epoch.delegates[(height)%uint64(numDelegates)], height, nil
}

// mintBlock picks the actions and creates an block to propose
func (ctx *rollDPoSCtx) mintBlock() (*blockchain.Block, error) {
	transfers, votes := ctx.actPool.PickActs()
	logger.Debug().
		Int("transfer", len(transfers)).
		Int("votes", len(votes)).
		Msg("pick actions from the action pool")
	blk, err := ctx.chain.MintNewBlock(transfers, votes, ctx.addr, "")
	if err != nil {
		logger.Error().Msg("error when minting a block")
		return nil, err
	}
	logger.Info().
		Uint64("height", blk.Height()).
		Int("transfers", len(blk.Transfers)).
		Int("votes", len(blk.Votes)).
		Msg("minted a new block")
	return blk, nil
}

// calcDurationSinceLastBlock returns the duration since last block time
func (ctx *rollDPoSCtx) calcDurationSinceLastBlock() (time.Duration, error) {
	height, err := ctx.chain.TipHeight()
	if err != nil {
		return 0, errors.Wrap(err, "error when getting the blockchain height")
	}
	blk, err := ctx.chain.GetBlockByHeight(height)
	if err != nil {
		return 0, errors.Wrapf(err, "error when getting the block at height: %d", height)
	}
	lastBlkTime := time.Unix(int64(blk.ConvertToBlockHeaderPb().Timestamp), 0)
	return time.Since(lastBlkTime), nil
}

// calcQuorum calculates if more than 2/3 vote yes or no
func (ctx *rollDPoSCtx) calcQuorum(decisions map[string]bool) (bool, bool) {
	yes := 0
	no := 0
	for _, decision := range decisions {
		if decision {
			yes++
		} else {
			no++
		}
	}
	numDelegates := len(ctx.epoch.delegates)
	return yes >= numDelegates*2/3+1, no >= numDelegates*2/3+1
}

// isEpochFinished checks the epoch is finished or not
func (ctx *rollDPoSCtx) isEpochFinished() (bool, error) {
	height, err := ctx.chain.TipHeight()
	if err != nil {
		return false, errors.Wrap(err, "error when getting the blockchain height")
	}
	// if the height of the last committed block is already the last one should be minted from this epochStart, go back
	// to epochStart start
	if height >= ctx.epoch.height+uint64(uint(len(ctx.epoch.delegates))*ctx.epoch.numSubEpochs)-1 {
		return true, nil
	}
	return false, nil
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
	block    *blockchain.Block
	prevotes map[string]bool
	votes    map[string]bool
	proposer string
}
