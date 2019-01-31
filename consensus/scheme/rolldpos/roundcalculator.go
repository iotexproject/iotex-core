// Copyright (c) 2019 IoTeX
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
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/endorsement"
)

type roundCalculator struct {
	chain                  blockchain.Blockchain
	blockInterval          time.Duration
	toleratedOvertime      time.Duration
	timeBasedRotation      bool
	rp                     *rolldpos.Protocol
	candidatesByHeightFunc CandidatesByHeightFunc
}

func (c *roundCalculator) BlockInterval() time.Duration {
	return c.blockInterval
}

func (c *roundCalculator) UpdateRound(round *roundCtx, height uint64, now time.Time) (*roundCtx, error) {
	epochNum := round.EpochNum()
	epochStartHeight := round.EpochStartHeight()
	delegates := round.Delegates()
	switch {
	case height < round.Height():
		return nil, errors.New("cannot update to a lower height")
	case height == round.Height():
		if now.Before(round.StartTime()) {
			return nil, errors.New("cannot update to a past time")
		}
	default:
		if height >= round.NextEpochStartHeight() {
			epochNum = c.rp.GetEpochNum(height)
			epochStartHeight = c.rp.GetEpochHeight(epochNum)
			var err error
			if delegates, err = c.Delegates(epochStartHeight); err != nil {
				return nil, err
			}
		}
	}
	roundNum, roundStartTime, err := c.roundInfo(height, now, true)
	if err != nil {
		return nil, err
	}
	var eManager *endorsementManager
	var status status
	var blockInLock []byte
	var proofOfLock []*endorsement.Endorsement
	if height == round.Height() {
		eManager = round.eManager.Cleanup(roundStartTime)
		status = round.status
		blockInLock = round.blockInLock
		proofOfLock = round.proofOfLock
	} else {
		eManager = newEndorsementManager()
	}
	proposer, err := c.calculateProposer(height, roundNum, delegates)
	if err != nil {
		return nil, err
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
		nextRoundStartTime: roundStartTime.Add(c.blockInterval),
		eManager:           eManager,
		status:             status,
		blockInLock:        blockInLock,
		proofOfLock:        proofOfLock,
	}, nil
}

func (c *roundCalculator) Proposer(height uint64, roundStartTime time.Time) string {
	round, err := c.newRound(height, roundStartTime, false)
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

func (c *roundCalculator) RoundInfo(
	height uint64,
	now time.Time,
) (roundNum uint32, roundStartTime time.Time, err error) {
	return c.roundInfo(height, now, false)
}

func (c *roundCalculator) roundInfo(
	height uint64,
	now time.Time,
	withToleration bool,
) (roundNum uint32, roundStartTime time.Time, err error) {
	lastBlockTime := time.Unix(c.chain.GenesisTimestamp(), 0)
	if height > 1 {
		var lastBlock *block.Block
		if lastBlock, err = c.chain.GetBlockByHeight(height - 1); err != nil {
			return
		}
		lastBlockCommitTime := lastBlock.CommitTime()
		lastBlockTime = lastBlockTime.Add(lastBlockCommitTime.Sub(lastBlockTime) / c.blockInterval * c.blockInterval)
	}
	if !lastBlockTime.Before(now) {
		err = errors.Errorf(
			"last block time %s is a future time, vs now %s",
			lastBlockTime,
			now,
		)
		return
	}
	duration := now.Sub(lastBlockTime)
	if duration > c.blockInterval {
		roundNum = uint32(duration / c.blockInterval)
		if !withToleration || duration%c.blockInterval < c.toleratedOvertime {
			roundNum--
		}
	}
	roundStartTime = lastBlockTime.Add(time.Duration(roundNum+1) * c.blockInterval)

	return roundNum, roundStartTime, nil
}

func (c *roundCalculator) Delegates(height uint64) ([]string, error) {
	epochStartHeight := c.rp.GetEpochHeight(c.rp.GetEpochNum(height))
	numDelegates := c.rp.NumDelegates()
	candidates, err := c.candidatesByHeightFunc(epochStartHeight)
	if err != nil {
		return nil, errors.Wrapf(
			err,
			"failed to get candidates on height %d",
			epochStartHeight,
		)
	}
	if len(candidates) < int(numDelegates) {
		return nil, errors.Errorf(
			"# of candidates %d is less than from required number %d",
			len(candidates),
			numDelegates,
		)
	}
	addrs := []string{}
	for i, candidate := range candidates {
		if uint64(i) >= c.rp.NumCandidateDelegates() {
			break
		}
		addrs = append(addrs, candidate.Address)
	}
	crypto.SortCandidates(addrs, epochStartHeight, crypto.CryptoSeed)

	return addrs[:numDelegates], nil
}

func (c *roundCalculator) NewRoundWithToleration(
	height uint64,
	now time.Time,
) (round *roundCtx, err error) {
	return c.newRound(height, now, true)
}

func (c *roundCalculator) NewRound(
	height uint64,
	now time.Time,
) (round *roundCtx, err error) {
	return c.newRound(height, now, false)
}

func (c *roundCalculator) newRound(
	height uint64,
	now time.Time,
	withToleration bool,
) (round *roundCtx, err error) {
	epochNum := uint64(0)
	epochStartHeight := uint64(0)
	var delegates []string
	var roundNum uint32
	var proposer string
	var roundStartTime time.Time
	if height != 0 {
		epochNum = c.rp.GetEpochNum(height)
		epochStartHeight := c.rp.GetEpochHeight(epochNum)
		if delegates, err = c.Delegates(epochStartHeight); err != nil {
			return
		}
		if roundNum, roundStartTime, err = c.roundInfo(height, now, withToleration); err != nil {
			return
		}
		if proposer, err = c.calculateProposer(height, roundNum, delegates); err != nil {
			return
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
		eManager:           newEndorsementManager(),
		roundStartTime:     roundStartTime,
		nextRoundStartTime: roundStartTime.Add(c.blockInterval),
		status:             open,
	}, nil
}

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
