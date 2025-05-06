// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package consensusfsm

import (
	"time"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

var (
	// DefaultDardanellesUpgradeConfig is the default config for dardanelles upgrade
	DefaultDardanellesUpgradeConfig = DardanellesUpgrade{
		UnmatchedEventTTL:            2 * time.Second,
		UnmatchedEventInterval:       100 * time.Millisecond,
		AcceptBlockTTL:               2 * time.Second,
		AcceptProposalEndorsementTTL: time.Second,
		AcceptLockEndorsementTTL:     time.Second,
		CommitTTL:                    time.Second,
		BlockInterval:                5 * time.Second,
		Delay:                        2 * time.Second,
	}
	// DefaultWakeUpgradeConfig is the default config for wake upgrade
	DefaultWakeUpgradeConfig = WakeUpgrade{
		UnmatchedEventTTL:            2 * time.Second,
		UnmatchedEventInterval:       100 * time.Millisecond,
		AcceptBlockTTL:               1 * time.Second,
		AcceptProposalEndorsementTTL: time.Second,
		AcceptLockEndorsementTTL:     500 * time.Millisecond,
		CommitTTL:                    500 * time.Millisecond,
		BlockInterval:                3 * time.Second,
	}
)

type (
	// WakeUpgrade is the config for wake upgrade
	WakeUpgrade struct {
		UnmatchedEventTTL            time.Duration `yaml:"unmatchedEventTTL"`
		UnmatchedEventInterval       time.Duration `yaml:"unmatchedEventInterval"`
		AcceptBlockTTL               time.Duration `yaml:"acceptBlockTTL"`
		AcceptProposalEndorsementTTL time.Duration `yaml:"acceptProposalEndorsementTTL"`
		AcceptLockEndorsementTTL     time.Duration `yaml:"acceptLockEndorsementTTL"`
		CommitTTL                    time.Duration `yaml:"commitTTL"`
		BlockInterval                time.Duration `yaml:"blockInterval"`
	}
	// DardanellesUpgrade is the config for dardanelles upgrade
	DardanellesUpgrade struct {
		UnmatchedEventTTL            time.Duration `yaml:"unmatchedEventTTL"`
		UnmatchedEventInterval       time.Duration `yaml:"unmatchedEventInterval"`
		AcceptBlockTTL               time.Duration `yaml:"acceptBlockTTL"`
		AcceptProposalEndorsementTTL time.Duration `yaml:"acceptProposalEndorsementTTL"`
		AcceptLockEndorsementTTL     time.Duration `yaml:"acceptLockEndorsementTTL"`
		CommitTTL                    time.Duration `yaml:"commitTTL"`
		BlockInterval                time.Duration `yaml:"blockInterval"`
		Delay                        time.Duration `yaml:"delay"`
	}

	// ConsensusTiming defines a set of time durations used in fsm and event queue size
	ConsensusTiming struct {
		EventChanSize                uint          `yaml:"eventChanSize"`
		UnmatchedEventTTL            time.Duration `yaml:"unmatchedEventTTL"`
		UnmatchedEventInterval       time.Duration `yaml:"unmatchedEventInterval"`
		AcceptBlockTTL               time.Duration `yaml:"acceptBlockTTL"`
		AcceptProposalEndorsementTTL time.Duration `yaml:"acceptProposalEndorsementTTL"`
		AcceptLockEndorsementTTL     time.Duration `yaml:"acceptLockEndorsementTTL"`
		CommitTTL                    time.Duration `yaml:"commitTTL"`
	}

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
		cfg           ConsensusTiming
		blockInterval time.Duration
		delay         time.Duration

		dardanelles       DardanellesUpgrade
		dardanellesHeight uint64
		wake              WakeUpgrade
		wakeHeight        uint64
	}
)

// NewConsensusConfig creates a ConsensusConfig out of config.
func NewConsensusConfig(timing ConsensusTiming, dardanelles DardanellesUpgrade, wake WakeUpgrade, g genesis.Genesis, delay time.Duration) ConsensusConfig {
	return &consensusCfg{
		cfg:               timing,
		dardanelles:       dardanelles,
		dardanellesHeight: g.DardanellesBlockHeight,
		blockInterval:     g.Blockchain.BlockInterval,
		delay:             delay,
		wake:              wake,
		wakeHeight:        g.WakeBlockHeight,
	}
}

func (c *consensusCfg) isDardanelles(height uint64) bool {
	return height >= c.dardanellesHeight
}

func (c *consensusCfg) isWake(height uint64) bool {
	return height >= c.wakeHeight
}

func (c *consensusCfg) EventChanSize() uint {
	return c.cfg.EventChanSize
}

func (c *consensusCfg) UnmatchedEventTTL(height uint64) time.Duration {
	if c.isWake(height) {
		return c.wake.UnmatchedEventTTL
	}
	if c.isDardanelles(height) {
		return c.dardanelles.UnmatchedEventTTL
	}
	return c.cfg.UnmatchedEventTTL
}

func (c *consensusCfg) UnmatchedEventInterval(height uint64) time.Duration {
	if c.isWake(height) {
		return c.wake.UnmatchedEventInterval
	}
	if c.isDardanelles(height) {
		return c.dardanelles.UnmatchedEventInterval
	}
	return c.cfg.UnmatchedEventInterval
}

func (c *consensusCfg) AcceptBlockTTL(height uint64) time.Duration {
	if c.isWake(height) {
		return c.wake.AcceptBlockTTL
	}
	if c.isDardanelles(height) {
		return c.dardanelles.AcceptBlockTTL
	}
	return c.cfg.AcceptBlockTTL
}

func (c *consensusCfg) AcceptProposalEndorsementTTL(height uint64) time.Duration {
	if c.isWake(height) {
		return c.wake.AcceptProposalEndorsementTTL
	}
	if c.isDardanelles(height) {
		return c.dardanelles.AcceptProposalEndorsementTTL
	}
	return c.cfg.AcceptProposalEndorsementTTL
}

func (c *consensusCfg) AcceptLockEndorsementTTL(height uint64) time.Duration {
	if c.isWake(height) {
		return c.wake.AcceptLockEndorsementTTL
	}
	if c.isDardanelles(height) {
		return c.dardanelles.AcceptLockEndorsementTTL
	}
	return c.cfg.AcceptLockEndorsementTTL
}

func (c *consensusCfg) CommitTTL(height uint64) time.Duration {
	if c.isWake(height) {
		return c.wake.CommitTTL
	}
	if c.isDardanelles(height) {
		return c.dardanelles.CommitTTL
	}
	return c.cfg.CommitTTL
}

func (c *consensusCfg) BlockInterval(height uint64) time.Duration {
	if c.isWake(height) {
		return c.wake.BlockInterval
	}
	if c.isDardanelles(height) {
		return c.dardanelles.BlockInterval
	}
	return c.blockInterval
}

func (c *consensusCfg) Delay(height uint64) time.Duration {
	if c.isDardanelles(height) {
		return c.dardanelles.Delay
	}
	return c.delay
}
