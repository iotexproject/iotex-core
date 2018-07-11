// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package network

import (
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
	"github.com/iotexproject/iotex-core/network/node"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/test/util"
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
			TTL:                     3,
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

func (d *MockDispatcher) Start(_ context.Context) error {
	return nil
}

func (d *MockDispatcher) Stop(_ context.Context) error {
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
		node.Start(ctx)
		nodes = append(nodes, node)
	}

	defer func() {
		for _, node := range nodes {
			node.Stop(ctx)
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
	ctx := context.Background()
	dp1 := &MockDispatcher2{T: t}
	addr1 := randomAddress()
	addr2 := randomAddress()
	p1 := NewOverlay(LoadTestConfig(addr1, true))
	p1.AttachDispatcher(dp1)
	p1.Start(ctx)
	dp2 := &MockDispatcher2{T: t}
	p2 := NewOverlay(LoadTestConfig(addr2, true))
	p2.AttachDispatcher(dp2)
	p2.Start(ctx)

	defer func() {
		p1.Stop(ctx)
		p2.Stop(ctx)
	}()

	// P1 tell Tx Msg
	p1.Tell(&node.Node{Addr: addr2}, &iproto.TxPb{Version: uint32(12345678)})
	// P2 tell Tx Msg
	p2.Tell(&node.Node{Addr: addr1}, &iproto.TxPb{Version: uint32(87654321)})

	err := util.WaitUntil(10*time.Millisecond, 5*time.Second, func() (bool, error) {
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

func TestOneConnPerIP(t *testing.T) {
	ctx := context.Background()
	dp1 := &MockDispatcher2{T: t}
	addr1 := randomAddress()
	addr2 := randomAddress()
	addr3 := randomAddress()
	p1 := NewOverlay(LoadTestConfig(addr1, false))
	p1.AttachDispatcher(dp1)
	p1.Start(ctx)
	dp2 := &MockDispatcher2{T: t}
	p2 := NewOverlay(LoadTestConfig(addr2, false))
	p2.AttachDispatcher(dp2)
	p2.Start(ctx)
	dp3 := &MockDispatcher2{T: t}
	p3 := NewOverlay(LoadTestConfig(addr3, false))
	p3.AttachDispatcher(dp3)
	p3.Start(ctx)

	defer func() {
		p1.Stop(ctx)
		p2.Stop(ctx)
		p3.Stop(ctx)
	}()

	err := util.WaitUntil(10*time.Millisecond, 5*time.Second, func() (bool, error) {
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
			addr1: []string{addr2, addr3, addr4},
			addr2: []string{addr1, addr3, addr4},
			addr3: []string{addr1, addr2, addr4},
			addr4: []string{addr1, addr2, addr3},
		},
	}
	topologyStr, err := yaml.Marshal(topology)
	assert.Nil(t, err)
	path := "/tmp/topology_" + strconv.Itoa(rand.Int()) + ".yaml"
	ioutil.WriteFile(path, topologyStr, 0666)

	nodes := make([]*IotxOverlay, 4)
	for i := 1; i <= 4; i++ {
		config := LoadTestConfig(addresses[i-1], true)
		config.PeerDiscovery = false
		config.TopologyPath = path
		dp := &MockDispatcher{}
		node := NewOverlay(config)
		node.AttachDispatcher(dp)
		node.Start(ctx)
		nodes[i-1] = node
	}

	defer func() {
		for _, node := range nodes {
			node.Stop(ctx)
		}
		if os.Remove(path) != nil {
			assert.Fail(t, "Error when deleting the test file")
		}
	}()

	err = util.WaitUntil(10*time.Millisecond, 5*time.Second, func() (bool, error) {
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
	p1.Start(ctx)
	c2 := make(chan bool)
	d2 := &MockDispatcher3{C: c2}
	p2 := NewOverlay(cfg2)
	p2.AttachDispatcher(d2)
	p2.Start(ctx)

	defer func() {
		p1.Stop(ctx)
		p2.Stop(ctx)
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
					p1.Tell(&node.Node{Addr: "127.0.0.1:10002"}, &iproto.TestPayload{MsgBody: bytes})
				} else {
					p1.Broadcast(&iproto.TestPayload{MsgBody: bytes})
				}
				<-c2
			}
		})
	} else {
		for i := 0; i < b.N; i++ {
			if tell {
				p1.Tell(&node.Node{Addr: "127.0.0.1:10002"}, &iproto.TestPayload{MsgBody: bytes})
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
