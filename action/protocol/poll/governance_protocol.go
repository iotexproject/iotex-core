// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"math/big"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/committee"
	"github.com/iotexproject/iotex-election/db"
	"github.com/iotexproject/iotex-election/util"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

type governanceChainCommitteeProtocol struct {
	candidatesByHeight        CandidatesByHeight
	getCandidates             GetCandidates
	getKickoutList            GetKickoutList
	getUnproductiveDelegate   GetUnproductiveDelegate
	getBlockTime              GetBlockTime
	electionCommittee         committee.Committee
	initGravityChainHeight    uint64
	numCandidateDelegates     uint64
	numDelegates              uint64
	addr                      address.Address
	initialCandidatesInterval time.Duration
	sr                        protocol.StateReader
	productivityByEpoch       ProductivityByEpoch
	productivityThreshold     uint64
	kickoutEpochPeriod        uint64
	kickoutIntensity          uint32
	maxKickoutPeriod          uint64
	indexer                   *CandidateIndexer
}

// NewGovernanceChainCommitteeProtocol creates a Poll Protocol which fetch result from governance chain
func NewGovernanceChainCommitteeProtocol(
	candidatesIndexer *CandidateIndexer,
	candidatesByHeight CandidatesByHeight,
	getCandidates GetCandidates,
	getKickoutList GetKickoutList,
	getUnproductiveDelegate GetUnproductiveDelegate,
	electionCommittee committee.Committee,
	initGravityChainHeight uint64,
	getBlockTime GetBlockTime,
	numCandidateDelegates uint64,
	numDelegates uint64,
	initialCandidatesInterval time.Duration,
	sr protocol.StateReader,
	productivityByEpoch ProductivityByEpoch,
	productivityThreshold uint64,
	kickoutEpochPeriod uint64,
	kickoutIntensity uint32,
	maxKickoutPeriod uint64,
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
		indexer:                   candidatesIndexer,
		candidatesByHeight:        candidatesByHeight,
		getCandidates:             getCandidates,
		getKickoutList:            getKickoutList,
		getUnproductiveDelegate:   getUnproductiveDelegate,
		electionCommittee:         electionCommittee,
		initGravityChainHeight:    initGravityChainHeight,
		getBlockTime:              getBlockTime,
		numCandidateDelegates:     numCandidateDelegates,
		numDelegates:              numDelegates,
		addr:                      addr,
		initialCandidatesInterval: initialCandidatesInterval,
		sr:                        sr,
		productivityByEpoch:       productivityByEpoch,
		productivityThreshold:     productivityThreshold,
		kickoutEpochPeriod:        kickoutEpochPeriod,
		kickoutIntensity:          kickoutIntensity,
		maxKickoutPeriod:          maxKickoutPeriod,
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
			&vote.Blacklist{
				BlacklistInfos: make(map[string]uint32),
				IntensityRate:  p.kickoutIntensity,
			}); err != nil {
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
		unqualifiedList, err := p.calculateKickoutBlackList(ctx, sm, epochNum+1)
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
	return p.readActiveBlockProducers(ctx, sr, false)
}

func (p *governanceChainCommitteeProtocol) NextDelegates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	return p.readActiveBlockProducers(ctx, sr, true)
}

func (p *governanceChainCommitteeProtocol) Candidates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	return p.readCandidates(ctx, sr, false)
}

func (p *governanceChainCommitteeProtocol) NextCandidates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	return p.readCandidates(ctx, sr, true)
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
			candidates, err := p.readCandidatesFromIndexer(ctx, epochStartHeight)
			if err == nil {
				return candidates.Serialize()
			}
			if err != nil {
				if errors.Cause(err) != ErrIndexerNotExist {
					return nil, err
				}
			}
		}
		candidates, err := p.readCandidates(ctx, sr, false)
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
			blockProducers, err := p.readBPFromIndexer(ctx, epochStartHeight)
			if err == nil {
				return blockProducers.Serialize()
			}
			if err != nil {
				if errors.Cause(err) != ErrIndexerNotExist {
					return nil, err
				}
			}
		}
		blockProducers, err := p.readBlockProducers(ctx, sr, false)
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
			activeBlockProducers, err := p.readABPFromIndexer(ctx, epochStartHeight)
			if err == nil {
				return activeBlockProducers.Serialize()
			}
			if err != nil {
				if errors.Cause(err) != ErrIndexerNotExist {
					return nil, err
				}
			}
		}
		activeBlockProducers, err := p.readActiveBlockProducers(ctx, sr, false)
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
		kickoutList, err := p.readKickoutList(ctx, sr, false)
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

