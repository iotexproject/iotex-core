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
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"

	cm "github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/proto"
)

func LoadTestConfig(addr string, allowMultiConnsPerIP bool) *config.Network {
	config := config.Config{
		NodeType: config.DelegateType,
		Network: config.Network{
			Addr: addr,
			MsgLogsCleaningInterval: 2 * time.Second,
			MsgLogRetention:         10 * time.Second,
			HealthCheckInterval:     time.Second,
			SilentInterval:          5 * time.Second,
			PeerMaintainerInterval:  time.Second,
			NumPeersLowerBound:      5,
			NumPeersUpperBound:      5,
			AllowMultiConnsPerIP:    allowMultiConnsPerIP,
			RateLimitEnabled:        false,
			PingInterval:            time.Second,
			BootstrapNodes:          []string{"127.0.0.1:10001", "127.0.0.1:10002"},
			MaxMsgSize:              1024 * 1024 * 10,
			PeerDiscovery:           true,
		},
	}
	return &config.Network
}

func LoadTestConfigWithTLSEnabled(addr string, allowMultiConnsPerIP bool) *config.Network {
	cfg := LoadTestConfig(addr, allowMultiConnsPerIP)
	cfg.TLSEnabled = true
	cfg.CACrtPath = "../test/assets/ssl/iotex.io.crt"
	cfg.PeerCrtPath = "../test/assets/ssl/127.0.0.1.crt"
	cfg.PeerKeyPath = "../test/assets/ssl/127.0.0.1.key"
	return cfg
}

type MockDispatcher struct {
}

func (d *MockDispatcher) Start() error {
	return nil
}

func (d *MockDispatcher) Stop() error {
	return nil
}

func (d *MockDispatcher) HandleBroadcast(proto.Message, chan bool) {
}

func (d *MockDispatcher) HandleTell(net.Addr, proto.Message, chan bool) {
}

type MockDispatcher1 struct {
	MockDispatcher
	Count uint32
}

func (d1 *MockDispatcher1) HandleBroadcast(proto.Message, chan bool) {
	d1.Count++
}

func TestOverlay(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping the overlay test in short mode.")
	}
	size := 10
	dps := []*MockDispatcher1{}
	nodes := []*Overlay{}
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
		node.Init()
		node.Start()
		nodes = append(nodes, node)
	}

	defer func() {
		for _, node := range nodes {
			node.Stop()
		}
	}()

	time.Sleep(10 * time.Second)
	for i := 0; i < size; i++ {
		assert.True(t, LenSyncMap(nodes[i].PM.Peers) >= nodes[i].PM.NumPeersLowerBound)
	}

	nodes[0].Broadcast(&iproto.TxPb{})
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

func (d2 *MockDispatcher2) HandleTell(sender net.Addr, message proto.Message, done chan bool) {
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
	assert.Equal(d2.T, iproto.MsgTxProtoMsgType, msgType)
	d2.Count++
}

func TestTell(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping the overlay test in short mode.")
	}
	dp1 := &MockDispatcher2{T: t}
	p1 := NewOverlay(LoadTestConfig("127.0.0.1:10001", true))
	p1.AttachDispatcher(dp1)
	p1.Init()
	p1.Start()
	dp2 := &MockDispatcher2{T: t}
	p2 := NewOverlay(LoadTestConfig("127.0.0.1:10002", true))
	p2.AttachDispatcher(dp2)
	p2.Init()
	p2.Start()

	time.Sleep(2 * time.Second)

	defer func() {
		p1.Stop()
		p2.Stop()
	}()

	// P1 tell Tx Msg
	p1.Tell(&cm.Node{Addr: "127.0.0.1:10002"}, &iproto.TxPb{Version: uint32(12345678)})
	time.Sleep(time.Second)
	assert.Equal(t, uint32(1), dp2.Count)

	// P2 tell Tx Msg
	p2.Tell(&cm.Node{Addr: "127.0.0.1:10001"}, &iproto.TxPb{Version: uint32(87654321)})
	time.Sleep(time.Second)
	assert.Equal(t, uint32(1), dp1.Count)
}

