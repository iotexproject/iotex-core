// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"time"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/endorsement"
)

var errInvalidCurrentTime = errors.New("invalid current time")

type roundCalculator struct {
	chain                ChainManager
	timeBasedRotation    bool
	rp                   *rolldpos.Protocol
	delegatesByEpochFunc DelegatesByEpochFunc
	beringHeight         uint64
	blockTime            *blockTime
}

// UpdateRound updates previous roundCtx
func (c *roundCalculator) UpdateRound(round *roundCtx, height uint64, blockInterval time.Duration, now time.Time, toleratedOvertime time.Duration) (*roundCtx, error) {
	epochNum := round.EpochNum()
	epochStartHeight := round.EpochStartHeight()
	delegates := round.Delegates()
	switch {
	case height < round.Height():
		return nil, errors.New("cannot update to a lower height")
	case height == round.Height():
		if now.Before(round.StartTime()) {
			return round, nil
		}
	default:
		if height >= round.NextEpochStartHeight() {
			// update the epoch
			epochNum = c.rp.GetEpochNum(height)
			epochStartHeight = c.rp.GetEpochHeight(epochNum)
			var err error
			if delegates, err = c.Delegates(height); err != nil {
				return nil, err
			}
		}
	}
	roundNum, roundStartTime, err := c.roundInfo(height, blockInterval, now, toleratedOvertime)
	if err != nil {
		return nil, err
	}
	proposer, err := c.calculateProposer(height, roundNum, delegates)
	if err != nil {
		return nil, err
	}
	var status status
	var blockInLock []byte
	var proofOfLock []*endorsement.Endorsement
	if height == round.Height() {
		err = round.eManager.Cleanup(roundStartTime)
		if err != nil {
			return nil, err
		}
		status = round.status
		blockInLock = round.blockInLock
		proofOfLock = round.proofOfLock
	} else {
		err = round.eManager.Cleanup(time.Time{})
		if err != nil {
			return nil, err
		}
	}
	return &roundCtx{
		epochNum:             epochNum,
		epochStartHeight:     epochStartHeight,
		nextEpochStartHeight: c.rp.GetEpochHeight(epochNum + 1),
		delegates:            delegates,

		height:             height,
		roundNum:           roundNum,
		proposer:           proposer,
		roundStartTime:     roundStartTime,
		nextRoundStartTime: roundStartTime.Add(blockInterval),
		eManager:           round.eManager,
		status:             status,
		blockInLock:        blockInLock,
		proofOfLock:        proofOfLock,
	}, nil
}

// Proposer returns the block producer of the round
func (c *roundCalculator) Proposer(height uint64, blockInterval time.Duration, roundStartTime time.Time) string {
	round, err := c.newRound(height, blockInterval, roundStartTime, nil, 0)
	if err != nil {
		return ""
	}

	return round.Proposer()
}

func (c *roundCalculator) IsDelegate(addr string, height uint64) bool {
	delegates, err := c.Delegates(height)
	if err != nil {
		return false
	}
	for _, d := range delegates {
		if addr == d {
			return true
		}
	}

	return false
}

// RoundInfo returns information of round by the given height and current time
func (c *roundCalculator) RoundInfo(
	height uint64,
	blockInterval time.Duration,
	now time.Time,
) (roundNum uint32, roundStartTime time.Time, err error) {
	return c.roundInfo(height, blockInterval, now, 0)
}

func (c *roundCalculator) roundInfo(
	height uint64,
	blockInterval time.Duration,
	now time.Time,
	toleratedOvertime time.Duration,
) (roundNum uint32, roundStartTime time.Time, err error) {
	lastBlockTime := c.blockTime.GenesisTime()
	if height > 1 {
		if height >= c.beringHeight {
			var lastBlkProposeTime time.Time
			lastBlkProposeTime, err = c.blockTime.BlockProposeTime(height - 1)
			if err != nil {
				return
			}
			lastBlockTime = lastBlockTime.Add(lastBlkProposeTime.Sub(lastBlockTime) / blockInterval * blockInterval)
		} else {
			var lastBlkCommitTime time.Time
			lastBlkCommitTime, err = c.blockTime.BlockCommitTime(height - 1)
			if err != nil {
				return
			}
			lastBlockTime = lastBlockTime.Add(lastBlkCommitTime.Sub(lastBlockTime) / blockInterval * blockInterval)
		}
	}
	if !lastBlockTime.Before(now) {
		// TODO: if this is the case, it is possible that the system time is far behind the time of other nodes.
		// better error handling may be needed on the caller side
		err = errors.Wrapf(
			errInvalidCurrentTime,
			"last block time %s is after than current time %s",
			lastBlockTime,
			now,
		)
		return
	}
	duration := now.Sub(lastBlockTime)
	if duration > blockInterval {
		roundNum = uint32(duration / blockInterval)
		if toleratedOvertime == 0 || duration%blockInterval < toleratedOvertime {
			roundNum--
		}
	}
	roundStartTime = lastBlockTime.Add(time.Duration(roundNum+1) * blockInterval)

	return roundNum, roundStartTime, nil
}

