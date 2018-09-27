// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package network

import (
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"net"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/network/node"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/testutil"
)

func LoadTestConfig(addr string, allowMultiConnsPerHost bool) *config.Network {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		host = "127.0.0.1"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		port = 0
	}
	config := config.Config{
		NodeType: config.DelegateType,
		Network: config.Network{
			Host: host,
			Port: port,
			MsgLogsCleaningInterval: 2 * time.Second,
			MsgLogRetention:         10 * time.Second,
			HealthCheckInterval:     time.Second,
			SilentInterval:          5 * time.Second,
			PeerMaintainerInterval:  time.Second,
			NumPeersLowerBound:      5,
			NumPeersUpperBound:      5,
			AllowMultiConnsPerHost:  allowMultiConnsPerHost,
			RateLimitEnabled:        false,
			PingInterval:            time.Second,
			BootstrapNodes:          []string{"127.0.0.1:10001", "127.0.0.1:10002"},
			MaxMsgSize:              1024 * 1024 * 10,
			PeerDiscovery:           true,
			TTL:                     3,
		},
	}
	return &config.Network
}

func LoadTestConfigWithTLSEnabled(addr string, allowMultiConnsPerHost bool) *config.Network {
	cfg := LoadTestConfig(addr, allowMultiConnsPerHost)
	cfg.TLSEnabled = true
	cfg.CACrtPath = "../test/assets/ssl/iotex.io.crt"
	cfg.PeerCrtPath = "../test/assets/ssl/127.0.0.1.crt"
	cfg.PeerKeyPath = "../test/assets/ssl/127.0.0.1.key"
	return cfg
}

type MockDispatcher struct {
}

func (d *MockDispatcher) AddSubscriber(uint32, dispatcher.Subscriber) {}

func (d *MockDispatcher) Start(_ context.Context) error {
	return nil
}

func (d *MockDispatcher) Stop(_ context.Context) error {
	return nil
}

func (d *MockDispatcher) HandleBroadcast(uint32, proto.Message, chan bool) {
}

func (d *MockDispatcher) HandleTell(uint32, net.Addr, proto.Message, chan bool) {
}

type MockDispatcher1 struct {
	MockDispatcher
	Count uint32
}

func (d1 *MockDispatcher1) AddSubscriber(uint32, dispatcher.Subscriber) {}

func (d1 *MockDispatcher1) HandleBroadcast(uint32, proto.Message, chan bool) {
	d1.Count++
}

func TestOverlay(t *testing.T) {
	ctx := context.Background()
	if testing.Short() {
		t.Skip("Skipping the IotxOverlay test in short mode.")
	}
	size := 10
	dps := []*MockDispatcher1{}
	nodes := []*IotxOverlay{}
	for i := 0; i < size; i++ {
		dp := &MockDispatcher1{}
		dps = append(dps, dp)
		var config *config.Network
		if i == 0 {
			config = LoadTestConfig("127.0.0.1:10001", true)
		} else if i == 1 {
			config = LoadTestConfig("127.0.0.1:10002", true)
		} else {
			config = LoadTestConfig("", true)
		}
		node := NewOverlay(config)
		node.AttachDispatcher(dp)
		err := node.Start(ctx)
		assert.NoError(t, err)
		nodes = append(nodes, node)
	}

	defer func() {
		for _, node := range nodes {
			err := node.Stop(ctx)
			assert.NoError(t, err)
		}
	}()

	time.Sleep(10 * time.Second)
	for i := 0; i < size; i++ {
		assert.True(t, LenSyncMap(nodes[i].PM.Peers) >= nodes[i].PM.NumPeersLowerBound)
	}

	err := nodes[0].Broadcast(config.Default.Chain.ID, &iproto.ActionPb{})
	assert.NoError(t, err)
	time.Sleep(5 * time.Second)
	for i, dp := range dps {
		if i == 0 {
			assert.Equal(t, uint32(0), dp.Count)
		} else {
			assert.Equal(t, uint32(1), dp.Count)
		}
	}
}

