// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

// ResolveBLSProducer returns the iotex addresses corresponding to a
// BLS-signed block's producer pubkey: the operator address (used by
// consumers that previously read blkCtx.Producer for an iotex-shaped
// identity — slasher productivity, log lines, etc.) and the reward
// address (used by EVM Coinbase and rewarding/reward.go post-fork in
// place of the legacy "Producer == fee_recipient" coincidence).
//
// blsPubKey must be the 48-byte BLS pubkey carried on the block header
// (Header.ProducerPubKey()). The lookup is one linear scan over the
// candidate-center via GetByBLSPubKey — registration is rare and the
// active candidate set is bounded.
//
// Returns a wrapped error when no candidate has registered the supplied
// BLS pubkey. blockchain.go uses this signal to fail block validation
// early instead of installing a half-populated BlockCtx.
func ResolveBLSProducer(ctx context.Context, sr protocol.StateReader, blsPubKey []byte) (operator address.Address, reward address.Address, err error) {
	if len(blsPubKey) == 0 {
		return nil, nil, errors.New("empty BLS pubkey")
	}
	csr, err := ConstructBaseView(sr)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to construct candidate state reader")
	}
	cand := csr.BaseView().candCenter.GetByBLSPubKey(blsPubKey)
	if cand == nil {
		return nil, nil, errors.Errorf("no candidate registered the BLS pubkey 0x%x", blsPubKey)
	}
	if cand.Operator == nil {
		return nil, nil, errors.New("candidate has nil operator address")
	}
	if cand.Reward == nil {
		return nil, nil, errors.New("candidate has nil reward address")
	}
	return cand.Operator, cand.Reward, nil
}
