// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package genesis

import (
	"math/big"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/config"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
)

const (
	// TODO: define features name here instead of hardfork
	FEATURE_PACIFIC = "pacific"
)

// Default contains the default genesis config
var Default = defaultConfig()

var (
	genesisTs     int64
	loadGenesisTs sync.Once
)

func init() {
	initTestDefaultConfig()
}

func defaultConfig() Genesis {
	return Genesis{
		Blockchain: Blockchain{
			Timestamp:               1546329600,
			BlockGasLimit:           20000000,
			ActionGasLimit:          5000000,
			BlockInterval:           10 * time.Second,
			NumSubEpochs:            2,
			DardanellesNumSubEpochs: 30,
			NumDelegates:            24,
			NumCandidateDelegates:   36,
			TimeBasedRotation:       false,
			PacificBlockHeight:      432001,
			AleutianBlockHeight:     864001,
			BeringBlockHeight:       1512001,
			CookBlockHeight:         1641601,
			DardanellesBlockHeight:  1816201,
			DaytonaBlockHeight:      3238921,
			EasterBlockHeight:       4478761,
			FbkMigrationBlockHeight: 5157001,
			FairbankBlockHeight:     5165641,
			GreenlandBlockHeight:    6544441,
			HawaiiBlockHeight:       11267641,
			IcelandBlockHeight:      12289321,
			JutlandBlockHeight:      16289321,

			Pacific: HardFork{
				Height: 432001,
				EnableFeatures: []string{
					FEATURE_PACIFIC,
				},
			},
			Aleutian: HardFork{
				Height: 864001,
				EnableFeatures: []string{},
			},
			Bering: HardFork{
				Height: 1512001,
				EnableFeatures: []string{},
			},
			Cook: HardFork{
				Height: 1641601,
				EnableFeatures: []string{},
			},
			Dardanelles: HardFork{
				Height: 1816201,
				EnableFeatures: []string{},
			},
			Daytona: HardFork{
				Height: 3238921,
				EnableFeatures: []string{},
			},
			Easter: HardFork{
				Height: 4478761,
				EnableFeatures: []string{},
			},
			FbkMigration: HardFork{
				Height: 5157001,
				EnableFeatures: []string{},
			},
			Fairbank: HardFork{
				Height: 5165641,
				EnableFeatures: []string{},
			},
			Greenland: HardFork{
				Height: 6544441,
				EnableFeatures: []string{},
			},
			Hawaii: HardFork{
				Height: 11267641,
				EnableFeatures: []string{},
			},
			Iceland: HardFork{
				Height: 12289321,
				EnableFeatures: []string{},
			},
			Jutland: HardFork{
				Height: 16289321,
				EnableFeatures: []string{},
			},
		},
		Account: Account{
			InitBalanceMap: make(map[string]string),
		},
		Poll: Poll{
			PollMode:                         "nativeMix",
			EnableGravityChainVoting:         true,
			GravityChainCeilingHeight:        10199000,
			ProbationEpochPeriod:             6,
			ProbationIntensityRate:           90,
			UnproductiveDelegateMaxCacheSize: 20,
		},
		Rewarding: Rewarding{
			InitBalanceStr:                 unit.ConvertIotxToRau(200000000).String(),
			BlockRewardStr:                 unit.ConvertIotxToRau(16).String(),
			DardanellesBlockRewardStr:      unit.ConvertIotxToRau(8).String(),
			EpochRewardStr:                 unit.ConvertIotxToRau(12500).String(),
			AleutianEpochRewardStr:         unit.ConvertIotxToRau(18750).String(),
			NumDelegatesForEpochReward:     100,
			ExemptAddrStrsFromEpochReward:  []string{},
			FoundationBonusStr:             unit.ConvertIotxToRau(80).String(),
			NumDelegatesForFoundationBonus: 36,
			FoundationBonusLastEpoch:       8760,
			FoundationBonusP2StartEpoch:    9698,
			FoundationBonusP2EndEpoch:      18458,
		},
		Staking: Staking{
			VoteWeightCalConsts: VoteWeightCalConsts{
				DurationLg: 1.2,
				AutoStake:  1,
				SelfStake:  1.06,
			},
			RegistrationConsts: RegistrationConsts{
				Fee:          unit.ConvertIotxToRau(100).String(),
				MinSelfStake: unit.ConvertIotxToRau(1200000).String(),
			},
			WithdrawWaitingPeriod: 3 * 24 * time.Hour,
			MinStakeAmount:        unit.ConvertIotxToRau(100).String(),
			BootstrapCandidates:   []BootstrapCandidate{},
		},
	}
}

