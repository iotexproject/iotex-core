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
	"github.com/iotexproject/iotex-core/blockchain/genesis"
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
type Protocol interface {
	// Initialize fetches the poll result for genesis block
	Initialize(context.Context, protocol.StateManager) error
	// Handle handles a vote
	Handle(context.Context, action.Action, protocol.StateManager) (*action.Receipt, error)
	// Validate validates a vote
	Validate(ctx context.Context, act action.Action) error
	// DelegatesByHeight returns the delegates by chain height
	DelegatesByHeight(uint64) (state.CandidateList, error)
	// ReadState read the state on blockchain via protocol
	ReadState(context.Context, protocol.StateManager, []byte, ...[]byte) ([]byte, error)
}

type lifeLongDelegatesProtocol struct {
	delegates state.CandidateList
}

// NewLifeLongDelegatesProtocol creates a poll protocol with life long delegates
func NewLifeLongDelegatesProtocol(delegates []genesis.Delegate) Protocol {
	var l state.CandidateList
	for _, delegate := range delegates {
		rewardAddress := delegate.RewardAddress
		if rewardAddress == "" {
			rewardAddress = delegate.Address
		}
		l = append(l, &state.Candidate{
			Address: delegate.Address,
			// TODO: load votes from genesis
			Votes:         new(big.Int).SetUint64(delegate.Votes),
			RewardAddress: rewardAddress,
		})
	}
	return &lifeLongDelegatesProtocol{delegates: l}
}

func (p *lifeLongDelegatesProtocol) Initialize(
	ctx context.Context,
	sm protocol.StateManager,
) (err error) {
	log.L().Info("Initialize lifelong delegates protocol")
	return setCandidates(sm, p.delegates, uint64(1))
}

func (p *lifeLongDelegatesProtocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	return handle(ctx, act, sm)
}

func (p *lifeLongDelegatesProtocol) Validate(ctx context.Context, act action.Action) error {
	return validate(ctx, p, act)
}

func (p *lifeLongDelegatesProtocol) DelegatesByHeight(height uint64) (state.CandidateList, error) {
	return p.delegates, nil
}

func (p *lifeLongDelegatesProtocol) ReadState(context.Context, protocol.StateManager, []byte, ...[]byte) ([]byte, error) {
	return nil, protocol.ErrUnimplemented
}

type governanceChainCommitteeProtocol struct {
	getBlockTime          GetBlockTime
	getEpochHeight        GetEpochHeight
	electionCommittee     committee.Committee
	initBeaconChainHeight uint64
}

// NewGovernanceChainCommitteeProtocol creates a Poll Protocol which fetch result from governance chain
func NewGovernanceChainCommitteeProtocol(
	electionCommittee committee.Committee,
	initBeaconChainHeight uint64,
	getBlockTime GetBlockTime,
	getEpochHeight GetEpochHeight,
) (Protocol, error) {
	if electionCommittee == nil {
		return nil, ErrNoElectionCommittee
	}
	if getBlockTime == nil {
		return nil, errors.New("getBlockTime api is not provided")
	}
	if getEpochHeight == nil {
		return nil, errors.New("getEpochHeight api is not provided")
	}

	return &governanceChainCommitteeProtocol{
		electionCommittee:     electionCommittee,
		initBeaconChainHeight: initBeaconChainHeight,
		getBlockTime:          getBlockTime,
		getEpochHeight:        getEpochHeight,
	}, nil
}

func (p *governanceChainCommitteeProtocol) Initialize(
	ctx context.Context,
	sm protocol.StateManager,
) (err error) {
	log.L().Info("Initialize poll protocol", zap.Uint64("height", p.initBeaconChainHeight))
	var ds state.CandidateList
	if ds, err = p.delegatesByBeaconChainHeight(p.initBeaconChainHeight); err != nil {
		return
	}
	log.L().Info("Validating delegates from beacon chain", zap.Any("delegates", ds))
	if err = validateDelegates(ds); err != nil {
		return
	}

	return setCandidates(sm, ds, uint64(1))
}

func (p *governanceChainCommitteeProtocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	return handle(ctx, act, sm)
}

func (p *governanceChainCommitteeProtocol) Validate(ctx context.Context, act action.Action) error {
	return validate(ctx, p, act)
}

func (p *governanceChainCommitteeProtocol) delegatesByBeaconChainHeight(height uint64) (state.CandidateList, error) {
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

func (p *governanceChainCommitteeProtocol) DelegatesByHeight(height uint64) (state.CandidateList, error) {
	epochHeight := p.getEpochHeight(height)
	blkTime, err := p.getBlockTime(epochHeight)
	if err != nil {
		return nil, err
	}
	beaconHeight, err := p.electionCommittee.HeightByTime(blkTime)
	log.L().Debug(
		"fetch delegates from beacon chain by time",
		zap.Uint64("beaconChainHeight", beaconHeight),
		zap.Time("time", blkTime),
	)
	if err != nil {
		return nil, err
	}
	return p.delegatesByBeaconChainHeight(beaconHeight)
}

func (p *governanceChainCommitteeProtocol) ReadState(context.Context, protocol.StateManager, []byte, ...[]byte) ([]byte, error) {
	return nil, protocol.ErrUnimplemented
}

func validateDelegates(cs state.CandidateList) error {
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

func handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	r, ok := act.(*action.PutPollResult)
	if !ok {
		return nil, nil
	}
	zap.L().Debug("Handle PutPollResult Action", zap.Uint64("height", r.Height()))

	return nil, setCandidates(sm, r.Candidates(), r.Height())
}

func validate(ctx context.Context, p Protocol, act action.Action) error {
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
	if err := validateDelegates(proposedDelegates); err != nil {
		return err
	}
	ds, err := p.DelegatesByHeight(vaCtx.BlockHeight)
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

// setCandidates sets the candidates for the given state manager
func setCandidates(
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
