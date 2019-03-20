// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package genesis

import (
	"flag"
	"math/big"
	"sort"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"go.uber.org/config"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
	"github.com/iotexproject/iotex-core/test/identityset"
)

var (
	// Default contains the default genesis config
	Default     Genesis
	genesisPath string
)

func init() {
	flag.StringVar(&genesisPath, "genesis-path", "", "Genesis path")
	initDefaultConfig()
}

func initDefaultConfig() {
	Default = Genesis{
		Blockchain: Blockchain{
			Timestamp:             1546329600,
			BlockGasLimit:         20000000,
			ActionGasLimit:        5000000,
			BlockInterval:         10 * time.Second,
			NumSubEpochs:          2,
			NumDelegates:          24,
			NumCandidateDelegates: 36,
			TimeBasedRotation:     false,
		},
		Account: Account{
			InitBalanceMap: make(map[string]string),
		},
		Poll: Poll{
			EnableGravityChainVoting: false,
		},
		Rewarding: Rewarding{
			InitBalanceStr:                 unit.ConvertIotxToRau(1200000000).String(),
			BlockRewardStr:                 unit.ConvertIotxToRau(16).String(),
			EpochRewardStr:                 unit.ConvertIotxToRau(300000).String(),
			NumDelegatesForEpochReward:     100,
			ExemptAddrStrsFromEpochReward:  []string{},
			FoundationBonusStr:             unit.ConvertIotxToRau(2880).String(),
			NumDelegatesForFoundationBonus: 36,
			FoundationBonusLastEpoch:       365,
		},
	}
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
		Account    `ymal:"account"`
		Poll       `yaml:"poll"`
		Rewarding  `yaml:"rewarding"`
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
		// NumDelegates is the number of delegates that participate into one epoch of block production
		NumDelegates uint64 `yaml:"numDelegates"`
		// NumCandidateDelegates is the number of candidate delegates, who may be selected as a delegate via roll dpos
		NumCandidateDelegates uint64 `yaml:"numCandidateDelegates"`
		// TimeBasedRotation is the flag to enable rotating delegates' time slots on a block height
		TimeBasedRotation bool `yaml:"timeBasedRotation"`
	}
	// Account contains the configs for account protocol
	Account struct {
		// InitBalanceMap is the address and initial balance mapping before the first block.
		InitBalanceMap map[string]string `yaml:"initBalances"`
	}
	// Poll contains the configs for poll protocol
	Poll struct {
		// EnableGravityChainVoting is a flag whether read voting from gravity chain
		EnableGravityChainVoting bool `yaml:"enableGravityChainVoting"`
		// GravityChainStartHeight is the height in gravity chain where the init poll result stored
		GravityChainStartHeight uint64 `yaml:"gravityChainStartHeight"`
		// GravityChainHeightInterval the height interval on gravity chain to pull delegate information
		GravityChainHeightInterval uint64 `yaml:"gravityChainHeightInterval"`
		// RegisterContractAddress is the address of register contract
		RegisterContractAddress string `yaml:"registerContractAddress"`
		// StakingContractAddress is the address of staking contract
		StakingContractAddress string `yaml:"stakingContractAddress"`
		// VoteThreshold is the vote threshold amount in decimal string format
		VoteThreshold string `yaml:"voteThreshold"`
		// ScoreThreshold is the score threshold amount in decimal string format
		ScoreThreshold string `yaml:"scoreThreshold"`
		// SelfStakingThreshold is self-staking vote threshold amount in decimal string format
		SelfStakingThreshold string `yaml:"selfStakingThreshold"`
		// Delegates is a list of delegates with votes
		Delegates []Delegate `yaml:"delegates"`
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
		// EpochReward is the epoch reward amount in decimal string format
		EpochRewardStr string `yaml:"epochReward"`
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
		// ProductivityThreshold is the percentage number that a delegate's productivity needs to reach to get the
		// epoch reward
		ProductivityThreshold uint64 `yaml:"productivityThreshold"`
	}
)

// New constructs a genesis config. It loads the default values, and could be overwritten by values defined in the yaml
// config files
func New() (Genesis, error) {
	opts := make([]config.YAMLOption, 0)
	opts = append(opts, config.Static(Default))
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
	return genesis, nil
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
