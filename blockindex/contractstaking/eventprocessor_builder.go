package contractstaking

import (
	"context"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
)

type eventProcessorBuilder struct {
	contractAddr address.Address
}

func newEventProcessorBuilder(contractAddr address.Address) *eventProcessorBuilder {
	return &eventProcessorBuilder{
		contractAddr: contractAddr,
	}
}

func (b *eventProcessorBuilder) Build(ctx context.Context, handler staking.EventHandler) staking.EventProcessor {
	return newContractStakingEventProcessor(b.contractAddr, handler)
}
