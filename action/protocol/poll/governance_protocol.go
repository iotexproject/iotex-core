// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
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
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

type governanceChainCommitteeProtocol struct {
	candidatesByHeight        CandidatesByHeight
	getBlockTime              GetBlockTime
	electionCommittee         committee.Committee
	initGravityChainHeight    uint64
	numCandidateDelegates     uint64
	numDelegates              uint64
	addr                      address.Address
	initialCandidatesInterval time.Duration
	stateReader               protocol.StateReader
}

// NewGovernanceChainCommitteeProtocol creates a Poll Protocol which fetch result from governance chain
func NewGovernanceChainCommitteeProtocol(
	candidatesByHeight CandidatesByHeight,
	electionCommittee committee.Committee,
	initGravityChainHeight uint64,
	getBlockTime GetBlockTime,
	numCandidateDelegates uint64,
	numDelegates uint64,
	initialCandidatesInterval time.Duration,
	sr protocol.StateReader,
) (Protocol, error) {
	if electionCommittee == nil {
		return nil, ErrNoElectionCommittee
	}
	if getBlockTime == nil {
		return nil, errors.New("getBlockTime api is not provided")
	}

	h := hash.Hash160b([]byte(protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		log.L().Panic("Error when constructing the address of poll protocol", zap.Error(err))
	}
	return &governanceChainCommitteeProtocol{
		candidatesByHeight:        candidatesByHeight,
		electionCommittee:         electionCommittee,
		initGravityChainHeight:    initGravityChainHeight,
		getBlockTime:              getBlockTime,
		numCandidateDelegates:     numCandidateDelegates,
		numDelegates:              numDelegates,
		addr:                      addr,
		initialCandidatesInterval: initialCandidatesInterval,
		stateReader:               sr,
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
	var ds state.CandidateList

	for {
		ds, err = p.candidatesByGravityChainHeight(p.initGravityChainHeight)
		if err == nil || errors.Cause(err) != db.ErrNotExist {
			break
		}
		log.L().Info("calling committee,waiting for a while", zap.Int64("duration", int64(p.initialCandidatesInterval.Seconds())), zap.String("unit", " seconds"))
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

func (p *governanceChainCommitteeProtocol) CreatePostSystemActions(ctx context.Context) ([]action.Envelope, error) {
	return createPostSystemActions(ctx, p)
}

func (p *governanceChainCommitteeProtocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	return handle(ctx, act, sm, p.addr.String())
}

func (p *governanceChainCommitteeProtocol) Validate(ctx context.Context, act action.Action) error {
	return validate(ctx, p, act)
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

func (p *governanceChainCommitteeProtocol) CalculateCandidatesByHeight(ctx context.Context, height uint64) (state.CandidateList, error) {
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

func (p *governanceChainCommitteeProtocol) DelegatesByEpoch(ctx context.Context, epochNum uint64) (state.CandidateList, error) {
	return p.readActiveBlockProducersByEpoch(ctx, epochNum)
}

func (p *governanceChainCommitteeProtocol) CandidatesByHeight(ctx context.Context, height uint64) (state.CandidateList, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	return p.candidatesByHeight(p.stateReader, rp.GetEpochHeight(rp.GetEpochNum(height)))
}

func (p *governanceChainCommitteeProtocol) ReadState(
	ctx context.Context,
	sm protocol.StateReader,
	method []byte,
	args ...[]byte,
) ([]byte, error) {
	switch string(method) {
	case "CandidatesByEpoch":
		if len(args) != 1 {
			return nil, errors.Errorf("invalid number of arguments %d", len(args))
		}
		delegates, err := p.readCandidatesByEpoch(ctx, byteutil.BytesToUint64(args[0]))
		if err != nil {
			return nil, err
		}
		return delegates.Serialize()
	case "BlockProducersByEpoch":
		if len(args) != 1 {
			return nil, errors.Errorf("invalid number of arguments %d", len(args))
		}
		blockProducers, err := p.readBlockProducersByEpoch(ctx, byteutil.BytesToUint64(args[0]))
		if err != nil {
			return nil, err
		}
		return blockProducers.Serialize()
	case "ActiveBlockProducersByEpoch":
		if len(args) != 1 {
			return nil, errors.Errorf("invalid number of arguments %d", len(args))
		}
		activeBlockProducers, err := p.readActiveBlockProducersByEpoch(ctx, byteutil.BytesToUint64(args[0]))
		if err != nil {
			return nil, err
		}
		return activeBlockProducers.Serialize()
	case "GetGravityChainStartHeight":
		if len(args) != 1 {
			return nil, errors.Errorf("invalid number of arguments %d", len(args))
		}
		gravityStartheight, err := p.getGravityHeight(ctx, byteutil.BytesToUint64(args[0]))
		if err != nil {
			return nil, err
		}
		return byteutil.Uint64ToBytes(gravityStartheight), nil
	default:
		return nil, errors.New("corresponding method isn't found")

	}
}

// Register registers the protocol with a unique ID
func (p *governanceChainCommitteeProtocol) Register(r *protocol.Registry) error {
	return r.Register(protocolID, p)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (p *governanceChainCommitteeProtocol) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(protocolID, p)
}

func (p *governanceChainCommitteeProtocol) readCandidatesByEpoch(ctx context.Context, epochNum uint64) (state.CandidateList, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	return p.candidatesByHeight(p.stateReader, rp.GetEpochHeight(epochNum))
}

func (p *governanceChainCommitteeProtocol) readBlockProducersByEpoch(ctx context.Context, epochNum uint64) (state.CandidateList, error) {
	candidates, err := p.readCandidatesByEpoch(ctx, epochNum)
	if err != nil {
		return nil, err
	}
	var blockProducers state.CandidateList
	for i, candidate := range candidates {
		if uint64(i) >= p.numCandidateDelegates {
			break
		}
		blockProducers = append(blockProducers, candidate)
	}
	return blockProducers, nil
}

func (p *governanceChainCommitteeProtocol) readActiveBlockProducersByEpoch(ctx context.Context, epochNum uint64) (state.CandidateList, error) {
	blockProducers, err := p.readBlockProducersByEpoch(ctx, epochNum)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get candidates in epoch %d", epochNum)
	}

	var blockProducerList []string
	blockProducerMap := make(map[string]*state.Candidate)
	for _, bp := range blockProducers {
		blockProducerList = append(blockProducerList, bp.Address)
		blockProducerMap[bp.Address] = bp
	}
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	epochHeight := rp.GetEpochHeight(epochNum)
	crypto.SortCandidates(blockProducerList, epochHeight, crypto.CryptoSeed)

	// TODO: kick-out unqualified delegates based on productivity

	length := int(p.numDelegates)
	if len(blockProducerList) < int(p.numDelegates) {
		// TODO: if the number of delegates is smaller than expected, should it return error or not?
		length = len(blockProducerList)
	}

	var activeBlockProducers state.CandidateList
	for i := 0; i < length; i++ {
		activeBlockProducers = append(activeBlockProducers, blockProducerMap[blockProducerList[i]])
	}
	return activeBlockProducers, nil
}

func (p *governanceChainCommitteeProtocol) getGravityHeight(ctx context.Context, height uint64) (uint64, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
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