func (p *governanceChainCommitteeProtocol) readCandidates(ctx context.Context, sr protocol.StateReader, readFromNext bool) (state.CandidateList, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	hu := config.NewHeightUpgrade(&bcCtx.Genesis)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	targetHeight, err := sr.Height()
	if err != nil {
		return nil, err
	}
	// make sure it's epochStartHeight
	targetEpochStartHeight := rp.GetEpochHeight(rp.GetEpochNum(targetHeight))
	if readFromNext {
		targetEpochNum := rp.GetEpochNum(targetEpochStartHeight) + 1
		targetEpochStartHeight = rp.GetEpochHeight(targetEpochNum) // next epoch start height
	}
	if hu.IsPre(config.Easter, targetEpochStartHeight) {
		return p.candidatesByHeight(sr, targetEpochStartHeight)
	}
	// After Easter height, kick-out unqualified delegates based on productivity
	candidates, stateHeight, err := p.getCandidates(sr, readFromNext)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get candidates at height %d after easter height", targetEpochStartHeight)
	}
	// to catch the corner case that since the new block is committed, shift occurs in the middle of processing the request
	if rp.GetEpochNum(targetEpochStartHeight) < rp.GetEpochNum(stateHeight) {
		return nil, errors.Wrap(ErrInconsistentHeight, "state factory epoch number became larger than target epoch number")
	}
	unqualifiedList, err := p.readKickoutList(ctx, sr, readFromNext)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get kickout list at height %d", targetEpochStartHeight)
	}
	// recalculate the voting power for blacklist delegates
	return filterCandidates(candidates, unqualifiedList, targetEpochStartHeight)
}

func (p *governanceChainCommitteeProtocol) readCandidatesFromIndexer(ctx context.Context, epochStartHeight uint64) (state.CandidateList, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	hu := config.NewHeightUpgrade(&bcCtx.Genesis)
	candidates, err := p.indexer.CandidateList(epochStartHeight)
	if err != nil {
		return nil, err
	}
	if hu.IsPre(config.Easter, epochStartHeight) {
		return candidates, nil
	}
	// After Easter height, kick-out unqualified delegates based on productivity
	kickoutList, err := p.indexer.KickoutList(epochStartHeight)
	if err != nil {
		return nil, err
	}
	// recalculate the voting power for blacklist delegates
	return filterCandidates(candidates, kickoutList, epochStartHeight)
}

func (p *governanceChainCommitteeProtocol) readBlockProducers(ctx context.Context, sr protocol.StateReader, readFromNext bool) (state.CandidateList, error) {
	candidates, err := p.readCandidates(ctx, sr, readFromNext)
	if err != nil {
		return nil, err
	}
	return p.calculateBlockProducer(candidates)
}

func (p *governanceChainCommitteeProtocol) readBPFromIndexer(ctx context.Context, epochStartHeight uint64) (state.CandidateList, error) {
	candidates, err := p.readCandidatesFromIndexer(ctx, epochStartHeight)
	if err != nil {
		return nil, err
	}
	return p.calculateBlockProducer(candidates)
}

func (p *governanceChainCommitteeProtocol) readActiveBlockProducers(ctx context.Context, sr protocol.StateReader, readFromNext bool) (state.CandidateList, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	targetHeight, err := sr.Height()
	if err != nil {
		return nil, err
	}
	// make sure it's epochStartHeight
	targetEpochStartHeight := rp.GetEpochHeight(rp.GetEpochNum(targetHeight))
	if readFromNext {
		targetEpochNum := rp.GetEpochNum(targetEpochStartHeight) + 1
		targetEpochStartHeight = rp.GetEpochHeight(targetEpochNum) // next epoch start height
	}
	blockProducers, err := p.readBlockProducers(ctx, sr, readFromNext)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read block producers at height %d", targetEpochStartHeight)
	}
	return p.calculateActiveBlockProducer(ctx, blockProducers, targetEpochStartHeight)
}

func (p *governanceChainCommitteeProtocol) readABPFromIndexer(ctx context.Context, epochStartHeight uint64) (state.CandidateList, error) {
	blockProducers, err := p.readBPFromIndexer(ctx, epochStartHeight)
	if err != nil {
		return nil, err
	}
	return p.calculateActiveBlockProducer(ctx, blockProducers, epochStartHeight)
}