type MockDispatcher2 struct {
	MockDispatcher
	T     *testing.T
	Count uint32
}

func (d2 *MockDispatcher2) AddSubscriber(uint32, dispatcher.Subscriber) {}

func (d2 *MockDispatcher2) HandleTell(chainID uint32, sender net.Addr, message proto.Message, done chan bool) {
	// Handle Tx Msg
	msgType, err := iproto.GetTypeFromProtoMsg(message)
	/*
		switch (msgType) {
		case iproto.MsgTxProtoMsgType:
			break
		default:
			break
		}
	*/
	assert.True(d2.T, strings.HasPrefix(sender.Network(), "tcp"))
	assert.True(d2.T, strings.HasPrefix(sender.String(), "127.0.0.1"))
	assert.Nil(d2.T, err)
	assert.Equal(d2.T, iproto.MsgActionType, msgType)
	d2.Count++
}

func TestTell(t *testing.T) {
	ctx := context.Background()
	dp1 := &MockDispatcher2{T: t}
	addr1 := randomAddress()
	addr2 := randomAddress()
	p1 := NewOverlay(LoadTestConfig(addr1, true))
	p1.AttachDispatcher(dp1)
	err := p1.Start(ctx)
	assert.NoError(t, err)
	dp2 := &MockDispatcher2{T: t}
	p2 := NewOverlay(LoadTestConfig(addr2, true))
	p2.AttachDispatcher(dp2)
	err = p2.Start(ctx)
	assert.NoError(t, err)

	defer func() {
		err := p1.Stop(ctx)
		assert.NoError(t, err)
		err = p2.Stop(ctx)
		assert.NoError(t, err)
	}()

	// P1 tell Tx Msg
	err = p1.Tell(config.Default.Chain.ID, &node.Node{Addr: addr2}, &iproto.ActionPb{})
	assert.NoError(t, err)
	// P2 tell Tx Msg
	err = p2.Tell(config.Default.Chain.ID, &node.Node{Addr: addr1}, &iproto.ActionPb{})
	assert.NoError(t, err)

	err = testutil.WaitUntil(10*time.Millisecond, 5*time.Second, func() (bool, error) {
		if dp2.Count != uint32(1) {
			return false, nil
		}
		if dp1.Count != uint32(1) {
			return false, nil
		}
		return true, nil
	})
	require.Nil(t, err)
}

func TestOneConnPerHost(t *testing.T) {
	ctx := context.Background()
	dp1 := &MockDispatcher2{T: t}
	addr1 := randomAddress()
	addr2 := randomAddress()
	addr3 := randomAddress()
	p1 := NewOverlay(LoadTestConfig(addr1, false))
	p1.AttachDispatcher(dp1)
	err := p1.Start(ctx)
	require.Nil(t, err)
	dp2 := &MockDispatcher2{T: t}
	p2 := NewOverlay(LoadTestConfig(addr2, false))
	p2.AttachDispatcher(dp2)
	err = p2.Start(ctx)
	assert.NoError(t, err)
	dp3 := &MockDispatcher2{T: t}
	p3 := NewOverlay(LoadTestConfig(addr3, false))
	p3.AttachDispatcher(dp3)
	err = p3.Start(ctx)
	assert.NoError(t, err)

	defer func() {
		err := p1.Stop(ctx)
		assert.NoError(t, err)
		err = p2.Stop(ctx)
		assert.NoError(t, err)
		err = p3.Stop(ctx)
		assert.NoError(t, err)
	}()

	err = testutil.WaitUntil(10*time.Millisecond, 5*time.Second, func() (bool, error) {
		if uint(1) != LenSyncMap(p1.PM.Peers) {
			return false, nil
		}
		if uint(1) != LenSyncMap(p2.PM.Peers) {
			return false, nil
		}
		if uint(1) != LenSyncMap(p3.PM.Peers) {
			return false, nil
		}
		return true, nil
	})
	require.Nil(t, err)
}

