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

// Do handles transition to stateInitPropose
func (s *proposerRotation) Do() {
	height, err := s.bc.TipHeight()
	if err != nil {
		logger.Error().Err(err).Msg("failed to get blockchain height")
		return
	}
	pr, err := s.prCb(s.pool, nil, 0, height+1)
	// If proposer is not the current node or it's not periodic proposer election on constant interval, then returns
	if pr.String() != s.self.String() ||
		(s.cfg.ProposerRotation.Interval != 0 && s.fsm.CurrentState() != stateRoundStart) {
		return
	}
	logger.Warn().
		Str("proposer", s.self.String()).
		Uint64("height", height+1).
		Msg("Propose new block height")

	s.handleEvent(&fsm.Event{
		State: stateInitPropose,
	})
}

// newProposerRotationNoDelay creates a ProposerRotation object
func newProposerRotationNoDelay(r *RollDPoS) *proposerRotation {
	return &proposerRotation{r}
}

// newProposerRotation creates a recurring task of proposer rotation.
func newProposerRotation(r *RollDPoS) *routine.RecurringTask {
	return routine.NewRecurringTask(&proposerRotation{r}, r.cfg.ProposerRotation.Interval)
}

// FixedProposer will always choose the first in the delegate list as the proposer
func FixedProposer(pool delegate.Pool, _ []byte, _ uint64, _ uint64) (net.Addr, error) {
	delegates, err := getNonEmptyProposerList(pool)
	if err != nil {
		return nil, err
	}
	return delegates[0], nil
}

// PseudoRotatedProposer will rotate among the delegates to choose the proposer
func PseudoRotatedProposer(pool delegate.Pool, _ []byte, _ uint64, height uint64) (net.Addr, error) {
	delegates, err := getNonEmptyProposerList(pool)
	if err != nil {
		return nil, err
	}
	return delegates[height%uint64(len(delegates))], nil
}

func getNonEmptyProposerList(pool delegate.Pool) ([]net.Addr, error) {
	// TODO: Need to check if the node should panic if it's not able to get the delegates
	// TODO: Get the delegates at the roundStart of an epoch and put it into an epoch context
	delegates, err := pool.AllDelegates()
	if err != nil {
		return nil, err
	}
	if len(delegates) == 0 {
		return nil, delegate.ErrZeroDelegate
	}
	return delegates, nil
}
