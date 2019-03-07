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
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// fund stores the balance of the rewarding fund. The difference between total and available balance should be
// equal to the unclaimed balance in all reward accounts
type fund struct {
	totalBalance     *big.Int
	unclaimedBalance *big.Int
}

// Serialize serializes fund state into bytes
func (f fund) Serialize() ([]byte, error) {
	gen := rewardingpb.Fund{
		TotalBalance:     f.totalBalance.String(),
		UnclaimedBalance: f.unclaimedBalance.String(),
	}
	return proto.Marshal(&gen)
}

// Deserialize deserializes bytes into fund state
func (f *fund) Deserialize(data []byte) error {
	gen := rewardingpb.Fund{}
	if err := proto.Unmarshal(data, &gen); err != nil {
		return err
	}
	totalBalance, ok := big.NewInt(0).SetString(gen.TotalBalance, 10)
	if !ok {
		return errors.New("failed to set total balance")
	}
	unclaimedBalance, ok := big.NewInt(0).SetString(gen.UnclaimedBalance, 10)
	if !ok {
		return errors.New("failed to set unclaimed balance")
	}
	f.totalBalance = totalBalance
	f.unclaimedBalance = unclaimedBalance
	return nil
}

// Deposit deposits token into the rewarding fund
func (p *Protocol) Deposit(
	ctx context.Context,
	sm protocol.StateManager,
	amount *big.Int,
) error {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	if err := p.assertEnoughBalance(raCtx, sm, amount); err != nil {
		return err
	}
	// Subtract balance from caller
	acc, err := accountutil.LoadOrCreateAccount(sm, raCtx.Caller.String(), big.NewInt(0))
	if err != nil {
		return err
	}
	acc.Balance = big.NewInt(0).Sub(acc.Balance, amount)
	accountutil.StoreAccount(sm, raCtx.Caller.String(), acc)
	// Add balance to fund
	f := fund{}
	if err := p.state(sm, fundKey, &f); err != nil {
		return err
	}
	f.totalBalance = big.NewInt(0).Add(f.totalBalance, amount)
	f.unclaimedBalance = big.NewInt(0).Add(f.unclaimedBalance, amount)
	return p.putState(sm, fundKey, &f)
}

// TotalBalance returns the total balance of the rewarding fund
func (p *Protocol) TotalBalance(
	ctx context.Context,
	sm protocol.StateManager,
) (*big.Int, error) {
	f := fund{}
	if err := p.state(sm, fundKey, &f); err != nil {
		return nil, err
	}
	return f.totalBalance, nil
}

// AvailableBalance returns the available balance of the rewarding fund
func (p *Protocol) AvailableBalance(
	ctx context.Context,
	sm protocol.StateManager,
) (*big.Int, error) {
	f := fund{}
	if err := p.state(sm, fundKey, &f); err != nil {
		return nil, err
	}
	return f.unclaimedBalance, nil
}

func (p *Protocol) assertEnoughBalance(
	raCtx protocol.RunActionsCtx,
	sm protocol.StateManager,
	amount *big.Int,
) error {
	acc, err := accountutil.LoadAccount(sm, hash.BytesToHash160(raCtx.Caller.Bytes()))
	if err != nil {
		return err
	}
	if acc.Balance.Cmp(amount) < 0 {
		return errors.New("balance is not enough for donation")
	}
	return nil
}

// DepositGas deposits gas into the rewarding fund
func DepositGas(ctx context.Context, sm protocol.StateManager, amount *big.Int, registry *protocol.Registry) error {
	// If the gas fee is 0, return immediately
	if amount.Cmp(big.NewInt(0)) == 0 {
		return nil
	}
	// TODO: we bypass the gas deposit for the actions in genesis block. Later we should remove this after we remove
	// genesis actions
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	if raCtx.BlockHeight == 0 {
		return nil
	}
	if registry == nil {
		return nil
	}
	p, ok := registry.Find(ProtocolID)
	if !ok {
		return nil
	}
	rp, ok := p.(*Protocol)
	if !ok {
		log.S().Panicf("Protocol %d is not a rewarding protocol", ProtocolID)
	}
	return rp.Deposit(ctx, sm, amount)
}
