// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"math/big"
	"strconv"

	"github.com/iotexproject/iotex-election/util"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
)

// Slasher is the module to slash candidates
type Slasher struct {
	productivity          Productivity
	getCandidates         GetCandidates
	getProbationList      GetProbationList
	getUnprodDelegate     GetUnproductiveDelegate
	indexer               *CandidateIndexer
	numCandidateDelegates uint64
	numDelegates          uint64
	numOfBlocksByEpoch    uint64
	prodThreshold         uint64
	probationEpochPeriod  uint64
	maxProbationPeriod    uint64
	probationIntensity    uint32
}

// NewSlasher returns a new Slasher
func NewSlasher(
	productivity Productivity,
	getCandidates GetCandidates,
	getProbationList GetProbationList,
	getUnprodDelegate GetUnproductiveDelegate,
	indexer *CandidateIndexer,
	numCandidateDelegates, numDelegates, dardanellesNumSubEpochs, thres, koPeriod, maxKoPeriod uint64,
	koIntensity uint32,
) (*Slasher, error) {
	return &Slasher{
		productivity:          productivity,
		getCandidates:         getCandidates,
		getProbationList:      getProbationList,
		getUnprodDelegate:     getUnprodDelegate,
		indexer:               indexer,
		numCandidateDelegates: numCandidateDelegates,
		numDelegates:          numDelegates,
		numOfBlocksByEpoch:    numDelegates * dardanellesNumSubEpochs,
		prodThreshold:         thres,
		probationEpochPeriod:  koPeriod,
		maxProbationPeriod:    maxKoPeriod,
		probationIntensity:    koIntensity,
	}, nil
}

// CreateGenesisStates creates genesis state for slasher
func (sh *Slasher) CreateGenesisStates(ctx context.Context, sm protocol.StateManager, indexer *CandidateIndexer) error {
	g := genesis.MustExtractGenesisContext(ctx)
	if g.IsEaster(uint64(1)) {
		if err := setNextEpochProbationList(sm,
			indexer,
			uint64(1),
			vote.NewProbationList(sh.probationIntensity)); err != nil {
			return err
		}
	}
	return nil
}

// CreatePreStates is to setup probation list
func (sh *Slasher) CreatePreStates(ctx context.Context, sm protocol.StateManager, indexer *CandidateIndexer) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	featureWithHeightCtx := protocol.MustGetFeatureWithHeightCtx(ctx)
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	epochNum := rp.GetEpochNum(blkCtx.BlockHeight)
	epochStartHeight := rp.GetEpochHeight(epochNum)
	epochLastHeight := rp.GetEpochLastBlockHeight(epochNum)
	nextEpochStartHeight := rp.GetEpochHeight(epochNum + 1)
	if featureCtx.UpdateBlockMeta {
		if err := sh.updateCurrentBlockMeta(ctx, sm); err != nil {
			return errors.Wrap(err, "faild to update current epoch meta")
		}
	}
	if blkCtx.BlockHeight == epochLastHeight && featureWithHeightCtx.CalculateProbationList(nextEpochStartHeight) {
		// if the block height is the end of epoch and next epoch is after the Easter height, calculate probation list for probation and write into state DB
		unqualifiedList, err := sh.CalculateProbationList(ctx, sm, epochNum+1)
		if err != nil {
			return err
		}
		return setNextEpochProbationList(sm, indexer, nextEpochStartHeight, unqualifiedList)
	}
	if blkCtx.BlockHeight == epochStartHeight && featureWithHeightCtx.CalculateProbationList(epochStartHeight) {
		prevHeight, err := shiftCandidates(sm)
		if err != nil {
			return err
		}
		afterHeight, err := shiftProbationList(sm)
		if err != nil {
			return err
		}
		if prevHeight != afterHeight {
			return errors.Wrap(ErrInconsistentHeight, "shifting candidate height is not same as shifting probation height")
		}
	}
	return nil
}

