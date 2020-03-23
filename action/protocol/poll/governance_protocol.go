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
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
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

	h := hash.Hash160b([]byte(protocolID))
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
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
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
	hu := config.NewHeightUpgrade(&bcCtx.Genesis)
	if hu.IsPost(config.Easter, uint64(1)) {
		if err := setNextEpochBlacklist(sm,
			p.indexer,
			uint64(1),
			p.sh.EmptyBlacklist()); err != nil {
			return err
		}
	}
	return setCandidates(ctx, sm, p.indexer, ds, uint64(1))
}

func (p *governanceChainCommitteeProtocol) CreatePostSystemActions(ctx context.Context) ([]action.Envelope, error) {
	return createPostSystemActions(ctx, p)
}

func (p *governanceChainCommitteeProtocol) CreatePreStates(ctx context.Context, sm protocol.StateManager) error {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	epochNum := rp.GetEpochNum(blkCtx.BlockHeight)
	epochStartHeight := rp.GetEpochHeight(epochNum)
	epochLastHeight := rp.GetEpochLastBlockHeight(epochNum)
	nextEpochStartHeight := rp.GetEpochHeight(epochNum + 1)
	hu := config.NewHeightUpgrade(&bcCtx.Genesis)
	if blkCtx.BlockHeight == epochLastHeight && hu.IsPost(config.Easter, nextEpochStartHeight) {
		// if the block height is the end of epoch and next epoch is after the Easter height, calculate blacklist for kick-out and write into state DB
		unqualifiedList, err := p.sh.CalculateKickoutList(ctx, sm, epochNum+1)
		if err != nil {
			return err
		}
		return setNextEpochBlacklist(sm, p.indexer, nextEpochStartHeight, unqualifiedList)
	}
	if blkCtx.BlockHeight == epochStartHeight && hu.IsPost(config.Easter, epochStartHeight) {
		prevHeight, err := shiftCandidates(sm)
		if err != nil {
			return err
		}
		afterHeight, err := shiftKickoutList(sm)
		if err != nil {
			return err
		}
		if prevHeight != afterHeight {
			return errors.Wrap(ErrInconsistentHeight, "shifting candidate height is not same as shifting kickout height")
		}
	}
	return nil
}

func (p *governanceChainCommitteeProtocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	return handle(ctx, act, sm, p.indexer, p.addr.String())
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

func (p *governanceChainCommitteeProtocol) Delegates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	return p.sh.GetActiveBlockProducers(ctx, sr, false)
}

func (p *governanceChainCommitteeProtocol) NextDelegates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	return p.sh.GetActiveBlockProducers(ctx, sr, true)
}

func (p *governanceChainCommitteeProtocol) Candidates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	return p.sh.GetCandidates(ctx, sr, false)
}

func (p *governanceChainCommitteeProtocol) NextCandidates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	return p.sh.GetCandidates(ctx, sr, true)
}

func (p *governanceChainCommitteeProtocol) ReadState(
	ctx context.Context,
	sr protocol.StateReader,
	method []byte,
	args ...[]byte,
) ([]byte, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	epochNum := rp.GetEpochNum(blkCtx.BlockHeight) // tip
	epochStartHeight := rp.GetEpochHeight(epochNum)
	switch string(method) {
	case "CandidatesByEpoch":
		if len(args) != 0 {
			epochNum = byteutil.BytesToUint64(args[0])
			epochStartHeight = rp.GetEpochHeight(epochNum)
		}
		if p.indexer != nil {
			candidates, err := p.sh.GetCandidatesFromIndexer(ctx, epochStartHeight)
			if err == nil {
				return candidates.Serialize()
			}
			if err != nil {
				if errors.Cause(err) != ErrIndexerNotExist {
					return nil, err
				}
			}
		}
		candidates, err := p.sh.GetCandidates(ctx, sr, false)
		if err != nil {
			return nil, err
		}
		return candidates.Serialize()
	case "BlockProducersByEpoch":
		if len(args) != 0 {
			epochNum = byteutil.BytesToUint64(args[0])
			epochStartHeight = rp.GetEpochHeight(epochNum)
		}
		if p.indexer != nil {
			blockProducers, err := p.sh.GetBPFromIndexer(ctx, epochStartHeight)
			if err == nil {
				return blockProducers.Serialize()
			}
			if err != nil {
				if errors.Cause(err) != ErrIndexerNotExist {
					return nil, err
				}
			}
		}
		blockProducers, err := p.sh.GetBlockProducers(ctx, sr, false)
		if err != nil {
			return nil, err
		}
		return blockProducers.Serialize()
	case "ActiveBlockProducersByEpoch":
		if len(args) != 0 {
			epochNum = byteutil.BytesToUint64(args[0])
			epochStartHeight = rp.GetEpochHeight(epochNum)
		}
		if p.indexer != nil {
			activeBlockProducers, err := p.sh.GetABPFromIndexer(ctx, epochStartHeight)
			if err == nil {
				return activeBlockProducers.Serialize()
			}
			if err != nil {
				if errors.Cause(err) != ErrIndexerNotExist {
					return nil, err
				}
			}
		}
		activeBlockProducers, err := p.sh.GetActiveBlockProducers(ctx, sr, false)
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
	case "KickoutListByEpoch":
		if len(args) != 0 {
			epochNum = byteutil.BytesToUint64(args[0])
			epochStartHeight = rp.GetEpochHeight(epochNum)
		}
		if p.indexer != nil {
			kickoutList, err := p.indexer.KickoutList(epochStartHeight)
			if err == nil {
				return kickoutList.Serialize()
			}
			if err != nil {
				if errors.Cause(err) != ErrIndexerNotExist {
					return nil, err
				}
			}
		}
		kickoutList, err := p.sh.GetKickoutList(ctx, sr, false)
		if err != nil {
			return nil, err
		}
		return kickoutList.Serialize()
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
