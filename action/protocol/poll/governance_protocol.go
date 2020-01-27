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
	kickoutListByEpoch        KickoutListByEpoch
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
	kickoutIntensity          float64
}

// NewGovernanceChainCommitteeProtocol creates a Poll Protocol which fetch result from governance chain
func NewGovernanceChainCommitteeProtocol(
	candidatesByHeight CandidatesByHeight,
	kickoutListByEpoch KickoutListByEpoch,
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
	kickoutIntensity float64,
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
		kickoutListByEpoch:        kickoutListByEpoch,
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

func (p *governanceChainCommitteeProtocol) CreatePreStates(ctx context.Context, sm protocol.StateManager) error {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	epochNum := rp.GetEpochNum(blkCtx.BlockHeight)
	epochLastHeight := rp.GetEpochLastBlockHeight(epochNum)
	nextEpochStartHeight := rp.GetEpochHeight(epochNum + 1)
	hu := config.NewHeightUpgrade(&bcCtx.Genesis)
	if blkCtx.BlockHeight == epochLastHeight && hu.IsPost(config.English, nextEpochStartHeight) {
		// if the block height is the end of epoch and next epoch is after the English height, calculate blacklist for kick-out and write into state DB
		unqualifiedList, err := p.calculateKickoutBlackList(ctx, epochNum+1)
		if err != nil {
			return err
		}

		return setKickoutBlackList(sm, unqualifiedList, epochNum+1)
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
	return p.readActiveBlockProducersByEpoch(ctx, epochNum)
}

func (p *governanceChainCommitteeProtocol) CandidatesByHeight(ctx context.Context, height uint64) (state.CandidateList, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	return p.candidatesByHeight(p.sr, rp.GetEpochHeight(rp.GetEpochNum(height)))
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
	return p.candidatesByHeight(p.sr, rp.GetEpochHeight(epochNum))
}

func (p *governanceChainCommitteeProtocol) readBlockProducersByEpoch(ctx context.Context, epochNum uint64) (state.CandidateList, error) {
	candidates, err := p.readCandidatesByEpoch(ctx, epochNum)
	if err != nil {
		return nil, err
	}
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	epochHeight := rp.GetEpochHeight(epochNum)
	hu := config.NewHeightUpgrade(&bcCtx.Genesis)
	if hu.IsPre(config.English, epochHeight) {
		var blockProducers state.CandidateList
		for i, candidate := range candidates {
			if uint64(i) >= p.numCandidateDelegates {
				break
			}
			blockProducers = append(blockProducers, candidate)
		}
		return blockProducers, nil
	}

	// After English height, kick-out unqualified delegates based on productivity
	unqualifiedList, err := p.kickoutListByEpoch(p.sr, epochNum)
	if err != nil {
		return nil, err
	}
	// recalculate the voting power for blacklist delegates
	candidatesMap := make(map[string]*state.Candidate)
	updatedVotingPower := make(map[string]*big.Int)
	for _, cand := range candidates {
		candidatesMap[cand.Address] = cand
		if _, ok := unqualifiedList[cand.Address]; ok {
			// if it is an unqualified delegate, multiply the voting power with kick-out intensity rate
			votingPower := new(big.Float).SetInt(cand.Votes)
			newVotingPower, _ := votingPower.Mul(votingPower, big.NewFloat(p.kickoutIntensity)).Int(nil)
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
	epochNum uint64,
) (vote.Blacklist, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	englishEpochNum := rp.GetEpochNum(config.English)

	unqualifiedDelegates := vote.Blacklist{}
	if epochNum <= englishEpochNum+p.kickoutEpochPeriod {
		// if epoch number is smaller than EnglishHeightEpoch+K(kickout period), calculate it one-by-one (initialize).
		round := epochNum - englishEpochNum // 0, 1, 2, 3 .. K
		for {
			if round == 0 {
				break
			}
			// N-1, N-2, ..., N-K
			isCurrentEpoch := false
			if round == 1 {
				isCurrentEpoch = true
			}
			uq, err := p.calculateUnproductiveDelegatesByEpoch(ctx, epochNum-round, isCurrentEpoch)
			if err != nil {
				return nil, err
			}
			for _, addr := range uq {
				if _, ok := unqualifiedDelegates[addr]; !ok {
					unqualifiedDelegates[addr] = 1
				} else {
					unqualifiedDelegates[addr]++
				}
			}
			round--
		}
		return unqualifiedDelegates, nil
	}

	// Blacklist[N] = Blacklist[N-1] - Low-productivity-list[N-K-1] + Low-productivity-list[N-1]
	blackListMapping, err := p.kickoutListByEpoch(p.sr, epochNum-1)
	if err != nil {
		return nil, err
	}
	skipList, err := p.calculateUnproductiveDelegatesByEpoch(ctx, epochNum-p.kickoutEpochPeriod-1, false)
	if err != nil {
		return nil, err
	}
	for _, addr := range skipList {
		if _, ok := blackListMapping[addr]; !ok {
			log.L().Fatal("skipping list element doesn't exist among one of existing map")
			continue
		}
		blackListMapping[addr]--
	}
	addList, err := p.calculateUnproductiveDelegatesByEpoch(ctx, epochNum-1, true)
	if err != nil {
		return nil, err
	}
	for _, addr := range addList {
		if _, ok := blackListMapping[addr]; ok {
			blackListMapping[addr]++
			continue
		}
		blackListMapping[addr] = 1
	}

	for addr, count := range blackListMapping {
		if count <= 0 {
			delete(blackListMapping, addr)
		}
	}

	return blackListMapping, nil
}

func (p *governanceChainCommitteeProtocol) calculateUnproductiveDelegatesByEpoch(
	ctx context.Context,
	epochNum uint64,
	isCurrentEpoch bool,
) ([]string, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	numBlks, produce, err := p.productivityByEpoch(ctx, epochNum)
	if err != nil {
		return nil, err
	}
	if isCurrentEpoch {
		// The current block is not included, so that we need to add it to the stats
		numBlks++
		produce[blkCtx.Producer.String()]++
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
