// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"flag"
	"os"
	"time"

	"github.com/pkg/errors"
	uconfig "go.uber.org/config"
	"google.golang.org/grpc/keepalive"

	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

// IMPORTANT: to define a config, add a field or a new config type to the existing config types. In addition, provide
// the default value in Default var.

func init() {
	flag.StringVar(&_overwritePath, "config-path", "", "Config path")
	flag.StringVar(&_secretPath, "secret-path", "", "Secret path")
}

var (
	// overwritePath is the path to the config file which overwrite default values
	_overwritePath string
	// secretPath is the path to the  config file store secret values
	_secretPath string
)

const (
	// DelegateType represents the delegate node type
	DelegateType = "delegate"
	// FullNodeType represents the full node type
	FullNodeType = "full_node"
	// LightweightType represents the lightweight type
	LightweightType = "lightweight"

	// RollDPoSScheme means randomized delegated proof of stake
	RollDPoSScheme = "ROLLDPOS"
	// StandaloneScheme means that the node creates a block periodically regardless of others (if there is any)
	StandaloneScheme = "STANDALONE"
	// NOOPScheme means that the node does not create only block
	NOOPScheme = "NOOP"
)

var (
	// Default is the default config
	Default = Config{
		NodeType: FullNodeType,
		Network: Network{
			IP:   "127.0.0.1",
			Port: 4689,
			MsgLogsCleaningInterval: 2 * time.Second,
			MsgLogRetention:         5 * time.Second,
			HealthCheckInterval:     time.Second,
			SilentInterval:          5 * time.Second,
			PeerMaintainerInterval:  time.Second,
			AllowMultiConnsPerIP:    false,
			NumPeersLowerBound:      5,
			NumPeersUpperBound:      5,
			PingInterval:            time.Second,
			RateLimitEnabled:        false,
			RateLimitPerSec:         10000,
			RateLimitWindowSize:     60 * time.Second,
			BootstrapNodes:          make([]string, 0),
			TLSEnabled:              false,
			CACrtPath:               "",
			PeerCrtPath:             "",
			PeerKeyPath:             "",
			KLClientParams:          keepalive.ClientParameters{},
			KLServerParams:          keepalive.ServerParameters{},
			KLPolicy:                keepalive.EnforcementPolicy{},
			MaxMsgSize:              10485760,
			PeerDiscovery:           true,
			TopologyPath:            "",
			TTL:                     3,
		},
		Chain: Chain{
			ChainDBPath:        "/tmp/chain.db",
			TrieDBPath:         "/tmp/trie.db",
			ProducerPubKey:     keypair.EncodePublicKey(keypair.ZeroPublicKey),
			ProducerPrivKey:    keypair.EncodePrivateKey(keypair.ZeroPrivateKey),
			InMemTest:          false,
			GenesisActionsPath: "",
			DelegateLRUSize:    10,
		},
		ActPool: ActPool{
			MaxNumActPerPool: 32000,
			MaxNumActPerAcct: 2000,
		},
		Consensus: Consensus{
			Scheme: NOOPScheme,
			RollDPoS: RollDPoS{
				DelegateInterval:  10 * time.Second,
				ProposerInterval:  10 * time.Second,
				ProposerCB:        "",
				EpochCB:           "",
				UnmatchedEventTTL: 3 * time.Second,
				RoundStartTTL:     10 * time.Second,
				AcceptProposeTTL:  time.Second,
				AcceptPrevoteTTL:  time.Second,
				AcceptVoteTTL:     time.Second,
				Delay:             5 * time.Second,
				NumSubEpochs:      1,
				EventChanSize:     10000,
				NumDelegates:      21,
			},
			BlockCreationInterval: 10 * time.Second,
		},
		BlockSync: BlockSync{
			Interval: 10 * time.Second,
		},

		Delegate: Delegate{
			Addrs:   make([]string, 0),
			RollNum: 0,
		},
		Dispatcher: Dispatcher{
			EventChanSize: 10000,
		},
		Explorer: Explorer{
			Enabled:   true,
			IsTest:    false,
			Port:      14004,
			TpsWindow: 10,
		},
		System: System{
			HeartbeatInterval: 10 * time.Second,
		},
	}

	// ErrInvalidCfg indicates the invalid config value
	ErrInvalidCfg = errors.New("invalid config value")

	// Validates is the collection config validation functions
	Validates = []Validate{
		ValidateAddr,
		ValidateConsensusScheme,
		ValidateRollDPoS,
		ValidateDispatcher,
		ValidateExplorer,
		ValidateNetwork,
		ValidateDelegate,
		ValidateActPool,
	}
)

