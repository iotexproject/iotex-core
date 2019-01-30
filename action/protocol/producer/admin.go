// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package producer

import (
	"bytes"
	"context"
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/producer/producerpb"
	"github.com/iotexproject/iotex-core/address"
)

var (
	adminKey = []byte("admin")
)

type admin struct {
	Admin       address.Address
	BlockReward *big.Int
	EpochReward *big.Int
}

// Serialize serializes admin state into bytes
func (a admin) Serialize() ([]byte, error) {
	gen := producerpb.Admin{
		Admin:       a.Admin.Bytes(),
		BlockReward: a.BlockReward.Bytes(),
		EpochReward: a.EpochReward.Bytes(),
	}
	return proto.Marshal(&gen)
}

// Deserialize deserializes bytes into admin state
func (a *admin) Deserialize(data []byte) error {
	gen := producerpb.Admin{}
	if err := proto.Unmarshal(data, &gen); err != nil {
		return err
	}
	var err error
	if a.Admin, err = address.BytesToAddress(gen.Admin); err != nil {
		return err
	}
	a.BlockReward = big.NewInt(0).SetBytes(gen.BlockReward)
	a.EpochReward = big.NewInt(0).SetBytes(gen.EpochReward)
	return nil
}

// Initialize initializes the block producer protocol by setting the original admin, block and epoch reward
func (p *Protocol) Initialize(
	ctx context.Context,
	sm protocol.StateManager,
	blockReward *big.Int,
	epochReward *big.Int,
) error {
	raCtx, ok := protocol.GetRunActionsCtx(ctx)
	if !ok {
		return errors.New("miss action validation context")
	}
	if err := p.assertAmount(blockReward); err != nil {
		return err
	}
	if err := p.assertAmount(epochReward); err != nil {
		return err
	}
	if err := p.putState(
		sm,
		adminKey,
		&admin{
			Admin:       raCtx.Caller,
			BlockReward: blockReward,
			EpochReward: epochReward,
		},
	); err != nil {
		return err
	}
	return nil
}

// Admin returns the address of current admin
func (p *Protocol) Admin(
	ctx context.Context,
	sm protocol.StateManager,
) (address.Address, error) {
	admin := admin{}
	if err := p.state(sm, adminKey, &admin); err != nil {
		return nil, err
	}
	return admin.Admin, nil
}

// SetAdmin sets a new admin address. Only the current admin could make this change
func (p *Protocol) SetAdmin(
	ctx context.Context,
	sm protocol.StateManager,
	addr address.Address,
) error {
	raCtx, ok := protocol.GetRunActionsCtx(ctx)
	if !ok {
		return errors.New("miss action validation context")
	}
	if err := p.assertAdminPermission(raCtx, sm); err != nil {
		return err
	}
	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return err
	}
	a.Admin = addr
	if err := p.putState(sm, adminKey, &a); err != nil {
		return err
	}
	return nil
}

// BlockReward returns the block reward amount
func (p *Protocol) BlockReward(
	ctx context.Context,
	sm protocol.StateManager,
) (*big.Int, error) {
	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return nil, err
	}
	return a.BlockReward, nil
}

// SetBlockReward sets the block reward amount for the block producer. Only the current admin could make this change
func (p *Protocol) SetBlockReward(
	ctx context.Context,
	sm protocol.StateManager,
	amount *big.Int,
) error {
	return p.setReward(ctx, sm, amount, true)
}

// EpochReward returns the epoch reward amount
func (p *Protocol) EpochReward(
	ctx context.Context,
	sm protocol.StateManager,
) (*big.Int, error) {
	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return nil, err
	}
	return a.EpochReward, nil
}

// SetEpochReward sets the epoch reward amount shared by all block producers in an epoch. Only the current admin could
// make this change
func (p *Protocol) SetEpochReward(
	ctx context.Context,
	sm protocol.StateManager,
	amount *big.Int,
) error {
	return p.setReward(ctx, sm, amount, false)
}

func (p *Protocol) assertAmount(amount *big.Int) error {
	if amount.Cmp(big.NewInt(0)) >= 0 {
		return nil
	}
	return errors.Errorf("reward amount %s shouldn't be negative", amount.String())
}

func (p *Protocol) assertAdminPermission(raCtx protocol.RunActionsCtx, sm protocol.StateManager) error {
	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return err
	}
	if bytes.Equal(a.Admin.Bytes(), raCtx.Caller.Bytes()) {
		return nil
	}
	return errors.Errorf("%s is not the block producer protocol admin", raCtx.Caller.Bech32())
}

func (p *Protocol) setReward(
	ctx context.Context,
	sm protocol.StateManager,
	amount *big.Int,
	blockLevel bool,
) error {
	raCtx, ok := protocol.GetRunActionsCtx(ctx)
	if !ok {
		return errors.New("miss action validation context")
	}
	if err := p.assertAdminPermission(raCtx, sm); err != nil {
		return err
	}
	if err := p.assertAmount(amount); err != nil {
		return err
	}
	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return err
	}
	if blockLevel {
		a.BlockReward = amount
	} else {
		a.EpochReward = amount
	}
	if err := p.putState(sm, adminKey, &a); err != nil {
		return err
	}
	return nil
}
