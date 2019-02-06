package p2p

import (
	"context"

	"github.com/libp2p/go-libp2p-net"
	"github.com/libp2p/go-libp2p-pubsub"
)

type broadcastCtxKey struct{}

type unicastCtxKey struct{}

// GetUnicastStream retrieves net.Stream from unicast request context.
func GetUnicastStream(ctx context.Context) (net.Stream, bool) {
	s, ok := ctx.Value(unicastCtxKey{}).(net.Stream)
	return s, ok
}

// GetBroadcastMsg retrieves *pubsub.Message from broadcast message context.
func GetBroadcastMsg(ctx context.Context) (*pubsub.Message, bool) {
	msg, ok := ctx.Value(broadcastCtxKey{}).(*pubsub.Message)
	return msg, ok
}
