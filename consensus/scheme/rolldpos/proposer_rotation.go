// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"github.com/iotexproject/iotex-core/common/routine"
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/logger"
)

// proposerRotation is supposed to rotate the proposer per round of PBFT.
// However, use the first delegate as the proposer for now.
// We can propose based on the block height in the future.
type proposerRotation struct {
	*RollDPoS
}

func (s *proposerRotation) Do() {
	proposer := s.delegates[0]
	s.proposer = proposer.String() == s.self.String()
	if !s.proposer || s.fsm.CurrentState() != stateStart {
		return
	}

	height, err := s.bc.TipHeight()
	if err == nil {
		logger.Warn().
			Str("proposer", s.self.String()).
			Uint64("height", height+1).
			Msg("Propose new block height")
	} else {
		logger.Error().Msg("Failed to get blockchain height")
	}
	s.fsm.HandleTransition(&fsm.Event{
		State: stateInitPropose,
	})
}

// NewProposerRotation creates a recurring task of proposer rotation.
func NewProposerRotation(r *RollDPoS) *routine.RecurringTask {
	return routine.NewRecurringTask(&proposerRotation{r}, r.cfg.ProposerRotation.Interval)
}