// ReadState defines slasher's read methods.
func (sh *Slasher) ReadState(
	ctx context.Context,
	sr protocol.StateReader,
	indexer *CandidateIndexer,
	method []byte,
	args ...[]byte,
) ([]byte, uint64, error) {
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	targetHeight, err := sr.Height()
	if err != nil {
		return nil, uint64(0), err
	}
	epochNum := rp.GetEpochNum(targetHeight)
	epochStartHeight := rp.GetEpochHeight(epochNum)
	if len(args) != 0 {
		epochNumArg, err := strconv.ParseUint(string(args[0]), 10, 64)
		if err != nil {
			return nil, uint64(0), err
		}
		if indexer == nil {
			// consistency check between sr.height and epochNumArg in case of using state reader(not indexer)
			if epochNum != epochNumArg {
				return nil, uint64(0), errors.New("Slasher ReadState arg epochNumber should be same as state reader height, need to set argument/height consistently")
			}
		}
		epochStartHeight = rp.GetEpochHeight(epochNumArg)
	}
	switch string(method) {
	case "CandidatesByEpoch":
		if indexer != nil {
			candidates, err := sh.GetCandidatesFromIndexer(ctx, epochStartHeight)
			if err == nil {
				data, err := candidates.Serialize()
				if err != nil {
					return nil, uint64(0), err
				}
				return data, epochStartHeight, nil
			}
			if err != nil && errors.Cause(err) != ErrIndexerNotExist {
				return nil, uint64(0), err
			}
		}
		candidates, height, err := sh.GetCandidates(ctx, sr, false)
		if err != nil {
			return nil, uint64(0), err
		}
		data, err := candidates.Serialize()
		if err != nil {
			return nil, uint64(0), err
		}
		return data, height, nil
	case "BlockProducersByEpoch":
		if indexer != nil {
			blockProducers, err := sh.GetBPFromIndexer(ctx, epochStartHeight)
			if err == nil {
				data, err := blockProducers.Serialize()
				if err != nil {
					return nil, uint64(0), err
				}
				return data, epochStartHeight, nil
			}
			if err != nil && errors.Cause(err) != ErrIndexerNotExist {
				return nil, uint64(0), err
			}
		}
		bp, height, err := sh.GetBlockProducers(ctx, sr, false)
		if err != nil {
			return nil, uint64(0), err
		}
		data, err := bp.Serialize()
		if err != nil {
			return nil, uint64(0), err
		}
		return data, height, nil
	case "ActiveBlockProducersByEpoch":
		if indexer != nil {
			activeBlockProducers, err := sh.GetABPFromIndexer(ctx, epochStartHeight)
			if err == nil {
				data, err := activeBlockProducers.Serialize()
				if err != nil {
					return nil, uint64(0), err
				}
				return data, epochStartHeight, nil
			}
			if err != nil && errors.Cause(err) != ErrIndexerNotExist {
				return nil, uint64(0), err
			}
		}
		abp, height, err := sh.GetActiveBlockProducers(ctx, sr, false)
		if err != nil {
			return nil, uint64(0), err
		}
		data, err := abp.Serialize()
		if err != nil {
			return nil, uint64(0), err
		}
		return data, height, nil
	case "ProbationListByEpoch":
		if indexer != nil {
			probationList, err := indexer.ProbationList(epochStartHeight)
			if err == nil {
				data, err := probationList.Serialize()
				if err != nil {
					return nil, uint64(0), err
				}
				return data, epochStartHeight, nil
			}
			if err != nil && errors.Cause(err) != ErrIndexerNotExist {
				return nil, uint64(0), err
			}
		}
		probationList, height, err := sh.GetProbationList(ctx, sr, false)
		if err != nil {
			return nil, uint64(0), err
		}
		data, err := probationList.Serialize()
		if err != nil {
			return nil, uint64(0), err
		}
		return data, height, nil
	default:
		return nil, uint64(0), errors.New("corresponding method isn't found")
	}
}

// GetCandidates returns filtered candidate list
func (sh *Slasher) GetCandidates(ctx context.Context, sr protocol.StateReader, readFromNext bool) (state.CandidateList, uint64, error) {
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	featureWithHeightCtx := protocol.MustGetFeatureWithHeightCtx(ctx)
	targetHeight, err := sr.Height()
	if err != nil {
		return nil, uint64(0), err
	}
	// make sure it's epochStartHeight
	targetEpochStartHeight := rp.GetEpochHeight(rp.GetEpochNum(targetHeight))
	if readFromNext {
		targetEpochNum := rp.GetEpochNum(targetEpochStartHeight) + 1
		targetEpochStartHeight = rp.GetEpochHeight(targetEpochNum) // next epoch start height
	}
	beforeEaster := !featureWithHeightCtx.CalculateProbationList(targetEpochStartHeight)
	candidates, stateHeight, err := sh.getCandidates(sr, targetEpochStartHeight, beforeEaster, readFromNext)
	if err != nil {
		return nil, uint64(0), errors.Wrapf(err, "failed to get candidates at height %d", targetEpochStartHeight)
	}
	// to catch the corner case that since the new block is committed, shift occurs in the middle of processing the request
	if rp.GetEpochNum(targetEpochStartHeight) < rp.GetEpochNum(stateHeight) {
		return nil, uint64(0), errors.Wrap(ErrInconsistentHeight, "state factory epoch number became larger than target epoch number")
	}
	if beforeEaster {
		return candidates, stateHeight, nil
	}
	// After Easter height, probation unqualified delegates based on productivity
	unqualifiedList, _, err := sh.GetProbationList(ctx, sr, readFromNext)
	if err != nil {
		return nil, uint64(0), errors.Wrapf(err, "failed to get probation list at height %d", targetEpochStartHeight)
	}
	// recalculate the voting power for probationlist delegates
	filteredCandidate, err := filterCandidates(candidates, unqualifiedList, targetEpochStartHeight)
	if err != nil {
		return nil, uint64(0), err
	}
	return filteredCandidate, stateHeight, nil
}