// Network is the config struct for network package
type (
	Network struct {
		IP                      string                      `yaml:"ip"`
		Port                    int                         `yaml:"port"`
		MsgLogsCleaningInterval time.Duration               `yaml:"msgLogsCleaningInterval"`
		MsgLogRetention         time.Duration               `yaml:"msgLogRetention"`
		HealthCheckInterval     time.Duration               `yaml:"healthCheckInterval"`
		SilentInterval          time.Duration               `yaml:"silentInterval"`
		PeerMaintainerInterval  time.Duration               `yaml:"peerMaintainerInterval"`
		AllowMultiConnsPerIP    bool                        `yaml:"allowMultiConnsPerIP"`
		NumPeersLowerBound      uint                        `yaml:"numPeersLowerBound"`
		NumPeersUpperBound      uint                        `yaml:"numPeersUpperBound"`
		PingInterval            time.Duration               `yaml:"pingInterval"`
		RateLimitEnabled        bool                        `yaml:"rateLimitEnabled"`
		RateLimitPerSec         uint64                      `yaml:"rateLimitPerSec"`
		RateLimitWindowSize     time.Duration               `yaml:"rateLimitWindowSize"`
		BootstrapNodes          []string                    `yaml:"bootstrapNodes"`
		TLSEnabled              bool                        `yaml:"tlsEnabled"`
		CACrtPath               string                      `yaml:"caCrtPath"`
		PeerCrtPath             string                      `yaml:"peerCrtPath"`
		PeerKeyPath             string                      `yaml:"peerKeyPath"`
		KLClientParams          keepalive.ClientParameters  `yaml:"klClientParams"`
		KLServerParams          keepalive.ServerParameters  `yaml:"klServerParams"`
		KLPolicy                keepalive.EnforcementPolicy `yaml:"klPolicy"`
		MaxMsgSize              int                         `yaml:"maxMsgSize"`
		PeerDiscovery           bool                        `yaml:"peerDiscovery"`
		TopologyPath            string                      `yaml:"topologyPath"`
		TTL                     uint32                      `yaml:"ttl"`
	}

	// Chain is the config struct for blockchain package
	Chain struct {
		ChainDBPath string `yaml:"chainDBPath"`
		TrieDBPath  string `yaml:"trieDBPath"`

		ProducerPubKey  string `yaml:"producerPubKey"`
		ProducerPrivKey string `yaml:"producerPrivKey"`

		// InMemTest creates in-memory DB file for local testing
		InMemTest          bool   `yaml:"inMemTest"`
		GenesisActionsPath string `yaml:"genesisActionsPath"`
		DelegateLRUSize    uint   `yaml:"delegateLRUSize"`
	}

	// Consensus is the config struct for consensus package
	Consensus struct {
		// There are three schemes that are supported
		Scheme                string        `yaml:"scheme"`
		RollDPoS              RollDPoS      `yaml:"rollDPoS"`
		BlockCreationInterval time.Duration `yaml:"blockCreationInterval"`
	}

	// BlockSync is the config struct for the BlockSync
	BlockSync struct {
		Interval time.Duration `yaml:"interval"` // update duration
	}

	// RollDPoS is the config struct for RollDPoS consensus package
	RollDPoS struct {
		DelegateInterval  time.Duration `yaml:"delegateInterval"`
		ProposerInterval  time.Duration `yaml:"proposerInterval"`
		ProposerCB        string        `yaml:"proposerCB"`
		EpochCB           string        `yaml:"epochCB"`
		UnmatchedEventTTL time.Duration `yaml:"unmatchedEventTTL"`
		RoundStartTTL     time.Duration `yaml:"roundStartTTL"`
		AcceptProposeTTL  time.Duration `yaml:"acceptProposeTTL"`
		AcceptPrevoteTTL  time.Duration `yaml:"acceptPrevoteTTL"`
		AcceptVoteTTL     time.Duration `yaml:"acceptVoteTTL"`
		Delay             time.Duration `yaml:"delay"`
		NumSubEpochs      uint          `yaml:"numSubEpochs"`
		EventChanSize     uint          `yaml:"eventChanSize"`
		NumDelegates      uint          `yaml:"numDelegates"`
	}
	// Delegate is the delegate config
	Delegate struct {
		Addrs   []string `yaml:"addrs"`
		RollNum uint     `yaml:"rollNum"`
	}

	// Dispatcher is the dispatcher config
	Dispatcher struct {
		EventChanSize uint `yaml:"eventChanSize"`
	}

	// Explorer is the explorer service config
	Explorer struct {
		Enabled   bool `yaml:"enabled"`
		IsTest    bool `yaml:"isTest"`
		Port      int  `yaml:"addr"`
		TpsWindow int  `yaml:"tpsWindow"`
	}

	// System is the system config
	System struct {
		HeartbeatInterval time.Duration `yaml:"heartbeatInterval"`
	}

	// ActPool is the actpool config
	ActPool struct {
		MaxNumActPerPool uint64 `yaml:"maxNumActPerPool"`
		MaxNumActPerAcct uint64 `yaml:"maxNumActPerAcct"`
	}

	// Config is the root config struct, each package's config should be put as its sub struct
	Config struct {
		NodeType   string     `yaml:"nodeType"`
		Network    Network    `yaml:"network"`
		Chain      Chain      `yaml:"chain"`
		ActPool    ActPool    `yaml:"actPool"`
		Consensus  Consensus  `yaml:"consensus"`
		Delegate   Delegate   `yaml:"delegate"`
		BlockSync  BlockSync  `yaml:"blockSync"`
		Dispatcher Dispatcher `yaml:"dispatcher"`
		Explorer   Explorer   `yaml:"explorer"`
		System     System     `yaml:"system"`
	}

	// Validate is the interface of validating the config
	Validate func(*Config) error
)

