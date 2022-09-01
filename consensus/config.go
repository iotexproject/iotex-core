package consensus

import (
	"github.com/iotexproject/iotex-core/consensus/scheme/rolldpos"
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
		Scheme:   StandaloneScheme,
		RollDPoS: rolldpos.DefaultConfig,
	}
)

type (
	// Config is the config struct for consensus package
	Config struct {
		// There are three schemes that are supported
		Scheme   string          `yaml:"scheme"`
		RollDPoS rolldpos.Config `yaml:"rollDPoS"`
	}
)