// GetBlockProducers returns BP list
func (sh *Slasher) GetBlockProducers(ctx context.Context, sr protocol.StateReader, readFromNext bool) (state.CandidateList, uint64, error) {
	candidates, height, err := sh.GetCandidates(ctx, sr, readFromNext)
	if err != nil {
		return nil, uint64(0), err
	}
	bp, err := sh.calculateBlockProducer(candidates)
	if err != nil {
		return nil, uint64(0), err
	}
	return bp, height, nil
}

// GetActiveBlockProducers returns active BP list
func (sh *Slasher) GetActiveBlockProducers(ctx context.Context, sr protocol.StateReader, readFromNext bool) (state.CandidateList, uint64, error) {
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	targetHeight, err := sr.Height()
	if err != nil {
		return nil, uint64(0), err
	}
	// make sure it's epochStartHeight
	targetEpochStartHeight := rp.GetEpochHeight(rp.GetEpochNum(targetHeight))
	if readFromNext {
		targetEpochNum := rp.GetEpochNum(targetEpochStartHeight) + 1
		targetEpochStartHeight = rp.GetEpochHeight(targetEpochNum) // next epoch start height
	}
	blockProducers, height, err := sh.GetBlockProducers(ctx, sr, readFromNext)
	if err != nil {
		return nil, uint64(0), errors.Wrapf(err, "failed to read block producers at height %d", targetEpochStartHeight)
	}
	abp, err := sh.calculateActiveBlockProducer(ctx, blockProducers, targetEpochStartHeight)
	if err != nil {
		return nil, uint64(0), err
	}
	return abp, height, nil
}

// GetCandidatesFromIndexer returns candidate list from indexer
func (sh *Slasher) GetCandidatesFromIndexer(ctx context.Context, epochStartHeight uint64) (state.CandidateList, error) {
	featureWithHeightCtx := protocol.MustGetFeatureWithHeightCtx(ctx)
	candidates, err := sh.indexer.CandidateList(epochStartHeight)
	if err != nil {
		return nil, err
	}
	if !featureWithHeightCtx.CalculateProbationList(epochStartHeight) {
		return candidates, nil
	}
	// After Easter height, probation unqualified delegates based on productivity
	probationList, err := sh.indexer.ProbationList(epochStartHeight)
	if err != nil {
		return nil, err
	}
	// recalculate the voting power for probationlist delegates
	return filterCandidates(candidates, probationList, epochStartHeight)
}

// GetBPFromIndexer returns BP list from indexer
func (sh *Slasher) GetBPFromIndexer(ctx context.Context, epochStartHeight uint64) (state.CandidateList, error) {
	candidates, err := sh.GetCandidatesFromIndexer(ctx, epochStartHeight)
	if err != nil {
		return nil, err
	}
	return sh.calculateBlockProducer(candidates)
}

// GetABPFromIndexer returns active BP list from indexer
func (sh *Slasher) GetABPFromIndexer(ctx context.Context, epochStartHeight uint64) (state.CandidateList, error) {
	blockProducers, err := sh.GetBPFromIndexer(ctx, epochStartHeight)
	if err != nil {
		return nil, err
	}
	return sh.calculateActiveBlockProducer(ctx, blockProducers, epochStartHeight)
}

