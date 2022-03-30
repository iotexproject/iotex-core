// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"strconv"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/committee"
	"github.com/iotexproject/iotex-election/db"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
)

type governanceChainCommitteeProtocol struct {
	getBlockTime              GetBlockTime
	electionCommittee         committee.Committee
	initGravityChainHeight    uint64
	addr                      address.Address
	initialCandidatesInterval time.Duration
	sh                        *Slasher
	indexer                   *CandidateIndexer
}

// NewGovernanceChainCommitteeProtocol creates a Poll Protocol which fetch result from governance chain
func NewGovernanceChainCommitteeProtocol(
	candidatesIndexer *CandidateIndexer,
	electionCommittee committee.Committee,
	initGravityChainHeight uint64,
	getBlockTime GetBlockTime,
	initialCandidatesInterval time.Duration,
	sh *Slasher,
) (Protocol, error) {
	if electionCommittee == nil {
		return nil, ErrNoElectionCommittee
	}
	if getBlockTime == nil {
		return nil, errors.New("getBlockTime api is not provided")
	}

	h := hash.Hash160b([]byte(_protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		log.L().Panic("Error when constructing the address of poll protocol", zap.Error(err))
	}

	return &governanceChainCommitteeProtocol{
		electionCommittee:         electionCommittee,
		initGravityChainHeight:    initGravityChainHeight,
		getBlockTime:              getBlockTime,
		addr:                      addr,
		initialCandidatesInterval: initialCandidatesInterval,
		sh:                        sh,
		indexer:                   candidatesIndexer,
	}, nil
}

func (p *governanceChainCommitteeProtocol) CreateGenesisStates(
	ctx context.Context,
	sm protocol.StateManager,
) (err error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	if blkCtx.BlockHeight != 0 {
		return errors.Errorf("Cannot create genesis state for height %d", blkCtx.BlockHeight)
	}
	log.L().Info("Initialize poll protocol", zap.Uint64("height", p.initGravityChainHeight))
	if err = p.sh.CreateGenesisStates(ctx, sm, p.indexer); err != nil {
		return
	}
	var ds state.CandidateList
	for {
		ds, err = p.candidatesByGravityChainHeight(p.initGravityChainHeight)
		if err == nil || errors.Cause(err) != db.ErrNotExist {
			break
		}
		log.L().Debug("calling committee,waiting for a while", zap.Int64("duration", int64(p.initialCandidatesInterval.Seconds())), zap.String("unit", " seconds"))
		time.Sleep(p.initialCandidatesInterval)
	}
	if err != nil {
		return
	}
	log.L().Info("Validating delegates from gravity chain", zap.Any("delegates", ds))
	if err = validateDelegates(ds); err != nil {
		return
	}
	return setCandidates(ctx, sm, p.indexer, ds, uint64(1))
}

func (p *governanceChainCommitteeProtocol) CreatePostSystemActions(ctx context.Context, sr protocol.StateReader) ([]action.Envelope, error) {
	return createPostSystemActions(ctx, sr, p)
}

func (p *governanceChainCommitteeProtocol) CreatePreStates(ctx context.Context, sm protocol.StateManager) error {
	return p.sh.CreatePreStates(ctx, sm, p.indexer)
}

func (p *governanceChainCommitteeProtocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	return handle(ctx, act, sm, p.indexer, p.addr.String())
}

func (p *governanceChainCommitteeProtocol) Validate(ctx context.Context, act action.Action, sr protocol.StateReader) error {
	return validate(ctx, sr, p, act)
}

func (p *governanceChainCommitteeProtocol) candidatesByGravityChainHeight(height uint64) (state.CandidateList, error) {
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
			CanName:       c.Name(),
		})
	}
	return l, nil
}

func (p *governanceChainCommitteeProtocol) CalculateCandidatesByHeight(ctx context.Context, _ protocol.StateReader, height uint64) (state.CandidateList, error) {
	gravityHeight, err := p.getGravityHeight(ctx, height)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get gravity chain height")
	}
	log.L().Debug(
		"fetch delegates from gravity chain",
		zap.Uint64("gravityChainHeight", gravityHeight),
	)
	return p.candidatesByGravityChainHeight(gravityHeight)
}

func (p *governanceChainCommitteeProtocol) CalculateUnproductiveDelegates(
	ctx context.Context,
	sr protocol.StateReader,
) ([]string, error) {
	return p.sh.calculateUnproductiveDelegates(ctx, sr)
}

func (p *governanceChainCommitteeProtocol) Delegates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	delegates, _, err := p.sh.GetActiveBlockProducers(ctx, sr, false)
	return delegates, err
}

func (p *governanceChainCommitteeProtocol) NextDelegates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	nextDelegates, _, err := p.sh.GetActiveBlockProducers(ctx, sr, true)
	return nextDelegates, err
}

func (p *governanceChainCommitteeProtocol) Candidates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	candidates, _, err := p.sh.GetCandidates(ctx, sr, false)
	return candidates, err
}

func (p *governanceChainCommitteeProtocol) NextCandidates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	candidates, _, err := p.sh.GetCandidates(ctx, sr, true)
	return candidates, err
}

func (p *governanceChainCommitteeProtocol) ReadState(
	ctx context.Context,
	sr protocol.StateReader,
	method []byte,
	args ...[]byte,
) ([]byte, uint64, error) {
	switch string(method) {
	case "GetGravityChainStartHeight":
		if len(args) != 1 {
			return nil, uint64(0), errors.Errorf("invalid number of arguments %d", len(args))
		}
		nativeHeight, err := strconv.ParseUint(string(args[0]), 10, 64)
		if err != nil {
			return nil, uint64(0), err
		}
		gravityStartheight, err := p.getGravityHeight(ctx, nativeHeight)
		if err != nil {
			return nil, uint64(0), err
		}
		return []byte(strconv.FormatUint(gravityStartheight, 10)), nativeHeight, nil
	default:
		return p.sh.ReadState(ctx, sr, p.indexer, method, args...)
	}
}

// Register registers the protocol with a unique ID
func (p *governanceChainCommitteeProtocol) Register(r *protocol.Registry) error {
	return r.Register(_protocolID, p)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (p *governanceChainCommitteeProtocol) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(_protocolID, p)
}

// Name returns the name of protocol
func (p *governanceChainCommitteeProtocol) Name() string {
	return _protocolID
}

func (p *governanceChainCommitteeProtocol) getGravityHeight(ctx context.Context, height uint64) (uint64, error) {
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	epochNumber := rp.GetEpochNum(height)
	epochHeight := rp.GetEpochHeight(epochNumber)
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
