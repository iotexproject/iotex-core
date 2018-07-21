// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"errors"

	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/logger"
)

// ruleDKGGenerate checks if the event is dkg generate
type ruleDKGGenerate struct {
	*RollDPoS
}

func (r ruleDKGGenerate) Condition(event *fsm.Event) bool {
	// Prevent cascading transition to DKG_GENERATE when moving back to EPOCH_START
	if event.State == stateEpochStart {
		return false
	}

	// Determine the epoch ordinal number
	height, err := r.bc.TipHeight()
	if err != nil {
		event.Err = err
		return false
	}
	epochNum, err := calcEpochNum(&r.cfg, height, r.pool)
	if err != nil {
		event.Err = err
		return false
	}

	// Determine the epoch height offset
	epochHeight, err := calEpochHeight(&r.cfg, epochNum, r.pool)
	if err != nil {
		event.Err = err
		return false
	}

	// Get the rolling delegates
	var delegates []string
	switch r.cfg.EpochCB {
	case "StartRollingEpoch":
		height, err := calEpochHeight(&r.cfg, epochNum, r.pool)
		if err != nil {
			event.Err = err
			return false
		}
		candidates, ok := r.bc.CandidatesByHeight(height - 1)
		if !ok {
			event.Err = errors.New("Epoch number of statefactory is inconsistent in StartRollingEpoch")
			return false
		}
		if len(candidates) < int(r.RollDPoS.cfg.NumDelegates) {
			event.Err = errors.New("Candidate pool does not have enough candidates")
			return false
		}
		candidates = candidates[:r.RollDPoS.cfg.NumDelegates]
		for _, candidate := range candidates {
			delegates = append(delegates, candidate.Address)
		}
	default:
		delegates, err = r.pool.RollDelegates(epochNum)
		if err != nil {
			event.Err = err
			return false
		}
	}

	// Get the sub-epoch number
	numSubEpochs := uint(1)
	if r.cfg.NumSubEpochs > 0 {
		numSubEpochs = r.cfg.NumSubEpochs
	}

	// The epochStart start height is going to be the next block to generate
	r.epochCtx = &epochCtx{
		num:          epochNum,
		height:       epochHeight,
		delegates:    delegates,
		numSubEpochs: numSubEpochs,
	}

	// Trigger the proposer election after entering the first round of consensus in an epoch if no delay
	if r.cfg.ProposerInterval == 0 {
		r.prnd.Handle()
	}

	logger.Info().
		Str("name", r.self).
		Uint64("height", r.epochCtx.height).
		Msg("enter an epoch")
	return true
}
