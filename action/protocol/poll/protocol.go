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

	"github.com/iotexproject/iotex-election/committee"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
)

const (
	protocolID = "poll"
)

const (
	_modeLifeLong      = "lifeLong"
	_modeGovernanceMix = "governanceMix" // mix governance with native staking contract
	_modeNative        = "native"        // only use go naitve staking
	_modeNativeMix     = "nativeMix"     // native with backward compatibility for governanceMix before fairbank
	_modeConsortium    = "consortium"

	blockMetaPrefix = "BlockMeta."
)

// ErrInconsistentHeight is an error that result of "readFromStateDB" is not consistent with others
var ErrInconsistentHeight = errors.New("data is inconsistent because the state height has been changed")

// ErrNoElectionCommittee is an error that the election committee is not specified
var ErrNoElectionCommittee = errors.New("no election committee specified")

// ErrProposedDelegatesLength is an error that the proposed delegate list length is not right
var ErrProposedDelegatesLength = errors.New("the proposed delegate list length")

// ErrDelegatesNotAsExpected is an error that the delegates are not as expected
var ErrDelegatesNotAsExpected = errors.New("delegates are not as expected")

// ErrDelegatesNotExist is an error that the delegates cannot be prepared
var ErrDelegatesNotExist = errors.New("delegates cannot be found")

type (
	// GetCandidates returns the current candidates
	GetCandidates func(protocol.StateReader, uint64, bool, bool) ([]*state.Candidate, uint64, error)

	// GetProbationList returns current the ProbationList
	GetProbationList func(protocol.StateReader, bool) (*vote.ProbationList, uint64, error)

	// GetUnproductiveDelegate returns unproductiveDelegate struct which contains a cache of upd info by epochs
	GetUnproductiveDelegate func(protocol.StateReader) (*vote.UnproductiveDelegate, error)

	// GetBlockTime defines a function to get block creation time
	GetBlockTime func(uint64) (time.Time, error)

	// Productivity returns the number of produced blocks per producer
	Productivity func(uint64, uint64) (map[string]uint64, error)

	// Protocol defines the protocol of handling votes
	Protocol interface {
		protocol.Protocol
		protocol.ActionValidator
		protocol.GenesisStateCreator
		Delegates(context.Context, protocol.StateReader) (state.CandidateList, error)
		NextDelegates(context.Context, protocol.StateReader) (state.CandidateList, error)
		Candidates(context.Context, protocol.StateReader) (state.CandidateList, error)
		NextCandidates(context.Context, protocol.StateReader) (state.CandidateList, error)
		// CalculateCandidatesByHeight calculates candidate and returns candidates by chain height
		// TODO: remove height, and read it from state reader
		CalculateCandidatesByHeight(context.Context, protocol.StateReader, uint64) (state.CandidateList, error)
		// CalculateUnproductiveDelegates calculates unproductive delegate on current epoch
		CalculateUnproductiveDelegates(context.Context, protocol.StateReader) ([]string, error)
	}
)

// FindProtocol finds the registered protocol from registry
func FindProtocol(registry *protocol.Registry) Protocol {
	if registry == nil {
		return nil
	}
	p, ok := registry.Find(protocolID)
	if !ok {
		return nil
	}
	pp, ok := p.(Protocol)
	if !ok {
		log.S().Panic("fail to cast poll protocol")
	}
	return pp
}

// MustGetProtocol return a registered protocol from registry
func MustGetProtocol(registry *protocol.Registry) Protocol {
	if registry == nil {
		log.S().Panic("registry cannot be nil")
	}
	p, ok := registry.Find(protocolID)
	if !ok {
		log.S().Panic("poll protocol is not registered")
	}

	pp, ok := p.(Protocol)
	if !ok {
		log.S().Panic("fail to cast poll protocol")
	}

	return pp
}

// NewProtocol instantiates a rewarding protocol instance.
func NewProtocol(
	cfg config.Config,
	candidateIndexer *CandidateIndexer,
	readContract ReadContract,
	getCandidates GetCandidates,
	getprobationList GetProbationList,
	getUnproductiveDelegate GetUnproductiveDelegate,
	electionCommittee committee.Committee,
	stakingProto *staking.Protocol,
	getBlockTimeFunc GetBlockTime,
	productivity Productivity,
	getBlockHash evm.GetBlockHash,
) (Protocol, error) {
	genesisConfig := cfg.Genesis
	if cfg.Consensus.Scheme != config.RollDPoSScheme {
		return nil, nil
	}

	var (
		slasher        *Slasher
		scoreThreshold *big.Int
	)
	switch genesisConfig.PollMode {
	case _modeGovernanceMix, _modeNative, _modeNativeMix:
		var (
			err error
			ok  bool
		)
		slasher, err = NewSlasher(
			productivity,
			getCandidates,
			getprobationList,
			getUnproductiveDelegate,
			candidateIndexer,
			genesisConfig.NumCandidateDelegates,
			genesisConfig.NumDelegates,
			genesisConfig.DardanellesNumSubEpochs,
			genesisConfig.ProductivityThreshold,
			genesisConfig.ProbationEpochPeriod,
			genesisConfig.UnproductiveDelegateMaxCacheSize,
			genesisConfig.ProbationIntensityRate)
		if err != nil {
			return nil, err
		}
		scoreThreshold, ok = new(big.Int).SetString(cfg.Genesis.ScoreThreshold, 10)
		if !ok {
			return nil, errors.Errorf("failed to parse score threshold %s", cfg.Genesis.ScoreThreshold)
		}
	}

	var stakingV1 Protocol
	switch genesisConfig.PollMode {
	case _modeGovernanceMix, _modeNativeMix:
		if !genesisConfig.EnableGravityChainVoting || electionCommittee == nil {
			return nil, errors.New("gravity chain voting is not enabled")
		}
		governance, err := NewGovernanceChainCommitteeProtocol(
			candidateIndexer,
			electionCommittee,
			genesisConfig.GravityChainStartHeight,
			getBlockTimeFunc,
			cfg.Chain.PollInitialCandidatesInterval,
			slasher,
		)
		if err != nil {
			return nil, err
		}
		stakingV1, err = NewStakingCommittee(
			electionCommittee,
			governance,
			readContract,
			cfg.Genesis.NativeStakingContractAddress,
			cfg.Genesis.NativeStakingContractCode,
			scoreThreshold,
		)
		if err != nil {
			return nil, err
		}
	}

	switch genesisConfig.PollMode {
	case _modeLifeLong:
		delegates := genesisConfig.Delegates
		if uint64(len(delegates)) < genesisConfig.NumDelegates {
			return nil, errors.New("invalid delegate address in genesis block")
		}
		return NewLifeLongDelegatesProtocol(delegates), nil
	case _modeGovernanceMix:
		return stakingV1, nil
	case _modeNativeMix, _modeNative:
		stakingV2, err := newNativeStakingV2(candidateIndexer, slasher, scoreThreshold, stakingProto)
		if err != nil {
			return nil, err
		}
		return NewStakingCommand(stakingV1, stakingV2)
	case _modeConsortium:
		return NewConsortiumCommittee(candidateIndexer, readContract, getBlockHash)
	default:
		return nil, errors.Errorf("unsupported poll mode %s", genesisConfig.PollMode)
	}
}