func initTestDefaultConfig() {
	Default = defaultConfig()
	Default.PacificBlockHeight = 0
	for i := 0; i < identityset.Size(); i++ {
		addr := identityset.Address(i).String()
		value := unit.ConvertIotxToRau(100000000).String()
		Default.InitBalanceMap[addr] = value
		if uint64(i) < Default.NumDelegates {
			Default.Delegates = append(Default.Delegates, Delegate{
				OperatorAddrStr: addr,
				RewardAddrStr:   addr,
				VotesStr:        value,
			})
		}
	}
}

type (
	// Genesis is the root level of genesis config. Genesis config is the network-wide blockchain config. All the nodes
	// participating into the same network should use EXACTLY SAME genesis config.
	Genesis struct {
		Blockchain `yaml:"blockchain"`
		Account    `yaml:"account"`
		Poll       `yaml:"poll"`
		Rewarding  `yaml:"rewarding"`
		Staking    `yaml:"staking"`
	}
	HardFork struct {
		Height uint64 `yaml:"height"`
		EnableFeatures []string `yaml:"enableFeatures"`
	}
	// Blockchain contains blockchain level configs
	Blockchain struct {
		// Timestamp is the timestamp of the genesis block
		Timestamp int64
		// BlockGasLimit is the total gas limit could be consumed in a block
		BlockGasLimit uint64 `yaml:"blockGasLimit"`
		// ActionGasLimit is the per action gas limit cap
		ActionGasLimit uint64 `yaml:"actionGasLimit"`
		// BlockInterval is the interval between two blocks
		BlockInterval time.Duration `yaml:"blockInterval"`
		// NumSubEpochs is the number of sub epochs in one epoch of block production
		NumSubEpochs uint64 `yaml:"numSubEpochs"`
		// DardanellesNumSubEpochs is the number of sub epochs starts from dardanelles height in one epoch of block production
		DardanellesNumSubEpochs uint64 `yaml:"dardanellesNumSubEpochs"`
		// NumDelegates is the number of delegates that participate into one epoch of block production
		NumDelegates uint64 `yaml:"numDelegates"`
		// NumCandidateDelegates is the number of candidate delegates, who may be selected as a delegate via roll dpos
		NumCandidateDelegates uint64 `yaml:"numCandidateDelegates"`
		// TimeBasedRotation is the flag to enable rotating delegates' time slots on a block height
		TimeBasedRotation bool `yaml:"timeBasedRotation"`
		// PacificBlockHeight is the start height of using the logic of Pacific version
		// TODO: PacificBlockHeight is not added into protobuf definition for backward compatibility
		PacificBlockHeight uint64 `yaml:"pacificHeight"`
		// AleutianBlockHeight is the start height of adding bloom filter of all events into block header
		AleutianBlockHeight uint64 `yaml:"aleutianHeight"`
		// BeringBlockHeight is the start height of evm upgrade
		BeringBlockHeight uint64 `yaml:"beringHeight"`
		// CookBlockHeight is the start height of native staking
		CookBlockHeight uint64 `yaml:"cookHeight"`
		// DardanellesBlockHeight is the start height of 5s block internal
		DardanellesBlockHeight uint64 `yaml:"dardanellesHeight"`
		// DaytonaBlockHeight is the height to fix low gas for read native staking contract
		DaytonaBlockHeight uint64 `yaml:"daytonaBlockHeight"`
		// EasterBlockHeight is the start height of probation for slashing
		EasterBlockHeight uint64 `yaml:"easterHeight"`
		// FbkMigrationBlockHeight is the start height for fairbank migration
		FbkMigrationBlockHeight uint64 `yaml:"fbkMigrationHeight"`
		// FairbankBlockHeight is the start height to switch to native staking V2
		FairbankBlockHeight uint64 `yaml:"fairbankHeight"`
		// GreenlandBlockHeight is the start height of storing latest 720 block meta and rewarding/staking bucket pool
		GreenlandBlockHeight uint64 `yaml:"greenlandHeight"`
		// HawaiiBlockHeight is the start height to
		// 1. fix GetBlockHash in EVM
		// 2. add revert message to log
		// 3. fix change to same candidate in staking protocol
		// 4. fix sorted map in StateDBAdapter
		// 5. use pending nonce in EVM
		HawaiiBlockHeight uint64 `yaml:"hawaiiHeight"`
		// IcelandBlockHeight is the start height to support chainID opcode in EVM
		IcelandBlockHeight uint64 `yaml:"icelandHeight"`
		// JutlandBlockHeight is the start height to cover all EVM error codes
		JutlandBlockHeight uint64 `yaml:"jutlandHeight"`

		Pacific HardFork `yaml:"pacific"`
		Aleutian HardFork `yaml:"aleutian"`
		Bering HardFork `yaml:"bering"`
		Cook HardFork `yaml:"cook"`
		Dardanelles HardFork `yaml:"dardanelles"`
		Daytona HardFork `yaml:"daytona"`
		Easter HardFork `yaml:"easter"`
		FbkMigration HardFork `yaml:"fbkMigration"`
		Fairbank HardFork `yaml:"fairbank"`
		Greenland HardFork `yaml:"greenland"`
		Hawaii HardFork `yaml:"hawaii"`
		Iceland HardFork `yaml:"iceland"`
		Jutland HardFork `yaml:"jutland"`

		FeatureHeightMap map[string]uint64 `yaml:"featureHeights"`
	}
	// Account contains the configs for account protocol
	Account struct {
		// InitBalanceMap is the address and initial balance mapping before the first block.
		InitBalanceMap map[string]string `yaml:"initBalances"`
	}
	// Poll contains the configs for poll protocol
	Poll struct {
		// PollMode is different based on chain type or poll input data source
		PollMode string `yaml:"pollMode"`
		// EnableGravityChainVoting is a flag whether read voting from gravity chain
		EnableGravityChainVoting bool `yaml:"enableGravityChainVoting"`
		// GravityChainStartHeight is the height in gravity chain where the init poll result stored
		GravityChainStartHeight uint64 `yaml:"gravityChainStartHeight"`
		// GravityChainCeilingHeight is the height in gravity chain where the poll is no longer needed
		GravityChainCeilingHeight uint64 `yaml:"gravityChainCeilingHeight"`
		// GravityChainHeightInterval the height interval on gravity chain to pull delegate information
		GravityChainHeightInterval uint64 `yaml:"gravityChainHeightInterval"`
		// RegisterContractAddress is the address of register contract
		RegisterContractAddress string `yaml:"registerContractAddress"`
		// StakingContractAddress is the address of staking contract
		StakingContractAddress string `yaml:"stakingContractAddress"`
		// NativeStakingContractAddress is the address of native staking contract
		NativeStakingContractAddress string `yaml:"nativeStakingContractAddress"`
		// NativeStakingContractCode is the code of native staking contract
		NativeStakingContractCode string `yaml:"nativeStakingContractCode"`
		// ConsortiumCommitteeCode is the code of consortiumCommittee contract
		ConsortiumCommitteeContractCode string `yaml:"consortiumCommitteeContractCode"`
		// VoteThreshold is the vote threshold amount in decimal string format
		VoteThreshold string `yaml:"voteThreshold"`
		// ScoreThreshold is the score threshold amount in decimal string format
		ScoreThreshold string `yaml:"scoreThreshold"`
		// SelfStakingThreshold is self-staking vote threshold amount in decimal string format
		SelfStakingThreshold string `yaml:"selfStakingThreshold"`
		// Delegates is a list of delegates with votes
		Delegates []Delegate `yaml:"delegates"`
		// ProbationEpochPeriod is a duration of probation after delegate's productivity is lower than threshold
		ProbationEpochPeriod uint64 `yaml:"probationEpochPeriod"`
		// ProbationIntensityRate is a intensity rate of probation range from [0, 100], where 100 is hard-probation
		ProbationIntensityRate uint32 `yaml:"probationIntensityRate"`
		// UnproductiveDelegateMaxCacheSize is a max cache size of upd which is stored into state DB (probationEpochPeriod <= UnproductiveDelegateMaxCacheSize)
		UnproductiveDelegateMaxCacheSize uint64 `yaml:unproductiveDelegateMaxCacheSize`
	}
	// Delegate defines a delegate with address and votes
	Delegate struct {
		// OperatorAddrStr is the address who will operate the node
		OperatorAddrStr string `yaml:"operatorAddr"`
		// RewardAddrStr is the address who will get the reward when operator produces blocks
		RewardAddrStr string `yaml:"rewardAddr"`
		// VotesStr is the score for the operator to rank and weight for rewardee to split epoch reward
		VotesStr string `yaml:"votes"`
	}
	// Rewarding contains the configs for rewarding protocol
	Rewarding struct {
		// InitBalanceStr is the initial balance of the rewarding protocol in decimal string format
		InitBalanceStr string `yaml:"initBalance"`
		// BlockReward is the block reward amount in decimal string format
		BlockRewardStr string `yaml:"blockReward"`
		// DardanellesBlockReward is the block reward amount starts from dardanelles height in decimal string format
		DardanellesBlockRewardStr string `yaml:"dardanellesBlockReward"`
		// EpochReward is the epoch reward amount in decimal string format
		EpochRewardStr string `yaml:"epochReward"`
		// AleutianEpochRewardStr is the epoch reward amount in decimal string format after aleutian fork
		AleutianEpochRewardStr string `yaml:"aleutianEpochReward"`
		// NumDelegatesForEpochReward is the number of top candidates that will share a epoch reward
		NumDelegatesForEpochReward uint64 `yaml:"numDelegatesForEpochReward"`
		// ExemptAddrStrsFromEpochReward is the list of addresses in encoded string format that exempt from epoch reward
		ExemptAddrStrsFromEpochReward []string `yaml:"exemptAddrsFromEpochReward"`
		// FoundationBonusStr is the bootstrap bonus in decimal string format
		FoundationBonusStr string `yaml:"foundationBonus"`
		// NumDelegatesForFoundationBonus is the number of top candidate that will get the bootstrap bonus
		NumDelegatesForFoundationBonus uint64 `yaml:"numDelegatesForFoundationBonus"`
		// FoundationBonusLastEpoch is the last epoch number that bootstrap bonus will be granted
		FoundationBonusLastEpoch uint64 `yaml:"foundationBonusLastEpoch"`
		// FoundationBonusP2StartEpoch is the start epoch number for part 2 foundation bonus
		FoundationBonusP2StartEpoch uint64 `yaml:"foundationBonusP2StartEpoch"`
		// FoundationBonusP2EndEpoch is the end epoch number for part 2 foundation bonus
		FoundationBonusP2EndEpoch uint64 `yaml:"foundationBonusP2EndEpoch"`
		// ProductivityThreshold is the percentage number that a delegate's productivity needs to reach not to get probation
		ProductivityThreshold uint64 `yaml:"productivityThreshold"`
	}
	// Staking contains the configs for staking protocol
	Staking struct {
		VoteWeightCalConsts   VoteWeightCalConsts  `yaml:"voteWeightCalConsts"`
		RegistrationConsts    RegistrationConsts   `yaml:"registrationConsts"`
		WithdrawWaitingPeriod time.Duration        `yaml:"withdrawWaitingPeriod"`
		MinStakeAmount        string               `yaml:"minStakeAmount"`
		BootstrapCandidates   []BootstrapCandidate `yaml:"bootstrapCandidates"`
	}

	// VoteWeightCalConsts contains the configs for calculating vote weight
	VoteWeightCalConsts struct {
		DurationLg float64 `yaml:"durationLg"`
		AutoStake  float64 `yaml:"autoStake"`
		SelfStake  float64 `yaml:"selfStake"`
	}

	// RegistrationConsts contains the configs for candidate registration
	RegistrationConsts struct {
		Fee          string `yaml:"fee"`
		MinSelfStake string `yaml:"minSelfStake"`
	}

	// BootstrapCandidate is the candidate data need to be provided to bootstrap candidate.
	BootstrapCandidate struct {
		OwnerAddress      string `yaml:"ownerAddress"`
		OperatorAddress   string `yaml:"operatorAddress"`
		RewardAddress     string `yaml:"rewardAddress"`
		Name              string `yaml:"name"`
		SelfStakingTokens string `yaml:"selfStakingTokens"`
	}
)

