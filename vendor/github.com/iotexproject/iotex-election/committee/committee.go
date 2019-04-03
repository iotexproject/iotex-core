// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package committee

import (
	"context"
	"math"
	"math/big"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-election/carrier"
	"github.com/iotexproject/iotex-election/db"
	"github.com/iotexproject/iotex-election/types"
	"github.com/iotexproject/iotex-election/util"
)

// Namespace to store the result in db
const Namespace = "electionNS"

// CalcGravityChainHeight calculates the corresponding gravity chain height for an epoch
type CalcGravityChainHeight func(uint64) (uint64, error)

// Config defines the config of the committee
type Config struct {
	NumOfRetries               uint8    `yaml:"numOfRetries"`
	GravityChainAPIs           []string `yaml:"gravityChainAPIs"`
	GravityChainHeightInterval uint64   `yaml:"gravityChainHeightInterval"`
	GravityChainStartHeight    uint64   `yaml:"gravityChainStartHeight"`
	RegisterContractAddress    string   `yaml:"registerContractAddress"`
	StakingContractAddress     string   `yaml:"stakingContractAddress"`
	PaginationSize             uint8    `yaml:"paginationSize"`
	VoteThreshold              string   `yaml:"voteThreshold"`
	ScoreThreshold             string   `yaml:"scoreThreshold"`
	SelfStakingThreshold       string   `yaml:"selfStakingThreshold"`
	CacheSize                  uint32   `yaml:"cacheSize"`
	NumOfFetchInParallel       uint8    `yaml:"numOfFetchInParallel"`
}

// STATUS represents the status of committee
type STATUS uint8

const (
	// STARTING stands for a starting status
	STARTING STATUS = iota
	// ACTIVE stands for an active status
	ACTIVE
	// INACTIVE stands for an inactive status
	INACTIVE
)

// Committee defines an interface of an election committee
// It could be considered as a light state db of gravity chain, that
type Committee interface {
	// Start starts the committee service
	Start(context.Context) error
	// Stop stops the committee service
	Stop(context.Context) error
	// ResultByHeight returns the result on a specific ethereum height
	ResultByHeight(height uint64) (*types.ElectionResult, error)
	// FetchResultByHeight returns the votes
	FetchResultByHeight(height uint64) (*types.ElectionResult, error)
	// HeightByTime returns the nearest result before time
	HeightByTime(timestamp time.Time) (uint64, error)
	// LatestHeight returns the height with latest result
	LatestHeight() uint64
	// Status returns the committee status
	Status() STATUS
}

type committee struct {
	db                   db.KVStore
	carrier              carrier.Carrier
	retryLimit           uint8
	paginationSize       uint8
	fetchInParallel      uint8
	voteThreshold        *big.Int
	scoreThreshold       *big.Int
	selfStakingThreshold *big.Int
	interval             uint64

	cache         *resultCache
	heightManager *heightManager

	startHeight         uint64
	nextHeight          uint64
	currentHeight       uint64
	lastUpdateTimestamp int64
	terminate           chan bool
	mutex               sync.RWMutex
}

// NewCommitteeWithKVStoreWithNamespace creates a committee with kvstore with namespace
func NewCommitteeWithKVStoreWithNamespace(kvstore db.KVStoreWithNamespace, cfg Config) (Committee, error) {
	return NewCommittee(db.NewKVStoreWithNamespaceWrapper(Namespace, kvstore), cfg)
}

