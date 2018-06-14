package blocksync

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core-internal/common"
	"github.com/iotexproject/iotex-core-internal/config"
	"github.com/iotexproject/iotex-core-internal/network"
	"github.com/iotexproject/iotex-core-internal/test/mock/mock_delegate"
	"net"
)

func TestSyncTaskInterval(t *testing.T) {
	assert := assert.New(t)

	interval := time.Duration(0)

	cfgLightWeight := &config.Config{
		NodeType: config.LightweightType,
	}
	lightWeight := SyncTaskInterval(cfgLightWeight)
	assert.Equal(interval, lightWeight)

	cfgDelegate := &config.Config{
		NodeType: config.DelegateType,
		BlockSync: config.BlockSync{
			Interval: interval,
		},
	}
	delegate := SyncTaskInterval(cfgDelegate)
	assert.Equal(interval, delegate)

	cfgFullNode := &config.Config{
		NodeType: config.FullNodeType,
		BlockSync: config.BlockSync{
			Interval: interval,
		},
	}
	interval <<= 2
	fullNode := SyncTaskInterval(cfgFullNode)
	assert.Equal(interval, fullNode)
}

func generateP2P() *network.Overlay {
	c := &config.Network{
		Addr: "127.0.0.1:10001",
		MsgLogsCleaningInterval: 2 * time.Second,
		MsgLogRetention:         10 * time.Second,
		HealthCheckInterval:     time.Second,
		SilentInterval:          5 * time.Second,
		PeerMaintainerInterval:  time.Second,
		NumPeersLowerBound:      5,
		NumPeersUpperBound:      5,
		AllowMultiConnsPerIP:    true,
		RateLimitEnabled:        false,
		PingInterval:            time.Second,
		BootstrapNodes:          []string{"127.0.0.1:10001", "127.0.0.1:10002"},
		MaxMsgSize:              1024 * 1024 * 10,
		PeerDiscovery:           true,
	}
	return network.NewOverlay(c)
}

func TestNewBlockSyncer(t *testing.T) {
	assert := assert.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mPool := mock_delegate.NewMockPool(ctrl)
	mPool.EXPECT().AllDelegates().Times(3).Return([]net.Addr{common.NewNode("", "123")}, nil)
	mPool.EXPECT().AnotherDelegate(gomock.Any()).Times(2).Return(common.NewNode("", "123"))

	p2p := generateP2P()

	// Lightweight
	cfgLightWeight := &config.Config{
		NodeType: config.LightweightType,
	}

	bsLightWeight := NewBlockSyncer(cfgLightWeight, nil, nil, nil, p2p, mPool)
	assert.Nil(bsLightWeight)

	// Delegate
	cfgDelegate := &config.Config{
		NodeType: config.DelegateType,
	}

	bsDelegate := NewBlockSyncer(cfgDelegate, nil, nil, nil, p2p, mPool)
	assert.Equal(bsDelegate.(*blockSyncer).fnd, "123")

	// FullNode
	cfgFullNode := &config.Config{
		NodeType: config.FullNodeType,
	}

	bsFullNode := NewBlockSyncer(cfgFullNode, nil, nil, nil, p2p, mPool)
	assert.Equal(bsFullNode.(*blockSyncer).fnd, "123")
}
