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
}

// NewGovernanceChainCommitteeProtocol creates a Poll Protocol which fetch result from governance chain
func NewGovernanceChainCommitteeProtocol(
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
		if err := setNextEpochBlacklist(sm, &vote.Blacklist{
			BlacklistInfos: make(map[string]uint32),
			IntensityRate:  p.kickoutIntensity,
		}); err != nil {
			return err
		}
	}
	return setCandidates(ctx, sm, ds, uint64(1))
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
		return setNextEpochBlacklist(sm, unqualifiedList)
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
		return nil
	}
	return nil
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
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	tipEpochNum := rp.GetEpochNum(bcCtx.Tip.Height)
	if tipEpochNum+1 == epochNum {
		return p.readActiveBlockProducersByEpoch(ctx, epochNum, true)
	} else if tipEpochNum == epochNum {
		return p.readActiveBlockProducersByEpoch(ctx, epochNum, false)
	}
	return nil, errors.Errorf("wrong epochNumber to get delegates, epochNumber %d can't be less than tip epoch number %d", epochNum, tipEpochNum)
}

func (p *governanceChainCommitteeProtocol) CandidatesByHeight(ctx context.Context, height uint64) (state.CandidateList, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	tipEpochNum := rp.GetEpochNum(bcCtx.Tip.Height)
	targetEpochNum := rp.GetEpochNum(height)
	targetEpochStartHeight := rp.GetEpochHeight(targetEpochNum)
	if tipEpochNum+1 == targetEpochNum {
		return p.readCandidatesByHeight(ctx, targetEpochStartHeight, true)
	} else if tipEpochNum == targetEpochNum {
		return p.readCandidatesByHeight(ctx, targetEpochStartHeight, false)
	}
	return nil, errors.Errorf("wrong epochNumber to get candidatesbyHeight, target epochNumber %d can't be less than tip epoch number %d", targetEpochNum, tipEpochNum)
}