// New constructs a genesis config. It loads the default values, and could be overwritten by values defined in the yaml
// config files
func New(genesisPath string) (Genesis, error) {
	def := defaultConfig()

	opts := make([]config.YAMLOption, 0)
	opts = append(opts, config.Static(def))
	if genesisPath != "" {
		opts = append(opts, config.File(genesisPath))
	}
	yaml, err := config.NewYAML(opts...)
	if err != nil {
		return Genesis{}, errors.Wrap(err, "error when constructing a genesis in yaml")
	}

	var genesis Genesis
	if err := yaml.Get(config.Root).Populate(&genesis); err != nil {
		return Genesis{}, errors.Wrap(err, "failed to unmarshal yaml genesis to struct")
	}
	genesis.loadFeatures()
	return genesis, nil
}

// SetGenesisTimestamp sets the genesis timestamp
func SetGenesisTimestamp(ts int64) {
	loadGenesisTs.Do(func() {
		genesisTs = ts
	})
}

// Timestamp returns the genesis timestamp
func Timestamp() int64 {
	return atomic.LoadInt64(&genesisTs)
}

// Hash is the hash of genesis config
func (g *Genesis) Hash() hash.Hash256 {
	gbProto := iotextypes.GenesisBlockchain{
		Timestamp:             g.Timestamp,
		BlockGasLimit:         g.BlockGasLimit,
		ActionGasLimit:        g.ActionGasLimit,
		BlockInterval:         g.BlockInterval.Nanoseconds(),
		NumSubEpochs:          g.NumSubEpochs,
		NumDelegates:          g.NumDelegates,
		NumCandidateDelegates: g.NumCandidateDelegates,
		TimeBasedRotation:     g.TimeBasedRotation,
	}

	initBalanceAddrs := make([]string, 0)
	for initBalanceAddr := range g.InitBalanceMap {
		initBalanceAddrs = append(initBalanceAddrs, initBalanceAddr)
	}
	sort.Strings(initBalanceAddrs)
	initBalances := make([]string, 0)
	for _, initBalanceAddr := range initBalanceAddrs {
		initBalances = append(initBalances, g.InitBalanceMap[initBalanceAddr])
	}
	aProto := iotextypes.GenesisAccount{
		InitBalanceAddrs: initBalanceAddrs,
		InitBalances:     initBalances,
	}

	dProtos := make([]*iotextypes.GenesisDelegate, 0)
	for _, d := range g.Delegates {
		dProto := iotextypes.GenesisDelegate{
			OperatorAddr: d.OperatorAddrStr,
			RewardAddr:   d.RewardAddrStr,
			Votes:        d.VotesStr,
		}
		dProtos = append(dProtos, &dProto)
	}
	pProto := iotextypes.GenesisPoll{
		EnableGravityChainVoting: g.EnableGravityChainVoting,
		GravityChainStartHeight:  g.GravityChainStartHeight,
		RegisterContractAddress:  g.RegisterContractAddress,
		StakingContractAddress:   g.StakingContractAddress,
		VoteThreshold:            g.VoteThreshold,
		ScoreThreshold:           g.ScoreThreshold,
		SelfStakingThreshold:     g.SelfStakingThreshold,
		Delegates:                dProtos,
	}

	rProto := iotextypes.GenesisRewarding{
		InitBalance:                    g.InitBalanceStr,
		BlockReward:                    g.BlockRewardStr,
		EpochReward:                    g.EpochRewardStr,
		NumDelegatesForEpochReward:     g.NumDelegatesForEpochReward,
		FoundationBonus:                g.FoundationBonusStr,
		NumDelegatesForFoundationBonus: g.NumDelegatesForFoundationBonus,
		FoundationBonusLastEpoch:       g.FoundationBonusLastEpoch,
		ProductivityThreshold:          g.ProductivityThreshold,
	}

	gProto := iotextypes.Genesis{
		Blockchain: &gbProto,
		Account:    &aProto,
		Poll:       &pProto,
		Rewarding:  &rProto,
	}
	b, err := proto.Marshal(&gProto)
	if err != nil {
		log.L().Panic("Error when marshaling genesis proto", zap.Error(err))
	}
	return hash.Hash256b(b)
}