// GetProbationList returns the probation list at given epoch
func (sh *Slasher) GetProbationList(ctx context.Context, sr protocol.StateReader, readFromNext bool) (*vote.ProbationList, uint64, error) {
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	featureWithHeightCtx := protocol.MustGetFeatureWithHeightCtx(ctx)
	targetHeight, err := sr.Height()
	if err != nil {
		return nil, uint64(0), err
	}
	// make sure it's epochStartHeight
	targetEpochStartHeight := rp.GetEpochHeight(rp.GetEpochHeight(targetHeight))
	if readFromNext {
		targetEpochNum := rp.GetEpochNum(targetEpochStartHeight) + 1
		targetEpochStartHeight = rp.GetEpochHeight(targetEpochNum) // next epoch start height
	}
	if !featureWithHeightCtx.CalculateProbationList(targetEpochStartHeight) {
		return nil, uint64(0), errors.New("Before Easter, there is no probation list in stateDB")
	}
	unqualifiedList, stateHeight, err := sh.getProbationList(sr, readFromNext)
	if err != nil {
		return nil, uint64(0), err
	}
	// to catch the corner case that since the new block is committed, shift occurs in the middle of processing the request
	if rp.GetEpochNum(targetEpochStartHeight) < rp.GetEpochNum(stateHeight) {
		return nil, uint64(0), errors.Wrap(ErrInconsistentHeight, "state factory tip epoch number became larger than target epoch number")
	}
	return unqualifiedList, stateHeight, nil
}

// CalculateProbationList calculates probation list according to productivity
func (sh *Slasher) CalculateProbationList(
	ctx context.Context,
	sm protocol.StateManager,
	epochNum uint64,
) (*vote.ProbationList, error) {
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	g := genesis.MustExtractGenesisContext(ctx)
	easterEpochNum := rp.GetEpochNum(g.EasterBlockHeight)

	nextProbationlist := &vote.ProbationList{
		IntensityRate: sh.probationIntensity,
	}
	upd, err := sh.getUnprodDelegate(sm)
	if err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			if upd, err = vote.NewUnproductiveDelegate(sh.probationEpochPeriod, sh.maxProbationPeriod); err != nil {
				return nil, errors.Wrap(err, "failed to make new upd")
			}
		} else {
			return nil, errors.Wrapf(err, "failed to read upd struct from state DB at epoch number %d", epochNum)
		}
	}
	unqualifiedDelegates := make(map[string]uint32)
	if epochNum <= easterEpochNum+sh.probationEpochPeriod {
		// if epoch number is smaller than easterEpochNum+K(probation period), calculate it one-by-one (initialize).
		log.L().Debug("Before using probation list",
			zap.Uint64("epochNum", epochNum),
			zap.Uint64("easterEpochNum", easterEpochNum),
			zap.Uint64("probationEpochPeriod", sh.probationEpochPeriod),
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
		uq, err := sh.calculateUnproductiveDelegates(ctx, sm)
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
		nextProbationlist.ProbationInfo = unqualifiedDelegates
		return nextProbationlist, setUnproductiveDelegates(sm, upd)
	}
	// ProbationList[N] = ProbationList[N-1] - Low-productivity-list[N-K-1] + Low-productivity-list[N-1]
	log.L().Debug("Using probationList",
		zap.Uint64("epochNum", epochNum),
		zap.Uint64("easterEpochNum", easterEpochNum),
		zap.Uint64("probationEpochPeriod", sh.probationEpochPeriod),
	)
	prevProbationlist, _, err := sh.getProbationList(sm, false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read latest probation list")
	}
	probationMap := prevProbationlist.ProbationInfo
	if probationMap == nil {
		probationMap = make(map[string]uint32)
	}
	skipList := upd.ReadOldestUPD()
	for _, addr := range skipList {
		if _, ok := probationMap[addr]; !ok {
			log.L().Fatal("skipping list element doesn't exist among one of existing map")
			continue
		}
		probationMap[addr]--
	}
	addList, err := sh.calculateUnproductiveDelegates(ctx, sm)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to calculate current epoch upd %d", epochNum-1)
	}
	if err := upd.AddRecentUPD(addList); err != nil {
		return nil, errors.Wrap(err, "failed to add recent upd")
	}
	for _, addr := range addList {
		if _, ok := probationMap[addr]; ok {
			probationMap[addr]++
			continue
		}
		probationMap[addr] = 1
	}

	for addr, count := range probationMap {
		if count == 0 {
			delete(probationMap, addr)
		}
	}
	nextProbationlist.ProbationInfo = probationMap
	return nextProbationlist, setUnproductiveDelegates(sm, upd)
}

