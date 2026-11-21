// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package p2p

import (
	"context"
	"testing"
	"time"

	p2plib "github.com/iotexproject/go-p2p"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/testutil"
)

// Pin the dependency contract used by agent.Start and downstream BlockPeer:
// author -> relay -> receiver must identify the signed author, not the relay
// or the PeerId claimed inside the application envelope.
func TestGossipBroadcastIdentitySurvivesRelay(t *testing.T) {
	r := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	var chainID uint32 = 987
	newHost := func(key string) *p2plib.Host {
		chainID++
		h, err := p2plib.NewHost(ctx, p2plib.HostName("127.0.0.1"),
			p2plib.Port(testutil.RandomPort()), p2plib.MasterKey(key),
			p2plib.SecureIO(), p2plib.Gossip(), p2plib.DHTProtocolID(chainID))
		r.NoError(err)
		t.Cleanup(func() { r.NoError(h.Close()) })
		return h
	}
	author, relay, receiver := newHost("author"), newHost("relay"), newHost("receiver")
	const topic = "identity-through-relay"
	noop := func(context.Context, peer.ID, []byte) error { return nil }
	r.NoError(author.AddBroadcastPubSub(ctx, topic, nil, noop))
	r.NoError(relay.AddBroadcastPubSub(ctx, topic, nil, noop))
	type received struct {
		publisher peer.ID
		data      []byte
	}
	delivered := make(chan received, 32)
	hops := make(chan peer.ID, 32)
	validator := func(_ context.Context, _ peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
		select {
		case hops <- msg.ReceivedFrom:
		default:
		}
		return pubsub.ValidationAccept
	}
	r.NoError(receiver.AddBroadcastPubSub(ctx, topic, validator,
		func(_ context.Context, pid peer.ID, data []byte) error {
			select {
			case delivered <- received{pid, data}:
			default:
			}
			return nil
		}))
	// Separate DHT protocols prevent routing-table refreshes from adding a
	// direct author/receiver connection. Pubsub still shares the same topic.
	r.NoError(author.ConnectWithMultiaddr(ctx, relay.Addresses()[0]))
	r.NoError(receiver.ConnectWithMultiaddr(ctx, relay.Addresses()[0]))
	wire, err := proto.Marshal(&iotexrpc.BroadcastMsg{PeerId: receiver.HostIdentity()})
	r.NoError(err)
	var got received
	r.Eventually(func() bool {
		if err := author.Broadcast(ctx, topic, wire); err != nil {
			return false
		}
		select {
		case got = <-delivered:
			return true
		default:
			return false
		}
	}, 15*time.Second, 100*time.Millisecond)
	r.Equal(author.HostIdentity(), got.publisher.String())
	r.NotEqual(relay.HostIdentity(), got.publisher.String())
	select {
	case hop := <-hops:
		r.Equal(relay.HostIdentity(), hop.String())
	default:
		t.Fatal("message never reached the receiver validator")
	}
	var envelope iotexrpc.BroadcastMsg
	r.NoError(proto.Unmarshal(got.data, &envelope))
	r.Equal(receiver.HostIdentity(), envelope.PeerId)
	r.NotEqual(envelope.PeerId, got.publisher.String())
	r.Len(receiver.ConnectedPeers(), 1)
	r.Equal(relay.HostIdentity(), receiver.ConnectedPeers()[0].ID.String())
}
