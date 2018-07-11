// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package network

import (
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"

	pb "github.com/iotexproject/iotex-core/network/proto"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/test/mock/mock_dispatcher"
	"github.com/iotexproject/iotex-core/test/util"
)

func TestRpcPingPong(t *testing.T) {
	ctx := context.Background()
	config := LoadTestConfig("", true)
	o := &IotxOverlay{Config: config}
	o.PM = &PeerManager{Overlay: o, NumPeersLowerBound: 1, NumPeersUpperBound: 1}
	s := NewRPCServer(o)
	o.RPC = s
	s.Start(ctx)
	p := NewPeer(s.Network(), s.String())
	p.Connect(config)

	defer func() {
		p.Close()
		s.Stop(ctx)
	}()

	util.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) { return o.RPC.Started(), nil })

	pong, err := p.Ping(&pb.Ping{Nonce: uint64(4689), Addr: "127.0.0.1:10001"})
	assert.Nil(t, err)
	assert.NotNil(t, pong)
	assert.Equal(t, uint64(4689), pong.AckNonce)
	value, ok := o.PM.Peers.Load("127.0.0.1:10001")
	assert.True(t, ok)
	assert.NotNil(t, value)
	assert.True(t, "127.0.0.1:10001" == value.(*Peer).String())
}

func TestGetPeers(t *testing.T) {
	ctx := context.Background()
	config := LoadTestConfig("", true)
	o := &IotxOverlay{Config: config}
	o.PM = &PeerManager{Overlay: o}
	o.PM.Peers.Store("127.0.0.1:10001", NewTCPPeer("127.0.0.1:10001"))
	o.PM.Peers.Store("127.0.0.1:10002", NewTCPPeer("127.0.0.1:10002"))
	s := NewRPCServer(o)
	o.RPC = s
	s.Start(ctx)
	p := NewPeer(s.Network(), s.String())
	p.Connect(config)

	defer func() {
		p.Close()
		s.Stop(ctx)
	}()

	util.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) { return o.RPC.Started(), nil })

	res, err := p.GetPeers(&pb.GetPeersReq{Count: 1})
	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, 1, len(res.Addr))
	assert.True(t, res.Addr[0] == "127.0.0.1:10001" || res.Addr[0] == "127.0.0.1:10002")

	res, err = p.GetPeers(&pb.GetPeersReq{Count: 2})
	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, 2, len(res.Addr))
	assert.True(t, res.Addr[0] == "127.0.0.1:10001" || res.Addr[0] == "127.0.0.1:10002")
	assert.True(t, res.Addr[1] == "127.0.0.1:10001" || res.Addr[1] == "127.0.0.1:10002")
	assert.False(t, res.Addr[0] == res.Addr[1])

	res, err = p.GetPeers(&pb.GetPeersReq{Count: 3})
	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, 2, len(res.Addr))
	assert.True(t, res.Addr[0] == "127.0.0.1:10001" || res.Addr[0] == "127.0.0.1:10002")
	assert.True(t, res.Addr[1] == "127.0.0.1:10001" || res.Addr[1] == "127.0.0.1:10002")
	assert.False(t, res.Addr[0] == res.Addr[1])

}

func TestBroadcast(t *testing.T) {
	ctx := context.Background()
	config := LoadTestConfig("", true)
	o := &IotxOverlay{Config: config}
	o.PM = &PeerManager{Overlay: o}
	o.Gossip = &Gossip{Overlay: o}
	s := NewRPCServer(o)
	o.RPC = s
	s.Start(ctx)
	p := NewPeer(s.Network(), s.String())
	p.Connect(config)

	defer func() {
		p.Close()
		s.Stop(ctx)
	}()

	util.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) { return o.RPC.Started(), nil })

	txMsg := &iproto.TxPb{}
	b, _ := proto.Marshal(txMsg)
	res, err := p.BroadcastMsg(
		&pb.BroadcastReq{Header: iproto.MagicBroadcastMsgHeader, MsgType: iproto.MsgTxProtoMsgType, MsgBody: b})
	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, iproto.MagicBroadcastMsgHeader, res.Header)
}

func TestRPCTell(t *testing.T) {
	ctx := context.Background()
	mctrl := gomock.NewController(t)
	dp := mock_dispatcher.NewMockDispatcher(mctrl)
	dp.EXPECT().HandleTell(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	config := LoadTestConfig("", true)
	o := &IotxOverlay{Dispatcher: dp, Config: config}
	s := NewRPCServer(o)
	o.RPC = s
	s.Start(ctx)
	p := NewPeer(s.Network(), s.String())
	p.Connect(config)

	defer func() {
		p.Close()
		s.Stop(ctx)
		mctrl.Finish()
	}()

	util.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) { return o.RPC.Started(), nil })

	txMsg := &iproto.TxPb{}
	b, _ := proto.Marshal(txMsg)
	res, err := p.Tell(&pb.TellReq{Header: iproto.MagicBroadcastMsgHeader,
		Addr:    s.String(),
		MsgType: iproto.MsgTxProtoMsgType,
		MsgBody: b})
	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, iproto.MagicBroadcastMsgHeader, res.Header)
}