func (g *Blockchain) loadFeatures() {
	hardForks := []*HardFork{
		&g.Pacific,
		&g.Aleutian,
		&g.Bering,
		&g.Cook,
		&g.Dardanelles,
		&g.Daytona,
		&g.Easter,
		&g.FbkMigration,
		&g.Fairbank,
		&g.Greenland,
		&g.Hawaii,
		&g.Iceland,
		&g.Jutland,
	}
	for _, v := range hardForks {
		for _, feature := range v.EnableFeatures {
			_, ok := g.FeatureHeightMap[feature]
			if !ok {
				g.FeatureHeightMap[feature] = v.Height
			}
		}
	}
}

func (g *Blockchain) supportFeature(height uint64, feature string) bool {
	ht, ok := g.FeatureHeightMap[feature]
	return ok && ht >= height
}

func (g *Blockchain) SupportPacific(height uint64) bool {
	return g.supportFeature(height, FEATURE_PACIFIC)
}

func (g *Blockchain) isPost(targetHeight, height uint64) bool {
	return height >= targetHeight
}

// IsPacific checks whether height is equal to or larger than pacific height
func (g *Blockchain) IsPacific(height uint64) bool {
	return g.isPost(g.PacificBlockHeight, height)
}

// IsAleutian checks whether height is equal to or larger than aleutian height
func (g *Blockchain) IsAleutian(height uint64) bool {
	return g.isPost(g.AleutianBlockHeight, height)
}

