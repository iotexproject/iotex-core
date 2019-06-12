// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/committee"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

const (
	// ProtocolID defines the ID of this protocol
	ProtocolID = "poll"
)

// ErrNoElectionCommittee is an error that the election committee is not specified
var ErrNoElectionCommittee = errors.New("no election committee specified")

// ErrProposedDelegatesLength is an error that the proposed delegate list length is not right
var ErrProposedDelegatesLength = errors.New("the proposed delegate list length")

// ErrDelegatesNotAsExpected is an error that the delegates are not as expected
var ErrDelegatesNotAsExpected = errors.New("delegates are not as expected")

// GetBlockTime defines a function to get block creation time
type GetBlockTime func(uint64) (time.Time, error)

// GetEpochHeight defines a function to get the corresponding epoch height given an epoch number
type GetEpochHeight func(uint64) uint64

// GetEpochNum defines a function to get epoch number given a block height
type GetEpochNum func(uint64) uint64

// Protocol defines the protocol of handling votes
type Protocol interface {
	// Initialize fetches the poll result for genesis block
	Initialize(context.Context, protocol.StateManager) error
	// Handle handles a vote
	Handle(context.Context, action.Action, protocol.StateManager) (*action.Receipt, error)
	// Validate validates a vote
	Validate(context.Context, action.Action) error
	// DelegatesByHeight returns the delegates by chain height
	DelegatesByHeight(uint64) (state.CandidateList, error)
	// ReadState read the state on blockchain via protocol
	ReadState(context.Context, protocol.StateManager, []byte, ...[]byte) ([]byte, error)
}

type lifeLongDelegatesProtocol struct {
	delegates state.CandidateList
	addr      address.Address
}