func (p *governanceChainCommitteeProtocol) ReadState(
	ctx context.Context,
	sm protocol.StateReader,
	method []byte,
	args ...[]byte,
) ([]byte, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	tipEpoch := rp.GetEpochNum(blkCtx.BlockHeight)
	switch string(method) {
	case "CandidatesByEpoch":
		if len(args) != 0 {
			inputEpochNum := byteutil.BytesToUint64(args[0])
			if inputEpochNum != tipEpoch {
				return nil, errors.New("previous epoch data isn't available with non-archive node")
			}
		}
		delegates, err := p.readCandidatesByEpoch(ctx, tipEpoch, false)
		if err != nil {
			return nil, err
		}
		return delegates.Serialize()
	case "BlockProducersByEpoch":
		if len(args) != 0 {
			inputEpochNum := byteutil.BytesToUint64(args[0])
			if inputEpochNum != tipEpoch {
				return nil, errors.New("previous epoch data isn't available with non-archive node")
			}
		}
		blockProducers, err := p.readBlockProducersByEpoch(ctx, byteutil.BytesToUint64(args[0]), false)
		if err != nil {
			return nil, err
		}
		return blockProducers.Serialize()
	case "ActiveBlockProducersByEpoch":
		if len(args) != 0 {
			inputEpochNum := byteutil.BytesToUint64(args[0])
			if inputEpochNum != tipEpoch {
				return nil, errors.New("previous epoch data isn't available with non-archive node")
			}
		}
		activeBlockProducers, err := p.readActiveBlockProducersByEpoch(ctx, byteutil.BytesToUint64(args[0]), false)
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
			inputEpochNum := byteutil.BytesToUint64(args[0])
			if inputEpochNum != tipEpoch {
				return nil, errors.New("previous epoch data isn't available with non-archive node")
			}
		}
		kickoutList, err := p.readKickoutList(ctx, tipEpoch, false)
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

func (p *governanceChainCommitteeProtocol) readCandidatesByEpoch(ctx context.Context, epochNum uint64, readFromNext bool) (state.CandidateList, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	return p.readCandidatesByHeight(ctx, rp.GetEpochHeight(epochNum), readFromNext)
}

func (p *governanceChainCommitteeProtocol) readCandidatesByHeight(ctx context.Context, epochStartHeight uint64, readFromNext bool) (state.CandidateList, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	hu := config.NewHeightUpgrade(&bcCtx.Genesis)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	if hu.IsPre(config.Easter, epochStartHeight) {
		return p.candidatesByHeight(p.sr, epochStartHeight)
	}
	sc, stateHeight, err := p.getCandidates(p.sr, readFromNext)
	if err != nil {
		return nil, err
	}
	// to catch the corner case that since the new block is committed, shift occurs in the middle of processing the request
	if epochStartHeight < rp.GetEpochHeight(rp.GetEpochNum(stateHeight)) {
		return nil, errors.Wrap(ErrInconsistentHeight, "state factory height became larger than target height")
	}
	return sc, nil
}

func (p *governanceChainCommitteeProtocol) readBlockProducersByEpoch(ctx context.Context, epochNum uint64, readFromNext bool) (state.CandidateList, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	candidates, err := p.readCandidatesByEpoch(ctx, epochNum, readFromNext)
	if err != nil {
		return nil, err
	}
	hu := config.NewHeightUpgrade(&bcCtx.Genesis)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	if hu.IsPre(config.Easter, rp.GetEpochHeight(epochNum)) || epochNum == 1 {
		var blockProducers state.CandidateList
		for i, candidate := range candidates {
			if uint64(i) >= p.numCandidateDelegates {
				break
			}
			blockProducers = append(blockProducers, candidate)
		}
		return blockProducers, nil
	}

	// After Easter height, kick-out unqualified delegates based on productivity
	unqualifiedList, err := p.readKickoutList(ctx, epochNum, readFromNext)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read kick-out list")
	}
	// recalculate the voting power for blacklist delegates
	candidatesMap := make(map[string]*state.Candidate)
	updatedVotingPower := make(map[string]*big.Int)
	intensityRate := float64(uint32(100)-unqualifiedList.IntensityRate) / float64(100)
	for _, cand := range candidates {
		candidatesMap[cand.Address] = cand
		if _, ok := unqualifiedList.BlacklistInfos[cand.Address]; ok {
			// if it is an unqualified delegate, multiply the voting power with kick-out intensity rate
			votingPower := new(big.Float).SetInt(cand.Votes)
			newVotingPower, _ := votingPower.Mul(votingPower, big.NewFloat(intensityRate)).Int(nil)
			updatedVotingPower[cand.Address] = newVotingPower
		} else {
			updatedVotingPower[cand.Address] = cand.Votes
		}
	}
	// sort again with updated voting power
	sorted := util.Sort(updatedVotingPower, epochNum)
	var verifiedCandidates state.CandidateList
	for i, name := range sorted {
		if uint64(i) >= p.numCandidateDelegates {
			break
		}
		verifiedCandidates = append(verifiedCandidates, candidatesMap[name])
	}

	return verifiedCandidates, nil
}

func (p *governanceChainCommitteeProtocol) readActiveBlockProducersByEpoch(ctx context.Context, epochNum uint64, readFromNext bool) (state.CandidateList, error) {
	blockProducers, err := p.readBlockProducersByEpoch(ctx, epochNum, readFromNext)
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

func (p *governanceChainCommitteeProtocol) readKickoutList(ctx context.Context, epochNum uint64, readFromNext bool) (*vote.Blacklist, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	unqualifiedList, stateHeight, err := p.getKickoutList(p.sr, readFromNext)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get kickout list when reading from state DB in epoch %d", epochNum)
	}
	// to catch the corner case that since the new block is committed, shift occurs in the middle of processing the request
	if epochNum < rp.GetEpochNum(stateHeight) {
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
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	easterEpochNum := rp.GetEpochNum(config.Easter)

	nextBlacklist := &vote.Blacklist{
		IntensityRate: p.kickoutIntensity,
	}
	upd, err := p.getUnproductiveDelegate(p.sr)
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
		uq, err := p.calculateUnproductiveDelegatesByEpoch(ctx, epochNum-1)
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
	prevBlacklist, _, err := p.getKickoutList(p.sr, false)
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
	addList, err := p.calculateUnproductiveDelegatesByEpoch(ctx, epochNum-1)
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
	epochNum uint64,
) ([]string, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	numBlks, produce, err := p.productivityByEpoch(ctx, epochNum)
	if err != nil {
		return nil, err
	}
	// The current block is not included, so that we need to add it to the stats
	numBlks++
	produce[blkCtx.Producer.String()]++

	unqualified := make([]string, 0)
	expectedNumBlks := numBlks / uint64(len(produce))
	for addr, actualNumBlks := range produce {
		if actualNumBlks*100/expectedNumBlks < p.productivityThreshold {
			unqualified = append(unqualified, addr)
		}
	}

	return unqualified, nil
}