// IsBering checks whether height is equal to or larger than bering height
func (g *Blockchain) IsBering(height uint64) bool {
	return g.isPost(g.BeringBlockHeight, height)
}

// IsCook checks whether height is equal to or larger than cook height
func (g *Blockchain) IsCook(height uint64) bool {
	return g.isPost(g.CookBlockHeight, height)
}

// IsDardanelles checks whether height is equal to or larger than dardanelles height
func (g *Blockchain) IsDardanelles(height uint64) bool {
	return g.isPost(g.DardanellesBlockHeight, height)
}

// IsDaytona checks whether height is equal to or larger than daytona height
func (g *Blockchain) IsDaytona(height uint64) bool {
	return g.isPost(g.DaytonaBlockHeight, height)
}

// IsEaster checks whether height is equal to or larger than easter height
func (g *Blockchain) IsEaster(height uint64) bool {
	return g.isPost(g.EasterBlockHeight, height)
}

// IsFairbank checks whether height is equal to or larger than fairbank height
func (g *Blockchain) IsFairbank(height uint64) bool {
	return g.isPost(g.FairbankBlockHeight, height)
}

// IsFbkMigration checks whether height is equal to or larger than fbk migration height
func (g *Blockchain) IsFbkMigration(height uint64) bool {
	return g.isPost(g.FbkMigrationBlockHeight, height)
}