// New creates a config instance. It first loads the default configs. If the config path is not empty, it will read from
// the file and override the default configs. By default, it will apply all validation functions. To bypass validation,
// use DoNotValidate instead.
func New(validates ...Validate) (*Config, error) {
	opts := make([]uconfig.YAMLOption, 0)
	opts = append(opts, uconfig.Static(Default))
	opts = append(opts, uconfig.Expand(os.LookupEnv))
	if _overwritePath != "" {
		opts = append(opts, uconfig.File(_overwritePath))
	}
	if _secretPath != "" {
		opts = append(opts, uconfig.File(_secretPath))
	}
	yaml, err := uconfig.NewYAML(opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init config")
	}

	var cfg Config
	if err := yaml.Get(uconfig.Root).Populate(&cfg); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal YAML config to struct")
	}

	// By default, the config needs to pass all the validation
	if len(validates) == 0 {
		validates = Validates
	}
	for _, validate := range validates {
		if err := validate(&cfg); err != nil {
			return nil, errors.Wrap(err, "failed to validate config")
		}
	}
	return &cfg, nil
}

// IsDelegate returns true if the node type is Delegate
func (cfg *Config) IsDelegate() bool {
	return cfg.NodeType == DelegateType
}

// IsFullnode returns true if the node type is Fullnode
func (cfg *Config) IsFullnode() bool {
	return cfg.NodeType == FullNodeType
}

// IsLightweight returns true if the node type is Lightweight
func (cfg *Config) IsLightweight() bool {
	return cfg.NodeType == LightweightType
}

// ProducerAddr returns address struct based on the data from producer pub/pri-key in the config
func (cfg *Config) ProducerAddr() (*iotxaddress.Address, error) {
	priKey, err := keypair.DecodePrivateKey(cfg.Chain.ProducerPrivKey)
	if err != nil {
		return nil, err
	}
	pubKey, err := keypair.DecodePublicKey(cfg.Chain.ProducerPubKey)
	addr, err := iotxaddress.GetAddress(pubKey, iotxaddress.IsTestnet, iotxaddress.ChainID)
	if err != nil {
		return nil, err
	}
	addr.PrivateKey = priKey
	return addr, nil
}