// NewCommittee creates a committee
func NewCommittee(kvstore db.KVStore, cfg Config) (Committee, error) {
	if !common.IsHexAddress(cfg.StakingContractAddress) {
		return nil, errors.New("Invalid staking contract address")
	}
	carrier, err := carrier.NewEthereumVoteCarrier(
		cfg.GravityChainAPIs,
		common.HexToAddress(cfg.RegisterContractAddress),
		common.HexToAddress(cfg.StakingContractAddress),
	)
	zap.L().Info(
		"Carrier created",
		zap.String("registerContractAddress", cfg.RegisterContractAddress),
		zap.String("stakingContractAddress", cfg.StakingContractAddress),
	)
	if err != nil {
		return nil, err
	}
	voteThreshold, ok := new(big.Int).SetString(cfg.VoteThreshold, 10)
	if !ok {
		return nil, errors.New("Invalid vote threshold")
	}
	scoreThreshold, ok := new(big.Int).SetString(cfg.ScoreThreshold, 10)
	if !ok {
		return nil, errors.New("Invalid score threshold")
	}
	selfStakingThreshold, ok := new(big.Int).SetString(cfg.SelfStakingThreshold, 10)
	if !ok {
		return nil, errors.New("Invalid self staking threshold")
	}
	fetchInParallel := uint8(10)
	if cfg.NumOfFetchInParallel > 0 {
		fetchInParallel = cfg.NumOfFetchInParallel
	}
	return &committee{
		db:                   kvstore,
		cache:                newResultCache(cfg.CacheSize),
		heightManager:        newHeightManager(),
		carrier:              carrier,
		retryLimit:           cfg.NumOfRetries,
		paginationSize:       cfg.PaginationSize,
		fetchInParallel:      fetchInParallel,
		voteThreshold:        voteThreshold,
		scoreThreshold:       scoreThreshold,
		selfStakingThreshold: selfStakingThreshold,
		terminate:            make(chan bool),
		startHeight:          cfg.GravityChainStartHeight,
		interval:             cfg.GravityChainHeightInterval,
		currentHeight:        0,
		nextHeight:           cfg.GravityChainStartHeight,
	}, nil
}

func (ec *committee) Start(ctx context.Context) (err error) {
	ec.mutex.Lock()
	defer ec.mutex.Unlock()
	if err := ec.db.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting db")
	}
	if startHeight, err := ec.db.Get(db.NextHeightKey); err == nil {
		zap.L().Info("restoring from db")
		ec.nextHeight = util.BytesToUint64(startHeight)
		for height := ec.startHeight; height < ec.nextHeight; height += ec.interval {
			zap.L().Info("loading", zap.Uint64("height", height))
			data, err := ec.db.Get(ec.dbKey(height))
			if err != nil {
				return err
			}
			r := &types.ElectionResult{}
			if err := r.Deserialize(data); err != nil {
				return err
			}
			ec.cache.insert(height, r)
			if err := ec.heightManager.add(height, r.MintTime()); err != nil {
				return err
			}
		}
	}
	zap.L().Info("catching up via network")
	tipHeight, err := ec.carrier.TipHeight()
	if err != nil {
		return errors.Wrap(err, "failed to get tip height")
	}
	if err := ec.sync(tipHeight); err != nil {
		return errors.Wrap(err, "failed to catch up via network")
	}
	zap.L().Info("subscribing to new block")
	heightChan := make(chan uint64)
	reportChan := make(chan error)
	go func() {
		for {
			select {
			case <-ec.terminate:
				ec.terminate <- true
				return
			case height := <-heightChan:
				if err := ec.Sync(height); err != nil {
					zap.L().Error("failed to sync", zap.Error(err))
				}
			case err := <-reportChan:
				zap.L().Error("something goes wrong", zap.Error(err))
			}
		}
	}()
	ec.carrier.SubscribeNewBlock(heightChan, reportChan, ec.terminate)
	return nil
}

func (ec *committee) Stop(ctx context.Context) error {
	ec.mutex.Lock()
	defer ec.mutex.Unlock()
	ec.terminate <- true
	ec.carrier.Close()

	return ec.db.Stop(ctx)
}

func (ec *committee) Status() STATUS {
	lastUpdateTimestamp := atomic.LoadInt64(&ec.lastUpdateTimestamp)
	switch {
	case lastUpdateTimestamp == 0:
		return STARTING
	case lastUpdateTimestamp > time.Now().Add(-60*time.Second).Unix():
		return ACTIVE
	default:
		return INACTIVE
	}
}