// NewLifeLongDelegatesProtocol creates a poll protocol with life long delegates
func NewLifeLongDelegatesProtocol(delegates []genesis.Delegate) Protocol {
	var l state.CandidateList
	for _, delegate := range delegates {
		rewardAddress := delegate.RewardAddr()
		if rewardAddress == nil {
			rewardAddress = delegate.OperatorAddr()
		}
		l = append(l, &state.Candidate{
			Address: delegate.OperatorAddr().String(),
			// TODO: load votes from genesis
			Votes:         delegate.Votes(),
			RewardAddress: rewardAddress.String(),
		})
	}
	h := hash.Hash160b([]byte(ProtocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		log.L().Panic("Error when constructing the address of poll protocol", zap.Error(err))
	}
	return &lifeLongDelegatesProtocol{delegates: l, addr: addr}
}

func (p *lifeLongDelegatesProtocol) Initialize(
	ctx context.Context,
	sm protocol.StateManager,
) (err error) {
	log.L().Info("Initialize lifelong delegates protocol")
	return setCandidates(sm, p.delegates, uint64(1))
}

func (p *lifeLongDelegatesProtocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	return handle(ctx, act, sm, p.addr.String())
}

func (p *lifeLongDelegatesProtocol) Validate(ctx context.Context, act action.Action) error {
	return validate(ctx, p, act)
}

func (p *lifeLongDelegatesProtocol) DelegatesByHeight(height uint64) (state.CandidateList, error) {
	return p.delegates, nil
}

func (p *lifeLongDelegatesProtocol) ReadState(
	ctx context.Context,
	sm protocol.StateManager,
	method []byte,
	args ...[]byte,
) ([]byte, error) {
	switch string(method) {
	case "DelegatesByEpoch":
		fallthrough
	case "BlockProducersByEpoch":
		fallthrough
	case "ActiveBlockProducersByEpoch":
		return p.readBlockProducers()
	case "GetGravityChainStartHeight":
		if len(args) != 1 {
			return nil, errors.Errorf("invalid number of arguments %d", len(args))
		}
		return args[0], nil
	default:
		return nil, errors.New("corresponding method isn't found")
	}
}

func (p *lifeLongDelegatesProtocol) readBlockProducers() ([]byte, error) {
	return p.delegates.Serialize()
}

type governanceChainCommitteeProtocol struct {
	cm                        protocol.ChainManager
	getBlockTime              GetBlockTime
	getEpochHeight            GetEpochHeight
	getEpochNum               GetEpochNum
	electionCommittee         committee.Committee
	initGravityChainHeight    uint64
	numCandidateDelegates     uint64
	numDelegates              uint64
	addr                      address.Address
	initialCandidatesInterval time.Duration
}

// NewGovernanceChainCommitteeProtocol creates a Poll Protocol which fetch result from governance chain
func NewGovernanceChainCommitteeProtocol(
	cm protocol.ChainManager,
	electionCommittee committee.Committee,
	initGravityChainHeight uint64,
	getBlockTime GetBlockTime,
	getEpochHeight GetEpochHeight,
	getEpochNum GetEpochNum,
	numCandidateDelegates uint64,
	numDelegates uint64,
	initialCandidatesInterval time.Duration,
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

	if getEpochNum == nil {
		return nil, errors.New("getEpochNum api is not provided")
	}

	h := hash.Hash160b([]byte(ProtocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		log.L().Panic("Error when constructing the address of poll protocol", zap.Error(err))
	}

	return &governanceChainCommitteeProtocol{
		cm:                        cm,
		electionCommittee:         electionCommittee,
		initGravityChainHeight:    initGravityChainHeight,
		getBlockTime:              getBlockTime,
		getEpochHeight:            getEpochHeight,
		getEpochNum:               getEpochNum,
		numCandidateDelegates:     numCandidateDelegates,
		numDelegates:              numDelegates,
		addr:                      addr,
		initialCandidatesInterval: initialCandidatesInterval,
	}, nil
}

func (p *governanceChainCommitteeProtocol) Initialize(
	ctx context.Context,
	sm protocol.StateManager,
) (err error) {
	log.L().Info("Initialize poll protocol", zap.Uint64("height", p.initGravityChainHeight))
	var ds state.CandidateList

	for {
		ds, err = p.delegatesByGravityChainHeight(p.initGravityChainHeight)
		if err == nil || errors.Cause(err) != db.ErrNotExist {
			break
		}
		log.L().Error("calling committee,waiting for a while", zap.Int64("duration", int64(p.initialCandidatesInterval.Seconds())), zap.String("unit", " seconds"))
		time.Sleep(p.initialCandidatesInterval)
	}

	if err != nil {
		return
	}
	log.L().Info("Validating delegates from gravity chain", zap.Any("delegates", ds))
	if err = validateDelegates(ds); err != nil {
		return
	}

	return setCandidates(sm, ds, uint64(1))
}

func (p *governanceChainCommitteeProtocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	return handle(ctx, act, sm, p.addr.String())
}

func (p *governanceChainCommitteeProtocol) Validate(ctx context.Context, act action.Action) error {
	return validate(ctx, p, act)
}

func (p *governanceChainCommitteeProtocol) delegatesByGravityChainHeight(height uint64) (state.CandidateList, error) {
	r, err := p.electionCommittee.ResultByHeight(height)
	if err != nil {
		return nil, err
	}
	l := state.CandidateList{}
	for _, c := range r.Delegates() {
		operatorAddress := string(c.OperatorAddress())
		if _, err := address.FromString(operatorAddress); err != nil {
			log.L().Debug(
				"candidate's operator address is invalid",
				zap.String("operatorAddress", operatorAddress),
				zap.String("name", string(c.Name())),
				zap.Error(err),
			)
			continue
		}
		rewardAddress := string(c.RewardAddress())
		if _, err := address.FromString(rewardAddress); err != nil {
			log.L().Debug(
				"candidate's reward address is invalid",
				zap.String("name", string(c.Name())),
				zap.String("rewardAddress", rewardAddress),
				zap.Error(err),
			)
			continue
		}
		l = append(l, &state.Candidate{
			Address:       operatorAddress,
			Votes:         c.Score(),
			RewardAddress: rewardAddress,
		})
	}
	return l, nil
}

func (p *governanceChainCommitteeProtocol) DelegatesByHeight(height uint64) (state.CandidateList, error) {
	gravityHeight, err := p.getGravityHeight(height)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get gravity chain height")
	}
	log.L().Debug(
		"fetch delegates from gravity chain",
		zap.Uint64("gravityChainHeight", gravityHeight),
	)
	return p.delegatesByGravityChainHeight(gravityHeight)
}

func (p *governanceChainCommitteeProtocol) ReadState(
	ctx context.Context,
	sm protocol.StateManager,
	method []byte,
	args ...[]byte,
) ([]byte, error) {
	switch string(method) {
	case "DelegatesByEpoch":
		if len(args) != 1 {
			return nil, errors.Errorf("invalid number of arguments %d", len(args))
		}
		delegates, err := p.readDelegatesByEpoch(byteutil.BytesToUint64(args[0]))
		if err != nil {
			return nil, err
		}
		return delegates.Serialize()
	case "BlockProducersByEpoch":
		if len(args) != 1 {
			return nil, errors.Errorf("invalid number of arguments %d", len(args))
		}
		blockProducers, err := p.readBlockProducersByEpoch(byteutil.BytesToUint64(args[0]))
		if err != nil {
			return nil, err
		}
		return blockProducers.Serialize()
	case "ActiveBlockProducersByEpoch":
		if len(args) != 1 {
			return nil, errors.Errorf("invalid number of arguments %d", len(args))
		}
		activeBlockProducers, err := p.readActiveBlockProducersByEpoch(byteutil.BytesToUint64(args[0]))
		if err != nil {
			return nil, err
		}
		return activeBlockProducers.Serialize()
	case "GetGravityChainStartHeight":
		if len(args) != 1 {
			return nil, errors.Errorf("invalid number of arguments %d", len(args))
		}
		gravityStartheight, err := p.getGravityHeight(byteutil.BytesToUint64(args[0]))
		if err != nil {
			return nil, err
		}
		return byteutil.Uint64ToBytes(gravityStartheight), nil
	default:
		return nil, errors.New("corresponding method isn't found")

	}
}

func (p *governanceChainCommitteeProtocol) readDelegatesByEpoch(epochNum uint64) (state.CandidateList, error) {
	epochHeight := p.getEpochHeight(epochNum)
	return p.cm.CandidatesByHeight(epochHeight)
}

func (p *governanceChainCommitteeProtocol) readBlockProducersByEpoch(epochNum uint64) (state.CandidateList, error) {
	delegates, err := p.readDelegatesByEpoch(epochNum)
	if err != nil {
		return nil, err
	}
	var blockProducers state.CandidateList
	for i, delegate := range delegates {
		if uint64(i) >= p.numCandidateDelegates {
			break
		}
		blockProducers = append(blockProducers, delegate)
	}
	return blockProducers, nil
}

func (p *governanceChainCommitteeProtocol) readActiveBlockProducersByEpoch(epochNum uint64) (state.CandidateList, error) {
	blockProducers, err := p.readBlockProducersByEpoch(epochNum)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get active block producers in epoch %d", epochNum)
	}

	var blockProducerList []string
	blockProducerMap := make(map[string]*state.Candidate)
	for _, bp := range blockProducers {
		blockProducerList = append(blockProducerList, bp.Address)
		blockProducerMap[bp.Address] = bp
	}

	epochHeight := p.getEpochHeight(epochNum)
	crypto.SortCandidates(blockProducerList, epochHeight, crypto.CryptoSeed)

	length := int(p.numDelegates)
	if len(blockProducerList) < int(p.numDelegates) {
		length = len(blockProducerList)
	}

	var activeBlockProducers state.CandidateList
	for i := 0; i < length; i++ {
		activeBlockProducers = append(activeBlockProducers, blockProducerMap[blockProducerList[i]])
	}
	return activeBlockProducers, nil
}