// Delegates returns list of delegates at given height
func (c *roundCalculator) Delegates(height uint64) ([]string, error) {
	epochNum := c.rp.GetEpochNum(height)
	return c.delegatesByEpochFunc(epochNum)
}

// NewRoundWithToleration starts new round with tolerated over time
func (c *roundCalculator) NewRoundWithToleration(
	height uint64,
	blockInterval time.Duration,
	now time.Time,
	eManager *endorsementManager,
	toleratedOvertime time.Duration,
) (round *roundCtx, err error) {
	return c.newRound(height, blockInterval, now, eManager, toleratedOvertime)
}

// NewRound starts new round and returns roundCtx
func (c *roundCalculator) NewRound(
	height uint64,
	blockInterval time.Duration,
	now time.Time,
	eManager *endorsementManager,
) (round *roundCtx, err error) {
	return c.newRound(height, blockInterval, now, eManager, 0)
}

func (c *roundCalculator) newRound(
	height uint64,
	blockInterval time.Duration,
	now time.Time,
	eManager *endorsementManager,
	toleratedOvertime time.Duration,
) (round *roundCtx, err error) {
	epochNum := uint64(0)
	epochStartHeight := uint64(0)
	var delegates []string
	var roundNum uint32
	var proposer string
	var roundStartTime time.Time
	if height != 0 {
		epochNum = c.rp.GetEpochNum(height)
		epochStartHeight = c.rp.GetEpochHeight(epochNum)
		if delegates, err = c.Delegates(height); err != nil {
			return
		}
		if roundNum, roundStartTime, err = c.roundInfo(height, blockInterval, now, toleratedOvertime); err != nil {
			return
		}
		if proposer, err = c.calculateProposer(height, roundNum, delegates); err != nil {
			return
		}
	}
	if eManager == nil {
		if eManager, err = newEndorsementManager(nil, nil); err != nil {
			return nil, err
		}
	}
	round = &roundCtx{
		epochNum:             epochNum,
		epochStartHeight:     epochStartHeight,
		nextEpochStartHeight: c.rp.GetEpochHeight(epochNum + 1),
		delegates:            delegates,

		height:             height,
		roundNum:           roundNum,
		proposer:           proposer,
		eManager:           eManager,
		roundStartTime:     roundStartTime,
		nextRoundStartTime: roundStartTime.Add(blockInterval),
		status:             _open,
	}
	eManager.SetIsMarjorityFunc(round.EndorsedByMajority)

	return round, nil
}

// calculateProposer calulates proposer according to height and round number
func (c *roundCalculator) calculateProposer(
	height uint64,
	round uint32,
	delegates []string,
) (proposer string, err error) {
	numDelegates := c.rp.NumDelegates()
	if numDelegates != uint64(len(delegates)) {
		err = errors.New("invalid delegate list")
		return
	}
	idx := height
	if c.timeBasedRotation {
		idx += uint64(round)
	}
	proposer = delegates[idx%numDelegates]
	return
}

type blockTime struct {
	bc blockchain.Blockchain
}

// BlockProposeTime return propose time by height
func (bt *blockTime) BlockProposeTime(height uint64) (time.Time, error) {
	header, err := bt.bc.BlockHeaderByHeight(height)
	if err != nil {
		return time.Now(), errors.Wrapf(
			err, "error when getting the block at height: %d",
			height,
		)
	}
	return header.Timestamp(), nil
}

// BlockCommitTime return commit time by height
func (bt *blockTime) BlockCommitTime(height uint64) (time.Time, error) {
	footer, err := bt.bc.BlockFooterByHeight(height)
	if err != nil {
		return time.Now(), errors.Wrapf(
			err, "error when getting the block at height: %d",
			height,
		)
	}
	return footer.CommitTime(), nil
}

// GenesisTime return Genesis time by default
func (bt *blockTime) GenesisTime() time.Time {
	return time.Unix(bt.bc.Genesis().Timestamp, 0)
}
