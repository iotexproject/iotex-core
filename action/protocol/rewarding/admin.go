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
)

// admin stores the admin data of the rewarding protocol
type admin struct {
	admin                      address.Address
	blockReward                *big.Int
	epochReward                *big.Int
	numDelegatesForEpochReward uint64
}

// Serialize serializes admin state into bytes
func (a admin) Serialize() ([]byte, error) {
	gen := rewardingpb.Admin{
		Admin:                      a.admin.Bytes(),
		BlockReward:                a.blockReward.String(),
		EpochReward:                a.epochReward.String(),
		NumDelegatesForEpochReward: a.numDelegatesForEpochReward,
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
	blockReward, ok := big.NewInt(0).SetString(gen.BlockReward, 10)
	if !ok {
		return errors.New("failed to set block reward")
	}
	epochReward, ok := big.NewInt(0).SetString(gen.EpochReward, 10)
	if !ok {
		return errors.New("failed to set epoch reward")
	}
	a.blockReward = blockReward
	a.epochReward = epochReward
	a.numDelegatesForEpochReward = gen.NumDelegatesForEpochReward
	return nil
}

// exempt stores the addresses that exempt from epoch reward
type exempt struct {
	addrs []address.Address
}

// Serialize serializes exempt state into bytes
func (e *exempt) Serialize() ([]byte, error) {
	epb := rewardingpb.Exempt{}
	for _, addr := range e.addrs {
		epb.Addrs = append(epb.Addrs, addr.Bytes())
	}
	return proto.Marshal(&epb)
}

// Deserialize deserializes bytes into exempt state
func (e *exempt) Deserialize(data []byte) error {
	epb := rewardingpb.Exempt{}
	if err := proto.Unmarshal(data, &epb); err != nil {
		return err
	}
	e.addrs = nil
	for _, addrBytes := range epb.Addrs {
		addr, err := address.FromBytes(addrBytes)
		if err != nil {
			return err
		}
		e.addrs = append(e.addrs, addr)
	}
	return nil
}

// Initialize initializes the rewarding protocol by setting the original admin, block and epoch reward
func (p *Protocol) Initialize(
	ctx context.Context,
	sm protocol.StateManager,
	adminAddr address.Address,
	initBalance *big.Int,
	blockReward *big.Int,
	epochReward *big.Int,
	numDelegatesForEpochReward uint64,
	exemptAddrs []address.Address,
) error {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	if err := p.assertZeroBlockHeight(raCtx.BlockHeight); err != nil {
		return err
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
			admin:                      adminAddr,
			blockReward:                blockReward,
			epochReward:                epochReward,
			numDelegatesForEpochReward: numDelegatesForEpochReward,
		},
	); err != nil {
		return err
	}
	if err := p.putState(
		sm,
		fundKey,
		&fund{
			totalBalance:     initBalance,
			unclaimedBalance: initBalance,
		},
	); err != nil {
		return err
	}
	return p.putState(
		sm,
		exemptKey,
		&exempt{
			addrs: exemptAddrs,
		},
	)
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
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	if err := p.assertAdminPermission(raCtx, sm); err != nil {
		return err
	}
	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return err
	}
	a.admin = addr
	return p.putState(sm, adminKey, &a)
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
	return a.blockReward, nil
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
	return a.epochReward, nil
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

// NumDelegatesForEpochReward returns the number of candidates sharing an epoch reward
func (p *Protocol) NumDelegatesForEpochReward(
	_ context.Context,
	sm protocol.StateManager,
) (uint64, error) {
	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return 0, err
	}
	return a.numDelegatesForEpochReward, nil
}

// SetNumDelegatesForEpochReward sets the number of candidates sharing an epoch reward
func (p *Protocol) SetNumDelegatesForEpochReward(
	ctx context.Context,
	sm protocol.StateManager,
	num uint64,
) error {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	if err := p.assertAdminPermission(raCtx, sm); err != nil {
		return err
	}
	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return err
	}
	a.numDelegatesForEpochReward = num
	return p.putState(sm, adminKey, &a)
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

func (p *Protocol) assertZeroBlockHeight(height uint64) error {
	if height != 0 {
		return errors.Errorf("current block height %d is not zero", height)
	}
	return nil
}

func (p *Protocol) setReward(
	ctx context.Context,
	sm protocol.StateManager,
	amount *big.Int,
	blockLevel bool,
) error {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
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
		a.blockReward = amount
	} else {
		a.epochReward = amount
	}
	return p.putState(sm, adminKey, &a)
}
