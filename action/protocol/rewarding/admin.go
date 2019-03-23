// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rewarding

import (
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
	blockReward                    *big.Int
	epochReward                    *big.Int
	numDelegatesForEpochReward     uint64
	foundationBonus                *big.Int
	numDelegatesForFoundationBonus uint64
	foundationBonusLastEpoch       uint64
	productivityThreshold          uint64
}

// Serialize serializes admin state into bytes
func (a admin) Serialize() ([]byte, error) {
	gen := rewardingpb.Admin{
		BlockReward:                    a.blockReward.String(),
		EpochReward:                    a.epochReward.String(),
		NumDelegatesForEpochReward:     a.numDelegatesForEpochReward,
		FoundationBonus:                a.foundationBonus.String(),
		NumDelegatesForFoundationBonus: a.numDelegatesForFoundationBonus,
		FoundationBonusLastEpoch:       a.foundationBonusLastEpoch,
		ProductivityThreshold:          a.productivityThreshold,
	}
	return proto.Marshal(&gen)
}

// Deserialize deserializes bytes into admin state
func (a *admin) Deserialize(data []byte) error {
	gen := rewardingpb.Admin{}
	if err := proto.Unmarshal(data, &gen); err != nil {
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
	foundationBonus, ok := big.NewInt(0).SetString(gen.FoundationBonus, 10)
	if !ok {
		return errors.New("failed to set bootstrap bonus")
	}
	a.blockReward = blockReward
	a.epochReward = epochReward
	a.numDelegatesForEpochReward = gen.NumDelegatesForEpochReward
	a.foundationBonus = foundationBonus
	a.numDelegatesForFoundationBonus = gen.NumDelegatesForFoundationBonus
	a.foundationBonusLastEpoch = gen.FoundationBonusLastEpoch
	a.productivityThreshold = gen.ProductivityThreshold
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
	initBalance *big.Int,
	blockReward *big.Int,
	epochReward *big.Int,
	numDelegatesForEpochReward uint64,
	exemptAddrs []address.Address,
	foundationBonus *big.Int,
	numDelegatesForFoundationBonus uint64,
	foundationBonusLastEpoch uint64,
	productivityThreshold uint64,
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
			blockReward:                    blockReward,
			epochReward:                    epochReward,
			numDelegatesForEpochReward:     numDelegatesForEpochReward,
			foundationBonus:                foundationBonus,
			numDelegatesForFoundationBonus: numDelegatesForFoundationBonus,
			foundationBonusLastEpoch:       foundationBonusLastEpoch,
			productivityThreshold:          productivityThreshold,
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

// FoundationBonus returns the foundation bonus amount
func (p *Protocol) FoundationBonus(_ context.Context, sm protocol.StateManager) (*big.Int, error) {
	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return nil, err
	}
	return a.foundationBonus, nil
}

// FoundationBonusLastEpoch returns the last epoch when the foundation bonus will still be granted
func (p *Protocol) FoundationBonusLastEpoch(_ context.Context, sm protocol.StateManager) (uint64, error) {
	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return 0, err
	}
	return a.foundationBonusLastEpoch, nil
}

// NumDelegatesForFoundationBonus returns the number of delegates that will get foundation bonus
func (p *Protocol) NumDelegatesForFoundationBonus(_ context.Context, sm protocol.StateManager) (uint64, error) {
	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return 0, err
	}
	return a.numDelegatesForFoundationBonus, nil
}

// ProductivityThreshold returns the productivity threshold
func (p *Protocol) ProductivityThreshold(_ context.Context, sm protocol.StateManager) (uint64, error) {
	a := admin{}
	if err := p.state(sm, adminKey, &a); err != nil {
		return 0, err
	}
	return a.productivityThreshold, nil
}

func (p *Protocol) assertAmount(amount *big.Int) error {
	if amount.Cmp(big.NewInt(0)) >= 0 {
		return nil
	}
	return errors.Errorf("reward amount %s shouldn't be negative", amount.String())
}

func (p *Protocol) assertZeroBlockHeight(height uint64) error {
	if height != 0 {
		return errors.Errorf("current block height %d is not zero", height)
	}
	return nil
}