// ValidateAddr validates the block producer address
func ValidateAddr(cfg *Config) error {
	addr, err := cfg.ProducerAddr()
	if err != nil {
		return err
	}
	// Validate producer's address
	if len(addr.RawAddress) > 0 && !iotxaddress.ValidateAddress(addr.RawAddress) {
		return errors.Wrap(ErrInvalidCfg, "invalid block producer address")
	}
	// Validate producer pubkey and prikey by signing a dummy message and verify it
	validationMsg := "connecting the physical world block by block"
	sig := crypto.Sign(addr.PrivateKey, []byte(validationMsg))
	if !crypto.Verify(addr.PublicKey, []byte(validationMsg), sig) {
		return errors.Wrap(ErrInvalidCfg, "block producer has unmatched pubkey and prikey")
	}
	return nil
}

// ValidateConsensusScheme validates the if scheme and node type match
func ValidateConsensusScheme(cfg *Config) error {
	switch cfg.NodeType {
	case DelegateType:
	case FullNodeType:
		if cfg.Consensus.Scheme != NOOPScheme {
			return errors.Wrap(ErrInvalidCfg, "consensus scheme of fullnode should be NOOP")
		}
	case LightweightType:
		if cfg.Consensus.Scheme != NOOPScheme {
			return errors.Wrap(ErrInvalidCfg, "consensus scheme of lightweight node should be NOOP")
		}
	default:
		return errors.Wrapf(ErrInvalidCfg, "unknown node type %s", cfg.NodeType)
	}
	return nil
}

// ValidateDispatcher validates the dispatcher configs
func ValidateDispatcher(cfg *Config) error {
	if cfg.Dispatcher.EventChanSize <= 0 {
		return errors.Wrapf(ErrInvalidCfg, "dispatcher event chan size should be greater than 0")
	}
	return nil
}

// ValidateRollDPoS validates the roll-DPoS configs
func ValidateRollDPoS(cfg *Config) error {
	if cfg.Consensus.Scheme == RollDPoSScheme && cfg.Consensus.RollDPoS.EventChanSize <= 0 {
		return errors.Wrapf(ErrInvalidCfg, "roll-DPoS event chan size should be greater than 0")
	}
	return nil
}

// ValidateExplorer validates the explorer configs
func ValidateExplorer(cfg *Config) error {
	if cfg.Explorer.Enabled && cfg.Explorer.TpsWindow <= 0 {
		return errors.Wrapf(ErrInvalidCfg, "tps window is not a positive integer when the explorer is enabled")
	}
	return nil
}

// ValidateNetwork validates the network configs
func ValidateNetwork(cfg *Config) error {
	if !cfg.Network.PeerDiscovery && cfg.Network.TopologyPath == "" {
		return errors.Wrap(ErrInvalidCfg, "either peer discover should be enabled or a topology should be given")
	}
	return nil
}

// ValidateDelegate validates the delegate configs
func ValidateDelegate(cfg *Config) error {
	if cfg.Delegate.RollNum > uint(len(cfg.Delegate.Addrs)) {
		return errors.Wrap(ErrInvalidCfg, "rolling delegates number is greater than total configured delegates")
	}
	return nil
}

// ValidateActPool validates the given config
func ValidateActPool(cfg *Config) error {
	maxNumActPerPool := cfg.ActPool.MaxNumActPerPool
	maxNumActPerAcct := cfg.ActPool.MaxNumActPerAcct
	if maxNumActPerPool <= 0 || maxNumActPerAcct <= 0 {
		return errors.Wrap(
			ErrInvalidCfg,
			"maximum number of actions per pool or per account cannot be zero or negative",
		)
	}
	if maxNumActPerPool < maxNumActPerAcct {
		return errors.Wrap(
			ErrInvalidCfg,
			"maximum number of actions per pool cannot be less than maximum number of actions per account",
		)
	}
	return nil
}

// DoNotValidate validates the given config
func DoNotValidate(cfg *Config) error { return nil }