func (ec *committee) Sync(tipHeight uint64) error {
	ec.mutex.Lock()
	defer ec.mutex.Unlock()

	return ec.sync(tipHeight)
}

func (ec *committee) sync(tipHeight uint64) error {
	zap.L().Info("new ethereum block", zap.Uint64("height", tipHeight))
	if ec.currentHeight < tipHeight {
		ec.currentHeight = tipHeight
	}
	var wg sync.WaitGroup
	limiter := make(chan bool, ec.fetchInParallel)
	results := map[uint64]*types.ElectionResult{}
	errs := map[uint64]error{}
	for nextHeight := ec.nextHeight; nextHeight < ec.currentHeight; nextHeight += ec.interval {
		if nextHeight > ec.currentHeight-12 {
			break
		}
		wg.Add(1)
		go func(height uint64) {
			defer func() {
				<-limiter
				wg.Done()
			}()
			limiter <- true
			results[height], errs[height] = ec.retryFetchResultByHeight(height)
		}(nextHeight)
	}
	wg.Wait()
	var heights []uint64
	for height := range results {
		heights = append(heights, height)
	}
	sort.Slice(heights, func(i, j int) bool {
		return heights[i] < heights[j]
	})
	for _, height := range heights {
		result := results[height]
		if err := errs[height]; err != nil {
			return err
		}
		if err := ec.heightManager.validate(height, result.MintTime()); err != nil {
			zap.L().Fatal(
				"Unexpected status that the upcoming block height or time is invalid",
				zap.Error(err),
			)
		}
		if err := ec.storeResult(height, result); err != nil {
			return errors.Wrapf(err, "failed to store result of height %d", height)
		}
		ec.nextHeight = height + ec.interval
	}
	atomic.StoreInt64(&ec.lastUpdateTimestamp, time.Now().Unix())

	return nil
}

func (ec *committee) LatestHeight() uint64 {
	ec.mutex.RLock()
	defer ec.mutex.RUnlock()
	return ec.heightManager.lastestHeight()
}

func (ec *committee) HeightByTime(ts time.Time) (uint64, error) {
	ec.mutex.RLock()
	defer ec.mutex.RUnlock()
	height := ec.heightManager.nearestHeightBefore(ts)
	if height == 0 {
		return 0, db.ErrNotExist
	}

	return height, nil
}

func (ec *committee) ResultByHeight(height uint64) (*types.ElectionResult, error) {
	ec.mutex.RLock()
	defer ec.mutex.RUnlock()
	return ec.resultByHeight(height)
}

func (ec *committee) resultByHeight(height uint64) (*types.ElectionResult, error) {
	if height < ec.startHeight {
		return nil, errors.Errorf(
			"height %d is lower than start height %d",
			height,
			ec.startHeight,
		)
	}
	if (height-ec.startHeight)%ec.interval != 0 {
		return nil, errors.Errorf(
			"height %d is an invalid height",
			height,
		)
	}
	result := ec.cache.get(height)
	if result != nil {
		return result, nil
	}
	data, err := ec.db.Get(ec.dbKey(height))
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, db.ErrNotExist
	}
	result = &types.ElectionResult{}

	return result, result.Deserialize(data)
}

func (ec *committee) calcWeightedVotes(v *types.Vote, now time.Time) *big.Int {
	if now.Before(v.StartTime()) {
		return big.NewInt(0)
	}
	remainingTime := v.RemainingTime(now).Seconds()
	weight := float64(1)
	if remainingTime > 0 {
		weight += math.Log(math.Ceil(remainingTime/86400)) / math.Log(1.2) / 100
	}
	amount := new(big.Float).SetInt(v.Amount())
	weightedAmount, _ := amount.Mul(amount, big.NewFloat(weight)).Int(nil)

	return weightedAmount
}