func TestRateLimit(t *testing.T) {
	ctx := context.Background()
	mctrl := gomock.NewController(t)
	dp := mock_dispatcher.NewMockDispatcher(mctrl)
	dp.EXPECT().HandleTell(gomock.Any(), gomock.Any(), gomock.Any()).Times(5)

	config := LoadTestConfig("", true)
	config.RateLimitEnabled = true
	config.RateLimitPerSec = 5
	config.RateLimitWindowSize = time.Second
	o := &IotxOverlay{Dispatcher: dp, Config: config}
	s := NewRPCServer(o)
	o.RPC = s
	s.Start(ctx)
	p := NewPeer(s.Network(), s.String())
	p.Connect(config)

	defer func() {
		p.Close()
		s.Stop(ctx)
		mctrl.Finish()
	}()

	util.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) { return o.RPC.Started(), nil })

	var res *pb.TellRes
	var err error
	for i := 0; i < 10; i++ {
		txMsg := &iproto.TxPb{}
		b, _ := proto.Marshal(txMsg)
		res, err = p.Tell(&pb.TellReq{Header: iproto.MagicBroadcastMsgHeader,
			Addr:    s.String(),
			MsgType: iproto.MsgTxProtoMsgType,
			MsgBody: b})
		if i < 5 {
			assert.Nil(t, err)
			assert.NotNil(t, res, i)
			assert.Equal(t, iproto.MagicBroadcastMsgHeader, res.Header)
		} else {
			assert.Nil(t, res)
			assert.NotNil(t, err)
			assert.True(t, strings.Contains(err.Error(), "sended requests too frequently"))
		}
	}
}

func TestSecureRpcPingPong(t *testing.T) {
	ctx := context.Background()
	config := LoadTestConfig("", true)
	config.TLSEnabled = true
	config.CACrtPath = "../test/assets/ssl/iotex.io.crt"
	config.PeerCrtPath = "../test/assets/ssl/127.0.0.1.crt"
	config.PeerKeyPath = "../test/assets/ssl/127.0.0.1.key"
	o := &IotxOverlay{Config: config}
	o.PM = &PeerManager{Overlay: o, NumPeersLowerBound: 1, NumPeersUpperBound: 1}
	s := NewRPCServer(o)
	o.RPC = s
	s.Start(ctx)
	p := NewPeer(s.Network(), s.String())
	p.Connect(config)

	defer func() {
		p.Close()
		s.Stop(ctx)
	}()

	util.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) { return o.RPC.Started(), nil })

	pong, err := p.Ping(&pb.Ping{Nonce: uint64(4689), Addr: "127.0.0.1:10001"})
	assert.Nil(t, err)
	assert.NotNil(t, pong)
	assert.Equal(t, uint64(4689), pong.AckNonce)
	value, ok := o.PM.Peers.Load("127.0.0.1:10001")
	assert.True(t, ok)
	assert.NotNil(t, value)
	assert.True(t, "127.0.0.1:10001" == value.(*Peer).String())
}

func TestKeepaliveParams(t *testing.T) {
	// This only verifies the config doesn't break connections
	ctx := context.Background()
	config := LoadTestConfig("", true)
	config.KLClientParams.Time = 50 * time.Millisecond
	config.KLClientParams.Timeout = 20 * time.Millisecond
	config.KLServerParams.Time = 50 * time.Second
	config.KLClientParams.Timeout = 20 * time.Millisecond
	config.KLPolicy.MinTime = 20 * time.Millisecond
	o := &IotxOverlay{Config: config}
	o.PM = &PeerManager{Overlay: o, NumPeersLowerBound: 1, NumPeersUpperBound: 1}
	s := NewRPCServer(o)
	o.RPC = s
	s.Start(ctx)
	p := NewPeer(s.Network(), s.String())
	p.Connect(config)

	defer func() {
		p.Close()
		s.Stop(ctx)
	}()

	util.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) { return o.RPC.Started(), nil })

	for i := 0; i < 5; i++ {
		time.Sleep(100 * time.Millisecond)
		pong, err := p.Ping(&pb.Ping{Nonce: uint64(4689), Addr: "127.0.0.1:10001"})
		assert.Nil(t, err)
		assert.NotNil(t, pong)
		assert.Equal(t, uint64(4689), pong.AckNonce)
		value, ok := o.PM.Peers.Load("127.0.0.1:10001")
		assert.True(t, ok)
		assert.NotNil(t, value)
		assert.True(t, "127.0.0.1:10001" == value.(*Peer).String())
	}
}
