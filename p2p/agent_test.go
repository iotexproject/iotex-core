// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package p2p

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestBroadcast(t *testing.T) {
	ctx := context.Background()
	n := 10
	agents := make([]*Agent, 0)
	defer func() {
		var err error
		for _, agent := range agents {
			err = agent.Stop(ctx)
		}
		require.NoError(t, err)
	}()
	counts := make(map[uint8]int)
	var mutex sync.Mutex
	for i := 0; i < n; i++ {
		cfg := config.Network{Host: "127.0.0.1", Port: testutil.RandomPort()}
		if i > 0 {
			cfg.BootstrapNodes = []string{agents[0].Self().String()}
		}
		agent := NewAgent(
			cfg,
			func(_ uint32, msg proto.Message, _ chan bool) {
				mutex.Lock()
				defer mutex.Unlock()
				testMsg, ok := msg.(*iproto.TestPayload)
				require.True(t, ok)
				var idx uint8 = testMsg.MsgBody[0]
				if _, ok = counts[idx]; ok {
					counts[idx]++
				} else {
					counts[idx] = 1
				}
			},
			func(_ uint32, _ net.Addr, _ proto.Message, _ chan bool) {},
		)
		agents = append(agents, agent)
		require.NoError(t, agents[i].Start(ctx))
	}

	for i := 0; i < n; i++ {
		require.NoError(t, agents[i].Broadcast(WitContext(ctx, Context{ChainID: 1}), &iproto.TestPayload{
			MsgBody: []byte{uint8(i)},
		}))
		require.NoError(t, testutil.WaitUntil(100*time.Millisecond, 10*time.Second, func() (bool, error) {
			return counts[uint8(i)] == n, nil
		}))
	}
}

func TestUnicast(t *testing.T) {
	ctx := context.Background()
	n := 10
	agents := make([]*Agent, 0)
	defer func() {
		var err error
		for _, agent := range agents {
			err = agent.Stop(ctx)
		}
		require.NoError(t, err)
	}()
	counts := make(map[uint8]int)
	var mutex sync.Mutex
	for i := 0; i < n; i++ {
		cfg := config.Network{Host: "127.0.0.1", Port: testutil.RandomPort()}
		if i > 0 {
			cfg.BootstrapNodes = []string{agents[0].Self().String()}
		}
		agent := NewAgent(
			cfg,
			func(_ uint32, _ proto.Message, _ chan bool) {
			},
			func(_ uint32, _ net.Addr, msg proto.Message, _ chan bool) {
				mutex.Lock()
				defer mutex.Unlock()
				testMsg, ok := msg.(*iproto.TestPayload)
				require.True(t, ok)
				var idx uint8 = testMsg.MsgBody[0]
				if _, ok = counts[idx]; ok {
					counts[idx]++
				} else {
					counts[idx] = 1
				}
			},
		)
		agents = append(agents, agent)
		require.NoError(t, agents[i].Start(ctx))
	}

	for i := 0; i < n; i++ {
		neighbors := agents[i].Neighbors()
		require.Equal(t, n-1, len(neighbors))
		for _, neighbor := range neighbors {
			require.NoError(t, agents[i].Unicast(WitContext(ctx, Context{ChainID: 1}), neighbor, &iproto.TestPayload{
				MsgBody: []byte{uint8(i)},
			}))
		}
		require.NoError(t, testutil.WaitUntil(100*time.Millisecond, 10*time.Second, func() (bool, error) {
			return counts[uint8(i)] == n-1, nil
		}))
	}
}