func (p *governanceChainCommitteeProtocol) readKickoutList(ctx context.Context, sr protocol.StateReader, readFromNext bool) (*vote.Blacklist, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	hu := config.NewHeightUpgrade(&bcCtx.Genesis)
	targetHeight, err := sr.Height()
	if err != nil {
		return nil, err
	}
	// make sure it's epochStartHeight
	targetEpochStartHeight := rp.GetEpochHeight(rp.GetEpochHeight(targetHeight))
	if readFromNext {
		targetEpochNum := rp.GetEpochNum(targetEpochStartHeight) + 1
		targetEpochStartHeight = rp.GetEpochHeight(targetEpochNum) // next epoch start height
	}
	if hu.IsPre(config.Easter, targetEpochStartHeight) {
		return nil, errors.New("Before Easter, there is no blacklist in stateDB")
	}
	unqualifiedList, stateHeight, err := p.getKickoutList(p.sr, readFromNext)
	if err != nil {
		return nil, err
	}
	// to catch the corner case that since the new block is committed, shift occurs in the middle of processing the request
	if rp.GetEpochNum(targetEpochStartHeight) < rp.GetEpochNum(stateHeight) {
		return nil, errors.Wrap(ErrInconsistentHeight, "state factory tip epoch number became larger than target epoch number")
	}
	return unqualifiedList, nil
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

func (p *governanceChainCommitteeProtocol) calculateKickoutBlackList(
	ctx context.Context,
	sm protocol.StateManager,
	epochNum uint64,
) (*vote.Blacklist, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	hu := config.NewHeightUpgrade(&bcCtx.Genesis)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	easterEpochNum := rp.GetEpochNum(hu.EasterBlockHeight())

	nextBlacklist := &vote.Blacklist{
		IntensityRate: p.kickoutIntensity,
	}
	upd, err := p.getUnproductiveDelegate(sm)
	if err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			if upd, err = vote.NewUnproductiveDelegate(p.kickoutEpochPeriod, p.maxKickoutPeriod); err != nil {
				return nil, errors.Wrap(err, "failed to make new upd")
			}
		} else {
			return nil, errors.Wrapf(err, "failed to read upd struct from state DB at epoch number %d", epochNum)
		}
	}
	unqualifiedDelegates := make(map[string]uint32)
	if epochNum <= easterEpochNum+p.kickoutEpochPeriod {
		// if epoch number is smaller than easterEpochNum+K(kickout period), calculate it one-by-one (initialize).
		log.L().Debug("Before using kick-out blacklist",
			zap.Uint64("epochNum", epochNum),
			zap.Uint64("easterEpochNum", easterEpochNum),
			zap.Uint64("kickoutEpochPeriod", p.kickoutEpochPeriod),
		)
		existinglist := upd.DelegateList()
		for _, listByEpoch := range existinglist {
			for _, addr := range listByEpoch {
				if _, ok := unqualifiedDelegates[addr]; !ok {
					unqualifiedDelegates[addr] = 1
				} else {
					unqualifiedDelegates[addr]++
				}
			}
		}
		// calculate upd of epochNum-1 (latest)
		uq, err := p.calculateUnproductiveDelegatesByEpoch(ctx, sm, epochNum-1)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to calculate current epoch upd %d", epochNum-1)
		}
		for _, addr := range uq {
			if _, ok := unqualifiedDelegates[addr]; !ok {
				unqualifiedDelegates[addr] = 1
			} else {
				unqualifiedDelegates[addr]++
			}
		}
		if err := upd.AddRecentUPD(uq); err != nil {
			return nil, errors.Wrap(err, "failed to add recent upd")
		}
		nextBlacklist.BlacklistInfos = unqualifiedDelegates
		return nextBlacklist, setUnproductiveDelegates(sm, upd)
	}
	// Blacklist[N] = Blacklist[N-1] - Low-productivity-list[N-K-1] + Low-productivity-list[N-1]
	log.L().Debug("Using kick-out blacklist",
		zap.Uint64("epochNum", epochNum),
		zap.Uint64("easterEpochNum", easterEpochNum),
		zap.Uint64("kickoutEpochPeriod", p.kickoutEpochPeriod),
	)
	prevBlacklist, _, err := p.getKickoutList(sm, false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read latest kick-out list")
	}
	blacklistMap := prevBlacklist.BlacklistInfos
	if blacklistMap == nil {
		blacklistMap = make(map[string]uint32)
	}
	skipList := upd.ReadOldestUPD()
	for _, addr := range skipList {
		if _, ok := blacklistMap[addr]; !ok {
			log.L().Fatal("skipping list element doesn't exist among one of existing map")
			continue
		}
		blacklistMap[addr]--
	}
	addList, err := p.calculateUnproductiveDelegatesByEpoch(ctx, sm, epochNum-1)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to calculate current epoch upd %d", epochNum-1)
	}
	if err := upd.AddRecentUPD(addList); err != nil {
		return nil, errors.Wrap(err, "failed to add recent upd")
	}
	for _, addr := range addList {
		if _, ok := blacklistMap[addr]; ok {
			blacklistMap[addr]++
			continue
		}
		blacklistMap[addr] = 1
	}

	for addr, count := range blacklistMap {
		if count == 0 {
			delete(blacklistMap, addr)
		}
	}
	nextBlacklist.BlacklistInfos = blacklistMap
	return nextBlacklist, setUnproductiveDelegates(sm, upd)
}

