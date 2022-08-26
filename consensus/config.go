package consensus

import (
	"time"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/db"
)

const (
	// RollDPoSScheme means randomized delegated proof of stake
	RollDPoSScheme = "ROLLDPOS"
	// StandaloneScheme means that the node creates a block periodically regardless of others (if there is any)
	StandaloneScheme = "STANDALONE"
	// NOOPScheme means that the node does not create only block
	NOOPScheme = "NOOP"
)

var (
	//DefaultConfig is the default config for blocksync
	DefaultConfig = Config{
		Scheme: StandaloneScheme,
		RollDPoS: RollDPoSConfig{
			FSM: Timing{
				UnmatchedEventTTL:            3 * time.Second,
				UnmatchedEventInterval:       100 * time.Millisecond,
				AcceptBlockTTL:               4 * time.Second,
				AcceptProposalEndorsementTTL: 2 * time.Second,
				AcceptLockEndorsementTTL:     2 * time.Second,
				CommitTTL:                    2 * time.Second,
				EventChanSize:                10000,
			},
			ToleratedOvertime: 2 * time.Second,
			Delay:             5 * time.Second,
			ConsensusDBPath:   "/var/data/consensus.db",
		},
	}

	//DefaultDardanellesUpgradeConfig is the default config for dardanelles upgrade
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
)

type (
	// Config is the config struct for consensus package
	Config struct {
		// There are three schemes that are supported
		Scheme   string         `yaml:"scheme"`
		RollDPoS RollDPoSConfig `yaml:"rollDPoS"`
	}

	// RollDPoSConfig is the config struct for RollDPoS consensus package
	RollDPoSConfig struct {
		FSM               Timing        `yaml:"fsm"`
		ToleratedOvertime time.Duration `yaml:"toleratedOvertime"`
		Delay             time.Duration `yaml:"delay"`
		ConsensusDBPath   string        `yaml:"consensusDBPath"`
	}

	// Timing defines a set of time durations used in fsm and event queue size
	Timing struct {
		EventChanSize                uint          `yaml:"eventChanSize"`
		UnmatchedEventTTL            time.Duration `yaml:"unmatchedEventTTL"`
		UnmatchedEventInterval       time.Duration `yaml:"unmatchedEventInterval"`
		AcceptBlockTTL               time.Duration `yaml:"acceptBlockTTL"`
		AcceptProposalEndorsementTTL time.Duration `yaml:"acceptProposalEndorsementTTL"`
		AcceptLockEndorsementTTL     time.Duration `yaml:"acceptLockEndorsementTTL"`
		CommitTTL                    time.Duration `yaml:"commitTTL"`
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

	// BuilderConfig returns the configuration of the builder
	BuilderConfig struct {
		Scheme             string
		DB                 db.Config
		Chain              blockchain.Config
		ActPool            actpool.Config
		Consensus          Config
		DardanellesUpgrade DardanellesUpgrade
		BlockSync          blocksync.Config
		Genesis            genesis.Genesis
		SystemActive       bool
	}

	// FSMConfig defines a set of time durations used in fsm
	FSMConfig interface {
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

	// config implements FSMConfig
	fsmCfg struct {
		cfg           Timing
		dardanelles   DardanellesUpgrade
		g             genesis.Genesis
		blockInterval time.Duration
		delay         time.Duration
	}
)

//CreateBuilderConfig creates a builder config
func CreateBuilderConfig(cfg Config, dc db.Config, bcc blockchain.Config, g genesis.Genesis, ac actpool.Config, du DardanellesUpgrade, bsc blocksync.Config) BuilderConfig {
	return BuilderConfig{
		DB:                 dc,
		Chain:              bcc,
		ActPool:            ac,
		Consensus:          cfg,
		DardanellesUpgrade: du,
		BlockSync:          bsc,
		Genesis:            g,
	}
}

// NewFSMConfig creates a FSMConfig out of config.
func NewFSMConfig(cfg Config, dardanelles DardanellesUpgrade, g genesis.Genesis) FSMConfig {
	return &fsmCfg{
		cfg:           cfg.RollDPoS.FSM,
		dardanelles:   dardanelles,
		g:             g,
		blockInterval: g.Blockchain.BlockInterval,
		delay:         cfg.RollDPoS.Delay,
	}
}

func (c *fsmCfg) EventChanSize() uint {
	return c.cfg.EventChanSize
}

func (c *fsmCfg) UnmatchedEventTTL(height uint64) time.Duration {
	if c.g.IsDardanelles(height) {
		return c.dardanelles.UnmatchedEventTTL
	}
	return c.cfg.UnmatchedEventTTL
}

func (c *fsmCfg) UnmatchedEventInterval(height uint64) time.Duration {
	if c.g.IsDardanelles(height) {
		return c.dardanelles.UnmatchedEventInterval
	}
	return c.cfg.UnmatchedEventInterval
}

func (c *fsmCfg) AcceptBlockTTL(height uint64) time.Duration {
	if c.g.IsDardanelles(height) {
		return c.dardanelles.AcceptBlockTTL
	}
	return c.cfg.AcceptBlockTTL
}

func (c *fsmCfg) AcceptProposalEndorsementTTL(height uint64) time.Duration {
	if c.g.IsDardanelles(height) {
		return c.dardanelles.AcceptProposalEndorsementTTL
	}
	return c.cfg.AcceptProposalEndorsementTTL
}

func (c *fsmCfg) AcceptLockEndorsementTTL(height uint64) time.Duration {
	if c.g.IsDardanelles(height) {
		return c.dardanelles.AcceptLockEndorsementTTL
	}
	return c.cfg.AcceptLockEndorsementTTL
}

func (c *fsmCfg) CommitTTL(height uint64) time.Duration {
	if c.g.IsDardanelles(height) {
		return c.dardanelles.CommitTTL
	}
	return c.cfg.CommitTTL
}

func (c *fsmCfg) BlockInterval(height uint64) time.Duration {
	if c.g.IsDardanelles(height) {
		return c.dardanelles.BlockInterval
	}
	return c.blockInterval
}

func (c *fsmCfg) Delay(height uint64) time.Duration {
	if c.g.IsDardanelles(height) {
		return c.dardanelles.Delay
	}
	return c.delay
}
