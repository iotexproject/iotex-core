// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
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
	foundationBonusExtension       []genesis.Period
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
		FoundationBonusExtension:       a.periodProto(),
	}
	return proto.Marshal(&gen)
}

func (a *admin) periodProto() []*rewardingpb.Period {
	if len(a.foundationBonusExtension) == 0 {
		return nil
	}

	pb := make([]*rewardingpb.Period, len(a.foundationBonusExtension))
	for i, ext := range a.foundationBonusExtension {
		pb[i] = &rewardingpb.Period{
			Start: ext.Start,
			End:   ext.End,
		}
	}
	return pb
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

	if len(gen.FoundationBonusExtension) == 0 {
		return nil
	}
	a.foundationBonusExtension = make([]genesis.Period, len(gen.FoundationBonusExtension))
	for i, v := range gen.FoundationBonusExtension {
		a.foundationBonusExtension[i] = genesis.Period{
			Start: v.Start,
			End:   v.End,
		}
	}
	return nil
}

func (a *admin) hasFoundationBonusExtension() bool {
	// starting Kamchatka height, we add the foundation bonus extension epoch into admin{} struct
	return len(a.foundationBonusExtension) > 0
}

func (a *admin) grantFoundationBonus(epoch uint64) bool {
	if epoch <= a.foundationBonusLastEpoch {
		return true
	}
	for _, ext := range a.foundationBonusExtension {
		if epoch >= ext.Start && epoch <= ext.End {
			return true
		}
	}
	return false
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

// CreateGenesisStates initializes the rewarding protocol by setting the original admin, block and epoch reward
func (p *Protocol) CreateGenesisStates(
	ctx context.Context,
	sm protocol.StateManager,
) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	g := genesis.MustExtractGenesisContext(ctx)
	if err := p.assertZeroBlockHeight(blkCtx.BlockHeight); err != nil {
		return err
	}

	blockReward := g.BlockReward()
	if err := p.assertAmount(blockReward); err != nil {
		return err
	}

	epochReward := g.EpochReward()
	if err := p.assertAmount(epochReward); err != nil {
		return err
	}

	if err := p.putState(
		ctx,
		sm,
		adminKey,
		&admin{
			blockReward:                    blockReward,
			epochReward:                    epochReward,
			numDelegatesForEpochReward:     g.NumDelegatesForEpochReward,
			foundationBonus:                g.FoundationBonus(),
			numDelegatesForFoundationBonus: g.NumDelegatesForFoundationBonus,
			foundationBonusLastEpoch:       g.FoundationBonusLastEpoch,
			productivityThreshold:          g.ProductivityThreshold,
		},
	); err != nil {
		return err
	}

	initBalance := g.InitBalance()
	if err := p.putState(
		ctx,
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
		ctx,
		sm,
		exemptKey,
		&exempt{
			addrs: g.ExemptAddrsFromEpochReward(),
		},
	)
}

// BlockReward returns the block reward amount
func (p *Protocol) BlockReward(
	ctx context.Context,
	sm protocol.StateReader,
) (*big.Int, error) {
	a := admin{}
	if _, err := p.state(ctx, sm, adminKey, &a); err != nil {
		return nil, err
	}
	return a.blockReward, nil
}

// EpochReward returns the epoch reward amount
func (p *Protocol) EpochReward(
	ctx context.Context,
	sm protocol.StateReader,
) (*big.Int, error) {
	a := admin{}
	if _, err := p.state(ctx, sm, adminKey, &a); err != nil {
		return nil, err
	}
	return a.epochReward, nil
}

// NumDelegatesForEpochReward returns the number of candidates sharing an epoch reward
func (p *Protocol) NumDelegatesForEpochReward(
	ctx context.Context,
	sm protocol.StateManager,
) (uint64, error) {
	a := admin{}
	if _, err := p.state(ctx, sm, adminKey, &a); err != nil {
		return 0, err
	}
	return a.numDelegatesForEpochReward, nil
}

// FoundationBonus returns the foundation bonus amount
func (p *Protocol) FoundationBonus(ctx context.Context, sm protocol.StateReader) (*big.Int, error) {
	a := admin{}
	if _, err := p.state(ctx, sm, adminKey, &a); err != nil {
		return nil, err
	}
	return a.foundationBonus, nil
}

// FoundationBonusLastEpoch returns the last epoch when the foundation bonus will still be granted
func (p *Protocol) FoundationBonusLastEpoch(ctx context.Context, sm protocol.StateReader) (uint64, error) {
	a := admin{}
	if _, err := p.state(ctx, sm, adminKey, &a); err != nil {
		return 0, err
	}
	return a.foundationBonusLastEpoch, nil
}

// NumDelegatesForFoundationBonus returns the number of delegates that will get foundation bonus
func (p *Protocol) NumDelegatesForFoundationBonus(ctx context.Context, sm protocol.StateReader) (uint64, error) {
	a := admin{}
	if _, err := p.state(ctx, sm, adminKey, &a); err != nil {
		return 0, err
	}
	return a.numDelegatesForFoundationBonus, nil
}

// ProductivityThreshold returns the productivity threshold
func (p *Protocol) ProductivityThreshold(ctx context.Context, sm protocol.StateManager) (uint64, error) {
	a := admin{}
	if _, err := p.state(ctx, sm, adminKey, &a); err != nil {
		return 0, err
	}
	return a.productivityThreshold, nil
}

// SetReward updates block or epoch reward amount
func (p *Protocol) SetReward(
	ctx context.Context,
	sm protocol.StateManager,
	amount *big.Int,
	blockLevel bool,
) error {
	if err := p.assertAmount(amount); err != nil {
		return err
	}
	a := admin{}
	if _, err := p.state(ctx, sm, adminKey, &a); err != nil {
		return err
	}
	if blockLevel {
		a.blockReward = amount
	} else {
		a.epochReward = amount
	}
	return p.putState(ctx, sm, adminKey, &a)
}

func (p *Protocol) assertAmount(amount *big.Int) error {
	if amount.Cmp(big.NewInt(0)) >= 0 {
		return nil
	}
	return errors.Errorf("amount %s shouldn't be negative", amount.String())
}

func (p *Protocol) assertZeroBlockHeight(height uint64) error {
	if height != 0 {
		return errors.Errorf("current block height %d is not zero", height)
	}
	return nil
}
