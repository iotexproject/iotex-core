// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"bytes"
	"context"
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// admin stores the admin data of the rewarding protocol
type admin struct {
	admin       address.Address
	BlockReward *big.Int
	EpochReward *big.Int
}

// Serialize serializes admin state into bytes
func (a admin) Serialize() ([]byte, error) {
	gen := rewardingpb.Admin{
		Admin:       a.admin.Bytes(),
		BlockReward: a.BlockReward.Bytes(),
		EpochReward: a.EpochReward.Bytes(),
	}
	return proto.Marshal(&gen)
}

// Deserialize deserializes bytes into admin state
func (a *admin) Deserialize(data []byte) error {
	gen := rewardingpb.Admin{}
	if err := proto.Unmarshal(data, &gen); err != nil {
		return err
	}
	var err error
	if a.admin, err = address.FromBytes(gen.Admin); err != nil {
		return err
	}
	a.BlockReward = big.NewInt(0).SetBytes(gen.BlockReward)
	a.EpochReward = big.NewInt(0).SetBytes(gen.EpochReward)
	return nil
}

// Initialize initializes the rewarding protocol by setting the original admin, block and epoch reward
func (p *Protocol) Initialize(
	ctx context.Context,
	sm protocol.StateManager,
	blockReward *big.Int,
	epochReward *big.Int,
) error {
	raCtx, ok := protocol.GetRunActionsCtx(ctx)
	if !ok {
		log.S().Panic("Miss run action context")
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
			admin:       raCtx.Caller,
			BlockReward: blockReward,
			EpochReward: epochReward,
		},
	); err != nil {
		return err
	}
	if err := p.putState(
		sm,
		fundKey,
		&fund{
			totalBalance:     big.NewInt(0),
			unclaimedBalance: big.NewInt(0),
		},
	); err != nil {
		return err
	}
	return nil
}

// Admin returns the address of current admin
func (p *Protocol) Admin(
	_ context.Context,
	sm protocol.StateManager,
) (address.Address, error) {
	admin := admin{}
	if err := p.state(sm, adminKey, &admin); err != nil {
		return nil, err
	}
	return admin.admin, nil
}

// SetAdmin sets a new admin address. Only the current admin could make this change
func (p *Protocol) SetAdmin(
	ctx context.Context,
	sm protocol.StateManager,
	addr address.Address,
) error {
	raCtx, ok := protocol.GetRunActionsCtx(ctx)
	if !ok {
		log.S().Panic("Miss run action context")
	}
	if err := p.assertAdminPermission(raCtx, sm); err != nil {
		return err
	}
	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return err
	}
	a.admin = addr
	if err := p.putState(sm, adminKey, &a); err != nil {
		return err
	}
	return nil
}

// BlockReward returns the block reward amount
func (p *Protocol) BlockReward(
	_ context.Context,
	sm protocol.StateManager,
) (*big.Int, error) {
	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return nil, err
	}
	return a.BlockReward, nil
}

// SetBlockReward sets the block reward amount for the block rewarding. Only the current admin could make this change
func (p *Protocol) SetBlockReward(
	ctx context.Context,
	sm protocol.StateManager,
	amount *big.Int,
) error {
	return p.setReward(ctx, sm, amount, true)
}

// EpochReward returns the epoch reward amount
func (p *Protocol) EpochReward(
	_ context.Context,
	sm protocol.StateManager,
) (*big.Int, error) {
	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return nil, err
	}
	return a.EpochReward, nil
}

// SetEpochReward sets the epoch reward amount shared by all beneficiaries in an epoch. Only the current admin could
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
	if bytes.Equal(a.admin.Bytes(), raCtx.Caller.Bytes()) {
		return nil
	}
	return errors.Errorf("%s is not the rewarding protocol admin", raCtx.Caller.String())
}

func (p *Protocol) setReward(
	ctx context.Context,
	sm protocol.StateManager,
	amount *big.Int,
	blockLevel bool,
) error {
	raCtx, ok := protocol.GetRunActionsCtx(ctx)
	if !ok {
		log.S().Panic("Miss run action context")
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
