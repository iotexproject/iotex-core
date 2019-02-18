// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-election/committee"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
)

const (
	// ProtocolID defines the ID of this protocol
	ProtocolID = "poll"
)

// Protocol defines the protocol of handling votes
type Protocol struct {
	electionCommittee committee.Committee
}

// NewProtocol instantiates the protocol of vote
func NewProtocol(electionCommittee committee.Committee) *Protocol {
	return &Protocol{electionCommittee: electionCommittee}
}

// Initialize fetches the poll result for genesis block
func (p *Protocol) Initialize(ctx context.Context, sm protocol.StateManager, height uint64) error {
	r, err := p.electionCommittee.ResultByHeight(height)
	if err != nil {
		return err
	}
	l := state.CandidateList{}
	for _, c := range r.Delegates() {
		l = append(l, &state.Candidate{
			Address:       string(c.OperatorAddress()),
			Votes:         c.Score(),
			RewardAddress: string(c.RewardAddress()),
		})
	}
	if err := p.validate(l); err != nil {
		return err
	}
	return p.setCandidates(sm, l, uint64(1))
}

// Handle handles a vote
func (p *Protocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	raCtx, ok := protocol.GetRunActionsCtx(ctx)
	if !ok {
		log.S().Panic("Miss run action context")
	}
	r, ok := act.(*action.PutPollResult)
	if !ok {
		return nil, nil
	}
	caller, err := util.LoadOrCreateAccount(sm, raCtx.Caller.String(), big.NewInt(0))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load or create the account of voter %s", raCtx.Caller.String())
	}
	util.SetNonce(r, caller)

	return nil, p.setCandidates(sm, r.Candidates(), r.Height())
}

// Validate validates a vote
func (p *Protocol) Validate(ctx context.Context, act action.Action) error {
	ppr, ok := act.(*action.PutPollResult)
	if !ok {
		if _, ok := act.(*action.Vote); ok {
			return errors.New("with poll protocol, votes cannot be processed")
		}
		return nil
	}
	vaCtx, ok := protocol.GetValidateActionsCtx(ctx)
	if !ok {
		log.S().Panic("Miss validate action context")
	}
	if vaCtx.ProducerAddr != vaCtx.Caller.String() {
		return errors.New("Only producer could create this protocol")
	}
	proposedDelegates := ppr.Candidates()
	if err := p.validate(proposedDelegates); err != nil {
		return err
	}
	result, err := p.electionCommittee.ResultByHeight(vaCtx.BlockHeight)
	if err != nil {
		return err
	}
	ds := result.Delegates()
	if len(ds) != len(proposedDelegates) {
		return errors.Errorf(
			"the proposed delegate list length, %d, is not as expected, %d",
			len(proposedDelegates),
			len(ds),
		)
	}
	for i, d := range ds {
		c := &state.Candidate{
			Address:       string(d.OperatorAddress()),
			Votes:         d.Score(),
			RewardAddress: string(d.RewardAddress()),
		}
		if !proposedDelegates[i].Equal(c) {
			return errors.Errorf(
				"delegates are not as expected, %v vs %v (expected)",
				proposedDelegates,
				ds,
			)
		}
	}
	return nil
}

// ReadState read the state on blockchain via protocol
func (p *Protocol) ReadState(context.Context, protocol.StateManager, []byte, ...[]byte) ([]byte, error) {
	return nil, protocol.ErrUnimplemented
}

func (p *Protocol) validate(cs state.CandidateList) error {
	zero := big.NewInt(0)
	addrs := map[string]bool{}
	lastVotes := zero
	for _, candidate := range cs {
		if _, exists := addrs[candidate.Address]; exists {
			return errors.Errorf("duplicate candidate %s", candidate.Address)
		}
		addrs[candidate.Address] = true
		if candidate.Votes.Cmp(zero) < 0 {
			return errors.New("votes for candidate cannot be negative")
		}
		if lastVotes.Cmp(zero) > 0 && lastVotes.Cmp(candidate.Votes) < 0 {
			return errors.New("candidate list is not sorted")
		}
	}
	return nil
}

// setCandidates sets the candidates for the given state manager
func (p *Protocol) setCandidates(
	sm protocol.StateManager,
	candidates state.CandidateList,
	height uint64,
) error {
	return sm.PutState(candidatesutil.ConstructKey(height), &candidates)
}