func TestConfigBasedTopology(t *testing.T) {
	ctx := context.Background()
	addr1 := randomAddress()
	addr2 := randomAddress()
	addr3 := randomAddress()
	addr4 := randomAddress()
	addresses := []string{addr1, addr2, addr3, addr4}
	topology := Topology{
		NeighborList: map[string][]string{
			addr1: {addr2, addr3, addr4},
			addr2: {addr1, addr3, addr4},
			addr3: {addr1, addr2, addr4},
			addr4: {addr1, addr2, addr3},
		},
	}
	topologyStr, err := yaml.Marshal(topology)
	assert.Nil(t, err)
	path := "/tmp/topology_" + strconv.Itoa(rand.Int()) + ".yaml"
	err = ioutil.WriteFile(path, topologyStr, 0666)
	assert.NoError(t, err)

	nodes := make([]*IotxOverlay, 4)
	for i := 1; i <= 4; i++ {
		config := LoadTestConfig(addresses[i-1], true)
		config.PeerDiscovery = false
		config.TopologyPath = path
		dp := &MockDispatcher{}
		node := NewOverlay(config)
		node.AttachDispatcher(dp)
		err = node.Start(ctx)
		assert.NoError(t, err)
		nodes[i-1] = node
	}

	defer func() {
		for _, node := range nodes {
			err := node.Stop(ctx)
			assert.NoError(t, err)
		}
		if os.Remove(path) != nil {
			assert.Fail(t, "Error when deleting the test file")
		}
	}()

	err = testutil.WaitUntil(10*time.Millisecond, 5*time.Second, func() (bool, error) {
		for _, node := range nodes {
			if uint(3) != LenSyncMap(node.PM.Peers) {
				return false, nil
			}
			addrs := make([]string, 0)
			node.PM.Peers.Range(func(key, value interface{}) bool {
				addrs = append(addrs, key.(string))
				return true
			})
			sort.Strings(addrs)
			sort.Strings(topology.NeighborList[node.RPC.String()])
			if !reflect.DeepEqual(topology.NeighborList[node.RPC.String()], addrs) {
				return false, nil
			}
		}
		return true, nil
	})
	require.Nil(t, err)
}

func TestRandomizePeerList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TestRandomizePeerList in short mode.")
	}

	ctx := context.Background()
	size := 10
	var dps []*MockDispatcher1
	var nodes []*IotxOverlay
	for i := 0; i < size; i++ {
		dp := &MockDispatcher1{}
		dps = append(dps, dp)
		cfg := LoadTestConfig(fmt.Sprintf("127.0.0.1:1000%d", i), true)
		require.NotNil(t, cfg)
		cfg.NumPeersLowerBound = 4
		cfg.NumPeersUpperBound = 4
		cfg.PeerForceDisconnectionRoundInterval = 1
		node := NewOverlay(cfg)
		node.AttachDispatcher(dp)
		require.NoError(t, node.Start(ctx))
		nodes = append(nodes, node)
	}
	defer func() {
		for _, n := range nodes {
			assert.NoError(t, n.Stop(ctx))
		}
	}()

	// Sleep for neighbors to be fully shuffled
	time.Sleep(5 * time.Second)

	err := nodes[0].Broadcast(config.Default.Chain.ID, &iproto.ActionPb{})
	require.Nil(t, err)
	time.Sleep(5 * time.Second)
	testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		for i := 0; i < size; i++ {
			if i == 0 {
				return uint32(0) != dps[i].Count, nil
			}
			return uint32(1) != dps[i].Count, nil
		}
		return true, nil
	})

}

type MockDispatcher3 struct {
	MockDispatcher
	C chan bool
}

func (d3 *MockDispatcher3) HandleTell(uint32, net.Addr, proto.Message, chan bool) {
	d3.C <- true
}

func (d3 *MockDispatcher3) HandleBroadcast(uint32, proto.Message, chan bool) {
	d3.C <- true
}

