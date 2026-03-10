package ioswarm

// Config holds IOSwarm coordinator configuration.
type Config struct {
	Enabled        bool         `yaml:"enabled"`
	GRPCPort       int          `yaml:"grpcPort"`       // default 14689
	SwarmAPIPort   int          `yaml:"swarmApiPort"`   // default 14690 (0 to disable)
	MaxAgents      int          `yaml:"maxAgents"`      // default 100
	TaskLevel      string       `yaml:"taskLevel"`      // "L1", "L2", "L3"
	ShadowMode     bool         `yaml:"shadowMode"`     // default true
	PollIntervalMS int          `yaml:"pollIntervalMs"` // default 1000
	MasterSecret    string       `yaml:"masterSecret"`    // HMAC master secret for agent auth (empty = no auth)
	DelegateAddress string       `yaml:"delegateAddress"` // delegate's IOTX address for reward payout
	Reward          RewardConfig `yaml:"reward"`
}

// DefaultConfig returns a Config with sane defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:        false,
		GRPCPort:       14689,
		SwarmAPIPort:   14690,
		MaxAgents:      100,
		TaskLevel:      "L2",
		ShadowMode:     true,
		PollIntervalMS: 1000,
		Reward:         DefaultRewardConfig(),
	}
}