func (ec *committee) fetchVotesByHeight(height uint64) ([]*types.Vote, error) {
	var allVotes []*types.Vote
	previousIndex := big.NewInt(0)
	for {
		var votes []*types.Vote
		var err error
		if previousIndex, votes, err = ec.carrier.Votes(
			height,
			previousIndex,
			ec.paginationSize,
		); err != nil {
			return nil, err
		}
		allVotes = append(allVotes, votes...)
		if len(votes) < int(ec.paginationSize) {
			break
		}
	}

	return allVotes, nil
}
func (ec *committee) voteFilter(v *types.Vote) bool {
	return ec.voteThreshold.Cmp(v.Amount()) > 0
}
func (ec *committee) candidateFilter(c *types.Candidate) bool {
	return ec.selfStakingThreshold.Cmp(c.SelfStakingTokens()) > 0 ||
		ec.scoreThreshold.Cmp(c.Score()) > 0
}
func (ec *committee) calculator(height uint64) (*types.ResultCalculator, error) {
	mintTime, err := ec.carrier.BlockTimestamp(height)
	switch errors.Cause(err) {
	case nil:
		break
	case ethereum.NotFound:
		return nil, db.ErrNotExist
	default:
		return nil, err
	}
	return types.NewResultCalculator(
		mintTime,
		ec.voteFilter,
		ec.calcWeightedVotes,
		ec.candidateFilter,
	), nil
}

func (ec *committee) fetchCandidatesByHeight(height uint64) ([]*types.Candidate, error) {
	var allCandidates []*types.Candidate
	previousIndex := big.NewInt(1)
	for {
		var candidates []*types.Candidate
		var err error
		if previousIndex, candidates, err = ec.carrier.Candidates(
			height,
			previousIndex,
			ec.paginationSize,
		); err != nil {
			return nil, err
		}
		allCandidates = append(allCandidates, candidates...)
		if len(candidates) < int(ec.paginationSize) {
			break
		}
	}
	return allCandidates, nil
}

func (ec *committee) FetchResultByHeight(height uint64) (*types.ElectionResult, error) {
	if height == 0 {
		var err error
		height, err = ec.carrier.TipHeight()
		if err != nil {
			return nil, err
		}
	}
	return ec.fetchResultByHeight(height)
}

func (ec *committee) fetchResultByHeight(height uint64) (*types.ElectionResult, error) {
	zap.L().Info("fetch result from ethereum", zap.Uint64("height", height))
	calculator, err := ec.calculator(height)
	if err != nil {
		return nil, err
	}
	candidates, err := ec.fetchCandidatesByHeight(height)
	if err != nil {
		return nil, err
	}
	if err := calculator.AddCandidates(candidates); err != nil {
		return nil, err
	}
	votes, err := ec.fetchVotesByHeight(height)
	if err != nil {
		return nil, err
	}
	if err := calculator.AddVotes(votes); err != nil {
		return nil, err
	}

	return calculator.Calculate()
}

func (ec *committee) dbKey(height uint64) []byte {
	return util.Uint64ToBytes(height)
}

func (ec *committee) storeResult(height uint64, result *types.ElectionResult) error {
	data, err := result.Serialize()
	if err != nil {
		return err
	}
	if err := ec.db.Put(ec.dbKey(height), data); err != nil {
		return errors.Wrapf(err, "failed to put election result into db")
	}
	if err := ec.db.Put(db.NextHeightKey, ec.dbKey(height+ec.interval)); err != nil {
		return err
	}
	ec.cache.insert(height, result)

	return ec.heightManager.add(height, result.MintTime())
}

func (ec *committee) retryFetchResultByHeight(height uint64) (*types.ElectionResult, error) {
	var result *types.ElectionResult
	var err error
	for i := uint8(0); i < ec.retryLimit; i++ {
		if result, err = ec.fetchResultByHeight(height); err == nil {
			return result, nil
		}
		zap.L().Error(
			"failed to fetch result by height",
			zap.Error(err),
			zap.Uint64("height", height),
			zap.Uint8("tried", i+1),
		)
	}
	return result, err
}