func runBenchmarkOp(tell bool, size int, parallel bool, tls bool, b *testing.B) {
	ctx := context.Background()
	var cfg1, cfg2 *config.Network
	if tls {
		cfg1 = LoadTestConfigWithTLSEnabled("127.0.0.1:10001", true)
		cfg2 = LoadTestConfigWithTLSEnabled("127.0.0.1:10002", true)
	} else {
		cfg1 = LoadTestConfig("127.0.0.1:10001", true)
		cfg2 = LoadTestConfig("127.0.0.1:10002", true)
	}
	c1 := make(chan bool)
	d1 := &MockDispatcher3{C: c1}
	p1 := NewOverlay(cfg1)
	p1.AttachDispatcher(d1)
	err := p1.Start(ctx)
	assert.NoError(b, err)
	c2 := make(chan bool)
	d2 := &MockDispatcher3{C: c2}
	p2 := NewOverlay(cfg2)
	p2.AttachDispatcher(d2)
	err = p2.Start(ctx)
	assert.NoError(b, err)
	chainID := config.Default.Chain.ID

	defer func() {
		err := p1.Stop(ctx)
		assert.NoError(b, err)
		err = p2.Stop(ctx)
		assert.NoError(b, err)
	}()

	time.Sleep(time.Second)

	bytes := make([]byte, size)
	for i := 0; i < size; i++ {
		bytes[i] = uint8(rand.Intn(math.MaxUint8))
	}
	b.ResetTimer()
	if parallel {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if tell {
					err := p1.Tell(chainID, &node.Node{Addr: "127.0.0.1:10002"}, &iproto.TestPayload{MsgBody: bytes})
					assert.NoError(b, err)
				} else {
					err := p1.Broadcast(chainID, &iproto.TestPayload{MsgBody: bytes})
					assert.NoError(b, err)
				}
				<-c2
			}
		})
	} else {
		for i := 0; i < b.N; i++ {
			if tell {
				err := p1.Tell(chainID, &node.Node{Addr: "127.0.0.1:10002"}, &iproto.TestPayload{MsgBody: bytes})
				assert.NoError(b, err)
			} else {
				err := p1.Broadcast(chainID, &iproto.TestPayload{MsgBody: bytes})
				assert.NoError(b, err)
			}
			<-c2
		}
	}
}

func generateBlockConfig() *map[string]int {
	return &map[string]int{
		"0K payload":   0,
		"1K payload":   1024,
		"10K payload":  1024 * 10,
		"100K payload": 1024 * 100,
		"1M payload":   1024 * 1024,
		"2M payload":   1024 * 1024 * 2,
		"5M payload":   1024 * 1024 * 5,
	}
}

func BenchmarkTell(b *testing.B) {
	for name, size := range *generateBlockConfig() {
		b.Run(name, func(b *testing.B) {
			runBenchmarkOp(true, size, false, false, b)
		})
	}
}

func BenchmarkSecureTell(b *testing.B) {
	for name, size := range *generateBlockConfig() {
		b.Run(name, func(b *testing.B) {
			runBenchmarkOp(true, size, false, true, b)
		})
	}
}

func BenchmarkParallelTell(b *testing.B) {
	for name, size := range *generateBlockConfig() {
		b.Run(name, func(b *testing.B) {
			runBenchmarkOp(true, size, true, false, b)
		})
	}
}

func BenchmarkParallelSecureTell(b *testing.B) {
	for name, size := range *generateBlockConfig() {
		b.Run(name, func(b *testing.B) {
			runBenchmarkOp(true, size, true, true, b)
		})
	}
}

func randomAddress() string {
	endPoint := rand.Intn(40000) + 10000
	return "127.0.0.1:" + strconv.Itoa(endPoint)
}

/*
func BenchmarkBroadcast(b *testing.B) {
	for name, size := range *generateBlockConfig() {
		b.Run(name, func(b *testing.B) {
			runBenchmarkOp(false, size, false, false, b)
		})
	}
}
*/
