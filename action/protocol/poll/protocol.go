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

// CandidatesByHeight returns the candidates of a given height
type CandidatesByHeight func(protocol.StateReader, uint64) ([]*state.Candidate, error)

// GetCandidates returns the current candidates
type GetCandidates func(protocol.StateReader, bool) ([]*state.Candidate, uint64, error)

// GetKickoutList returns current the blacklist
type GetKickoutList func(protocol.StateReader, bool) (*vote.Blacklist, uint64, error)

// GetUnproductiveDelegate returns unproductiveDelegate struct which contains a cache of upd info by epochs
type GetUnproductiveDelegate func(protocol.StateReader) (*vote.UnproductiveDelegate, error)

// GetBlockTime defines a function to get block creation time
type GetBlockTime func(uint64) (time.Time, error)

// Productivity returns the number of produced blocks per producer
type Productivity func(uint64, uint64) (map[string]uint64, error)

// Protocol defines the protocol of handling votes
type Protocol interface {
	protocol.Protocol
	protocol.GenesisStateCreator
	Delegates(context.Context, protocol.StateReader) (state.CandidateList, error)
	NextDelegates(context.Context, protocol.StateReader) (state.CandidateList, error)
	Candidates(context.Context, protocol.StateReader) (state.CandidateList, error)
	NextCandidates(context.Context, protocol.StateReader) (state.CandidateList, error)
	// CalculateCandidatesByHeight calculates candidate and returns candidates by chain height
	CalculateCandidatesByHeight(context.Context, uint64) (state.CandidateList, error)
}

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
	candidatesByHeight CandidatesByHeight,
	getCandidates GetCandidates,
	getkickoutList GetKickoutList,
	getUnproductiveDelegate GetUnproductiveDelegate,
	electionCommittee committee.Committee,
	stakingV2 *staking.Protocol,
	getBlockTimeFunc GetBlockTime,
	productivity Productivity,
) (Protocol, error) {
	genesisConfig := cfg.Genesis
	if cfg.Consensus.Scheme != config.RollDPoSScheme {
		return nil, nil
	}

	switch genesisConfig.PollMode {
	case _modeLifeLong:
		delegates := genesisConfig.Delegates
		if uint64(len(delegates)) < genesisConfig.NumDelegates {
			return nil, errors.New("invalid delegate address in genesis block")
		}
		return NewLifeLongDelegatesProtocol(delegates), nil
	case _modeGovernanceMix:
		if !genesisConfig.EnableGravityChainVoting || electionCommittee == nil {
			return nil, errors.New("gravity chain voting is not enabled")
		}
		slasher, err := NewSlasher(
			&genesisConfig,
			productivity,
			candidatesByHeight,
			getCandidates,
			getkickoutList,
			getUnproductiveDelegate,
			candidateIndexer,
			genesisConfig.NumCandidateDelegates,
			genesisConfig.NumDelegates,
			genesisConfig.ProductivityThreshold,
			genesisConfig.KickoutEpochPeriod,
			genesisConfig.UnproductiveDelegateMaxCacheSize,
			genesisConfig.KickoutIntensityRate)
		if err != nil {
			return nil, err
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
		scoreThreshold, ok := new(big.Int).SetString(cfg.Genesis.ScoreThreshold, 10)
		if !ok {
			return nil, errors.Errorf("failed to parse score threshold %s", cfg.Genesis.ScoreThreshold)
		}
		return NewStakingCommittee(
			electionCommittee,
			governance,
			readContract,
			cfg.Genesis.NativeStakingContractAddress,
			cfg.Genesis.NativeStakingContractCode,
			scoreThreshold,
		)
	case _modeNative:
		// TODO
		return nil, errors.New("not implemented")
	case _modeNativeMix:
		// TODO
		return nil, errors.New("not implemented")
	case _modeConsortium:
		return NewConsortiumCommittee(candidateIndexer, readContract)
	default:
		return nil, errors.Errorf("unsupported poll mode %s", genesisConfig.PollMode)
	}
}
