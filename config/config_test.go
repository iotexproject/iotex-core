// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/keepalive"
	"gopkg.in/yaml.v2"
)

func TestLoadTestConfig(t *testing.T) {
	config1 := LoadTestConfig()
	configStr, err := yaml.Marshal(config1)
	assert.Nil(t, err)
	path := "/tmp/config_" + strconv.Itoa(rand.Int()) + ".yaml"
	ioutil.WriteFile(path, configStr, 0666)

	defer func() {
		if os.Remove(path) != nil {
			assert.Fail(t, "Error when deleting the test file")
		}
	}()

	config2, err := LoadConfigWithPath(path)
	assert.Nil(t, err)
	assert.NotNil(t, config2)
	assert.Equal(t, config1, config2)
}

func TestLoadProdConfig(t *testing.T) {
	config, err := LoadConfigWithPath("../config.yaml")
	assert.Nil(t, err)
	assert.NotNil(t, config)
	assert.NotEmpty(t, config.Chain.ChainDBPath)
	assert.NotEmpty(t, config.Chain.MinerAddr)
}

func TestValidateConfig(t *testing.T) {
	cfg := LoadTestConfig()
	cfg.Chain.MinerAddr = "invalid_address"
	err := validateConfig(cfg)
	assert.NotNil(t, err)
	assert.Equal(t, "invalid miner's address", err.Error())

	cfg = LoadTestConfig()
	cfg.Chain.MinerAddr = ""
	cfg.NodeType = "invalid_type"
	err = validateConfig(cfg)
	assert.NotNil(t, err)
	assert.Equal(t, "unknown node type invalid_type", err.Error())

	cfg = LoadTestConfig()
	cfg.Network.PeerDiscovery = false
	err = validateConfig(cfg)
	assert.NotNil(t, err)
	assert.Equal(t, "either peer discover should be enabled or a topology should be given", err.Error())

	cfg = LoadTestConfig()
	cfg.NodeType = FullNodeType
	cfg.Consensus.Scheme = "RDPOS"
	err = validateConfig(cfg)
	assert.NotNil(t, err)
	assert.Equal(t, "consensus scheme of fullnode should be NOOP", err.Error())

	cfg.NodeType = LightweightType
	err = validateConfig(cfg)
	assert.NotNil(t, err)
	assert.Equal(t, "consensus scheme of lightweight node should be NOOP", err.Error())
}

func LoadTestConfig() *Config {
	return &Config{
		NodeType: FullNodeType,
		Network: Network{
			MsgLogsCleaningInterval: 2 * time.Second,
			MsgLogRetention:         10 * time.Second,
			HealthCheckInterval:     time.Second,
			SilentInterval:          5 * time.Second,
			PeerMaintainerInterval:  time.Second,
			NumPeersLowerBound:      5,
			NumPeersUpperBound:      5,
			AllowMultiConnsPerIP:    false,
			PingInterval:            time.Second,
			RateLimitEnabled:        true,
			RateLimitPerSec:         5,
			RateLimitWindowSize:     60 * time.Second,
			BootstrapNodes:          []string{},
			TLSEnabled:              false,
			CACrtPath:               "",
			PeerCrtPath:             "",
			PeerKeyPath:             "",
			KLClientParams:          keepalive.ClientParameters{Time: 60 * time.Second},
			KLServerParams:          keepalive.ServerParameters{Time: 60 * time.Second},
			KLPolicy:                keepalive.EnforcementPolicy{MinTime: 30 * time.Second},
			MaxMsgSize:              1024 * 1024 * 10,
			PeerDiscovery:           true,
			TopologyPath:            "",
		},
		Chain: Chain{
			ChainDBPath: "./a/fake/path",
		},
		Consensus: Consensus{
			Scheme: "NOOP",
		},
		Delegate: Delegate{
			Addrs: []string{"127.0.0.1:10001"},
		},
	}
}

func TestLoadTestTopology(t *testing.T) {
	topology1 := LoadTestTopology()
	topologyStr, err := yaml.Marshal(topology1)
	assert.Nil(t, err)
	path := "/tmp/topology_" + strconv.Itoa(rand.Int()) + ".yaml"
	ioutil.WriteFile(path, topologyStr, 0666)

	defer func() {
		if os.Remove(path) != nil {
			assert.Fail(t, "Error when deleting the test file")
		}
	}()

	topology2, err := LoadTopology(path)
	assert.Nil(t, err)
	assert.NotNil(t, topology2)
	assert.Equal(t, topology1, topology2)
}

func LoadTestTopology() *Topology {
	return &Topology{
		NeighborList: map[string][]string{
			"127.0.0.1:10001": {"127.0.0.1:10002", "127.0.0.1:10003", "127.0.0.1:10004"},
			"127.0.0.1:10002": {"127.0.0.1:10001", "127.0.0.1:10003", "127.0.0.1:10004"},
			"127.0.0.1:10003": {"127.0.0.1:10001", "127.0.0.1:10002", "127.0.0.1:10004"},
			"127.0.0.1:10004": {"127.0.0.1:10001", "127.0.0.1:10002", "127.0.0.1:10003"},
		},
	}
}
