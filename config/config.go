// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/golang/glog"
	"google.golang.org/grpc/keepalive"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/iotxaddress"
)

const (
	// DefaultConfigPath is the default config path
	DefaultConfigPath = "./config.yaml"
)

const (
	// DelegateType represents the delegate node type
	DelegateType = "delegate"
	// FullNodeType represents the full node type
	FullNodeType = "full_node"
	// LightweightType represents the lightweight type
	LightweightType = "lightweight"
)

// Network is the config struct for network package
type Network struct {
	Addr                    string
	MsgLogsCleaningInterval time.Duration
	MsgLogRetention         time.Duration
	HealthCheckInterval     time.Duration
	SilentInterval          time.Duration
	PeerMaintainerInterval  time.Duration
	AllowMultiConnsPerIP    bool
	NumPeersLowerBound      uint
	NumPeersUpperBound      uint
	PingInterval            time.Duration
	RateLimitEnabled        bool
	RateLimitPerSec         uint64
	RateLimitWindowSize     time.Duration
	BootstrapNodes          []string
	TLSEnabled              bool
	CACrtPath               string
	PeerCrtPath             string
	PeerKeyPath             string
	KLClientParams          keepalive.ClientParameters
	KLServerParams          keepalive.ServerParameters
	KLPolicy                keepalive.EnforcementPolicy
	MaxMsgSize              int
	PeerDiscovery           bool
	TopologyPath            string
}

// Chain is the config struct for blockchain package
type Chain struct {
	ChainDBPath string

	// MinerAddr is an address where the block rewards will be sent to.
	MinerAddr string
}

// Consensus is the config struct for consensus package
type Consensus struct {
	// There are three schemes that are supported:
	// RDPOS -- Randomized Delegated Proof of Stake
	// STANDALONE -- The node creates a block periodically regardless of others (if there is any)
	// NOOP -- The node does not create only block
	Scheme                string
	RDPoS                 RDPoS
	BlockCreationInterval time.Duration
}

// RDPoS is the config struct for RDPoS consensus package
type RDPoS struct {
	ProposerRotation  ProposerRotation
	UnmatchedEventTTL time.Duration
	AcceptPropose     AcceptPropose
	AcceptPrevote     AcceptPrevote
	AcceptVote        AcceptVote
}

// ProposerRotation is the RDPoS ProposerRotation config
type ProposerRotation struct {
	// Interval determines how long to propose another round of RDPoS.
	Interval time.Duration
	// Enabled flags whether we periodically rotate the proposer and trigger a new round of RDPoS
	Enabled bool
}

// AcceptPropose is the RDPoS AcceptPropose config
type AcceptPropose struct {
	// TTL is the time the state machine will wait for the AcceptPropose state.
	// Once timeout, it will move to the next state.
	TTL time.Duration
}

// AcceptPrevote is the RDPoS AcceptPrevote config
type AcceptPrevote struct {
	// TTL is the time the state machine will wait for the AcceptPrevote state.
	// Once timeout, it will move to the next state.
	TTL time.Duration
}

// AcceptVote is the RDPoS AcceptVote config
type AcceptVote struct {
	// TTL is the time the state machine will wait for the AcceptVote state.
	// Once timeout, it will move to the next state.
	TTL time.Duration
}

// Delegate is the delegate config
type Delegate struct {
	Addrs []string
}

// RPC is the chain service config
type RPC struct {
	Port string
}

// Config is the root config struct, each package's config should be put as its sub struct
type Config struct {
	NodeType  string
	Network   Network
	Chain     Chain
	Consensus Consensus
	Delegate  Delegate
	RPC       RPC
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

// LoadConfig loads the config instance from the default config path
func LoadConfig() (*Config, error) {
	return LoadConfigWithPath(DefaultConfigPath)
}

// LoadConfigWithPath loads the config instance and validates fields
func LoadConfigWithPath(path string) (*Config, error) {
	return loadConfigWithPathInternal(path, true)
}

// LoadConfigWithPathWithoutValidation loads the config instance but doesn't validate fields
func LoadConfigWithPathWithoutValidation(path string) (*Config, error) {
	return loadConfigWithPathInternal(path, false)
}

// loadConfigWithPathInternal loads the config instance. If validation is true, the function will check if the fields
// are valid or not.
func loadConfigWithPathInternal(path string, validate bool) (*Config, error) {
	configBytes, err := ioutil.ReadFile(path)
	if err != nil {
		glog.Errorf("Error when reading the config file: %v\n", err)
		return nil, err
	}

	config := Config{}
	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		glog.Errorf("Error when decoding the config file: %v\n", err)
		return nil, err
	}

	if validate {
		if err = validateConfig(&config); err != nil {
			glog.Errorf("Error when validating config: %v\n", err)
			return nil, err
		}
	}
	return &config, nil
}

// validateConfig validates the given config
func validateConfig(cfg *Config) error {
	// Validate miner's address
	if len(cfg.Chain.MinerAddr) > 0 && !iotxaddress.ValidateAddress(cfg.Chain.MinerAddr) {
		return fmt.Errorf("invalid miner's address")
	}

	// Validate node type
	switch cfg.NodeType {
	case DelegateType:
		break
	case FullNodeType:
		if cfg.Consensus.Scheme != "NOOP" {
			return fmt.Errorf("consensus scheme of fullnode should be NOOP")
		}
	case LightweightType:
		if cfg.Consensus.Scheme != "NOOP" {
			return fmt.Errorf("consensus scheme of lightweight node should be NOOP")
		}
	default:
		return fmt.Errorf("unknown node type %s", cfg.NodeType)
	}

	if !cfg.Network.PeerDiscovery && cfg.Network.TopologyPath == "" {
		return fmt.Errorf("either peer discover should be enabled or a topology should be given")
	}
	return nil
}

// Topology is the neighbor list for each node. This is used for generating the P2P network in a given topology. Note
// that the list contains the outgoing connections.
type Topology struct {
	NeighborList map[string][]string
}

// LoadTopology loads the topology struct from the given yaml file
func LoadTopology(path string) (*Topology, error) {
	topologyBytes, err := ioutil.ReadFile(path)
	if err != nil {
		glog.Errorf("Error when reading the topology file: %v\n", err)
		return nil, err
	}

	topology := Topology{}
	err = yaml.Unmarshal(topologyBytes, &topology)
	if err != nil {
		glog.Errorf("Error when decoding the topology file: %v\n", err)
		return nil, err
	}

	return &topology, nil
}
