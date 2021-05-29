// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensusfsm

import (
	"time"

	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
)

type (
	// ConsensusConfig defines a set of time durations used in fsm
	ConsensusConfig interface {
		EventChanSize() uint
		UnmatchedEventTTL(uint64) time.Duration
		UnmatchedEventInterval(uint64) time.Duration
		AcceptBlockTTL(uint64) time.Duration
		AcceptProposalEndorsementTTL(uint64) time.Duration
		AcceptLockEndorsementTTL(uint64) time.Duration
		CommitTTL(uint64) time.Duration
		BlockInterval(uint64) time.Duration
		Delay(uint64) time.Duration
	}

	// config implements ConsensusConfig
	consensusCfg struct {
		cfg           config.ConsensusTiming
		dardanelles   config.DardanellesUpgrade
		g             genesis.Genesis
		blockInterval time.Duration
		delay         time.Duration
	}
)

// NewConsensusConfig creates a ConsensusConfig out of config.
func NewConsensusConfig(cfg config.Config) ConsensusConfig {
	return &consensusCfg{
		cfg:           cfg.Consensus.RollDPoS.FSM,
		dardanelles:   cfg.DardanellesUpgrade,
		g:             cfg.Genesis,
		blockInterval: cfg.Genesis.Blockchain.BlockInterval,
		delay:         cfg.Consensus.RollDPoS.Delay,
	}
}

func (c *consensusCfg) EventChanSize() uint {
	return c.cfg.EventChanSize
}

func (c *consensusCfg) UnmatchedEventTTL(height uint64) time.Duration {
	if c.g.IsDardanelles(height) {
		return c.dardanelles.UnmatchedEventTTL
	}
	return c.cfg.UnmatchedEventTTL
}

func (c *consensusCfg) UnmatchedEventInterval(height uint64) time.Duration {
	if c.g.IsDardanelles(height) {
		return c.dardanelles.UnmatchedEventInterval
	}
	return c.cfg.UnmatchedEventInterval
}

func (c *consensusCfg) AcceptBlockTTL(height uint64) time.Duration {
	if c.g.IsDardanelles(height) {
		return c.dardanelles.AcceptBlockTTL
	}
	return c.cfg.AcceptBlockTTL
}

func (c *consensusCfg) AcceptProposalEndorsementTTL(height uint64) time.Duration {
	if c.g.IsDardanelles(height) {
		return c.dardanelles.AcceptProposalEndorsementTTL
	}
	return c.cfg.AcceptProposalEndorsementTTL
}

func (c *consensusCfg) AcceptLockEndorsementTTL(height uint64) time.Duration {
	if c.g.IsDardanelles(height) {
		return c.dardanelles.AcceptLockEndorsementTTL
	}
	return c.cfg.AcceptLockEndorsementTTL
}

func (c *consensusCfg) CommitTTL(height uint64) time.Duration {
	if c.g.IsDardanelles(height) {
		return c.dardanelles.CommitTTL
	}
	return c.cfg.CommitTTL
}

func (c *consensusCfg) BlockInterval(height uint64) time.Duration {
	if c.g.IsDardanelles(height) {
		return c.dardanelles.BlockInterval
	}
	return c.blockInterval
}

func (c *consensusCfg) Delay(height uint64) time.Duration {
	if c.g.IsDardanelles(height) {
		return c.dardanelles.Delay
	}
	return c.delay
}