func TestOneConnPerIP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping the overlay test in short mode.")
	}
	dp1 := &MockDispatcher2{T: t}
	p1 := NewOverlay(LoadTestConfig("127.0.0.1:10001", false))
	p1.AttachDispatcher(dp1)
	p1.Init()
	p1.Start()
	dp2 := &MockDispatcher2{T: t}
	p2 := NewOverlay(LoadTestConfig("127.0.0.1:10002", false))
	p2.AttachDispatcher(dp2)
	p2.Init()
	p2.Start()
	dp3 := &MockDispatcher2{T: t}
	p3 := NewOverlay(LoadTestConfig("127.0.0.1:10003", false))
	p3.AttachDispatcher(dp3)
	p3.Init()
	p3.Start()

	time.Sleep(2 * time.Second)

	defer func() {
		p1.Stop()
		p2.Stop()
		p3.Stop()
	}()

	assert.Equal(t, uint(1), LenSyncMap(p1.PM.Peers))
	assert.Equal(t, uint(1), LenSyncMap(p2.PM.Peers))
	assert.Equal(t, uint(1), LenSyncMap(p3.PM.Peers))
}

func TestConfigBasedTopology(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping the overlay test in short mode.")
	}
	topology := config.Topology{
		NeighborList: map[string][]string{
			"127.0.0.1:10001": []string{"127.0.0.1:10002", "127.0.0.1:10003", "127.0.0.1:10004"},
			"127.0.0.1:10002": []string{"127.0.0.1:10001", "127.0.0.1:10003", "127.0.0.1:10004"},
			"127.0.0.1:10003": []string{"127.0.0.1:10001", "127.0.0.1:10002", "127.0.0.1:10004"},
			"127.0.0.1:10004": []string{"127.0.0.1:10001", "127.0.0.1:10002", "127.0.0.1:10003"},
		},
	}
	topologyStr, err := yaml.Marshal(topology)
	assert.Nil(t, err)
	path := "/tmp/topology_" + strconv.Itoa(rand.Int()) + ".yaml"
	ioutil.WriteFile(path, topologyStr, 0666)

	nodes := make([]*Overlay, 4)
	for i := 1; i <= 4; i++ {
		config := LoadTestConfig(fmt.Sprintf("127.0.0.1:1000%d", i), true)
		config.PeerDiscovery = false
		config.TopologyPath = path
		dp := &MockDispatcher{}
		node := NewOverlay(config)
		node.AttachDispatcher(dp)
		node.Init()
		node.Start()
		nodes[i-1] = node
	}

	defer func() {
		for _, node := range nodes {
			node.Stop()
		}
		if os.Remove(path) != nil {
			assert.Fail(t, "Error when deleting the test file")
		}
	}()

	time.Sleep(2 * time.Second)

	for _, node := range nodes {
		assert.Equal(t, uint(3), LenSyncMap(node.PM.Peers))
		addrs := make([]string, 0)
		node.PM.Peers.Range(func(key, value interface{}) bool {
			addrs = append(addrs, key.(string))
			return true
		})
		sort.Strings(addrs)
		assert.Equal(t, topology.NeighborList[node.PRC.String()], addrs)
	}
}

type MockDispatcher3 struct {
	MockDispatcher
	C chan bool
}

func (d3 *MockDispatcher3) HandleTell(net.Addr, proto.Message, chan bool) {
	d3.C <- true
}

func (d3 *MockDispatcher3) HandleBroadcast(proto.Message, chan bool) {
	d3.C <- true
}

func runBenchmarkOp(tell bool, size int, parallel bool, tls bool, b *testing.B) {
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
	p1.Init()
	p1.Start()
	c2 := make(chan bool)
	d2 := &MockDispatcher3{C: c2}
	p2 := NewOverlay(cfg2)
	p2.AttachDispatcher(d2)
	p2.Init()
	p2.Start()

	defer func() {
		p1.Stop()
		p2.Stop()
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
					p1.Tell(&cm.Node{Addr: "127.0.0.1:10002"}, &iproto.TestPayload{MsgBody: bytes})
				} else {
					p1.Broadcast(&iproto.TestPayload{MsgBody: bytes})
				}
				<-c2
			}
		})
	} else {
		for i := 0; i < b.N; i++ {
			if tell {
				p1.Tell(&cm.Node{Addr: "127.0.0.1:10002"}, &iproto.TestPayload{MsgBody: bytes})
			} else {
				p1.Broadcast(&iproto.TestPayload{MsgBody: bytes})
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

/*
func BenchmarkBroadcast(b *testing.B) {
	for name, size := range *generateBlockConfig() {
		b.Run(name, func(b *testing.B) {
			runBenchmarkOp(false, size, false, false, b)
		})
	}
}
*/
