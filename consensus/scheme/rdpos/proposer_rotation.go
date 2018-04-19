// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rdpos

import (
	"github.com/golang/glog"

	"github.com/iotexproject/iotex-core/common/routine"
	"github.com/iotexproject/iotex-core/consensus/fsm"
)

// proposerRotation is supposed to rotate the proposer per round of PBFT.
// However, use the first delegate as the proposer for now.
// We can propose based on the block height in the future.
type proposerRotation struct {
	*RDPoS
}

func (s *proposerRotation) Do() {
	proposer := s.delegates[0]
	s.proposer = proposer.String() == s.self.String()
	if !s.proposer || s.fsm.CurrentState() != stateStart {
		return
	}

	glog.Warningf("[%s] propose a new block height %v", s.self, s.bc.TipHeight()+1)
	s.fsm.HandleTransition(&fsm.Event{
		State: stateInitPropose,
	})
}

// NewProposerRotation creates a recurring task of proposer rotation.
func NewProposerRotation(r *RDPoS) *routine.RecurringTask {
	return routine.NewRecurringTask(&proposerRotation{r}, r.cfg.ProposerRotation.Interval)
}
