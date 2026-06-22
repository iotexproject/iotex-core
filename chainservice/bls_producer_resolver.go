// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package chainservice

import (
	"context"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
)

// stakingBLSProducerResolver adapts the staking protocol's
// BLS-pubkey → candidate lookup to blockchain.BLSProducerResolver.
// Lives in chainservice rather than blockchain because the
// blockchain package must not import staking (the reverse is fine
// — staking sits on top of blockchain).
type stakingBLSProducerResolver struct {
	sr protocol.StateReader
}

// ResolveBLSProducer satisfies blockchain.BLSProducerResolver.
func (r *stakingBLSProducerResolver) ResolveBLSProducer(ctx context.Context, blsPubKey []byte) (address.Address, address.Address, error) {
	return staking.ResolveBLSProducer(ctx, r.sr, blsPubKey)
}
