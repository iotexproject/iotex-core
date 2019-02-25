// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"math/big"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-election/committee"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
)

const (
	// ProtocolID defines the ID of this protocol
	ProtocolID = "poll"
)

// ErrNoElectionCommittee is an error that the election committee is not specified
var ErrNoElectionCommittee = errors.New("no election committee specified")

// GetBlockTime defines a function to get block creation time
type GetBlockTime func(uint64) (time.Time, error)

// GetEpochHeight defines a function to get the corresponding epoch height
type GetEpochHeight func(height uint64) uint64

// Protocol defines the protocol of handling votes
type Protocol struct {
	getBlockTime      GetBlockTime
	getEpochHeight    GetEpochHeight
	electionCommittee committee.Committee
}

// NewProtocol instantiates the protocol of vote
func NewProtocol(
	getBlockTime GetBlockTime,
	getEpochHeight GetEpochHeight,
	electionCommittee committee.Committee,
) *Protocol {
	return &Protocol{
		getBlockTime:      getBlockTime,
		getEpochHeight:    getEpochHeight,
		electionCommittee: electionCommittee,
	}
}

// ElectionCommittee returns the election committee in this protocol instance
func (p *Protocol) ElectionCommittee() committee.Committee {
	return p.electionCommittee
}

// Initialize fetches the poll result for genesis block
func (p *Protocol) Initialize(
	ctx context.Context,
	sm protocol.StateManager,
	height uint64,
	addrs []string,
) (err error) {
	log.L().Info("Initialize poll protocol", zap.Uint64("height", height))
	var ds state.CandidateList
	if height == 0 || p.electionCommittee == nil {
		for _, addr := range addrs {
			ds = append(ds, &state.Candidate{
				Address:       addr,
				Votes:         big.NewInt(0),
				RewardAddress: addr,
			})
		}
	} else {
		log.L().Info("Loading delegates from beacon chain")
		if ds, err = p.DelegatesByBeaconChainHeight(height); err != nil {
			return
		}
		log.L().Info("Validating delegates from beacon chain", zap.Any("delegates", ds))
		if err = p.validate(ds); err != nil {
			return
		}
	}

	return p.setCandidates(sm, ds, uint64(1))
}

// Handle handles a vote
func (p *Protocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	r, ok := act.(*action.PutPollResult)
	if !ok {
		return nil, nil
	}
	zap.L().Debug("Handle PutPollResult Action", zap.Uint64("height", r.Height()))

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
	epochHeight := p.getEpochHeight(vaCtx.BlockHeight)
	ds, err := p.DelegatesByHeight(epochHeight)
	if err != nil {
		return err
	}
	if len(ds) != len(proposedDelegates) {
		return errors.Errorf(
			"the proposed delegate list length, %d, is not as expected, %d",
			len(proposedDelegates),
			len(ds),
		)
	}
	for i, d := range ds {
		if !proposedDelegates[i].Equal(d) {
			return errors.Errorf(
				"delegates are not as expected, %v vs %v (expected)",
				proposedDelegates,
				ds,
			)
		}
	}
	return nil
}

// DelegatesByBeaconChainHeight returns the delegates by beacon chain height
func (p *Protocol) DelegatesByBeaconChainHeight(height uint64) (state.CandidateList, error) {
	electionCommittee := p.electionCommittee
	if electionCommittee == nil {
		return nil, ErrNoElectionCommittee
	}
	r, err := p.electionCommittee.ResultByHeight(height)
	if err != nil {
		return nil, err
	}
	l := state.CandidateList{}
	for _, c := range r.Delegates() {
		l = append(l, &state.Candidate{
			Address:       string(c.OperatorAddress()),
			Votes:         c.Score(),
			RewardAddress: string(c.RewardAddress()),
		})
	}
	return l, nil
}

// DelegatesByHeight returns the delegates by chain height
func (p *Protocol) DelegatesByHeight(height uint64) (state.CandidateList, error) {
	blkTime, err := p.getBlockTime(height)
	if err != nil {
		return nil, err
	}
	electionCommittee := p.electionCommittee
	if electionCommittee == nil {
		return nil, ErrNoElectionCommittee
	}
	beaconHeight, err := electionCommittee.HeightByTime(blkTime)
	log.L().Debug(
		"fetch delegates from beacon chain by time",
		zap.Uint64("beaconChainHeight", beaconHeight),
		zap.Time("time", blkTime),
	)
	if err != nil {
		return nil, err
	}
	return p.DelegatesByBeaconChainHeight(beaconHeight)
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
	for _, candidate := range candidates {
		delegate, err := accountutil.LoadOrCreateAccount(sm, candidate.Address, big.NewInt(0))
		if err != nil {
			return errors.Wrapf(err, "failed to load or create the account for delegate %s", candidate.Address)
		}
		delegate.IsCandidate = true
		if err := candidatesutil.LoadAndAddCandidates(sm, candidate.Address); err != nil {
			return err
		}
		if err := accountutil.StoreAccount(sm, candidate.Address, delegate); err != nil {
			return errors.Wrap(err, "failed to update pending account changes to trie")
		}
		log.L().Debug(
			"add candidate",
			zap.String("address", candidate.Address),
			zap.String("rewardAddress", candidate.RewardAddress),
			zap.String("score", candidate.Votes.String()),
		)
	}
	return sm.PutState(candidatesutil.ConstructKey(height), &candidates)
}
