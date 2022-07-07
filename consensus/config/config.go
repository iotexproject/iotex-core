package config

import "time"

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
)