func (p *governanceChainCommitteeProtocol) getGravityHeight(height uint64) (uint64, error) {
	epochNumber := p.getEpochNum(height)
	epochHeight := p.getEpochHeight(epochNumber)
	blkTime, err := p.getBlockTime(epochHeight)
	if err != nil {
		return 0, err
	}
	log.L().Debug(
		"get gravity chain height by time",
		zap.Time("time", blkTime),
	)
	return p.electionCommittee.HeightByTime(blkTime)
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

func handle(ctx context.Context, act action.Action, sm protocol.StateManager, protocolAddr string) (*action.Receipt, error) {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	r, ok := act.(*action.PutPollResult)
	if !ok {
		return nil, nil
	}
	zap.L().Debug("Handle PutPollResult Action", zap.Uint64("height", r.Height()))

	if err := setCandidates(sm, r.Candidates(), r.Height()); err != nil {
		return nil, errors.Wrap(err, "failed to set candidates")
	}
	return &action.Receipt{
		Status:          action.SuccessReceiptStatus,
		ActionHash:      raCtx.ActionHash,
		BlockHeight:     raCtx.BlockHeight,
		GasConsumed:     raCtx.IntrinsicGas,
		ContractAddress: protocolAddr,
	}, nil
}

func validate(ctx context.Context, p Protocol, act action.Action) error {
	ppr, ok := act.(*action.PutPollResult)
	if !ok {
		return nil
	}
	vaCtx := protocol.MustGetValidateActionsCtx(ctx)
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
		msg := fmt.Sprintf(", %d, is not as expected, %d",
			len(proposedDelegates),
			len(ds))
		return errors.Wrap(ErrProposedDelegatesLength, msg)
	}
	for i, d := range ds {
		if !proposedDelegates[i].Equal(d) {
			msg := fmt.Sprintf(", %v vs %v (expected)",
				proposedDelegates,
				ds)
			return errors.Wrap(ErrDelegatesNotAsExpected, msg)
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
		if err := candidatesutil.LoadAndAddCandidates(sm, height, candidate.Address); err != nil {
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
