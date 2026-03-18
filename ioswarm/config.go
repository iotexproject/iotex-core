package ioswarm

// Config holds IOSwarm coordinator configuration.
type Config struct {
	Enabled         bool         `yaml:"enabled"`
	GRPCPort        int          `yaml:"grpcPort"`        // default 14689
	SwarmAPIPort    int          `yaml:"swarmApiPort"`     // default 14690 (0 to disable)
	MaxAgents       int          `yaml:"maxAgents"`        // default 100
	TaskLevel       string       `yaml:"taskLevel"`       // "L1", "L2", "L3", "L4"
	ShadowMode      bool         `yaml:"shadowMode"`      // default true
	PollIntervalMS  int          `yaml:"pollIntervalMs"`  // default 1000
	MasterSecret    string       `yaml:"masterSecret"`    // HMAC master secret for agent auth (empty = no auth)
	DelegateAddress string       `yaml:"delegateAddress"` // delegate's IOTX address for reward payout
	EpochRewardIOTX float64      `yaml:"epochRewardIOTX"` // IOTX per epoch for reward distribution (default 800)

	// On-chain reward pool settlement
	RewardContract  string  `yaml:"rewardContract"`  // AgentRewardPool contract address (empty = disabled)
	RewardSignerKey string  `yaml:"rewardSignerKey"` // hex private key for signing depositAndSettle txs
	RewardRPCURL    string  `yaml:"rewardRpcUrl"`    // RPC endpoint (default: https://babel-api.mainnet.iotex.io)
	RewardChainID   int64   `yaml:"rewardChainId"`   // chain ID (default: 4689 for IoTeX mainnet)

	DiffBufferSize   int          `yaml:"diffBufferSize"`   // ring buffer size for state diff broadcaster (default 100)
	DiffStoreEnabled bool         `yaml:"diffStoreEnabled"` // persist state diffs to disk (default true for L4)
	DiffStorePath    string       `yaml:"diffStorePath"`    // path to statediffs.db (default: <datadir>/statediffs.db)
	DiffRetainHeight uint64       `yaml:"diffRetainHeight"` // prune diffs older than this many blocks (default 10000 ≈ 27h; 0 = keep all)
	Reward           RewardConfig `yaml:"reward"`
}

// DefaultConfig returns a Config with sane defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:         false,
		GRPCPort:        14689,
		SwarmAPIPort:    14690,
		MaxAgents:       100,
		TaskLevel:       "L2",
		ShadowMode:      true,
		PollIntervalMS:  1000,
		EpochRewardIOTX: 800,
		DiffBufferSize:   100,
		DiffStoreEnabled: true,
		DiffRetainHeight: 10000, // ~27 hours at 10s/block; set 0 to keep all
		Reward:           DefaultRewardConfig(),
	}
}
