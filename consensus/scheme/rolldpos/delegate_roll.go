// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/routine"
)

// delegateRoll is supposed to roll the delegates for each epoch
type delegateRoll struct {
	*RollDPoS
}

// Handle handles transition to stateDKGGenerate
func (d *delegateRoll) Handle() {
	if d.fsm.CurrentState() != stateEpochStart {
		return
	}
	height, err := d.bc.TipHeight()
	if err != nil {
		logger.Error().Err(err).Msg("error when query the blockchain height")
		return
	}
	epochNum, err := calcEpochNum(&d.cfg, height, d.pool)
	if err != nil {
		logger.Error().Err(err).Msg("error when determining the epoch ordinal number")
		return
	}

	ok, err := d.epochStartCb(d.self, epochNum, d.pool, d.bc, &d.cfg)
	if err != nil {
		logger.Error().Err(err).Msg("error when determining if the node will participate into next epoch")
		return
	}
	if ok {
		logger.Info().
			Str("name", d.self).
			Uint64("epoch", epochNum).
			Msg("the current node is the delegate")
		d.enqueueEvent(&fsm.Event{
			State: stateDKGGenerate,
		})
	} else {
		logger.Info().
			Str("name", d.self).
			Uint64("epoch", epochNum).
			Msg("the current node is not the delegate")
	}
}

// newDelegateRoll creates a recurring task of delegate roll
func newDelegateRoll(r *RollDPoS) *routine.RecurringTask {
	dr := &delegateRoll{r}
	return routine.NewRecurringTask(dr.Handle, r.cfg.DelegateInterval)
}

// NeverStartNewEpoch will never allow to start a new epochStart after the first one
func NeverStartNewEpoch(_ string, _ uint64, _ delegate.Pool, _ blockchain.Blockchain, _ *config.RollDPoS) (bool, error) {
	return false, nil
}

// PseudoStarNewEpoch will always allow to start a new epochStart after the first one
func PseudoStarNewEpoch(_ string, _ uint64, _ delegate.Pool, _ blockchain.Blockchain, _ *config.RollDPoS) (bool, error) {
	return true, nil
}

// PseudoStartRollingEpoch will only allows the delegates chosen for given epoch to enter the epoch
func PseudoStartRollingEpoch(self string, epochNum uint64, pool delegate.Pool, _ blockchain.Blockchain, _ *config.RollDPoS) (bool, error) {
	delegates, err := pool.RollDelegates(epochNum)
	if err != nil {
		return false, err
	}
	for _, d := range delegates {
		if self == d {
			return true, nil
		}
	}
	return false, nil
}

// StartRollingEpoch will only allows the delegates chosen for given epoch to enter the epoch
func StartRollingEpoch(self string, epochNum uint64, pool delegate.Pool, bc blockchain.Blockchain, cfg *config.RollDPoS) (bool, error) {
	height, err := calEpochHeight(cfg, epochNum, pool)
	if err != nil {
		return false, err
	}
	candidates, ok := bc.CandidatesByHeight(height - 1)
	if !ok {
		return false, errors.New("Epoch number of statefactory is inconsistent in StartRollingEpoch")
	}
	if len(candidates) < int(cfg.NumDelegates) {
		return false, errors.New("Candidate pool does not have enough candidates")
	}
	candidates = candidates[:cfg.NumDelegates]

	for _, d := range candidates {
		if self == d.Address {
			return true, nil
		}
	}
	return false, nil
}

// calcEpochNum calculates the epoch ordinal number
func calcEpochNum(cfg *config.RollDPoS, height uint64, pool delegate.Pool) (uint64, error) {
	numDlgs, err := pool.NumDelegatesPerEpoch()
	if err != nil {
		return 0, err
	}
	epochNum := height/(uint64(numDlgs)*uint64(cfg.NumSubEpochs)) + 1
	return epochNum, nil
}

// calEpochHeight calculates the epoch start height offset by the epoch ordinal number
func calEpochHeight(cfg *config.RollDPoS, epochNum uint64, pool delegate.Pool) (uint64, error) {
	numDlgs, err := pool.NumDelegatesPerEpoch()
	if err != nil {
		return 0, err
	}
	epochHeight := uint64(numDlgs)*uint64(cfg.NumSubEpochs)*(epochNum-1) + 1
	return epochHeight, nil
}