// IsGreenland checks whether height is equal to or larger than greenland height
func (g *Blockchain) IsGreenland(height uint64) bool {
	return g.isPost(g.GreenlandBlockHeight, height)
}

// IsHawaii checks whether height is equal to or larger than hawaii height
func (g *Blockchain) IsHawaii(height uint64) bool {
	return g.isPost(g.HawaiiBlockHeight, height)
}

// IsIceland checks whether height is equal to or larger than iceland height
func (g *Blockchain) IsIceland(height uint64) bool {
	return g.isPost(g.IcelandBlockHeight, height)
}

// IsJutland checks whether height is equal to or larger than jutland height
func (g *Blockchain) IsJutland(height uint64) bool {
	return g.isPost(g.JutlandBlockHeight, height)
}

// InitBalances returns the address that have initial balances and the corresponding amounts. The i-th amount is the
// i-th address' balance.
func (a *Account) InitBalances() ([]address.Address, []*big.Int) {
	// Make the list always be ordered
	addrStrs := make([]string, 0)
	for addrStr := range a.InitBalanceMap {
		addrStrs = append(addrStrs, addrStr)
	}
	sort.Strings(addrStrs)
	addrs := make([]address.Address, 0)
	amounts := make([]*big.Int, 0)
	for _, addrStr := range addrStrs {
		addr, err := address.FromString(addrStr)
		if err != nil {
			log.L().Panic("Error when decoding the account protocol init balance address from string.", zap.Error(err))
		}
		addrs = append(addrs, addr)
		amount, ok := big.NewInt(0).SetString(a.InitBalanceMap[addrStr], 10)
		if !ok {
			log.S().Panicf("Error when casting init balance string %s into big int", a.InitBalanceMap[addrStr])
		}
		amounts = append(amounts, amount)
	}
	return addrs, amounts
}

