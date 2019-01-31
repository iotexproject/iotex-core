// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package producer

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"

	"github.com/iotexproject/iotex-core/pkg/hash"

	"github.com/iotexproject/iotex-core/pkg/enc"

	"github.com/iotexproject/iotex-core/address"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
)

var (
	adminKey = []byte("admin")
	fundKey  = []byte("fund")
)

// Protocol defines the protocol of block producer fund operation and block producer rewarding process.
type Protocol struct {
	keyPrefix []byte
}

// NewProtocol instantiates a block producer protocol instance
func NewProtocol(caller address.Address, nonce uint64) *Protocol {
	var nonceBytes [8]byte
	enc.MachineEndian.PutUint64(nonceBytes[:], nonce)
	return &Protocol{
		keyPrefix: hash.Hash160b(append(caller.Bytes(), nonceBytes[:]...)),
	}
}

// Handle handles the actions on the block producer protocol
func (p *Protocol) Handle(
	ctx context.Context,
	act action.Action,
	sm protocol.StateManager,
) (*action.Receipt, error) {
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

func (p *Protocol) Claim(
	ctx context.Context,
	sm protocol.StateManager,
	amount *big.Int,
) error {
	return nil
}

func (p *Protocol) UnclaimedBalance(
	ctx context.Context,
	sm protocol.StateManager,
) (*big.Int, error) {
	return nil, nil
}

func (p *Protocol) SettleBlockReward(
	ctx context.Context,
	sm protocol.StateManager) error {
	return nil
}

func (p *Protocol) SettleEpochReward(
	ctx context.Context,
	sm protocol.StateManager,
) error {
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
