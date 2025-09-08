package stakingindex

import (
	"context"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
)

// EventProcessorBuilder is the interface to build event processor
type EventProcessorBuilder interface {
	Build(context.Context, staking.EventHandler) staking.EventProcessor
}

type eventProcessorBuilder struct {
	contractAddr address.Address
	timestamped  bool
	muteHeight   uint64
}

func newEventProcessorBuilder(contractAddr address.Address, timestamped bool, muteHeight uint64) *eventProcessorBuilder {
	return &eventProcessorBuilder{
		contractAddr: contractAddr,
		timestamped:  timestamped,
		muteHeight:   muteHeight,
	}
}

func (b *eventProcessorBuilder) Build(ctx context.Context, handler staking.EventHandler) staking.EventProcessor {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	muted := b.muteHeight > 0 && blkCtx.BlockHeight >= b.muteHeight
	return newEventProcessor(b.contractAddr, blkCtx, handler, b.timestamped, muted)
}
