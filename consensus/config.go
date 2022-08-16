package consensus

import "time"

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
		RollDPoS: RollDPoS{
			FSM: ConsensusTiming{
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
		Scheme   string   `yaml:"scheme"`
		RollDPoS RollDPoS `yaml:"rollDPoS"`
	}

	// RollDPoS is the config struct for RollDPoS consensus package
	RollDPoS struct {
		FSM               ConsensusTiming `yaml:"fsm"`
		ToleratedOvertime time.Duration   `yaml:"toleratedOvertime"`
		Delay             time.Duration   `yaml:"delay"`
		ConsensusDBPath   string          `yaml:"consensusDBPath"`
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
)
