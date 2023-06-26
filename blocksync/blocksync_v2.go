package blocksync

import (
	"context"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/libp2p/go-libp2p-core/peer"
)

type blockSyncerV2 struct {
	tipHeightHandler     TipHeight
	blockByHeightHandler BlockByHeight
	commitBlockHandler   CommitBlock
	p2pNeighbor          Neighbors
	unicastOutbound      UniCastOutbound
	blockP2pPeer         BlockPeer
}

func NewBlockSyncerV2(
	cfg Config,
	tipHeightHandler TipHeight,
	blockByHeightHandler BlockByHeight,
	commitBlockHandler CommitBlock,
	p2pNeighbor Neighbors,
	uniCastHandler UniCastOutbound,
	blockP2pPeer BlockPeer,
) (BlockSync, error) {
	bs := &blockSyncerV2{
		tipHeightHandler:     tipHeightHandler,
		blockByHeightHandler: blockByHeightHandler,
		commitBlockHandler:   commitBlockHandler,
		p2pNeighbor:          p2pNeighbor,
		unicastOutbound:      uniCastHandler,
		blockP2pPeer:         blockP2pPeer,
	}
	return bs, nil
}

func (*blockSyncerV2) Start(context.Context) error {
	return nil
}

func (*blockSyncerV2) Stop(context.Context) error {
	return nil
}

func (*blockSyncerV2) TargetHeight() uint64 {
	return 0
}

func (*blockSyncerV2) ProcessSyncRequest(context.Context, peer.AddrInfo, uint64, uint64) error {
	return nil
}

func (*blockSyncerV2) ProcessBlock(context.Context, string, *block.Block) error {
	return nil
}

func (*blockSyncerV2) SyncStatus() (uint64, uint64, uint64, string) {
	return 0, 0, 0, ""
}

func (*blockSyncerV2) BuildReport() string {
	return ""
}