// OperatorAddr is the address of operator
func (d *Delegate) OperatorAddr() address.Address {
	addr, err := address.FromString(d.OperatorAddrStr)
	if err != nil {
		log.L().Panic("Error when decoding the poll protocol operator address from string.", zap.Error(err))
	}
	return addr
}

// RewardAddr is the address of rewardee, which is allowed to be nil
func (d *Delegate) RewardAddr() address.Address {
	if d.RewardAddrStr == "" {
		return nil
	}
	addr, err := address.FromString(d.RewardAddrStr)
	if err != nil {
		log.L().Panic("Error when decoding the poll protocol rewardee address from string.", zap.Error(err))
	}
	return addr
}

// Votes returns the votes
func (d *Delegate) Votes() *big.Int {
	val, ok := big.NewInt(0).SetString(d.VotesStr, 10)
	if !ok {
		log.S().Panicf("Error when casting votes string %s into big int", d.VotesStr)
	}
	return val
}

// InitBalance returns the init balance of the rewarding fund
func (r *Rewarding) InitBalance() *big.Int {
	val, ok := big.NewInt(0).SetString(r.InitBalanceStr, 10)
	if !ok {
		log.S().Panicf("Error when casting init balance string %s into big int", r.InitBalanceStr)
	}
	return val
}

// BlockReward returns the block reward amount
func (r *Rewarding) BlockReward() *big.Int {
	val, ok := big.NewInt(0).SetString(r.BlockRewardStr, 10)
	if !ok {
		log.S().Panicf("Error when casting block reward string %s into big int", r.BlockRewardStr)
	}
	return val
}

// EpochReward returns the epoch reward amount
func (r *Rewarding) EpochReward() *big.Int {
	val, ok := big.NewInt(0).SetString(r.EpochRewardStr, 10)
	if !ok {
		log.S().Panicf("Error when casting epoch reward string %s into big int", r.EpochRewardStr)
	}
	return val
}

// AleutianEpochReward returns the epoch reward amount after Aleutian fork
func (r *Rewarding) AleutianEpochReward() *big.Int {
	val, ok := big.NewInt(0).SetString(r.AleutianEpochRewardStr, 10)
	if !ok {
		log.S().Panicf("Error when casting epoch reward string %s into big int", r.EpochRewardStr)
	}
	return val
}

// DardanellesBlockReward returns the block reward amount after dardanelles fork
func (r *Rewarding) DardanellesBlockReward() *big.Int {
	val, ok := big.NewInt(0).SetString(r.DardanellesBlockRewardStr, 10)
	if !ok {
		log.S().Panicf("Error when casting block reward string %s into big int", r.EpochRewardStr)
	}
	return val
}

// ExemptAddrsFromEpochReward returns the list of addresses that exempt from epoch reward
func (r *Rewarding) ExemptAddrsFromEpochReward() []address.Address {
	addrs := make([]address.Address, 0)
	for _, addrStr := range r.ExemptAddrStrsFromEpochReward {
		addr, err := address.FromString(addrStr)
		if err != nil {
			log.L().Panic("Error when decoding the rewarding protocol exempt address from string.", zap.Error(err))
		}
		addrs = append(addrs, addr)
	}
	return addrs
}

// FoundationBonus returns the bootstrap bonus amount rewarded per epoch
func (r *Rewarding) FoundationBonus() *big.Int {
	val, ok := big.NewInt(0).SetString(r.FoundationBonusStr, 10)
	if !ok {
		log.S().Panicf("Error when casting bootstrap bonus string %s into big int", r.EpochRewardStr)
	}
	return val
}
