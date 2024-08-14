package dispatcher

import (
	"context"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Subscriber is the dispatcher subscriber interface
type Subscriber interface {
	ReportFullness(context.Context, iotexrpc.MessageType, float32)
	HandleAction(context.Context, *iotextypes.Action) error
	HandleBlock(context.Context, string, *iotextypes.Block) error
	HandleSyncRequest(context.Context, peer.AddrInfo, *iotexrpc.BlockSync) error
	HandleConsensusMsg(*iotextypes.ConsensusMessage) error
	HandleNodeInfoRequest(context.Context, peer.AddrInfo, *iotextypes.NodeInfoRequest) error
	HandleNodeInfo(context.Context, string, *iotextypes.NodeInfo) error
	HandleActionRequest(ctx context.Context, peer peer.AddrInfo, actHash hash.Hash256) error
	HandleActionHash(ctx context.Context, actHash hash.Hash256, from string) error
}
