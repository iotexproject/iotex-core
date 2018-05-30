// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"net"

	"github.com/iotexproject/iotex-core/common/routine"
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/logger"
)

// proposerRotation is supposed to rotate the proposer per round of PBFT.
// However, use the first delegate as the proposer for now.
// We can propose based on the block height in the future.
type proposerRotation struct {
	*RollDPoS
}

func (s *proposerRotation) Do() {
	isPr, err := s.prCb(s.self, s.pool, nil, 0)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get delegates")
	}
	if !isPr || s.fsm.CurrentState() != stateStart {
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

// FixedProposer will check if the current node is the first in the delegate list
func FixedProposer(self net.Addr, pool delegate.Pool, _ []byte, _ uint64) (bool, error) {
	// TODO: Need to check if the node should panic if it's not able to get the delegates
	delegates, err := pool.AllDelegates()
	if err != nil {
		return false, err
	}
	if len(delegates) == 0 {
		return false, delegate.ErrZeroDelegate
	}
	return self.String() == delegates[0].String(), nil
}