func (p *governanceChainCommitteeProtocol) calculateUnproductiveDelegatesByEpoch(
	ctx context.Context,
	sm protocol.StateManager,
	epochNum uint64,
) ([]string, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	delegates, err := p.readActiveBlockProducers(ctx, sm, false)
	if err != nil {
		return nil, err
	}
	numBlks, produce, err := p.productivityByEpoch(ctx, epochNum)
	if err != nil {
		return nil, err
	}
	// The current block is not included, so add it
	numBlks++
	if _, ok := produce[blkCtx.Producer.String()]; ok {
		produce[blkCtx.Producer.String()]++
	} else {
		produce[blkCtx.Producer.String()] = 1
	}

	for _, abp := range delegates {
		if _, ok := produce[abp.Address]; !ok {
			produce[abp.Address] = 0
		}
	}
	unqualified := make([]string, 0)
	expectedNumBlks := numBlks / uint64(len(produce))
	for addr, actualNumBlks := range produce {
		if actualNumBlks*100/expectedNumBlks < p.productivityThreshold {
			unqualified = append(unqualified, addr)
		}
	}

	return unqualified, nil
}

// filterCandidates returns filtered candidate list by given raw candidate/ kick-out list
func filterCandidates(
	candidates state.CandidateList,
	unqualifiedList *vote.Blacklist,
	epochStartHeight uint64,
) (state.CandidateList, error) {
	candidatesMap := make(map[string]*state.Candidate)
	updatedVotingPower := make(map[string]*big.Int)
	intensityRate := float64(uint32(100)-unqualifiedList.IntensityRate) / float64(100)
	for _, cand := range candidates {
		filterCand := cand.Clone()
		if _, ok := unqualifiedList.BlacklistInfos[cand.Address]; ok {
			// if it is an unqualified delegate, multiply the voting power with kick-out intensity rate
			votingPower := new(big.Float).SetInt(filterCand.Votes)
			filterCand.Votes, _ = votingPower.Mul(votingPower, big.NewFloat(intensityRate)).Int(nil)
		}
		updatedVotingPower[filterCand.Address] = filterCand.Votes
		candidatesMap[filterCand.Address] = filterCand
	}
	// sort again with updated voting power
	sorted := util.Sort(updatedVotingPower, epochStartHeight)
	var verifiedCandidates state.CandidateList
	for _, name := range sorted {
		verifiedCandidates = append(verifiedCandidates, candidatesMap[name])
	}

	return verifiedCandidates, nil
}

// calculateBlockProducer calculates block producer by given candidate list
func (p *governanceChainCommitteeProtocol) calculateBlockProducer(
	candidates state.CandidateList,
) (state.CandidateList, error) {
	var blockProducers state.CandidateList
	for i, candidate := range candidates {
		if uint64(i) >= p.numCandidateDelegates {
			break
		}
		if candidate.Votes.Cmp(big.NewInt(0)) == 0 {
			// if the voting power is 0, exclude from being a block producer(hard kickout)
			continue
		}
		blockProducers = append(blockProducers, candidate)
	}
	return blockProducers, nil
}

// calculateActiveBlockProducer calculates active block producer by given block producer list
func (p *governanceChainCommitteeProtocol) calculateActiveBlockProducer(
	ctx context.Context,
	blockProducers state.CandidateList,
	epochStartHeight uint64,
) (state.CandidateList, error) {
	var blockProducerList []string
	blockProducerMap := make(map[string]*state.Candidate)
	for _, bp := range blockProducers {
		blockProducerList = append(blockProducerList, bp.Address)
		blockProducerMap[bp.Address] = bp
	}
	crypto.SortCandidates(blockProducerList, epochStartHeight, crypto.CryptoSeed)

	length := int(p.numDelegates)
	if len(blockProducerList) < length {
		// TODO: if the number of delegates is smaller than expected, should it return error or not?
		length = len(blockProducerList)
		log.L().Warn(
			"the number of block producer is less than expected",
			zap.Int("actual block producer", len(blockProducerList)),
			zap.Uint64("expected", p.numDelegates),
		)
	}
	var activeBlockProducers state.CandidateList
	for i := 0; i < length; i++ {
		activeBlockProducers = append(activeBlockProducers, blockProducerMap[blockProducerList[i]])
	}

	return activeBlockProducers, nil
}