func (sh *Slasher) calculateUnproductiveDelegates(ctx context.Context, sr protocol.StateReader) ([]string, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	epochNum := rp.GetEpochNum(blkCtx.BlockHeight)
	delegates, _, err := sh.GetActiveBlockProducers(ctx, sr, false)
	if err != nil {
		return nil, err
	}
	productivityFunc := sh.productivity
	if featureCtx.CurrentEpochProductivity {
		productivityFunc = func(start, end uint64) (map[string]uint64, error) {
			return currentEpochProductivity(sr, start, end, sh.numOfBlocksByEpoch)
		}
	}
	numBlks, produce, err := rp.ProductivityByEpoch(
		epochNum,
		bcCtx.Tip.Height,
		productivityFunc,
	)
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
		if actualNumBlks*100/expectedNumBlks < sh.prodThreshold {
			unqualified = append(unqualified, addr)
		}
	}
	return unqualified, nil
}

func (sh *Slasher) updateCurrentBlockMeta(ctx context.Context, sm protocol.StateManager) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	currentBlockMeta := NewBlockMeta(blkCtx.BlockHeight, blkCtx.Producer.String(), blkCtx.BlockTimeStamp)
	return setCurrentBlockMeta(sm, currentBlockMeta, blkCtx.BlockHeight, sh.numOfBlocksByEpoch)
}

// calculateBlockProducer calculates block producer by given candidate list
func (sh *Slasher) calculateBlockProducer(candidates state.CandidateList) (state.CandidateList, error) {
	var blockProducers state.CandidateList
	for i, candidate := range candidates {
		if uint64(i) >= sh.numCandidateDelegates {
			break
		}
		if candidate.Votes.Cmp(big.NewInt(0)) == 0 {
			// if the voting power is 0, exclude from being a block producer(hard probation)
			continue
		}
		blockProducers = append(blockProducers, candidate)
	}
	return blockProducers, nil
}

// calculateActiveBlockProducer calculates active block producer by given block producer list
func (sh *Slasher) calculateActiveBlockProducer(
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

	length := int(sh.numDelegates)
	if len(blockProducerList) < length {
		// TODO: if the number of delegates is smaller than expected, should it return error or not?
		length = len(blockProducerList)
		log.L().Warn(
			"the number of block producer is less than expected",
			zap.Int("actual block producer", len(blockProducerList)),
			zap.Uint64("expected", sh.numDelegates),
		)
	}
	var activeBlockProducers state.CandidateList
	for i := 0; i < length; i++ {
		activeBlockProducers = append(activeBlockProducers, blockProducerMap[blockProducerList[i]])
	}
	return activeBlockProducers, nil
}

// filterCandidates returns filtered candidate list by given raw candidate/ probation list
func filterCandidates(
	candidates state.CandidateList,
	unqualifiedList *vote.ProbationList,
	epochStartHeight uint64,
) (state.CandidateList, error) {
	candidatesMap := make(map[string]*state.Candidate)
	updatedVotingPower := make(map[string]*big.Int)
	intensityRate := float64(uint32(100)-unqualifiedList.IntensityRate) / float64(100)
	for _, cand := range candidates {
		filterCand := cand.Clone()
		if _, ok := unqualifiedList.ProbationInfo[cand.Address]; ok {
			// if it is an unqualified delegate, multiply the voting power with probation intensity rate
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

// currentEpochProductivity returns the map of the number of blocks produced per delegate of current epoch
func currentEpochProductivity(sr protocol.StateReader, start uint64, end uint64, numOfBlocksByEpoch uint64) (map[string]uint64, error) {
	log.L().Debug("Read current epoch productivity",
		zap.Uint64("start height", start),
		zap.Uint64("end height", end),
	)
	stats := make(map[string]uint64)
	blockmetas, err := allBlockMetasFromDB(sr, numOfBlocksByEpoch)
	if err != nil {
		return nil, err
	}
	expectedCount := end - start + 1
	count := uint64(0)
	for _, blockmeta := range blockmetas {
		if blockmeta.Height < start || blockmeta.Height > end {
			continue
		}
		stats[blockmeta.Producer]++
		count++
	}
	if expectedCount != count {
		log.L().Debug(
			"block metas from stateDB count is not same as expected",
			zap.Uint64("expected", expectedCount),
			zap.Uint64("actual", count),
		)
		return nil, errors.New("block metas from stateDB doesn't have enough data for given start, end height")
	}
	return stats, nil
}
