// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package producer

import (
	"context"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	// BlockReward indicates that the action is to grant block reward
	BlockReward = iota
	// EpochReward indicates that the action is to grant epoch reward
	EpochReward
)

var (
	adminKey                    = []byte("admin")
	fundKey                     = []byte("fund")
	blockRewardHistoryKeyPrefix = []byte("blockRewardHistory")
	epochRewardHistoryKeyPrefix = []byte("epochRewardHistory")
	accountKeyPrefix            = []byte("account")
)

// Protocol defines the protocol of block producer fund operation and block producer rewarding process.
type Protocol struct {
	addr      address.Address
	keyPrefix []byte
}

// NewProtocol instantiates a block producer protocol instance
func NewProtocol(caller address.Address, nonce uint64) *Protocol {
	var nonceBytes [8]byte
	enc.MachineEndian.PutUint64(nonceBytes[:], nonce)
	h := hash.Hash160b(append(caller.Bytes(), nonceBytes[:]...))
	return &Protocol{
		addr:      address.New(h),
		keyPrefix: h,
	}
}

// Handle handles the actions on the block producer protocol
func (p *Protocol) Handle(
	ctx context.Context,
	act action.Action,
	sm protocol.StateManager,
) (*action.Receipt, error) {
	// TODO: simplify the boilerplate
	switch act := act.(type) {
	case *SetBlockProducerReward:
		switch act.RewardType() {
		case BlockReward:
			gasConsumed, err := act.IntrinsicGas()
			if err != nil {
				return p.createReceipt(1, act.Hash(), 0), nil
			}
			if err := p.SetBlockReward(ctx, sm, act.Amount()); err != nil {
				return p.createReceipt(1, act.Hash(), gasConsumed), nil
			}
			return p.createReceipt(0, act.Hash(), gasConsumed), nil
		case EpochReward:
			gasConsumed, err := act.IntrinsicGas()
			if err != nil {
				return p.createReceipt(1, act.Hash(), 0), nil
			}
			if err := p.SetEpochReward(ctx, sm, act.Amount()); err != nil {
				return p.createReceipt(1, act.Hash(), gasConsumed), nil
			}
			return p.createReceipt(0, act.Hash(), gasConsumed), nil
		}
	case *DonateToProducerFund:
		gasConsumed, err := act.IntrinsicGas()
		if err != nil {
			return p.createReceipt(1, act.Hash(), 0), nil
		}
		if err := p.Donate(ctx, sm, act.Amount()); err != nil {
			return p.createReceipt(1, act.Hash(), gasConsumed), nil
		}
		return p.createReceipt(0, act.Hash(), gasConsumed), nil
	case *ClaimFromProducerFund:
		gasConsumed, err := act.IntrinsicGas()
		if err != nil {
			return p.createReceipt(1, act.Hash(), 0), nil
		}
		if err := p.Claim(ctx, sm, act.Amount()); err != nil {
			return p.createReceipt(1, act.Hash(), gasConsumed), nil
		}
		return p.createReceipt(0, act.Hash(), gasConsumed), nil
	case *GrantBlockProducerReward:
		switch act.RewardType() {
		case BlockReward:
			gasConsumed, err := act.IntrinsicGas()
			if err != nil {
				return p.createReceipt(1, act.Hash(), 0), nil
			}
			if err := p.GrantBlockReward(ctx, sm); err != nil {
				return p.createReceipt(1, act.Hash(), gasConsumed), nil
			}
			return p.createReceipt(0, act.Hash(), gasConsumed), nil
		case EpochReward:
			gasConsumed, err := act.IntrinsicGas()
			if err != nil {
				return p.createReceipt(1, act.Hash(), 0), nil
			}
			if err := p.GrantEpochReward(ctx, sm); err != nil {
				return p.createReceipt(1, act.Hash(), gasConsumed), nil
			}
			return p.createReceipt(0, act.Hash(), gasConsumed), nil
		}
	}
	return nil, nil
}

// Validate validates the actions on the block producer protocol
func (p *Protocol) Validate(
	ctx context.Context,
	act action.Action,
) error {
	// TODO: validate interface shouldn't be required for protocol code
	return nil
}

func (p *Protocol) state(sm protocol.StateManager, key []byte, value interface{}) error {
	keyHash := byteutil.BytesTo20B(hash.Hash160b(append(p.keyPrefix, key...)))
	return sm.State(keyHash, value)
}

func (p *Protocol) putState(sm protocol.StateManager, key []byte, value interface{}) error {
	keyHash := byteutil.BytesTo20B(hash.Hash160b(append(p.keyPrefix, key...)))
	return sm.PutState(keyHash, value)
}

func (p *Protocol) deleteState(sm protocol.StateManager, key []byte) error {
	keyHash := byteutil.BytesTo20B(hash.Hash160b(append(p.keyPrefix, key...)))
	return sm.DelState(keyHash)
}

func (p *Protocol) createReceipt(status uint64, actHash hash.Hash32B, gasConsumed uint64) *action.Receipt {
	// TODO: need to review the fields
	return &action.Receipt{
		ReturnValue:     nil,
		Status:          0,
		ActHash:         actHash,
		GasConsumed:     gasConsumed,
		ContractAddress: p.addr.Bech32(),
		Logs:            nil,
	}
}
